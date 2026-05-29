package biz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

const cancelledByUserMessage = "Cancelled by user"

// ActionUsecase handles action business logic (queuing, status tracking)
type ActionUsecase struct {
	store storage.Store
}

// NewActionUsecase creates a new action use case
func NewActionUsecase(store storage.Store) *ActionUsecase {
	return &ActionUsecase{
		store: store,
	}
}

// QueueAction queues a new action for a device
func (uc *ActionUsecase) QueueAction(ctx context.Context, req *QueueActionRequest) (*models.Action, error) {
	// Validate
	if req.DeviceID == "" {
		return nil, fmt.Errorf("device ID is empty")
	}
	if req.ActionType == "" {
		return nil, fmt.Errorf("action type is empty")
	}

	// Validate payload is present and not empty
	// Some actions (like get_config) don't require a payload
	// Only validate if a payload is provided
	if req.Payload != nil && (req.Payload.Value == nil || len(req.Payload.Value) == 0) {
		return nil, fmt.Errorf("action payload cannot be empty if provided")
	}

	// Get tenant_id from context (set by middleware from JWT)
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id not found in context")
	}

	// Check device exists
	_, err := uc.store.Devices().GetByID(ctx, req.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Create action
	action := &models.Action{
		Id:         generateUUID(),
		DeviceId:   req.DeviceID,
		Type:       req.ActionType,
		Payload:    req.Payload,
		MaxRetries: req.MaxRetries,
		RetryCount: 0,
		Status:     models.ActionStatusQueued,
		CreatedAt:  time.Now(),
	}

	// Set expiry if TTL provided
	if req.TTLSeconds > 0 {
		action.ExpiresAt = time.Now().Add(time.Duration(req.TTLSeconds) * time.Second)
	}

	// Save with tenant isolation
	if err := uc.store.Actions().Save(ctx, tenantID, action); err != nil {
		return nil, fmt.Errorf("failed to save action: %w", err)
	}

	return action, nil
}

// CancelAction cancels a QUEUED action. Only QUEUED actions may be cancelled.
func (uc *ActionUsecase) CancelAction(ctx context.Context, tenantID, actionID string) (*models.Action, error) {
	if actionID == "" {
		return nil, fmt.Errorf("action ID is empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is empty")
	}

	if err := uc.store.Actions().CancelQueued(ctx, tenantID, actionID, cancelledByUserMessage); err != nil {
		if errors.Is(err, storage.ErrActionNotCancellable) {
			action, getErr := uc.store.Actions().GetByID(ctx, tenantID, actionID)
			if getErr != nil {
				return nil, fmt.Errorf("action not found: %w", getErr)
			}
			return nil, fmt.Errorf("cannot cancel action in %s status: %w", action.Status, storage.ErrActionNotCancellable)
		}
		return nil, fmt.Errorf("failed to cancel action: %w", err)
	}

	return uc.store.Actions().GetByID(ctx, tenantID, actionID)
}

// ListActions retrieves actions for a device
func (uc *ActionUsecase) ListActions(ctx context.Context, deviceID string, filters *storage.ActionListFilter) ([]*models.Action, int32, error) {
	if deviceID == "" {
		return nil, 0, fmt.Errorf("device ID is empty")
	}

	if filters == nil {
		filters = &storage.ActionListFilter{Page: 1, PageSize: 50}
	}

	// Ensure tenantID is in filter (caller should set it, but validate)
	if filters.TenantID == "" {
		return nil, 0, fmt.Errorf("tenant ID is required in filter")
	}

	return uc.store.Actions().ListForDevice(ctx, deviceID, filters)
}

// GetAction retrieves an action by ID - requires tenant context
func (uc *ActionUsecase) GetAction(ctx context.Context, tenantID, actionID string) (*models.Action, error) {
	if actionID == "" {
		return nil, fmt.Errorf("action ID is empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is empty")
	}

	return uc.store.Actions().GetByID(ctx, tenantID, actionID)
}

// MarkActionDelivered marks an action as delivered to device
func (uc *ActionUsecase) MarkActionDelivered(ctx context.Context, tenantID, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID is empty")
	}

	return uc.store.Actions().MarkDelivered(ctx, tenantID, actionID)
}

// MarkActionCompleted marks an action as successfully completed
func (uc *ActionUsecase) MarkActionCompleted(ctx context.Context, tenantID, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID is empty")
	}

	return uc.store.Actions().MarkCompleted(ctx, tenantID, actionID)
}

// MarkActionFailed marks an action as failed
func (uc *ActionUsecase) MarkActionFailed(ctx context.Context, tenantID, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID is empty")
	}

	return uc.store.Actions().MarkFailed(ctx, tenantID, actionID)
}

// RetryAction increments retry count for a failed action
func (uc *ActionUsecase) RetryAction(ctx context.Context, tenantID, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}
	if tenantID == "" {
		return fmt.Errorf("tenant ID is empty")
	}

	// Get action
	action, err := uc.store.Actions().GetByID(ctx, tenantID, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	// Check if retries available
	if action.RetryCount >= action.MaxRetries {
		return fmt.Errorf("max retries exceeded")
	}

	// Reset status to QUEUED for retry
	if err := uc.store.Actions().UpdateStatus(ctx, tenantID, actionID, models.ActionStatusQueued); err != nil {
		return fmt.Errorf("failed to reset action status: %w", err)
	}

	// Increment retry count
	return uc.store.Actions().IncrementRetry(ctx, tenantID, actionID)
}

// QueueActionRequest represents a request to queue a new action
type QueueActionRequest struct {
	DeviceID   string
	ActionType string
	Payload    *anypb.Any
	MaxRetries int32
	TTLSeconds int32
}
