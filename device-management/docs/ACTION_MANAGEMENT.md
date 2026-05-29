# Action Management in Device Management (DM)

This document describes how DM queues work for edge devices (DCD / umh-core), how actions change state, and how expiry, cancellation, and cleanup behave.

**Related:** [ENUM_REFERENCE.md](./ENUM_REFERENCE.md) (status integer values), [Bruno cancel tests](../bruno/01-DeviceActions/README-DeviceActions-API-Testing.md).

---

## Overview

Management APIs (northbound) **queue** actions in PostgreSQL. Edge devices (southbound) **pull** actions via `/v2/instance/pull` and **push** replies via `/v2/instance/push`. The UI polls `GetXActionResponse` endpoints until the action reaches a terminal status.

There are two paths into the system:

| Path | Who | Entry |
|------|-----|--------|
| **Queue** | Console / API | `GetDeviceConfig`, `DeployProtocolConverter`, etc. → `ActionQueuedResponse` |
| **Pull** | Edge device | JWT cookie → pending actions as UMH messages |
| **Push** | Edge device | Action replies, status messages, logs payloads |

---

## Action status state machine

```mermaid
stateDiagram-v2
    [*] --> QUEUED : API queues action

    QUEUED --> DELIVERED : Device pull (atomic claim)
    QUEUED --> CANCELLED : POST .../cancel (UI timeout)
    QUEUED --> EXPIRED : Per-action expires_at OR DM_ACTION_AUTO_EXPIRE_MINUTES

    DELIVERED --> PROCESSING : Push: action-confirmed / action-executing
    DELIVERED --> COMPLETED : Push: action-success (e.g. subscribe)
    DELIVERED --> FAILED : Push: action-failure
    DELIVERED --> FAILED_PARSING_RESPONSE : Push: unparseable reply

    PROCESSING --> COMPLETED : Push: action-success
    PROCESSING --> FAILED : Push: action-failure
    PROCESSING --> FAILED_PARSING_RESPONSE : Push: unparseable reply

    COMPLETED --> [*] : Retention cleanup deletes row
    FAILED --> [*]
    FAILED_PARSING_RESPONSE --> [*]
    EXPIRED --> [*]
    CANCELLED --> [*]
```

### Status reference

| Status | Code | Set by | Notes |
|--------|------|--------|-------|
| `QUEUED` | 1 | Queue API | Waiting for device pull |
| `DELIVERED` | 2 | Pull (claim) | Atomically claimed; message returned to device |
| `PROCESSING` | 3 | Push | Intermediate `action-confirmed` / `action-executing` |
| `COMPLETED` | 4 | Push | Terminal success |
| `FAILED` | 5 | Push | Terminal failure (`error_message` populated) |
| `EXPIRED` | 6 | TTL / auto-expire | Terminal; device must not receive |
| `CANCELLED` | 7 | Cancel API | Terminal; UI/user abort |
| `FAILED_PARSING_RESPONSE` | 8 | Push | Terminal; reply could not be parsed |

### Rules

- **Only `QUEUED`** may become `CANCELLED` or `EXPIRED` (via auto-expire sweep).
- **`DELIVERED` / `PROCESSING`** complete only through device push (`COMPLETED` / `FAILED` / `FAILED_PARSING_RESPONSE`).
- **DCDs never receive** `CANCELLED` or `EXPIRED` actions — pull uses an atomic claim (`UPDATE … WHERE status = QUEUED`).
- **Push replies are ignored** for actions already `CANCELLED` or `EXPIRED` (late stray replies do not overwrite terminal state).

---

## Pull: claim-on-deliver (race-safe)

Previous pattern (`ListPending` then `MarkDelivered`) allowed a cancel to land between list and deliver. Pull now:

1. **`ExpireQueuedPastDeadline`** — any `QUEUED` row with `expires_at < now` → `EXPIRED`.
2. **`ClaimQueuedForDevice`** — single SQL `UPDATE … SET status = DELIVERED WHERE status = QUEUED … RETURNING`.

If cancel or auto-expire wins the race, the row is no longer `QUEUED` and is **not** returned to the device.

**Subscribe actions** are special: after claim they are immediately marked `COMPLETED` on pull (edge handles subscribe without action-reply).

Implementation: `InstanceUsecase.PullMessages` in `internal/biz/instance.go`, storage in `internal/storage/postgres/action.go`.

