package storage

import (
	"context"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

// ActionWorkflowStore persists composite action workflows.
type ActionWorkflowStore interface {
	Save(ctx context.Context, wf *models.ActionWorkflow) error
	GetByID(ctx context.Context, tenantID, id string) (*models.ActionWorkflow, error)
	GetByDeployActionID(ctx context.Context, tenantID, deployActionID string) (*models.ActionWorkflow, error)
	GetByConfigureActionID(ctx context.Context, tenantID, configureActionID string) (*models.ActionWorkflow, error)
	GetByRollbackActionID(ctx context.Context, tenantID, rollbackActionID string) (*models.ActionWorkflow, error)
	Update(ctx context.Context, tenantID string, wf *models.ActionWorkflow) error
	HasActiveForDeviceConverter(ctx context.Context, tenantID, deviceID, converterUUID string) (bool, error)
	GetActiveForDeviceConverter(ctx context.Context, tenantID, deviceID, converterUUID string) (*models.ActionWorkflow, error)
	HasActiveForDeviceName(ctx context.Context, tenantID, deviceID, converterName string) (bool, error)
}
