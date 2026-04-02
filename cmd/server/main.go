package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"dc-monitor/pkg/protocol"
	"github.com/gorilla/websocket"
)

const (
	UDPPort     = ":9000"
	HTTPPort    = ":8080"
	WorkerCount = 16
	BufferSize  = 10000
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

	packetQueue := make(chan []byte, BufferSize)
	var wg sync.WaitGroup

	for i := 1; i <= WorkerCount; i++ {
		wg.Add(1)
		go worker(i, packetQueue, &wg)
	}

	// ==========================================
	// 2. SETUP DAS ROTAS WEB
	// ==========================================

	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/api/control", handleClientControl)
	http.Handle("/", http.FileServer(http.Dir("./client")))

	go func() {
		fmt.Println("🌐 Servidor Web e Interface Gráfica na porta 8080...")
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
	fmt.Println("📡 Servidor Central escutando telemetria UDP na porta 9000...")

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
	
	payload, _ := json.Marshal(msg)

	var actuatorURL string
	if msg.Type == "hvac" {
		actuatorURL = "http://hvac_service:8081/trigger"
	} else if msg.Type == "lb" {
		actuatorURL = "http://lb_service:8082/trigger"
	} else {
		http.Error(w, "Atuador desconhecido", http.StatusBadRequest)
		return
	}

	// 3. Comunica com o atuador
	resp, err := http.Post(actuatorURL, "application/json", bytes.NewBuffer(payload))
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Servidor falhou ao contatar o Atuador %s\n", msg.Type)
		http.Error(w, "Falha de comunicação com o atuador", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("📡 Servidor roteou comando de %s para o Nó %d\n", msg.Type, msg.TargetNode)
	w.WriteHeader(http.StatusOK)
}

func worker(id int, queue <-chan []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	
	for rawData := range queue {

		var header protocol.TelemetryPacket
		
		if err := json.Unmarshal(rawData, &header); err != nil {
			continue
		}

		// REGRA 1
		isCritical := header.CurrentState == 2 

		// REGRA 2
		isTickInterval := header.TickCount % TicksPerSecond == 0

		if isCritical || isTickInterval {
			broadcast <- rawData
		}
	}
}