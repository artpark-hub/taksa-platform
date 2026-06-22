package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/data"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/modbus"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/opcua"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

const workflowTTLSeconds = 7200

// ProtocolConverterWorkflowUsecase orchestrates facade deploy + configure workflows.
type ProtocolConverterWorkflowUsecase struct {
	store              storage.Store
	actionUc           *ActionUsecase
	protocolConverterRepo *data.ProtocolConverterRepo
}

func NewProtocolConverterWorkflowUsecase(store storage.Store, actionUc *ActionUsecase, pcRepo *data.ProtocolConverterRepo) *ProtocolConverterWorkflowUsecase {
	return &ProtocolConverterWorkflowUsecase{
		store:                 store,
		actionUc:              actionUc,
		protocolConverterRepo: pcRepo,
	}
}

// StartOpcUaDeploy creates a workflow and queues the deploy child action.
func (uc *ProtocolConverterWorkflowUsecase) StartOpcUaDeploy(ctx context.Context, req *v1.DeployOpcUaProtocolConverterRequest) (*models.ActionWorkflow, error) {
	if err := opcua.ValidateDeployRequest(req); err != nil {
		return nil, err
	}
	if err := opcua.ValidateSectionModes(req.GetInput(), req.GetReadFlow()); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id not found in context")
	}

	name := req.GetName()
	converterUUID := GenerateUUIDFromName(name)

	if uc.protocolConverterRepo != nil {
		if existing, _ := uc.protocolConverterRepo.GetByUUID(ctx, tenantID, req.GetDeviceId(), converterUUID); existing != nil {
			return nil, fmt.Errorf("protocol converter already exists for name %q", name)
		}
	}
	if active, _ := uc.store.ActionWorkflows().HasActiveForDeviceName(ctx, tenantID, req.GetDeviceId(), name); active {
		return nil, fmt.Errorf("deploy workflow already in progress for name %q", name)
	}

	shell := opcua.BuildDeployShell(name, req.GetConnection(), req.GetLocation(), req.GetState())
	var configureJSON string
	if req.GetApplyReadConfig() {
		cfg, err := opcua.BuildConfigurePayload(converterUUID, name, req.GetConnection(), req.GetLocation(),
			req.GetInput(), req.GetReadFlow(), req.GetTemplateVariables(), req.GetState())
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("marshal configure payload: %w", err)
		}
		configureJSON = string(b)
	}

	now := time.Now()
	wf := &models.ActionWorkflow{
		ID:                   generateUUID(),
		TenantID:             tenantID,
		DeviceID:             req.GetDeviceId(),
		WorkflowType:         models.WorkflowTypeDeployOpcUa,
		ProtocolKind:         models.ProtocolKindOpcUa,
		ConverterUUID:        converterUUID,
		ConverterName:        name,
		Status:               models.ActionStatusProcessing,
		Stage:                models.WorkflowStageDeploying,
		PendingConfigureJSON: configureJSON,
		CreatedAt:            now,
		UpdatedAt:            now,
		ExpiresAt:            now.Add(workflowTTLSeconds * time.Second),
	}

	deployAction, err := uc.queueProtocolConverterAction(ctx, req.GetDeviceId(), "deploy-protocol-converter", shell)
	if err != nil {
		return nil, err
	}
	wf.DeployActionID = deployAction.Id

	if err := uc.store.ActionWorkflows().Save(ctx, wf); err != nil {
		return nil, fmt.Errorf("save workflow: %w", err)
	}
	uc.seedPendingCatalog(ctx, tenantID, req.GetDeviceId(), converterUUID, name)
	return wf, nil
}

