package processor

import (
	//"math/rand"
	//"time"
	//"pkg/protocol"
	//"fmt"
)

type NodeSystemSensor struct {
	ID          		int

	CurrentTemp 		float64
	CurrentStress     	float64
	CurrentPower 		float64
	CurrentLatency   	float64

	InputThroughput 	float64
	InputInterrupt 		float64
	HVACCoolingLevel 	float64

	BasePower 			float64
	MaxPower			float64
	ThermalMass			float64
	AmbientTemp			float64
	NetworkCap			float64
	BaseLatency			float64
}