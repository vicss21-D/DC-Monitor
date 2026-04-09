package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"dc-monitor/cmd/logs"
	"dc-monitor/pkg/protocol"

	"github.com/gorilla/websocket"
)

// ==========================================
// VARIÁVEIS GLOBAIS DE ALTA PERFORMANCE
// ==========================================

// Watchdog: Array atômico para armazenar o Unix Timestamp do último contato
var lastSeen [9]int64

// Timers Críticos: Array atômico (0 = Normal, >0 = Timestamp Crítico, -1 = Disparado)
var criticalStart [9]int64

// Quadro de Estado Atual: Armazena o pacote mais recente para o batching de 1s
var latestPackets [9]protocol.TelemetryPacket
var latestPacketsMutex sync.RWMutex

// Canal de Saída para o Front-end (Agora aceita interfaces para criar o Envelope JSON)
var broadcast = make(chan interface{}, 100)

const (
	UDPPort     = ":9000"
	HTTPPort    = ":8080"
	WorkerCount = 16
	BufferSize  = 10000
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var clientsMutex sync.Mutex // Protege o mapa de clientes no envio concorrente

func main() {
	addr, _ := net.ResolveUDPAddr("udp", UDPPort)
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	logChannel := make(chan protocol.TelemetryPacket, 10000)
	go logs.CSVLoggerWorker(logChannel)

	packetQueue := make(chan []byte, BufferSize)
	var wg sync.WaitGroup

	// Inicia os Workers e passa o WaitGroup
	for i := 1; i <= WorkerCount; i++ {
		wg.Add(1)
		go worker(i, packetQueue, &wg, logChannel)
	}

	// ==========================================
	// SETUP DAS ROTAS WEB
	// ==========================================
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/api/control", handleClientControl)
	//http.Handle("/", http.FileServer(http.Dir("./client")))

	go func() {
		fmt.Println("Servidor HTTP na porta 8080...")
		if err := http.ListenAndServe(HTTPPort, nil); err != nil {
			log.Fatal("Erro fatal no servidor HTTP:", err)
		}
	}()

	// Inicia as Goroutines de transmissão
	go handleMessages()
	go telemetryBroadcaster()

	// ==========================================
	// LOOP PRINCIPAL (UDP) ASSÍNCRONO
	// ==========================================
	go func() {
		networkBuffer := make([]byte, 2048)
		fmt.Println("Servidor Central escutando telemetria UDP na porta 9000...")
		for {
			n, _, err := conn.ReadFromUDP(networkBuffer)
			if err != nil {
				break // Sai do loop se a conexão for fechada
			}
			dataCopy := make([]byte, n)
			copy(dataCopy, networkBuffer[:n])
			packetQueue <- dataCopy
		}
	}()

	// ==========================================
	// GRACEFUL SHUTDOWN (DESLIGAMENTO SEGURO)
	// ==========================================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit // Trava a execução até receber o sinal de desligamento
	fmt.Println("\n[SHUTDOWN] Sinal recebido. Iniciando desligamento seguro...")

	conn.Close()       // Para de receber pacotes novos
	close(packetQueue) // Avisa os workers para esvaziarem a fila e saírem
	wg.Wait()          // Espera todos os 16 workers terminarem

	fmt.Println("[SHUTDOWN] Servidor desligado com segurança.")
}

// ---------------------------------------------------------
// WORKERS E PROCESSAMENTO
// ---------------------------------------------------------

func worker(id int, queue <-chan []byte, wg *sync.WaitGroup, logChannel chan<- protocol.TelemetryPacket) {
	defer wg.Done()

	// Quando close(packetQueue) for chamado na main, o loop termina naturalmente
	for rawData := range queue {
		var packet protocol.TelemetryPacket
		if err := json.Unmarshal(rawData, &packet); err != nil {
			continue
		}

		if packet.ID < 1 || packet.ID > 8 {
			continue // Proteção de limite do array
		}

		now := time.Now().Unix()

		// 1. WATCHDOG ATÔMICO
		atomic.StoreInt64(&lastSeen[packet.ID], now)

		// 2. QUADRO DE ESTADO (SWAP)
		latestPacketsMutex.Lock()
		latestPackets[packet.ID] = packet
		latestPacketsMutex.Unlock()

		// 3. LÓGICA DE ESTADO CRÍTICO E AUTO-HEAL
		if packet.CurrentState == 2 {
			// QoS: Dispara ao WebSocket imediatamente contornando o Batch
			envelope := map[string]interface{}{
				"type":    "critical",
				"payload": []protocol.TelemetryPacket{packet},
			}
			select {
			case broadcast <- envelope:
			default: // Backpressure
			}

			// Cronômetro Atômico sem Mutex
			start := atomic.LoadInt64(&criticalStart[packet.ID])
			if start == 0 {
				atomic.StoreInt64(&criticalStart[packet.ID], now)
			} else if start > 0 && now-start >= 5 {
				atomic.StoreInt64(&criticalStart[packet.ID], -1) // -1 = Já disparado
				go autoHealNode(packet.ID)
			}
		} else {
			// Se o estado voltar ao normal, reseta o cronômetro
			atomic.StoreInt64(&criticalStart[packet.ID], 0)
		}

		// Envia para log
		select {
		case logChannel <- packet:
		default:
		}
	}
}

// ---------------------------------------------------------
// BROADCASTER (BATCHING 1Hz) E WATCHDOG
// ---------------------------------------------------------

func telemetryBroadcaster() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()
		var batch []protocol.TelemetryPacket

		latestPacketsMutex.RLock()
		for i := 1; i <= 8; i++ {
			p := latestPackets[i]

			// Verifica se o nó caiu (mais de 5s sem resposta)
			lastContact := atomic.LoadInt64(&lastSeen[i])
			if lastContact == 0 || now-lastContact > 5 {
				p.ID = i
				p.CurrentState = -1 // Código de Offline para o Front-end
			}

			if p.ID != 0 {
				batch = append(batch, p)
			}
		}
		latestPacketsMutex.RUnlock()

		if len(batch) > 0 {
			envelope := map[string]interface{}{
				"type":    "batch",
				"payload": batch,
			}
			select {
			case broadcast <- envelope:
			default:
				// Backpressure: Cliente web lento, dropa o frame inteiro
			}
		}
	}
}

