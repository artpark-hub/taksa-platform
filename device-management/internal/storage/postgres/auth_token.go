package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
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

// GetAllValidAuthTokens retrieves all non-expired tokens with their device IDs for a tenant
// Used during login: iterate all tokens, hash each one, compare with client-sent hash
// This matches umh-mock-console's login validation approach
// tenantID filters to only return tokens from this tenant
func (s *AuthTokenStore) GetAllValidAuthTokens(ctx context.Context, tenantID string) (map[string]string, error) {
	if tenantID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT token, device_id FROM auth_tokens
	WHERE tenant_id = $1 AND expires_at > CURRENT_TIMESTAMP
	ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to query valid auth tokens: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var token, deviceID string
		if err := rows.Scan(&token, &deviceID); err != nil {
			continue
		}
		result[token] = deviceID
	}

	return result, rows.Err()
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

// GetByHash is deprecated - use GetAllValidAuthTokens instead
// This method is kept for backward compatibility but should not be used
// tenantID ensures we only return tokens from this tenant
func (s *AuthTokenStore) GetByHash(ctx context.Context, tenantID, tokenHash string) (deviceID string, err error) {
	if tenantID == "" || tokenHash == "" {
		return "", ErrInvalidInput
	}

	query := `
	SELECT device_id FROM auth_tokens
	WHERE tenant_id = $1 AND token = $2 AND expires_at > CURRENT_TIMESTAMP
	LIMIT 1
	`

	err = s.db.QueryRowContext(ctx, query, tenantID, tokenHash).Scan(&deviceID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get token by hash: %w", err)
	}

	return deviceID, nil
}

// GetByRawHash retrieves the device ID associated with a single hash
// Used internally during token validation or searching
func (s *AuthTokenStore) GetByRawHash(ctx context.Context, rawHash string) (deviceID string, err error) {
	if rawHash == "" {
		return "", ErrInvalidInput
	}

	query := `
	SELECT device_id FROM auth_tokens
	WHERE raw_token_hash = $1 AND expires_at > CURRENT_TIMESTAMP
	LIMIT 1
	`

	err = s.db.QueryRowContext(ctx, query, rawHash).Scan(&deviceID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get token by raw hash: %w", err)
	}

	return deviceID, nil
}

// GetByToken retrieves a token by its raw token string with tenant isolation
// Used to update token expiry on login renewal
// tenantID ensures we only retrieve tokens from this tenant
func (s *AuthTokenStore) GetByToken(ctx context.Context, tenantID, rawToken string) (*models.AuthToken, error) {
	if tenantID == "" || rawToken == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT id, token, device_id, created_at, expires_at
	FROM auth_tokens
	WHERE tenant_id = $1 AND token = $2
	LIMIT 1
	`

	row := s.db.QueryRowContext(ctx, query, tenantID, rawToken)

	token := &models.AuthToken{}
	var id, createdAt, expiresAt string

	err := row.Scan(&id, &token.Token, &token.DeviceID, &createdAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Parse timestamps
	if createdAtTime, err := time.Parse(time.RFC3339, createdAt); err == nil {
		token.CreatedAt = createdAtTime
	}

	if expiresAtTime, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		token.ExpiresAt = expiresAtTime
	}

	return token, nil
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

// GetByDeviceID retrieves all tokens for a device within a tenant
// tenantID + deviceID ensures isolation
func (s *AuthTokenStore) GetByDeviceID(ctx context.Context, tenantID, deviceID string) ([]*models.AuthToken, error) {
	if tenantID == "" || deviceID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT id, token_hash, device_id, created_at, expires_at
	FROM auth_tokens
	WHERE tenant_id = $1 AND device_id = $2
	ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, tenantID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokens for device: %w", err)
	}
	defer rows.Close()

	tokens := make([]*models.AuthToken, 0)
	for rows.Next() {
		token, err := s.scanAuthToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading tokens: %w", err)
	}

	return tokens, nil
}

// IsValid checks if a token hash is valid and not expired within a tenant
// tenantID ensures we only validate tokens from this tenant
func (s *AuthTokenStore) IsValid(ctx context.Context, tenantID, tokenHash string) (bool, error) {
	if tenantID == "" || tokenHash == "" {
		return false, ErrInvalidInput
	}

	var exists bool
	query := `
	SELECT EXISTS(
		SELECT 1 FROM auth_tokens
		WHERE tenant_id = $1 AND token_hash = $2 AND expires_at > CURRENT_TIMESTAMP
	)
	`

	err := s.db.QueryRowContext(ctx, query, tenantID, tokenHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check token validity: %w", err)
	}

	return exists, nil
}

// List retrieves tokens with optional filtering within a tenant
// tenantID filters to only return tokens from this tenant
func (s *AuthTokenStore) List(ctx context.Context, tenantID string, filters *storage.TokenListFilter) ([]*models.AuthToken, error) {
	if tenantID == "" {
		return nil, ErrInvalidInput
	}

	if filters == nil {
		filters = &storage.TokenListFilter{}
	}

	query := `
	SELECT id, token_hash, device_id, created_at, expires_at
	FROM auth_tokens
	WHERE tenant_id = $1
	`

	args := []interface{}{tenantID}
	paramCounter := 2

	if filters.DeviceID != "" {
		query += fmt.Sprintf(" AND device_id = $%d", paramCounter)
		args = append(args, filters.DeviceID)
		paramCounter++
	}

	if !filters.IncludeExpired {
		query += " AND expires_at > CURRENT_TIMESTAMP"
	}

	if !filters.Before.IsZero() {
		query += fmt.Sprintf(" AND created_at < $%d", paramCounter)
		args = append(args, filters.Before.Format(time.RFC3339))
		paramCounter++
	}

	if !filters.After.IsZero() {
		query += fmt.Sprintf(" AND created_at > $%d", paramCounter)
		args = append(args, filters.After.Format(time.RFC3339))
		paramCounter++
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokens: %w", err)
	}
	defer rows.Close()

	tokens := make([]*models.AuthToken, 0)
	for rows.Next() {
		token, err := s.scanAuthToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading tokens: %w", err)
	}

	return tokens, nil
}

// Delete removes a token with tenant isolation
// tenantID ensures we only delete tokens from this tenant
func (s *AuthTokenStore) Delete(ctx context.Context, tenantID, tokenHash string) error {
	if tenantID == "" || tokenHash == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM auth_tokens WHERE tenant_id = $1 AND token_hash = $2", tenantID, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
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

// DeleteExpired removes all expired tokens for a tenant
// tenantID scopes the deletion to this tenant only
func (s *AuthTokenStore) DeleteExpired(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM auth_tokens WHERE tenant_id = $1 AND expires_at <= CURRENT_TIMESTAMP"

	_, err := s.db.ExecContext(ctx, query, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
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

// Helper methods

func (s *AuthTokenStore) scanAuthToken(rows *sql.Rows) (*models.AuthToken, error) {
	token := &models.AuthToken{}
	var id, createdAt, expiresAt string

	err := rows.Scan(
		&id,
		&token.Token,
		&token.DeviceID,
		&createdAt,
		&expiresAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan auth token: %w", err)
	}

	// Parse timestamps
	if createdAtTime, err := time.Parse(time.RFC3339, createdAt); err == nil {
		token.CreatedAt = createdAtTime
	}

	if expiresAtTime, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		token.ExpiresAt = expiresAtTime
	}

	return token, nil
}
