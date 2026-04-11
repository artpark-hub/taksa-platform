FROM alpine:3.23

RUN apk add --no-cache gettext

WORKDIR /app

# Binary and configs are pre-built by 'make build' before docker build
COPY bin/user-management /app/user-management
COPY configs /app/configs

EXPOSE 8083

CMD ["sh", "-c", "envsubst < /app/configs/config.yaml > /tmp/config_resolved.yaml && /app/user-management -conf /tmp/config_resolved.yaml"]
