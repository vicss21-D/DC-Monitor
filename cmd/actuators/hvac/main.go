package main

import (
	//"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"dc-monitor/pkg/protocol" 
)

func main() {

	http.HandleFunc("/trigger", handleTrigger)

	fmt.Println("❄️ Atuador HVAC independente iniciado na porta 8081...")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleTrigger(w http.ResponseWriter, r *http.Request) {
	
	var msg protocol.ControlMessage

	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	tcpPort := os.Getenv("NODE_TCP_PORT")
	
	if tcpPort == "" {
		tcpPort = "9001"
	}

	targetAddr := fmt.Sprintf("sensor_%d:%s", msg.TargetNode, tcpPort)

	conn, err := net.DialTimeout("tcp", targetAddr, 2*time.Second)

	if err != nil {
		fmt.Printf("Atuador HVAC falhou ao conectar no Nó %d: %v\n", msg.TargetNode, err)
		http.Error(w, "Falha na atuação física", http.StatusServiceUnavailable)
		return
	}

	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(&msg); err != nil {
		fmt.Printf("Atuador HVAC falhou ao enviar sinal para o Nó %d\n", msg.TargetNode)
		return
	}

	fmt.Printf("Atuador HVAC aplicou sinal [%s] no Nó %d\n", msg.Signal, msg.TargetNode)
	w.WriteHeader(http.StatusOK)
}