package protocol

type HVACState int

const (
	StateOff HVACState = iota
	StateBalanced
	StateMaximum
)