// ---------------------------------------------------------
// FUNÇÕES DE REDE E WEBSOCKET
// ---------------------------------------------------------

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			clientsMutex.Lock()
			delete(clients, ws)
			clientsMutex.Unlock()
			break
		}
	}
}

func handleMessages() {
	for msg := range broadcast {
		// Serializa o Envelope (Batch ou Critical)
		jsonData, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		clientsMutex.Lock()
		for client := range clients {
			if err := client.WriteMessage(websocket.TextMessage, jsonData); err != nil {
				client.Close()
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
	}
}

// ---------------------------------------------------------
// COMANDOS DE ATUADOR E RETRY
// ---------------------------------------------------------

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

	err := sendCommandWithRetry(msg, actuatorURL, 1) // 1 tentativa para comandos manuais
	if err != nil {
		fmt.Printf("[ERRO MANUAL] Falha ao contatar Atuador %s: %v\n", msg.Type, err)
		http.Error(w, "Falha de comunicação com o atuador", http.StatusInternalServerError)
		return
	}

	fmt.Printf("[MANUAL] Servidor roteou comando de %s para o Nó %d\n", msg.Type, msg.TargetNode)
	w.WriteHeader(http.StatusOK)
}

func autoHealNode(nodeID int) {
	fmt.Printf("⚠️ Nó %d em estado crítico por >5s. Iniciando Auto-Heal!\n", nodeID)

	go func() {
		payloadLB := protocol.ControlMessage{Type: "lb", Signal: "trigger_on", TargetNode: nodeID, Requester: "auto"}
		urlLB := os.Getenv("LB_SERVICE_URL")
		if err := sendCommandWithRetry(payloadLB, urlLB, 3); err != nil {
			fmt.Printf("ERRO FATAL LB (Nó %d): %v\n", nodeID, err)
		} else {
			fmt.Printf("Comando LB entregue ao Nó %d com sucesso!\n", nodeID)
		}
	}()

	go func() {
		payloadHVAC := protocol.ControlMessage{Type: "hvac", Signal: "trigger_on", TargetNode: nodeID, Requester: "auto"}
		urlHVAC := os.Getenv("HVAC_SERVICE_URL")
		if err := sendCommandWithRetry(payloadHVAC, urlHVAC, 3); err != nil {
			fmt.Printf("ERRO FATAL HVAC (Nó %d): %v\n", nodeID, err)
		} else {
			fmt.Printf("Comando HVAC entregue ao Nó %d com sucesso!\n", nodeID)
		}
	}()
}

// Wrapper para adicionar resiliência de rede aos disparos HTTP
func sendCommandWithRetry(payload protocol.ControlMessage, targetURL string, maxRetries int) error {
	var err error
	for i := 1; i <= maxRetries; i++ {
		err = sendHTTPCommand(payload, targetURL)
		if err == nil {
			return nil
		}
		if i < maxRetries {
			fmt.Printf("[FALHA DE REDE] Tentativa %d/%d falhou. Retentando em 1s...\n", i, maxRetries)
			time.Sleep(1 * time.Second)
		}
	}
	return fmt.Errorf("Desistindo após %d tentativas: %v", maxRetries, err)
}

func sendHTTPCommand(payload protocol.ControlMessage, targetURL string) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status HTTP: %d", resp.StatusCode)
	}
	return nil
}
