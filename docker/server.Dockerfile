# Estágio de Compilação
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
# Compila o binário estático do Servidor Central
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin_server ./cmd/server/main.go

# Estágio de Execução
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin_server .
# A pasta web será injetada via volume no docker-compose, não precisamos copiar aqui.

CMD ["./bin_server"]