FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin_lb ./cmd/actuators/load_balancer/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bin_lb .

EXPOSE 8082

CMD ["./bin_lb"]