package processor

import (
	"math/rand"
	"time"
	//"pkg/protocol"
	"fmt"
)

type CoreSensor struct {

	ID 				int
	Temperature 	float64
	Voltage 		float64
	Utilization 	float64
	Frequency 		float64

	BaseFrequency 	float64
	MaxFrequency 	float64
	MinVoltage 		float64
	MaxVoltage 		float64
}