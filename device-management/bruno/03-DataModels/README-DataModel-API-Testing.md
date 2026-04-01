# DataModel API Testing Guide

## Overview

This directory contains Bruno test files for the DataModel API endpoints. The tests follow a sequential pattern where each test builds on previous ones by sharing environment variables.

## API Endpoints

### 1. Add Data Model (Asynchronous)
- **Method:** POST
- **Path:** `/api/v1/devicemgmt/devices/{device_id}/data-models`
- **Purpose:** Queue action to add a new data model to a device
- **Returns:** action_id for polling results

### 2. Get Data Model Action Result (Polling)
- **Method:** GET
- **Path:** `/api/v1/devicemgmt/devices/{device_id}/data-models/{action_id}/result`
- **Purpose:** Poll for completion of a data model action
- **Returns:** Action status and result (name, version, description, encodedStructure)

### 3. List Data Models (Synchronous)
- **Method:** GET
- **Path:** `/api/v1/devicemgmt/devices/{device_id}/data-models`
- **Purpose:** Query local database for all data models
- **Supports:** Pagination, name_filter (substring), version_filter (exact match)
- **Returns:** Array of DataModelSummary objects

### 4. Edit Data Model (Asynchronous)
- **Method:** PATCH
- **Path:** `/api/v1/devicemgmt/devices/{device_id}/data-models/{name}`
- **Purpose:** Queue action to edit an existing data model
- **Returns:** action_id for polling results

### 5. Get Data Model (Asynchronous)
- **Method:** GET
- **Path:** `/api/v1/devicemgmt/devices/{device_id}/data-models/{name}`
- **Purpose:** Queue action to retrieve data model details from device
- **Returns:** action_id for polling results

### 6. Delete Data Model (Asynchronous)
- **Method:** DELETE
- **Path:** `/api/v1/devicemgmt/devices/{device_id}/data-models/{name}`
- **Purpose:** Queue action to delete all versions of a data model
- **Returns:** action_id for polling results

## Test Sequence

### Sequential Execution

Tests are numbered and should be run in order:

```
01-AddDataModel
    ↓
02-GetDataModelActionResult (polls #1 result)
    ↓
03-ListDataModels (queries database after #1 completes)
    ↓
04-EditDataModel (modifies model from #1)
    ↓
05-GetDataModel (retrieves model from #1)
    ↓
06-DeleteDataModel (removes model from #1)
```

### Environment Variables Flow

```
01-AddDataModel
  └─→ Sets: action_id_data_model, data_model_name, timestamp

02-GetDataModelActionResult
  └─→ Uses: action_id_data_model
  └─→ Sets: data_model_version

03-ListDataModels
  └─→ Independent (queries all models)
  └─→ Sets: next_page_token (if pagination exists)

04-EditDataModel
  └─→ Uses: data_model_name
  └─→ Sets: action_id_data_model (new)

05-GetDataModel
  └─→ Uses: data_model_name
  └─→ Sets: action_id_data_model (new)

06-DeleteDataModel
  └─→ Uses: data_model_name
  └─→ Sets: action_id_data_model (new)
```

## Running the Tests

### Via Bruno CLI

```bash
# Navigate to project root
cd /home/rajeevb/projects/taksa-platform-dm

# Run all DataModel tests in sequence
bruno run --collection bruno --env default

# Run specific test
bruno run --file bruno/03-DataModels/01-AddDataModel.bru --env default

# Run with custom base_url
bruno run --env default --set base_url=http://localhost:8000
```

### Via Bruno UI

1. Open Bruno application
2. Load collection: `/home/rajeevb/projects/taksa-platform-dm/bruno`
3. Select environment: `default` (or custom)
4. Navigate to `03-DataModels` folder
5. Click **Run** on each test sequentially
6. Monitor console output for stored variables and results

## Preconditions

### Required Environment Variables (in `environments/default.bru`)

```
base_url: http://localhost:8000          # Server URL
device_id: a010ca9a-74bd-4b44-923e-dfc72c51fbc0   # Device UUID
```

### Device Must Be Registered

Before running these tests, ensure:
- Device is registered via `RegisterDevice` API
- Device has valid authentication token
- Device is accessible and online

## Test Scenarios

