package protocol

type TelemetryPacket struct {

	ID					int 		`json:"node_id"`
	Timestamp			int64		`json:"timestamp"`

	CurrentState		NodeState	`json:"current_state"`

	Temperature 		float64		`json:"temperature"`
	Stress				float64		`json:"stress"`
	Power				float64		`json:"power_draw"`
	Latency				float64		`json:"latency"`

	InputThroughput 	float64		`json:"throughput"`
	InputInterrupts 	float64		`json:"interrupts"`
	HVACCoolingLevel 	float64		`json:"cooling_level"`
}