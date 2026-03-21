package protocol

type NodeState int

const (
	StateNormalLoad NodeState = iota
	StateHighLoad
	StateCriticalLoad
)