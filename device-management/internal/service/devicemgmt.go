package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/data"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// Validation maps for Log and Metric types
var (
	// ValidLogTypes defines all allowed log sources
	// See LogType enum in api/umh-core/v2/models.proto
	ValidLogTypes = map[string]struct{}{
		"agent":                    {}, // General agent logs
		"dfc":                      {}, // Data Flow Component (requires uuid)
		"protocol-converter-read":  {}, // Protocol Converter read logs (requires uuid)
		"protocol-converter-write": {}, // Protocol Converter write logs (requires uuid)
		"redpanda":                 {}, // Redpanda broker logs
		"topic-browser":            {}, // Topic browser logs
		"stream-processor":         {}, // Stream processor logs (requires uuid)
	}

	// LogTypesRequiringUUID specifies which log types need a uuid
	LogTypesRequiringUUID = map[string]struct{}{
		"dfc":                      {},
		"protocol-converter-read":  {},
		"protocol-converter-write": {},
		"stream-processor":         {},
	}

	// ValidMetricTypes defines all allowed metric sources
	// See MetricResourceType enum in api/umh-core/v2/models.proto
	ValidMetricTypes = map[string]struct{}{
		"dfc":                 {}, // Data Flow Component metrics
		"protocol-converter": {}, // Protocol Converter metrics
		"redpanda":            {}, // Redpanda broker metrics
		"stream-processor":    {}, // Stream processor metrics
		"topic-browser":       {}, // Topic browser metrics
	}

	// MetricTypesRequiringUUID specifies which metric types need a uuid
	MetricTypesRequiringUUID = map[string]struct{}{
		"dfc":                 {},
		"protocol-converter": {},
		"stream-processor":    {},
	}
)

// Helper to convert time.Time to *timestamppb.Timestamp for API responses
func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// decompressPayload detects and decompresses gzip or zstd compressed data.
// Returns the decompressed bytes if compression is detected, otherwise returns the original data.
// Supports:
// - gzip: magic bytes 0x1f 0x8b
// - zstd: magic bytes 0x28 0xb5 0x2f 0xfd
func decompressPayload(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// Check for zstd compression (magic bytes: 0x28 0xb5 0x2f 0xfd)
	if len(data) >= 4 && data[0] == 0x28 && data[1] == 0xb5 && data[2] == 0x2f && data[3] == 0xfd {
		decoder, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
		}
		defer decoder.Close()

		decompressed, err := io.ReadAll(decoder)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress zstd data: %w", err)
		}
		return decompressed, nil
	}

	// Check for gzip compression (magic bytes: 0x1f 0x8b)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip data: %w", err)
		}
		return decompressed, nil
	}

	// Not compressed, return as-is
	return data, nil
}

// DeviceMgmtService implements v1.DeviceMgmtServiceServer
type DeviceMgmtService struct {
	v1.UnimplementedDeviceMgmtServiceServer

	deviceUc              *biz.DeviceUsecase
	actionUc              *biz.ActionUsecase
	instanceUc            *biz.InstanceUsecase
	protocolConverterRepo *data.ProtocolConverterRepo
	dataModelRepo         *data.DataModelRepo
	streamProcessorRepo   *data.StreamProcessorRepo
}

// NewDeviceMgmtService creates a new device management service
func NewDeviceMgmtService(deviceUc *biz.DeviceUsecase, actionUc *biz.ActionUsecase, instanceUc *biz.InstanceUsecase, protocolConverterRepo *data.ProtocolConverterRepo, dataModelRepo *data.DataModelRepo, streamProcessorRepo *data.StreamProcessorRepo) *DeviceMgmtService {
	return &DeviceMgmtService{
		deviceUc:              deviceUc,
		actionUc:              actionUc,
		instanceUc:            instanceUc,
		protocolConverterRepo: protocolConverterRepo,
		dataModelRepo:         dataModelRepo,
		streamProcessorRepo:   streamProcessorRepo,
	}
}

// Health checks service health status
// RPC: GET /health
// Used by Docker healthchecks, Kubernetes probes, load balancers
func (s *DeviceMgmtService) Health(ctx context.Context, req *emptypb.Empty) (*v1.HealthResponse, error) {
	// Check database connectivity by attempting a simple operation
	dbConnected := true
	var dbErr string
	
	// Try to ping the database via a simple devices list operation
	_, err := s.deviceUc.ListDevices(ctx, &storage.DeviceListFilter{PageSize: 1})
	if err != nil {
		dbConnected = false
		dbErr = err.Error()
	}

	status := "healthy"
	message := "Service is healthy"
	
	if !dbConnected {
		status = "degraded"
		message = "Service is running but database connection failed: " + dbErr
	}

	diagnostics := map[string]string{
		"database_driver": "sqlite3", // TODO: Make this configurable
	}
	if !dbConnected {
		diagnostics["database_error"] = dbErr
	}

	return &v1.HealthResponse{
		Status:             status,
		Message:            message,
		Version:            "ce1a8bb", // TODO: Get from build info
		Timestamp:          timestamppb.Now(),
		DatabaseConnected:  dbConnected,
		Diagnostics:        diagnostics,
	}, nil
}

