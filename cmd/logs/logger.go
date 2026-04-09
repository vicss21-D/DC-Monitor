package logs

import (
	"fmt"
	"os"

	"dc-monitor/pkg/protocol"
)

// CSVLoggerWorker escuta um canal de telemetria e registra cada pacote em um arquivo CSV
// Esta função roda em uma goroutine separada para não bloquear o processamento principal
func CSVLoggerWorker(logChannel <-chan protocol.TelemetryPacket) {

	// Abre o arquivo de log em modo append (adiciona sem sobrescrever dados existentes)
	file, err := os.OpenFile("logs.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível abrir o arquivo de log: %v\n", err)
		return
	}
	defer file.Close()

	// Verifica se o arquivo está vazio; se sim, escreve o cabeçalho CSV
	stat, _ := file.Stat()
	if stat.Size() == 0 {
		file.WriteString("node_id,timestamp,current_state,temperature,stress,power_draw,latency,tick_count,hvac_state,lb_active\n")
	}

	// Loop infinito que processa cada pacote de telemetria recebido no canal
	for packet := range logChannel {

		// Formata o pacote como uma linha CSV com 10 colunas
		csvLine := fmt.Sprintf("%d,%d,%d,%.2f,%.2f,%.2f,%.2f,%d,%d,%t\n",
			packet.ID,
			packet.Timestamp,
			packet.CurrentState,
			packet.Temperature,
			packet.Stress,
			packet.Power,
			packet.Latency,
			packet.TickCount,
			packet.HVACState,
			packet.LBActive,
		)

		// Escreve a linha no arquivo
		_, err := file.WriteString(csvLine)
		if err != nil {
			fmt.Printf("[ERRO] Falha ao gravar no CSV: %v\n", err)
		}
	}
}
