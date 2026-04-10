FROM golang:1.24 AS builder

COPY . /src
WORKDIR /src

RUN make init && make config && make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/bin/nats-data-collector /app/nats-data-collector
COPY --from=builder /src/configs /app/configs

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /app/configs

CMD ["./nats-data-collector", "-conf", "/app/configs/config_docker.yaml"]
