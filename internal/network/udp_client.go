package network

import (
	"dc-monitor/pkg/protocol"
	"encoding/json"
	"fmt"
	"net"
)

type TelemetryClient struct {
	conn *net.UDPConn
}

func NewTelemetryClient(serverAddr string) (*TelemetryClient, error) {

	addr, err := net.ResolveUDPAddr("udp", serverAddr)

	if err != nil {
		return nil, fmt.Errorf("Erro na conversão de endereço UDP: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)

	if err != nil {
		return nil, fmt.Errorf("Erro ao criar o socket: %w", err)
	}

	return &TelemetryClient{
		conn: conn,
	}, nil
}

func (c *TelemetryClient) Send(packet protocol.TelemetryPacket) error {

	payload, err := json.Marshal(packet)

	if err != nil {
		return fmt.Errorf("Erro ao criar o pacote: %w", err)
	}

	_, err = c.conn.Write(payload)

	if err != nil {
		return fmt.Errorf("Erro ao enviar pacote UDP %w", err)
	}

	return nil
}

func (c *TelemetryClient) CloseClient() {
	if c.conn != nil {
		c.conn.Close()
	}
}