// RegisterDevice creates a new device and generates auth token
// RPC: POST /api/v1/devicemgmt/devices
func (s *DeviceMgmtService) RegisterDevice(ctx context.Context, req *v1.RegisterDeviceRequest) (*v1.RegisterDeviceResponse, error) {
	if req == nil || req.Name == "" || req.CreatedBy == "" || req.Location == nil {
		return nil, status.Error(codes.InvalidArgument, "name, createdBy, and location are required")
	}

	if req.Location.Levels == nil || req.Location.Levels["0"] == "" {
		return nil, status.Error(codes.InvalidArgument, "location level '0' (company) is required")
	}

	// Call business logic
	resp, err := s.deviceUc.RegisterDevice(ctx, &biz.RegisterDeviceRequest{
		CreatedBy:   req.CreatedBy,
		Name:        req.Name,
		Location:    req.Location,
		Certificate: req.Certificate,
	})
	if err != nil {
		if err.Error() == "device already registered" {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Map response
	return &v1.RegisterDeviceResponse{
		Device:       resp.Device,
		AuthToken:    resp.AuthToken,
		Instructions: resp.Instructions,
	}, nil
}

// ListDevices retrieves all devices with cursor-based pagination
// RPC: GET /api/v1/devicemgmt/devices
func (s *DeviceMgmtService) ListDevices(ctx context.Context, req *v1.ListDevicesRequest) (*v1.ListDevicesResponse, error) {
	if req == nil {
		req = &v1.ListDevicesRequest{PageSize: 20}
	}

	// Default pagination with max limit
	if req.PageSize <= 0 {
		req.PageSize = 20
	} else if req.PageSize > 100 {
		req.PageSize = 100
	}

	// Decode page token to offset
	offset := int32(0)
	if req.PageToken != "" {
		var err error
		offset, err = decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
	}

	// Map sort field enum to string
	sortBy := "created_at" // default
	if req.SortBy == v1.DeviceSortField_NAME {
		sortBy = "name"
	} else if req.SortBy == v1.DeviceSortField_LAST_SEEN {
		sortBy = "last_seen"
	} else if req.SortBy == v1.DeviceSortField_CREATED_BY {
		sortBy = "created_by"
	}

	// Build filter
	filter := &storage.DeviceListFilter{
		PageSize:       req.PageSize,
		Offset:         offset,
		LocationFilter: req.LocationLevel_0,
		Search:         req.Search,
		CreatedBy:      req.CreatedBy,
		SortBy:         sortBy,
		SortDesc:       req.SortDesc,
	}

	// TODO: Implement status filtering with repeated DeviceStatus
	// Currently the storage layer uses *DeviceStatus pointer
	// Need to refactor to support multiple status filters

	// Call business logic
	devices, err := s.deviceUc.ListDevices(ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Prepare next page token
	// Storage layer fetches pageSize + 1 to detect if more results exist
	nextPageToken := ""
	if len(devices) > int(req.PageSize) {
		// More results available - truncate to requested page size
		devices = devices[:req.PageSize]
		nextPageToken = encodePageToken(offset + req.PageSize)
	}

	// Map response
	return &v1.ListDevicesResponse{
		Devices:         devices,
		NextPageToken:   nextPageToken,
	}, nil
}

// encodePageToken encodes an offset into a page token string
func encodePageToken(offset int32) string {
	return fmt.Sprintf("%d", offset)
}

// decodePageToken decodes a page token string back to an offset
func decodePageToken(token string) (int32, error) {
	if token == "" {
		return 0, nil // First page, offset is 0
	}
	var offset int32
	_, err := fmt.Sscanf(token, "%d", &offset)
	return offset, err
}

// GetDevice retrieves a device by ID
// RPC: GET /api/v1/devicemgmt/devices/{device_id}
func (s *DeviceMgmtService) GetDevice(ctx context.Context, req *v1.GetDeviceRequest) (*v1.Device, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Call business logic
	device, err := s.deviceUc.GetDevice(ctx, req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return device, nil
}

// GetDeviceHealth retrieves device health metrics from latest Push API data
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/health
func (s *DeviceMgmtService) GetDeviceHealth(ctx context.Context, req *v1.GetDeviceHealthRequest) (*v1.DeviceHealthResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Call business logic
	resp, err := s.deviceUc.GetDeviceHealth(ctx, req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	// Map response
	return &v1.DeviceHealthResponse{
		DeviceId:        resp.DeviceId,
		LastUpdated:     resp.LastUpdated,
		Status:          resp.Status,
		StatusB64:       resp.StatusB64,
		DeviceTimestamp: resp.DeviceTimestamp,
		ErrorMessage:    resp.ErrorMessage,
	}, nil
}

// UpdateDevice updates device information
// RPC: PATCH /api/v1/devicemgmt/devices/{device_id}
func (s *DeviceMgmtService) UpdateDevice(ctx context.Context, req *v1.UpdateDeviceRequest) (*v1.Device, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Build update struct
	updates := &biz.DeviceUpdate{}
	if req.Name != "" {
		updates.Name = &req.Name
	}
	if req.Location != nil {
		updates.Location = req.Location
	}

	// Call business logic
	device, err := s.deviceUc.UpdateDevice(ctx, req.DeviceId, updates)
	if err != nil {
		if err.Error() == "device not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return device, nil
}

// DeleteDevice removes a device
// RPC: DELETE /api/v1/devicemgmt/devices/{device_id}
func (s *DeviceMgmtService) DeleteDevice(ctx context.Context, req *v1.DeleteDeviceRequest) (*emptypb.Empty, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Call business logic
	err := s.deviceUc.DeleteDevice(ctx, req.DeviceId)
	if err != nil {
		if err.Error() == "record not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// GetDeviceConfig retrieves device configuration
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/config
func (s *DeviceMgmtService) GetDeviceConfig(ctx context.Context, req *v1.GetDeviceConfigRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Queue a get-config-file action (umh-core GetConfigFile doesn't use payload)
	// Default TTL: 5 minutes (300 seconds)
	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-config-file",
		Payload:    nil,
		TTLSeconds: 300, // 5 minutes
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

// SetDeviceConfig updates device configuration
// RPC: POST /api/v1/devicemgmt/devices/config/{device_id}
func (s *DeviceMgmtService) SetDeviceConfig(ctx context.Context, req *v1.SetDeviceConfigRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Payload == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}

	// Queue a set-config-file action with the config as payload
	// This translates to sending a set-config-file action to the device
	// Note: SetConfigFilePayload must be marshalled to JSON (not proto binary)
	// so that actionToJSONContent can unmarshal it correctly
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload to JSON")
	}

	// Create an Any message with the JSON payload
	payload := &anypb.Any{
		TypeUrl: "type.googleapis.com/api.umh_core.v2.SetConfigFilePayload",
		Value:   payloadJSON,
	}

	// Default TTL: 5 minutes (300 seconds)
	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "set-config-file",
		Payload:    payload,
		TTLSeconds: 300, // 5 minutes
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Return response with pending status
	return &v1.ActionQueuedResponse{
		ActionId:  action.Id,
		CreatedAt: timeToProto(action.CreatedAt),
		ExpiresAt: timeToProto(action.ExpiresAt),
	}, nil
}

// GetDeviceConfigActionResponse retrieves the result of a GetDeviceConfig or SetDeviceConfig action
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/config/{action_id}/result
func (s *DeviceMgmtService) GetDeviceConfigActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.DeviceConfigActionResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Retrieve the action
	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}

	// Verify the action belongs to the requested device
	if action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	// Map internal action status to proto action status
	protoStatus := actionStatusToProto(action.Status)

	// Build response
	resp := &v1.DeviceConfigActionResponse{
		ActionId:     action.Id,
		Status:       protoStatus,
		CompletedAt:  timeToProto(action.CompletedAt),
		ErrorMessage: action.ErrorMessage,
	}

	// Retrieve result payload only if action completed successfully
	// DeviceConfigActionResponse.status controls success/failure:
	//   - status=COMPLETED → result field is populated (operation succeeded on device)
	//   - status=FAILED → result field is empty, error_message contains error details
	//   - other statuses → action still pending, result field empty
	if action.Status == models.ActionStatusCompleted {
		// Fetch result from messages table via action_message_tracking correlation
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
		if err == nil && resultPayload != nil {
			// Deserialize based on action type
			switch action.Type {
			case "get-config-file":
				var getResp v2.GetConfigFileResponse
				if err := json.Unmarshal(resultPayload, &getResp); err == nil {
					resp.Result = &v1.DeviceConfigActionResponse_GetConfigResponse{
						GetConfigResponse: &getResp,
					}
				}
			case "set-config-file":
				var setResp v2.SetConfigFileResponse
				if err := json.Unmarshal(resultPayload, &setResp); err == nil {
					resp.Result = &v1.DeviceConfigActionResponse_SetConfigResponse{
						SetConfigResponse: &setResp,
					}
				}
			}
		}
	}

	return resp, nil
}

// GetLogs retrieves device logs
// RPC: POST /api/v1/devicemgmt/devices/{device_id}/logs
func (s *DeviceMgmtService) GetLogs(ctx context.Context, req *v1.GetLogsRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Payload == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}

	// Validate log type
	logType := req.Payload.Type
	if logType == "" {
		return nil, status.Error(codes.InvalidArgument, "payload.type is required")
	}

	if _, valid := ValidLogTypes[logType]; !valid {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf(
			"invalid log type: %q. Allowed types: agent, dfc, protocol-converter-read, protocol-converter-write, redpanda, topic-browser, stream-processor",
			logType))
	}

	// Validate uuid requirement
	if _, requiresUUID := LogTypesRequiringUUID[logType]; requiresUUID {
		if req.Payload.Uuid == "" {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf(
				"payload.uuid is required for log type %q", logType))
		}
	}

	// Marshal payload to JSON (must be JSON, not proto binary, for umh-core compatibility)
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload to JSON")
	}

	// Create an Any message with the JSON payload
	payload := &anypb.Any{
		TypeUrl: "type.googleapis.com/api.umh_core.v2.GetLogsPayload",
		Value:   payloadJSON,
	}

	// Queue a get-logs action with 5 minute TTL
	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-logs",
		Payload:    payload,
		TTLSeconds: 300, // 5 minutes
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

// GetMetrics retrieves device metrics
// RPC: POST /api/v1/devicemgmt/devices/{device_id}/metrics
func (s *DeviceMgmtService) GetMetrics(ctx context.Context, req *v1.GetMetricsRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Payload == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}

	// Validate metric type
	metricType := req.Payload.Type
	if metricType == "" {
		return nil, status.Error(codes.InvalidArgument, "payload.type is required")
	}

	if _, valid := ValidMetricTypes[metricType]; !valid {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf(
			"invalid metric type: %q. Allowed types: dfc, protocol-converter, redpanda, stream-processor, topic-browser",
			metricType))
	}

	// Validate uuid requirement
	if _, requiresUUID := MetricTypesRequiringUUID[metricType]; requiresUUID {
		if req.Payload.Uuid == "" {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf(
				"payload.uuid is required for metric type %q", metricType))
		}
	}

	// Marshal payload to JSON (must be JSON, not proto binary, for umh-core compatibility)
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload to JSON")
	}

	// Create an Any message with the JSON payload
	payload := &anypb.Any{
		TypeUrl: "type.googleapis.com/api.umh_core.v2.GetMetricsPayload",
		Value:   payloadJSON,
	}

	// Queue a get-metrics action with 5 minute TTL
	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-metrics",
		Payload:    payload,
		TTLSeconds: 300, // 5 minutes
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

