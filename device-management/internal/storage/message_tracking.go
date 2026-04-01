package storage

import (
	"context"

	"taksa-platform-dm/internal/models"
)

// MessageTrackingStore handles storage of message-to-action mappings
// Used for request-response correlation: maps device message traceIds to action IDs
type MessageTrackingStore interface {
	// Save stores a message tracking record
	Save(ctx context.Context, tracking *models.MessageTracking) error

	// GetByTraceId retrieves message tracking by message trace ID
	GetByTraceId(ctx context.Context, messageTraceId string) (*models.MessageTracking, error)

	// GetByActionId retrieves all message tracking records for an action
	GetByActionId(ctx context.Context, actionId string) ([]*models.MessageTracking, error)

	// Delete removes a message tracking record
	Delete(ctx context.Context, messageTraceId string) error
}
