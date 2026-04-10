# Protocol Converter APIs - Bruno Test Collection

Complete test suite for the Protocol Converter management APIs implemented in Phase 3.

## Overview

This collection provides comprehensive testing for all Protocol Converter endpoints, including:
- Asynchronous deployment operations (Deploy, Edit, Delete, Get)
- Result polling
- Synchronous listing with pagination and filtering

## Tests Included

### 1. DeployProtocolConverter
**Endpoint**: `POST /api/v1/devicemgmt/devices/{device_id}/protocol-converters`

Queues an asynchronous action to deploy a new protocol converter to a device.

**What it tests**:
- ✅ Successful action queuing (HTTP 200)
- ✅ Action ID generation (UUID format)
- ✅ Timestamp fields (created_at, expires_at)
- ✅ Response structure validation

**Expected Result**: Action queued, action_id saved for polling

---

### 2. GetProtocolConverterActionResult
**Endpoint**: `GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{action_id}/result`

Polls for the result of a protocol converter action (deploy, edit, delete, or get).

**What it tests**:
- ✅ Status transitions (QUEUED, DELIVERED, PROCESSING, COMPLETED, FAILED, etc.)
- ✅ Result field presence on COMPLETED status
- ✅ Error message presence on FAILED status
- ✅ Result structure (uuid, name, type fields)
- ✅ Response validation

**Expected Result**: Action status and result (if completed)

**Key Status Values**:
- `QUEUED`: Action waiting to be sent to device
- `DELIVERED`: Action sent, awaiting device response
- `PROCESSING`: Device executing the action
- `COMPLETED`: Success, result field populated
- `FAILED`: Failed, error_message field populated
- `EXPIRED`: Action TTL exceeded
- `CANCELLED`: Action was cancelled

---

### 3. ListProtocolConverters
**Endpoint**: `GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters`

Lists all protocol converters for a device with pagination support.

**What it tests**:
- ✅ List retrieval (HTTP 200)
- ✅ Response structure (converters array, next_page_token)
- ✅ Converter fields (uuid, name, type, status, health)
- ✅ Valid status values
- ✅ UUID format validation
- ✅ Error message presence only for FAILED status

**Expected Result**: Array of converter summaries with pagination info

**Converter Fields**:
- `uuid`: Unique identifier
- `name`: Display name
- `type`: Protocol type
- `connectionUUID`: Associated connection
- `deployment_status`: PENDING, ACTIVE, or FAILED
- `health_status`: ONLINE, OFFLINE, or UNKNOWN
- `last_sync_time`: Last StatusMessage sync timestamp
- `error_message`: Error details (only if FAILED)

---

### 4. EditProtocolConverter
**Endpoint**: `PATCH /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{uuid}`

Queues an asynchronous action to edit an existing protocol converter.

**What it tests**:
- ✅ Edit action queuing (HTTP 200)
- ✅ Action ID generation
- ✅ Timestamp fields
- ✅ UUID path parameter handling

**Expected Result**: Edit action queued, action_id saved

**Note**: UUID path parameter takes precedence over body uuid

---

### 5. GetProtocolConverter
**Endpoint**: `GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{uuid}`

Queues a GET action to retrieve detailed information about a converter.

**What it tests**:
- ✅ GET action queuing (HTTP 200)
- ✅ Action ID generation
- ✅ Timestamp fields

**Expected Result**: GET action queued, result available via polling

---

### 6. DeleteProtocolConverter
**Endpoint**: `DELETE /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{uuid}`

Queues an asynchronous action to delete/remove a protocol converter.

**What it tests**:
- ✅ Delete action queuing (HTTP 200)
- ✅ Action ID generation
- ✅ Timestamp fields

**Expected Result**: Delete action queued, converter will be removed

**Note**: DELETE actions may not have a result field, but status will be COMPLETED on success

---

### 7. ListProtocolConverters-WithPagination
**Endpoint**: `GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters?page_size=5&page_token={token}`

Demonstrates pagination cursor usage for browsing large converter lists.

**What it tests**:
- ✅ Page size limits (max 100, default 20)
- ✅ Pagination token usage
- ✅ Next page availability
- ✅ Converter structure in each page

**Expected Result**: Next page of results with updated pagination token

**Usage Pattern**:
1. Run ListProtocolConverters (saves next_page_token)
2. Run this test to get next page
3. Repeat until next_page_token is empty

---

### 8. ListProtocolConverters-WithFilters
**Endpoint**: `GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters?type_filter=generic-generate&deployment_status_filter=ACTIVE`

Demonstrates filtering converters by type and deployment status.

