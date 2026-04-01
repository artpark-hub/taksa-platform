package storage

import (
	"context"

	"taksa-platform-dm/internal/models"
)

// ActionStore defines the interface for action storage operations
type ActionStore interface {
	// Save persists an action to storage
	Save(ctx context.Context, action *models.Action) error

	// GetByID retrieves an action by its ID
	GetByID(ctx context.Context, id string) (*models.Action, error)

	// GetByDeviceID retrieves all actions for a device
	GetByDeviceID(ctx context.Context, deviceID string) ([]*models.Action, error)

	// ListForDevice retrieves actions for a device with filtering
	ListForDevice(ctx context.Context, deviceID string, filters *ActionListFilter) ([]*models.Action, int32, error)

	// ListPending retrieves all pending (QUEUED) actions for a device
	ListPending(ctx context.Context, deviceID string) ([]*models.Action, error)

	// UpdateStatus updates an action's status
	UpdateStatus(ctx context.Context, id string, status models.ActionStatus) error

	// UpdateErrorMessage updates the error message for an action
	UpdateErrorMessage(ctx context.Context, id string, errorMessage string) error

	// MarkDelivered marks an action as delivered
	MarkDelivered(ctx context.Context, id string) error

	// MarkCompleted marks an action as completed
	MarkCompleted(ctx context.Context, id string) error

	// MarkFailed marks an action as failed
	MarkFailed(ctx context.Context, id string) error

	// IncrementRetry increments the retry count for an action
	IncrementRetry(ctx context.Context, id string) error

	// Delete removes an action
	Delete(ctx context.Context, id string) error

	// DeleteByDeviceID removes all actions for a device
	DeleteByDeviceID(ctx context.Context, deviceID string) error

	// CleanupExpired removes all expired actions
	CleanupExpired(ctx context.Context) error
}

// ActionListFilter defines filtering and pagination for action listing
type ActionListFilter struct {
	Page           int32                   // Page number (1-indexed)
	PageSize       int32                   // Items per page
	StatusFilter   *models.ActionStatus    // Filter by status (nil = all)
	IncludeHistory bool                    // Include completed/failed actions
	SortBy         string                  // "created_at", "expires_at", "status"
	SortDesc       bool                    // Sort descending
}
