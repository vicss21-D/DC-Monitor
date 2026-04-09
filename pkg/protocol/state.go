package protocol

// NodeState representa os estados operacionais possíveis de um nó do data center
type NodeState int

const (
	// StateNormalLoad indica operação normal com carga baixa/média (0-60% CPU)
	StateNormalLoad NodeState = iota
	// StateHighLoad indica carga elevada no nó (60-90% CPU) - requer atenção
	StateHighLoad
	// StateCriticalLoad indica sobrecarga crítica do nó (>90% CPU) - ativa auto-heal automático
	StateCriticalLoad
)