// GetLogsActionResponse retrieves the result of a GetLogs action
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/logs/{action_id}/result
//
// Response payload format:
// - result_payload contains raw JSON bytes from umh-core GetLogsResponse
// - Format: JSON array of log strings ["line1", "line2", ...]
// - May be gzip or zstd compressed (client responsible for decompression)
func (s *DeviceMgmtService) GetLogsActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.LogsActionResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Retrieve the action
	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}

	// Verify the action belongs to the requested device
	if action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	// Map internal action status to proto action status
	protoStatus := actionStatusToProto(action.Status)

	// Build response
	resp := &v1.LogsActionResponse{
		ActionId:     action.Id,
		Status:       protoStatus,
		CompletedAt:  timeToProto(action.CompletedAt),
		ErrorMessage: action.ErrorMessage,
	}

	// Retrieve result payload only if action completed successfully
	// LogsActionResponse.status controls success/failure:
	//   - status=COMPLETED → result_payload field is populated (operation succeeded on device)
	//     result_payload = decompressed JSON string of log array
	//   - status=FAILED → result_payload field is empty, error_message contains error details
	//   - other statuses (QUEUED, DELIVERED, PROCESSING) → action still pending, result_payload empty
	if action.Status == models.ActionStatusCompleted {
		// Fetch result from messages table via action_message_tracking correlation
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
		if err == nil && resultPayload != nil {
			// Detect and decompress if payload is gzip or zstd compressed
			decompressed, err := decompressPayload(resultPayload)
			if err != nil {
				// Log decompression error but don't fail the request
				// Return original payload if decompression fails (convert bytes to string)
				resp.ResultPayload = string(resultPayload)
			} else {
				// Return decompressed payload as JSON string (no base64 encoding)
				resp.ResultPayload = string(decompressed)
			}
		}
	}

	return resp, nil
}

// GetMetricsActionResponse retrieves the result of a GetMetrics action
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/metrics/{action_id}/result
//
// Response payload format:
// - result_payload contains JSON string from umh-core GetMetricsResponse
// - Format: JSON array of Metric objects with {name, path, componentType, valueType, value}
// - Server automatically decompresses gzip/zstd payloads before returning
// - Client can directly parse as JSON without base64 decoding
func (s *DeviceMgmtService) GetMetricsActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.MetricsActionResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Retrieve the action
	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}

	// Verify the action belongs to the requested device
	if action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	// Map internal action status to proto action status
	protoStatus := actionStatusToProto(action.Status)

	// Build response
	resp := &v1.MetricsActionResponse{
		ActionId:     action.Id,
		Status:       protoStatus,
		CompletedAt:  timeToProto(action.CompletedAt),
		ErrorMessage: action.ErrorMessage,
	}

	// Retrieve result payload only if action completed successfully
	// MetricsActionResponse.status controls success/failure:
	//   - status=COMPLETED → result_payload field is populated (operation succeeded on device)
	//     result_payload = decompressed JSON string of Metric array
	//   - status=FAILED → result_payload field is empty, error_message contains error details
	//   - other statuses (QUEUED, DELIVERED, PROCESSING) → action still pending, result_payload empty
	if action.Status == models.ActionStatusCompleted {
		// Fetch result from messages table via action_message_tracking correlation
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
		if err == nil && resultPayload != nil {
			// Detect and decompress if payload is gzip or zstd compressed
			decompressed, err := decompressPayload(resultPayload)
			if err != nil {
				// Log decompression error but don't fail the request
				// Return original payload if decompression fails (convert bytes to string)
				resp.ResultPayload = string(resultPayload)
			} else {
				// Return decompressed payload as JSON string (no base64 encoding)
				resp.ResultPayload = string(decompressed)
			}
		}
	}

	return resp, nil
}

// ============================================================================
// Protocol Converter RPCs (4 endpoints)
// ============================================================================