### Scenario 1: Basic CRUD Workflow
```
1. Add a data model
2. Poll until completion
3. List to verify it exists
4. Edit with new description
5. Get details
6. Delete
```

**Expected Results:**
- All actions queue successfully (HTTP 200)
- Polling shows COMPLETED status
- List shows the model after step 1
- Edit updates description
- Delete removes model

### Scenario 2: Pagination Testing

Modify test #3 (ListDataModels) to test pagination:

```
Get request: /api/v1/devicemgmt/devices/{{device_id}}/data-models?page_size=5
Expected: next_page_token returned if more than 5 models exist
```

Then run paginated request using the token:
```
Get request: /api/v1/devicemgmt/devices/{{device_id}}/data-models?page_size=5&page_token={{next_page_token}}
```

### Scenario 3: Filtering Testing

Modify test #3 to test filters:

```
Get request: /api/v1/devicemgmt/devices/{{device_id}}/data-models?name_filter=test
Expected: Only models with "test" in name returned
```

## Troubleshooting

### Issue: "No data_model_name set" Warning

**Cause:** Previous test (AddDataModel) didn't run successfully

**Solution:**
1. Check test #1 output for errors
2. Verify device_id is valid in environment
3. Check server logs for action queueing errors
4. Re-run test #1

### Issue: GetDataModelActionResult Returns QUEUED Status

**Cause:** Action is still being processed by device

**Solution:**
1. Wait 2-5 seconds
2. Re-run test #2 to poll again
3. If still processing after 10 retries, check device logs
4. Device may be offline or overloaded

### Issue: ListDataModels Returns Empty Array

**Cause:** 
- Model hasn't been synced to database yet
- Action #1 failed to complete
- Device hasn't sent status message

**Solution:**
1. Verify test #1 completed successfully (check action_id_data_model)
2. Check test #2 shows COMPLETED status
3. Check device status and connectivity
4. Wait a moment for sync to occur

### Issue: Delete Returns 404 Not Found

**Cause:** Model name is incorrect or already deleted

**Solution:**
1. Verify data_model_name environment variable is set
2. Run test #3 (ListDataModels) to verify model exists
3. Check if model was already deleted by previous run

## Response Format Examples

### AddDataModel Response
```json
{
  "actionId": "550e8400-e29b-41d4-a716-446655440000",
  "createdAt": "2025-03-25T10:30:00Z",
  "expiresAt": "2025-03-25T11:30:00Z"
}
```

### GetDataModelActionResult Response (COMPLETED)
```json
{
  "actionId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "COMPLETED",
  "completedAt": "2025-03-25T10:35:00Z",
  "result": {
    "name": "test-model-1234567890",
    "version": "1",
    "description": "Test data model",
    "encodedStructure": "base64-encoded-yaml..."
  }
}
```

### ListDataModels Response
```json
{
  "models": [
    {
      "name": "test-model-1234567890",
      "version": "1",
      "description": "Test data model"
    }
  ],
  "nextPageToken": ""
}
```

## Manual Testing

For quick manual testing without Bruno, use curl:

```bash
# Add Data Model
curl -X POST http://localhost:8000/api/v1/devicemgmt/devices/{{device_id}}/data-models \
  -H "Content-Type: application/json" \
  -d '{"payload": {"name": "manual-test"}}'

# List Data Models
curl -X GET http://localhost:8000/api/v1/devicemgmt/devices/{{device_id}}/data-models

# Get Action Result
curl -X GET http://localhost:8000/api/v1/devicemgmt/devices/{{device_id}}/data-models/{{action_id}}/result

# Edit Data Model
curl -X PATCH http://localhost:8000/api/v1/devicemgmt/devices/{{device_id}}/data-models/manual-test \
  -H "Content-Type: application/json" \
  -d '{"payload": {"name": "manual-test", "description": "updated"}}'

# Delete Data Model
curl -X DELETE http://localhost:8000/api/v1/devicemgmt/devices/{{device_id}}/data-models/manual-test
```

## Notes

- All async operations (Add, Edit, Get, Delete) follow the queue-and-poll pattern
- ListDataModels is the only synchronous operation
- Tests are device-specific - ensure correct device_id is set
- Action TTL is 3600 seconds (1 hour) - poll within this window
- Version is stored as string (e.g., "1", "2", "3") representing integer versions from umh-core
