package node

import (
	"edge-nodes/pkg/protocol"
	"math/rand"
	"sync/atomic"
)

// NodeSystemSensor simula um nó físico do data center com propriedades termodinâmicas,
// de rede e computacionais. É responsável por gerar telemetria realista com base em carga.
type NodeSystemSensor struct {
	// Identificador único e estado operacional do nó
	ID int

	State     protocol.NodeState
	TickCount int

	// Métricas de desempenho em tempo real
	CurrentTemp    float64
	CurrentStress  float64
	CurrentPower   float64
	CurrentLatency float64

	// Tráfego de entrada (throughput) e interrupções de hardware
	InputThroughput float64
	InputInterrupts float64
	// HVACState armazena o estado do resfriamento, necessário thread-safe
	HVACState atomic.Int64

	// Parâmetros físicos do hardware (constantes para este nó)
	BasePower   float64
	MaxPower    float64
	ThermalMass float64
	AmbientTemp float64
	MaxTemp     float64
	NetworkCap  float64
	BaseLatency float64

	// Estados dos atuadores (thread-safe usando atomic)
	LBActive   atomic.Bool
	HVACActive atomic.Bool
}



// NewNode cria uma nova instância de nó sensor com configurações padrão inicializadas
// Cada nó começa em estado equilibrado com temperatura ambiente
func NewNode(id int) *NodeSystemSensor {
	node := &NodeSystemSensor{
		ID: id,

		// Valores iniciais das métricas em repouso
		CurrentTemp:    22.0,
		CurrentStress:  0.0,
		CurrentPower:   250.0,
		CurrentLatency: 5.0,

		// Tráfego inicial baixo
		InputThroughput: 0.2,
		InputInterrupts: 5000.0,

		// Parâmetros de hardware típicos de servidor
		BasePower:   150.0,  // Potência mínima em repouso
		MaxPower:    650.0,  // Consumo máximo sob carga total
		ThermalMass: 1000.0, // Inércia térmica do hardware
		AmbientTemp: 20.0,   // Temperatura do ar ambiente
		NetworkCap:  9.0,    // Capacidade de rede em Gbps
		BaseLatency: 2.0,    // Latência mínima em ms
	}

	// Inicializa HVAC em estado equilibrado
	node.HVACState.Store(int64(protocol.StateBalanced))

	return node
}

// Tick orquestra um ciclo completo de simulação do nó
// Atualiza todas as métricas: estado, tráfego, CPU, latência, energia e temperatura
func (n *NodeSystemSensor) Tick() {
	n.TickCount++

	// Executa todas as funções de simulação em sequência durante este tick
	n.simulateStateAnomalies()  // Injeta falhas/anomalias de carga
	n.generateNetworkTraffic()  // Gera tráfego baseado no estado
	n.computeSystemStress()     // Calcula carga de CPU
	n.computeNetworkLatency()   // Calcula latência de rede
	n.computePowerConsumption() // Calcula consumo energético
	n.updateThermodynamics()    // Atualiza temperatura
}

// simulateStateAnomalies injeta falhas e picos de carga baseados em probabilidade
// Se Load Balancer está ativo, força o nó a voltar ao estado normal
// A cada 1000 ticks, há chance de transição de estado (1% -> Crítico, 5% -> Alto)
func (n *NodeSystemSensor) simulateStateAnomalies() {

	// Load Balancer ativo prioriza manutenção do estado normal
	if n.LBActive.Load() {
		n.State = protocol.StateNormalLoad
		return
	}

	// Só tenta mudar de estado a cada 1000 ticks (amortecimento)
	// e se NÃO estiver já em estado crítico (evita oscilações)
	if n.TickCount%1000 != 0 || n.State == protocol.StateCriticalLoad {
		return
	}

	// Gera número aleatório 0-99 para decidir novo estado
	chance := rand.Intn(100)

	// 1% de chance: Estado Crítico (ativa auto-heal no servidor)
	if chance <= 1 {
		n.State = protocol.StateCriticalLoad
		// 5% de chance: Estado Alto (requer atenção)
	} else if chance <= 6 {
		n.State = protocol.StateHighLoad
		// 94% de chance: Estado Normal (operação regular)
	} else {
		n.State = protocol.StateNormalLoad
	}
}

// generateNetworkTraffic simula o tráfego de rede e interrupções de hardware baseado no estado atual
// Estado crítico causa 1200000 interrupções/s e throughput muito baixo (gargalo)
func (n *NodeSystemSensor) generateNetworkTraffic() {

	// Varia o tráfego e interrupções de acordo com o estado operacional
	switch n.State {

	case protocol.StateNormalLoad:
		// Tráfego: 0.4-0.6 Gbps com variação aleatória
		n.InputThroughput = 0.5 + (rand.Float64()*0.2 - 0.1)
		// Interrupções: ~5000/s (baixas) com ruído
		n.InputInterrupts = 5000.0 + (rand.Float64()*1000 - 500)
	case protocol.StateHighLoad:
		// Tráfego: 8.0-9.0 Gbps (utilização alta)
		n.InputThroughput = 8.5 + (rand.Float64()*1.0 - 0.5)
		// Interrupções: ~60000/s (muitas interrupções de hardware)
		n.InputInterrupts = 60000.0 + (rand.Float64()*5000 - 2500)
	case protocol.StateCriticalLoad:
		// Tráfego: apenas 0.05-0.15 Gbps (gargalo severo)
		n.InputThroughput = 0.1 + (rand.Float64() * 0.05)
		// Interrupções: massivo ~1.2M/s (escalonamento de contexto)
		n.InputInterrupts = 1200000.0 + (rand.Float64()*100000 - 50000)
	}

	// Garante que o throughput nunca fica negativo (float64 arredonda)
	if n.InputThroughput < 0 {
		n.InputThroughput = 0.01
	}
}

