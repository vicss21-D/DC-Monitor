package network

import (
	"dc-monitor/pkg/protocol"
	"encoding/json"
	"fmt"
	"net"
)

// TelemetryClient encapsula a conexão UDP para enviar pacotes de telemetria ao servidor central
type TelemetryClient struct {
	// conn armazena a conexão UDP aberta para o servidor
	conn *net.UDPConn
}

// NewTelemetryClient cria uma nova instância do cliente e estabelece conexão UDP com o servidor
// Retorna error se não conseguir resolver o endereço ou conectar ao servidor
func NewTelemetryClient(serverAddr string) (*TelemetryClient, error) {

	// Resolve o endereço UDP do servidor (parse de host:port)
	addr, err := net.ResolveUDPAddr("udp", serverAddr)

	if err != nil {
		return nil, fmt.Errorf("Erro na conversão de endereço UDP: %w", err)
	}

	// Estabelece a conexão UDP com o servidor remoto
	conn, err := net.DialUDP("udp", nil, addr)

	if err != nil {
		return nil, fmt.Errorf("Erro ao criar o socket: %w", err)
	}

	return &TelemetryClient{
		conn: conn,
	}, nil
}

// Send envia um pacote de telemetria para o servidor central em UDP
// O pacote é convertido para JSON antes do envio
func (c *TelemetryClient) Send(packet protocol.TelemetryPacket) error {

	// Serializa o pacote para formato JSON
	payload, err := json.Marshal(packet)

	if err != nil {
		return fmt.Errorf("Erro ao criar o pacote: %w", err)
	}

	// Envia o pacote JSON pela conexão UDP
	_, err = c.conn.Write(payload)

	if err != nil {
		return fmt.Errorf("Erro ao enviar pacote UDP %w", err)
	}

	return nil
}

// CloseClient fecha a conexão UDP com o servidor
// Deve ser chamado quando o cliente não for mais necessário
func (c *TelemetryClient) CloseClient() {
	if c.conn != nil {
		c.conn.Close()
	}
}
