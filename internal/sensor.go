package node

import (
	"dc-monitor/pkg/protocol"
	"math/rand"
	"sync/atomic"
	"time"
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
	HVACState 			atomic.Int64

	BasePower 			float64
	MaxPower			float64
	ThermalMass			float64
	AmbientTemp			float64
	MaxTemp				float64
	NetworkCap			float64
	BaseLatency			float64

	LBActive            atomic.Bool
	HVACActive          atomic.Bool
}

func setHVAC(n* NodeSystemSensor) {
	n.HVACActive.Store(true)
	time.Sleep(2 * time.Second)
	n.HVACActive.Store(false)
}

func setLB(n* NodeSystemSensor) {
	n.LBActive.Store(true)
	time.Sleep(2 * time.Second)
	n.LBActive.Store(false)
}

func NewNode(id int) *NodeSystemSensor{
	node := &NodeSystemSensor{
		ID: 				id,
		
		CurrentTemp: 		22.0,
		CurrentStress:     	0.0,
		CurrentPower: 		250.0,
		CurrentLatency:   	5.0,

		InputThroughput: 	0.2,
		InputInterrupts: 	5000.0,			

		BasePower: 			150.0,
		MaxPower:			650.0,
		ThermalMass:		1000.0,	
		AmbientTemp:		20.0,
		MaxTemp:			105.0,	
		NetworkCap:			9.0,
		BaseLatency:		2.0,	
	}

	node.HVACState.Store(int64(protocol.StateBalanced))
	
	return node
}

// Tick orquestra o ciclo de vida do sensor a cada iteração

func (n *NodeSystemSensor) Tick() {
	n.TickCount++

	n.simulateStateAnomalies()
	n.generateNetworkTraffic()
	n.computeSystemStress()
	n.computeNetworkLatency()
	n.computePowerConsumption()
	n.updateThermodynamics()
}

// Injeta falhas e picos de carga baseados em probabilidade

func (n *NodeSystemSensor) simulateStateAnomalies() {

	if n.LBActive.Load() {
		n.State = protocol.StateNormalLoad
		return
	}

	if n.TickCount%1000 != 0 || n.State == protocol.StateCriticalLoad {
		return
	}

	chance := rand.Intn(100)

	if chance <= 1 {
		n.State = protocol.StateCriticalLoad
	} else if chance <= 6 {
		n.State = protocol.StateHighLoad
	} else {
		n.State = protocol.StateNormalLoad
	}
}

// Simula o tráfego e as interrupções de hardware com base no estado atual

func (n *NodeSystemSensor) generateNetworkTraffic() {

	switch n.State {

	case protocol.StateNormalLoad:
		n.InputThroughput = 0.5 + (rand.Float64()*0.2 - 0.1)
		n.InputInterrupts = 5000.0 + (rand.Float64()*1000 - 500)
	case protocol.StateHighLoad:
		n.InputThroughput = 8.5 + (rand.Float64()*1.0 - 0.5)
		n.InputInterrupts = 60000.0 + (rand.Float64()*5000 - 2500)
	case protocol.StateCriticalLoad:
		n.InputThroughput = 0.1 + (rand.Float64() * 0.05)
		n.InputInterrupts = 1200000.0 + (rand.Float64()*100000 - 50000)
	}

	if n.InputThroughput < 0 {
		n.InputThroughput = 0.01
	}
}

// Calcula a carga do sistema com base nas interrupções e tráfego de rede

func (n *NodeSystemSensor) computeSystemStress() {
	weightThroughput := (n.InputThroughput / n.NetworkCap) * 60.0
	weightInterrupts := (n.InputInterrupts / 1000000.0) * 100.0

	n.CurrentStress = weightThroughput + weightInterrupts
	if n.CurrentStress > 100.0 {
		n.CurrentStress = 100.0
	}

	availableCPUForData := 100.0 - weightInterrupts
	if availableCPUForData < 0 {
		availableCPUForData = 0
	}

	actualThroughput := n.InputThroughput * (availableCPUForData / 100.0)
	if actualThroughput > n.NetworkCap {
		actualThroughput = n.NetworkCap
	}
}

func (n *NodeSystemSensor) computeNetworkLatency() {
	utilizationFraction := n.CurrentStress / 100.0

	if utilizationFraction >= 0.99 {
		n.CurrentLatency = 5000.0
	} else {
		n.CurrentLatency = n.BaseLatency / (1.0 - utilizationFraction)
	}
	n.CurrentLatency += rand.Float64() * 0.5
}

func (n *NodeSystemSensor) computePowerConsumption() {
	utilizationFraction := n.CurrentStress / 100.0
	dynamicPower := (n.MaxPower - n.BasePower) * utilizationFraction
	
	n.CurrentPower = n.BasePower + dynamicPower + (rand.Float64() * 5.0)
}

func (n *NodeSystemSensor) updateThermodynamics() {
	
	generationFactor := 0.01 
	heatGenerated := n.CurrentPower * generationFactor

	var coolingPower float64
	
	currentState := protocol.HVACState(n.HVACState.Load())

	switch currentState {
	case protocol.StateOff:
		coolingPower = 0.01
	case protocol.StateBalanced:
		coolingPower = 0.10
	case protocol.StateMaximum:
		coolingPower = 0.30
	default:
		coolingPower = 0.10 // Fallback
	}

	heatDissipated := coolingPower * (n.CurrentTemp - n.AmbientTemp)

	tempVariation := (heatGenerated - heatDissipated) / n.ThermalMass
	n.CurrentTemp += tempVariation

	sensorNoise := (rand.Float64() * 0.2) - 0.1
	n.CurrentTemp += sensorNoise

	if n.CurrentTemp < n.AmbientTemp {
		n.CurrentTemp = n.AmbientTemp + (rand.Float64() * 0.5)
	}
	if n.CurrentTemp > 105.0 {
		n.CurrentTemp = 105.0 - (rand.Float64() * 0.5)
	}
}