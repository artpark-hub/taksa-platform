package biz

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"taksa-platform-dm/internal/models"
	"taksa-platform-dm/internal/storage"
)

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

	// Save
	if err := uc.store.Actions().Save(ctx, action); err != nil {
		return nil, fmt.Errorf("failed to save action: %w", err)
	}

	return action, nil
}

// CancelAction cancels a pending action
func (uc *ActionUsecase) CancelAction(ctx context.Context, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}

	// Get action
	action, err := uc.store.Actions().GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	// Can only cancel queued/delivered actions
	if action.Status != models.ActionStatusQueued && action.Status != models.ActionStatusDelivered {
		return fmt.Errorf("cannot cancel action in %s status", action.Status)
	}

	// Mark as cancelled
	return uc.store.Actions().UpdateStatus(ctx, actionID, models.ActionStatusCancelled)
}

// ListActions retrieves actions for a device
func (uc *ActionUsecase) ListActions(ctx context.Context, deviceID string, filters *storage.ActionListFilter) ([]*models.Action, int32, error) {
	if deviceID == "" {
		return nil, 0, fmt.Errorf("device ID is empty")
	}

	if filters == nil {
		filters = &storage.ActionListFilter{Page: 1, PageSize: 50}
	}

	return uc.store.Actions().ListForDevice(ctx, deviceID, filters)
}

// GetAction retrieves an action by ID
func (uc *ActionUsecase) GetAction(ctx context.Context, actionID string) (*models.Action, error) {
	if actionID == "" {
		return nil, fmt.Errorf("action ID is empty")
	}

	return uc.store.Actions().GetByID(ctx, actionID)
}

// MarkActionDelivered marks an action as delivered to device
func (uc *ActionUsecase) MarkActionDelivered(ctx context.Context, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}

	return uc.store.Actions().MarkDelivered(ctx, actionID)
}

// MarkActionCompleted marks an action as successfully completed
func (uc *ActionUsecase) MarkActionCompleted(ctx context.Context, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}

	return uc.store.Actions().MarkCompleted(ctx, actionID)
}

// MarkActionFailed marks an action as failed
func (uc *ActionUsecase) MarkActionFailed(ctx context.Context, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}

	return uc.store.Actions().MarkFailed(ctx, actionID)
}

// RetryAction increments retry count for a failed action
func (uc *ActionUsecase) RetryAction(ctx context.Context, actionID string) error {
	if actionID == "" {
		return fmt.Errorf("action ID is empty")
	}

	// Get action
	action, err := uc.store.Actions().GetByID(ctx, actionID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}

	// Check if retries available
	if action.RetryCount >= action.MaxRetries {
		return fmt.Errorf("max retries exceeded")
	}

	// Reset status to QUEUED for retry
	if err := uc.store.Actions().UpdateStatus(ctx, actionID, models.ActionStatusQueued); err != nil {
		return fmt.Errorf("failed to reset action status: %w", err)
	}

	// Increment retry count
	return uc.store.Actions().IncrementRetry(ctx, actionID)
}

// QueueActionRequest represents a request to queue a new action
type QueueActionRequest struct {
	DeviceID   string
	ActionType string
	Payload    *anypb.Any
	MaxRetries int32
	TTLSeconds int32
}
