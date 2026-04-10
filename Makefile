# ==========================================
# MAKEFILE - ORQUESTRAÇÃO EDGE COMPUTING
# ==========================================

# Variáveis e Caminhos
SERVER_DIR = edge-server
NODES_DIR = edge-nodes

# O nome do contêiner do servidor backend conforme configurado no docker-compose.yml
SERVER_CONTAINER = dc_backend
CSV_FILE = logs.csv

.PHONY: help up-all down-all build-all up-server up-nodes down-server down-nodes logs-server logs-nodes logs-size logs-lines logs-header clean

# Mostra essa ajuda por padrão se você digitar apenas 'make'
help:
	@echo "============================================================"
	@echo "   Comandos de Orquestração - Data Center Gateway"
	@echo "============================================================"
	@echo ""
	@echo "=== CICLO DE VIDA GLOBAL ==="
	@echo "  make up-all        - Inicia Servidor e Borda (Nodes) simultaneamente"
	@echo "  make down-all      - Para e remove todos os contêineres do projeto"
	@echo "  make build-all     - Recompila tudo do zero (Go e Front-end) e inicia"
	@echo "  make clean         - Para todos os contêineres e limpa volumes/redes orfãos"
	@echo ""
	@echo "=== CONTROLE INDIVIDUAL (EDGE SERVER) ==="
	@echo "  make up-server     - Inicia apenas o Gateway Central"
	@echo "  make down-server   - Para apenas o Gateway Central"
	@echo "  make logs-server   - Mostra os logs do Gateway Go ao vivo"
	@echo ""
	@echo "=== CONTROLE INDIVIDUAL (EDGE NODES) ==="
	@echo "  make up-nodes      - Inicia apenas a Borda (Nginx, Sensores, Atuadores)"
	@echo "  make down-nodes    - Para apenas a Borda"
	@echo "  make logs-nodes    - Mostra os logs da Borda ao vivo"
	@echo ""
	@echo "=== INSPEÇÃO DE DADOS (CSV TELEMETRIA) ==="
	@echo "  make logs-size     - Verifica o tamanho do arquivo logs.csv no disco"
	@echo "  make logs-lines    - Conta quantas linhas de dados foram gravadas"
	@echo "  make logs-header   - Mostra o cabeçalho e as primeiras linhas do CSV"

# ==========================================
# COMANDOS GLOBAIS
# ==========================================
up-all: up-server up-nodes

down-all: down-server down-nodes

build-all:
	cd $(SERVER_DIR) && docker compose up -d --build
	cd $(NODES_DIR) && docker compose up -d --build

clean:
	cd $(SERVER_DIR) && docker compose down --remove-orphans
	cd $(NODES_DIR) && docker compose down --remove-orphans

# ==========================================
# COMANDOS DO SERVIDOR (GATEWAY)
# ==========================================
up-server:
	cd $(SERVER_DIR) && docker compose up -d

down-server:
	cd $(SERVER_DIR) && docker compose down

logs-server:
	cd $(SERVER_DIR) && docker compose logs -f

# ==========================================
# COMANDOS DOS NÓS DE BORDA (EDGE NODES)
# ==========================================
up-nodes:
	cd $(NODES_DIR) && docker compose up -d

down-nodes:
	cd $(NODES_DIR) && docker compose down

logs-nodes:
	cd $(NODES_DIR) && docker compose logs -f

# ==========================================
# COMANDOS DE PERSISTÊNCIA (EXECUÇÃO DENTRO DO CONTÊINER)
# ==========================================
logs-lines:
	docker exec -it $(SERVER_CONTAINER) wc -l $(CSV_FILE)

logs-header:
	docker exec -it $(SERVER_CONTAINER) head -n 10 $(CSV_FILE)

logs-size:
	docker exec -it $(SERVER_CONTAINER) wc -c $(CSV_FILE)