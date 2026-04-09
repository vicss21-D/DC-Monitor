// ==========================================
// VARIÁVEIS GLOBAIS
// ==========================================
// Variável global para armazenar a conexão WebSocket (necessário no escopo global para auto-reconexão)
let ws;
// Mapa para gerenciar timeouts de alertas (evita múltiplos piscares simultâneos)
const alertTimeouts = {};
// ID do nó cuja gaveta de detalhes está aberta (null se nenhuma aberta)
let activeDrawerNode = null;
// Instância do gráfico Chart.js para histórico de temperatura e stress
let historyChart = null;
// Número máximo de pontos mantidos no histórico (evita consumo de memória)
const MAX_HISTORY = 10;
// Cache em memória do histórico de cada nó {temp: [], stress: [], labels: []}
const nodeHistory = {};      

// ==========================================
// 1. INICIALIZAÇÃO DO WEBSOCKET (Com Auto-Heal)
// ==========================================
// Conecta-se ao servidor via WebSocket usando protocolo seguro (wss) ou inseguro (ws)
// Implementa reconexão automática com intervalo de 3 segundos
function connectWebSocket() {
    // Define protocolo com base no contexto (HTTPS usa WSS, HTTP usa WS)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Monta URL do WebSocket no mesmo host do servidor
    const wsURL = `${protocol}//${window.location.host}/ws`;
    
    // Log de diagnóstico
    console.log("Tentando conectar ao Servidor Central...");
    // Cria instância WebSocket
    ws = new WebSocket(wsURL);

    // Event handler: Ativado quando a conexão é estabelecida com sucesso
    ws.onopen = function() {
        console.log("✅ Conexão WebSocket estabelecida com o Gateway.");
    };

    // Event handler: Ativado quando mensagens chegam do servidor
    ws.onmessage = function(event) {
        try {
            // Desserializa JSON recebido
            const envelope = JSON.parse(event.data);
            
            // Verifica se é o nosso novo Padrão Envelope (Batch ou Critical)
            if (envelope.type === 'batch' || envelope.type === 'critical') {
                
                // O Payload agora é um Array com até 8 nós
                envelope.payload.forEach(packet => {
                    
                    // Atualiza o card do nó na dashboard principal
                    updateDashboard(packet);
                    
                    // Dispara alerta visual se o pacote veio pela via crítica (emergência)
                    if (envelope.type === 'critical') {
                        triggerVisualAlert(packet.node_id);
                    }

                    // Incrementa histórico de métricas para o gráfico
                    const id = packet.node_id;
                    // Cria estrutura de histórico se não existir
                    if (!nodeHistory[id]) {
                        nodeHistory[id] = { temp: [], stress: [], labels: [] };
                    }
                    
                    // Ignora nós offline e armazena apenas nós com dados válidos
                    if (packet.current_state !== -1) {
                        nodeHistory[id].labels.push('');
                        nodeHistory[id].temp.push(packet.temperature);
                        nodeHistory[id].stress.push(packet.stress);
                        
                        // Remove dados antigos se ultrapassar limite de histórico
                        if (nodeHistory[id].temp.length > MAX_HISTORY) {
                            nodeHistory[id].labels.shift();
                            nodeHistory[id].temp.shift();
                            nodeHistory[id].stress.shift();
                        }
                    }
                    
                    // Atualiza painel detalhado (gaveta) apenas se estiver aberto para este nó
                    if (activeDrawerNode === id) {
                        // Exibe '-- ' para nós offline
                        if (packet.current_state === -1) {
                            document.getElementById('drawer-power').innerText = "-- W";
                            document.getElementById('drawer-latency').innerText = "-- ms";
                        } else {
                            // Exibe dados com 1 casa decimal
                            document.getElementById('drawer-power').innerText = Number(packet.power_draw).toFixed(1) + " W";
                            document.getElementById('drawer-latency').innerText = Number(packet.latency).toFixed(1) + " ms";
                        }
                        
                        // Atualiza gráfico histórico com novos dados
                        historyChart.data.labels = nodeHistory[id].labels;
                        historyChart.data.datasets[0].data = nodeHistory[id].temp;
                        historyChart.data.datasets[1].data = nodeHistory[id].stress;
                        historyChart.update();
                    }
                });
            }
        } catch (error) {
            console.error("Erro no pacote:", error);
        }
    };

    // Event handler: Executado quando erro de conexão ocorre
    ws.onerror = function(error) {
        console.error("⚠️ Erro de rede no WebSocket.");
        // Fecha a conexão para ativar o handler onclose que fará reconexão
        ws.close();
    };

    // A MÁGICA DA RECONEXÃO ESTÁ AQUI
    // Event handler: Reconexão automática quando servidor desconecta
    ws.onclose = function() {
        console.warn("❌ Conexão perdida (Servidor parado). Tentando reconectar em 3 segundos...");
        
        // Marca todos os nós como OFFLINE visualmente (cinzento)
        const badges = document.querySelectorAll('.badge');
        badges.forEach(b => { 
            b.innerText = 'OFFLINE'; 
            if(b.parentElement && b.parentElement.parentElement) {
                b.parentElement.parentElement.className = 'sensor-card offline'; 
            }
        });

        // Agenda reconexão após 3 segundos (backoff simples)
        setTimeout(connectWebSocket, 3000);
    };
}