// StartModbusDeploy creates a workflow and queues the deploy child action for Modbus bridges.
func (uc *ProtocolConverterWorkflowUsecase) StartModbusDeploy(ctx context.Context, req *v1.DeployModbusProtocolConverterRequest) (*models.ActionWorkflow, error) {
	if err := modbus.ValidateDeployRequest(req); err != nil {
		return nil, err
	}
	if err := modbus.ValidateSectionModes(req.GetInput(), req.GetReadFlow()); err != nil {
		return nil, err
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id not found in context")
	}

	name := req.GetName()
	converterUUID := GenerateUUIDFromName(name)

	if uc.protocolConverterRepo != nil {
		if existing, _ := uc.protocolConverterRepo.GetByUUID(ctx, tenantID, req.GetDeviceId(), converterUUID); existing != nil {
			return nil, fmt.Errorf("protocol converter already exists for name %q", name)
		}
	}
	if active, _ := uc.store.ActionWorkflows().HasActiveForDeviceName(ctx, tenantID, req.GetDeviceId(), name); active {
		return nil, fmt.Errorf("deploy workflow already in progress for name %q", name)
	}

	shell := modbus.BuildDeployShell(name, req.GetConnection(), req.GetLocation(), req.GetState())
	var configureJSON string
	if req.GetApplyReadConfig() {
		cfg, err := modbus.BuildConfigurePayload(converterUUID, name, req.GetConnection(), req.GetLocation(),
			req.GetInput(), req.GetReadFlow(), req.GetTemplateVariables(), req.GetState())
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("marshal configure payload: %w", err)
		}
		configureJSON = string(b)
	}

	now := time.Now()
	wf := &models.ActionWorkflow{
		ID:                   generateUUID(),
		TenantID:             tenantID,
		DeviceID:             req.GetDeviceId(),
		WorkflowType:         models.WorkflowTypeDeployModbus,
		ProtocolKind:         models.ProtocolKindModbusTCP,
		ConverterUUID:        converterUUID,
		ConverterName:        name,
		Status:               models.ActionStatusProcessing,
		Stage:                models.WorkflowStageDeploying,
		PendingConfigureJSON: configureJSON,
		CreatedAt:            now,
		UpdatedAt:            now,
		ExpiresAt:            now.Add(workflowTTLSeconds * time.Second),
	}

	deployAction, err := uc.queueProtocolConverterAction(ctx, req.GetDeviceId(), "deploy-protocol-converter", shell)
	if err != nil {
		return nil, err
	}
	wf.DeployActionID = deployAction.Id

	if err := uc.store.ActionWorkflows().Save(ctx, wf); err != nil {
		return nil, fmt.Errorf("save workflow: %w", err)
	}
	uc.seedPendingCatalog(ctx, tenantID, req.GetDeviceId(), converterUUID, name)
	return wf, nil
}

// HasActiveWorkflowForConverter reports whether a facade deploy workflow is still in flight.
func (uc *ProtocolConverterWorkflowUsecase) HasActiveWorkflowForConverter(ctx context.Context, tenantID, deviceID, converterUUID string) bool {
	if uc == nil || uc.store == nil || converterUUID == "" {
		return false
	}
	active, err := uc.store.ActionWorkflows().HasActiveForDeviceConverter(ctx, tenantID, deviceID, converterUUID)
	return err == nil && active
}

// ShouldDeferCatalogPromotion keeps catalog entries PENDING until the workflow finishes.
func (uc *ProtocolConverterWorkflowUsecase) ShouldDeferCatalogPromotion(ctx context.Context, tenantID, deviceID, actionID, converterUUID string) bool {
	if uc == nil || uc.store == nil {
		return false
	}
	if wf, err := uc.findWorkflowForAction(ctx, tenantID, actionID); err == nil && wf != nil && isWorkflowInFlight(wf.Status) {
		return true
	}
	return uc.HasActiveWorkflowForConverter(ctx, tenantID, deviceID, converterUUID)
}

// DeleteBlockedMessage returns a user-facing reason when delete must wait for an in-flight deploy workflow.
func (uc *ProtocolConverterWorkflowUsecase) DeleteBlockedMessage(ctx context.Context, tenantID, deviceID, converterUUID string) string {
	if uc == nil || uc.store == nil || !uc.HasActiveWorkflowForConverter(ctx, tenantID, deviceID, converterUUID) {
		return ""
	}
	wf, err := uc.store.ActionWorkflows().GetActiveForDeviceConverter(ctx, tenantID, deviceID, converterUUID)
	if err != nil || wf == nil {
		return "protocol converter deploy is in progress; cancel the deploy workflow or wait for it to complete before deleting"
	}
	stage := wf.Stage
	if stage == "" {
		stage = "IN_PROGRESS"
	}
	return fmt.Sprintf(
		"protocol converter deploy is in progress (stage: %s). Cancel workflow action %s or wait for it to complete before deleting",
		stage, wf.ID,
	)
}

func isWorkflowInFlight(status models.ActionStatus) bool {
	return status == models.ActionStatusQueued || status == models.ActionStatusProcessing
}

func (uc *ProtocolConverterWorkflowUsecase) seedPendingCatalog(ctx context.Context, tenantID, deviceID, converterUUID, name string) {
	if uc.protocolConverterRepo == nil || converterUUID == "" {
		return
	}
	_ = uc.protocolConverterRepo.UpsertPending(ctx, tenantID, deviceID, converterUUID, name, "protocol-converter", "")
}

