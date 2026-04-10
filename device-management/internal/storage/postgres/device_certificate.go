package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
)

// DeviceCertificateStore implements storage.DeviceCertificateStore for PostgreSQL
type DeviceCertificateStore struct {
	db *sql.DB
}

// SaveDevice persists a device certificate (one per device)
func (s *DeviceCertificateStore) SaveDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	if deviceID == "" || certificate == nil || certificate.Certificate == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO device_certificates (
		device_id, certificate, private_key, expires_at
	) VALUES ($1, $2, $3, $4)
	ON CONFLICT(device_id) DO UPDATE SET
		certificate = EXCLUDED.certificate,
		private_key = EXCLUDED.private_key,
		expires_at = EXCLUDED.expires_at
	`

	// Expire in 1 year if not specified
	expiresAt := time.Now().AddDate(1, 0, 0)

	_, err := s.db.ExecContext(ctx, query,
		deviceID,
		certificate.Certificate,
		"", // Empty private key for now
		expiresAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to save device certificate: %w", err)
	}

	return nil
}

// GetByDevice retrieves the certificate for a device
func (s *DeviceCertificateStore) GetByDevice(ctx context.Context, deviceID string) (*v2.Certificate, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	cert := &v2.Certificate{}

	row := s.db.QueryRowContext(ctx,
		`SELECT certificate FROM device_certificates WHERE device_id = $1`,
		deviceID)

	err := row.Scan(&cert.Certificate)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device certificate: %w", err)
	}

	return cert, nil
}

// UpdateDevice updates a device certificate
func (s *DeviceCertificateStore) UpdateDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	if deviceID == "" || certificate == nil {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE device_certificates SET certificate = $1 WHERE device_id = $2`,
		certificate.Certificate,
		deviceID)

	if err != nil {
		return fmt.Errorf("failed to update device certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDevice removes device certificate for a device
func (s *DeviceCertificateStore) DeleteByDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM device_certificates WHERE device_id = $1", deviceID)
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
