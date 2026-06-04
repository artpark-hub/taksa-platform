# FAQ: UNS, Topic Browser, and tag discovery (umh-core)

Questions and answers for product, integration, and support teams. This document clarifies **what umh-core does and does not do** around the Unified Namespace (UNS), Topic Browser, and industrial **tag discovery**—especially when configuring Protocol Converters (bridges).

**Related technical docs**

| Topic | Document |
|-------|----------|
| DM topic catalog (API, merge, DB) | [TOPIC_BROWSER.md](./TOPIC_BROWSER.md) |
| UI integration against DM | [TOPIC_BROWSER_UI_GUIDE.md](./TOPIC_BROWSER_UI_GUIDE.md) |
| Edge actions (deploy/edit PC, subscribe) | [ACTION_MANAGEMENT.md](./ACTION_MANAGEMENT.md) |
| umh-core getting started | [taksa-edge-umh/umh-core/docs](https://github.com/united-manufacturing-hub/united-manufacturing-hub/tree/main/umh-core/docs) (upstream) |

---

## Concepts: UNS, topics, and tags

### What is the Unified Namespace (UNS)?

The UNS is the **event-driven data backbone** on an umh-core instance. Data that has passed through a bridge (Protocol Converter read flow) is published to Kafka (topic `umh.messages`) with a canonical address called a **UMH topic**, for example:

```text
umh.v1.enterprise.siteA.machine-1._raw.sensors.DB1.DW20
```

Consumers (Topic Browser, stream processors, external apps) read from this namespace rather than talking to each PLC directly.

### What is a “tag” in this stack?

In industrial systems, a **tag** is one measurable point (temperature, speed, a PLC address, an OPC UA node). In umh-core:

- The **protocol layer** uses the vendor identifier (e.g. `DB1.DW20`, OPC UA node id, Modbus register).
- The **UNS layer** exposes a **topic** built from `location_path`, `data_contract`, optional `virtual_path`, and `tag_name` (often copied from protocol metadata in the Tag Processor).

“Tag discovery” in conversation often means **either** browsing the device address space **or** seeing names appear in Topic Browser—they are **not the same mechanism**.

### What is a UMH topic vs protocol metadata?

| Layer | Example | Who creates it |
|-------|---------|----------------|
| Protocol metadata | `opcua_tag_name`, `s7_address`, `modbus_address` | benthos-umh input on read |
| UMH topic | `umh.v1.enterprise…._raw.Temperature` | Tag Processor + UNS output |

Topic Browser indexes **UMH topics** and attached **metadata** (including original protocol fields). It does not replace an OPC UA address-space browser.

---

## Topic Browser: what it is

### What is the Topic Browser in umh-core?

A **downstream catalog** of what is already flowing in the UNS:

1. Bridges publish to Kafka.
2. A dedicated **Topic Browser** Benthos service consumes `umh.messages`, runs the `topic_browser` processor, and writes structured protobuf bundles to stdout.
3. umh-core caches those bundles and attaches them to **status messages** for subscribers (`core.topicBrowser.unsBundles`).

It answers: *“What topics exist on this instance right now, and what are their latest values/metadata?”*

### Does Topic Browser connect to OPC UA, S7, or Modbus?

**No.** Its input is only the UNS/Kafka consumer (`umh.v1.*` on `umh.messages`). It never opens `opc.tcp://…` or S7 connections.

### Does Topic Browser perform tag discovery on the PLC/server?

**No.** It performs **UNS topic discovery**: new topics appear when a bridge **publishes** them. Original tag names may show up in **metadata** (e.g. `opcua_tag_name`) only because the bridge put them there.

### How does Management Console or Taksa DM see Topic Browser data?

| Path | Behavior |
|------|----------|
| **umh Management Console** (closed source) | Subscribes to edge status; renders live tree from `topicBrowser` / bundles (same family as umh-core GraphQL). |
| **Taksa Device Management** | Ingests `core.topicBrowser` from status pushes, materializes **PostgreSQL** `device_topics`, serves list/detail/tree APIs. See [TOPIC_BROWSER.md](./TOPIC_BROWSER.md). |

Neither path browses the plant network for tags; both depend on **status heartbeats** after data is (or was) flowing.

### Is there a GraphQL API on the edge?

Yes, when enabled (`agent.graphql.enabled`, default port **8090**). Queries read the same **Topic Browser cache** as status—not the OPC UA server. See umh-core docs: `docs/reference/http-api/topic-browser-graphql.md`.

### What assumptions should we state about Topic Browser freshness?

- The edge sends status about **once per second** only while a **subscriber** exists (`subscribe` action, ~5 minute TTL).
- Taksa DM can **auto-resubscribe** when catalog or heartbeat is stale; large fleets may disable that and call `EnsureDeviceStatusSubscription` per device. See [TOPIC_BROWSER.md](./TOPIC_BROWSER.md).
- DM’s database is a **projection**, not a live stream to the DCD; lag equals last processed status message.

---

## Tag discovery: protocol layer (bridges)

### Where does real OPC UA “tag discovery” happen?

Inside the **bridge read Data Flow Component**, in **benthos-umh** `opcua` input:

- You configure one or more root **`nodeIDs`** (often folders).
- On start, the plugin **browses** the server (Organizes references, depth limits, server profiles) and **subscribes** to discovered variables.
- Messages carry metadata such as `opcua_tag_name`, `opcua_tag_path`, `opcua_attr_nodeid`.

This runs in the **same process as production reading**, not in the Topic Browser service.

### Does umh-core expose `get-opcua-tags` to browse tags before configuring a bridge?

**No (open-source umh-core, including latest upstream `main` / v0.44.18).**

- The action type `get-opcua-tags` exists in **model constants** (legacy Classic companion contract).
- There is **no handler** in `pkg/communicator/actions/actions.go`, no dedicated action file, and it is **not** listed in `supportedFeatures` sent to the console.

Taksa `models.proto` may define `GetOpcuaTagsPayload`; that does **not** mean the edge implements it.

### What other “discovery” actions are defined but not implemented on the edge?

Same pattern—constants only, no switch case:

| Action | Intended use (historical) |
|--------|-------------------------|
| `test-network-connection` | Reachability before deploy |
| `test-benthos-input` | Try input YAML without full PC |
| `deploy-connection` / `deploy-opcua-datasource` | Classic connection/datasource model |

**Implemented** today for Protocol Converters: `deploy-protocol-converter`, `edit-protocol-converter`, `get-protocol-converter`, `delete-protocol-converter`, plus logs/metrics/config.

### How do S7, Modbus, and Ethernet/IP handle tags?

| Protocol | Browse/discover on device? | Configuration |
|----------|----------------------------|---------------|
| **OPC UA** | Yes, under configured `nodeIDs` (folder browse + subscribe) | Root `nodeIDs` in read input YAML |
| **Siemens S7** | No | Explicit `addresses` list |
| **Modbus** | No | Explicit register/address list |
| **Ethernet/IP** | No (per benthos-umh docs) | Known tag names from PLC tooling |
| **Sparkplug B** | Tags from BIRTH/DBIRTH | MQTT + Sparkplug host mode |

Documentation sometimes says **“automatic tag mapping”**: protocol addresses become `tag_name` in the UNS **after** you configure what to read—not that the system lists all PLC tags for you in the UI.

### What does connection configuration (nmap) discover?

**TCP reachability** (IP/port open) for the Protocol Converter template. It does **not** list tags or OPC UA nodes.

---

## Protocol Converter configuration workflow

### What is the typical order of steps?

```text
1. Connection (IP, port)     → nmap / health on PC
2. Read flow (protocol input) → nodeIDs or addresses + tag_processor
3. Deploy / edit via actions → edge reconciles Benthos DFCs
4. Data flows to UNS         → Kafka
5. Topic Browser / DM catalog → topics visible (with lag)
```

Step 2 is where protocol-specific discovery runs (OPC UA browse) or where you **manually** list addresses (S7/Modbus).

### Can users pick OPC UA tags in the UI before deploying a read flow?

**Not via a supported umh-core discovery API.** Options in practice:

1. **Manual YAML** — enter `nodeIDs` / addresses (Taksa UI defaults are placeholders).
2. **External tools** — UA Expert, vendor exports, engineering documents.
3. **Deploy-then-inspect** — minimal read config → browse runs on edge → use Topic Browser (or DM APIs) to see resulting topics/metadata → refine `edit-protocol-converter`.

### After a fresh bridge deploy, when will Topic Browser show anything?

Only after:

- Read DFC is **active** and connected,
- Protocol input is **successfully** reading (OPC UA browse + subscribe completed for configured roots),
- Tag Processor publishes to UNS,
- Subscriber exists so status includes `topicBrowser` bundles.

Connection-only deploy (general tab, no read YAML) → **no topics**.

### Can Topic Browser help choose `nodeIDs` for OPC UA?

**Indirectly, after the fact:** metadata may include `opcua_attr_nodeid` / paths for topics already publishing. It does **not** return the full server tree for configuration.

---

## Taksa Device Management: capabilities and limits

### Does DM implement `get-opcua-tags`?

**No.** DM queues actions documented in [ACTION_MANAGEMENT.md](./ACTION_MANAGEMENT.md); there is no queue/API for `get-opcua-tags` in the current service layer.

### What does DM add on top of umh-core Topic Browser?

- Persistent **topic catalog** per device (`device_topics`).
- **List / detail / lazy tree** APIs with filters (text, metadata, path prefix).
- **Catalog status** and staleness warnings.
- Merge rules for full replace vs incremental upsert from status bundles. See [TOPIC_BROWSER.md](./TOPIC_BROWSER.md).

### When can DM’s topic list be wrong or stale?

| Situation | Effect |
|-----------|--------|
| Incremental status only (no full snapshot) | New topics appear; **removed** bridge topics may **remain** until full replace or `topicCount == 0` |
| False full-replace heuristic | Rare shrink if bundle 0 looks “complete” but is not authoritative |
| No status subscription | Catalog stops updating while edge may still run |
| Large fleet + auto-resubscribe off | Must explicitly `EnsureDeviceStatusSubscription` |

Details: **Full-catalog heuristic** in [TOPIC_BROWSER.md](./TOPIC_BROWSER.md).

---

## Common misconceptions (quick corrections)

| Misconception | Reality |
|---------------|---------|
| “Topic Browser discovers OPC UA tags on the server.” | Topic Browser discovers **UNS topics** from Kafka traffic. |
| “Updating our umh-core fork will add `get-opcua-tags`.” | Upstream **main** / **v0.44.18** also has no handler. |
| “Topic Browser runs before configure read.” | It runs **after** publish; needs a subscriber for status. |
| “Tag discovery = one feature in MC.” | Split: **protocol browse** (in bridge DFC) vs **UNS catalog** (Topic Browser / DM). |
| “S7 bridge auto-finds all DB addresses.” | You must list `addresses`; only mapping to tag names is automatic. |

---

## Assumptions to communicate externally

Use these when writing UX copy, RFPs, or customer-facing capability statements:

1. **UNS is the integration surface** for analytics and catalog APIs; PLCs are reached only through Protocol Converters.
2. **Tag configuration** for non-OPC-UA protocols requires **prior knowledge** of addresses/tags (or imports), not edge browse APIs.
3. **OPC UA** supports **folder-root browse** at runtime in the read pipeline; there is **no** separate pre-deploy browse API in open-source umh-core.
4. **Topic Browser** (and DM’s catalog) reflect **published** data; they are unsuitable as the sole step for greenfield tag picking unless you accept deploy-then-inspect.
5. **Catalog completeness** in DM depends on edge status semantics (`topicCount`, bundle 0 shape); incremental updates may retain obsolete topics until a full snapshot or empty catalog.
6. **Management Console** features not in open-source repos (e.g. closed-source browse UI) must be validated separately; do not infer them from `GetOpcuaTagsPayload` in protos alone.

---

## Decision guide: which mechanism do I need?

```text
Need to…                                    Use…
────────────────────────────────────────────────────────────────────────
Test IP/port reachable                      PC connection / nmap (not tags)
List OPC UA nodes before any deploy         External OPC UA client OR manual nodeIDs
Browse OPC UA under a folder at runtime     opcua input in read DFC (benthos-umh)
Map PLC address → UNS tag name              tag_processor (msg.meta.*_address → tag_name)
See what is live in UNS on edge             Topic Browser / GraphQL :8090
Query topics from cloud / console UI        Taksa DM ListDeviceTopics / ListTopicNodes
Remove stale topics from DM DB after bridge swap   Full status snapshot or topicCount=0
Implement pre-deploy tag picker via API     Not available in umh-core today; custom work
```

---

## Glossary

| Term | Meaning |
|------|---------|
| **Bridge / Protocol Converter** | Named edge component: connection probe + read (and optionally write) DFCs |
| **DFC** | Data Flow Component — Benthos pipeline (input / processors / output) |
| **Tag Processor** | JavaScript/bloblang processor setting `location_path`, `data_contract`, `tag_name` |
| **`_raw`** | Data contract with no schema validation |
| **`unsBundles`** | Protobuf batches in status (`topicBrowser`) for catalog sync |
| **`uns_tree_id`** | Stable hash id for a topic row (DM + GraphQL alignment) |
| **Subscriber** | Management client that triggered edge `subscribe` for status heartbeats |

---

## Open questions / future work (not commitments)

Items often requested but **not** in open-source umh-core today:

- `get-opcua-tags` (and related Classic connection/datasource actions)
- `test-benthos-input` for safe input trials
- REST `GET /api/v1/uns/tree` (mentioned in FAQ/roadmap docs, not in core codebase)
- Wire-level “full catalog” flag to harden DM replace semantics without heuristics

Product should treat these as **roadmap or custom development**, not current edge behavior.

---

## Document maintenance

| Check | When |
|-------|------|
| umh-core `actions.go` case list | After edge upgrades |
| `supportedFeatures` in status | After edge upgrades |
| DM merge rules | When changing `internal/topicbrowser/merge.go` |
| This FAQ | When customer-facing capability statements change |

*Last aligned with: umh-core upstream `main` / tag v0.44.18 (no `get-opcua-tags` handler); Taksa DM topic sync as documented in TOPIC_BROWSER.md.*
