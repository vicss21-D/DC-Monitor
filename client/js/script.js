// ==========================================
// 1. INICIALIZAÇÃO DO WEBSOCKET
// ==========================================
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsURL = `${protocol}//${window.location.host}/ws`;
const ws = new WebSocket(wsURL);

// ==========================================
// VARIÁVEIS DA GAVETA E GRÁFICO
// ==========================================

let activeDrawerNode = null; // Guarda o ID do nó que está com a gaveta aberta
let historyChart = null;     // Instância do Chart.js
const MAX_HISTORY = 10;      // Guarda apenas as últimas 10 atualizações (Janela deslizante)
const nodeHistory = {};      // Dicionário de memória: { 1: { temp: [], stress: [], labels: [] } }

// Inicializa a configuração vazia do Chart.js
function initChart() {
    const ctx = document.getElementById('historyChart').getContext('2d');
    historyChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: [],
            datasets: [
                { label: 'Temperatura (°C)', borderColor: '#f44336', data: [], tension: 0.4 },
                { label: 'CPU Stress (%)', borderColor: '#ff9800', data: [], tension: 0.4 }
            ]
        },
        options: {
            responsive: true,
            animation: false, // Desliga a animação padrão para não piscar a cada atualização
            scales: { y: { beginAtZero: true, max: 110 } }
        }
    });
}

// Abrir e Fechar a Gaveta
function openDetailsModal(nodeId) {
    activeDrawerNode = nodeId;
    document.getElementById('drawer-title').innerText = `Detalhes do Nó ${nodeId}`;
    document.body.classList.add('drawer-open');
    
    // Se ainda não iniciamos o gráfico, inicializa na primeira abertura
    if (!historyChart) initChart();
}

function closeDetailsModal() {
    activeDrawerNode = null;
    document.body.classList.remove('drawer-open');
}

ws.onopen = function() {
    console.log("Conexão WebSocket estabelecida com o Gateway.");
};

ws.onmessage = function(event) {
    try {
        const packet = JSON.parse(event.data);
        
        // 1. Atualiza o Card na tela principal
        updateDashboard(packet);
        console.log(packet)
        // 2. Registra no Histórico da Memória (Janela Deslizante)
        const id = packet.node_id;
        if (!nodeHistory[id]) {
            nodeHistory[id] = { temp: [], stress: [], labels: [] };
        }
        
        // Adiciona os novos valores aos arrays
        nodeHistory[id].labels.push(''); // Apenas para criar o eixo X
        nodeHistory[id].temp.push(packet.temperature);
        nodeHistory[id].stress.push(packet.stress);
        
        // Corta os dados antigos se passar do limite (O segredo contra memory leaks)
        if (nodeHistory[id].temp.length > MAX_HISTORY) {
            nodeHistory[id].labels.shift();
            nodeHistory[id].temp.shift();
            nodeHistory[id].stress.shift();
        }
        
        // 3. Atualiza a Gaveta SOMENTE se estiver aberta para este nó específico
        if (activeDrawerNode === id) {
            document.getElementById('drawer-power').innerText = Number(packet.power_draw).toFixed(1) + " W";
            document.getElementById('drawer-latency').innerText = Number(packet.latency).toFixed(1) + " ms";
            
            // Injeta os arrays de histórico no Chart.js e manda ele redesenhar
            historyChart.data.labels = nodeHistory[id].labels;
            historyChart.data.datasets[0].data = nodeHistory[id].temp;
            historyChart.data.datasets[1].data = nodeHistory[id].stress;
            historyChart.update();
        }

    } catch (error) {
        console.error("Erro no pacote:", error);
    }
};

ws.onerror = function(error) {
    console.error("Erro de rede no WebSocket:", error);
};

ws.onclose = function() {
    console.warn("Aviso: Conexão WebSocket encerrada.");
};