---

## Push: correlating replies

Device replies include:

- **`metadata.traceId`** (primary) — stored in `action_message_tracking` at pull time.
- **`Payload.actionUUID`** (fallback) — equals DM action `id`.

Intermediate states update to `PROCESSING`. Terminal states set `completed_at` and optionally `error_message`.

Implementation: `PushMessages`, `correlateResponseByTraceId`, `correlateResponseByActionUUID` in `internal/biz/instance.go`.

---

## Cancellation (UI / API)

**Endpoint:**

```http
POST /api/v1/devicemgmt/devices/{device_id}/actions/{action_id}/cancel
Content-Type: application/json

{}
```

**Behavior:**

- Atomic: `UPDATE … WHERE status = QUEUED` → `CANCELLED`, sets `completed_at` and `error_message` (`"Cancelled by user"`).
- Returns **`400 FailedPrecondition`** if the action was already pulled or is terminal.
- UI should re-poll `GetXActionResponse` when cancel fails — often the device just pulled the action (`DELIVERED`).

**Bruno tests:** `bruno/01-DeviceActions/06-*.bru`

---

## Expiry

Two mechanisms:

### 1. Per-action TTL (`expires_at`)

When queueing, `TTLSeconds > 0` sets `expires_at` on the row. Before pull (and on each cleanup tick), overdue `QUEUED` rows → `EXPIRED` with message `"Per-action TTL exceeded"`.

### 2. Auto-expire queued actions (optional)

| Variable | Default | Effect |
|----------|---------|--------|
| `DM_ACTION_AUTO_EXPIRE_MINUTES` | **unset (disabled)** | When set, `QUEUED` actions with `created_at` older than N minutes → `EXPIRED` |

Use when devices may be offline for long periods and the UI does not cancel explicitly. **Not** the same as per-action `TTLSeconds`.

---

## Retention cleanup (background)

Terminal rows and old messages are **deleted** periodically. There is **no HTTP cleanup endpoint**.

| Variable | Default | Effect |
|----------|---------|--------|
| `DM_ACTION_RETENTION_MINUTES` | 60 | Delete terminal actions + messages older than this |
| `DM_ACTION_CLEANUP_INTERVAL_MINUTES` | 10 | Ticker interval between sweeps |

**Terminal statuses deleted:** `COMPLETED`, `FAILED`, `FAILED_PARSING_RESPONSE`, `CANCELLED`, `EXPIRED`.

**Timestamp used:** `COALESCE(completed_at, delivered_at, created_at)`.

**Note:** The loop runs on the ticker only — first deletion happens after **one full interval** following process start, not immediately at startup.

Each tick also runs per-action TTL expiry and (if configured) auto-expire before deletion.

Implementation: `StartActionCleanupLoop` in `internal/biz/action_cleanup.go`.

---

## Environment variables (summary)

```bash
# Retention / deletion
DM_ACTION_RETENTION_MINUTES=60
DM_ACTION_CLEANUP_INTERVAL_MINUTES=10

# Optional: auto-expire stale QUEUED (unset = off)
# DM_ACTION_AUTO_EXPIRE_MINUTES=30
```

Pass through Docker Compose (`docker-compose.yml`) or `.env` for local runs.

---

## UI polling guidance

1. Queue action → receive `action_id`.
2. Poll `GetXActionResponse` with exponential backoff.
3. Stop when status is terminal: `COMPLETED`, `FAILED`, `FAILED_PARSING_RESPONSE`, `EXPIRED`, `CANCELLED`.
4. If wait budget exceeded while still `QUEUED`, call **Cancel** then poll once more.
5. On cancel failure (`400`), poll again — device may have pulled the action.

Typical happy path timing:

```
QUEUED → (pull) → DELIVERED → (push confirmed) → PROCESSING → (push success) → COMPLETED
```

---

## Key source files

| Area | Path |
|------|------|
| Pull / push / PROCESSING | `internal/biz/instance.go` |
| Queue / cancel biz | `internal/biz/action.go` |
| Cleanup / auto-expire loop | `internal/biz/action_cleanup.go` |
| Storage (claim, cancel, expire) | `internal/storage/postgres/action.go` |
| Cancel RPC | `api/devicemgmt/v1/devicemgmt.proto`, `internal/service/devicemgmt.go` |
| Schema | `db/schema.postgres.sql` (`actions` table) |