// DeployProtocolConverter deploys a new protocol converter
// RPC: POST /api/v1/devicemgmt/devices/{device_id}/protocol-converters
func (s *DeviceMgmtService) DeployProtocolConverter(ctx context.Context, req *v1.DeployProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Payload == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}

	// Validate required payload fields
	if req.Payload.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "payload.name is required")
	}
	if req.Payload.Connection == nil || req.Payload.Connection.Ip == "" || req.Payload.Connection.Port == 0 {
		return nil, status.Error(codes.InvalidArgument, "payload.connection (IP + Port) is required")
	}

	// UUID is optional - umh-core will generate it from the name if not provided
	// This is intentional: same name always produces same UUID (idempotent)

	// Marshal payload to JSON for wire protocol
	payload, err := convertRequestToAny(req.Payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	// Queue action
	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "deploy-protocol-converter",
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

// EditProtocolConverter edits an existing protocol converter
// RPC: PATCH /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{uuid}
func (s *DeviceMgmtService) EditProtocolConverter(ctx context.Context, req *v1.EditProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required (path parameter)")
	}
	if req.Payload == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}

	// Validate UUID consistency: if payload.uuid is provided, it must match path uuid
	if req.Payload.Uuid != "" && req.Payload.Uuid != req.Uuid {
		return nil, status.Error(codes.InvalidArgument,
			fmt.Sprintf("payload uuid (%s) does not match path uuid (%s)",
				req.Payload.Uuid, req.Uuid))
	}

	// Set the authoritative UUID from path into payload
	req.Payload.Uuid = req.Uuid

	// At least one field must be provided for editing (besides uuid)
	// Note: All fields except UUID are optional for partial updates
	if req.Payload.Name == "" && req.Payload.Connection == nil &&
		req.Payload.ReadDfc == nil && req.Payload.WriteDfc == nil &&
		req.Payload.TemplateInfo == nil && req.Payload.Meta == nil &&
		len(req.Payload.Location) == 0 {
		return nil, status.Error(codes.InvalidArgument,
			"at least one field must be provided for editing")
	}

	// Marshal only the payload (the full ProtocolConverter object)
	// umh-core expects the entire ProtocolConverter with UUID set
	payload, err := convertRequestToAny(req.Payload)
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

