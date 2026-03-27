package node

import (
	"math/rand"
	"dc-monitor/pkg/protocol"
	//"fmt"
)

type NodeSystemSensor struct {
	ID          		int

	State				protocol.NodeState
	TickCount 			int

	CurrentTemp 		float64
	CurrentStress     	float64
	CurrentPower 		float64
	CurrentLatency   	float64

	InputThroughput 	float64
	InputInterrupts 	float64
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
		InputInterrupts: 	5000.0,	
		HVACCoolingLevel:	0.3,

		BasePower: 			150.0,
		MaxPower:			650.0,
		ThermalMass:		50.0,	
		AmbientTemp:		20.0,	
		NetworkCap:			9.0,
		BaseLatency:		2.0,	
	}
}

func (n *NodeSystemSensor) Tick() {

	n.TickCount++

	if n.TickCount % 1000 == 0 && n.State != protocol.StateCriticalLoad {
	
		chance := rand.Intn(100) 

		if chance <= 1 {
			
			n.State = protocol.StateCriticalLoad
		} else if chance <= 6 { 
			
			n.State = protocol.StateHighLoad
		} else {
			
			n.State = protocol.StateNormalLoad
		}
	}

	switch n.State {

		case protocol.StateNormalLoad:
			
			n.InputThroughput = 0.5 + (rand.Float64() * 0.2 - 0.1)
		
			n.InputInterrupts = 5000.0 + (rand.Float64() * 1000 - 500)

		case protocol.StateHighLoad:
			
			n.InputThroughput = 8.5 + (rand.Float64() * 1.0 - 0.5)
			
			n.InputInterrupts = 60000.0 + (rand.Float64() * 5000 - 2500)

		case protocol.StateCriticalLoad:
			
			n.InputThroughput = 0.1 + (rand.Float64() * 0.05) 
			n.InputInterrupts = 1200000.0 + (rand.Float64() * 100000 - 50000)
		}

	if n.InputThroughput < 0 { 
		n.InputThroughput = 0.01 
	}

	weightThroughput := (n.InputThroughput / n.NetworkCap) * 60.0
	weightInterrupts := (n.InputInterrupts / 1000000.0) * 100.0
	
	n.CurrentStress = weightThroughput + weightInterrupts
	if n.CurrentStress > 100.0 {
		n.CurrentStress = 100.0
	}

	availableCPUForData := 100.0 - weightInterrupts
	if availableCPUForData < 0 { availableCPUForData = 0 }
	
	actualThroughput := n.InputThroughput * (availableCPUForData / 100.0)
	if actualThroughput > n.NetworkCap { actualThroughput = n.NetworkCap }

	utilizationFraction := n.CurrentStress / 100.0 
	
	if utilizationFraction >= 0.99 {
		n.CurrentLatency = 5000.0 
	} else {
		n.CurrentLatency = n.BaseLatency / (1.0 - utilizationFraction)
	}
	n.CurrentLatency += rand.Float64() * 0.5

	dynamicPower := (n.MaxPower - n.BasePower) * utilizationFraction
	n.CurrentPower = n.BasePower + dynamicPower + (rand.Float64() * 5.0)

	heatGenerated := n.CurrentPower * 0.0005 
	
	heatDissipated := n.HVACCoolingLevel * 0.002 * (n.CurrentTemp - n.AmbientTemp)
	
	tempVariation := (heatGenerated - heatDissipated) / n.ThermalMass
	n.CurrentTemp += tempVariation

	if n.CurrentTemp > 105.0 {
		n.CurrentTemp = 105.0 + (rand.Float64() * 0.5)
	}
}