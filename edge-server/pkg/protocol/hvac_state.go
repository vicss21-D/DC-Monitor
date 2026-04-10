package protocol

// HVACState representa os níveis de resfriamento do sistema HVAC no data center
type HVACState int

const (
	// StateOff indica que o sistema de resfriamento está desligado (sem resfriamento ativo)
	StateOff HVACState = iota
	// StateBalanced indica operação normal do HVAC com resfriamento equilibrado
	StateBalanced
	// StateMaximum indica o máximo resfriamento disponível para emergências térmicas
	StateMaximum
)
