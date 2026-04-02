# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Monorepo for the **Taksa Platform** — a microservices-based IoT/device management platform with three services:

- `device-management/` — Device registration, telemetry, and action distribution (Go/Kratos, HTTP :8000, gRPC :9000, SQLite or PostgreSQL)
- `user-management/` — User CRUD, JWT tokens, Ory Kratos identity integration (Go/Kratos, HTTP :8083, external PostgreSQL via Kratos)
- `ui-service/` — Web dashboard and auth flows (Next.js 16 / React 19 / TypeScript 5, port :3000)

There is no top-level Makefile or CI/CD configuration. Each service is built independently from its own directory.

## Commands

All `make` commands must be run from the respective service directory.

### device-management (Go 1.24)

```bash
# One-time setup
make init           # Install protoc, wire, kratos CLI, and other proto tools

# Code generation (run after modifying .proto files or adding DI providers)
make api            # Regenerate gRPC + HTTP stubs from .proto files
make config         # Regenerate config structs from internal proto
make wire           # Regenerate Wire dependency injection (wire_gen.go)
make generate       # All generation + go mod tidy
make all            # api + config + generate + wire + build

# Build
make build          # Build binary to ./bin/
make clean          # Remove build artifacts and data/ directory

# Docker
make docker-fresh   # Clean build + start container
make docker-init    # Initialize Docker environment
make docker-up / docker-down / docker-start / docker-stop
make docker-logs    # Tail container logs
make docker-clean   # Clean Docker resources
make reset          # Full clean reset (removes data/, containers, images)

# Tests (standard Go testing, no external framework)
cd device-management
go test ./...                             # All tests
go test ./internal/biz/...               # Business logic tests
go test ./internal/storage/sqlite/...    # Storage tests (uses in-memory SQLite)
go test -run TestName ./internal/biz/... # Single test
```

### user-management (Go 1.22)

```bash
make init       # Install proto tools
make api        # Regenerate API proto stubs
make config     # Regenerate config proto
make generate   # All generation + go mod tidy
make all        # api + config + generate (no wire step — wire uses go generate)
make build      # Build binary

cd user-management
go test ./...   # Run tests
```

**Key difference from device-management:** No `make wire` target. Wire generation is handled via `go generate` within the `make generate` step.

### ui-service (Node 20)

```bash
npm install     # Install dependencies
npm run dev     # Dev server with hot reload
npm run build   # Production build
npm run lint    # ESLint
npm start       # Start production server

# Docker (via Makefile)
make build      # Build Docker image
make up / make down / make restart
make logs       # Tail logs
make clean      # Clean Docker resources
```

## Architecture

### Backend Services (Kratos Layered Architecture)

Both Go services follow the same structural pattern:

```
cmd/<service>/        # Entry point, Wire injection (wire.go → wire_gen.go)
internal/
  service/            # HTTP/gRPC handlers — thin layer calling biz
  biz/                # Business logic, domain entities, use case interfaces
  data/               # Repository implementations (implements biz interfaces)
  storage/            # Low-level DB drivers (sqlite/, postgres/) implementing Store interface
  server/             # HTTP and gRPC server setup, middleware
  db/                 # Database connection factory
api/<service>/v1/     # .proto definitions → generated .pb.go files
configs/config.yaml   # Runtime configuration
db/                   # SQL schema files and migrations (device-management only)
```

**Dependency flow:** `service → biz → data → storage`

The `internal/data/` layer is a **repository pattern** on top of `internal/storage/`. The `data` layer handles domain logic translation; the `storage` layer handles raw SQL queries and schema.

### Device Management — Async Action Pattern

Device component operations (start, stop, configure) are **asynchronous**:
1. Client sends action request → receives `action_id`
2. Device polls `/v2/instance/pull` to receive queued actions
3. Device sends results via `/v2/instance/push`
4. Client polls for result using `action_id`

Direct CRUD operations (device registration, list, delete) are synchronous.

The `data/` directory at the service root is a **persistent volume mount** for SQLite data (mapped to `/data` in containers). The Makefile's `make clean` and `make reset` targets remove this directory.

### Proto API Structure

- `device-management/api/devicemgmt/v1/` — Management API (CRUD, health)
- `device-management/api/umh-core/v2/` — Device-facing API (register, login, push, pull)
- `device-management/api/common/` — Shared wire protocol models
- `user-management/api/tenants/v1/` — User/tenant management API

After modifying any `.proto` file, run `make api` (and `make config` for config protos) before building. Generated files are committed to the repo.

### User Management — Kratos Integration

Wraps an external **Ory Kratos** service for identity management:
- Admin API at `http://taksa-kratos:4434` (user creation/deletion)
- Public API at `http://taksa-kratos:4433` (auth flows)
- No embedded database — relies on Kratos and PostgreSQL configured via `TAKSA_DATABASE_DSN`

### UI Service

Next.js app with pages under `src/app/`. Routes include `/dashboard`, `/register`, `/recovery`, `/reset-password`.

## Configuration

- **device-management**: Copy `.env.example` to `.env` (vars prefixed `TAKSA_DM_*`). Edit `configs/config.yaml`. DB defaults to SQLite at `/data/taksa_platform_dm.db`; PostgreSQL also supported.
- **user-management**: Config uses envsubst — set `TAKSA_DATABASE_DSN` and Kratos URLs as environment variables. See `configs/config.yaml`.
- **ui-service**: Standard Next.js configuration.

## API Testing

`device-management/bruno/` contains Bruno collections for manual API testing. See `device-management/TESTING_QUICKSTART.md` for a walkthrough. Additional docs in `device-management/docs/`.
