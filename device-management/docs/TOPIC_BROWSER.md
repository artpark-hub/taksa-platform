# Device topic browser (UNS catalog in DM)

This document describes how Device Management (DM) exposes UNS topic browser data through APIs and persistence. The design **does not require changes to umh-core or the edge**: DM reads whatever device **status** messages already include under `core.topicBrowser`, materializes rows in PostgreSQL, and serves list/detail RPCs aligned with umh-core GraphQL semantics.

---

## Goals

- Give API consumers a **stable, queryable** topic catalog per device (filters, pagination, last-event shape).
- **Parity** with umh-core GraphQL for canonical topic strings, `uns_tree_id` hashing, text/metadata filters, and default batch limits.
- **No new edge contracts**: ingestion uses only fields present on the status wire format today.

---

## High-level architecture

```mermaid
flowchart TB
  subgraph edge["Edge / umh (unchanged)"]
    HB["Status JSON\nstatus / StatusMessage / status-message"]
  end

  subgraph dm["Device Management"]
    PUSH["Instance push handler\n`internal/biz/instance.go`"]
    SYNC["`syncDeviceTopicsFromStatusMessage`\n`internal/biz/instance_device_topics_sync.go`"]
    MERGE["`topicbrowser.MergeFromStatusMessageContent`\n`internal/topicbrowser/`"]
    REPO["`DeviceTopicRepo`\n`internal/data/device_topic.go`"]
    DB[("PostgreSQL `device_topics`")]
    SVC["`DeviceMgmtService`\n`internal/service/device_topics.go`"]
    API["ListDeviceTopics / GetDeviceTopic"]
  end

  HB --> PUSH
  PUSH --> SYNC
  SYNC --> MERGE
  MERGE --> REPO
  REPO --> DB
  DB --> REPO
  REPO --> SVC
  SVC --> API
```

**Summary:** Status batches trigger topic sync alongside protocol converters and stream processors. The merge step decodes protobuf `UnsBundle` frames from JSON, merges maps and events, then either **replaces** the whole device catalog, **clears** it, or **upserts** incrementally. HTTP/gRPC handlers read from `device_topics` only.

---

## Protobuf layout

| Artifact | Path | Purpose |
|----------|------|---------|
| Wire types | `api/uns/v1/topic_browser.proto` | `UnsBundle`, `TopicMap`, `TopicInfo`, `EventTable`, payloads — aligned with umh-core topic browser protos. |
| Public API | `api/devicemgmt/v1/device_topics.proto` | `DeviceTopic`, `ListDeviceTopics*`, `GetDeviceTopic*`, filters, last-event messages. |
| RPC registration | `api/devicemgmt/v1/devicemgmt.proto` | `ListDeviceTopics`, `GetDeviceTopic` on `DeviceMgmtService`. |

---

## HTTP surface (Google API annotations)

| Operation | Method | Path |
|-----------|--------|------|
| List topics | `POST` | `/api/v1/devicemgmt/devices/{device_id}/topics/list` |
| Get one topic | `GET` | `/api/v1/devicemgmt/devices/{device_id}/topics/detail` |

List uses **POST** so the body can carry `text`, `meta[]`, and pagination without long query strings. Get uses **query parameters** `unsTreeId` and/or `canonicalTopic` (one is required). Topic Browser HTTP JSON uses **camelCase** (proto default JSON names), consistent with `ListDevicesResponse` and other DM response bodies.

---

## Status payload contract (ingestion)

DM parses JSON with tolerant key casing, for example:

```json
{
  "core": {
    "topicBrowser": {
      "topicCount": 42,
      "unsBundles": {
        "0": "<base64 UnsBundle protobuf>",
        "1": "<base64 UnsBundle protobuf>"
      }
    }
  }
}
```

- `unsBundles` is a map from **stringified integer index** → **base64-encoded** `UnsBundle` (`proto.Unmarshal` after decode).
- Bundles are processed in **ascending numeric key order** so ordering matches umh-core framing.
- `topicCount` is read for **merge semantics** (see below); parsers accept common JSON number shapes and string digits when needed.

---

## Merge semantics (`MergeResult`)

Implemented in `internal/topicbrowser/merge.go`.

