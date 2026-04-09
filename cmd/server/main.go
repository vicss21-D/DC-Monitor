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
// ARQUITETURA:
// 1. Watchdog: Monitora último contato de cada nó para detectar desconexões
// 2. Timers Críticos: Rastreia quanto tempo um nó está em estado crítico
// 3. Quadro de Estado: Cache do pacote mais recente de cada nó (para batching)
// 4. Motor Duplo: Dois canais com prioridades diferentes para transmissão ao frontend

// Watchdog: Array atômico para armazenar o Unix Timestamp do último contato
// lastSeen[nodeID] = timestamp do último pacote recebido
var lastSeen [9]int64

// Timers Críticos: Array atômico com lógica de máquina de estados
// 0 = Estado normal (não há timer ativo)
// >0 = Timestamp quando o nó entrou em estado crítico
// -1 = Auto-heal já foi disparado para este nó
var criticalStart [9]int64

// Quadro de Estado Atual: Cache em memória do último TelemetryPacket de cada nó
// Usado para batching 1Hz: coleta-se todos os "frames" atuais e envia em lote
var latestPackets [9]protocol.TelemetryPacket
var latestPacketsMutex sync.RWMutex

// ==========================================
// CANAIS DE SAÍDA (MOTOR DUPLO COM 2 PRIORIDADES)
// ==========================================
// DESIGN:
// - VIA CRÍTICA: Garante entrega de eventos urgentes (estados críticos)
// - VIA NORMAL: Streaming contínuo com descard de pacotes antigos (ring buffer)

// Via VIP (Crítica): Fila tradicional FIFO com 50 slots para alertas urgentes
// Se lotado, os alertas mais recentes são DESCARTADOS (não se acumula backlog)
var broadcastCritical = make(chan interface{}, 50)

// Via Normal: Janela Deslizante (Ring Buffer) com espaço para 10 "frames"
// Se o cliente estiver lento, os dados antigos são descartados (não bloqueia)
// Ideal para múltiplos clients heterogêneos (alguns rápidos, outros lentos)
var broadcastNormal = RingChannel(10)

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
var clientsMutex sync.Mutex // Protege escritas concorrentes no mapa de clientes (múltiplas goroutines de broadcast)

