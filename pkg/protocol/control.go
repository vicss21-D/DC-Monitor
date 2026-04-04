package protocol

type ControlMessage struct {
	Type       string `json:"type"`        // Qual atuador: "hvac" ou "lb"
	Signal     string `json:"signal"`      // Qual ação: "trigger_on" ou "trigger_off"
	TargetNode int    `json:"target_node"` // ID numérico do nó alvo (ex: 1, 2, 3...)
	Requester  string `json:"requester"`   // Quem solicitou: "user" ou "auto"
}