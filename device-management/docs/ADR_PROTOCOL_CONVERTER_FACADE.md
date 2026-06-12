# ADR: Protocol converter facade APIs

> **UI integration:** [`OPC_UA_FACADE_UI_GUIDE.md`](./OPC_UA_FACADE_UI_GUIDE.md)

## Status

Accepted — implementation in progress.

## Context

Generic `protocol-converters` APIs expose the full umh-core `ProtocolConverter` model. The UI today assembles `readDFC`, `templateInfo`, and YAML client-side. We add typed facade APIs (OPC-UA, Modbus) in device-management with server-side build/parse/validate.

An OPC-UA protocol converter is ultimately a **Benthos pipeline** on the device (taksa-benthos-umh plugins), orchestrated by umh-core. The facade proto is a structured view of that pipeline, not a duplicate of every umh-core field.

See [BENTHOS_PIPELINE_MAPPING.md](./BENTHOS_PIPELINE_MAPPING.md) for the full facade → readDFC → Benthos mapping.

## Decisions

### API layers

| Layer | Purpose |
|-------|---------|
| Typed facade (`/protocol-converters/opc-ua`, `/modbus`) | Product UI — structured + raw YAML per section |
| Generic (`/protocol-converters`) | Power users, automation, full umh-core payload |

List and delete remain generic. Inventory is sync from DB + StatusMessage; full config is always fetched async from the DCD.

### Deploy orchestration

- Facade **POST deploy** returns one **workflow `action_id`**.
- Server queues `deploy-protocol-converter` (shell), then on success `edit-protocol-converter` (read flow).
- Edit failure triggers `delete-protocol-converter` rollback (best effort).
- Facade **PATCH edit** queues a single `edit` action (no workflow).
- Deploy is **create-only** (`409` if name/uuid already exists for tenant+device).

### Workflow status vs stage

Workflow **status** reuses `ActionStatus` values the UI already polls (`QUEUED`, `PROCESSING`, `COMPLETED`, `FAILED`, `CANCELLED`, `EXPIRED`).

**Stage** is an extra field only when status is `PROCESSING`:

| Stage | Meaning |
|-------|---------|
| `DEPLOYING` | Deploy child in flight |
| `CONFIGURING` | Edit/configure child in flight |
| `ROLLING_BACK` | Delete child in flight after configure failed |

Child actions keep the full DM lifecycle (`QUEUED` → `DELIVERED` → `PROCESSING` → terminal). Stage is workflow-only metadata, not a replacement for action states.

`rollback_status`: `CLEAN` (delete succeeded) or `ORPHAN_SHELL` (delete failed; bridge may remain on device).

### GET (typed)

- Always **async**: queue `get-protocol-converter`, return child `action_id`.
- Poll `GET .../opc-ua/actions/{action_id}/result` for typed body.
- Response includes **raw YAML** (verbatim from device) + **structured** (parsed) + `parse_status` per section.
- Response includes umh-core **`processing_mode`**, **`protocol`**, and **`state`** when present on GET payload.
- Wrong protocol kind on typed GET → `400` / `FAILED_PRECONDITION`.
- No DB cache of YAML as source of truth.

### Dual-mode submit

Per section (`input`, `processor`, `buffer`): `mode` = `STRUCTURED` | `RAW`. Server uses one source per section; both set without `mode` → `400`.

### OPC-UA structured contract (v1)

Aligned with **taksa-benthos-umh** `opcua_plugin` and `tag_processor_plugin`:

- Input: standard (endpoint, node IDs, subscribe/poll/heartbeat) + advanced (profile, security, certs, connection tuning).
- YAML key for server profile is **`profile`** (not `serverProfile`).
- Poll mode uses **`poll_rate`** (ms) → `pollRate`; there is no `subscribeInterval` plugin field.
- Processor: defaults, tag mappings, conditions, `advanced_processing`.
- Inject (`yaml_inject`): buffer and optionally cache/rate_limit resources (`readDFC.rawYAML`).
- **Downsampler** is runtime-injected by umh-core as a separate pipeline processor — not in structured facade v1.
- **UNS output** is runtime-injected — `output` on GET is read-only info only.
- Desired bridge **`state`**: `active` (default) or `stopped` on deploy/edit.

### Extension without proto changes

- `input.raw_yaml`, `read_flow.raw_processor_yaml`, `read_flow.raw_buffer_yaml`
- `additional_settings` on input for any opcua plugin key

### Out of scope v1

- Preview endpoint (UI generates YAML locally until added).
- Pre-deploy OPC UA browse (`GetOPCUATags`).
- `writeDFC`.
- Relational / custom multi-processor bridges.
- DB persistence of YAML blobs.

## References

- [ACTION_MANAGEMENT.md](./ACTION_MANAGEMENT.md)
- [BENTHOS_PIPELINE_MAPPING.md](./BENTHOS_PIPELINE_MAPPING.md)
- [FAQ_UNS_TOPIC_BROWSER_AND_TAG_DISCOVERY.md](./FAQ_UNS_TOPIC_BROWSER_AND_TAG_DISCOVERY.md)
- Golden fixtures: `testdata/protocolconverter/fixtures/`
- benthos-umh: `docs/input/opc-ua-input.md`, `docs/processing/tag-processor.md`