| Condition | `FullCatalogReplace` | Database effect |
|-----------|---------------------|-----------------|
| `topicCount == 0` | `true` | **Clear** all `device_topics` rows for that tenant + device (authoritative empty catalog). |
| `unsBundles["0"]` decodes and `len(uns_map.entries) == topicCount` with `topicCount > 0` | `true` | **Replace all** rows (umh-core bootstrap full cache snapshot). |
| Otherwise | `false` | **Upsert** merged rows only; **no deletes** of topics missing from the payload. |

**Event merging:** For each `uns_tree_id`, the event with the greatest `produced_at_ms` wins across bundles.

**Topic map merging:** Later bundles overwrite earlier entries for the same map key.

---

## Full-catalog heuristic: limitations and producer invariants

The condition on **bundle index `0` only** (`len(uns_map.entries) == topicCount`) is **not** a proof that the map is complete in all edge cases, but it matches umh-core’s bootstrap snapshot shape. It is a **heuristic** that matches the “cache bundle” shape in umh-core: equality is only safe if producers maintain an invariant along the lines of:

> Whenever a status message includes `topicCount` and an `uns_map` whose entry count equals that number, that map is the **authoritative full** UNS topic set for the device at that time.

DM does **not** verify topic-by-topic that the map matches every bridge or data source; it trusts that invariant when deciding `FullCatalogReplace`.

### Topology changes with the same total count (e.g. bridge swap)

If one bridge is removed and another is added such that the **total** topic count stays **N**:

- A **true full snapshot** in one message (one bundle’s map has all current topics, `topicCount == N`, `len(entries) == N`) → replace is **correct**: DM drops removed-bridge rows and reflects the new catalog.
- **Incremental-only** traffic (partial maps, none of which alone has `len(entries) == topicCount`) → DM **upserts** only; rows for the removed bridge can **remain** until a later message qualifies for full replace or empty catalog (`topicCount == 0`).

Cardinality alone does not encode “swap”; **completeness** is what matters, and that is implied only by producer behavior, not by the number **N** itself.

### False positive (unsafe replace)

If some bundle **accidentally** satisfies `len(entries) == topicCount` but is **not** the full catalog (stale or wrong `topicCount`, wrong bundle, or a slice that happens to have the same size), DM may run **replace-all**. Persisted rows are built from the **merge of all bundles** in that status message (`internal/topicbrowser/merge.go`); if merged rows are still incomplete relative to the real edge catalog, DM’s table can **shrink incorrectly** relative to reality.

### False negative (stale removals)

The check is **per bundle**, not “combined size across bundles == `topicCount`”. If the full catalog is **split** across several bundles such that **no single** `uns_map` has `len(entries) == topicCount`, `FullCatalogReplace` stays false and DM only upserts — **removals** may not be applied from that message.

### Hardening (optional, mostly outside current “no wire change” scope)

Stricter behavior usually needs **explicit** signaling on the wire (for example a dedicated “full catalog” flag) or producer rules that guarantee when equality may fire. Tightening heuristics in DM alone (e.g. only trust equality when there is exactly one bundle) trades false positives against false negatives; choose based on how umh-core actually emits `topicCount` and `unsBundles`.

---

## Identity: canonical topic and `uns_tree_id`

- **Canonical topic** — Same construction as umh-core GraphQL `buildTopicName`: `level0`, `location_sublevels`, `data_contract`, optional `virtual_path`, `name`, joined with `.`. See `internal/topicbrowser/topicname.go` (`BuildCanonicalTopic`).
- **`uns_tree_id`** — xxhash over the same logical fields with NUL-terminated segments, **hex-encoded**, matching umh-core `hashUNSTableEntry`. See `internal/topicbrowser/hash.go` (`HashTopicInfo`).

Rows are unique on `(tenant_id, device_id, uns_tree_id)`; detail lookup can use either `uns_tree_id` or exact `canonical_topic`.

---

## Persistence

Table: `device_topics` (see `db/schema.postgres.sql`).

Stores canonical path, tree id, level metadata, `metadata_json`, optional serialized last event (`last_event_json` / `last_event_at`), and timestamps. List queries support:

- **`text`** — Case-insensitive substring on `canonical_topic` **or** the text serialization of `metadata_json` (GraphQL-style topic text filter).
- **`meta`** — AND of exact key/value pairs on metadata (`jsonb_each_text`).

Default page size **100**, maximum **500**. Responses include `totalCount` and `filteredCount`.

