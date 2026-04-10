# Instance API Testing Guide (umh-core Compatible)

Updated Bruno tests for device-management instance APIs with full umh-core compatibility.

## Test Suite Files

1. **Instance-API-Complete-Flow.bru** - Overview and setup (seq: 0)
2. **Instance-Login.bru** - Device authentication (seq: 1)
3. **Instance-Pull.bru** - Retrieve queued messages (seq: 2)
4. **Instance-Push.bru** - Send device telemetry (seq: 3)

## Prerequisites

1. **Start device-management server:**
   ```bash
   cd <project-root>/repos/taksa-platform/device-management
   go run ./cmd/device-management/
   ```

2. **Register a device** (if not already done):
   - Device will return an auth token
   - Save this token

3. **Set environment variable** in Bruno:
   - Click environment icon
   - Add variable: `token_hash` = your-device-double-sha3-hash
   - Or manually set in Instance-Login Authorization header

## Test Execution Order

### Step 1: Login (seq: 1)
```
GET /api/v2/instance/login
Headers:
  Authorization: Bearer {{token_hash}}
  Content-Type: application/json

Expected Response:
  ✓ HTTP 200
  ✓ Set-Cookie header with JWT (HttpOnly, SameSite=Strict)
  ✓ JSON body with device info
  ✓ All fields in camelCase (encryptedPrivateKey, not encrypted_private_key)
  ✓ userCount as number (not string)
```

**Automatically sets:**
- Cookie: token=<jwt>
- Environment: device_uuid, device_name

### Step 2: Pull Messages (seq: 2)
```
GET /api/v2/instance/pull
Headers:
  Cookie: token=<from-login>
  Connection: keep-alive
  Keep-Alive: timeout=30, max=1000
  X-Features: longpoll;

Expected Response:
  ✓ HTTP 200
  ✓ {"UMHMessages": [...]}
  ✓ Message fields: email, content, umhInstance (camelCase!), metadata
  ✓ content is JSON string with MessageType and Payload
```

### Step 3: Push Telemetry (seq: 3)
```
POST /api/v2/instance/push
Headers:
  Cookie: token=<from-login>
  Content-Type: application/json
  Connection: keep-alive
  Keep-Alive: timeout=30, max=1000
  X-Features: longpoll;

Body:
{
  "UMHMessages": [
    {
      "email": "device@example.com",
      "content": "{\"MessageType\": \"status\", \"Payload\": {...}}",
      "umhInstance": "device-uuid-here",
      "metadata": null
    }
  ]
}

Expected Response:
  ✓ HTTP 200
  ✓ {} (empty JSON object)
  ✓ No errors in logs
```

## Key Compatibility Points

### 1. Authentication
- **Method**: Double SHA3 hash in Authorization header
- **Format**: `Authorization: Bearer SHA3(SHA3(token))`
- **Cookie**: JWT stored in `token` cookie (HttpOnly, SameSite=Strict)
- **Subsequent Requests**: Use token cookie (auto-managed by Bruno)

### 2. Headers (Critical for umh-core)
```
Connection: keep-alive
Keep-Alive: timeout=30, max=1000
X-Features: longpoll;
```
These enable long-polling support for Pull endpoint.

### 3. JSON Field Names (MUST be camelCase)
```
✓ Correct:
  - encryptedPrivateKey
  - umhInstance
  - licenseStatus
  - isActive
  - validTo

✗ Wrong (snake_case):
  - encrypted_private_key
  - umh_instance
  - license_status
  - is_active
  - valid_to
```

### 4. UMHMessage Structure
```json
{
  "email": "string",
  "content": "string (JSON-serialized)",
  "umhInstance": "uuid-string",
  "metadata": {
    "traceId": "optional",
    "requestId": "optional",
    "correlationId": "optional"
  }
}
```

### 5. Message Content Format
```json
{
  "MessageType": "status|telemetry|action|action-reply",
  "Payload": {
    // Message-specific data
  }
}
```

## Running All Tests

### Option 1: Sequential in Bruno UI
1. Click Instance-Login → Send
2. Verify device_uuid in response
3. Click Instance-Pull → Send
4. Click Instance-Push → Send

### Option 2: Run Collection
```bash
# If using Bruno CLI
bruno run --collection ./bruno
```

## Troubleshooting

### "device_uuid not set" Error
**Fix:** Run Instance-Login first, verify the response contains uuid field.

### "Invalid token" Error
**Fix:** 
1. Verify token_hash is correctly computed (double SHA3)
2. Verify device exists in database
3. Check server logs for authentication errors

### "Cookie not found" Error
**Fix:** 
1. Verify Set-Cookie header is present in Login response
2. Bruno should auto-save cookies; check cookie manager
3. Try manually adding Cookie header if auto-save fails

### Fields showing snake_case instead of camelCase
**Fix:** This is a server-side issue. Verify:
1. `MarshalLoginResponseJSON()` is being called
2. `MarshalPullResponseJSON()` is being called
3. `loginResponseEncoder()` is registered in HTTP server

### Pull returns empty messages
**Expected behavior** if no actions queued. You can:
1. Use console UI to queue an action
2. Manually insert action in database for testing

### Push returns 400 Bad Request
**Check:**
1. Body is valid PushPayload JSON
2. umhInstance matches authenticated device UUID
3. content is valid JSON string
4. All fields present (email, content, umhInstance, metadata)

## Performance Notes

- **Pull**: Polls every 10ms in umh-core (can test with delays)
- **Push**: Batches telemetry (multiple messages per request)
- **Keep-Alive**: Required for long-polling, timeout=30s

## Files Modified

All files follow umh-core's exact request/response patterns:

| Component | Status | Changes |
|-----------|--------|---------|
| Login response | ✅ Verified | camelCase fields, JWT cookie |
| Pull endpoint | ✅ Verified | camelCase UMHMessage fields |
| Push endpoint | ✅ Verified | Accepts PushPayload, stores messages |
| Headers | ✅ Verified | Connection, Keep-Alive, X-Features |
| JSON marshalling | ✅ Verified | Custom marshalling for camelCase |
| Cookie handling | ✅ Verified | HttpOnly, SameSite=Strict, MaxAge=3600 |

## References

- umh-core Pull Implementation: `umh-core/pkg/communicator/api/v2/pull/pull.go`
- umh-core Push Implementation: `umh-core/pkg/communicator/api/v2/push/push.go`
- umh-core HTTP Client: `umh-core/pkg/communicator/api/v2/http/requester.go`
- device-management Instance Service: `internal/service/instance.go`
