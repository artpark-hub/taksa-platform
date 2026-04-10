package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
)

// UserCertificateStore implements storage.UserCertificateStore for PostgreSQL
type UserCertificateStore struct {
	db *sql.DB
}

// SaveUser persists a user certificate
// Note: deviceID must be passed separately as the Certificate proto doesn't include it
func (s *UserCertificateStore) SaveUser(ctx context.Context, certificate *v2.Certificate) error {
	if certificate == nil || certificate.UserEmail == "" || certificate.Certificate == "" {
		return ErrInvalidInput
	}

	// For now, we need a separate method that takes deviceID
	// This is a limitation of using the API proto for storage
	// A better solution would be to create an internal storage struct
	return fmt.Errorf("SaveUser requires deviceID - use SaveUserWithDevice instead")
}

// SaveUserWithDevice persists a user certificate with deviceID
func (s *UserCertificateStore) SaveUserWithDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	if deviceID == "" || certificate == nil || certificate.UserEmail == "" || certificate.Certificate == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO user_certificates (
		id, device_id, user_email, certificate, private_key, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT(device_id, user_email) DO UPDATE SET
		certificate = EXCLUDED.certificate,
		private_key = EXCLUDED.private_key,
		expires_at = EXCLUDED.expires_at
	`

	// Expire in 1 year
	expiresAt := time.Now().AddDate(1, 0, 0)

	_, err := s.db.ExecContext(ctx, query,
		deviceID+"_"+certificate.UserEmail, // Simple ID generation
		deviceID,
		certificate.UserEmail,
		certificate.Certificate,
		"", // Empty private key for now
		expiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to save user certificate: %w", err)
	}

	return nil
}

// GetByID retrieves a user certificate by ID
func (s *UserCertificateStore) GetByID(ctx context.Context, id string) (*v2.Certificate, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}

	return s.getCertificate(ctx, "WHERE id = $1", id)
}

// GetByDeviceAndEmail retrieves a certificate for a device and user email
func (s *UserCertificateStore) GetByDeviceAndEmail(ctx context.Context, deviceID, email string) (*v2.Certificate, error) {
	if deviceID == "" || email == "" {
		return nil, ErrInvalidInput
	}

	return s.getCertificate(ctx, "WHERE device_id = $1 AND user_email = $2", deviceID, email)
}

// GetByEmail retrieves a certificate by user email
func (s *UserCertificateStore) GetByEmail(ctx context.Context, email string) (*v2.Certificate, error) {
	if email == "" {
		return nil, ErrInvalidInput
	}

	return s.getCertificate(ctx, "WHERE user_email = $1", email)
}

// ListByDevice retrieves all user certificates for a device
func (s *UserCertificateStore) ListByDevice(ctx context.Context, deviceID string) ([]*v2.Certificate, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	return s.listCertificates(ctx,
		"WHERE device_id = $1 ORDER BY created_at DESC", deviceID)
}

// UpdateUser updates a user certificate (requires deviceID and certificate.UserEmail)
func (s *UserCertificateStore) UpdateUser(ctx context.Context, certificate *v2.Certificate) error {
	return fmt.Errorf("UpdateUser requires deviceID - use UpdateUserWithDevice instead")
}

// UpdateUserWithDevice updates a user certificate with deviceID
func (s *UserCertificateStore) UpdateUserWithDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	if deviceID == "" || certificate == nil || certificate.UserEmail == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE user_certificates SET certificate = $1 WHERE device_id = $2 AND user_email = $3`,
		certificate.Certificate,
		deviceID, certificate.UserEmail)

	if err != nil {
		return fmt.Errorf("failed to update user certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByID removes a user certificate by ID
func (s *UserCertificateStore) DeleteByID(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM user_certificates WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDevice removes all user certificates for a device
func (s *UserCertificateStore) DeleteByDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM user_certificates WHERE device_id = $1", deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete user certificates for device: %w", err)
	}

	return nil
}

// DeleteByDeviceAndEmail removes a specific user certificate
func (s *UserCertificateStore) DeleteByDeviceAndEmail(ctx context.Context, deviceID, email string) error {
	if deviceID == "" || email == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"DELETE FROM user_certificates WHERE device_id = $1 AND user_email = $2",
		deviceID, email)
	if err != nil {
		return fmt.Errorf("failed to delete user certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// CleanupExpired removes all expired user certificates
func (s *UserCertificateStore) CleanupExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM user_certificates WHERE expires_at < NOW() AND expires_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired user certificates: %w", err)
	}

	return nil
}

// Exists checks if a user certificate exists for a device and email
func (s *UserCertificateStore) Exists(ctx context.Context, deviceID, email string) (bool, error) {
	if deviceID == "" || email == "" {
		return false, ErrInvalidInput
	}

	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM user_certificates WHERE device_id = $1 AND user_email = $2)",
		deviceID, email).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check user certificate existence: %w", err)
	}

	return exists, nil
}

// getCertificate is a helper to retrieve a single user certificate
func (s *UserCertificateStore) getCertificate(ctx context.Context, whereClause string, args ...interface{}) (*v2.Certificate, error) {
	cert := &v2.Certificate{}
	var userEmail sql.NullString

	query := `SELECT user_email, certificate FROM user_certificates ` + whereClause

	row := s.db.QueryRowContext(ctx, query, args...)
	err := row.Scan(&userEmail, &cert.Certificate)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user certificate: %w", err)
	}

	// Set user email
	if userEmail.Valid {
		cert.UserEmail = userEmail.String
	}

	return cert, nil
}

// listCertificates is a helper to retrieve multiple user certificates
func (s *UserCertificateStore) listCertificates(ctx context.Context, whereClause string, args ...interface{}) ([]*v2.Certificate, error) {
	query := `SELECT user_email, certificate FROM user_certificates ` + whereClause

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user certificates: %w", err)
	}
	defer rows.Close()

	var certs []*v2.Certificate
	for rows.Next() {
		cert := &v2.Certificate{}
		var userEmail sql.NullString

		err := rows.Scan(&userEmail, &cert.Certificate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user certificate: %w", err)
		}

		// Set user email
		if userEmail.Valid {
			cert.UserEmail = userEmail.String
		}

		certs = append(certs, cert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return certs, nil
}
