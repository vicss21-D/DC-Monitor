# ==========================================
# ESTÁGIO 1: A Fábrica (Builder)
# ==========================================
FROM golang:alpine AS builder

WORKDIR /app

# Copia os arquivos de gerenciamento de dependências
COPY go.mod go.sum ./

# Baixa as dependências com segurança
RUN go mod download

# Copia todo o código-fonte para dentro do contêiner
COPY . .

# Compila o binário estático do Nó de Hardware
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin_node ./cmd/hardware_node/main.go

# ==========================================
# ESTÁGIO 2: O Produto Final (Runner)
# ==========================================
FROM alpine:latest

WORKDIR /app

# Copia APENAS o binário pronto do estágio anterior
COPY --from=builder /app/bin_node .

# Comando de inicialização
CMD ["./bin_node"]