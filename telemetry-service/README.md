# Telemetry Service (NATS -> Postgres worker)

This service consumes UMH telemetry messages from NATS/JetStream and persists them into Postgres.

Prerequisites
- NATS / JetStream accessible to the process
- Postgres database reachable by the configured DSN

Quick run (local config path)
```bash
go build -o ./bin/ ./...
./bin/telemetry-service -conf ./configs
```

Docker
```bash
# build image
docker build -t telemetry-service:latest .

# run container (example)
docker run --rm -v $(pwd)/configs:/app/configs telemetry-service:latest
```

Configuration
Place configuration files under `configs/` (the Docker image uses `/app/configs`). The service reads `server` and `data` sections — specifically `data.database` (driver, source) and `data.nats` (url, subject, queue_group).

If you want to test ingestion, ensure `data.nats.subject` matches the UMH subject (e.g. `umh.>`) and `data.database.source` points to a writable Postgres instance.

Development
- Generate internal protos: `make init && make config`
- Build: `make build`

Telemetry service behavior
- Subscribes to the configured NATS subject (supports wildcards)
- Uses a queue group when `queue_group` is set to enable work-sharing across replicas
- Uses JetStream when available and falls back to plain NATS subscriptions

