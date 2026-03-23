package main

import (
	"fmt" // remove after client interface (maybe)
	"sync"
	"log"
	"time"

	"dc-monitor/internal/network"
	"dc-monitor/pkg/protocol"
	"dc-monitor/internal"
)

func main() {

	n := 10 
	fmt.Printf("Iniciando... Sensores: %d\n", n)

	client, err := network.NewTelemetryClient("127.0.0.1:9000") // might change address later, also consider use a DNS
	if err != nil {
		log.Fatalf("Falha de rede: %v", err)
	}
	defer client.CloseClient()

	var wg sync.WaitGroup

	for i := 1; i <= n; i++ {
		wg.Add(1)

		go func(nodeID int) {
			defer wg.Done()

			node := node.NewNode(nodeID)
			
			ticker := time.NewTicker(1 * time.Millisecond)
			defer ticker.Stop()

			for range ticker.C {

				node.Tick()

				packet := protocol.TelemetryPacket {
					ID:       			node.ID,
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
		} (i)
	}
	
	wg.Wait()
}