package protocol

// TelemetryPacket constit cada relatório de telemetria enviado por um nó ao servidor central
// Contém métricas de desempenho, temperatura e estado do nó
type TelemetryPacket struct {
	// ID identifica exclusivamente qual nó enviou este pacote (range 1-8)
	ID int `json:"node_id"`
	// Timestamp registra o timestamp Unix em milissegundos quando o pacote foi criado
	Timestamp int64 `json:"timestamp"`
	// TickCount é um contador incremental de ciclos de simulação do nó
	TickCount int `json:"tick"`
	// CurrentState indica o estado operacional atual do nó (Normal, Alto, Crítico ou Offline)
	CurrentState NodeState `json:"current_state"`
	// Temperature é a temperatura atual do nó em graus Celsius
	Temperature float64 `json:"temperature"`
	// Stress é a carga de CPU atual em percentual (0-100%)
	Stress float64 `json:"stress"`
	// Power é o consumo de energia atual do nó em Watts
	Power float64 `json:"power_draw"`
	// Latency é a latência de rede atual em milissegundos
	Latency float64 `json:"latency"`
	// InputThroughput indica o throughput de entrada de dados em Gbps
	InputThroughput float64 `json:"throughput"`
	// InputInterrupts conta o número de interrupções de hardware do nó por segundo
	InputInterrupts float64 `json:"interrupts"`
	// HVACState armazena o estado atual do sistema de resfriamento (0=Off, 1=Balanced, 2=Maximum)
	HVACState int64 `json:"hvac_state"`
	// LBActive indica se o Load Balancer está ativo/drenando tráfego no nó
	LBActive bool `json:"lb_active"`
}
