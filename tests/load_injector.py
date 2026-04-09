# Script para injetar carga de teste no servidor de monitoramento
# Simula um nó em estado crítico enviando pacotes UDP continuamente

import socket, json, time

# Cria um socket UDP para comunicação
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

# Simula o Nó 5 em estado crítico (temperatura 45.5°C, stress 100%)
# Usado para testar a Via Crítica (emergência) do servidor
payload = {
    "id": 5, 
    "temperature": 45.5,  # Muito quente
    "stress": 100.0,       # CPU maxed out
    "latency": 5.0, 
    "current_state": 2,    # 2 = Estado Crítico (ativa auto-heal)
    "hvac_state": 0,       # HVAC desligado
    "lb_active": False     # Load Balancer inativo
}

# Envia 5000 pacotes continuamente para simular carga de teste
# Intervalo de 10ms = 100 pacotes/segundo = 5000 pacotes em 50 segundos
for _ in range(5000):
    sock.sendto(json.dumps(payload).encode(), ("192.168.1.7", 9000))
    time.sleep(0.01)  # 10ms entre pacotes

print("Fogo simulado no Nó 9!")