package storage

import (
	"context"

	"taksa-platform-dm/internal/models"
)

// ActionResponseStore handles storage of device responses to actions
// Used for request-response correlation: stores responses correlated to actions via traceId
type ActionResponseStore interface {
	// Save stores an action response
	Save(ctx context.Context, response *models.ActionResponse) error

	// GetByActionId retrieves all responses for an action
	GetByActionId(ctx context.Context, actionId string) ([]*models.ActionResponse, error)

	// GetByTraceId retrieves response by message trace ID
	GetByTraceId(ctx context.Context, messageTraceId string) (*models.ActionResponse, error)

	// GetByDeviceId retrieves all responses from a device
	GetByDeviceId(ctx context.Context, deviceId string) ([]*models.ActionResponse, error)

	// Delete removes an action response
	Delete(ctx context.Context, responseId string) error
}
