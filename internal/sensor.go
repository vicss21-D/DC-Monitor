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
	InputInterrupt 		int
	HVACCoolingLevel 	float64

	BasePower 			float64
	MaxPower			float64
	ThermalMass			float64
	AmbientTemp			float64
	NetworkCap			float64
	BaseLatency			float64
}

func NewNode(id int) *NodeSystemSensor{
	return &NodeSystemSensor{
		ID: 				id,
		
		CurrentTemp: 		22.0,
		CurrentStress:     	0.0,
		CurrentPower: 		250.0,
		CurrentLatency:   	5.0,

		InputThroughput: 	0.2,
		InputInterrupt: 	5000,	
		HVACCoolingLevel:	0.3,

		BasePower: 			150.0,
		MaxPower:			650.0,
		ThermalMass:		50.0,	
		AmbientTemp:		20.0,	
		NetworkCap:			9.0,
		BaseLatency:		2.0,	
	}
}