// Inicia a conexão WebSocket quando a página HTML carrega
connectWebSocket();

// ==========================================
// FUNÇÕES DA GAVETA E GRÁFICO
// ==========================================
// Inicializa o gráfico Chart.js com histórico de temperatura e CPU stress
function initChart() {
    // Obtém contexto 2D do canvas para desenhar o gráfico
    const ctx = document.getElementById('historyChart').getContext('2d');
    // Cria instância do gráfico com configurações
    historyChart = new Chart(ctx, {
        // Tipo de gráfico: linha (line chart)
        type: 'line',
        // Dados: labels (eixo X) e datasets (2 séries: temperatura e stress)
        data: {
            labels: [],
            datasets: [
                // Série 1: Temperatura em Celsius (cor vermelha)
                { label: 'Temperatura (°C)', borderColor: '#f44336', data: [], tension: 0.4 },
                // Série 2: CPU Stress em percentual (cor laranja)
                { label: 'CPU Stress (%)', borderColor: '#ff9800', data: [], tension: 0.4 }
            ]
        },
        // Opções de renderização
        options: {
            responsive: true,        // Adapta-se ao tamanho do container
            animation: false,         // Desativa animação para melhor performance
            scales: { y: { beginAtZero: true, max: 110 } }  // Eixo Y: 0-110
        }
    });
}

// Abre a gaveta (drawer) com detalhes completos do nó selecionado
function openDetailsModal(nodeId) {
	// Marca qual nó tem a gaveta aberta
	activeDrawerNode = nodeId;
	// Atualiza título da gaveta
	document.getElementById('drawer-title').innerText = `Detalhes do Nó ${nodeId}`;
	// Adiciona classe CSS para mostrar a gaveta (pode ter animação)
	document.body.classList.add('drawer-open');
	
	// Inicializa gráfico na primeira abertura
    if (!historyChart) initChart();
}

// Fecha a gaveta (drawer) de detalhes
function closeDetailsModal() {
	// Limpa o nó ativo
	activeDrawerNode = null;
	// Remove a classe CSS que mostra a gaveta
    document.body.classList.remove('drawer-open');
}

// ==========================================
// 2. ATUALIZAÇÃO DA INTERFACE (DOM)
// ==========================================
function updateDashboard(packet) {
    const normalZone = document.getElementById('dashboard-normal');
    const criticalZone = document.getElementById('dashboard-critical');
    const criticalWrapper = document.getElementById('zone-critical-wrapper');
    
    const nodeId = `node-${packet.node_id}`;
    let card = document.getElementById(nodeId);

    if (!card) {
        card = document.createElement('div');
        card.id = nodeId;
        card.onclick = function(e) {
            if(e.target.closest('.control-group')) return; 
            openDetailsModal(packet.node_id);
        };
        
        card.innerHTML = `
            <div class="card-header">
                <h3>Nó ${packet.node_id}</h3>
                <span id="badge-${packet.node_id}" class="badge">NORMAL</span>
            </div>
            <div class="card-body">
                <p><strong>Temp:</strong> <span id="temp-${packet.node_id}"></span> °C</p>
                <p><strong>CPU:</strong> <span id="stress-${packet.node_id}"></span> %</p>
                <p><strong>Latência:</strong> <span id="latency-${packet.node_id}"></span> ms</p>
            </div>
            <div class="control-group">
                <div class="switch-container">
                    <span style="font-size: 0.9rem; color: #555;"><strong>Balanceador</strong></span>
                    <label class="switch">
                        <input type="checkbox" id="lb-switch-${packet.node_id}" onchange="toggleLB(${packet.node_id}, this.checked)">
                        <span class="slider"></span>
                    </label>
                </div>
                <div class="hvac-label"><strong>Nível do HVAC</strong></div>
                <div class="segmented-control">
                    <button id="hvac-0-${packet.node_id}" onclick="setHVAC(${packet.node_id}, 'set_off')">OFF</button>
                    <button id="hvac-1-${packet.node_id}" onclick="setHVAC(${packet.node_id}, 'set_balanced')">BAL</button>
                    <button id="hvac-2-${packet.node_id}" onclick="setHVAC(${packet.node_id}, 'set_max')">MAX</button>
                </div>
            </div>
        `;
        normalZone.appendChild(card);
    }

    if (packet.current_state === -1) {
        document.getElementById(`temp-${packet.node_id}`).innerText = "--";
        document.getElementById(`stress-${packet.node_id}`).innerText = "--";
        document.getElementById(`latency-${packet.node_id}`).innerText = "--";
    } else {
        document.getElementById(`temp-${packet.node_id}`).innerText = Number(packet.temperature).toFixed(1);
        document.getElementById(`stress-${packet.node_id}`).innerText = Number(packet.stress).toFixed(0);
        document.getElementById(`latency-${packet.node_id}`).innerText = Number(packet.latency).toFixed(0);
    }

    const badge = document.getElementById(`badge-${packet.node_id}`);
    const lbSwitch = document.getElementById(`lb-switch-${packet.node_id}`);
    const hvacBtn0 = document.getElementById(`hvac-0-${packet.node_id}`);
    const hvacBtn1 = document.getElementById(`hvac-1-${packet.node_id}`);
    const hvacBtn2 = document.getElementById(`hvac-2-${packet.node_id}`);
    
    if (packet.current_state === -1) {
        card.className = 'sensor-card offline';
        badge.innerText = 'OFFLINE';
        if (card.parentElement !== normalZone) normalZone.appendChild(card);
        
        lbSwitch.disabled = true;
        hvacBtn0.disabled = true; hvacBtn1.disabled = true; hvacBtn2.disabled = true;
    } 
    else if (packet.current_state === 2 || packet.current_state === 1) {
        if (packet.current_state === 2) {
            card.className = 'sensor-card critical';
            badge.innerText = 'CRÍTICO';
        } else {
            card.className = 'sensor-card warning';
            badge.innerText = 'ALTO';
        }
        if (card.parentElement !== criticalZone) criticalZone.appendChild(card);
        
        lbSwitch.disabled = false;
        hvacBtn0.disabled = false; hvacBtn1.disabled = false; hvacBtn2.disabled = false;
    } 
    else {
        card.className = 'sensor-card normal';
        badge.innerText = 'NORMAL';
        if (card.parentElement !== normalZone) normalZone.appendChild(card);
        
        lbSwitch.disabled = false;
        hvacBtn0.disabled = false; hvacBtn1.disabled = false; hvacBtn2.disabled = false;
    }

    if (criticalZone.children.length > 0) {
        criticalWrapper.style.display = 'block';
    } else {
        criticalWrapper.style.display = 'none';
    }

    if (packet.current_state !== -1) {
        if (lbSwitch.checked !== packet.lb_active) {
            lbSwitch.checked = packet.lb_active;
        }

        hvacBtn0.className = ''; hvacBtn1.className = ''; hvacBtn2.className = '';
        if (packet.hvac_state === 0) hvacBtn0.className = 'active-off';
        else if (packet.hvac_state === 1) hvacBtn1.className = 'active-bal';
        else if (packet.hvac_state === 2) hvacBtn2.className = 'active-max';
    }
}

