package logs

import (
	"fmt"
	"os"

	"dc-monitor/pkg/protocol"
)

func CSVLoggerWorker(logChannel <-chan protocol.TelemetryPacket) {

	file, err := os.OpenFile("logs.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível abrir o arquivo de log: %v\n", err)
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	if stat.Size() == 0 {
		file.WriteString("node_id,current_state,temperature,stress,power_draw,latency,tick_count,hvac_state,lb_active\n")
	}

	for packet := range logChannel {

		csvLine := fmt.Sprintf("%d,%d,%.2f,%.2f,%.2f,%.2f,%d,%d,%t\n",
			packet.ID,
			packet.CurrentState,
			packet.Temperature,
			packet.Stress,
			packet.Power,
			packet.Latency,
			packet.TickCount,
			packet.HVACState,
			packet.LBActive,
		)

		_, err := file.WriteString(csvLine)
		if err != nil {
			fmt.Printf("[ERRO] Falha ao gravar no CSV: %v\n", err)
		}
	}
}