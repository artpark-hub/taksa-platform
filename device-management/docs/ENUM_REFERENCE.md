# API Enum Reference

## Standard Protobuf Behavior

This API follows standard Protobuf conventions where enums are returned as **integer values** in JSON responses.

---

## Device Status Enum

API responses return `status` as an integer. Use this mapping to understand the values:

| Value | Name | Meaning |
|-------|------|---------|
| **0** | `DEVICE_STATUS_UNSPECIFIED` | Unspecified (should not occur) |
| **1** | `PENDING` | Device created, awaiting first login |
| **2** | `ACTIVE` | Device authenticated and communicating |
| **3** | `INACTIVE` | Device not communicating |
| **4** | `SUSPENDED` | Device administratively suspended |
| **5** | `DECOMMISSIONED` | Device no longer in use |

### Example

**API Response**:
```json
{
  "device": {
    "id": "abc123",
    "name": "EdgeDevice-01",
    "status": 1
  }
}
```

**Interpretation**: `status: 1` means the device is in `PENDING` state (newly created).

---

## Action Status Enum

Async action responses return `status` as an integer:

| Value | Name | Meaning |
|-------|------|---------|
| **0** | `ACTION_STATUS_UNSPECIFIED` | Unspecified |
| **1** | `QUEUED` | Action queued for device |
| **2** | `DELIVERED` | Device acknowledged receipt |
| **3** | `PROCESSING` | Device processing action |
| **4** | `COMPLETED` | Action completed successfully |
| **5** | `FAILED` | Action failed |
| **6** | `EXPIRED` | Action expired without completion |
| **7** | `CANCELLED` | Action was cancelled |

---

## Health Status Enum

Service health check returns `status` as a string (human-readable):

```json
{
  "status": "healthy",
  "database_connected": true,
  "diagnostics": {...}
}
```

Valid values: `"healthy"`, `"degraded"`, `"unhealthy"`

---

## Why Integers for Protobuf Enums?

Protobuf follows a standard convention of returning enums as integers in JSON because:

1. **Backward compatibility**: Enum numbering never changes
2. **Performance**: Integers are more compact than strings
3. **Language-agnostic**: Works the same way across all languages
4. **Wire efficiency**: Smaller payload size

This is consistent with the industry standard used by Google, gRPC, and other major frameworks.

---

## How to Use

### In Code

**Python/JavaScript**:
```javascript
const deviceStatus = response.device.status;
const statusName = ['UNSPECIFIED', 'PENDING', 'ACTIVE', 'INACTIVE', 'SUSPENDED', 'DECOMMISSIONED'][deviceStatus];
console.log(`Device is ${statusName}`);
```

**Go**:
```go
device := response.Device
statusName := v1.DeviceStatus_name[int32(device.Status)]
```

### In Tests

Reference the enum mappings in test assertions:

```javascript
// Instead of:
expect(device.status).to.equal('PENDING');  // ✗ Wrong

// Use:
expect(device.status).to.equal(1);  // ✓ Correct (1 = PENDING)
```

---

## Proto Definitions

See the proto files for authoritative enum definitions:

- Device Status: `api/devicemgmt/v1/models.proto` (enum DeviceStatus)
- Action Status: `api/devicemgmt/v1/devicemgmt.proto` (enum ActionStatus)