// Dispara alerta visual no card: pisca por 1.5 segundos
function triggerVisualAlert(nodeId) {
	// Obtém elemento do card
	const card = document.getElementById(`node-${nodeId}`);
	if (card) {
		// Adiciona classe CSS para efeito de piscar
		card.classList.add('blinking-alert');
		
		// Limpa timeout anterior se existir (evita múltiplos alertas)
		if (alertTimeouts[nodeId]) {
			clearTimeout(alertTimeouts[nodeId]);
		}
		
		// Remove efeito de piscar após 1.5 segundos
        alertTimeouts[nodeId] = setTimeout(() => {
            card.classList.remove('blinking-alert');
        }, 1500);
    }
}

// ==========================================
// DISPARADORES DE EVENTO
// ==========================================
// Handler: Dispara comando de Load Balancer quando usuário alterna o switch
function toggleLB(nodeId, isChecked) {
	// Converte boolean do switch em sinal de comando
	const signal = isChecked ? 'trigger_on' : 'trigger_off';
	// Envia comando HTTP ao servidor
    sendControl('lb', signal, nodeId);
}

// Handler: Dispara comando de HVAC quando usuário alterna o segmented control
function setHVAC(nodeId, levelSignal) {
	// levelSignal pode ser: set_off, set_balanced, set_max
    sendControl('hvac', levelSignal, nodeId);
}

// ==========================================
// 3. COMANDOS DE ATUAÇÃO FÍSICA (HTTP)
// ==========================================
// Envia comando de controle ao servidor central via HTTP POST
// Responsável por comunicar com atuadores (HVAC e Load Balancer)
function sendControl(type, signal, nodeId) {
    // Monta payload JSON com informações do comando
    const payload = {
        type: type,             // 'hvac' ou 'lb'
        signal: signal,         // 'set_max', 'set_balanced', 'set_off', etc
        target_node: parseInt(nodeId, 10)  // ID numérico do nó alvo
    };

    // Log para debugging
    console.log(`[OUT] Disparando comando HTTP para API Gateway:`, payload);

    // Envia requisição HTTP POST para /api/control
    fetch('/api/control', {
        method: 'POST',
        headers: { 
            'Content-Type': 'application/json' 
        },
        body: JSON.stringify(payload)
    })
    // Handler de sucesso
    .then(response => {
        if (!response.ok) {
            throw new Error(`Servidor Gateway retornou Status ${response.status}`);
        }
        // Log de confirmação
        console.log(`[IN] Comando [${type}:${signal}] confirmado pelo Nó ${nodeId}`);
    })
    // Handler de erro
    .catch(err => {
        console.error(`[FALHA] Não foi possível atuar no Nó ${nodeId}:`, err);
        alert(`Falha de comunicação ao tentar enviar o comando para o Nó ${nodeId}. Verifique o console.`);
    });
}