// ==========================================
// 2. ATUALIZAÇÃO DA INTERFACE (DOM)
// ==========================================
function updateDashboard(packet) {
    // Referências das zonas criadas no novo HTML
    const normalZone = document.getElementById('dashboard-normal');
    const criticalZone = document.getElementById('dashboard-critical');
    const criticalWrapper = document.getElementById('zone-critical-wrapper');
    
    const nodeId = `node-${packet.node_id}`;
    let card = document.getElementById(nodeId);

    // 1. Criação do Card (Apenas na primeira vez que o nó é visto)
    if (!card) {
        card = document.createElement('div');
        card.id = nodeId;
        // O onclick abre o modal de detalhes
        card.onclick = function(e) {
            // Ignora o clique se o usuário clicou nos botões de controle
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
                    <span style="font-size: 0.9rem; color: #555;"><strong>Balanceador de Carga</strong></span>
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
        
        // CORREÇÃO: Anexa o card provisoriamente na zona normal para que
        // os 'document.getElementById' da Etapa 2 consigam encontrá-lo.
        normalZone.appendChild(card);
    }

    // 2. Atualização dos Dados Básicos
    document.getElementById(`temp-${packet.node_id}`).innerText = Number(packet.temperature).toFixed(1);
    document.getElementById(`stress-${packet.node_id}`).innerText = Number(packet.stress).toFixed(0);
    document.getElementById(`latency-${packet.node_id}`).innerText = Number(packet.latency).toFixed(0);

    // 3. Ordenação Dinâmica por Zonas (Movimentação FÍSICA do DOM)
    const badge = document.getElementById(`badge-${packet.node_id}`);
    
    // Se for Crítico (2) ou Alto (1), vai para a zona superior de alerta
    if (packet.current_state === 2 || packet.current_state === 1) {
        if (packet.current_state === 2) {
            card.className = 'sensor-card critical';
            badge.innerText = 'CRÍTICO';
        } else {
            card.className = 'sensor-card warning';
            badge.innerText = 'ALTO';
        }
        
        // Move o card para a zona crítica se ele já não estiver lá
        if (card.parentElement !== criticalZone) {
            criticalZone.appendChild(card);
        }
    } else {
        // Se for Normal (0), vai para a zona de baixo
        card.className = 'sensor-card normal';
        badge.innerText = 'NORMAL';
        
        // Move o card para a zona normal se ele já não estiver lá
        if (card.parentElement !== normalZone) {
            normalZone.appendChild(card);
        }
    }

    // Oculta a Zona Crítica inteira (título e borda) se não houver nenhum nó em pane
    if (criticalZone.children.length > 0) {
        criticalWrapper.style.display = 'block';
    } else {
        criticalWrapper.style.display = 'none';
    }

    // 4. Sincronização Absoluta dos Atuadores (A Fonte da Verdade)
    // Atualiza o Switch do LB (Se true, ativa o slider indicando drenagem)
    const lbSwitch = document.getElementById(`lb-switch-${packet.node_id}`);
    if (lbSwitch.checked !== packet.lb_active) {
        lbSwitch.checked = packet.lb_active;
    }

    // Atualiza o Controle Segmentado do HVAC (Limpa as cores e colore apenas o ativo)
    document.getElementById(`hvac-0-${packet.node_id}`).className = '';
    document.getElementById(`hvac-1-${packet.node_id}`).className = '';
    document.getElementById(`hvac-2-${packet.node_id}`).className = '';
    
    if (packet.hvac_state === 0) {
        document.getElementById(`hvac-0-${packet.node_id}`).className = 'active-off';
    } else if (packet.hvac_state === 1) {
        document.getElementById(`hvac-1-${packet.node_id}`).className = 'active-bal';
    } else if (packet.hvac_state === 2) {
        document.getElementById(`hvac-2-${packet.node_id}`).className = 'active-max';
    }
}

// ==========================================
// DISPARADORES DE EVENTO
// ==========================================
function toggleLB(nodeId, isChecked) {
    const signal = isChecked ? 'trigger_on' : 'trigger_off';
    sendControl('lb', signal, nodeId);
}

function setHVAC(nodeId, levelSignal) {
    sendControl('hvac', levelSignal, nodeId);
}
// ==========================================
// 3. COMANDOS DE ATUAÇÃO FÍSICA (HTTP)
// ==========================================
function sendControl(type, signal, nodeId) {
    const payload = {
        type: type,             
        signal: signal,         
        target_node: parseInt(nodeId, 10) 
    };

    console.log(`[OUT] Disparando comando HTTP para API Gateway:`, payload);

    fetch('/api/control', {
        method: 'POST',
        headers: { 
            'Content-Type': 'application/json' 
        },
        body: JSON.stringify(payload)
    })
    .then(response => {
        if (!response.ok) {
            throw new Error(`Servidor Gateway retornou Status ${response.status}`);
        }
        console.log(`[IN] Comando [${type}:${signal}] confirmado pelo Nó ${nodeId}`);
    })
    .catch(err => {
        console.error(`[FALHA] Não foi possível atuar no Nó ${nodeId}:`, err);
        alert(`Falha de comunicação ao tentar enviar o comando para o Nó ${nodeId}. Verifique o console.`);
    });
}