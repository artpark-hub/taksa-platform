package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// ActionWorkflowStore implements storage.ActionWorkflowStore.
type ActionWorkflowStore struct {
	db *sql.DB
}

func (s *ActionWorkflowStore) Save(ctx context.Context, wf *models.ActionWorkflow) error {
	if wf == nil || wf.ID == "" || wf.TenantID == "" || wf.DeviceID == "" {
		return ErrInvalidInput
	}
	query := `
		INSERT INTO action_workflows (
			id, tenant_id, device_id, workflow_type, protocol_kind,
			converter_uuid, converter_name, status, stage, rollback_status,
			deploy_action_id, configure_action_id, rollback_action_id,
			pending_configure_json, error_message, created_at, updated_at, expires_at, completed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`
	_, err := s.db.ExecContext(ctx, query,
		wf.ID, wf.TenantID, wf.DeviceID, wf.WorkflowType, wf.ProtocolKind,
		nullString(wf.ConverterUUID), nullString(wf.ConverterName), int32(wf.Status),
		nullString(wf.Stage), nullString(wf.RollbackStatus),
		nullString(wf.DeployActionID), nullString(wf.ConfigureActionID), nullString(wf.RollbackActionID),
		nullString(wf.PendingConfigureJSON), nullString(wf.ErrorMessage), wf.CreatedAt, wf.UpdatedAt,
		optionalTimeValue(wf.ExpiresAt), optionalTimeValue(wf.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("save action workflow: %w", err)
	}
	return nil
}

func (s *ActionWorkflowStore) GetByID(ctx context.Context, tenantID, id string) (*models.ActionWorkflow, error) {
	return s.getOne(ctx, `SELECT id, tenant_id, device_id, workflow_type, protocol_kind,
		converter_uuid, converter_name, status, stage, rollback_status,
		deploy_action_id, configure_action_id, rollback_action_id,
		pending_configure_json, error_message, created_at, updated_at, expires_at, completed_at
		FROM action_workflows WHERE tenant_id = $1 AND id = $2`, tenantID, id)
}

func (s *ActionWorkflowStore) GetByDeployActionID(ctx context.Context, tenantID, deployActionID string) (*models.ActionWorkflow, error) {
	return s.getOne(ctx, `SELECT id, tenant_id, device_id, workflow_type, protocol_kind,
		converter_uuid, converter_name, status, stage, rollback_status,
		deploy_action_id, configure_action_id, rollback_action_id,
		pending_configure_json, error_message, created_at, updated_at, expires_at, completed_at
		FROM action_workflows WHERE tenant_id = $1 AND deploy_action_id = $2`, tenantID, deployActionID)
}

func (s *ActionWorkflowStore) GetByConfigureActionID(ctx context.Context, tenantID, configureActionID string) (*models.ActionWorkflow, error) {
	return s.getOne(ctx, `SELECT id, tenant_id, device_id, workflow_type, protocol_kind,
		converter_uuid, converter_name, status, stage, rollback_status,
		deploy_action_id, configure_action_id, rollback_action_id,
		pending_configure_json, error_message, created_at, updated_at, expires_at, completed_at
		FROM action_workflows WHERE tenant_id = $1 AND configure_action_id = $2`, tenantID, configureActionID)
}

func (s *ActionWorkflowStore) GetByRollbackActionID(ctx context.Context, tenantID, rollbackActionID string) (*models.ActionWorkflow, error) {
	return s.getOne(ctx, `SELECT id, tenant_id, device_id, workflow_type, protocol_kind,
		converter_uuid, converter_name, status, stage, rollback_status,
		deploy_action_id, configure_action_id, rollback_action_id,
		pending_configure_json, error_message, created_at, updated_at, expires_at, completed_at
		FROM action_workflows WHERE tenant_id = $1 AND rollback_action_id = $2`, tenantID, rollbackActionID)
}

func (s *ActionWorkflowStore) Update(ctx context.Context, tenantID string, wf *models.ActionWorkflow) error {
	if wf == nil || wf.ID == "" {
		return ErrInvalidInput
	}
	wf.UpdatedAt = time.Now()
	query := `
		UPDATE action_workflows SET
			converter_uuid = $3, converter_name = $4, status = $5, stage = $6,
			rollback_status = $7, deploy_action_id = $8, configure_action_id = $9,
			rollback_action_id = $10, pending_configure_json = $11, error_message = $12, updated_at = $13,
			expires_at = $14, completed_at = $15
		WHERE tenant_id = $1 AND id = $2
	`
	_, err := s.db.ExecContext(ctx, query,
		tenantID, wf.ID,
		nullString(wf.ConverterUUID), nullString(wf.ConverterName), int32(wf.Status),
		nullString(wf.Stage), nullString(wf.RollbackStatus),
		nullString(wf.DeployActionID), nullString(wf.ConfigureActionID), nullString(wf.RollbackActionID),
		nullString(wf.PendingConfigureJSON), nullString(wf.ErrorMessage), wf.UpdatedAt,
		optionalTimeValue(wf.ExpiresAt), optionalTimeValue(wf.CompletedAt),
	)
	if err != nil {
		return fmt.Errorf("update action workflow: %w", err)
	}
	return nil
}

func (s *ActionWorkflowStore) HasActiveForDeviceConverter(ctx context.Context, tenantID, deviceID, converterUUID string) (bool, error) {
	return s.hasActive(ctx, `tenant_id = $1 AND device_id = $2 AND converter_uuid = $3`, tenantID, deviceID, converterUUID)
}

func (s *ActionWorkflowStore) HasActiveForDeviceName(ctx context.Context, tenantID, deviceID, converterName string) (bool, error) {
	return s.hasActive(ctx, `tenant_id = $1 AND device_id = $2 AND converter_name = $3`, tenantID, deviceID, converterName)
}

func (s *ActionWorkflowStore) hasActive(ctx context.Context, where string, args ...interface{}) (bool, error) {
	query := fmt.Sprintf(`
		SELECT 1 FROM action_workflows
		WHERE %s AND status IN ($%d, $%d)
		LIMIT 1
	`, where, len(args)+1, len(args)+2)
	fullArgs := append(append([]interface{}{}, args...),
		int32(models.ActionStatusQueued), int32(models.ActionStatusProcessing))
	var n int
	err := s.db.QueryRowContext(ctx, query, fullArgs...).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *ActionWorkflowStore) getOne(ctx context.Context, query string, args ...interface{}) (*models.ActionWorkflow, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	wf, err := scanActionWorkflow(row)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	return wf, err
}

func scanActionWorkflow(row *sql.Row) (*models.ActionWorkflow, error) {
	wf := &models.ActionWorkflow{}
	var status int32
	var converterUUID, converterName, stage, rollbackStatus sql.NullString
	var deployID, configureID, rollbackID, pendingConfigure, errMsg sql.NullString
	var expiresAt, completedAt sql.NullTime
	err := row.Scan(
		&wf.ID, &wf.TenantID, &wf.DeviceID, &wf.WorkflowType, &wf.ProtocolKind,
		&converterUUID, &converterName, &status, &stage, &rollbackStatus,
		&deployID, &configureID, &rollbackID, &pendingConfigure, &errMsg,
		&wf.CreatedAt, &wf.UpdatedAt, &expiresAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	wf.Status = models.ActionStatus(status)
	wf.ConverterUUID = converterUUID.String
	wf.ConverterName = converterName.String
	wf.Stage = stage.String
	wf.RollbackStatus = rollbackStatus.String
	wf.DeployActionID = deployID.String
	wf.ConfigureActionID = configureID.String
	wf.RollbackActionID = rollbackID.String
	wf.PendingConfigureJSON = pendingConfigure.String
	wf.ErrorMessage = errMsg.String
	if expiresAt.Valid {
		wf.ExpiresAt = expiresAt.Time
	}
	if completedAt.Valid {
		wf.CompletedAt = completedAt.Time
	}
	return wf, nil
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
