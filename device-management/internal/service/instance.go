package service

import (
	"context"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/artpark-hub/taksa-platform/device-management/api/common"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
)

// InstanceService implements v2.InstanceServiceServer
type InstanceService struct {
	v2.UnimplementedInstanceServiceServer

	instanceUc *biz.InstanceUsecase
	authUc     *biz.AuthUsecase
	logger     *zap.Logger
}

// NewInstanceService creates a new instance service
func NewInstanceService(instanceUc *biz.InstanceUsecase, authUc *biz.AuthUsecase, logger *zap.Logger) *InstanceService {
	return &InstanceService{
		instanceUc: instanceUc,
		authUc:     authUc,
		logger:     logger,
	}
}

// Login authenticates a device and returns JWT token
// RPC: POST /api/v2/instance/login
// NOTE: No request body. Device identification comes from Authorization header and JWT context.
func (s *InstanceService) Login(ctx context.Context, req *emptypb.Empty) (*v2.LoginResponse, error) {
	// Extract token hash from context (HTTP AuthMiddleware or gRPC tenant interceptor on Login).
	// Device sends: Authorization: Bearer <token-hash>
	tokenHash := middleware.GetAuthorizationToken(ctx)
	if tokenHash == "" {
		s.logger.Warn("Login failed: missing or invalid authorization header")
		return nil, kerrors.Unauthorized("missing_authorization_token", "missing or invalid authorization header")
	}

	s.logger.Debug("Login API called",
		zap.String("token_hash_preview", tokenHash[:min(len(tokenHash), 16)]),
	)

	// Call business logic - token_hash maps to device_id internally
	resp, err := s.instanceUc.Login(ctx, tokenHash)
	if err != nil {
		s.logger.Error("Login failed",
			zap.String("token_hash_preview", tokenHash[:min(len(tokenHash), 16)]),
			zap.Error(err),
		)
		return nil, kerrors.Unauthorized("invalid_auth_token", err.Error())
	}

	s.logger.Info("Device logged in successfully",
		zap.String("device_id", resp.DeviceId),
	)

	// Map response to match umh-core's InstanceLoginResponse struct EXACTLY
	// File: umh-core/pkg/communicator/backend_api_structs/backend_api_struct.go:119-125
	// Populate CompanyDetails with available device/tenant information
	// This provides umh-core with useful metadata that may be needed in future versions

	// Convert certificate pointers (nil-safe)
	var certificate *string
	if resp.Certificate != "" {
		certificate = &resp.Certificate
	}

	var encryptedPrivateKey *string
	if resp.EncryptedPrivateKey != "" {
		encryptedPrivateKey = &resp.EncryptedPrivateKey
	}

	// Build CompanyDetails from available device/tenant information
	// owner and name = device owner (tenant identifier)
	// licenseStatus.validTo = auth token expiry date
	// licenseStatus.isActive = true (device has valid token)
	tokenExpiryDate := resp.Device.AuthTokenExpiresAt.AsTime().Format("2006-01-02")
	owner := resp.Device.CreatedBy
	companyDetails := &common.CompanyDetails{
		Owner: &owner,
		Name:  resp.Device.CreatedBy,
		LicenseStatus: &common.LicenseStatus{
			ValidTo:     tokenExpiryDate,
			IsActive:    true,
			Description: "Device token valid until " + tokenExpiryDate,
		},
	}

	// Build LoginResponse proto
	// Proto fields already use camelCase (encryptedPrivateKey, companyDetails, etc)
	// JSON marshaling will automatically produce correct output
	protoResp := &v2.LoginResponse{
		Certificate:         certificate,
		EncryptedPrivateKey: encryptedPrivateKey,
		Uuid:                resp.DeviceId,
		Name:                resp.Name,
		CompanyDetails:      companyDetails,
		JwtToken:            resp.JwtToken, // Will be set as cookie by HTTP encoder, not in JSON (taksa-specific field)
		ExpiresAt:           resp.ExpiresAt,
	}

	return protoResp, nil
}