// HandleActionTerminal advances workflow state when a child action reaches a terminal status.
func (uc *ProtocolConverterWorkflowUsecase) HandleActionTerminal(ctx context.Context, action *models.Action, finalStatus models.ActionStatus, errorMessage string, resultPayload []byte) {
	if action == nil || uc == nil || uc.store == nil {
		return
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return
	}

	wf, err := uc.findWorkflowForAction(ctx, tenantID, action.Id)
	if err != nil || wf == nil {
		return
	}
	if isActionTerminalForPushIgnore(wf.Status) {
		return
	}

	switch action.Id {
	case wf.DeployActionID:
		uc.onDeployTerminal(ctx, tenantID, wf, finalStatus, errorMessage, resultPayload)
	case wf.ConfigureActionID:
		uc.onConfigureTerminal(ctx, tenantID, wf, finalStatus, errorMessage)
	case wf.RollbackActionID:
		uc.onRollbackTerminal(ctx, tenantID, wf, finalStatus, errorMessage)
	}
}

func (uc *ProtocolConverterWorkflowUsecase) findWorkflowForAction(ctx context.Context, tenantID, actionID string) (*models.ActionWorkflow, error) {
	if wf, err := uc.store.ActionWorkflows().GetByDeployActionID(ctx, tenantID, actionID); err == nil && wf != nil {
		return wf, nil
	}
	if wf, err := uc.store.ActionWorkflows().GetByConfigureActionID(ctx, tenantID, actionID); err == nil && wf != nil {
		return wf, nil
	}
	return uc.store.ActionWorkflows().GetByRollbackActionID(ctx, tenantID, actionID)
}

func (uc *ProtocolConverterWorkflowUsecase) onDeployTerminal(ctx context.Context, tenantID string, wf *models.ActionWorkflow,
	finalStatus models.ActionStatus, errorMessage string, resultPayload []byte) {

	if finalStatus != models.ActionStatusCompleted {
		uc.failWorkflow(ctx, tenantID, wf, models.WorkflowStageDeploying, errorMessage)
		return
	}

	uuid := wf.ConverterUUID
	if parsed := extractProtocolConverterUUID(resultPayload); parsed != "" {
		uuid = parsed
		wf.ConverterUUID = uuid
	}

	if wf.PendingConfigureJSON == "" {
		uc.completeWorkflow(ctx, tenantID, wf)
		return
	}

	cfg := &v2.ProtocolConverter{}
	if err := protojson.Unmarshal([]byte(wf.PendingConfigureJSON), cfg); err != nil {
		uc.failWorkflow(ctx, tenantID, wf, models.WorkflowStageDeploying, fmt.Sprintf("configure payload corrupt: %v", err))
		return
	}
	cfg.Uuid = uuid

	editAction, err := uc.queueProtocolConverterAction(ctx, wf.DeviceID, "edit-protocol-converter", cfg)
	if err != nil {
		uc.failWorkflow(ctx, tenantID, wf, models.WorkflowStageConfiguring, err.Error())
		return
	}

	wf.Stage = models.WorkflowStageConfiguring
	wf.ConfigureActionID = editAction.Id
	wf.ExpiresAt = time.Now().Add(workflowTTLSeconds * time.Second)
	_ = uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
}

func (uc *ProtocolConverterWorkflowUsecase) onConfigureTerminal(ctx context.Context, tenantID string, wf *models.ActionWorkflow,
	finalStatus models.ActionStatus, errorMessage string) {

	if finalStatus == models.ActionStatusCompleted {
		uc.completeWorkflow(ctx, tenantID, wf)
		return
	}

	wf.Stage = models.WorkflowStageRollingBack
	wf.ErrorMessage = errorMessage
	wf.RollbackStatus = ""
	deleteAction, err := uc.queueDeleteProtocolConverter(ctx, wf.DeviceID, wf.ConverterUUID)
	if err != nil {
		wf.Status = models.ActionStatusFailed
		wf.RollbackStatus = models.RollbackOrphanShell
		wf.CompletedAt = time.Now()
		_ = uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
		return
	}
	wf.RollbackActionID = deleteAction.Id
	_ = uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
}

