# Device Management API - Bruno Testing Guide

This directory contains Bruno API test files for the Taksa Platform Device Management API (device-management).

## API Patterns

### Synchronous Operations
Immediate response operations:
- **RegisterDevice**: Create new device (returns device_id, auth_token)
- **ListDevices**: Paginated device listing (cursor-based)
- **GetDevice**: Retrieve single device details
- **UpdateDevice**: Modify device information
- **GetDeviceHealth**: Retrieve health metrics from Push API
- **DeleteDevice**: Decommission device

### Asynchronous Operations (Queue-and-Poll)
Actions that return action_id for polling:
- **GetDeviceConfig**: Queue config file retrieval
- **SetDeviceConfig**: Queue config file update
- **GetLogs**: Queue log retrieval
- **GetMetrics**: Queue metrics retrieval
- **GetActionResult**: Poll for action status/result

## Test Organization

```
00-DeviceMgmt/         # Device CRUD operations
  01-RegisterDevice.bru
  02-ListDevices.bru
  03-GetDevice.bru
  04-UpdateDevice.bru
  05-GetDeviceHealth.bru
  06-DeleteDevice.bru

01-DeviceActions/      # Async action operations
  01-GetDeviceConfig.bru
  02-SetDeviceConfig.bru
  03-GetLogs.bru
  04-GetMetrics.bru
  05-GetActionResult.bru
```

## Setup

### 1. Environment Variables

Before running tests, configure in Bruno environment (`environments/default.bru`):

```
base_url=http://localhost:8080
device_id=          # Set by RegisterDevice
action_id_config=   # Set by GetDeviceConfig
action_id_logs=     # Set by GetLogs
action_id_metrics=  # Set by GetMetrics
start_time_ms=      # Auto-set to 1 hour ago
timestamp=          # Auto-set for unique naming
```

### 2. Running Tests

#### Complete Flow
Run in order:
1. **RegisterDevice** → Sets `device_id`, `token_raw`, `token_hash`
2. **ListDevices** → Lists all devices
3. **GetDevice** → Retrieves specific device
4. **UpdateDevice** → Modifies device
5. **GetDeviceHealth** → Checks health
6. **DeleteDevice** → Decommissions device

#### Action Flow (requires existing device)
1. **GetDeviceConfig** → Queues action, sets `action_id_config`
2. **SetDeviceConfig** → Queues update action
3. **GetLogs** → Queues log retrieval, sets `action_id_logs`
4. **GetMetrics** → Queues metrics retrieval
5. **GetActionResult** → Poll for any action result

## Key Features

### Cursor-Based Pagination
ListDevices uses cursor-based pagination:
- **Request**: `page_size` + `page_token`
- **Response**: `devices` array + `next_page_token` for next page
- **Empty token**: Indicates no more pages

Example pagination:
```
1. GET /api/v1/devicemgmt/devices?page_size=10
   → Returns 10 devices + next_page_token: "abc123"

2. GET /api/v1/devicemgmt/devices?page_size=10&page_token=abc123
   → Returns next 10 devices + next_page_token: "def456"

3. GET /api/v1/devicemgmt/devices?page_size=10&page_token=def456
   → Returns remaining devices + next_page_token: "" (empty = end)
```

### Type-Safe Filtering (ListDevices)
```
GET /api/v1/devicemgmt/devices?
  page_size=50&
  status=ACTIVE&          # DeviceStatus enum
  location_level_0=Enterprise&
  search=device-name&
  sort_by=CREATED_AT&     # DeviceSortField enum
  sort_desc=true
```

Supported values:
- **status**: PENDING, ACTIVE, INACTIVE, SUSPENDED, DECOMMISSIONED
- **sort_by**: CREATED_AT (default), NAME, LAST_SEEN

### Action Polling
Asynchronous actions return immediately with `action_id`:

```
Action Status Flow:
QUEUED → DELIVERED → PROCESSING → COMPLETED
                                ↓
                                FAILED
                                EXPIRED
                                CANCELLED
```

Poll strategy:
- First poll: 100-200ms delay
- Subsequent: Exponential backoff (0.5s, 1s, 2s, 5s, 10s)
- Max retries: ~20 (handles ~30 second operations)

### Device Location (ISA-95 Hierarchy)
```
{
  "location": {
    "levels": {
      "0": "Enterprise",      # Required
      "1": "Site",           # Optional
      "2": "Area",           # Optional
      "3": "Line",           # Optional
      "4": "WorkCell",       # Optional
      "5": "..."             # Unlimited
    }
  }
}
```

### Device Status States
```
PENDING         → Newly registered, awaiting first login
ACTIVE          → Authenticated and communicating
INACTIVE        → Not communicating
SUSPENDED       → Administratively suspended
DECOMMISSIONED  → No longer in use
```

## Response Types

### Synchronous Responses
- **Device**: Full device object with all metadata
- **DeviceHealthResponse**: Health metrics from last Push API message
- **Empty**: Successful delete returns empty response

### Asynchronous Responses
- **ActionQueuedResponse**: `{action_id, timestamp}`
- **DeviceConfigActionResponse**: `{action_id, status, result, error_message, completed_at}`
- **LogsActionResponse**: `{action_id, status, result_payload (bytes, may be compressed), ...}`
- **MetricsActionResponse**: `{action_id, status, result_payload (bytes, may be compressed), ...}`

## Compressed Payloads

Logs and Metrics responses may be compressed:

```
// Check magic bytes:
result_payload[0:2] == 0x28 0xb5 0x2f 0xfd  → zstd compression
result_payload[0:2] == 0x1f 0x8b            → gzip compression
else                                        → uncompressed JSON
```

## Error Handling

All errors follow standard gRPC error codes:

```
400 Bad Request    → InvalidArgument
401 Unauthorized   → Unauthenticated
403 Forbidden      → PermissionDenied
404 Not Found      → NotFound
409 Conflict       → AlreadyExists
500 Server Error   → Internal
```

## Database Compatibility

Tests run against both:
- **PostgreSQL**: Production database
- **SQLite**: Local testing database

Both implementations use identical cursor pagination logic.

## Tips & Tricks

### Reuse device_id across test runs
```
Edit environments/default.bru:
device_id=<your-device-id>
```

Then run individual tests without RegisterDevice.

### Test action polling
Logs/Metrics actions may complete immediately if device is responsive:
```
1. GetLogs → Returns action_id
2. GetActionResult → Immediately returns COMPLETED with result_payload
```

### Verify proto changes
After modifying `devicemgmt.proto`:
```bash
cd <project-root>/repos/taksa-platform/device-management
make clean && make all
```

Tests will auto-fail with new fields in responses, indicating proto changes.

## Common Issues

### Missing device_id
**Error**: `404 Not Found`
**Fix**: Run RegisterDevice first, or set `device_id` in environment

### Action expired
**Error**: Status=EXPIRED after ~5 minutes
**Fix**: Device may be offline. Check GetDeviceHealth

### Compressed payload decoding
**Error**: JSON parse error on result_payload
**Fix**: Check magic bytes and decompress (zstd/gzip) before JSON parsing

### Cursor token invalid
**Error**: `400 Bad Request: invalid page_token`
**Fix**: Page tokens are opaque - don't manually edit. Use returned tokens only.
