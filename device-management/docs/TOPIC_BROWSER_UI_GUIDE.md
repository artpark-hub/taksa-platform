# Topic Browser APIs — UI developer guide

Guide for building a **management console** topic browser against Device Management (DM). Data is **materialized in PostgreSQL** from edge status (`core.topicBrowser`); the UI does not talk to the DCD directly for reads.

**Related:** [TOPIC_BROWSER.md](./TOPIC_BROWSER.md) (architecture, merge semantics, ops).

---

## Base URL and auth

| Item | Value |
|------|--------|
| Base URL | e.g. `https://<dm-host>:8000` (local: `http://localhost:8000`) |
| Auth | `Authorization: Bearer <console JWT>` |
| Content-Type | `application/json` for POST bodies |
| JSON field names | **camelCase** in request/response bodies and Topic Browser GET query params (e.g. `nextPageToken`, `pageSize`, `catalogLastSyncedAt`, `unsTreeId`), matching other Device Management JSON APIs |

The JWT must include **`tenant_id`** for the tenant that owns the device (from Oathkeeper / console login). Do **not** use the device southbound login cookie for these routes.

```http
GET /api/v1/devicemgmt/devices/{deviceId}/topics/catalog-status
Authorization: Bearer eyJhbGciOi...
```

`deviceId` in the path must match the device UUID you are viewing.

---

## Which API when (quick map)

```text
┌─────────────────────────────────────────────────────────────────┐
│  Open device topic browser screen                               │
└───────────────────────────────┬─────────────────────────────────┘
                                ▼
              GET .../topics/catalog-status  (freshness banner)
              POST .../status-subscription   (only if catalog stale / ops mode)
                                ▼
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│  Left: tree      │     │  Center: search  │     │  Right: detail   │
│  ListTopicNodes  │     │  ListDeviceTopics│     │  GetDeviceTopic  │
│  (lazy expand)   │     │  (table / filter)│     │  (one topic)     │
└──────────────────┘     └──────────────────┘     └──────────────────┘
```

| UI need | API | Notes |
|---------|-----|--------|
| “Is data fresh?” / loading state | `GetDeviceTopicCatalogStatus` | Cheap; call on screen open and after refresh |
| Expand ISA-95 tree | `ListTopicNodes` | Small payloads; one call per expanded path |
| Search / filter table | `ListDeviceTopics` | Full rows; use `omitLastEvent: true` for tables |
| Topic detail panel | `GetDeviceTopic` | Full metadata + last event |
| Force edge to send status again | `EnsureDeviceStatusSubscription` | When catalog stuck or auto-resubscribe disabled |

---

## 1. Catalog status (freshness)

Use this to drive banners: “syncing…”, “last updated …”, “N topics”.

### Request

```http
GET /api/v1/devicemgmt/devices/f0521328-915e-462d-8638-b6db8851624e/topics/catalog-status
Authorization: Bearer <token>
```

No body.

### Response (healthy, 2 topics)

```json
{
  "deviceId": "f0521328-915e-462d-8638-b6db8851624e",
  "catalogLastSyncedAt": "2026-05-22T09:05:06.834818Z",
  "reportedTopicCount": 2,
  "materializedTopicCount": 2,
  "lastSyncMode": "FULL_REPLACE",
  "lastFullReplaceAt": "2026-05-22T09:05:06.834818Z",
  "lastHadBundleZero": true,
  "catalogStaleWarning": false
}
```

### Field guide

| Field | UI use |
|-------|--------|
| `catalogLastSyncedAt` | “Last synced” timestamp; if old while device is online, show warning |
| `materializedTopicCount` | Badge count in header |
| `reportedTopicCount` | What edge last reported; can lag behind materialized briefly |
| `lastSyncMode` | `EMPTY` = last status said zero topics (catalog cleared); `FULL_REPLACE` / `INCREMENTAL` = normal |
| `catalogStaleWarning` | `true` if DB has more rows than edge last reported (stale removals possible) |

### UI logic (example)

```javascript
const STALE_MS = 2 * 60 * 1000; // align with DM default catalog_stale_threshold
const syncedAt = new Date(res.catalogLastSyncedAt);
const isStale = Date.now() - syncedAt.getTime() > STALE_MS;
if (isStale) showBanner('Topic catalog may be outdated. Refreshing…');
```

