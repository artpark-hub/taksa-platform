# -------- Build stage --------
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache make git protobuf

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make init && make all && go build -o device-management ./cmd/device-management

# -------- Runtime stage --------
FROM alpine:3.23

RUN apk add --no-cache ca-certificates wget gettext

# Create app user
RUN addgroup -S taksa && adduser -S taksa -G taksa

# Create directories with writable permissions
# /data is world-writable for host volume mount compatibility (docker-compose dev)
RUN mkdir -p /data /home/taksa && chown -R taksa:taksa /home/taksa && chmod 777 /data

WORKDIR /home/taksa

# Copy pre-built binary and configs from builder
COPY --from=builder --chown=taksa:taksa /app/device-management /home/taksa/device-management
COPY --from=builder --chown=taksa:taksa /app/configs ./configs

# Switch to non-root user
USER taksa

# Health check
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/ || exit 1

# Expose ports
EXPOSE 8000 9000

CMD ["/home/taksa/device-management", "-conf", "/home/taksa/configs"]
