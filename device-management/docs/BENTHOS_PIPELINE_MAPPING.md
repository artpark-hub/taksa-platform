# OPC-UA protocol converter → Benthos pipeline mapping

This document maps the **device-management OPC-UA facade** to the runtime stack:

1. **Facade proto** (`protocol_converter_opcua.proto`) — structured UI contract
2. **umh-core** `ProtocolConverter.readDFC` — persisted action payload
3. **taksa-benthos-umh** — Benthos plugins executed on the device

## Runtime pipeline (read flow)

An OPC-UA time-series read bridge becomes this Benthos config at runtime:

```yaml
input:
  opcua: { ... }              # opcua_plugin — from readDFC.inputs.data

buffer:
  none: {}                    # from readDFC.rawYAML inject

pipeline:
  processors:
    - tag_processor: { ... }  # readDFC.pipeline.processors["0"]
    - downsampler: {}          # AUTO-INJECTED by umh-core for timeseries (not in stored config)

output:
  uns: { bridged_by: ... }    # AUTO-INJECTED by umh-core (never in facade payload)
```

References:

- `taksa-benthos-umh/docs/input/opc-ua-input.md`
- `taksa-benthos-umh/docs/processing/tag-processor.md`
- `taksa-benthos-umh/docs/processing/downsampler.md`
- `taksa-edge-umh/umh-core/pkg/service/protocolconverter/runtime_config/runtime_config.go`

## Facade → readDFC mapping

| Facade field | umh-core `readDFC` | Benthos |
|--------------|-------------------|---------|
| `connection`, `location`, `state` | Top-level `ProtocolConverter` | Connection service + FSM |
| `input` (structured/raw) | `inputs.type=opcua`, `inputs.data` | `input.opcua` |
| `read_flow.processor` | `pipeline.processors["0"]` | `tag_processor` |
| `read_flow.yaml_inject` | `rawYAML.data` | `buffer`, `cache_resources`, `rate_limit_resources` |
| `template_variables` | `templateInfo.variables` | Template rendering (`{{ .IP }}`, `AddressMappings`, …) |
| `output` (GET only) | *(runtime)* | `uns` output plugin |
| downsampler | *(runtime inject)* | `downsampler` processor |

## OPC-UA input YAML keys (benthos-umh)

Structured proto fields map to these **plugin** keys (not legacy UI names):

| Proto | YAML key | Notes |
|-------|----------|-------|
| `standard.endpoint` | `endpoint` | Default `opc.tcp://{{ .IP }}:{{ .PORT }}` |
| `standard.subscribe_node_ids` | `nodeIDs` | List |
| `standard.poll_rate` | `pollRate` | Milliseconds (poll mode) |
| `standard.subscribe_enabled` | `subscribeEnabled` | Default true in facade |
| `standard.use_heartbeat` | `useHeartbeat` | Default true in facade |
| `advanced.profile` | `profile` | **Not** `serverProfile` |
| `advanced.*` | same camelCase | See `opcua_plugin/core_connection.go` |
| `additional_settings[]` | arbitrary keys | Escape hatch |

**Removed / invalid:** `subscribeInterval` is not a benthos-umh field. Legacy devices may still return it; parse ignores it.

## Tag processor (benthos-umh)

| Proto | YAML key |
|-------|----------|
| `processor.defaults` + `tag_mappings` | `defaults` (generated JS) |
| `processor.conditions` | `conditions` |
| `processor.advanced_processing` | `advancedProcessing` |
| `processor.defaults_code` / `conditions_yaml` | raw override |

Required metadata for UNS: `location_path`, `data_contract`, `tag_name` (tag processor generates `umh_topic`).

OPC-UA condition fields use expressions like `msg.meta.opcua_attr_nodeid` (see opcua input metadata table in benthos-umh docs).

## Downsampler

- **Not** part of the facade structured processor config.
- umh-core appends an empty `downsampler: {}` processor at runtime when the pipeline is time-series.
- Custom downsampler YAML belongs in `read_flow.raw_processor_yaml` (power users) or via `ds_*` message metadata set in tag_processor conditions.

## Template variables

| Variable | Source |
|----------|--------|
| `IP`, `PORT` | Connection (always set by server) |
| `AddressMappings` | JSON array from `tag_mappings` when node overrides exist |
| User-defined | `template_variables` map |

Runtime-injected (not in templateInfo): `location_path`, `location`, `internal.*`.

## Extension points (avoid proto churn)

| Mode | Use for |
|------|---------|
| `input.mode = RAW` | Full opcua block |
| `read_flow.processor_mode = RAW` | Full tag_processor + optional second processor |
| `read_flow.buffer_mode = RAW` | cache/rate_limit/buffer inject |
| `additional_settings` | Any opcua plugin key not yet in structured advanced |

## Out of scope v1

- `writeDFC` / OPC-UA output
- Relational bridges (`nodered_js` processor)
- OPC-UA tag browse (`GetOPCUATags`)
- Multi-processor custom pipelines (`processingMode: custom`)
