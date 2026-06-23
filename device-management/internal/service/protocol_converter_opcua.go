package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	pcconv "github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/opcua"
)

// DeployOpcUaProtocolConverter starts a deploy+configure workflow.
func (s *DeviceMgmtService) DeployOpcUaProtocolConverter(ctx context.Context, req *v1.DeployOpcUaProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if s.pcWorkflowUc == nil {
		return nil, status.Error(codes.Internal, "workflow usecase not configured")
	}

	wf, err := s.pcWorkflowUc.StartOpcUaDeploy(ctx, req)
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

// EditOpcUaProtocolConverter queues a single edit action with a built umh-core payload.
func (s *DeviceMgmtService) EditOpcUaProtocolConverter(ctx context.Context, req *v1.EditOpcUaProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id and uuid are required")
	}
	if err := opcua.ValidateEditRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := opcua.ValidateSectionModes(req.GetInput(), req.GetReadFlow()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tenantID := middleware.GetTenantID(ctx)
	if err := s.assertOpcUaConverterKind(ctx, tenantID, req.DeviceId, req.Uuid); err != nil {
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

	cfg, err := opcua.BuildConfigurePayload(req.Uuid, name, req.GetConnection(), req.GetLocation(),
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

// GetOpcUaProtocolConverter queues async get from the DCD.
func (s *DeviceMgmtService) GetOpcUaProtocolConverter(ctx context.Context, req *v1.GetOpcUaProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id and uuid are required")
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if err := s.assertOpcUaConverterKind(ctx, tenantID, req.DeviceId, req.Uuid); err != nil {
		return nil, err
	}

	getPayload := map[string]string{"uuid": req.Uuid}
	payloadJSON, err := json.Marshal(getPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	getAny := anypbFromJSON(payloadJSON)
	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-protocol-converter",
		Payload:    getAny,
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

// GetOpcUaProtocolConverterActionResponse polls workflow or child action results.
func (s *DeviceMgmtService) GetOpcUaProtocolConverterActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.OpcUaProtocolConverterActionResponse, error) {
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
			if wf.WorkflowType != models.WorkflowTypeDeployOpcUa {
				return nil, status.Error(codes.NotFound, "action is not an OPC-UA deploy workflow")
			}
			return s.buildOpcUaWorkflowPollResponse(ctx, tenantID, wf)
		}
	}

	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}
	if req.DeviceId != "" && action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	resp := &v1.OpcUaProtocolConverterActionResponse{
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
	if !opcua.IsOpcUaProtocolConverter(pc) {
		// umh-core edit-protocol-converter replies with {uuid} only; run GET for full facade.
		if action.Type == "edit-protocol-converter" && opcua.IsMinimalProtocolConverterReply(pc) {
			return resp, nil
		}
		return nil, status.Error(codes.FailedPrecondition, "protocol converter is not OPC-UA kind")
	}
	facade := opcua.ToFacade(pc)
	s.enrichOpcUaFacadeFromCatalog(ctx, tenantID, action.DeviceId, facade)
	resp.Result = facade
	return resp, nil
}

func (s *DeviceMgmtService) buildOpcUaWorkflowPollResponse(ctx context.Context, tenantID string, wf *models.ActionWorkflow) (*v1.OpcUaProtocolConverterActionResponse, error) {
	resp := &v1.OpcUaProtocolConverterActionResponse{
		ActionId:     wf.ID,
		Status:       actionStatusToProto(wf.Status),
		CompletedAt:  timeToProto(wf.CompletedAt),
		ErrorMessage: wf.ErrorMessage,
		Stage:        workflowStageToProto(wf.Stage),
		RollbackStatus: rollbackStatusToProto(wf.RollbackStatus),
		Steps:        &v1.ProtocolConverterWorkflowSteps{},
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
		resp.Result = s.resolveOpcUaWorkflowFacade(ctx, tenantID, wf)
	}

	return resp, nil
}

func (s *DeviceMgmtService) resolveOpcUaWorkflowFacade(ctx context.Context, tenantID string, wf *models.ActionWorkflow) *v1.OpcUaProtocolConverter {
	if wf.ConfigureActionID != "" {
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, wf.ConfigureActionID)
		if err == nil && len(resultPayload) > 0 {
			if decompressed, err := decompressPayload(resultPayload); err == nil {
				if pc, err := unmarshalProtocolConverterResult(decompressed); err == nil && pc != nil {
					if !opcua.IsMinimalProtocolConverterReply(pc) && opcua.IsOpcUaProtocolConverter(pc) {
						facade := opcua.ToFacade(pc)
						s.enrichOpcUaFacadeFromCatalog(ctx, tenantID, wf.DeviceID, facade)
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
	facade := opcua.ToFacade(&pc)
	s.enrichOpcUaFacadeFromCatalog(ctx, tenantID, wf.DeviceID, facade)
	return facade
}

func (s *DeviceMgmtService) enrichOpcUaFacadeFromCatalog(ctx context.Context, tenantID, deviceID string, facade *v1.OpcUaProtocolConverter) {
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

func (s *DeviceMgmtService) assertOpcUaConverterKind(ctx context.Context, tenantID, deviceID, uuid string) error {
	if s.protocolConverterRepo == nil {
		return nil
	}
	row, err := s.protocolConverterRepo.GetByUUID(ctx, tenantID, deviceID, uuid)
	if err != nil || row == nil {
		return nil
	}
	// Catalog type is often the DFC kind ("protocol-converter") from heartbeat sync, not the wire
	// protocol (opcua/modbus). Only reject when the catalog already records a different protocol.
	if pcconv.IsKnownNonOpcUaCatalogType(row.Type) {
		return status.Error(codes.FailedPrecondition,
			fmt.Sprintf("protocol converter %s is kind %q, not opc-ua", uuid, row.Type))
	}
	return nil
}

func workflowStageToProto(stage string) v1.ProtocolConverterWorkflowStage {
	switch stage {
	case models.WorkflowStageDeploying:
		return v1.ProtocolConverterWorkflowStage_DEPLOYING
	case models.WorkflowStageConfiguring:
		return v1.ProtocolConverterWorkflowStage_CONFIGURING
	case models.WorkflowStageRollingBack:
		return v1.ProtocolConverterWorkflowStage_ROLLING_BACK
	default:
		return v1.ProtocolConverterWorkflowStage_PROTOCOL_CONVERTER_WORKFLOW_STAGE_UNSPECIFIED
	}
}

func rollbackStatusToProto(rs string) v1.ProtocolConverterRollbackStatus {
	switch rs {
	case models.RollbackClean:
		return v1.ProtocolConverterRollbackStatus_ROLLBACK_CLEAN
	case models.RollbackOrphanShell:
		return v1.ProtocolConverterRollbackStatus_ROLLBACK_ORPHAN_SHELL
	default:
		return v1.ProtocolConverterRollbackStatus_PROTOCOL_CONVERTER_ROLLBACK_STATUS_UNSPECIFIED
	}
}

func unmarshalProtocolConverterResult(data []byte) (*v2.ProtocolConverter, error) {
	var pc v2.ProtocolConverter
	if err := protojson.Unmarshal(data, &pc); err == nil && (pc.GetUuid() != "" || pc.GetName() != "") {
		return &pc, nil
	}
	var wrap map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, err
	}
	if raw, ok := wrap["Payload"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(raw, &inner); err == nil {
			if reply, ok := inner["actionReplyPayload"]; ok {
				if err := protojson.Unmarshal(reply, &pc); err == nil {
					return &pc, nil
				}
			}
		}
	}
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, err
	}
	return &pc, nil
}

func anypbFromJSON(b []byte) *anypb.Any {
	return &anypb.Any{
		TypeUrl: "type.googleapis.com/api.umh_core.v2.GetProtocolConverterPayload",
		Value:   b,
	}
}
