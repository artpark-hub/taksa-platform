FROM golang:1.24.6 AS builder
COPY . /src
WORKDIR /src
ARG GOPROXY=""
# If a GOPROXY is provided at build-time, set it; otherwise leave Go defaults intact.
RUN if [ -n "$GOPROXY" ]; then go env -w GOPROXY="$GOPROXY"; fi
RUN go mod download
RUN go build -o /app/telemetry-service ./cmd/telemetry-service

FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates netbase && rm -rf /var/lib/apt/lists/
WORKDIR /app
COPY --from=builder /app/telemetry-service /app/telemetry-service
COPY configs /app/configs
CMD ["./telemetry-service", "-conf", "/app/configs/config_docker.yaml"]
