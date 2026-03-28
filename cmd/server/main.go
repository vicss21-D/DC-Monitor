package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"

	"dc-monitor/pkg/protocol"
)

const (
	Port        = ":9000"
	WorkerCount = 10    
	BufferSize  = 10000 
)

func main() {

	addr, err := net.ResolveUDPAddr("udp", Port)
	if err != nil {
		log.Fatalf("Erro ao resolver endereço: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor UDP: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Servidor Central Iniciado na porta %s\n", Port)
	fmt.Printf("Iniciando pool com %d Workers...\n", WorkerCount)

	packetQueue := make(chan []byte, BufferSize)

	var wg sync.WaitGroup

	for i := 1; i <= WorkerCount; i++ {
		wg.Add(1)
		go worker(i, packetQueue, &wg) 
	}

	networkBuffer := make([]byte, 2048)

	fmt.Println("Aguardando telemetria dos nós...")

	for {

		n, _, err := conn.ReadFromUDP(networkBuffer)
		if err != nil {
			log.Printf("Erro de leitura UDP: %v", err)
			continue
		}

		dataCopy := make([]byte, n)
		copy(dataCopy, networkBuffer[:n])

		packetQueue <- dataCopy
	}
}

func worker(id int, queue <-chan []byte, wg *sync.WaitGroup) {
	defer wg.Done() 

	for rawData := range queue {
		var packet protocol.TelemetryPacket

		err := json.Unmarshal(rawData, &packet)
		if err != nil {
			log.Printf("Worker %d falhou ao decodificar JSON: %v", id, err)
			continue
		}

		// ==========================================
		// LB
		// ==========================================
		
		if packet.CurrentState == protocol.StateCriticalLoad {
			fmt.Printf("[ALERTA CRÍTICO - Tratado pelo Worker %d] Nó %d atingiu %.2f°C | Stress: %.2f%%\n",
				id, packet.ID, packet.Temperature, packet.Stress)
			
		} else if packet.CurrentState == protocol.StateHighLoad {
			//fmt.Printf("[AVISO] Nó %d em Estado de Alta Carga.\n", packet.ID)
		}
	}
}