Table: `device_topic_catalog` — per-device sync metadata (`last_synced_at`, counts, `last_sync_mode`, `catalog_stale_warning` via API).

---

## API surface (DM-only enhancements)

| RPC | HTTP | Purpose |
|-----|------|---------|
| `ListDeviceTopics` | `POST .../topics/list` | Flat list + `text`, `meta`, `pathPrefix`, `omitLastEvent`, pagination |
| `GetDeviceTopic` | `GET .../topics/detail` | Single topic |
| `GetDeviceTopicCatalogStatus` | `GET .../topics/catalog-status` | Sync metadata and staleness hint |
| `EnsureDeviceStatusSubscription` | `POST .../status-subscription` | Queue edge `subscribe` (explicit; use when auto resubscribe is off) |
| `ListTopicNodes` | `POST .../topics/nodes/list` | Lazy tree children at `pathPrefix` |

---

## When sync runs

Topic sync is invoked from the instance usecase when processing push messages whose type is one of: `status-message`, `StatusMessage`, or `status` — the same branch that syncs protocol converters and stream processors (`internal/biz/instance.go`).

Status push requires an active edge **subscriber** (from a `subscribe` action delivered via **pull**). The edge emits status about once per second only while a subscriber exists (~5 minute TTL).

### Subscribe keepalive (device-management)

| Mechanism | When |
|-----------|------|
| **Login** | Always queues `subscribe`. |
| **Pull (auto)** | When `device_status_subscription.auto_resubscribe_status_messages` is **true** (default) and either persisted status is older than `status_heartbeat_stale_threshold` **or** `catalog_last_synced_at` is older than `catalog_stale_threshold` while `last_seen` is still fresh. |
| **EnsureDeviceStatusSubscription** | Explicit northbound API — always queues `subscribe` (use when auto is disabled or UI opens a device). |
| **GetDeviceHealth** (best-effort) | Same as pull auto when enabled. |

Config (`configs/config.yaml` or env):

- `device_status_subscription.auto_resubscribe_status_messages` — default **true**; set **false** for large fleets (e.g. 100+ DCDs with heavy status payloads); then call **EnsureDeviceStatusSubscription** per device that needs catalog sync.
- `TAKSA_DM_AUTO_RESUBSCRIBE_STATUS_MESSAGES` — `true` (default) or `false`; overrides YAML when set.
- `catalog_stale_threshold` / `status_heartbeat_stale_threshold` — default **120s**.

**Scale note:** Live subscription implies ~1 status push per second per device (full core snapshot, zstd when large). Auto resubscribe avoids stale catalog without renewing every device on a timer; it only queues `subscribe` when sync is actually stale.

---

## API examples

### List topics (filtered, paginated)

```http
POST /api/v1/devicemgmt/devices/dev-abc123/topics/list
Content-Type: application/json

{
  "deviceId": "dev-abc123",
  "pageSize": 50,
  "pageToken": "",
  "text": "Line1",
  "meta": [
    { "key": "line", "eq": "1" }
  ]
}
```

### Get topic by tree id

```http
GET /api/v1/devicemgmt/devices/dev-abc123/topics/detail?unsTreeId=a1b2c3d4e5f6...
```

### Get topic by canonical path

```http
GET /api/v1/devicemgmt/devices/dev-abc123/topics/detail?canonicalTopic=Enterprise.Site.Line1._historian.Temperature
```

---

## Consumer mental model

1. **API source of truth** is the **DM database** projection, not a live subscription to the edge.
2. The projection updates on **each qualifying status message** processed for that device.
3. **Topic removals** in DM appear only when the payload signals a **full replace** or **empty catalog** (`topicCount == 0`). Incremental upserts alone never delete stale topics; plan UI and integrations accordingly. Reliability of the `len(entries) == topicCount` signal is documented under **Full-catalog heuristic** above.

---

## Related files (quick index)

| Area | Location |
|------|----------|
| Merge + decode + hashing | `internal/topicbrowser/` |
| Sync orchestration | `internal/biz/instance_device_topics_sync.go` |
| DB access | `internal/data/device_topic.go` |
| HTTP/gRPC handlers | `internal/service/device_topics.go` |
| DDL | `db/schema.postgres.sql` (`device_topics`) |
| OpenAPI (generated / mirrored) | `openapi.yaml` (search `ListDeviceTopics`, `GetDeviceTopic`) |