func (uc *ProtocolConverterWorkflowUsecase) onRollbackTerminal(ctx context.Context, tenantID string, wf *models.ActionWorkflow,
	finalStatus models.ActionStatus, errorMessage string) {

	wf.Status = models.ActionStatusFailed
	wf.CompletedAt = time.Now()
	wf.Stage = ""
	if finalStatus == models.ActionStatusCompleted || protocolconverter.IsNotFoundError(errorMessage) {
		wf.RollbackStatus = models.RollbackClean
		if uc.protocolConverterRepo != nil && wf.ConverterUUID != "" {
			_ = uc.protocolConverterRepo.Delete(ctx, tenantID, wf.DeviceID, wf.ConverterUUID)
		}
	} else {
		wf.RollbackStatus = models.RollbackOrphanShell
		if errorMessage != "" {
			wf.ErrorMessage = errorMessage
		}
	}
	_ = uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
}

func (uc *ProtocolConverterWorkflowUsecase) failWorkflow(ctx context.Context, tenantID string, wf *models.ActionWorkflow, stage, msg string) {
	wf.Status = models.ActionStatusFailed
	wf.Stage = stage
	wf.ErrorMessage = msg
	wf.CompletedAt = time.Now()
	_ = uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
	if uc.protocolConverterRepo != nil && wf.ConverterUUID != "" {
		_ = uc.protocolConverterRepo.Delete(ctx, tenantID, wf.DeviceID, wf.ConverterUUID)
	}
}

func (uc *ProtocolConverterWorkflowUsecase) completeWorkflow(ctx context.Context, tenantID string, wf *models.ActionWorkflow) {
	wf.Status = models.ActionStatusCompleted
	wf.Stage = ""
	wf.CompletedAt = time.Now()
	_ = uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
	if uc.protocolConverterRepo != nil && wf.ConverterUUID != "" {
		_ = uc.protocolConverterRepo.PromoteDeployed(ctx, tenantID, wf.DeviceID, wf.ConverterUUID)
	}
}

func (uc *ProtocolConverterWorkflowUsecase) queueProtocolConverterAction(ctx context.Context, deviceID, actionType string, payload *v2.ProtocolConverter) (*models.Action, error) {
	b, err := protojson.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return uc.actionUc.QueueAction(ctx, &QueueActionRequest{
		DeviceID:   deviceID,
		ActionType: actionType,
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/api.umh_core.v2.ProtocolConverter",
			Value:   b,
		},
		MaxRetries: 3,
		TTLSeconds: 3600,
	})
}

func (uc *ProtocolConverterWorkflowUsecase) queueDeleteProtocolConverter(ctx context.Context, deviceID, uuid string) (*models.Action, error) {
	deletePayload := map[string]string{"uuid": uuid}
	b, _ := json.Marshal(deletePayload)
	return uc.actionUc.QueueAction(ctx, &QueueActionRequest{
		DeviceID:   deviceID,
		ActionType: "delete-protocol-converter",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/api.umh_core.v2.DeleteProtocolConverterPayload",
			Value:   b,
		},
		MaxRetries: 3,
		TTLSeconds: 3600,
	})
}

// GetWorkflow returns a workflow by id for polling.
func (uc *ProtocolConverterWorkflowUsecase) GetWorkflow(ctx context.Context, tenantID, workflowID string) (*models.ActionWorkflow, error) {
	wf, err := uc.store.ActionWorkflows().GetByID(ctx, tenantID, workflowID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, err
	}
	return wf, nil
}

// CancelWorkflow cancels QUEUED child actions and marks workflow cancelled.
func (uc *ProtocolConverterWorkflowUsecase) CancelWorkflow(ctx context.Context, tenantID, deviceID, workflowID string) error {
	wf, err := uc.GetWorkflow(ctx, tenantID, workflowID)
	if err != nil {
		return err
	}
	if wf.DeviceID != deviceID {
		return fmt.Errorf("workflow does not belong to device")
	}
	for _, childID := range []string{wf.DeployActionID, wf.ConfigureActionID, wf.RollbackActionID} {
		if childID == "" {
			continue
		}
		_, _ = uc.actionUc.CancelAction(ctx, tenantID, deviceID, childID)
	}
	wf.Status = models.ActionStatusCancelled
	wf.Stage = ""
	wf.CompletedAt = time.Now()
	return uc.store.ActionWorkflows().Update(ctx, tenantID, wf)
}

func extractProtocolConverterUUID(resultPayload []byte) string {
	if len(resultPayload) == 0 {
		return ""
	}
	var pc v2.ProtocolConverter
	if err := protojson.Unmarshal(resultPayload, &pc); err != nil {
		var wrap map[string]json.RawMessage
		if err2 := json.Unmarshal(resultPayload, &wrap); err2 != nil {
			return ""
		}
		if raw, ok := wrap["uuid"]; ok {
			_ = json.Unmarshal(raw, &pc.Uuid)
			return pc.Uuid
		}
		return ""
	}
	return pc.Uuid
}
