package protocol

// ControlMessage define a estrutura de mensagem de controle para acionar atuadores
// nos nós do data center (HVAC e Load Balancer).
type ControlMessage struct {
	// Type especifica qual atuador será acionado: "hvac" ou "lb" (Load Balancer)
	Type string `json:"type"` // Qual atuador: "hvac" ou "lb"
	// Signal especifica qual ação executar no atuador: "set_max", "set_balanced", "set_off", "trigger_on", etc.
	Signal string `json:"signal"` // Qual ação: "trigger_on" ou "trigger_off"
	// TargetNode é o ID numérico do nó específico que receberá o comando de controle
	TargetNode int `json:"target_node"` // ID numérico do nó alvo (ex: 1, 2, 3...)
	// Requester identifica a origem do comando: "user" para comandos manuais do usuário, "auto" para auto-heal
	Requester string `json:"requester"` // Quem solicitou: "user" ou "auto"
}
