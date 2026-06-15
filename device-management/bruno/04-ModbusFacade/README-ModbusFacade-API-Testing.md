# Modbus facade APIs — Bruno test collection

Typed protocol-converter facade for Modbus TCP bridges (`/protocol-converters/modbus`).

**UI developers:** see [`docs/MODBUS_FACADE_UI_GUIDE.md`](../../docs/MODBUS_FACADE_UI_GUIDE.md) for poll semantics, section modes, and GET-after-edit.

## Environment variables

| Variable | Purpose |
|----------|---------|
| `base_url` | device-management HTTP base (e.g. `http://localhost:8000`) |
| `device_id` | Target DCD |
| `modbus_converter_uuid` | Bridge UUID for GET / EDIT |
| `modbus_action_id` | Child action id from GET or EDIT (poll in 04) |
| `modbus_workflow_action_id` | Workflow id from deploy (poll in 02) |

`converter_uuid` from the generic `02-ProtocolConverters` folder is copied when `modbus_converter_uuid` is unset.

## Recommended flows

### A — Existing bridge on DCD (GET only)

1. **00-ListProtocolConverters-SetUuid** — sets `modbus_converter_uuid` from catalog  
2. **03-GetModbusProtocolConverter**  
3. **04-GetModbusActionResult** — poll until `status=COMPLETED`

### B — Deploy via facade

1. **01-DeployModbusProtocolConverter**  
2. **02-GetModbusWorkflowResult** — poll until workflow `COMPLETED`  
3. **03-GetModbusProtocolConverter** — optional verify  
4. **04-GetModbusActionResult**

### C — Edit via facade

1. Ensure `modbus_converter_uuid` is set  
2. **05-EditModbusProtocolConverter**  
3. **04-GetModbusActionResult** — `result` may be absent on success  
4. **03** + **04** — mandatory GET refresh after edit

## CLI examples

```bash
bru run 04-ModbusFacade/01-DeployModbusProtocolConverter.bru --env default
bru run 04-ModbusFacade/02-GetModbusWorkflowResult.bru --env default
bru run 04-ModbusFacade/03-GetModbusProtocolConverter.bru --env default
bru run 04-ModbusFacade/04-GetModbusActionResult.bru --env default
```
