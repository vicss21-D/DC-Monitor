package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	"net"
	"encoding/json"

	"dc-monitor/internal/network"
	"dc-monitor/pkg/protocol"
	"dc-monitor/internal"
)

type ControlMessage struct {
	Type       string `json:"type"`        // "hvac" ou "lb"
	Signal     string `json:"signal"`      // "trigger_on" ou "trigger_off"
	TargetNode int    `json:"target_node"` // ID do nó alvo
}

func main() {
	
	nodeIDStr := os.Getenv("NODE_ID")
	nodeID, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		log.Fatalf("Erro: Variável de ambiente NODE_ID inválida ou não definida.")
	}

	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		// Fallback
		serverAddr = "127.0.0.1:9000" 
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

		packet := protocol.TelemetryPacket{
			ID:           		node.ID,
			Timestamp:        	time.Now().UnixMilli(),
			TickCount:        	node.TickCount,
			CurrentState:     	node.State,
			Stress:        		node.CurrentStress,
			InputThroughput: 	node.InputThroughput,
			InputInterrupts:    node.InputInterrupts,
			Latency:          	node.CurrentLatency,
			Power:        		node.CurrentPower,
			Temperature:      	node.CurrentTemp,
		}

		client.Send(packet)
	}
}

func startActuatorListener(node *node.NodeSystemSensor, tcpPort string) {
	listener, err := net.Listen("tcp", ":"+tcpPort)
	if err != nil {
		fmt.Printf("❌ Nó %d: Falha ao abrir porta TCP %s - %v\n", node.ID, tcpPort, err)
		return
	}
	defer listener.Close()

	fmt.Printf("📡 Nó %d escutando comandos do Atuador na porta TCP %s\n", node.ID, tcpPort)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		// Processa o comando em paralelo para não travar o listener
		go handleActuatorCommand(conn, node)
	}
}

// handleActuatorCommand decodifica a ordem e altera a struct atômica
func handleActuatorCommand(conn net.Conn, node *node.NodeSystemSensor) {
	defer conn.Close()

	var msg protocol.ControlMessage
	if err := json.NewDecoder(conn).Decode(&msg); err != nil {
		return
	}

	// 1. Roteamento do Comando HVAC
	if msg.Type == "hvac" {
		if msg.Signal == "trigger_on" {
			// Liga a refrigeração máxima
			node.HVACState.Store(int64(protocol.StateMaximum))
		
		} else if msg.Signal == "trigger_off" {
			// Retorna à refrigeração padrão
			node.HVACState.Store(int64(protocol.StateBalanced))

		}
	
	// 2. Roteamento do Comando Load Balancer
	} else if msg.Type == "lb" {
		if msg.Signal == "trigger_on" {
			node.LBActive.Store(true)

		} else {
			node.LBActive.Store(false)
		}
	}
}