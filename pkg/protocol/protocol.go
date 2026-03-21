package protocol

type TelemetryPacket struct {

	ID					int
	Timestamp			int64

	CurrentState		NodeState

	Temperature 		float64
	Stress				float64
	Power				float64
	Latency				float64

	InputThroughput 	float64
	InputInterrupts 	float64
	HVACCoolingLevel 	float64
}