// Pull retrieves queued messages for the device
// RPC: GET /api/v2/instance/pull
func (s *InstanceService) Pull(ctx context.Context, req *emptypb.Empty) (*v2.PullResponse, error) {
	s.logger.Debug("Pull API called")

	// Extract device_id and tenant_id from context (set by middleware from JWT)
	deviceID := middleware.GetDeviceID(ctx)
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" || deviceID == "" {
		s.logger.Warn("Pull failed: missing tenant_id or device_id in context")
		return nil, kerrors.Unauthorized("invalid_token", "missing tenant_id or device_id in token")
	}

	s.logger.Debug("Pull API device context",
		zap.String("device_id", deviceID),
		zap.String("tenant_id", tenantID),
	)

	// Retrieve queued messages for this device
	messageData, err := s.instanceUc.PullMessages(ctx, deviceID)
	if err != nil {
		s.logger.Error("Failed to pull messages",
			zap.String("device_id", deviceID),
			zap.Error(err),
		)
		return &v2.PullResponse{
			UMHMessages: []*v2.UMHMessage{},
		}, nil
	}

	// Convert interface{} to []*v2.UMHMessage
	messages := []*v2.UMHMessage{}
	if messageData != nil {
		if msgMaps, ok := messageData.([]map[string]interface{}); ok {
			messages = make([]*v2.UMHMessage, len(msgMaps))
			for i, msgMap := range msgMaps {
				msg := &v2.UMHMessage{}
				if email, ok := msgMap["email"].(string); ok {
					msg.Email = email
				}
				if content, ok := msgMap["content"].(string); ok {
					msg.Content = content
				}
				if umhInstance, ok := msgMap["umhInstance"].(string); ok {
					msg.UmhInstance = umhInstance
				}

				// Extract and convert metadata from map to proto type
				if metadataMap, ok := msgMap["metadata"].(map[string]interface{}); ok {
					metadata := &common.MessageMetadata{}

					if traceId, ok := metadataMap["traceId"].(string); ok {
						metadata.TraceId = traceId
						s.logger.Debug("Message metadata extracted",
							zap.String("traceId", traceId),
						)
					}

					msg.Metadata = metadata
				}

				messages[i] = msg
			}
		}
	}

	s.logger.Debug("Pull completed",
		zap.String("device_id", deviceID),
		zap.Int("message_count", len(messages)),
	)

	return &v2.PullResponse{
		UMHMessages: messages,
	}, nil
}

// Push sends device status/telemetry messages
// RPC: POST /api/v2/instance/push
func (s *InstanceService) Push(ctx context.Context, req *v2.PushRequest) (*emptypb.Empty, error) {
	s.logger.Debug("Push API called",
		zap.Int("message_count", len(req.UMHMessages)),
	)

	if req == nil || len(req.UMHMessages) == 0 {
		s.logger.Warn("Push failed: no messages provided")
		return nil, status.Error(codes.InvalidArgument, "messages are required")
	}

	// Extract device_id and tenant_id from context (set by middleware from JWT)
	deviceID := middleware.GetDeviceID(ctx)
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" || deviceID == "" {
		s.logger.Warn("Push failed: missing tenant_id or device_id in context")
		return nil, kerrors.Unauthorized("invalid_token", "missing tenant_id or device_id in token")
	}

	s.logger.Debug("Processing push messages",
		zap.Int("message_count", len(req.UMHMessages)),
		zap.String("device_id", deviceID),
		zap.String("tenant_id", tenantID),
	)

	// Log metadata from incoming messages for tracking
	for i, msg := range req.UMHMessages {
		if msg.Metadata != nil {
			s.logger.Debug("Message metadata received",
				zap.Int("message_index", i),
				zap.String("traceId", msg.Metadata.TraceId),
			)
		}
	}

	// Process push messages
	err := s.instanceUc.PushMessages(ctx, req.UMHMessages, deviceID)
	if err != nil {
		s.logger.Error("Push failed",
			zap.Int("message_count", len(req.UMHMessages)),
			zap.Error(err),
		)
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.logger.Info("Push completed successfully",
		zap.Int("message_count", len(req.UMHMessages)),
	)

	return &emptypb.Empty{}, nil
}

// GetCertificate retrieves device certificate
// RPC: GET /v2/instance/user/certificate?email=<email>
// Device identity comes from JWT token in cookie (extracted by middleware)
// tenant_id and device_id are extracted from context by middleware
// Response matches umh-core's UserCertificateResponse exactly
// (pkg/communicator/api/v2/pull/pull.go:147-151)
func (s *InstanceService) GetCertificate(ctx context.Context, req *v2.GetCertificateRequest) (*v2.Certificate, error) {
	// Extract tenant_id and device_id from context (set by middleware from JWT)
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		s.logger.Warn("GetCertificate failed: missing tenant_id in context")
		return nil, kerrors.Unauthorized("invalid_token", "missing tenant_id in context")
	}

	deviceID := middleware.GetDeviceID(ctx)
	if deviceID == "" {
		s.logger.Warn("GetCertificate failed: missing device_id in context")
		return nil, kerrors.Unauthorized("invalid_token", "missing device_id in context")
	}

	s.logger.Debug("GetCertificate API called",
		zap.String("tenant_id", tenantID),
		zap.String("device_id", deviceID),
		zap.String("email", req.Email),
	)

	if req.Email == "" {
		s.logger.Warn("GetCertificate failed: missing email")
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	cert, err := s.instanceUc.GetCertificate(ctx, deviceID, req.Email)
	if err != nil {
		s.logger.Error("GetCertificate failed",
			zap.String("tenant_id", tenantID),
			zap.String("device_id", deviceID),
			zap.String("email", req.Email),
			zap.Error(err),
		)
		return nil, status.Error(codes.NotFound, err.Error())
	}

	s.logger.Info("Certificate retrieved successfully",
		zap.String("tenant_id", tenantID),
		zap.String("device_id", deviceID),
		zap.String("email", req.Email),
	)

	// Return only userEmail and certificate fields (matching umh-core expectations)
	// Proto fields use camelCase, JSON marshaling will handle correct output
	return &v2.Certificate{
		UserEmail:   req.Email,
		Certificate: cert.Certificate,
	}, nil
}
