package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
)

// UserCertificateStore implements storage.UserCertificateStore for PostgreSQL
type UserCertificateStore struct {
	db *sql.DB
}

// SaveUser persists a user certificate with tenant isolation
func (s *UserCertificateStore) SaveUser(ctx context.Context, tenantID string, certificate *v2.Certificate) error {
	if certificate == nil || certificate.UserEmail == "" || certificate.Certificate == "" || tenantID == "" {
		return ErrInvalidInput
	}

	deviceID := middleware.GetDeviceID(ctx)
	if deviceID == "" {
		return fmt.Errorf("device_id not found in context")
	}

	return s.SaveUserWithDevice(ctx, tenantID, deviceID, certificate)
}

// SaveUserWithDevice persists a user certificate with tenantID and deviceID
func (s *UserCertificateStore) SaveUserWithDevice(ctx context.Context, tenantID, deviceID string, certificate *v2.Certificate) error {
	if tenantID == "" || deviceID == "" || certificate == nil || certificate.UserEmail == "" || certificate.Certificate == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO user_certificates (
		id, device_id, tenant_id, user_email, certificate, private_key, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT(device_id, tenant_id, user_email) DO UPDATE SET
		certificate = EXCLUDED.certificate,
		private_key = EXCLUDED.private_key,
		expires_at = EXCLUDED.expires_at
	WHERE user_certificates.tenant_id = $3
	`

	// Expire in 1 year
	expiresAt := time.Now().AddDate(1, 0, 0)

	_, err := s.db.ExecContext(ctx, query,
		deviceID+"_"+certificate.UserEmail, // Simple ID generation
		deviceID,
		tenantID,
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

// GetByID retrieves a user certificate by ID with tenant isolation
func (s *UserCertificateStore) GetByID(ctx context.Context, id string) (*v2.Certificate, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		return s.getCertificate(ctx, "WHERE id = $1 AND tenant_id = $2", id, tenantID)
	}
	return s.getCertificate(ctx, "WHERE id = $1", id)
}

// GetByDeviceAndEmail retrieves a certificate for a device and user email with tenant isolation
func (s *UserCertificateStore) GetByDeviceAndEmail(ctx context.Context, deviceID, email string) (*v2.Certificate, error) {
	if deviceID == "" || email == "" {
		return nil, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		return s.getCertificate(ctx, "WHERE device_id = $1 AND tenant_id = $2 AND user_email = $3", deviceID, tenantID, email)
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

// ListByDevice retrieves all user certificates for a device with tenant isolation
func (s *UserCertificateStore) ListByDevice(ctx context.Context, deviceID string) ([]*v2.Certificate, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		return s.listCertificates(ctx,
			"WHERE device_id = $1 AND tenant_id = $2 ORDER BY created_at DESC", deviceID, tenantID)
	}
	return s.listCertificates(ctx,
		"WHERE device_id = $1 ORDER BY created_at DESC", deviceID)
}

// UpdateUser updates a user certificate with tenant isolation
func (s *UserCertificateStore) UpdateUser(ctx context.Context, tenantID string, certificate *v2.Certificate) error {
	if tenantID == "" || certificate == nil || certificate.UserEmail == "" {
		return ErrInvalidInput
	}

	deviceID := middleware.GetDeviceID(ctx)
	if deviceID == "" {
		return fmt.Errorf("device_id not found in context")
	}

	return s.UpdateUserWithDevice(ctx, tenantID, deviceID, certificate)
}

// UpdateUserWithDevice updates a user certificate with tenantID and deviceID
func (s *UserCertificateStore) UpdateUserWithDevice(ctx context.Context, tenantID, deviceID string, certificate *v2.Certificate) error {
	if tenantID == "" || deviceID == "" || certificate == nil || certificate.UserEmail == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE user_certificates SET certificate = $1 WHERE device_id = $2 AND tenant_id = $3 AND user_email = $4`,
		certificate.Certificate,
		deviceID, tenantID, certificate.UserEmail)

	if err != nil {
		return fmt.Errorf("failed to update user certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByID removes a user certificate by ID with tenant isolation
func (s *UserCertificateStore) DeleteByID(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM user_certificates WHERE id = $1"
	args := []interface{}{id}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM user_certificates WHERE id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete user certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDevice removes all user certificates for a device with tenant isolation
func (s *UserCertificateStore) DeleteByDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM user_certificates WHERE device_id = $1"
	args := []interface{}{deviceID}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM user_certificates WHERE device_id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete user certificates for device: %w", err)
	}

	return nil
}

// DeleteByDeviceAndEmail removes a specific user certificate with tenant isolation
func (s *UserCertificateStore) DeleteByDeviceAndEmail(ctx context.Context, deviceID, email string) error {
	if deviceID == "" || email == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM user_certificates WHERE device_id = $1 AND user_email = $2"
	args := []interface{}{deviceID, email}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM user_certificates WHERE device_id = $1 AND tenant_id = $2 AND user_email = $3"
		args = []interface{}{deviceID, tenantID, email}
	}
	result, err := s.db.ExecContext(ctx, query, args...)
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

	query := "SELECT EXISTS(SELECT 1 FROM user_certificates WHERE device_id = $1 AND user_email = $2)"
	args := []interface{}{deviceID, email}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "SELECT EXISTS(SELECT 1 FROM user_certificates WHERE device_id = $1 AND user_email = $2 AND tenant_id = $3)"
		args = append(args, tenantID)
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&exists)

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
