package network

import (
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