package storage

import (
	"context"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// AuthTokenStore defines the interface for auth token storage operations
type AuthTokenStore interface {
	// Save persists a raw auth token to storage with tenant isolation
	// rawToken is the raw token string (not hashed)
	// tenantID is required for multi-tenancy isolation
	Save(ctx context.Context, tenantID string, token *models.AuthToken, rawToken string) error

	// GetAllValidAuthTokensSystemWide retrieves all non-expired tokens across all tenants
	// Returns map[rawToken] → AuthTokenInfo{DeviceID, TenantID}
	// SECURITY: Only used by device login which lacks tenant context
	GetAllValidAuthTokensSystemWide(ctx context.Context) (map[string]models.AuthTokenInfo, error)

	// UpdateExpiry updates a token's expiry date with tenant isolation
	// Used to renew token on successful login
	// tenantID ensures we only update tokens from this tenant
	UpdateExpiry(ctx context.Context, tenantID, rawToken string, expiresAt time.Time) error

	// DeleteByDeviceID removes all tokens for a device with tenant isolation
	// tenantID + deviceID ensures we only delete tokens from this tenant/device
	DeleteByDeviceID(ctx context.Context, tenantID, deviceID string) error
}

