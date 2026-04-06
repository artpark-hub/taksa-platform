# Instance Service - Testing Quick Start

## Pre-Flight Checklist (5 minutes)

- [ ] Go 1.20+ installed: `go version`
- [ ] Proto tools installed: `make init`
- [ ] Protos regenerated: `make all`
- [ ] Project builds: `make build`
- [ ] SQLite available: `sqlite3 --version`

## Start Service (30 seconds)

**Option A: kratos run from sources (development)**
```bash
cd <project-root>/repos/taksa-platform/device-management
make clean; make all;
kratos run
```

**Option B: go run from sources (development)**

```bash
cd <project-root>/repos/taksa-platform/device-management/cmd/device-management
go run . -conf ../../configs
```

Wait for:
```
INFO grpc listening on ...
INFO http listening on [::]:8000
```

## Quick Test (2 minutes)

### Option A: Using Bruno CLI (recommended)

```bash
# Terminal 2
cd <project-root>/repos/taksa-platform/device-management

# 1. Register device and export variables
./setup_device.sh
# This outputs: export DEVICE_ID=... export DOUBLE_HASH=... export AUTH_TOKEN=...
# Copy and paste those exports

# 2. Run all Instance API tests
cd bruno
bru run --env default \
  --env-var device_id="$DEVICE_ID" \
  --env-var double_hash="$DOUBLE_HASH"

# 3. View results with reporting
bru run --env default \
  --env-var device_id="$DEVICE_ID" \
  --env-var double_hash="$DOUBLE_HASH" \
  --reporter-json results.json

cat results.json | jq .
```

### Option B: Using the test script

```bash
# Terminal 2
cd <project-root>/repos/taksa-platform/device-management

# 1. Register device
./test-instance-service.sh register-device --name "quick-test" --serial "QT-001"
# Note the printed DEVICE_ID, AUTH_TOKEN, DOUBLE_HASH

# 2. Set environment variables
export DEVICE_ID=dev-1234567890
export DOUBLE_HASH=abc123def456...

# 3. Login
./test-instance-service.sh login
# Note the JWT_TOKEN returned

# 4. Push status (if you have JWT_TOKEN set)
export JWT_TOKEN=<value-from-login>
./test-instance-service.sh push-status

# 5. Pull actions
./test-instance-service.sh pull-actions
```

### Option C: Manual curl commands

```bash
# 1. Register
curl -X POST http://localhost:8000/v2/device/register \
  -H "Content-Type: application/json" \
  -d '{"name":"quick-test","serial_number":"QT-001","location":"Lab","company":"Test"}' | jq .

# 2. Save values from response
export DEVICE_ID=dev-xxx
export DOUBLE_HASH=xxx  # Compute using SHA3(SHA3(token))

# 3. Login
curl -X POST http://localhost:8000/v2/instance/login \
  -H "Authorization: Bearer $DOUBLE_HASH" \
  -H "Content-Type: application/json" \
  -d '{"device_id":"'$DEVICE_ID'"}' | jq .

# 4. Save JWT
export JWT_TOKEN=eyJ...

# 5. Push
curl -X POST http://localhost:8000/v2/instance/push \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "messages":[{
      "umhInstance":"'$DEVICE_ID'",
      "email":"test@local",
      "content":"{\"MessageType\":\"status\",\"Payload\":{\"cpu\":50}}",
      "timestamp":"2026-01-08T12:30:00Z"
    }]
  }' | jq .

# 6. Pull
curl -X GET "http://localhost:8000/v2/instance/pull?device_id=$DEVICE_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .
```

## Full Testing Guide

See `BRUNO_INSTANCE_API_TESTING.md` for:
- Complete API documentation with all endpoints
- Bruno CLI and GUI testing options
- Device registration and setup
- Environment configuration
- Debugging tips and troubleshooting

## Key Endpoints

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/v2/device/register` | POST | - | Register new device |
| `/v2/instance/login` | POST | Bearer (hash) | Authenticate device |
| `/v2/instance/push` | POST | Bearer (JWT) | Send device telemetry |
| `/v2/instance/pull` | GET | Bearer (JWT) | Retrieve pending actions |
| `/v2/instance/user/certificate` | GET | Bearer (JWT) | Get device certificate |

## Troubleshooting

**Port already in use:**
```bash
lsof -i :8000
kill -9 <PID>
```

**Database locked:**
```bash
rm test.db  # Reset database
```

**Proto mismatch:**
```bash
make all  # Regenerate all protos
go mod tidy
```

**Missing dependencies:**
```bash
make init  # Install proto tools
go mod download
```

## Next Steps

1. Test with umh-core integration (see INSTANCE_SERVICE_TESTING_GUIDE.md)
2. Verify database persistence
3. Test error scenarios (invalid token, expired JWT, etc.)
4. Load testing with multiple concurrent devices
5. Monitor logs and performance metrics
