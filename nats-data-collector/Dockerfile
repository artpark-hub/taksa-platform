FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        netbase \
        gettext \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

WORKDIR /app

# Binary and configs are pre-built by 'make build' before docker build
COPY bin/nats-data-collector /app/nats-data-collector
COPY configs /app/configs

EXPOSE 8000
EXPOSE 9000
VOLUME /app/configs

CMD ["sh", "-c", "envsubst < /app/configs/config.yaml > /tmp/config_resolved.yaml && /app/nats-data-collector -conf /tmp/config_resolved.yaml"]
