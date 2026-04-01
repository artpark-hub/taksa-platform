package storage

import (
	"context"

	v2 "taksa-platform-dm/api/umh-core/v2"
)

// DeviceCertificateStore defines the interface for device certificate storage
// Device certificates are device's own certificates for mTLS communication
type DeviceCertificateStore interface {
	// SaveDevice persists a device certificate (one per device)
	SaveDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error

	// GetByDevice retrieves the certificate for a device
	GetByDevice(ctx context.Context, deviceID string) (*v2.Certificate, error)

	// UpdateDevice updates a device certificate
	UpdateDevice(ctx context.Context, deviceID string, certificate *v2.Certificate) error

	// DeleteByDevice removes device certificate for a device
	DeleteByDevice(ctx context.Context, deviceID string) error

	// CleanupExpired removes expired device certificates
	CleanupExpired(ctx context.Context) error
}

// UserCertificateStore defines the interface for user certificate storage
// User certificates are for individual users on a device
type UserCertificateStore interface {
	// SaveUser persists a user certificate (multiple per device)
	SaveUser(ctx context.Context, certificate *v2.Certificate) error

	// GetByID retrieves a user certificate by ID
	GetByID(ctx context.Context, id string) (*v2.Certificate, error)

	// GetByDeviceAndEmail retrieves a certificate for a device and user email
	GetByDeviceAndEmail(ctx context.Context, deviceID, email string) (*v2.Certificate, error)

	// GetByEmail retrieves a certificate by user email
	GetByEmail(ctx context.Context, email string) (*v2.Certificate, error)

	// ListByDevice retrieves all user certificates for a device
	ListByDevice(ctx context.Context, deviceID string) ([]*v2.Certificate, error)

	// UpdateUser updates a user certificate
	UpdateUser(ctx context.Context, certificate *v2.Certificate) error

	// DeleteByID removes a user certificate by ID
	DeleteByID(ctx context.Context, id string) error

	// DeleteByDevice removes all user certificates for a device
	DeleteByDevice(ctx context.Context, deviceID string) error

	// DeleteByDeviceAndEmail removes a specific user certificate
	DeleteByDeviceAndEmail(ctx context.Context, deviceID, email string) error

	// CleanupExpired removes all expired user certificates
	CleanupExpired(ctx context.Context) error

	// Exists checks if a user certificate exists for a device and email
	Exists(ctx context.Context, deviceID, email string) (bool, error)
}

// CertificateStore is a combined interface for both device and user certificates
type CertificateStore interface {
	DeviceCertificateStore
	UserCertificateStore
	// Helper methods to access individual stores
	DeviceStore() DeviceCertificateStore
	UserStore() UserCertificateStore
}
