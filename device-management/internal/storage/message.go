package storage

import (
	"context"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// MessageStore defines the interface for message storage operations
type MessageStore interface {
	// Save persists a message to storage with tenant isolation
	// tenantID is required for multi-tenancy isolation
	Save(ctx context.Context, tenantID string, message *models.Message) error

	// GetByID retrieves a message by its ID
	GetByID(ctx context.Context, id string) (*models.Message, error)

	// GetByDeviceID retrieves all messages for a device
	GetByDeviceID(ctx context.Context, deviceID string) ([]*models.Message, error)

	// ListHistory retrieves message history with filtering and pagination
	ListHistory(ctx context.Context, filters *MessageListFilter) ([]*models.Message, int32, error)

	// GetRecentByDevice retrieves recent messages for a device
	GetRecentByDevice(ctx context.Context, deviceID string, limit int32) ([]*models.Message, error)

	// Delete removes a message by ID
	Delete(ctx context.Context, id string) error

	// DeleteByDeviceID removes all messages for a device
	DeleteByDeviceID(ctx context.Context, deviceID string) error

	// CleanupOld removes messages created before the specified time
	CleanupOld(ctx context.Context, before time.Time) error

	// CleanupExpired removes all expired messages
	CleanupExpired(ctx context.Context) error

	// CountByDevice returns the number of messages for a device
	CountByDevice(ctx context.Context, deviceID string) (int32, error)

	// CountByDeviceAndDirection returns the number of messages by direction
	CountByDeviceAndDirection(ctx context.Context, deviceID string, direction int32) (int32, error)

	// GetByTraceID retrieves messages by trace ID
	GetByTraceID(ctx context.Context, traceID string) ([]*models.Message, error)
}

// MessageListFilter defines filtering and pagination for message listing
type MessageListFilter struct {
	DeviceID    string // Filter by device ID (required)
	MessageType string // Filter by message type
	Direction   *int32 // Filter by direction (nil = all)
	Page        int32                    // Page number (1-indexed)
	PageSize    int32                    // Items per page
	SortBy      string                   // "created_at", "type"
	SortDesc    bool                     // Sort descending
}

// DefaultMessageListFilter returns default filter values
func DefaultMessageListFilter() *MessageListFilter {
	return &MessageListFilter{
		Page:     1,
		PageSize: 50,
		SortBy:   "created_at",
		SortDesc: true,
	}
}
