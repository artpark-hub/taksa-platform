package storage

import (
	"context"
	"time"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
)

// DeviceStore defines the interface for device storage operations
type DeviceStore interface {
	// Save persists a device to storage
	Save(ctx context.Context, device *v1.Device) error

	// GetByID retrieves a device by its ID
	GetByID(ctx context.Context, id string) (*v1.Device, error)

	// GetByCreatedByAndName retrieves a device by created_by (tenant) and name (per-tenant unique)
	GetByCreatedByAndName(ctx context.Context, createdBy, name string) (*v1.Device, error)

	// GetByUUID retrieves a device by UUID
	GetByUUID(ctx context.Context, uuid string) (*v1.Device, error)

	// List retrieves devices with optional filtering and cursor pagination
	// Returns up to page_size+1 results (extra result indicates more pages exist)
	List(ctx context.Context, filters *DeviceListFilter) ([]*v1.Device, error)

	// ListSummaries retrieves device summaries with optional filtering and pagination
	ListSummaries(ctx context.Context, filters *DeviceListFilter) ([]*v1.DeviceSummary, error)

	// Update updates an existing device
	Update(ctx context.Context, device *v1.Device) error

	// Delete removes a device by ID
	Delete(ctx context.Context, id string) error

	// UpdateStatus updates only the device status
	UpdateStatus(ctx context.Context, id string, status v1.DeviceStatus) error

	// UpdateLastSeen updates the last_seen timestamp
	UpdateLastSeen(ctx context.Context, id string, timestamp time.Time) error

	// UpdateLastLogin updates the last login timestamp
	UpdateLastLogin(ctx context.Context, id string, timestamp time.Time) error

	// UpdateAuthTokenExpiresAt updates only the auth_token_expires_at timestamp
	UpdateAuthTokenExpiresAt(ctx context.Context, id string, timestamp time.Time) error

	// NATSMirrorDeployed returns true if UNS-to-NATS-mirror was successfully deployed on this device.
	NATSMirrorDeployed(ctx context.Context, deviceID string) (bool, error)

	// GetNATSMirrorConfigFingerprint returns the fingerprint of NATS URLs last applied (empty if never set).
	GetNATSMirrorConfigFingerprint(ctx context.Context, deviceID string) (string, error)

	// SetNATSMirrorApplied records successful deploy or edit of UNS-to-NATS-mirror on the device row.
	SetNATSMirrorApplied(ctx context.Context, deviceID string, deployedAt time.Time, configFingerprint string) error

	// ClearNATSMirrorApplied clears mirror deploy state (e.g. after edit fails because DFC missing on edge).
	ClearNATSMirrorApplied(ctx context.Context, deviceID string) error

	// ListNATSMirrorDevicesNeedingUpdate returns devices needing mirror deploy or edit (never applied or stale fingerprint).
	ListNATSMirrorDevicesNeedingUpdate(ctx context.Context, currentFingerprint string) ([]NATSMirrorDeviceRef, error)
}

// NATSMirrorDeviceRef identifies a device for fleet NATS mirror reconciliation.
type NATSMirrorDeviceRef struct {
	ID       string
	TenantID string
}

// DeviceListFilter defines filtering and pagination for device listing
type DeviceListFilter struct {
	PageSize       int32              // Items per page
	Offset         int32              // Calculated from page_token
	StatusFilters  []v1.DeviceStatus  // Filter by status (empty = all)
	LocationFilter string             // Filter by company/plant
	Search         string             // Search in device name
	CreatedBy      string             // Filter by device owner/tenant UUID
	SortBy         string             // "name", "created_at", "last_seen", "created_by"
	SortDesc       bool               // Sort descending
}

// DefaultDeviceListFilter returns default filter values
func DefaultDeviceListFilter() *DeviceListFilter {
	return &DeviceListFilter{
		PageSize: 50,
		SortBy:   "created_at",
		SortDesc: true,
	}
}
