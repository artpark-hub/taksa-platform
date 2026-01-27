FROM golang:1.24 AS builder
COPY . /src
WORKDIR /src
RUN go env -w GOPROXY=direct
RUN go mod download
RUN go build -o /app/telemetry-service ./cmd/telemetry-service

FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates netbase && rm -rf /var/lib/apt/lists/
WORKDIR /app
COPY --from=builder /app/telemetry-service /app/telemetry-service
COPY configs /app/configs
CMD ["./telemetry-service", "-conf", "/app/configs/config.yaml"]
