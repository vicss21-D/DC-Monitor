package processor

import (
	"math/rand"
	//"time"
	//"pkg/protocol"
	//"fmt"
)

type CoreSensor struct {
	ID          int
	Temperature float64
	Voltage     float64
	Utilization float64
	Frequency   float64

	BaseFrequency float64
	MaxFrequency  float64
	MinVoltage    float64
	MaxVoltage    float64
}

func NewCoreSensor(id int) *CoreSensor {
	return &CoreSensor{
		ID:          id,
		Temperature: 	35.0,
		Voltage:     	0.9,
		Utilization: 	0.0,
		Frequency:   	3.600,

		BaseFrequency: 	3.600,
		MaxFrequency:  	5.200,
		MinVoltage:    	0.9,
		MaxVoltage:    	1.3,
	}
}

func (c *CoreSensor) tickUpdate() {

	c.Utilization += (rand.Float64() * 10) - 5

	if c.Utilization > 100.0 {
		c.Utilization = 100.0
	}

	if c.Utilization < 0.0 {
		c.Utilization = 0.0
	}

	// Temperature

	// Voltage

	// Frequency
}
