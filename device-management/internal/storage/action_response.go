package storage

import (
	"context"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// ActionResponseStore handles storage of device responses to actions
// Used for request-response correlation: stores responses correlated to actions via traceId
type ActionResponseStore interface {
	// Save stores an action response
	Save(ctx context.Context, response *models.ActionResponse) error

	// GetByActionID retrieves all responses for an action
	GetByActionID(ctx context.Context, actionID string) ([]*models.ActionResponse, error)

	// GetByTraceID retrieves response by message trace ID
	GetByTraceID(ctx context.Context, messageTraceID string) (*models.ActionResponse, error)

	// GetByDeviceID retrieves all responses from a device
	GetByDeviceID(ctx context.Context, deviceID string) ([]*models.ActionResponse, error)

	// Delete removes an action response
	Delete(ctx context.Context, responseID string) error
}
