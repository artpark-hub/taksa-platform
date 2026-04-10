package storage

import (
	"context"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// AuthTokenStore defines the interface for auth token storage operations
type AuthTokenStore interface {
	// Save persists a raw auth token to storage
	// rawToken is the raw token string (not hashed)
	Save(ctx context.Context, token *models.AuthToken, rawToken string) error

	// GetAllValidAuthTokens retrieves all non-expired tokens with their device IDs
	// Used during login validation: iterate and hash-compare like umh-mock-console
	GetAllValidAuthTokens(ctx context.Context) (map[string]string, error)

	// GetByHash retrieves a token ID by its raw token (deprecated, use GetAllValidAuthTokens)
	GetByHash(ctx context.Context, tokenHash string) (deviceID string, err error)

	// GetByToken retrieves a token by its raw token string
	// Used to update token expiry on login renewal
	GetByToken(ctx context.Context, rawToken string) (*models.AuthToken, error)

	// UpdateExpiry updates a token's expiry date
	// Used to renew token on successful login
	UpdateExpiry(ctx context.Context, rawToken string, expiresAt time.Time) error

	// GetByDeviceID retrieves all tokens for a device
	GetByDeviceID(ctx context.Context, deviceID string) ([]*models.AuthToken, error)

	// IsValid checks if a token is valid and not expired
	IsValid(ctx context.Context, tokenHash string) (bool, error)

	// List retrieves tokens with optional filtering
	List(ctx context.Context, filters *TokenListFilter) ([]*models.AuthToken, error)

	// Delete removes a token
	Delete(ctx context.Context, tokenHash string) error

	// DeleteExpired removes all expired tokens
	DeleteExpired(ctx context.Context) error

	// DeleteByDeviceID removes all tokens for a device
	DeleteByDeviceID(ctx context.Context, deviceID string) error
}

// TokenListFilter defines filtering for token listing
type TokenListFilter struct {
	DeviceID    string     // Filter by device ID
	IncludeExpired bool     // Include expired tokens
	Before      time.Time  // Tokens created before this time
	After       time.Time  // Tokens created after this time
}
