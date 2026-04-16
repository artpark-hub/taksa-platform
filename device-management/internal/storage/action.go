package storage

import (
	"context"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// ActionStore defines the interface for action storage operations
type ActionStore interface {
	// Save persists an action to storage (tenantID should be set in action.CreatedBy for device-facing, or via context for admin)
	Save(ctx context.Context, tenantID string, action *models.Action) error

	// GetByID retrieves an action by its ID
	GetByID(ctx context.Context, tenantID, id string) (*models.Action, error)

	// GetByDeviceID retrieves all actions for a device
	GetByDeviceID(ctx context.Context, tenantID, deviceID string) ([]*models.Action, error)

	// ListForDevice retrieves actions for a device with filtering
	ListForDevice(ctx context.Context, deviceID string, filters *ActionListFilter) ([]*models.Action, int32, error)

	// ListPending retrieves all pending (QUEUED) actions for a device
	ListPending(ctx context.Context, tenantID, deviceID string) ([]*models.Action, error)

	// UpdateStatus updates an action's status
	UpdateStatus(ctx context.Context, tenantID, id string, status models.ActionStatus) error

	// UpdateErrorMessage updates the error message for an action
	UpdateErrorMessage(ctx context.Context, tenantID, id string, errorMessage string) error

	// MarkDelivered marks an action as delivered
	MarkDelivered(ctx context.Context, tenantID, id string) error

	// MarkCompleted marks an action as completed
	MarkCompleted(ctx context.Context, tenantID, id string) error

	// MarkFailed marks an action as failed
	MarkFailed(ctx context.Context, tenantID, id string) error

	// IncrementRetry increments the retry count for an action
	IncrementRetry(ctx context.Context, tenantID, id string) error

	// Delete removes an action
	Delete(ctx context.Context, tenantID, id string) error

	// DeleteByDeviceID removes all actions for a device
	DeleteByDeviceID(ctx context.Context, tenantID, deviceID string) error

	// CleanupExpired removes all expired actions
	CleanupExpired(ctx context.Context) error
}

// ActionListFilter defines filtering and pagination for action listing
type ActionListFilter struct {
	TenantID       string                  // Filter by tenant (required for multi-tenancy)
	Page           int32                   // Page number (1-indexed)
	PageSize       int32                   // Items per page
	StatusFilter   *models.ActionStatus    // Filter by status (nil = all)
	IncludeHistory bool                    // Include completed/failed actions
	SortBy         string                  // "created_at", "expires_at", "status"
	SortDesc       bool                    // Sort descending
}
