package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	pcconv "github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/modbus"
)

// DeployModbusProtocolConverter starts a deploy+configure workflow.
func (s *DeviceMgmtService) DeployModbusProtocolConverter(ctx context.Context, req *v1.DeployModbusProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if s.pcWorkflowUc == nil {
		return nil, status.Error(codes.Internal, "workflow usecase not configured")
	}

	wf, err := s.pcWorkflowUc.StartModbusDeploy(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &v1.ActionQueuedResponse{
		ActionId:  wf.ID,
		CreatedAt: timeToProto(wf.CreatedAt),
		ExpiresAt: timeToProto(wf.ExpiresAt),
	}, nil
}

// EditModbusProtocolConverter queues a single edit action with a built umh-core payload.
func (s *DeviceMgmtService) EditModbusProtocolConverter(ctx context.Context, req *v1.EditModbusProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id and uuid are required")
	}
	if err := modbus.ValidateEditRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := modbus.ValidateSectionModes(req.GetInput(), req.GetReadFlow()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tenantID := middleware.GetTenantID(ctx)
	if err := s.assertModbusConverterKind(ctx, tenantID, req.DeviceId, req.Uuid); err != nil {
		return nil, err
	}

	name := req.GetName()
	if name == "" {
		if row, _ := s.protocolConverterRepo.GetByUUID(ctx, tenantID, req.DeviceId, req.Uuid); row != nil {
			name = row.Name
		}
	}
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetConnection() == nil {
		return nil, status.Error(codes.InvalidArgument, "connection is required")
	}

	cfg, err := modbus.BuildConfigurePayload(req.Uuid, name, req.GetConnection(), req.GetLocation(),
		req.GetInput(), req.GetReadFlow(), req.GetTemplateVariables(), req.GetState())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	payload, err := convertRequestToAny(cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "edit-protocol-converter",
		Payload:    payload,
		MaxRetries: 3,
		TTLSeconds: 3600,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ActionQueuedResponse{
		ActionId:  action.Id,
		CreatedAt: timeToProto(action.CreatedAt),
		ExpiresAt: timeToProto(action.ExpiresAt),
	}, nil
}

// GetModbusProtocolConverter queues async get from the DCD.
func (s *DeviceMgmtService) GetModbusProtocolConverter(ctx context.Context, req *v1.GetModbusProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id and uuid are required")
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if err := s.assertModbusConverterKind(ctx, tenantID, req.DeviceId, req.Uuid); err != nil {
		return nil, err
	}

	getPayload := map[string]string{"uuid": req.Uuid}
	payloadJSON, err := json.Marshal(getPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-protocol-converter",
		Payload:    anypbFromJSON(payloadJSON),
		MaxRetries: 3,
		TTLSeconds: 3600,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &v1.ActionQueuedResponse{
		ActionId:  action.Id,
		CreatedAt: timeToProto(action.CreatedAt),
		ExpiresAt: timeToProto(action.ExpiresAt),
	}, nil
}

// GetModbusProtocolConverterActionResponse polls workflow or child action results.
func (s *DeviceMgmtService) GetModbusProtocolConverterActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.ModbusProtocolConverterActionResponse, error) {
	if req == nil || req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	if s.pcWorkflowUc != nil {
		if wf, err := s.pcWorkflowUc.GetWorkflow(ctx, tenantID, req.ActionId); err == nil && wf != nil {
			if req.DeviceId != "" && wf.DeviceID != req.DeviceId {
				return nil, status.Error(codes.PermissionDenied, "workflow does not belong to device")
			}
			if wf.WorkflowType != models.WorkflowTypeDeployModbus {
				return nil, status.Error(codes.NotFound, "action is not a Modbus deploy workflow")
			}
			return s.buildModbusWorkflowPollResponse(ctx, tenantID, wf)
		}
	}

	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}
	if req.DeviceId != "" && action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	resp := &v1.ModbusProtocolConverterActionResponse{
		ActionId:     action.Id,
		Status:       actionStatusToProto(action.Status),
		CompletedAt:  timeToProto(action.CompletedAt),
		ErrorMessage: action.ErrorMessage,
	}

	if action.Status != models.ActionStatusCompleted {
		return resp, nil
	}

	resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
	if err != nil || len(resultPayload) == 0 {
		return resp, nil
	}
	decompressed, err := decompressPayload(resultPayload)
	if err != nil {
		return resp, nil
	}
	pc, err := unmarshalProtocolConverterResult(decompressed)
	if err != nil || pc == nil {
		return resp, nil
	}
	if !modbus.IsModbusProtocolConverter(pc) {
		if action.Type == "edit-protocol-converter" && modbus.IsMinimalProtocolConverterReply(pc) {
			return resp, nil
		}
		return nil, status.Error(codes.FailedPrecondition, "protocol converter is not Modbus kind")
	}
	facade := modbus.ToFacade(pc)
	s.enrichModbusFacadeFromCatalog(ctx, tenantID, action.DeviceId, facade)
	resp.Result = facade
	return resp, nil
}

func (s *DeviceMgmtService) buildModbusWorkflowPollResponse(ctx context.Context, tenantID string, wf *models.ActionWorkflow) (*v1.ModbusProtocolConverterActionResponse, error) {
	resp := &v1.ModbusProtocolConverterActionResponse{
		ActionId:       wf.ID,
		Status:         actionStatusToProto(wf.Status),
		CompletedAt:    timeToProto(wf.CompletedAt),
		ErrorMessage:   wf.ErrorMessage,
		Stage:          workflowStageToProto(wf.Stage),
		RollbackStatus: rollbackStatusToProto(wf.RollbackStatus),
		Steps:          &v1.ProtocolConverterWorkflowSteps{},
	}

	if wf.DeployActionID != "" {
		if a, err := s.actionUc.GetAction(ctx, tenantID, wf.DeployActionID); err == nil {
			resp.Steps.Deploy = &v1.ProtocolConverterWorkflowStep{ActionId: a.Id, Status: actionStatusToProto(a.Status)}
		}
	}
	if wf.ConfigureActionID != "" {
		if a, err := s.actionUc.GetAction(ctx, tenantID, wf.ConfigureActionID); err == nil {
			resp.Steps.Configure = &v1.ProtocolConverterWorkflowStep{ActionId: a.Id, Status: actionStatusToProto(a.Status)}
		}
	}
	if wf.RollbackActionID != "" {
		if a, err := s.actionUc.GetAction(ctx, tenantID, wf.RollbackActionID); err == nil {
			resp.Steps.Rollback = &v1.ProtocolConverterWorkflowStep{ActionId: a.Id, Status: actionStatusToProto(a.Status)}
		}
	}

	if wf.Status == models.ActionStatusCompleted {
		resp.Result = s.resolveModbusWorkflowFacade(ctx, tenantID, wf)
	}

	return resp, nil
}

func (s *DeviceMgmtService) resolveModbusWorkflowFacade(ctx context.Context, tenantID string, wf *models.ActionWorkflow) *v1.ModbusProtocolConverter {
	if wf.ConfigureActionID != "" {
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, wf.ConfigureActionID)
		if err == nil && len(resultPayload) > 0 {
			if decompressed, err := decompressPayload(resultPayload); err == nil {
				if pc, err := unmarshalProtocolConverterResult(decompressed); err == nil && pc != nil {
					if !modbus.IsMinimalProtocolConverterReply(pc) && modbus.IsModbusProtocolConverter(pc) {
						facade := modbus.ToFacade(pc)
						s.enrichModbusFacadeFromCatalog(ctx, tenantID, wf.DeviceID, facade)
						return facade
					}
				}
			}
		}
	}
	if wf.PendingConfigureJSON == "" {
		return nil
	}
	var pc v2.ProtocolConverter
	if err := protojson.Unmarshal([]byte(wf.PendingConfigureJSON), &pc); err != nil {
		return nil
	}
	if pc.GetUuid() == "" {
		pc.Uuid = wf.ConverterUUID
	}
	facade := modbus.ToFacade(&pc)
	s.enrichModbusFacadeFromCatalog(ctx, tenantID, wf.DeviceID, facade)
	return facade
}

func (s *DeviceMgmtService) enrichModbusFacadeFromCatalog(ctx context.Context, tenantID, deviceID string, facade *v1.ModbusProtocolConverter) {
	if s.protocolConverterRepo == nil || facade == nil || facade.GetUuid() == "" {
		return
	}
	row, err := s.protocolConverterRepo.GetByUUID(ctx, tenantID, deviceID, facade.GetUuid())
	if err != nil || row == nil {
		return
	}
	facade.DeploymentStatus = row.DeploymentStatus
	facade.HealthStatus = row.HealthStatus
	if row.ErrorMessage.Valid {
		facade.ErrorMessage = row.ErrorMessage.String
	}
}

func (s *DeviceMgmtService) assertModbusConverterKind(ctx context.Context, tenantID, deviceID, uuid string) error {
	if s.protocolConverterRepo == nil {
		return nil
	}
	row, err := s.protocolConverterRepo.GetByUUID(ctx, tenantID, deviceID, uuid)
	if err != nil || row == nil {
		return nil
	}
	if pcconv.IsKnownNonModbusCatalogType(row.Type) {
		return status.Error(codes.FailedPrecondition,
			fmt.Sprintf("protocol converter %s is kind %q, not modbus", uuid, row.Type))
	}
	return nil
}
