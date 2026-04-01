# Estágio de Compilação
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
# Compila o binário estático do Nó de Hardware
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin_node ./cmd/hardware_node/main.go

# Estágio de Execução
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin_node .

CMD ["./bin_node"]