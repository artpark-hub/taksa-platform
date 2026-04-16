package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
)

// DeviceCertificateStore implements storage.DeviceCertificateStore for PostgreSQL
type DeviceCertificateStore struct {
	db *sql.DB
}

// SaveDevice persists a device certificate (one per device) with tenant isolation
func (s *DeviceCertificateStore) SaveDevice(ctx context.Context, tenantID, deviceID string, certificate *v2.Certificate) error {
	if tenantID == "" || deviceID == "" || certificate == nil || certificate.Certificate == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO device_certificates (
		device_id, tenant_id, certificate, private_key, expires_at
	) VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT(device_id) DO UPDATE SET
		certificate = EXCLUDED.certificate,
		private_key = EXCLUDED.private_key,
		expires_at = EXCLUDED.expires_at
	WHERE device_certificates.tenant_id = $2
	`

	// Expire in 1 year if not specified
	expiresAt := time.Now().AddDate(1, 0, 0)

	_, err := s.db.ExecContext(ctx, query,
		deviceID,
		tenantID,
		certificate.Certificate,
		"", // Empty private key for now
		expiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to save device certificate: %w", err)
	}

	return nil
}

// GetByDevice retrieves the certificate for a device with tenant isolation
func (s *DeviceCertificateStore) GetByDevice(ctx context.Context, deviceID string) (*v2.Certificate, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	cert := &v2.Certificate{}

	query := `SELECT certificate FROM device_certificates WHERE device_id = $1`
	args := []interface{}{deviceID}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = `SELECT certificate FROM device_certificates WHERE device_id = $1 AND tenant_id = $2`
		args = append(args, tenantID)
	}
	row := s.db.QueryRowContext(ctx, query, args...)

	err := row.Scan(&cert.Certificate)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device certificate: %w", err)
	}

	return cert, nil
}

// UpdateDevice updates a device certificate with tenant isolation
func (s *DeviceCertificateStore) UpdateDevice(ctx context.Context, tenantID, deviceID string, certificate *v2.Certificate) error {
	if tenantID == "" || deviceID == "" || certificate == nil {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE device_certificates SET certificate = $1 WHERE device_id = $2 AND tenant_id = $3`,
		certificate.Certificate,
		deviceID,
		tenantID)

	if err != nil {
		return fmt.Errorf("failed to update device certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDevice removes device certificate for a device with tenant isolation
func (s *DeviceCertificateStore) DeleteByDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM device_certificates WHERE device_id = $1"
	args := []interface{}{deviceID}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM device_certificates WHERE device_id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete device certificate: %w", err)
	}

	return nil
}

// CleanupExpired removes expired device certificates
func (s *DeviceCertificateStore) CleanupExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM device_certificates WHERE expires_at < NOW() AND expires_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired device certificates: %w", err)
	}

	return nil
}
