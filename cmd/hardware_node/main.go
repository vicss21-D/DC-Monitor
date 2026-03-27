package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"dc-monitor/internal/network"
	"dc-monitor/internal"
	"dc-monitor/pkg/protocol"
)

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

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		node.Tick()

		packet := protocol.TelemetryPacket{
			ID:           		node.ID,
			Timestamp:        	time.Now().UnixMilli(),
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