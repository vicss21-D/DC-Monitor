package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"edge-nodes/internal"
	"edge-nodes/internal/network"
	"edge-nodes/pkg/protocol"
)

func main() {

	nodeIDStr := os.Getenv("NODE_ID")
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		log.Fatalf("Erro: Variável de ambiente NODE_ID inválida ou não definida.")
	}

	serverAddr := os.Getenv("GATEWAY_IP")+":9000"
	if serverAddr == "" {
		// Fallback
		serverAddr = "dc_server:9000"
	}

	fmt.Printf("Iniciando (Nó %d). Alvo: %s\n", nodeID, serverAddr)

	client, err := network.NewTelemetryClient(serverAddr)

	if err != nil {
		log.Fatalf("Falha crítica de rede no Nó %d: %v", nodeID, err)
	}

	defer client.CloseClient()

	node := node.NewNode(nodeID)

	go startActuatorListener(node, "9001")

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		node.Tick()

		// Monta pacote de telemetria com as métricas atuais do nó
		// Lê valores simultâneos de simulação e estados dos atuadores via atomic loads
		// Timestamp em milissegundos para sincronização no servidor central
		packet := protocol.TelemetryPacket{
			ID:              node.ID,
			Timestamp:       time.Now().UnixMilli(),
			TickCount:       node.TickCount,
			CurrentState:    node.State,
			Stress:          node.CurrentStress,
			InputThroughput: node.InputThroughput,
			InputInterrupts: node.InputInterrupts,
			Latency:         node.CurrentLatency,
			Power:           node.CurrentPower,
			Temperature:     node.CurrentTemp,
			HVACState:       node.HVACState.Load(),
			LBActive:        node.LBActive.Load(),
		}

		client.Send(packet)
	}
}

func startActuatorListener(node *node.NodeSystemSensor, tcpPort string) {
	// Cria listener TCP na porta especificada
	listener, err := net.Listen("tcp", ":"+tcpPort)
	if err != nil {
		fmt.Printf("Nó %d: Falha ao abrir porta TCP %s - %v\n", node.ID, tcpPort, err)
		return
	}
	defer listener.Close()

	// Listener ativo aguardando conexões de atuadores
	fmt.Printf("Nó %d escutando comandos do Atuador na porta TCP %s\n", node.ID, tcpPort)

	for {
		// Bloqueia até uma conexão TCP chegar
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		// Processa cada comando em goroutine separada (não bloqueia listener)
		go handleActuatorCommand(conn, node)
	}
}

// handleActuatorCommand decodifica e processa mensagens de controle dos atuadores
// Atualiza atomicamente o estado do nó (HVAC ou Load Balancer) para thread-safety
func handleActuatorCommand(conn net.Conn, node *node.NodeSystemSensor) {
	defer conn.Close()

	// Decodifica comando JSON do atuador
	var msg protocol.ControlMessage
	if err := json.NewDecoder(conn).Decode(&msg); err != nil {
		return
	}

	// ========== 1. ROTEAMENTO DO COMANDO HVAC ==========
	if msg.Type == "hvac" {
		if msg.Requester == "auto" {
			// Ação Automática: Resfriamento forçado temporário
			fmt.Printf("[Nó %d]: HVAC máximo por 2s.\n", node.ID)
			node.HVACState.Store(int64(protocol.StateMaximum))
			time.Sleep(2 * time.Second)
			node.HVACState.Store(int64(protocol.StateBalanced))
			fmt.Printf("[Nó %d]: HVAC normalizado.\n", node.ID)

		} else {
			// Novas rotas baseadas no controle segmentado
			if msg.Signal == "set_max" {
				node.HVACState.Store(int64(protocol.StateMaximum))
			} else if msg.Signal == "set_balanced" {
				node.HVACState.Store(int64(protocol.StateBalanced))
			} else if msg.Signal == "set_off" {
				node.HVACState.Store(int64(protocol.StateOff))
			}
		}

		// ==========================================
		// 2. ATUADOR LOAD BALANCER
		// ==========================================
	} else if msg.Type == "lb" {
		if msg.Requester == "auto" {
			// Ação Automática: Drenagem temporária
			fmt.Printf("[Nó %d] Drenado tráfego por 2s.\n", node.ID)
			node.LBActive.Store(true)
			time.Sleep(2 * time.Second)
			node.LBActive.Store(false)
			fmt.Printf("[Nó %d] Tráfego restaurado.\n", node.ID)

		} else {
			// Ação Manual: Drenagem manual com o seu limite nativo de 2s
			if msg.Signal == "trigger_on" {
				node.LBActive.Store(true)
				time.Sleep(2 * time.Second)
				node.LBActive.Store(false)
			} else {
				node.LBActive.Store(false)
			}
		}
	}
}
