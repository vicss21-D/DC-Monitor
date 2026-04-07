# ==========================================
# MAKEFILE - GERENCIAMENTO DO CLUSTER
# ==========================================

# Variáveis
SERVER_CONTAINER = dc_server
CSV_FILE = logs.csv

.PHONY: help up down build restart logs logs-server logs-size logs-lines logs-header clean

# Mostra essa ajuda por padrão se você digitar apenas 'make'

help:
	@echo "Comandos disponíveis:"
	@echo "  make up          	- Inicia os contêineres em segundo plano"
	@echo "  make down        	- Para e remove os contêineres"
	@echo "  make build       	- Recompila o código Go e reinicia os contêineres"
	@echo "  make restart     	- Reinicia todos os contêineres rapidamente"
	@echo "  make logs        	- Mostra os logs de todos os contêineres ao vivo"
	@echo "  make logs-server 	- Mostra apenas os logs do Servidor Central ao vivo"
	@echo "  make logs-size    	- Verifica o tamanho do arquivo de telemetria no disco"
	@echo "  make logs-lines   	- Conta quantas linhas de dados foram gravadas no CSV"
	@echo "  make logs-header   - Mostra o cabeçalho do arquivo CSV"
	@echo "  make clean       	- Para os contêineres e limpa volumes/redes orfãos"

# Comandos de Ciclo de Vida
up:
	docker compose up -d

down:
	docker compose down

build:
	docker compose up -d --build

restart:
	docker compose restart

# Comandos de Observabilidade
logs:
	docker compose logs -f

logs-server:
	docker logs -f $(SERVER_CONTAINER)

# Comandos de Inspeção de Dados (Persistência)

logs-lines:
	docker exec -it $(SERVER_CONTAINER) wc -l $(CSV_FILE)

logs-header:
	docker exec -it $(SERVER_CONTAINER) head -n 10 $(CSV_FILE)

logs-size:
	docker exec -it $(SERVER_CONTAINER) wc -c $(CSV_FILE)

# Limpeza profunda
clean:
	docker compose down --remove-orphans