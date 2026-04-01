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

## Notes

1. All mutation operations (Deploy, Edit, Delete) are asynchronous
2. List operation is synchronous (queries local database cache)
3. Results are synced to local database when action completes
4. Delete actions have empty result when status=COMPLETED
5. Processor UUID is UUID format (same as ProtocolConverters)