// main é a função principal do servidor central de monitoramento do data center
// Arquitetura:
// 1. Listener UDP: Recebe pacotes de telemetria dos 8 nós em paralelo
// 2. Worker Pool: 16 goroutines processam telemetria em paralelo (CPU-bound parsing)
// 3. Broadcaster: 1Hz tick que envia "frames" em lote para todos os cliente WebSocket
// 4. Auto-Heal: Dispara atuadores quando nós entram em estado crítico por >5s
func main() {
	// Resolve e vincula o endereço UDP (0.0.0.0:9000) para receber de qualquer interface
	addr, _ := net.ResolveUDPAddr("udp", UDPPort)
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	// Canal de log para salvar telemetria em CSV (capacidade para 10k pacotes em buffer)
	logChannel := make(chan protocol.TelemetryPacket, 10000)
	// Inicia goroutine de logging que escreve em logs.csv continuamente
	go logs.CSVLoggerWorker(logChannel)

	// Fila de pacotes UDP brutos para processamento em paralelo
	packetQueue := make(chan []byte, BufferSize)
	var wg sync.WaitGroup

	// Inicia um pool de 16 workers que processam pacotes UDP em paralelo
	// Cada worker desserializa JSON, atualiza cache e lógica de auto-heal
	for i := 1; i <= WorkerCount; i++ {
		wg.Add(1)
		go worker(i, packetQueue, &wg, logChannel)
	}

	// ==========================================
	// SETUP DAS ROTAS WEB E WEBSOCKET
	// ==========================================
	http.HandleFunc("/ws", handleConnections)            // WebSocket para frontend (streaming)
	http.HandleFunc("/api/control", handleClientControl) // HTTP POST para enviar comandos

	// Servidor HTTP roda em goroutine background (escuta requisições da web)
	go func() {
		fmt.Println("Servidor HTTP na porta 8080...")
		if err := http.ListenAndServe(HTTPPort, nil); err != nil {
			log.Fatal("Erro fatal no servidor HTTP:", err)
		}
	}()

	// Inicia as goroutines de transmissão
	// handleMessages: Lê dos 2 canais (critico + normal) e envia para todos WebSocket
	// telemetryBroadcaster: Tick a cada 1s, agrega pacotes e os empurra para os canais
	go handleMessages()
	go telemetryBroadcaster()

	// ==========================================
	// LOOP PRINCIPAL (UDP) - Recebimento em Assincronismo
	// ==========================================
	// Dedica uma goroutine apenas para ler UDP frames e colocar na fila
	// Não faz parsing aqui (evita congestão), apenas copia bytes
	go func() {
		networkBuffer := make([]byte, 2048) // MTU-sized buffer
		fmt.Println("Servidor Central escutando telemetria UDP na porta 9000...")
		for {
			// Bloqueia até receber um pacote UDP (0.0.0.0:9000)
			n, _, err := conn.ReadFromUDP(networkBuffer)
			if err != nil {
				break // Sai do loop se a conexão for fechada (caso desligamento)
			}
			// Cria uma cópia do buffer para não sobrescrever na próxima iteração
			dataCopy := make([]byte, n)
			copy(dataCopy, networkBuffer[:n])
			// Enfileira para processamento paralelo (não bloqueia por muito tempo)
			packetQueue <- dataCopy
		}
	}()

	// ==========================================
	// GRACEFUL SHUTDOWN (DESLIGAMENTO SEGURO)
	// ==========================================
	// Canal para receber sinais do SO (Ctrl+C, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Bloqueia até receber um sinal de desligamento
	<-quit
	fmt.Println("\n[SHUTDOWN] Sinal recebido. Iniciando desligamento seguro...")

	conn.Close()       // Para de receber pacotes UDP novos
	close(packetQueue) // Sinaliza aos workers para terminarem após processar fila
	wg.Wait()          // Aguarda todos os 16 workers finalizarem suas tarefas

	fmt.Println("[SHUTDOWN] Servidor desligado com segurança.")
}

