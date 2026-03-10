# -------- Build stage --------
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o user-service ./cmd/user-management

# -------- Runtime stage --------
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/user-service .
COPY --from=builder /app/configs ./configs

EXPOSE 8083
EXPOSE 9003

CMD ["./user-service", "-conf", "./configs/config.yaml"]
