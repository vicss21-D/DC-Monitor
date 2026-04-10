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

	"edge-nodes/pkg/protocol"
)

// main inicia o serviço de atuador Load Balancer (LB)
// Este atuador escuta na porta 8082 e gerencia o balanceamento de carga/drenagem de tráfego
func main() {

	// Registra a rota HTTP que recebe comandos de controle
	http.HandleFunc("/trigger", handleTrigger)

	fmt.Println("❄️ Atuador Load Balancer iniciado na porta 8082...")
	// Bloqueia e aguarda requisições HTTP na porta 8082
	log.Fatal(http.ListenAndServe(":8082", nil))
}

// handleTrigger processa requisições HTTP POST com comandos de Load Balancer
// Decodifica o JSON, valida e envia a mensagem para o nó sensor alvo via TCP
func handleTrigger(w http.ResponseWriter, r *http.Request) {

	// Decodifica o corpo JSON da requisição em uma estrutura ControlMessage
	var msg protocol.ControlMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	// Obtém a porta TCP do nó sensor a partir da variável de ambiente NODE_TCP_PORT
	tcpPort := os.Getenv("NODE_TCP_PORT")
	// Se a variável não estiver definida, usa a porta default 9001
	if tcpPort == "" {
		tcpPort = "9001"
	}

	// Monta o endereço TCP do nó sensor alvo (Ex: sensor_1:9001)
	targetAddr := fmt.Sprintf("sensor_%d:%s", msg.TargetNode, tcpPort)

	// Estabelece conexão TCP com timeout de 2 segundos para garantir responsividade
	conn, err := net.DialTimeout("tcp", targetAddr, 2*time.Second)

	if err != nil {
		fmt.Printf("Atuador Load Balancer falhou ao conectar no Nó %d: %v\n", msg.TargetNode, err)
		http.Error(w, "Falha na atuação física", http.StatusServiceUnavailable)
		return
	}

	defer conn.Close()

	// Envia a mensagem de controle em formato JSON para o nó sensor
	if err := json.NewEncoder(conn).Encode(&msg); err != nil {
		fmt.Printf("Atuador Load Balancer falhou ao enviar sinal para o Nó %d\n", msg.TargetNode)
		return
	}

	// Log de sucesso da ação
	fmt.Printf("Atuador Load Balancer aplicou sinal [%s] no Nó %d\n", msg.Signal, msg.TargetNode)
	// Retorna status OK ao cliente HTTP
	w.WriteHeader(http.StatusOK)
}
