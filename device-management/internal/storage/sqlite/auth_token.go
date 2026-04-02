package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"


	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// AuthTokenStore implements storage.AuthTokenStore for SQLite
type AuthTokenStore struct {
	db *sql.DB
}

// Save persists an auth token to storage
// Stores the raw token (not hashed) so we can hash it during validation
// This matches umh-mock-console approach: GetAllValidAuthTokens retrieves raw tokens
// and hashes them during login to compare with client-sent hash
func (s *AuthTokenStore) Save(ctx context.Context, token *models.AuthToken, rawToken string) error {
	if token == nil || token.DeviceId == "" || rawToken == "" {
		return ErrInvalidInput
	}

	id := generateUUID()
	query := `
	INSERT INTO auth_tokens (id, token, device_id, created_at, expires_at)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		id,
		rawToken,
		token.DeviceId,
		time.Now().Format(time.RFC3339),
		token.ExpiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to save auth token: %w", err)
	}

	return nil
}

// GetAllValidAuthTokens retrieves all non-expired tokens with their device IDs
// Used during login: iterate all tokens, hash each one, compare with client-sent hash
// This matches umh-mock-console's login validation approach
func (s *AuthTokenStore) GetAllValidAuthTokens(ctx context.Context) (map[string]string, error) {
	query := `
	SELECT token, device_id FROM auth_tokens
	WHERE expires_at > datetime('now')
	ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
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

// GetByHash is deprecated - use GetAllValidAuthTokens instead
// This method is kept for backward compatibility but should not be used
func (s *AuthTokenStore) GetByHash(ctx context.Context, tokenHash string) (deviceID string, err error) {
	if tokenHash == "" {
		return "", ErrInvalidInput
	}

	query := `
	SELECT device_id FROM auth_tokens
	WHERE token = ? AND expires_at > datetime('now')
	LIMIT 1
	`

	err = s.db.QueryRowContext(ctx, query, tokenHash).Scan(&deviceID)
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
	WHERE raw_token_hash = ? AND expires_at > datetime('now')
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

// GetByToken retrieves a token by its raw token string
// Used to update token expiry on login renewal
func (s *AuthTokenStore) GetByToken(ctx context.Context, rawToken string) (*models.AuthToken, error) {
	if rawToken == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT id, token, device_id, created_at, expires_at
	FROM auth_tokens
	WHERE token = ?
	LIMIT 1
	`

	row := s.db.QueryRowContext(ctx, query, rawToken)

	token := &models.AuthToken{}
	var id, createdAt, expiresAt string

	err := row.Scan(&id, &token.Token, &token.DeviceId, &createdAt, &expiresAt)
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

// UpdateExpiry updates a token's expiry date
// Used to renew token on successful login
func (s *AuthTokenStore) UpdateExpiry(ctx context.Context, rawToken string, expiresAt time.Time) error {
	if rawToken == "" {
		return ErrInvalidInput
	}

	query := `
	UPDATE auth_tokens
	SET expires_at = ?
	WHERE token = ?
	`

	result, err := s.db.ExecContext(ctx, query, expiresAt.Format(time.RFC3339), rawToken)
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

// GetByDeviceID retrieves all tokens for a device
func (s *AuthTokenStore) GetByDeviceID(ctx context.Context, deviceID string) ([]*models.AuthToken, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT id, token_hash, device_id, created_at, expires_at
	FROM auth_tokens
	WHERE device_id = ?
	ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID)
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

// IsValid checks if a token hash is valid and not expired
func (s *AuthTokenStore) IsValid(ctx context.Context, tokenHash string) (bool, error) {
	if tokenHash == "" {
		return false, ErrInvalidInput
	}

	var exists bool
	query := `
	SELECT EXISTS(
		SELECT 1 FROM auth_tokens
		WHERE token_hash = ? AND expires_at > datetime('now')
	)
	`

	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check token validity: %w", err)
	}

	return exists, nil
}

// List retrieves tokens with optional filtering
func (s *AuthTokenStore) List(ctx context.Context, filters *storage.TokenListFilter) ([]*models.AuthToken, error) {
	if filters == nil {
		filters = &storage.TokenListFilter{}
	}

	query := `
	SELECT id, token_hash, device_id, created_at, expires_at
	FROM auth_tokens
	WHERE 1=1
	`

	args := []interface{}{}

	if filters.DeviceID != "" {
		query += " AND device_id = ?"
		args = append(args, filters.DeviceID)
	}

	if !filters.IncludeExpired {
		query += " AND expires_at > datetime('now')"
	}

	if !filters.Before.IsZero() {
		query += " AND created_at < ?"
		args = append(args, filters.Before.Format(time.RFC3339))
	}

	if !filters.After.IsZero() {
		query += " AND created_at > ?"
		args = append(args, filters.After.Format(time.RFC3339))
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

// Delete removes a token by its hash
func (s *AuthTokenStore) Delete(ctx context.Context, tokenHash string) error {
	if tokenHash == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM auth_tokens WHERE token_hash = ?", tokenHash)
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

// DeleteExpired removes all expired tokens
func (s *AuthTokenStore) DeleteExpired(ctx context.Context) error {
	query := "DELETE FROM auth_tokens WHERE expires_at <= datetime('now')"

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	return nil
}

// DeleteByDeviceID removes all tokens for a device
func (s *AuthTokenStore) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM auth_tokens WHERE device_id = ?", deviceID)
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
		&token.DeviceId,
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


