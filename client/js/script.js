// ==========================================
// VARIÁVEIS GLOBAIS
// ==========================================
let ws; // Declarado aqui em cima para o Auto-Heal funcionar
const alertTimeouts = {};
let activeDrawerNode = null; 
let historyChart = null;     
const MAX_HISTORY = 10;      
const nodeHistory = {};      

// ==========================================
// 1. INICIALIZAÇÃO DO WEBSOCKET (Com Auto-Heal)
// ==========================================
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsURL = `${protocol}//${window.location.host}/ws`;
    
    console.log("Tentando conectar ao Servidor Central...");
    ws = new WebSocket(wsURL);

    ws.onopen = function() {
        console.log("✅ Conexão WebSocket estabelecida com o Gateway.");
    };

    ws.onmessage = function(event) {
        try {
            const envelope = JSON.parse(event.data);
            
            // Verifica se é o nosso novo Padrão Envelope (Batch ou Critical)
            if (envelope.type === 'batch' || envelope.type === 'critical') {
                
                // O Payload agora é um Array com até 8 nós
                envelope.payload.forEach(packet => {
                    
                    // 1. Atualiza o Card na tela principal
                    updateDashboard(packet);
                    
                    // Pisca o painel se a mensagem chegou pela Via Rápida Crítica
                    if (envelope.type === 'critical') {
                        triggerVisualAlert(packet.node_id);
                    }

                    // 2. Registra no Histórico da Memória (Ignora nós Offline)
                    const id = packet.node_id;
                    if (!nodeHistory[id]) {
                        nodeHistory[id] = { temp: [], stress: [], labels: [] };
                    }
                    
                    if (packet.current_state !== -1) {
                        nodeHistory[id].labels.push('');
                        nodeHistory[id].temp.push(packet.temperature);
                        nodeHistory[id].stress.push(packet.stress);
                        
                        if (nodeHistory[id].temp.length > MAX_HISTORY) {
                            nodeHistory[id].labels.shift();
                            nodeHistory[id].temp.shift();
                            nodeHistory[id].stress.shift();
                        }
                    }
                    
                    // 3. Atualiza a Gaveta SOMENTE se estiver aberta para este nó
                    if (activeDrawerNode === id) {
                        if (packet.current_state === -1) {
                            document.getElementById('drawer-power').innerText = "-- W";
                            document.getElementById('drawer-latency').innerText = "-- ms";
                        } else {
                            document.getElementById('drawer-power').innerText = Number(packet.power_draw).toFixed(1) + " W";
                            document.getElementById('drawer-latency').innerText = Number(packet.latency).toFixed(1) + " ms";
                        }
                        
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

    ws.onerror = function(error) {
        console.error("⚠️ Erro de rede no WebSocket.");
        ws.close(); // Força o onclose a ser ativado para reconectar
    };

    // A MÁGICA DA RECONEXÃO ESTÁ AQUI
    ws.onclose = function() {
        console.warn("❌ Conexão perdida (Servidor parado). Tentando reconectar em 3 segundos...");
        
        // Pinta todo mundo de cinza enquanto o servidor não volta
        const badges = document.querySelectorAll('.badge');
        badges.forEach(b => { 
            b.innerText = 'OFFLINE'; 
            if(b.parentElement && b.parentElement.parentElement) {
                b.parentElement.parentElement.className = 'sensor-card offline'; 
            }
        });

        // Tenta rodar a função de conectar novamente após 3000ms
        setTimeout(connectWebSocket, 3000);
    };
}

// Dispara a conexão pela primeira vez quando a página carrega
connectWebSocket();

// ==========================================
// FUNÇÕES DA GAVETA E GRÁFICO
// ==========================================
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
            animation: false, 
            scales: { y: { beginAtZero: true, max: 110 } }
        }
    });
}

function openDetailsModal(nodeId) {
    activeDrawerNode = nodeId;
    document.getElementById('drawer-title').innerText = `Detalhes do Nó ${nodeId}`;
    document.body.classList.add('drawer-open');
    
    if (!historyChart) initChart();
}

function closeDetailsModal() {
    activeDrawerNode = null;
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

function triggerVisualAlert(nodeId) {
    const card = document.getElementById(`node-${nodeId}`);
    if (card) {
        card.classList.add('blinking-alert');
        
        if (alertTimeouts[nodeId]) {
            clearTimeout(alertTimeouts[nodeId]);
        }
        
        alertTimeouts[nodeId] = setTimeout(() => {
            card.classList.remove('blinking-alert');
        }, 1500);
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