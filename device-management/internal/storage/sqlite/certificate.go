package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	v2 "taksa-platform-dm/api/umh-core/v2"
	"taksa-platform-dm/internal/storage"
)

// CertificateStore implements storage.CertificateStore for SQLite
// It wraps both DeviceCertificateStore and UserCertificateStore
type CertificateStore struct {
	db     *sql.DB
	device *DeviceCertificateStore
	user   *UserCertificateStore
}

// NewCertificateStore creates a new CertificateStore combining device and user certificates
func NewCertificateStore(db *sql.DB) *CertificateStore {
	return &CertificateStore{
		db:     db,
		device: &DeviceCertificateStore{db: db},
		user:   &UserCertificateStore{db: db},
	}
}

// DeviceStore returns the device certificate store (implements interface method)
func (s *CertificateStore) DeviceStore() storage.DeviceCertificateStore {
	return s.device
}

// UserStore returns the user certificate store (implements interface method)
func (s *CertificateStore) UserStore() storage.UserCertificateStore {
	return s.user
}

// Save persists a certificate to storage
// For device certificates (empty userEmail), use SaveDeviceCert()
// For user certificates (with userEmail), use SaveUserCert(deviceID, certificate)
func (s *CertificateStore) Save(ctx context.Context, certificate *v2.Certificate) error {
	if certificate == nil {
		return ErrInvalidInput
	}
	// This is a legacy method - use SaveDeviceCert or SaveUserCert instead
	return fmt.Errorf("use SaveDeviceCert or SaveUserCert methods instead of Save")
}

// Implement DeviceCertificateStore methods
// SaveDevice persists a device certificate
func (s *CertificateStore) SaveDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	return s.device.SaveDevice(ctx, deviceID, certificate)
}

// UpdateDevice updates a device certificate
func (s *CertificateStore) UpdateDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	return s.device.UpdateDevice(ctx, deviceID, certificate)
}

// Implement UserCertificateStore methods
// SaveUser persists a user certificate
func (s *CertificateStore) SaveUser(ctx context.Context, certificate *v2.Certificate) error {
	return s.user.SaveUser(ctx, certificate)
}

// UpdateUser updates a user certificate
func (s *CertificateStore) UpdateUser(ctx context.Context, certificate *v2.Certificate) error {
	return s.user.UpdateUser(ctx, certificate)
}

// SaveUserCert persists a user certificate with deviceID (helper)
func (s *CertificateStore) SaveUserCert(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	return s.user.SaveUserWithDevice(ctx, deviceID, certificate)
}

// SaveDeviceCert persists a device certificate (helper - same as SaveDevice)
func (s *CertificateStore) SaveDeviceCert(ctx context.Context, deviceID string, certificate *v2.Certificate) error {
	return s.device.SaveDevice(ctx, deviceID, certificate)
}

// GetByID delegates to user store (device certs don't have IDs)
func (s *CertificateStore) GetByID(ctx context.Context, id string) (*v2.Certificate, error) {
	return s.user.GetByID(ctx, id)
}

// GetByDevice retrieves device certificate
func (s *CertificateStore) GetByDevice(ctx context.Context, deviceID string) (*v2.Certificate, error) {
	return s.device.GetByDevice(ctx, deviceID)
}

// GetByDeviceAndEmail retrieves user certificate for device and email
func (s *CertificateStore) GetByDeviceAndEmail(ctx context.Context, deviceID, email string) (*v2.Certificate, error) {
	return s.user.GetByDeviceAndEmail(ctx, deviceID, email)
}

// GetByEmail retrieves user certificate by email
func (s *CertificateStore) GetByEmail(ctx context.Context, email string) (*v2.Certificate, error) {
	return s.user.GetByEmail(ctx, email)
}

// ListByDevice retrieves all user certificates for a device
func (s *CertificateStore) ListByDevice(ctx context.Context, deviceID string) ([]*v2.Certificate, error) {
	return s.user.ListByDevice(ctx, deviceID)
}

// Update updates a certificate (legacy method - use UpdateDevice or UpdateUser instead)
func (s *CertificateStore) Update(ctx context.Context, certificate *v2.Certificate) error {
	if certificate == nil {
		return ErrInvalidInput
	}
	// Legacy method - not recommended
	return fmt.Errorf("use UpdateDevice or UpdateUser methods instead of Update")
}

// DeleteByID removes a user certificate by ID
func (s *CertificateStore) DeleteByID(ctx context.Context, id string) error {
	return s.user.DeleteByID(ctx, id)
}

// Delete delegates to DeleteByID for backward compatibility
func (s *CertificateStore) Delete(ctx context.Context, id string) error {
	return s.DeleteByID(ctx, id)
}

// DeleteByDevice removes both device and user certificates for a device
func (s *CertificateStore) DeleteByDevice(ctx context.Context, deviceID string) error {
	if err := s.device.DeleteByDevice(ctx, deviceID); err != nil && err != ErrNotFound {
		return err
	}
	if err := s.user.DeleteByDevice(ctx, deviceID); err != nil && err != ErrNotFound {
		return err
	}
	return nil
}

// DeleteByDeviceAndEmail removes a user certificate
func (s *CertificateStore) DeleteByDeviceAndEmail(ctx context.Context, deviceID, email string) error {
	return s.user.DeleteByDeviceAndEmail(ctx, deviceID, email)
}

// CleanupExpired removes expired certificates from both tables
func (s *CertificateStore) CleanupExpired(ctx context.Context) error {
	if err := s.device.CleanupExpired(ctx); err != nil {
		return err
	}
	if err := s.user.CleanupExpired(ctx); err != nil {
		return err
	}
	return nil
}

// Exists checks if user certificate exists
func (s *CertificateStore) Exists(ctx context.Context, deviceID, email string) (bool, error) {
	return s.user.Exists(ctx, deviceID, email)
}
