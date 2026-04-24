# JWT Secret Generation and Persistence

## Overview

The device-management service now implements **generate-once, persist-under-/data** strategy for JWT signing secrets. This eliminates the need to manually configure `TAKSA_DM_JWT_SECRET` in environment variables or `config.yaml` for each deployment.

## Behavior

### Priority Order

The JWT secret resolution follows this priority:

1. **Explicit Configuration** (highest priority)
   - Environment variable: `TAKSA_DM_JWT_SECRET`
   - Config file: `server.jwt_secret` in `config.yaml`
   - Use if non-empty

2. **Persisted Secret**
   - Check for existing secret at `/data/jwt.secret`
   - Use if found (ensures session continuity across restarts)

3. **Generate and Persist** (lowest priority)
   - Generate new 256-bit (32-byte) random secret
   - Convert to hex string (64 characters)
   - Write to `/data/jwt.secret` with restrictive permissions (0600)
   - Return for immediate use

### Session Continuity

By persisting the secret to `/data/` (a persistent volume mount), the service automatically maintains session continuity across restarts:

- On first start: Generates and persists a secret → devices can login and receive cookies
- On restart: Reads persisted secret → existing device cookies remain valid
- No manual intervention required

## Implementation Details

### File Location

- **Default path**: `/data/jwt.secret`
- **Permissions**: `0600` (owner read/write only)
- **Format**: Hex-encoded string (64 characters)

### Code Changes

#### `internal/utils/jwt_secret.go`

Provides two main functions:

```go
// Standard usage (uses /data/jwt.secret)
secret, err := utils.GetOrGenerateJWTSecret(envSecret)

// Testing with custom path
secret, err := utils.GetOrGenerateJWTSecretWithPath(envSecret, "/custom/path")
```

#### `cmd/device-management/main.go`

Replaces the panic on missing secret with automatic generation:

```go
// Old (before)
if bc.Server.JwtSecret == "" {
    panic("TAKSA_DM_JWT_SECRET is required...")
}

// New (after)
jwtSecret := strings.TrimSpace(bc.Server.JwtSecret)
if jwtSecret == `""` {
    jwtSecret = ""
} else if len(jwtSecret) >= 2 && strings.HasPrefix(jwtSecret, `"`) && strings.HasSuffix(jwtSecret, `"`) {
    jwtSecret = strings.Trim(jwtSecret, `"`)
}

jwtSecret, err := utils.GetOrGenerateJWTSecret(jwtSecret)
if err != nil {
    panic(fmt.Sprintf("failed to get or generate JWT secret: %v", err))
}
bc.Server.JwtSecret = jwtSecret
```

## Usage

### Docker Deployment

No changes needed to existing deployments:

```yaml
services:
  device-management:
    image: device-management:latest
    volumes:
      - data:/data
    # TAKSA_DM_JWT_SECRET can be omitted or left empty
```

The service will:
1. Start up
2. Check for `/data/jwt.secret`
3. Generate one if missing
4. Use it for all JWT operations

### Local Development

```bash
# First run: generates /data/jwt.secret
go run ./cmd/device-management/main.go

# Subsequent runs: reuse existing /data/jwt.secret
go run ./cmd/device-management/main.go
```

### Testing

All tests use `GetOrGenerateJWTSecretWithPath()` with temporary directories:

```bash
go test ./internal/utils/...
```

## Security Considerations

### Random Generation

- Uses `crypto/rand.Read()` for cryptographically secure randomness
- 256-bit (32-byte) secrets provide 2^256 possible values

### File Permissions

- `0600` (owner read/write only) — prevents unauthorized access
- Directory created with `0755` (standard) — needed for path traversal
- Mounted as `/data` in containers with appropriate ownership

### No Automatic Rotation

Secrets are **not** rotated automatically. If a new secret is needed:

1. Delete `/data/jwt.secret`
2. Restart the service
3. New secret is generated

This prevents session disruption while still allowing manual rotation when needed.

## Troubleshooting

### Secret Generation Fails

If the service panics with "failed to generate JWT secret":

1. Check that `/data` directory is writable
2. Ensure sufficient disk space
3. Check file system permissions

**Example error:**
```
panic: failed to generate JWT secret: failed to create directory /data: permission denied
```

**Solution:**
```bash
# Verify mount point
mount | grep /data
# Should show: <volume> on /data type <fstype> (rw,...)

# Fix permissions if needed
chmod 755 /data
```

### Secret Persists Between Deployments

This is **intentional behavior** — the secret survives restarts to maintain session continuity.

If you need to reset:
```bash
# In the container or volume
rm /data/jwt.secret

# Restart service (will generate new secret)
```

### Cannot Access /data

If `/data` is not available (e.g., testing without persistent volume):

1. The service will try to create `/data` at runtime
2. If the host filesystem is read-only, this fails
3. For testing, set `TAKSA_DM_JWT_SECRET` explicitly:

```bash
export TAKSA_DM_JWT_SECRET="test-secret"
go run ./cmd/device-management/main.go
```

## Integration with Auth

The secret is passed to:

- **AuthUsecase** (`internal/biz/auth.go`): Uses for JWT signing in `GenerateJWT()`, verification in `VerifyJWT()`
- **HTTP Server** (`internal/server/http.go`): Uses for middleware JWT validation via `HTTPTenantMiddleware`

Both consume `bc.Server.JwtSecret`, which is now automatically set.

## Testing

Unit tests cover:

1. ✅ **Env secret priority** — Explicit config takes precedence
2. ✅ **File generation** — Random secret created and written to disk
3. ✅ **File reading** — Persisted secret read on subsequent calls
4. ✅ **Persistence** — Same secret returned across multiple calls
5. ✅ **File permissions** — Secret file created with 0600 mode

Run tests:
```bash
go test -v ./internal/utils/...
```
