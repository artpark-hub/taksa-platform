# OPC-UA facade APIs — Bruno test collection

Typed protocol-converter facade for OPC-UA bridges (`/protocol-converters/opc-ua`).

**UI developers:** see [`docs/OPC_UA_FACADE_UI_GUIDE.md`](../../docs/OPC_UA_FACADE_UI_GUIDE.md) for poll semantics, section modes, and GET-after-edit.

## Environment variables

Set in `environments/default.bru` (Bruno desktop) or your active environment:

| Variable | Purpose |
|----------|---------|
| `base_url` | device-management HTTP base (e.g. `http://localhost:8000`) |
| `device_id` | Target DCD |
| `opcua_converter_uuid` | Bridge UUID for GET / EDIT |
| `opcua_action_id` | Child action id from GET or EDIT (poll in 04) |
| `opcua_workflow_action_id` | Workflow id from deploy (poll in 02) |

`converter_uuid` from the generic `02-ProtocolConverters` folder is copied automatically when `opcua_converter_uuid` is unset.

## Recommended flows

### A — Existing bridge on DCD (GET only)

1. **00-ListProtocolConverters-SetUuid** — sets `opcua_converter_uuid` from catalog  
   *or* set `opcua_converter_uuid` manually from UMH console  
2. **03-GetOpcUaProtocolConverter**  
3. **04-GetOpcUaActionResult** — poll until `status=COMPLETED`  
   Inspect `result.rawInputYaml`, `result.input`, `result.readFlow`

### B — Deploy via facade

1. **01-DeployOpcUaProtocolConverter**  
2. **02-GetOpcUaWorkflowResult** — poll until workflow `COMPLETED`  
3. **03-GetOpcUaProtocolConverter** — optional verify typed GET  
4. **04-GetOpcUaActionResult**

### C — Edit via facade

1. Ensure `opcua_converter_uuid` is set (flow A step 1 or after deploy)  
2. **05-EditOpcUaProtocolConverter**  
3. **04-GetOpcUaActionResult** — poll until `COMPLETED`

## Endpoints

| Bruno | Method | Path |
|-------|--------|------|
| 01 Deploy | POST | `/devices/{device_id}/protocol-converters/opc-ua` |
| 02 Workflow poll | GET | `/devices/{device_id}/protocol-converters/opc-ua/actions/{workflow_id}/result` |
| 03 Get | GET | `/devices/{device_id}/protocol-converters/opc-ua/{uuid}` |
| 04 Action poll | GET | `/devices/{device_id}/protocol-converters/opc-ua/actions/{action_id}/result` |
| 05 Edit | PATCH | `/devices/{device_id}/protocol-converters/opc-ua/{uuid}` |

Generic list/delete remain on `/protocol-converters` (see `02-ProtocolConverters`).

## Polling

- GET / EDIT: use **04** with `opcua_action_id`  
- Deploy workflow: use **02** with `opcua_workflow_action_id`  
- Poll every 1–2s while `status` is `QUEUED`, `DELIVERED`, or `PROCESSING`  
- DCD must be online so actions can be delivered

## Status values

`QUEUED` → `DELIVERED` → `PROCESSING` → `COMPLETED` | `FAILED` | `EXPIRED` | `CANCELLED`

Deploy workflow may also include `stage`: `DEPLOYING`, `CONFIGURING`, `ROLLING_BACK`.

## CLI (optional)

From `device-management/bruno`:

```bash
bru run 03-OpcUaFacade/03-GetOpcUaProtocolConverter.bru --env default
bru run 03-OpcUaFacade/04-GetOpcUaActionResult.bru --env default
```

Ensure `opcua_converter_uuid` / `opcua_action_id` are set in the environment file first.
