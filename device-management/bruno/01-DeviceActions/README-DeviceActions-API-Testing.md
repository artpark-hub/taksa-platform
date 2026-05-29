# Device Actions — Bruno Testing Guide

Async actions (config, logs, metrics, deploy/edit/delete components) share the same lifecycle in device-management (DM). This folder tests queue-and-poll flows plus **cancellation**.

See also: [Action Management](../../docs/ACTION_MANAGEMENT.md) for the full state machine, expiry, and background cleanup.

## Endpoints in this folder

| File | Purpose |
|------|---------|
| `01-GetDeviceConfig.bru` | Queue get-config action |
| `02-SetDeviceConfig.bru` | Queue set-config action |
| `03-GetLogs.bru` | Queue get-logs action |
| `04-GetMetrics.bru` | Queue get-metrics action |
| `05-GetActionResult.bru` | Poll config action result |
| `06-QueueActionForCancel.bru` | Queue action for cancel flow |
| `06-CancelQueuedAction.bru` | **Cancel API** — QUEUED → CANCELLED |
| `06a-GetActionResult-AfterCancel.bru` | Verify CANCELLED via poll |
| `06b-CancelAction-AlreadyCancelled.bru` | Negative: second cancel → 400 |

## Cancel action flow

Run in order (device must **not** pull between queue and cancel):

```
1. 06-QueueActionForCancel.bru   → sets action_id_cancel
2. 06-CancelQueuedAction.bru     → POST .../actions/{action_id}/cancel
3. 06a-GetActionResult-AfterCancel.bru → status = CANCELLED
4. 06b-CancelAction-AlreadyCancelled.bru → expect 400
```

**Important:** If `Instance-Pull.bru` runs between steps 1 and 2, the action becomes `DELIVERED` and cancel will fail with `400 FailedPrecondition`.

## Action cleanup (background — no HTTP endpoint)

Terminal action **deletion** is performed by a background loop inside DM, not via a REST API. Configure with:

| Env var | Default | Purpose |
|---------|---------|---------|
| `DM_ACTION_RETENTION_MINUTES` | 60 | How long terminal rows are kept |
| `DM_ACTION_CLEANUP_INTERVAL_MINUTES` | 10 | How often the sweep runs |
| `DM_ACTION_AUTO_EXPIRE_MINUTES` | *(unset)* | Optional: mark stale `QUEUED` as `EXPIRED` |

There is no Bruno request for cleanup. To verify manually after retention elapses:

```sql
-- Should return 0 rows for old terminal action IDs after retention + one cleanup interval
SELECT id, status, completed_at FROM actions WHERE id = '<action_id_cancel>';
```

For fast local verification, set aggressive values in `.env`, restart DM, wait ≥ one cleanup interval:

```
DM_ACTION_RETENTION_MINUTES=2
DM_ACTION_CLEANUP_INTERVAL_MINUTES=1
```

## Auto-expire (optional)

When `DM_ACTION_AUTO_EXPIRE_MINUTES` is set, `QUEUED` actions older than that threshold become `EXPIRED` on the cleanup tick (not on queue).

Bruno flow to observe expiry:

1. Queue an action (`06-QueueActionForCancel.bru`) — do **not** cancel or pull.
2. Set `DM_ACTION_AUTO_EXPIRE_MINUTES=1` and restart DM.
3. Wait ~2 minutes (one interval + margin).
4. Poll with `06a-GetActionResult-AfterCancel.bru` (adjust URL/action id) — expect `EXPIRED`.

## Environment variables

```
base_url=http://localhost:8000
device_id=           # from RegisterDevice or default.bru
action_id_config=    # from GetDeviceConfig
action_id_cancel=    # from 06-QueueActionForCancel
```

## Status values when polling

| Value | Name | Meaning |
|-------|------|---------|
| 1 | QUEUED | Waiting for device pull |
| 2 | DELIVERED | Sent to device, awaiting reply |
| 3 | PROCESSING | Device reported in-progress |
| 4 | COMPLETED | Success |
| 5 | FAILED | Device reported failure |
| 6 | EXPIRED | TTL or auto-expire |
| 7 | CANCELLED | UI/API cancel |

JSON may return enum as integer or string depending on client/proto settings — tests accept both.
