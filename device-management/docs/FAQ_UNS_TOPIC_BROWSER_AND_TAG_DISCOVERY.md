# FAQ: UNS, Topic Browser, and tag discovery (umh-core)

Questions and answers for product, integration, and support teams. This document clarifies **what umh-core does and does not do** around the Unified Namespace (UNS), Topic Browser, and industrial **tag discovery**—especially when configuring Protocol Converters (bridges).

**Related technical docs**

| Topic | Document |
|-------|----------|
| DM topic catalog (API, merge, DB) | [TOPIC_BROWSER.md](./TOPIC_BROWSER.md) |
| UI integration against DM | [TOPIC_BROWSER_UI_GUIDE.md](./TOPIC_BROWSER_UI_GUIDE.md) |
| Edge actions (deploy/edit PC, subscribe) | [ACTION_MANAGEMENT.md](./ACTION_MANAGEMENT.md) |
| umh-core getting started | [taksa-edge-umh/umh-core/docs](https://github.com/united-manufacturing-hub/united-manufacturing-hub/tree/main/umh-core/docs) (upstream) |
| **Official Topic Browser (UMH)** | [docs.umh.app — Topic Browser](https://docs.umh.app/usage/unified-namespace/topic-browser) |

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

“Tag discovery” in conversation often means **either** browsing the device address space (bridge read DFC) **or** seeing names appear in Topic Browser. Per [UMH documentation](https://docs.umh.app/usage/unified-namespace/topic-browser), Topic Browser is for **UNS exploration and verification**, not the former.

### What is a UMH topic vs protocol metadata?

| Layer | Example | Who creates it |
|-------|---------|----------------|
| Protocol metadata | `opcua_tag_name`, `s7_address`, `modbus_address` | benthos-umh input on read |
| UMH topic | `umh.v1.enterprise…._raw.Temperature` | Tag Processor + UNS output |

Topic Browser indexes **UMH topics** and attached **metadata** (including original protocol fields where bridges add them). It does **not** replace an OPC UA address-space browser or a tag-configuration wizard.

---

## Topic Browser: intended purpose (per UMH documentation)

Official definition: Topic Browser is for **real-time exploration of Unified Namespace data** via the [Management Console](https://management.umh.app) or a per-instance **GraphQL API** — not for discovering what exists on a PLC before a bridge is configured.

Source: [Topic Browser — docs.umh.app](https://docs.umh.app/usage/unified-namespace/topic-browser).

### What is Topic Browser meant for?

| Intended use | What you do in practice |
|--------------|-------------------------|
| **Explore the UNS** | Browse a hierarchical topic tree (ISA-95-style location → data contract → virtual paths → tag names). |
| **Find data** | Search by topic name or metadata; filter by contract, location, or keys like `data_contract`. |
| **Verify data flow** | Confirm a topic exists, see the **current value** and **Produced At** time, skim **history** (last values). |
| **Debug bridges** | Inspect **metadata** (e.g. `bridged_by`, `opcua_tag_name`, `s7_address`) to see which bridge produced a topic and trace delays. |
| **Programmatic access** | Query the same catalog on the edge via GraphQL (`:8090/graphql`) when enabled. |

Management Console (UMH Core version, not Classic) provides:

- **Left panel** — expandable tree (Enterprise → Site → Area → Line → Equipment), including `_raw` / model contracts and **virtual paths**.
- **Right panel** — location, data contract, latest value, metadata, and history (chart or table; **last 100 values** per topic).
- **Live updates** and **auto-aggregation** across connected UMH instances (no manual refresh for each message).

### What is Topic Browser *not* meant for?

| Not a substitute for | Why |
|----------------------|-----|
| **PLC / OPC UA tag discovery** | It never connects to `opc.tcp://…`, S7, or Modbus. It only reads what already reached the UNS. |
| **Pre-deploy bridge configuration** | Topics appear **after** read flows publish. Connection-only deploy shows nothing useful in the tree. |
| **Authoritative engineering tag database** | It reflects **runtime** namespace state, with caching, batching, and (in DM) projection lag — not a guaranteed full plant inventory. |

When documentation says “live topic discovery,” in the [Topic Browser sense](https://docs.umh.app/usage/unified-namespace/topic-browser) that means **new UNS topics appear in the tree as bridges publish them** — not that the edge browses the device address space for you.

### How does umh-core implement Topic Browser technically?

Behind the UI/API, on each instance:

1. Bridges publish to Kafka (`umh.messages`).
2. Internal **Topic Browser** service (`internal.topicbrowser.desiredState: active`) consumes UNS traffic via Benthos (`topic_browser` processor).
3. umh-core caches protobuf **UNS bundles** and ships them to subscribers in status (`core.topicBrowser.unsBundles`).

So the product surface (explore / verify / debug) sits on top of a **read-only UNS mirror**, not on protocol inputs.

### GraphQL on the edge (optional)

Per [official configuration](https://docs.umh.app/usage/unified-namespace/topic-browser):

```yaml
internal:
  topicBrowser:
    desiredState: active

agent:
  graphql:
    enabled: true    # required; disabled by default
    port: 8090
```

Example queries filter by text or metadata (e.g. `data_contract: _pump_v1`) and return `topic`, `metadata`, and `lastEvent`. Same semantic data as the console tree, scoped to **one instance**.

### Official performance and scale expectations

From UMH docs (set expectations accordingly):

- **History depth** — last **100** values per topic in the browser UI.
- **High rate** — very fast producers are **batched** for display.
- **Large namespaces** — above ~**10,000** topics, the tree may load **progressively**.
- **Per instance** — each UMH Core instance has its own Topic Browser service (MC aggregates multiple instances).

### How do Management Console and Taksa DM relate to this?

| Consumer | Role vs official “explore UNS” purpose |
|----------|----------------------------------------|
| **[management.umh.app](https://management.umh.app) Topic Browser** | Primary UX described in UMH docs: live tree, search, detail, history. |
| **Taksa Device Management** | **Persistent projection** of `core.topicBrowser` into PostgreSQL + northbound list/detail/tree APIs — same topic naming and metadata ideas, but **not** a live socket to the DCD. See [TOPIC_BROWSER.md](./TOPIC_BROWSER.md), [TOPIC_BROWSER_UI_GUIDE.md](./TOPIC_BROWSER_UI_GUIDE.md). |

Both consume **already-published UNS** data paths; neither implements OPC UA browse for configuration.

### Freshness assumptions (edge + DM)

- Edge status (~1 Hz) requires an active **subscriber** (`subscribe`, ~5 min TTL).
- Taksa DM: optional **auto-resubscribe**, catalog staleness thresholds, `EnsureDeviceStatusSubscription` — see [TOPIC_BROWSER.md](./TOPIC_BROWSER.md).
- DM catalog can lag the live MC view by one or more status intervals; removals may lag until a **full catalog** snapshot — see FAQ section on DM limits below.

---

## Tag discovery: protocol layer (bridges)

### Where does real OPC UA “tag discovery” happen?

Inside the **bridge read Data Flow Component**, in **benthos-umh** `opcua` input:

- You configure one or more root **`nodeIDs`** (often folders).
- On start, the plugin **browses** the server (Organizes references, depth limits, server profiles) and **subscribes** to discovered variables.
- Messages carry metadata such as `opcua_tag_name`, `opcua_tag_path`, `opcua_attr_nodeid`.

This runs in the **same process as production reading**, not in the Topic Browser service.

### Is there a Management Console or companion API to browse tags before configuring a bridge?

**No.** Open-source umh-core does not offer a separate “discover tags on the device” step ahead of deploy. Configuration uses manual input YAML (addresses / `nodeIDs`), external engineering tools, or deploy-then-verify via Topic Browser.

Edge actions for Protocol Converters today are deploy, edit, get, delete, plus logs, metrics, and config — not pre-browse of the plant.

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

**Not as a built-in browse step.** Options in practice:

1. **Manual YAML** — enter `nodeIDs` / addresses (Taksa UI defaults are placeholders).
2. **External tools** — UA Expert, vendor exports, engineering documents.
3. **Deploy-then-inspect** — minimal read config → protocol browse runs in the read DFC → use Topic Browser to **verify** flow and read metadata (official [debugging / verifying data flow](https://docs.umh.app/usage/unified-namespace/topic-browser) use cases) → refine `edit-protocol-converter`.

### After a fresh bridge deploy, when will Topic Browser show anything?

Only after:

- Read DFC is **active** and connected,
- Protocol input is **successfully** reading (OPC UA browse + subscribe completed for configured roots),
- Tag Processor publishes to UNS,
- Subscriber exists so status includes `topicBrowser` bundles.

Connection-only deploy (general tab, no read YAML) → **no topics**.

### Can Topic Browser help choose `nodeIDs` for OPC UA?

**Only indirectly, and only after data is flowing** — which is outside its documented primary purpose. Metadata on a topic may show `opcua_tag_name` or `opcua_attr_nodeid` for **already-published** points (useful for [debugging](https://docs.umh.app/usage/unified-namespace/topic-browser)), but Topic Browser does **not** expose the full OPC UA server tree for greenfield configuration.

---

## Taksa Device Management: capabilities and limits

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
| “Topic Browser is for discovering PLC tags.” | Per [UMH docs](https://docs.umh.app/usage/unified-namespace/topic-browser), it is for **exploring, finding, verifying, and debugging** data **already in the UNS** — not for device address-space browse. |
| “Topic Browser discovers OPC UA tags on the server.” | New topics appear in the tree when bridges **publish** to the UNS; optional metadata may echo `opcua_tag_name` — that is traceability, not server browse. |
| “Topic Browser replaces UA Expert for setup.” | Use protocol config + OPC UA input browse in the read DFC; use Topic Browser **after** deploy to confirm topics and values. |
| “Topic Browser runs before configure read.” | No topics until read flow publishes; MC/DM need status subscription for live catalog updates. |
| “Tag discovery = one feature in MC.” | Split: **protocol browse** (bridge read DFC) vs **UNS exploration** (Topic Browser / DM per official intent). |
| “S7 bridge auto-finds all DB addresses.” | You must list `addresses`; only mapping to tag names is automatic. |

---

## Assumptions to communicate externally

Use these when writing UX copy, RFPs, or customer-facing capability statements:

1. **UNS is the integration surface** for analytics and catalog APIs; PLCs are reached only through Protocol Converters.
2. **Tag configuration** for non-OPC-UA protocols requires **prior knowledge** of addresses/tags (or imports), not edge browse APIs.
3. **OPC UA** supports **folder-root browse** at runtime in the read pipeline only — not a separate pre-deploy tag-picker in MC or open-source umh-core.
4. **Topic Browser** — Communicate using UMH’s framing: [real-time UNS exploration](https://docs.umh.app/usage/unified-namespace/topic-browser) (tree, search, values, history, metadata for debugging) — **not** pre-configuration tag discovery.
5. **Deploy-then-inspect** is a practical pattern for OPC UA `nodeIDs`, but it is **verification/debugging**, not the documented primary purpose of Topic Browser.
6. **Catalog completeness** in DM depends on edge status semantics (`topicCount`, bundle 0 shape); incremental updates may retain obsolete topics until a full snapshot or empty catalog.
7. **GraphQL** on the edge is **off by default**; enable `agent.graphql` for programmatic queries aligned with the same cache as the console.

---

## Decision guide: which mechanism do I need?

Aligned with [Topic Browser use cases](https://docs.umh.app/usage/unified-namespace/topic-browser) vs protocol configuration:

| Need to… | Use… |
|----------|------|
| Test IP/port reachable | PC connection / nmap (not tags) |
| List OPC UA nodes before any deploy | External OPC UA client or manual `nodeIDs` |
| Browse OPC UA under a folder at runtime | `opcua` input in read DFC (benthos-umh) |
| Map PLC address → UNS tag name | `tag_processor` (`msg.meta.*_address` → `tag_name`) |
| Explore / search UNS (official Topic Browser purpose) | Management Console Topic Browser or GraphQL `:8090` |
| Verify bridge is publishing (value, time) | Topic Browser detail + history panel |
| Debug which bridge owns a topic | Topic Browser metadata (e.g. `bridged_by`) |
| Query topics from cloud / Taksa UI | DM `ListDeviceTopics` / `ListTopicNodes` |
| Remove stale topics from DM DB after bridge swap | Full status snapshot or `topicCount == 0` |
| Pick OPC UA tags before any bridge deploy | External OPC UA tool or manual `nodeIDs` |

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
| **Topic Browser (product)** | UMH feature to explore/verify/debug **UNS** data ([official scope](https://docs.umh.app/usage/unified-namespace/topic-browser)) |

---

## Document maintenance

| Check | When |
|-------|------|
| [UMH Topic Browser docs](https://docs.umh.app/usage/unified-namespace/topic-browser) | When product positioning changes |
| DM merge rules | When changing `internal/topicbrowser/merge.go` |
| This FAQ | When customer-facing capability statements change |

*Last aligned with: [docs.umh.app Topic Browser](https://docs.umh.app/usage/unified-namespace/topic-browser); Taksa DM topic sync as in TOPIC_BROWSER.md.*
