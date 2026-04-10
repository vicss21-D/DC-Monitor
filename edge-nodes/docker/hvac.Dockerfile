FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin_hvac ./cmd/actuators/hvac/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/bin_hvac .

EXPOSE 8081

CMD ["./bin_hvac"]