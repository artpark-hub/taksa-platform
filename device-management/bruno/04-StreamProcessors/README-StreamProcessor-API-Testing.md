# Stream Processor API Testing Guide

Stream Processors follow the async queue-and-poll pattern (same as Protocol Converters and Data Models).

## Testing Sequence

### 1. Deploy Stream Processor
```
POST /api/v1/devicemgmt/devices/{device_id}/stream-processors
```
- Returns `action_id` immediately
- Stores in Bruno env var: `action_id_stream_processor`

### 2. Poll Action Result
```
GET /api/v1/devicemgmt/devices/{device_id}/stream-processors/{action_id}/result
```
- Repeat until `status=COMPLETED` or `status=FAILED`
- On success, extracts processor UUID and stores in `stream_processor_uuid`

### 3. List Stream Processors
```
GET /api/v1/devicemgmt/devices/{device_id}/stream-processors
```
- Queries local database cache (synchronized from device)
- Shows all processors for the device
- Supports filtering and pagination (see Filter Tests section below)

### 4. Edit Stream Processor
```
PATCH /api/v1/devicemgmt/devices/{device_id}/stream-processors/{uuid}
```
- Updates processor configuration
- Returns new `action_id` for polling

### 5. Get Stream Processor Details
```
GET /api/v1/devicemgmt/devices/{device_id}/stream-processors/{uuid}
```
- Fetches full processor details from device
- Returns `action_id` for polling

### 6. Delete Stream Processor
```
DELETE /api/v1/devicemgmt/devices/{device_id}/stream-processors/{uuid}
```
- Queues deletion action
- Status=COMPLETED means successful deletion

## Environment Variables

Required:
- `base_url` - e.g., `http://localhost:8000`
- `device_id` - Device UUID

Auto-populated:
- `action_id_stream_processor` - From Deploy/Edit/Get/Delete responses
- `stream_processor_uuid` - From GetStreamProcessorActionResult response
- `timestamp` - For unique naming

## Expected Responses

### Deploy/Edit/Get/Delete
```json
{
  "actionId": "uuid",
  "createdAt": "2026-03-27T16:00:00Z",
  "expiresAt": "2026-03-27T17:00:00Z"
}
```

### GetStreamProcessorActionResult
```json
{
  "actionId": "uuid",
  "status": "COMPLETED",
  "completedAt": "2026-03-27T16:01:00Z",
  "result": {
    "uuid": "processor-uuid",
    "name": "stream-processor-name",
    "model": {
      "name": "model-name",
      "version": "1"
    },
    "encodedConfig": "base64-encoded-yaml",
    "ignoreHealthCheck": false,
    "metadata": {
      "key": "value"
    }
  }
}
```

### ListStreamProcessors
```json
{
  "processors": [
    {
      "uuid": "processor-uuid",
      "name": "stream-processor-name",
      "model": {
        "name": "model-name",
        "version": "1"
      }
    }
  ],
  "nextPageToken": ""
}
```

## Payload Format

### Deploy Request (JSON → YAML → Base64 encoding handled by service)
```json
{
  "name": "unique-processor-name",
  "uuid": "",
  "config": {
    "sources": {},
    "mappings": []
  },
  "modelName": "data-model-name",
  "modelVersion": "1.0",
  "location": {
    "0": "Enterprise",
    "1": "Site1"
  },
  "ignoreHealthCheck": false
}
```

**Required fields:**
- `name`: Processor name
- `config`: Sources and mappings (JSON structure)
- `modelName`: Data model reference name
- `modelVersion`: Data model reference version

**Service layer automatically:**
1. Converts `config` JSON object → YAML string
2. Encodes YAML → Base64
3. Queues action with encoded config to umh-core

### Edit Request (all fields optional for partial updates)
```json
{
  "name": "updated-name",
  "config": {
    "sources": {},
    "mappings": []
  },
  "modelName": "new-model",
  "modelVersion": "2.0"
}
```

## Status Codes

- **200 OK**: Request accepted (for async actions, action is queued)
- **400 Bad Request**: Missing required fields or invalid data
- **404 Not Found**: Device or processor not found
- **500 Internal Server Error**: Server error

## Filter Tests

The following filter tests support testing the ListStreamProcessors endpoint with various filter combinations:

### 07-ListStreamProcessors-WithPagination
Tests cursor-based pagination with `page_size` and `page_token` parameters.
- Demonstrates paginating through large result sets
- Shows how to use `next_page_token` for subsequent requests

### 08-ListStreamProcessors-WithFilters
Tests combining multiple filters with AND logic.
- Example: `name_filter=processor&deployment_status_filter=ACTIVE`
- Verifies both filter conditions are applied

### 09-ListStreamProcessors-FilterByUUID
Tests exact UUID matching.
- Returns at most 1 result
- Useful for retrieving a specific processor by UUID

### 10-ListStreamProcessors-FilterByName
Tests substring search on processor name.
- Case-sensitive matching
- Returns all processors containing the filter string

### 11-ListStreamProcessors-FilterByDeploymentStatus
Tests filtering by deployment status.
- Valid values: `PENDING`, `ACTIVE`, `FAILED`
- Maps from device Health.State
- Failed processors include `errorMessage` field

### 12-ListStreamProcessors-FilterByHealthStatus
Tests filtering by health status.
- Valid values: `ONLINE`, `OFFLINE`, `UNKNOWN`
- Maps from device Health.Category
- Status progression: UNKNOWN → ONLINE/OFFLINE after StatusMessage heartbeat

### 13-ListStreamProcessors-FilterByModelName
Tests substring search on model name.
- Searches StreamProcessorModelRef.name field
- Useful for finding processors using specific models

## Available Query Parameters

```
GET /api/v1/devicemgmt/devices/{device_id}/stream-processors?param1=value1&param2=value2
```

**Pagination:**
- `page_size` (int): Results per page (default 20, max 100)
- `page_token` (string): Cursor token from previous response

**Filters (all optional):**
- `uuid_filter` (string): Exact UUID match
- `name_filter` (string): Substring match on processor name
- `deployment_status_filter` (string): PENDING | ACTIVE | FAILED
- `health_status_filter` (string): ONLINE | OFFLINE | UNKNOWN
- `model_name_filter` (string): Substring match on model name

All filters work independently or combined with AND logic.

## Notes

1. All mutation operations (Deploy, Edit, Delete) are asynchronous
2. List operation is synchronous (queries local database cache)
3. Results are synced to local database when action completes
4. Delete actions have empty result when status=COMPLETED
5. Processor UUID is UUID format (same as ProtocolConverters)
6. Status fields (`deployment_status`, `health_status`, `last_sync_time`) are populated from device StatusMessage heartbeats and action results
7. `last_sync_time` is RFC3339 formatted timestamp of last sync from device