---

## 2. Ensure status subscription (optional refresh)

Queues an edge **`subscribe`** action so the DCD resumes **status push** (~1/s), which updates the catalog. Call when:

- User clicks **Refresh** on the topic browser, or
- `catalogLastSyncedAt` is stale and auto-resubscribe is disabled in DM config.

Default DM config re-subscribes automatically on pull when stale; this API is still useful for an explicit **Refresh** button.

### Request

```http
POST /api/v1/devicemgmt/devices/f0521328-915e-462d-8638-b6db8851624e/status-subscription
Authorization: Bearer <token>
Content-Type: application/json

{
  "deviceId": "f0521328-915e-462d-8638-b6db8851624e",
  "resubscribed": true
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `deviceId` | Yes | Same as path `deviceId` |
| `resubscribed` | No | Omit to let DM choose: **`false`** when catalog is empty / last sync was `EMPTY` (bootstrap bundle 0); otherwise **`true`** (TTL refresh). Set `false` explicitly to force full snapshot after a cleared catalog |

### Response

```json
{
  "actionId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "alreadyPending": false,
  "createdAt": "2026-05-22T09:10:00Z",
  "expiresAt": "2026-05-22T09:12:00Z"
}
```

| Field | UI use |
|-------|--------|
| `alreadyPending` | `true` → subscribe already queued; wait and poll catalog status |
| `actionId` | Optional display in debug; device pulls action, not the UI |

After calling, poll **`GetDeviceTopicCatalogStatus`** every few seconds until `catalogLastSyncedAt` advances.

---

## 3. Tree navigation — `ListTopicNodes`

Returns **one level** of the hierarchy under a path. Topics are sorted by canonical path; segments are derived from dot-separated paths (e.g. `Artpark.Artgarage.GroundFloor.Line1._historian.hello_world`).

### Request — root level

```http
POST /api/v1/devicemgmt/devices/f0521328-915e-462d-8638-b6db8851624e/topics/nodes/list
Authorization: Bearer <token>
Content-Type: application/json

{
  "deviceId": "f0521328-915e-462d-8638-b6db8851624e",
  "pathPrefix": []
}
```

### Response — root (example)

```json
{
  "pathPrefix": "",
  "nodes": [
    {
      "segment": "Artpark",
      "isLeaf": false,
      "descendantLeafCount": 2
    }
  ]
}
```

### Request — under `Artpark` → `Artgarage` → `GroundFloor`

```json
{
  "deviceId": "f0521328-915e-462d-8638-b6db8851624e",
  "pathPrefix": ["Artpark", "Artgarage", "GroundFloor"]
}
```

### Response — children (example)

```json
{
  "pathPrefix": "Artpark.Artgarage.GroundFloor",
  "nodes": [
    {
      "segment": "Line1",
      "isLeaf": false,
      "descendantLeafCount": 1
    },
    {
      "segment": "Line2",
      "isLeaf": false,
      "descendantLeafCount": 1
    }
  ]
}
```

### Request — leaf level (topic row)

When `isLeaf` is `true`, use `unsTreeId` and `canonicalTopic` for the detail panel (or call `GetDeviceTopic`).

```json
{
  "pathPrefix": "Artpark.Artgarage.GroundFloor.Line1._historian",
  "nodes": [
    {
      "segment": "hello_world",
      "isLeaf": true,
      "descendantLeafCount": 1,
      "unsTreeId": "eb329c21a444dfa1",
      "canonicalTopic": "Artpark.Artgarage.GroundFloor.Line1._historian.hello_world"
    }
  ]
}
```

### Optional filters (same as list)

```json
{
  "deviceId": "...",
  "pathPrefix": ["Artpark"],
  "text": "Line2",
  "meta": [{ "key": "tag_name", "eq": "hello_artpark" }]
}
```

| Field | Description |
|-------|-------------|
| `pathPrefix` | Segments already expanded (array), **not** a single dotted string |
| `text` | Substring match on canonical topic or metadata text |
| `meta` | Exact metadata key/value AND filters |

### Tree UI pattern

```javascript
async function loadChildren(deviceId, pathPrefixSegments) {
  const res = await post(`/devices/${deviceId}/topics/nodes/list`, {
    deviceId,
    pathPrefix: pathPrefixSegments,
  });
  return res.nodes.map((n) => ({
    id: [...pathPrefixSegments, n.segment].join('.'),
    label: n.segment,
    isLeaf: n.isLeaf,
    unsTreeId: n.unsTreeId,
    childCount: n.descendantLeafCount,
  }));
}
```

---

## 4. Flat list — `ListDeviceTopics`

Returns **full topic rows** (same shape as detail), paginated. Best for **search results** and **tables**, not for tree shells.

**Recommendation for tables:** set `"omitLastEvent": true` and load **`GetDeviceTopic`** when the user selects a row.

### Request — first page (compact table)

```http
POST /api/v1/devicemgmt/devices/f0521328-915e-462d-8638-b6db8851624e/topics/list
Authorization: Bearer <token>
Content-Type: application/json

