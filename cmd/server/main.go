package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"dc-monitor/cmd/logs"
	"dc-monitor/pkg/protocol"

	"github.com/gorilla/websocket"
)

var serverTriggerMutex sync.Mutex
var criticalStartTimes = make(map[int]time.Time)

const (
	UDPPort        = ":9000"
	HTTPPort       = ":8080"
	WorkerCount    = 16
	BufferSize     = 10000
	TicksPerSecond = 1000
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte)

func main() {

	addr, _ := net.ResolveUDPAddr("udp", UDPPort)
	conn, err := net.ListenUDP("udp", addr)

	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	logChannel := make(chan protocol.TelemetryPacket, 10000)
	go logs.CSVLoggerWorker(logChannel)

	packetQueue := make(chan []byte, BufferSize)
	var wg sync.WaitGroup

	for i := 1; i <= WorkerCount; i++ {
		wg.Add(1)
		go worker(i, packetQueue, &wg, logChannel)
	}

	// ==========================================
	// 2. SETUP DAS ROTAS WEB
	// ==========================================

	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/api/control", handleClientControl)
	//http.Handle("/", http.FileServer(http.Dir("./client")))

	go func() {
		fmt.Println("Servidor Web e Interface Gráfica na porta 8080...")
		if err := http.ListenAndServe(HTTPPort, nil); err != nil {
			log.Fatal("Erro fatal no servidor HTTP:", err)
		}
	}()

	// Inicia a Goroutine que dispara os WebSockets
	go handleMessages()

	// ==========================================
	// 3. O LOOP PRINCIPAL (UDP)
	// ==========================================
	networkBuffer := make([]byte, 2048)
	fmt.Println("Servidor Central escutando telemetria UDP na porta 9000...")

	for {
		n, _, err := conn.ReadFromUDP(networkBuffer)
		if err != nil {
			continue
		}
		dataCopy := make([]byte, n)
		copy(dataCopy, networkBuffer[:n])
		packetQueue <- dataCopy
	}
}

// ---------------------------------------------------------
// FUNÇÕES AUXILIARES DE REDE E WEBSOCKET
// ---------------------------------------------------------

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	clients[ws] = true
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			delete(clients, ws)
			break
		}
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func handleClientControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var msg protocol.ControlMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	msg.Requester = "user"

	var actuatorURL string
	if msg.Type == "hvac" {
		actuatorURL = os.Getenv("HVAC_SERVICE_URL")
	} else if msg.Type == "lb" {
		actuatorURL = os.Getenv("LB_SERVICE_URL")
	} else {
		http.Error(w, "Atuador desconhecido", http.StatusBadRequest)
		return
	}

	err := sendHTTPCommand(msg, actuatorURL)
	if err != nil {
		fmt.Printf("[ERRO MANUAL] Falha ao contatar Atuador %s: %v\n", msg.Type, err)
		http.Error(w, "Falha de comunicação com o atuador", http.StatusInternalServerError)
		return
	}

	fmt.Printf("[MANUAL] Servidor roteou comando de %s para o Nó %d\n", msg.Type, msg.TargetNode)
	w.WriteHeader(http.StatusOK)
}

func worker(id int, queue <-chan []byte, wg *sync.WaitGroup, logChannel chan<- protocol.TelemetryPacket) {
	defer wg.Done()

	for rawData := range queue {

		var header protocol.TelemetryPacket

		if err := json.Unmarshal(rawData, &header); err != nil {
			continue
		}

		// REGRA 1
		isCritical := header.CurrentState == 2

		serverTriggerMutex.Lock()

		if isCritical {
			startTime, exists := criticalStartTimes[header.ID]
			if !exists {

				criticalStartTimes[header.ID] = time.Now()
			} else if time.Since(startTime) >= 5*time.Second {

				delete(criticalStartTimes, header.ID)

				// Dispara o gatilho automático em uma rotina separada
				go autoHealNode(header.ID)
			}
		} else {
			delete(criticalStartTimes, header.ID)
		}

		serverTriggerMutex.Unlock()

		// REGRA 2
		isTickInterval := header.TickCount%TicksPerSecond == 0

		if isCritical || isTickInterval {
			broadcast <- rawData
		}

		select {
		case logChannel <- header:

		default:
			fmt.Printf("[AVISO] Canal de log cheio, descartando pacote do Nó %d\n", header.ID)
		}
	}
}

func autoHealNode(nodeID int) {
	fmt.Printf("Nó %d em estado crítico por >3s.\n", nodeID)

	// Dispara o Load Balancer Automático
	go func() {
		payloadLB := protocol.ControlMessage{
			Type:       "lb",
			Signal:     "trigger_on",
			TargetNode: nodeID,
			Requester:  "auto",
		}

		urlLB := os.Getenv("LB_SERVICE_URL")
		err := sendHTTPCommand(payloadLB, urlLB)
		if err != nil {
			fmt.Printf("Erro: Nó %d: %v\n", nodeID, err)
		} else {
			fmt.Printf("Comando entregue ao Nó %d\n", nodeID)
		}
	}()

	// Dispara o HVAC Automático
	go func() {
		payloadHVAC := protocol.ControlMessage{
			Type:       "hvac",
			Signal:     "trigger_on",
			TargetNode: nodeID,
			Requester:  "auto",
		}

		urlHVAC := os.Getenv("HVAC_SERVICE_URL")
		err := sendHTTPCommand(payloadHVAC, urlHVAC)
		if err != nil {
			fmt.Printf("ERRO: Nó %d: %v\n", nodeID, err)
		} else {
			fmt.Printf("Comando entregue ao Nó %d\n", nodeID)
		}
	}()
}

func sendHTTPCommand(payload protocol.ControlMessage, targetURL string) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao serializar json: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("erro ao criar requisicao http: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Timeout curto impede que uma falha de rede trave as goroutines do Servidor Central
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("falha na conexao com atuador: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("atuador retornou status: %d", resp.StatusCode)
	}

	return nil
}
