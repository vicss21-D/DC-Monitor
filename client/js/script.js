// ==========================================
// 1. INICIALIZAÇÃO DO WEBSOCKET
// ==========================================
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsURL = `${protocol}//${window.location.host}/ws`;
const ws = new WebSocket(wsURL);

ws.onopen = function() {
    console.log("Conexão WebSocket estabelecida com o Gateway.");
};

ws.onmessage = function(event) {
    try {
        const packet = JSON.parse(event.data);
        //console.log("Pacote recebido:", packet);
        updateDashboard(packet);
    } catch (error) {
        console.error("Erro ao fazer parse do pacote de telemetria:", error);
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
    const dashboard = document.getElementById('dashboard');
    const nodeId = `node-${packet.node_id}`;
    let card = document.getElementById(nodeId);

    // Se o card não existe, cria a estrutura inteira UMA ÚNICA VEZ
    if (!card) {
        card = document.createElement('div');
        card.id = nodeId;
        card.className = 'sensor-card';
        
        card.innerHTML = `
            <h3>Nó de Hardware ${packet.node_id}</h3>
            <p><strong>Status:</strong> <span id="status-${packet.node_id}"></span></p>
            <p><strong>Temperatura:</strong> <span id="temp-${packet.node_id}"></span> °C</p>
            <p><strong>Estresse da CPU:</strong> <span id="stress-${packet.node_id}"></span> %</p>
            <p><strong>Potência:</strong> <span id="power-${packet.node_id}"></span> W</p>
            <p><strong>Latência:</strong> <span id="latency-${packet.node_id}"></span> ms</p>
            
            <div class="controls">
                <button class="btn-on" onclick="sendControl('hvac', 'trigger_on', ${packet.node_id})">Ligar HVAC</button>
                <button class="btn-off" onclick="sendControl('hvac', 'trigger_off', ${packet.node_id})">Desligar HVAC</button>
                <button class="btn-on" onclick="sendControl('lb', 'trigger_on', ${packet.node_id})">Drenar Tráfego</button>
                <button class="btn-off" onclick="sendControl('lb', 'trigger_off', ${packet.node_id})">Restaurar</button>
            </div>
        `;
        dashboard.appendChild(card);
    }

    // Atualiza a cor do card baseado no status
    let statusClass = 'status-normal';
    if (packet.current_state === 1) { 
        statusClass = 'status-warning';
    } else if (packet.current_state === 2) {
        statusClass = 'status-critical';
    }
    card.className = `sensor-card ${statusClass}`;

    // Atualiza APENAS o texto dos valores (os botões não são recriados)
    document.getElementById(`status-${packet.node_id}`).innerText = packet.current_state;
    document.getElementById(`temp-${packet.node_id}`).innerText = Number(packet.temperature).toFixed(2);
    document.getElementById(`stress-${packet.node_id}`).innerText = Number(packet.stress).toFixed(2);
    document.getElementById(`power-${packet.node_id}`).innerText = Number(packet.power_draw).toFixed(2);
    document.getElementById(`latency-${packet.node_id}`).innerText = Number(packet.latency).toFixed(2);
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