// GetProtocolConverter retrieves protocol converter details
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{uuid}
func (s *DeviceMgmtService) GetProtocolConverter(ctx context.Context, req *v1.GetProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	// Create GetProtocolConverterPayload as JSON
	// umh-core expects: {"uuid": "<uuid>"}
	getPayload := map[string]string{
		"uuid": req.Uuid,
	}
	payloadJSON, err := json.Marshal(getPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	payload := &anypb.Any{
		TypeUrl: "type.googleapis.com/api.umh_core.v2.GetProtocolConverterPayload",
		Value:   payloadJSON,
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-protocol-converter",
		Payload:    payload,
		MaxRetries: 3,
		TTLSeconds: 1800,
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

// DeleteProtocolConverter deletes a protocol converter
// RPC: DELETE /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{uuid}
func (s *DeviceMgmtService) DeleteProtocolConverter(ctx context.Context, req *v1.DeleteProtocolConverterRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	// Create DeleteProtocolConverterPayload as JSON
	// umh-core expects: {"uuid": "<uuid>"}
	deletePayload := map[string]string{
		"uuid": req.Uuid,
	}
	payloadJSON, err := json.Marshal(deletePayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	payload := &anypb.Any{
		TypeUrl: "type.googleapis.com/api.umh_core.v2.DeleteProtocolConverterPayload",
		Value:   payloadJSON,
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "delete-protocol-converter",
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

// GetProtocolConverterActionResponse retrieves the result of a protocol converter action
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters/{action_id}/result
//
// Response payload format:
// - result field contains ProtocolConverter object (Deploy/Edit/Get actions)
// - result field is empty for Delete actions (status=COMPLETED just confirms deletion)
// - error_message populated only if status=FAILED
func (s *DeviceMgmtService) GetProtocolConverterActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.ProtocolConverterActionResponse, error) {
	// TEMP: Force it to work
	if req == nil || req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Retrieve the action
	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}

	// Verify the action belongs to the requested device (if device_id provided)
	if req.DeviceId != "" && action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	// Map internal action status to proto action status
	protoStatus := actionStatusToProto(action.Status)
	fmt.Printf("DEBUG: Action status=%s, Action.Status=%d, models.ActionStatusCompleted=%d\n", protoStatus, action.Status, models.ActionStatusCompleted)

	// Build response
	resp := &v1.ProtocolConverterActionResponse{
		ActionId:     action.Id,
		Status:       protoStatus,
		CompletedAt:  timeToProto(action.CompletedAt),
		ErrorMessage: action.ErrorMessage,
	}

	// Populate error message if action failed or parse failed
	if action.Status == models.ActionStatusFailed || action.Status == models.ActionStatusFailedParsingResponse {
		resp.ErrorMessage = action.ErrorMessage
		return resp, nil
	}

	// Retrieve result payload only if action completed successfully
	// - status=COMPLETED → result field is populated (ProtocolConverter object)
	// - status=FAILED → result field is empty, error_message contains error details
	// - status=DELETED (Delete action succeeded) → result field is empty (expected)
	// - other statuses (QUEUED, DELIVERED, PROCESSING) → action still pending, result empty
	if action.Status == models.ActionStatusCompleted {
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
		if err == nil && resultPayload != nil && len(resultPayload) > 0 {
			// Decompress if needed
			decompressed, err := decompressPayload(resultPayload)
			if err == nil {
				// Try to unmarshal result payload into ProtocolConverter
				var result v2.ProtocolConverter
				if err := json.Unmarshal(decompressed, &result); err == nil {
					resp.Result = &result
				}
				// If unmarshal fails, just leave result empty (expected for Delete actions)
			}
		}
	}

	return resp, nil
}

// ListProtocolConverters lists all protocol converters for a device
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/protocol-converters
func (s *DeviceMgmtService) ListProtocolConverters(ctx context.Context, req *v1.ListProtocolConvertersRequest) (*v1.ListProtocolConvertersResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Validate device exists and get its status for health determination
	device, err := s.deviceUc.GetDevice(ctx, req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}

	// Determine health status based on device heartbeat
	deviceHealthStatus := "UNKNOWN"
	if device.LastSeen != nil {
		lastSeen := device.LastSeen.AsTime()
		if lastSeen.After(time.Now().Add(-5 * time.Minute)) {
			deviceHealthStatus = "ONLINE"
		} else {
			deviceHealthStatus = "OFFLINE"
		}
	}

	// Handle pagination
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 20 // default
	}
	if pageSize > 100 {
		pageSize = 100 // max
	}

	// Decode page token to get offset
	offset := int64(0)
	if req.PageToken != "" {
		decodedOffset, err := decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = int64(decodedOffset)
	}

	// Build query for repository
	query := &data.ListQuery{
		DeviceID:              req.DeviceId,
		UUIDFilter:            req.UuidFilter,
		NameFilter:            req.NameFilter,
		TypeFilter:            req.TypeFilter,
		DeploymentStatusFilter: req.DeploymentStatusFilter,
		ConnectionUUIDFilter:   req.ConnectionUuidFilter,
		HealthStatusFilter:     req.HealthStatusFilter,
		Offset:                offset,
		Limit:                 int64(pageSize) + 1, // Fetch one extra to detect if more pages exist
	}

	// Query the database
	converters, err := s.protocolConverterRepo.List(ctx, tenantID, query)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to list protocol converters: %v", err))
	}

	// Prepare next page token
	// Storage layer fetches pageSize + 1 to detect if more results exist
	nextPageToken := ""
	if len(converters) > int(pageSize) {
		// More results available - truncate to requested page size
		converters = converters[:pageSize]
		nextPageToken = encodePageToken(int32(offset + int64(pageSize)))
	}

	// Build response
	// Note: We always initialize converters slice to ensure it appears in JSON output (even if empty)
	// This is necessary because proto3 omits nil/empty slices by default
	response := &v1.ListProtocolConvertersResponse{
		Converters: make([]*v1.ProtocolConverterSummary, len(converters)),
		NextPageToken: nextPageToken,
	}

	for i, pc := range converters {
		// Determine health status from DB or device heartbeat
		healthStatus := pc.HealthStatus
		if healthStatus == "UNKNOWN" && deviceHealthStatus != "UNKNOWN" {
			healthStatus = deviceHealthStatus
		}

		// Format last_sync_time as RFC3339
		var lastSyncTime string
		if pc.LastSynced.Valid {
			lastSyncTime = pc.LastSynced.String
		}

		// Build error message string
		errorMessage := ""
		if pc.ErrorMessage.Valid {
			errorMessage = pc.ErrorMessage.String
		}

		response.Converters[i] = &v1.ProtocolConverterSummary{
			Uuid:               pc.UUID,
			Name:               pc.Name,
			Type:               pc.Type,
			ConnectionUUID:     pc.ConnectionUUID,
			DeploymentStatus:   pc.DeploymentStatus,
			HealthStatus:       healthStatus,
			LastSyncTime:       lastSyncTime,
			ErrorMessage:       errorMessage,
		}
	}

	// NOTE: Proto3 omits empty slices/zero values in JSON by default
	// This is a known issue - the proto library doesn't emit defaults
	// If converters is empty, this will return {} instead of {converters: [], next_page_token: ""}
	// The fix would require:
	// 1. Using protojson with custom marshal options (not available in this proto version), or
	// 2. Converting response to a custom struct, or
	// 3. Using google.api.HttpRule with custom encoding
	// For now, returning the proto response as-is
	return response, nil
}

// ============================================================================
// Data Flow Component RPCs (4 endpoints)
// ============================================================================

// DeployDataFlowComponent deploys a new data flow component
// RPC: POST /api/v1/devicemgmt/devices/{device_id}/data-flow-components
func (s *DeviceMgmtService) DeployDataFlowComponent(ctx context.Context, req *v1.DeployDataFlowComponentRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	payload, err := convertRequestToAny(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "deploy_data_flow_component",
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

// EditDataFlowComponent edits an existing data flow component
// RPC: PATCH /api/v1/devicemgmt/devices/{device_id}/data-flow-components/{uuid}
func (s *DeviceMgmtService) EditDataFlowComponent(ctx context.Context, req *v1.EditDataFlowComponentRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	payload, err := convertRequestToAny(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "edit_data_flow_component",
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

// GetDataFlowComponent retrieves data flow component details
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/data-flow-components/{uuid}
func (s *DeviceMgmtService) GetDataFlowComponent(ctx context.Context, req *v1.GetDataFlowComponentRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	payload, err := convertRequestToAny(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get_data_flow_component",
		Payload:    payload,
		MaxRetries: 3,
		TTLSeconds: 1800,
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

// DeleteDataFlowComponent deletes a data flow component
// RPC: DELETE /api/v1/devicemgmt/devices/{device_id}/data-flow-components/{uuid}
func (s *DeviceMgmtService) DeleteDataFlowComponent(ctx context.Context, req *v1.DeleteDataFlowComponentRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	payload, err := convertRequestToAny(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "delete_data_flow_component",
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

// ============================================================================
// Data Model RPCs (5 endpoints)
// ============================================================================

// AddDataModel adds a new data model
// RPC: POST /api/v1/devicemgmt/devices/{device_id}/data-models
func (s *DeviceMgmtService) AddDataModel(ctx context.Context, req *v1.AddDataModelRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Validate required fields
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Structure == nil {
		return nil, status.Error(codes.InvalidArgument, "structure is required")
	}

	// Convert JSON structure to YAML
	yamlStructure, err := protoStructToYAMLString(req.Structure)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert structure: %v", err))
	}

	// Encode structure (YAML) to base64
	encodedStructure := base64.StdEncoding.EncodeToString([]byte(yamlStructure))

	// Create umh-core payload with encoded structure
	addPayload := &v2.AddDataModelPayload{
		Name:              req.Name,
		Description:       req.Description,
		EncodedStructure:  encodedStructure,
	}

	payload, err := convertRequestToAny(addPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "add-datamodel",
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

// EditDataModel edits an existing data model
// RPC: PATCH /api/v1/devicemgmt/devices/{device_id}/data-models/{name}
func (s *DeviceMgmtService) EditDataModel(ctx context.Context, req *v1.EditDataModelRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required (path parameter)")
	}
	if req.Structure == nil {
		return nil, status.Error(codes.InvalidArgument, "structure is required")
	}

	// Convert JSON structure to YAML
	yamlStructure, err := protoStructToYAMLString(req.Structure)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert structure: %v", err))
	}

	// Encode structure (YAML) to base64
	encodedStructure := base64.StdEncoding.EncodeToString([]byte(yamlStructure))

	// Create umh-core payload with encoded structure
	editPayload := &v2.EditDataModelPayload{
		Name:              req.Name,
		Description:       req.Description,
		EncodedStructure:  encodedStructure,
	}

	payload, err := convertRequestToAny(editPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "edit-datamodel",
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

// GetDataModel retrieves data model details
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/data-models/{name}
func (s *DeviceMgmtService) GetDataModel(ctx context.Context, req *v1.GetDataModelRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required (path parameter)")
	}

	// Create payload with name, version, and getEnrichedTree flag
	getPayload := &v2.GetDataModelPayload{
		Name:            req.Name,
		Version:         req.Version, // Optional: empty string means get all versions
		GetEnrichedTree: req.GetEnrichedTree,
	}

	payload, err := convertRequestToAny(getPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-datamodel",
		Payload:    payload,
		MaxRetries: 3,
		TTLSeconds: 1800,
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

// DeleteDataModel deletes a data model
// RPC: DELETE /api/v1/devicemgmt/devices/{device_id}/data-models/{name}
func (s *DeviceMgmtService) DeleteDataModel(ctx context.Context, req *v1.DeleteDataModelRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required (path parameter)")
	}

	// Create payload using proper proto marshaling (avoid string formatting security issues)
	deletePayload := &v2.DeleteDataModelPayload{
		Name: req.Name,
	}
	
	payload, err := convertRequestToAny(deletePayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "delete-datamodel",
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

// GetDataModelActionResponse retrieves the result of a data model action
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/data-models/{name}/result
//
// Response payload format:
// - result field contains DataModelOperationResult object (Add/Edit/Get actions)
// - result field is empty for Delete actions (status=COMPLETED just confirms deletion)
// - error_message populated only if status=FAILED
func (s *DeviceMgmtService) GetDataModelActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.DataModelActionResponse, error) {
	if req == nil || req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Retrieve the action
	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}

	// Verify the action belongs to the requested device (if device_id provided)
	if req.DeviceId != "" && action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	response := &v1.DataModelActionResponse{
		ActionId:    action.Id,
		Status:      actionStatusToProto(action.Status),
		CompletedAt: timeToProto(action.CompletedAt),
	}

	// Handle failed actions first
	if action.Status == models.ActionStatusFailed || action.Status == models.ActionStatusFailedParsingResponse {
		response.ErrorMessage = action.ErrorMessage
		return response, nil
	}

	// Only parse result if action succeeded
	// Retrieve result payload (may be compressed)
	if action.Status == models.ActionStatusCompleted {
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
		if err == nil && resultPayload != nil && len(resultPayload) > 0 {
			// Decompress if needed
			decompressed, err := decompressPayload(resultPayload)
			if err == nil {
				// Custom unmarshaling to handle structure field (which is a map in umh-core response)
				var rawResult map[string]interface{}
				if err := json.Unmarshal(decompressed, &rawResult); err == nil {
					// Handle two response formats from umh-core:
					// Format 1: Direct result (AddDataModel, EditDataModel)
					//   {name, version, description, structure, dataContract}
					// Format 2: Versions map (GetDataModel)
					//   {name, description, versions: {v1: {...}, v2: {...}, ...}}
					
					var resultData map[string]interface{}
					
					// Check if this is a versions-based response (GetDataModel)
					if versions, ok := rawResult["versions"].(map[string]interface{}); ok && len(versions) > 0 {
						// GetDataModel returns versions map
						// If request had a specific version, return that one
						// If no version specified, return all versions
						modelName := getStringField(rawResult, "name")
						modelDescription := getStringField(rawResult, "description")
						
						// Extract requested version from action payload (if specified)
						requestedVersion := ""
						if action.Payload != nil {
							var requestPayload map[string]interface{}
							if err := json.Unmarshal(action.Payload.Value, &requestPayload); err == nil {
								if v, ok := requestPayload["version"].(string); ok {
									requestedVersion = v
								}
							}
						}
						
						if requestedVersion != "" {
							// Return specific version requested
							versionKey := "v" + requestedVersion
							if versionData, ok := versions[versionKey].(map[string]interface{}); ok {
								resultData = make(map[string]interface{})
								resultData["name"] = modelName
								resultData["description"] = modelDescription
								resultData["version"] = requestedVersion
								
								// Try encodedStructure first (base64-encoded YAML)
								if encodedStructure, ok := versionData["encodedStructure"].(string); ok && encodedStructure != "" {
									if decodedYAML, err := base64.StdEncoding.DecodeString(encodedStructure); err == nil {
										var structureMap map[string]interface{}
										if err := yaml.Unmarshal(decodedYAML, &structureMap); err == nil {
											resultData["structure"] = structureMap
										}
									}
								} else if structureStr, ok := versionData["structure"].(string); ok && structureStr != "" {
									// Fallback: structure as JSON string
									var structureMap map[string]interface{}
									if err := json.Unmarshal([]byte(structureStr), &structureMap); err == nil {
										resultData["structure"] = structureMap
									}
								}
								if dataContract, ok := versionData["dataContract"]; ok {
									resultData["dataContract"] = dataContract
								}
							}
							
							// Build single result response
							if len(resultData) > 0 {
								var structProto *structpb.Struct
								if structVal, ok := resultData["structure"]; ok {
									if structMap, ok := structVal.(map[string]interface{}); ok {
										structProto, _ = mapToProtoStruct(structMap)
									}
								}
								
								result := &v2.DataModelOperationResult{
									Name:        getStringField(resultData, "name"),
									Version:     convertVersionToString(resultData["version"]),
									Description: getStringField(resultData, "description"),
									EncodedStructure: "",
									Structure: structProto,
									DataContract: unmarshalDataContractInfo(resultData["dataContract"]),
								}
								response.Result = result
							}
						} else {
							// Return all versions as a map
							response.Results = make(map[string]*v2.DataModelOperationResult)
							
							for versionKey, versionValue := range versions {
								if versionData, ok := versionValue.(map[string]interface{}); ok {
									// Extract version number from key (v1 -> 1, v2 -> 2, etc)
									versionNum := versionKey
									if len(versionKey) > 1 && versionKey[0] == 'v' {
										versionNum = versionKey[1:]
									}
									
									resultForVersion := &v2.DataModelOperationResult{
										Name:        modelName,
										Version:     versionNum,
										Description: modelDescription,
										EncodedStructure: "",
									}
									
									// Try encodedStructure first (base64-encoded YAML)
									if encodedStructure, ok := versionData["encodedStructure"].(string); ok && encodedStructure != "" {
										if decodedYAML, err := base64.StdEncoding.DecodeString(encodedStructure); err == nil {
											var structureMap map[string]interface{}
											if err := yaml.Unmarshal(decodedYAML, &structureMap); err == nil {
												if structProto, err := mapToProtoStruct(structureMap); err == nil {
													resultForVersion.Structure = structProto
												}
											}
										}
									} else if structureStr, ok := versionData["structure"].(string); ok && structureStr != "" {
										// Fallback: structure as JSON string
										if structProto, err := jsonStringToProtoStruct(structureStr); err == nil {
											resultForVersion.Structure = structProto
										}
									}
									
									if dataContract, ok := versionData["dataContract"]; ok {
										resultForVersion.DataContract = unmarshalDataContractInfo(dataContract)
									}
									
									response.Results[versionNum] = resultForVersion
								}
							}
						}
					} else {
						// Format 1: Direct result (AddDataModel, EditDataModel)
						resultData = rawResult
						if len(resultData) > 0 {
							var structProto *structpb.Struct
							if structVal, ok := resultData["structure"]; ok {
								if structMap, ok := structVal.(map[string]interface{}); ok {
									structProto, _ = mapToProtoStruct(structMap)
								}
							}
							
							result := &v2.DataModelOperationResult{
								Name:        getStringField(resultData, "name"),
								Version:     convertVersionToString(resultData["version"]),
								Description: getStringField(resultData, "description"),
								EncodedStructure: "",
								Structure: structProto,
								DataContract: unmarshalDataContractInfo(resultData["dataContract"]),
							}
							response.Result = result
						}
					}
				}
				// If unmarshal fails, just leave result empty (expected for Delete actions)
			}
		}
	}

	return response, nil
}

// ListDataModels lists all data models for a device (synchronous database query)
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/data-models
func (s *DeviceMgmtService) ListDataModels(ctx context.Context, req *v1.ListDataModelsRequest) (*v1.ListDataModelsResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	// Validate device exists
	device, err := s.deviceUc.GetDevice(ctx, req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}
	_ = device // device exists check passed

	// Handle pagination
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 20 // default
	}
	if pageSize > 100 {
		pageSize = 100 // max
	}

	// Decode page token to get offset
	offset := int64(0)
	if req.PageToken != "" {
		decodedOffset, err := decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = int64(decodedOffset)
	}

	// Build query for repository
	query := &data.DataModelListQuery{
		DeviceID:      req.DeviceId,
		NameFilter:    req.NameFilter,
		VersionFilter: req.VersionFilter,
		Offset:        offset,
		Limit:         int64(pageSize) + 1, // Fetch one extra to detect if more pages exist
	}

	// Query the database
	models, err := s.dataModelRepo.List(ctx, tenantID, query)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to list data models: %v", err))
	}

	// Prepare next page token
	// Storage layer fetches pageSize + 1 to detect if more results exist
	nextPageToken := ""
	if len(models) > int(pageSize) {
		// More results available - truncate to requested page size
		models = models[:pageSize]
		nextPageToken = encodePageToken(int32(offset + int64(pageSize)))
	}

	// Build response
	response := &v1.ListDataModelsResponse{
		Models:        make([]*v1.DataModelSummary, len(models)),
		NextPageToken: nextPageToken,
	}

	for i, dm := range models {
		response.Models[i] = &v1.DataModelSummary{
			Name:        dm.Name,
			Version:     dm.Version,
			Description: dm.Description.String,
		}
	}

	return response, nil
}

// ============================================================================
// Stream Processor RPCs (4 endpoints)
// ============================================================================

// DeployStreamProcessor deploys a new stream processor
// RPC: POST /api/v1/devicemgmt/devices/{device_id}/stream-processors
func (s *DeviceMgmtService) DeployStreamProcessor(ctx context.Context, req *v1.DeployStreamProcessorRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	// Encode config: JSON → YAML → Base64
	encodedConfig, err := encodeStreamProcessorConfig(req.Config)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to encode config: %v", err))
	}

	// Validate required fields for umh-core
	if req.ModelName == "" {
		return nil, status.Error(codes.InvalidArgument, "model_name is required")
	}
	if req.ModelVersion == "" {
		return nil, status.Error(codes.InvalidArgument, "model_version is required")
	}

	// Create umh-core compatible payload
	deployPayload := &v2.StreamProcessor{
		Uuid:              req.Uuid,  // Empty string = server-generated by umh-core
		Name:              req.Name,
		EncodedConfig:     encodedConfig,
		IgnoreHealthCheck: req.IgnoreHealthCheck,
		Model: &v2.StreamProcessorModelRef{
			Name:    req.ModelName,
			Version: req.ModelVersion,
		},
		Location: req.Location,  // map[int32]string - umh-core expects map[int]string but protobuf handles it
	}

	payload, err := convertRequestToAny(deployPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "deploy-stream-processor",
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

// EditStreamProcessor edits an existing stream processor
// RPC: PATCH /api/v1/devicemgmt/devices/{device_id}/stream-processors/{uuid}
func (s *DeviceMgmtService) EditStreamProcessor(ctx context.Context, req *v1.EditStreamProcessorRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required (path parameter)")
	}

	// Validate required fields (umh-core requires these to always be present)
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Config == nil {
		return nil, status.Error(codes.InvalidArgument, "config is required")
	}
	if req.ModelName == "" {
		return nil, status.Error(codes.InvalidArgument, "model_name is required")
	}
	if req.ModelVersion == "" {
		return nil, status.Error(codes.InvalidArgument, "model_version is required")
	}

	// Encode config (always required)
	encodedConfig, err := encodeStreamProcessorConfig(req.Config)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to encode config: %v", err))
	}

	// Build complete payload (all required fields must be present for umh-core validation)
	editPayload := &v2.StreamProcessor{
		Uuid:              req.Uuid,
		Name:              req.Name,
		EncodedConfig:     encodedConfig,
		IgnoreHealthCheck: req.IgnoreHealthCheck,
		Location:          req.Location,
		Model: &v2.StreamProcessorModelRef{
			Name:    req.ModelName,
			Version: req.ModelVersion,
		},
	}

	payload, err := convertRequestToAny(editPayload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "edit-stream-processor",
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

// GetStreamProcessor retrieves stream processor details
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/stream-processors/{uuid}
func (s *DeviceMgmtService) GetStreamProcessor(ctx context.Context, req *v1.GetStreamProcessorRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	payload, err := convertRequestToAny(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "get-stream-processor",
		Payload:    payload,
		MaxRetries: 3,
		TTLSeconds: 1800,
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

// DeleteStreamProcessor deletes a stream processor
// RPC: DELETE /api/v1/devicemgmt/devices/{device_id}/stream-processors/{uuid}
func (s *DeviceMgmtService) DeleteStreamProcessor(ctx context.Context, req *v1.DeleteStreamProcessorRequest) (*v1.ActionQueuedResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "uuid is required")
	}

	payload, err := convertRequestToAny(req)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal payload")
	}

	action, err := s.actionUc.QueueAction(ctx, &biz.QueueActionRequest{
		DeviceID:   req.DeviceId,
		ActionType: "delete-stream-processor",
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

// GetStreamProcessorActionResponse retrieves the result of a stream processor action
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/stream-processors/{action_id}/result
func (s *DeviceMgmtService) GetStreamProcessorActionResponse(ctx context.Context, req *v1.ActionResultRequest) (*v1.StreamProcessorActionResponse, error) {
	if req == nil || req.ActionId == "" {
		return nil, status.Error(codes.InvalidArgument, "action_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	action, err := s.actionUc.GetAction(ctx, tenantID, req.ActionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("action not found: %v", err))
	}

	if req.DeviceId != "" && action.DeviceId != req.DeviceId {
		return nil, status.Error(codes.PermissionDenied, "action does not belong to device")
	}

	protoStatus := actionStatusToProto(action.Status)

	resp := &v1.StreamProcessorActionResponse{
		ActionId:     action.Id,
		Status:       protoStatus,
		CompletedAt:  timeToProto(action.CompletedAt),
		ErrorMessage: action.ErrorMessage,
	}

	if action.Status == models.ActionStatusFailed || action.Status == models.ActionStatusFailedParsingResponse {
		resp.ErrorMessage = action.ErrorMessage
		return resp, nil
	}

	if action.Status == models.ActionStatusCompleted {
		resultPayload, err := s.instanceUc.GetActionResultPayload(ctx, tenantID, req.ActionId)
		if err == nil && resultPayload != nil && len(resultPayload) > 0 {
			decompressed, err := decompressPayload(resultPayload)
			if err == nil {
				var result v2.StreamProcessor
				if err := json.Unmarshal(decompressed, &result); err == nil {
					// Populate basic fields from result
					resp.Uuid = result.Uuid
					resp.Name = result.Name
					
					// Decode encodedConfig from base64 YAML to JSON Struct
					if result.EncodedConfig != "" {
						if decodedYAML, err := base64.StdEncoding.DecodeString(result.EncodedConfig); err == nil {
							var configMap map[string]interface{}
							if err := yaml.Unmarshal(decodedYAML, &configMap); err == nil {
								// Convert map to protobuf Struct
								if configStruct, err := structpb.NewStruct(configMap); err == nil {
									resp.Config = configStruct
								}
							}
						}
					}
					
					// Extract model reference
					if result.Model != nil {
						resp.Model = &v1.StreamProcessorModelRef{
							Name:    result.Model.Name,
							Version: result.Model.Version,
						}
					}
					
					// Extract location
					if result.Location != nil {
						resp.Location = result.Location
					}
				}
			}
		}
	}

	return resp, nil
}

// ListStreamProcessors lists all stream processors for a device
// RPC: GET /api/v1/devicemgmt/devices/{device_id}/stream-processors
func (s *DeviceMgmtService) ListStreamProcessors(ctx context.Context, req *v1.ListStreamProcessorsRequest) (*v1.ListStreamProcessorsResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// Multi-tenancy: extract tenant_id from context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}

	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Decode page token to get offset
	offset := int32(0)
	if req.PageToken != "" {
		decodedOffset, err := decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = decodedOffset
	}

	query := &data.StreamProcessorListQuery{
		DeviceID:              req.DeviceId,
		UUIDFilter:            req.UuidFilter,
		NameFilter:            req.NameFilter,
		DeploymentStatusFilter: req.DeploymentStatusFilter,
		HealthStatusFilter:     req.HealthStatusFilter,
		ModelNameFilter:        req.ModelNameFilter,
		Offset:                int64(offset),
		Limit:                 int64(pageSize) + 1, // Fetch one extra to detect if more pages exist
	}

	models, err := s.streamProcessorRepo.List(ctx, tenantID, query)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to list stream processors: %v", err))
	}

	// Prepare next page token
	// Storage layer fetches pageSize + 1 to detect if more results exist
	nextPageToken := ""
	if len(models) > int(pageSize) {
		// More results available - truncate to requested page size
		models = models[:pageSize]
		nextPageToken = encodePageToken(offset + pageSize)
	}

	response := &v1.ListStreamProcessorsResponse{
		Processors:    make([]*v1.StreamProcessorSummary, len(models)),
		NextPageToken: nextPageToken,
	}

	for i, sp := range models {
		response.Processors[i] = &v1.StreamProcessorSummary{
			Uuid: sp.UUID,
			Name: sp.Name,
			Model: &v2.StreamProcessorModelRef{
				Name:    sp.ModelName.String,
				Version: sp.ModelVersion.String,
			},
		}
	}

	return response, nil
}

// actionStatusToProto maps internal ActionStatus to proto ActionStatus
func actionStatusToProto(status models.ActionStatus) v1.ActionStatus {
	switch status {
	case models.ActionStatusQueued:
		return v1.ActionStatus_QUEUED
	case models.ActionStatusDelivered:
		return v1.ActionStatus_DELIVERED
	case models.ActionStatusProcessing:
		return v1.ActionStatus_PROCESSING
	case models.ActionStatusCompleted:
		return v1.ActionStatus_COMPLETED
	case models.ActionStatusFailed:
		return v1.ActionStatus_FAILED
	case models.ActionStatusExpired:
		return v1.ActionStatus_EXPIRED
	case models.ActionStatusCancelled:
		return v1.ActionStatus_CANCELLED
	case models.ActionStatusFailedParsingResponse:
		return v1.ActionStatus_FAILED_PARSING_RESPONSE
	default:
		return v1.ActionStatus_ACTION_STATUS_UNSPECIFIED
	}
}

// getStringField extracts a string value from a map, returning empty string if not found
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// convertVersionToString converts a version value (int, float64, or string) to string
func convertVersionToString(versionVal interface{}) string {
	switch v := versionVal.(type) {
	case float64:
		return fmt.Sprintf("%.0f", v) // "1", "2", etc
	case int:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return ""
	}
}

// marshalFieldToString converts a map or other field to a JSON string
func marshalFieldToString(field interface{}) string {
	if field == nil {
		return ""
	}
	jsonBytes, err := json.Marshal(field)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// protoStructToYAMLString converts a protobuf Struct to YAML string
func protoStructToYAMLString(s *structpb.Struct) (string, error) {
	if s == nil {
		return "", fmt.Errorf("struct is nil")
	}
	
	// Convert Struct to map[string]interface{}
	structMap := s.AsMap()
	
	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(structMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to YAML: %w", err)
	}
	
	return string(yamlBytes), nil
}

// jsonStringToProtoStruct converts a JSON string to protobuf Struct
func jsonStringToProtoStruct(jsonStr string) (*structpb.Struct, error) {
	if jsonStr == "" {
		return nil, fmt.Errorf("json string is empty")
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	
	return structpb.NewStruct(data)
}

// mapToProtoStruct converts a map to protobuf Struct
func mapToProtoStruct(m map[string]interface{}) (*structpb.Struct, error) {
	if len(m) == 0 {
		return nil, fmt.Errorf("map is empty")
	}
	
	return structpb.NewStruct(m)
}

// unmarshalDataContractInfo converts a map to DataContractInfo proto
func unmarshalDataContractInfo(field interface{}) *v2.DataContractInfo {
	if field == nil {
		return nil
	}
	
	m, ok := field.(map[string]interface{})
	if !ok {
		return nil
	}
	
	return &v2.DataContractInfo{
		Name:   getStringField(m, "name"),
		Model:  getStringField(m, "model"),
		Status: getStringField(m, "status"),
	}
}

// encodeStreamProcessorConfig converts JSON config to YAML base64
func encodeStreamProcessorConfig(config *structpb.Struct) (string, error) {
	if config == nil {
		return "", nil  // Optional field, can be empty
	}
	
	// JSON → YAML
	yamlStr, err := protoStructToYAMLString(config)
	if err != nil {
		return "", fmt.Errorf("failed to convert config to YAML: %w", err)
	}
	
	// YAML → Base64
	encoded := base64.StdEncoding.EncodeToString([]byte(yamlStr))
	return encoded, nil
}

func convertRequestToAny(msg interface{}) (*anypb.Any, error) {
	// Convert a protobuf message to anypb.Any with JSON serialization
	// IMPORTANT: Must serialize to JSON, not proto binary, for umh-core compatibility
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	// Type assert to proto message
	protoMsg, ok := msg.(interface{ ProtoReflect() protoreflect.Message })
	if !ok {
		return nil, fmt.Errorf("message is not a proto message")
	}

	// Get the message descriptor for TypeUrl
	msgDescriptor := protoMsg.ProtoReflect().Descriptor()
	fullName := string(msgDescriptor.FullName())
	typeUrl := "type.googleapis.com/" + fullName

	// Marshal to JSON (not proto binary!)
	payloadJSON, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	// Create Any with JSON payload
	anyPb := &anypb.Any{
		TypeUrl: typeUrl,
		Value:   payloadJSON,
	}

	return anyPb, nil
}