**What it tests**:
- ✅ Type filtering (type_filter parameter)
- ✅ Status filtering (deployment_status_filter parameter)
- ✅ Filter result accuracy
- ✅ Combined filter usage

**Expected Result**: Filtered converter list

**Supported Filters**:
- `type_filter`: Any protocol type (e.g., "generic-generate", "benthos-opcua")
- `deployment_status_filter`: "PENDING", "ACTIVE", or "FAILED"
- Both optional and can be combined

---

## Test Execution Order

Recommended sequence for testing:

1. **Deploy** → DeployProtocolConverter
2. **Poll** → GetProtocolConverterActionResult (repeat until COMPLETED)
3. **List** → ListProtocolConverters
4. **Filter** → ListProtocolConverters-WithFilters
5. **Paginate** → ListProtocolConverters-WithPagination
6. **Edit** → EditProtocolConverter (on existing converter)
7. **Get** → GetProtocolConverter (queue detail retrieval)
8. **Delete** → DeleteProtocolConverter (cleanup)

## Environment Variables

The tests use Bruno environment variables. You must set:

```
base_url: http://localhost:8000
device_id: (your device UUID)
```

**Automatically Set by Tests**:
- `timestamp`: Current timestamp (for unique naming)
- `action_id`: From DeployProtocolConverter
- `converter_uuid`: From ListProtocolConverters
- `edit_action_id`: From EditProtocolConverter
- `get_action_id`: From GetProtocolConverter
- `delete_action_id`: From DeleteProtocolConverter
- `next_page_token`: From ListProtocolConverters (if available)

## Required Device

Tests require a registered device. You can:

1. Register a device using the DeviceMgmt API tests
2. Use existing device: `211abb78-2db2-4cb5-96fd-85dd21d34d87`

## Key Testing Patterns

### Pattern 1: Async Action Queue & Poll
```
1. POST to queue action (Deploy, Edit, Delete, Get)
2. Save action_id from response
3. GET with action_id to poll results
4. Check status (QUEUED, PROCESSING, COMPLETED, FAILED, etc.)
5. Once COMPLETED, result available
```

### Pattern 2: Pagination
```
1. GET list with page_size=N
2. If next_page_token present, use it for next request
3. Continue until next_page_token is empty
```

### Pattern 3: Filtering
```
1. GET list with optional filters
2. All results match filter criteria
3. Can combine multiple filters
```

## Status Codes

| Code | Meaning | Handled By |
|------|---------|-----------|
| 200 | Success | All endpoints |
| 400 | Bad request (invalid filter, pagination token) | List endpoint |
| 404 | Device/converter not found | All endpoints |
| 500 | Server error | All endpoints |

## Database State

After running these tests, the database will contain:

1. **Protocol Converter Records**
   - deployment_status: PENDING, ACTIVE, or FAILED
   - health_status: ONLINE, OFFLINE, or UNKNOWN
   - last_synced: Timestamp of last StatusMessage sync

2. **Action Records**
   - Status tracked (QUEUED, DELIVERED, PROCESSING, COMPLETED, FAILED)
   - Results stored (only if COMPLETED)
   - Errors stored (if FAILED)

## Monitoring During Tests

### View Logs
```bash
docker compose logs -f | grep -i "protocol\|converter"
```

### Check Database
```bash
sqlite3 /tmp/taksa-dm-data/taksa_platform_dm.db
SELECT uuid, name, deployment_status, health_status, last_synced 
FROM protocol_converters 
ORDER BY created_at DESC;
```

## Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| Device not found (404) | device_id not set or invalid | Use registered device UUID |
| Converter not found (404) | converter_uuid not from previous test | Run ListProtocolConverters first |
| Invalid page_token | Malformed or expired token | Start from first page again |
| Action still QUEUED | Device hasn't received it yet | Poll again in a moment |
| Action FAILED | Device execution error | Check error_message in result |

## Related Documentation

- **PHASE_3_COMPLETE_FINAL_STATUS.md** - Architecture and implementation details
- **PHASE_3_TASK_2_IMPLEMENTATION.md** - StatusMessage reconciliation details
- **PROTOCOL_CONVERTER_PHASE_3_COMPLETION.md** - Complete API reference

## Notes

- All async actions have TTL (default 3600s / 1 hour)
- Deployment status updates are tracked via action results (Task 1)
- Health status updates are tracked via StatusMessage (Task 2)
- Device state always wins in case of conflicts
- ListProtocolConverters returns device's actual state via database

---

**Status**: ✅ Complete and Ready for Testing  
**Protocol Converter APIs**: Phase 3 Complete
