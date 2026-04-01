package protocol

// ControlMessage é o formato do JSON enviado pelo navegador e repassado via TCP
type ControlMessage struct {
	Type       string `json:"type"`        // Qual atuador: "hvac" ou "lb"
	Signal     string `json:"signal"`      // Qual ação: "trigger_on" ou "trigger_off"
	TargetNode int    `json:"target_node"` // ID numérico do nó alvo (ex: 1, 2, 3...)
}