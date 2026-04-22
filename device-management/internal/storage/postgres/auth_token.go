package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// AuthTokenStore implements storage.AuthTokenStore for PostgreSQL
type AuthTokenStore struct {
	db *sql.DB
}

// Save persists an auth token to storage with tenant isolation
// Stores the raw token (not hashed) so we can hash it during validation
// This matches umh-mock-console approach: GetAllValidAuthTokens retrieves raw tokens
// and hashes them during login to compare with client-sent hash
// tenantID ensures token is scoped to the correct tenant
func (s *AuthTokenStore) Save(ctx context.Context, tenantID string, token *models.AuthToken, rawToken string) error {
	if tenantID == "" || token == nil || token.DeviceID == "" || rawToken == "" {
		return ErrInvalidInput
	}

	id := generateUUID()
	query := `
	INSERT INTO auth_tokens (id, tenant_id, token, device_id, created_at, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.ExecContext(ctx, query,
		id,
		tenantID,
		rawToken,
		token.DeviceID,
		time.Now().Format(time.RFC3339),
		token.ExpiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to save auth token: %w", err)
	}

	return nil
}

// GetAllValidAuthTokensSystemWide retrieves all non-expired tokens across ALL tenants.
// Returns map[rawToken] → AuthTokenInfo{DeviceID, TenantID} so the caller gets
// both device_id and tenant_id resolved from the token in a single lookup.
// SECURITY: Only used by device login which lacks tenant context.
func (s *AuthTokenStore) GetAllValidAuthTokensSystemWide(ctx context.Context) (map[string]models.AuthTokenInfo, error) {
	query := `
	SELECT token, device_id, tenant_id FROM auth_tokens
	WHERE expires_at > CURRENT_TIMESTAMP
	ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query valid auth tokens: %w", err)
	}
	defer rows.Close()

	result := make(map[string]models.AuthTokenInfo)
	for rows.Next() {
		var token, deviceID, tenantID string
		if err := rows.Scan(&token, &deviceID, &tenantID); err != nil {
			continue
		}
		result[token] = models.AuthTokenInfo{DeviceID: deviceID, TenantID: tenantID}
	}

	return result, rows.Err()
}

// UpdateExpiry updates a token's expiry date with tenant isolation
// Used to renew token on successful login
// tenantID ensures we only update tokens from this tenant
func (s *AuthTokenStore) UpdateExpiry(ctx context.Context, tenantID, rawToken string, expiresAt time.Time) error {
	if tenantID == "" || rawToken == "" {
		return ErrInvalidInput
	}

	query := `
	UPDATE auth_tokens
	SET expires_at = $1
	WHERE tenant_id = $2 AND token = $3
	`

	result, err := s.db.ExecContext(ctx, query, expiresAt.Format(time.RFC3339), tenantID, rawToken)
	if err != nil {
		return fmt.Errorf("failed to update token expiry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDeviceID removes all tokens for a device with tenant isolation
// tenantID + deviceID ensures we only delete tokens from this tenant/device
func (s *AuthTokenStore) DeleteByDeviceID(ctx context.Context, tenantID, deviceID string) error {
	if tenantID == "" || deviceID == "" {
		return ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM auth_tokens WHERE tenant_id = $1 AND device_id = $2", tenantID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete tokens for device: %w", err)
	}

	return nil
}