// computeSystemStress calcula a carga do sistema (CPU) com base nas interrupções e tráfego de rede
// Interrupções têm maior peso (75%) que o throughput (25%) na CPU final
func (n *NodeSystemSensor) computeSystemStress() {
	// Throughput contribui até 60% da carga final (0.5/9.0 Gbps * 60%)
	weightThroughput := (n.InputThroughput / n.NetworkCap) * 60.0
	// Interrupções contribuem até 100% da carga (1.2M/1M * 100% => capped a 100%)
	weightInterrupts := (n.InputInterrupts / 1000000.0) * 100.0

	// CPU total = throughput + interrupções, capped em 100%
	n.CurrentStress = weightThroughput + weightInterrupts
	if n.CurrentStress > 100.0 {
		n.CurrentStress = 100.0
	}

	// Calcula CPU disponível para processamento de dados (reduz conforme interrupções sobem)
	availableCPUForData := 100.0 - weightInterrupts
	if availableCPUForData < 0 {
		availableCPUForData = 0
	}

	// Throughput efetivo reduz conforme CPU gasta com interrupções
	actualThroughput := n.InputThroughput * (availableCPUForData / 100.0)
	if actualThroughput > n.NetworkCap {
		actualThroughput = n.NetworkCap
	}
}

// computeNetworkLatency calcula a latência de rede com base na utilização de CPU
// Latência segue a fórmula de fila M/M/1: L = L0 / (1 - utilização)
func (n *NodeSystemSensor) computeNetworkLatency() {
	// Normaliza utilização entre 0.0 e 1.0
	utilizationFraction := n.CurrentStress / 100.0

	// Em utilização extrema (99%+), latência vai para 5000ms (timeout)
	if utilizationFraction >= 0.99 {
		n.CurrentLatency = 5000.0
	} else {
		// Latência aumenta exponencialmente conforme se aproxima de 100%
		n.CurrentLatency = n.BaseLatency / (1.0 - utilizationFraction)
	}
	// Adiciona ruído sensorístico (0-0.5ms)
	n.CurrentLatency += rand.Float64() * 0.5
}

// computePowerConsumption calcula o consumo de energia do nó
// Potência é linear com a utilização de CPU: P = BasePower + (MaxPower-BasePower)*stress%
func (n *NodeSystemSensor) computePowerConsumption() {
	// Converte utilização de CPU (0-100%) para fração (0.0-1.0)
	utilizationFraction := n.CurrentStress / 100.0
	// Potência dinâmica adicional com base em CPU (0W em repouso, max em 100%)
	dynamicPower := (n.MaxPower - n.BasePower) * utilizationFraction

	// Total = BasePower (sempre) + Dinâmica (varia com CPU) + Ruído sensorístico
	n.CurrentPower = n.BasePower + dynamicPower + (rand.Float64() * 5.0)
}

// updateThermodynamics simula a dinâmica térmica do hardware
// Temperatura varia conforme: Heat_generated (CPU) vs Heat_dissipated (HVAC)
func (n *NodeSystemSensor) updateThermodynamics() {

	// Converte parte da energia em calor dissipado (1% da potência = calor)
	generationFactor := 0.01
	heatGenerated := n.CurrentPower * generationFactor

	// Potência de resfriamento varia com o estado do HVAC
	var coolingPower float64

	// Lê o estado do HVAC de forma thread-safe
	currentState := protocol.HVACState(n.HVACState.Load())

	// Define eficiência de resfriamento baseado no nível de HVAC
	switch currentState {
	case protocol.StateOff:
		// Resfriamento mínimo se HVAC desligado (apenas ventilação passiva)
		coolingPower = 0.01
	case protocol.StateBalanced:
		// Resfriamento normal (produz em 10% da perda térmica)
		coolingPower = 0.10
	case protocol.StateMaximum:
		// Máximo resfriamento para emergência (30% de eficiência)
		coolingPower = 0.30
	default:
		// Fallback para equilibrado se valor desconhecido
		coolingPower = 0.10
	}

	// Calor dissipado pelo HVAC (proporcional à diferença de temperatura)
	heatDissipated := coolingPower * (n.CurrentTemp - n.AmbientTemp)

	// Variação de temperatura em graus Celsius (Lei de Newton de Resfriamento)
	tempVariation := (heatGenerated - heatDissipated) / n.ThermalMass
	n.CurrentTemp += tempVariation

	// Adiciona ruído sensorístico (±0.1°C)
	sensorNoise := (rand.Float64() * 0.2) - 0.1
	n.CurrentTemp += sensorNoise

	// Garante que temp mínima nunca fica abaixo de ambiente + ruído
	if n.CurrentTemp < n.AmbientTemp {
		n.CurrentTemp = n.AmbientTemp + (rand.Float64() * 0.5)
	}
	// Garante que temp máxima nunca excede ~105°C (proteção física)
	if n.CurrentTemp > 105.0 {
		n.CurrentTemp = 105.0 - (rand.Float64() * 0.5)
	}
}