// ---------------------------------------------------------
// WORKERS E PROCESSAMENTO DE TELEMETRIA
// ---------------------------------------------------------
// worker processa pacotes UDP em paralelo (16 instâncias)
// Responsabilidades:
// 1. Desserializar JSON em TelemetryPacket
// 2. Atualizar lastSeen para detecção de nó offline
// 3. Atualizar latestPackets cache (para batching 1Hz)
// 4. Lógica de Auto-Heal: Se nó crítico por >5s, dispara atuadores
// 5. Encaminhar para logging em CSV
func worker(id int, queue <-chan []byte, wg *sync.WaitGroup, logChannel chan<- protocol.TelemetryPacket) {
	defer wg.Done()

	// Processa cada pacote da fila até ela ser fechada (indicador de shutdown)
	for rawData := range queue {
		// Desserializa JSON para struct TelemetryPacket
		var packet protocol.TelemetryPacket
		if err := json.Unmarshal(rawData, &packet); err != nil {
			continue // Ignora pacotes mal formados
		}

		if packet.ID < 1 || packet.ID > 8 {
			continue // Proteção: descarta pacotes com ID fora do range [1-8]
		}

		now := time.Now().Unix()

		// ========== 1. WATCHDOG ATÔMICO ==========
		// Marca timestamp do último contato para cada nó (detecção de desconexão)
		atomic.StoreInt64(&lastSeen[packet.ID], now)

		// ========== 2. QUADRO DE ESTADO (CACHE PARA BATCHING) ==========
		// Armazena o pacote mais recente em latestPackets para ser coletado a cada 1s
		latestPacketsMutex.Lock()
		latestPackets[packet.ID] = packet
		latestPacketsMutex.Unlock()

		// ========== 3. LÓGICA DE ESTADO CRÍTICO E AUTO-HEAL ==========
		// Se nó estiver em estado crítico (value 2), envia para Via Crítica (VIP channel)
		if packet.CurrentState == 2 {
			// QoS: Via VIP garante entrega ao frontend com alta prioridade
			envelope := map[string]interface{}{
				"type":    "critical",                         // Sinaliza ao frontend: vermelho, pisca!
				"payload": []protocol.TelemetryPacket{packet}, // Dados do nó crítico
			}

			// Tenta enfileirar no canal crítico (se lotado, descarta com silent fail)
			select {
			case broadcastCritical <- envelope:
			default:
				log.Println("[AVISO] Canal Crítico lotado! Alerta descartado para salvar CPU.")
			}

			// Cronômetro Atômico sem Mutex (3 possíveis estados):
			// 0 = Nó está normal, timer não ativo
			// >0 = Timestamp de início do estado crítico
			// -1 = Auto-heal já foi disparado para este nó
			start := atomic.LoadInt64(&criticalStart[packet.ID])
			if start == 0 {
				// Primeira detecção de criticidade: inicia cronômetro
				atomic.StoreInt64(&criticalStart[packet.ID], now)
			} else if start > 0 && now-start >= 5 {
				// Nó está crítico há >=5 segundos: dispara auto-heal
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
			// Empurra para a Janela Deslizante (Se a rede estiver lenta, descarta o velho)
			broadcastNormal.Push(envelope)
		}
	}
}

// ---------------------------------------------------------
// FUNÇÕES DE REDE E WEBSOCKET
// ---------------------------------------------------------

// handleConnections aceita upgrade de conexão HTTP para WebSocket
// Registra o cliente no mapa global (clients) para posterior broadcast de dados
// Mantém conexão aberta lendo mensagens até desconexão ou erro
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

// handleMessages orquestra o roteamento de mensagens dos dois canais para os clientes
// PRIORIDADE 1 (Via Crítica): Mensagens de nós em estado crítico (emergência)
// PRIORIDADE 2 (Via Normal): Batch de telemetria (1s) dos nós operacionais
// Implementa double-select para garantir que mensagens críticas não fiquem retidas
func handleMessages() {
	normalOut := broadcastNormal.Out()

	for {
		select {
		// PRIORIDADE 1: Via Crítica
		case msg := <-broadcastCritical:
			sendToAllClients(msg)

		default:
			// PRIORIDADE 2: Lê o que estiver disponível
			select {
			case msg := <-broadcastCritical:
				sendToAllClients(msg)
			case msg := <-normalOut:
				sendToAllClients(msg)
			}
		}
	}
}

// sendToAllClients transmite uma mensagem JSON para todos os clientes WebSocket conectados
// Remove automaticamente clientes que apresentarem erro de conexão
func sendToAllClients(msg interface{}) {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return
	}

	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}

// ---------------------------------------------------------
// COMANDOS DE ATUADOR E RETRY
// ---------------------------------------------------------

// handleClientControl processa comandos manuais enviados pelo frontend (Dashboard)
// Valida JSON, roteia para o serviço de atuador apropriado (HVAC ou LB) via HTTP
// Retorna erro 400 se JSON inválido ou 500 se comunicação com atuador falhar
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

// autoHealNode dispara sequência de recuperação para nó em estado crítico
// Envia 2 comandos paralelos com retry=3: Load Balancer + HVAC em nível máximo
// Cada comando roda em goroutine separada para não bloquear processamento
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

// sendCommandWithRetry encapsula lógica de retry com backoff para garantir entrega de comando
// Realiza até maxRetries tentativas com espera de 1s entre falhas
// Retorna erro após esgotar tentativas
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

// sendHTTPCommand realiza POST HTTP com timeout de 2 segundos para enviar comando a atuador
// Serializando ControlMessage em JSON e validando status de resposta (200 ou 202)
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
