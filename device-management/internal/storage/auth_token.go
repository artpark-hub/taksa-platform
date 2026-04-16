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

	// GetAllValidAuthTokens retrieves all non-expired tokens with their device IDs for a tenant
	// Used during login validation: iterate and hash-compare like umh-mock-console
	// tenantID filters to only return tokens for this tenant
	GetAllValidAuthTokens(ctx context.Context, tenantID string) (map[string]string, error)

	// GetAllValidAuthTokensSystemWide retrieves all non-expired tokens across all tenants
	// Returns map[rawToken] → AuthTokenInfo{DeviceID, TenantID}
	// SECURITY: Only used by device login which lacks tenant context
	GetAllValidAuthTokensSystemWide(ctx context.Context) (map[string]models.AuthTokenInfo, error)

	// GetByHash retrieves a token ID by its raw token (deprecated, use GetAllValidAuthTokens)
	// tenantID ensures we only return tokens from this tenant
	GetByHash(ctx context.Context, tenantID, tokenHash string) (deviceID string, err error)

	// GetByToken retrieves a token by its raw token string with tenant isolation
	// Used to update token expiry on login renewal
	GetByToken(ctx context.Context, tenantID, rawToken string) (*models.AuthToken, error)

	// UpdateExpiry updates a token's expiry date with tenant isolation
	// Used to renew token on successful login
	// tenantID ensures we only update tokens from this tenant
	UpdateExpiry(ctx context.Context, tenantID, rawToken string, expiresAt time.Time) error

	// GetByDeviceID retrieves all tokens for a device within a tenant
	// tenantID + deviceID ensures isolation
	GetByDeviceID(ctx context.Context, tenantID, deviceID string) ([]*models.AuthToken, error)

	// IsValid checks if a token is valid and not expired within a tenant
	// tenantID ensures we only validate tokens from this tenant
	IsValid(ctx context.Context, tenantID, tokenHash string) (bool, error)

	// List retrieves tokens with optional filtering within a tenant
	// tenantID filters to only return tokens from this tenant
	List(ctx context.Context, tenantID string, filters *TokenListFilter) ([]*models.AuthToken, error)

	// Delete removes a token with tenant isolation
	// tenantID ensures we only delete tokens from this tenant
	Delete(ctx context.Context, tenantID, tokenHash string) error

	// DeleteExpired removes all expired tokens for a tenant
	// tenantID scopes the deletion to this tenant only
	DeleteExpired(ctx context.Context, tenantID string) error

	// DeleteByDeviceID removes all tokens for a device with tenant isolation
	// tenantID + deviceID ensures we only delete tokens from this tenant/device
	DeleteByDeviceID(ctx context.Context, tenantID, deviceID string) error
}

// TokenListFilter defines filtering for token listing
type TokenListFilter struct {
	DeviceID    string     // Filter by device ID
	IncludeExpired bool     // Include expired tokens
	Before      time.Time  // Tokens created before this time
	After       time.Time  // Tokens created after this time
}
