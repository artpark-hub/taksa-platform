package storage

import (
	"context"
	"time"

	v1 "taksa-platform-dm/api/devicemgmt/v1"
)

// DeviceStore defines the interface for device storage operations
type DeviceStore interface {
	// Save persists a device to storage
	Save(ctx context.Context, device *v1.Device) error

	// GetByID retrieves a device by its ID
	GetByID(ctx context.Context, id string) (*v1.Device, error)

	// GetBySerialNumber retrieves a device by serial number
	GetBySerialNumber(ctx context.Context, serialNumber string) (*v1.Device, error)

	// GetByUUID retrieves a device by UUID
	GetByUUID(ctx context.Context, uuid string) (*v1.Device, error)

	// List retrieves devices with optional filtering and cursor pagination
	// Returns up to page_size+1 results (extra result indicates more pages exist)
	List(ctx context.Context, filters *DeviceListFilter) ([]*v1.Device, error)

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
}

// DeviceListFilter defines filtering and pagination for device listing
type DeviceListFilter struct {
	PageSize       int32                // Items per page
	Offset         int32                // Calculated from page_token
	StatusFilter   *v1.DeviceStatus     // Filter by status (nil = all)
	LocationFilter string               // Filter by company/plant
	Search         string               // Search in name, serial number, etc.
	SortBy         string               // "name", "created_at", "last_seen"
	SortDesc       bool                 // Sort descending
}

// DefaultDeviceListFilter returns default filter values
func DefaultDeviceListFilter() *DeviceListFilter {
	return &DeviceListFilter{
		PageSize: 50,
		SortBy:   "created_at",
		SortDesc: true,
	}
}