{
  "deviceId": "f0521328-915e-462d-8638-b6db8851624e",
  "pageSize": 20,
  "pageToken": "",
  "omitLastEvent": true
}
```

### Response — page 1 of 2 (example)

```json
{
  "topics": [
    {
      "topic": "Artpark.Artgarage.GroundFloor.Line1._historian.hello_world",
      "unsTreeId": "eb329c21a444dfa1",
      "level0": "Artpark",
      "locationSublevels": ["Artgarage", "GroundFloor", "Line1"],
      "dataContract": "_historian",
      "name": "hello_world",
      "metadata": [
        { "key": "tag_name", "value": "hello_world" },
        { "key": "location_path", "value": "Artpark.Artgarage.GroundFloor.Line1" }
      ],
      "updatedAt": "2026-05-22T09:03:03.842267Z"
    }
  ],
  "nextPageToken": "1",
  "totalCount": 2,
  "filteredCount": 2
}
```

### Request — second page

`pageToken` is an **offset**, not a page number. `"1"` means “skip the first topic.”

```json
{
  "deviceId": "f0521328-915e-462d-8638-b6db8851624e",
  "pageSize": 1,
  "pageToken": "1",
  "omitLastEvent": true
}
```

### Response — page 2 (last)

```json
{
  "topics": [
    {
      "topic": "Artpark.Artgarage.GroundFloor.Line2._historian.hello_artpark",
      "unsTreeId": "4393d7af6f3ed0bb",
      "level0": "Artpark",
      "locationSublevels": ["Artgarage", "GroundFloor", "Line2"],
      "dataContract": "_historian",
      "name": "hello_artpark",
      "metadata": [
        { "key": "tag_name", "value": "hello_artpark" }
      ],
      "updatedAt": "2026-05-22T09:05:06.834818Z"
    }
  ],
  "nextPageToken": "",
  "totalCount": 2,
  "filteredCount": 2
}
```

Empty `nextPageToken` → end of list.

### Pagination cheat sheet

| Field | Meaning |
|-------|---------|
| `pageSize` | Items per page (default **100**, max **500**) |
| `pageToken` | Omit or `""` for first page; otherwise use previous `nextPageToken` |
| `totalCount` | All topics for device (ignores filters) |
| `filteredCount` | Topics matching `text` / `meta` / `pathPrefix` |

```javascript
async function listAllTopics(deviceId, filters) {
  const items = [];
  let pageToken = '';
  do {
    const res = await post(`/devices/${deviceId}/topics/list`, {
      deviceId,
      pageSize: 100,
      pageToken,
      omitLastEvent: true,
      ...filters,
    });
    items.push(...res.topics);
    pageToken = res.nextPageToken || '';
  } while (pageToken);
  return items;
}
```

### Filters

**Search box (`text`):** case-insensitive substring on canonical path or metadata.

```json
{
  "deviceId": "...",
  "text": "Line2",
  "omitLastEvent": true
}
```

**Metadata filter (`meta`):** exact key/value (repeat for AND).

```json
{
  "deviceId": "...",
  "meta": [{ "key": "tag_name", "eq": "hello_artpark" }]
}
```

**Path prefix (`pathPrefix`):** dotted string; topics under that path.

```json
{
  "deviceId": "...",
  "pathPrefix": "Artpark.Artgarage.GroundFloor.Line1.",
  "omitLastEvent": true
}
```

Note: `pathPrefix` here is a **string**, unlike `ListTopicNodes` which uses a **segment array**.

### List vs detail payload size

| | `ListDeviceTopics` | `GetDeviceTopic` |
|--|-------------------|------------------|
| Identity | `topic`, `unsTreeId`, levels, `name` | Same |
| Metadata | **Full** array from edge | **Full** |
| Last value | Optional (`omitLastEvent`) | Always included when stored |

There is no separate “summary” list type today. For a slim table, show `topic`, `name`, and 1–2 metadata keys (e.g. `tag_name`) client-side, then fetch detail on click.

---

## 5. Topic detail — `GetDeviceTopic`

### Request (by tree id — preferred)

```http
GET /api/v1/devicemgmt/devices/f0521328-915e-462d-8638-b6db8851624e/topics/detail?unsTreeId=eb329c21a444dfa1
Authorization: Bearer <token>
```

### Request (by canonical path)

```http
GET /api/v1/devicemgmt/devices/f0521328-915e-462d-8638-b6db8851624e/topics/detail?canonicalTopic=Artpark.Artgarage.GroundFloor.Line1._historian.hello_world
```

Provide **one** of `unsTreeId` or `canonicalTopic`, not both.

### Response (with last event — example shape)

```json
{
  "topic": "Artpark.Artgarage.GroundFloor.Line1._historian.hello_world",
  "unsTreeId": "eb329c21a444dfa1",
  "level0": "Artpark",
  "locationSublevels": ["Artgarage", "GroundFloor", "Line1"],
  "dataContract": "_historian",
  "name": "hello_world",
  "metadata": [
    { "key": "tag_name", "value": "hello_world" },
    { "key": "timestamp_ms", "value": "1779440580591" }
  ],
  "timeSeries": {
    "producedAt": "2026-05-22T09:03:03.842267Z",
    "scalarType": "STRING",
    "stringValue": "hello-world-1779440580591"
  },
  "updatedAt": "2026-05-22T09:03:03.842267Z"
}
```

Last event may be `timeSeries` or `relational` depending on the data contract. If nothing is stored, the oneof is omitted.

---

## Recommended screen flows

### A. Topic browser page load

1. `GET .../topics/catalog-status` → show count + last sync.
2. If stale → `POST .../status-subscription` → poll catalog status.
3. `POST .../topics/nodes/list` with `pathPrefix: []` → render tree root.

### B. User expands tree node

1. Append segment to `pathPrefix` array.
2. `POST .../topics/nodes/list` → render children.
3. If child `isLeaf` → on select, `GET .../topics/detail?unsTreeId=...`.

### C. User searches in a table

1. `POST .../topics/list` with `text` / `meta`, `omitLastEvent: true`, paginate with `nextPageToken`.
2. On row click → `GET .../topics/detail?unsTreeId=...`.

### D. User clicks Refresh

1. `POST .../status-subscription` with `resubscribed: true`.
2. Poll `GET .../topics/catalog-status` until `catalogLastSyncedAt` changes.
3. Re-fetch tree root or current list.

---

## HTTP status codes

| Code | Typical cause |
|------|----------------|
| `200` | Success |
| `400` | Invalid `pageToken`, missing `deviceId`, both `unsTreeId` and `canonicalTopic` set |
| `401` / `403` | Missing/invalid Bearer or wrong tenant |
| `404` | Unknown `deviceId` or topic not found on detail |
| `500` | Server error |

---

## JSON naming

HTTP JSON uses **camelCase** (e.g. `unsTreeId`, `nextPageToken`, `catalogLastSyncedAt`), matching gRPC-Gateway / OpenAPI output.

---

## OpenAPI

Generated spec: `device-management/openapi.yaml` — search for `ListDeviceTopics`, `ListTopicNodes`, `GetDeviceTopic`, `GetDeviceTopicCatalogStatus`, `EnsureDeviceStatusSubscription`.

---

## Bruno collection

Runnable examples: `device-management/bruno/05-TopicBrowser/` (set `base_url`, `device_id`, `console_bearer_token`).
