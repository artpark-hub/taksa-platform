package service

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/artpark-hub/taksa-platform/device-management/api/common"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/biz"
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
	// Extract token hash from context (set by middleware)
	// Device sends: Authorization: Bearer <token-hash>
	tokenHash, ok := ctx.Value("authorization").(string)
	if !ok || tokenHash == "" {
		s.logger.Warn("Login failed: missing or invalid authorization header")
		return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization header")
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
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	s.logger.Info("Device logged in successfully",
		zap.String("device_id", resp.DeviceId),
	)

	// Map response to match umh-core's InstanceLoginResponse struct EXACTLY
	// File: umh-core/pkg/communicator/backend_api_structs/backend_api_struct.go:119-125
	// NOTE: Company details are no longer managed by device management layer.
	// Device registration and login focus on device identification, location, and status.
	// Company information should be managed separately if needed.

	// Convert certificate pointers (nil-safe)
	var certificate *string
	if resp.Certificate != "" {
		certificate = &resp.Certificate
	}

	var encryptedPrivateKey *string
	if resp.EncryptedPrivateKey != "" {
		encryptedPrivateKey = &resp.EncryptedPrivateKey
	}

	// Send empty CompanyDetails (device management no longer persists company data)
	// The proto requires this field for wire protocol compatibility, but company info
	// is external to device management and should be provided by other services
	companyDetails := &common.CompanyDetails{
		// All fields left empty - company data is not managed here
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
	}

	return protoResp, nil
}

// Pull retrieves queued messages for the device
// RPC: GET /api/v2/instance/pull
func (s *InstanceService) Pull(ctx context.Context, req *emptypb.Empty) (*v2.PullResponse, error) {
	s.logger.Debug("Pull API called")

	// Extract JWT from context (set by middleware)
	jwtDeviceID := ""
	if jwtToken, ok := ctx.Value("jwt_token").(string); ok && jwtToken != "" {
		// s.logger.Debug("JWT token found in context",
		// 	zap.String("token_preview", jwtToken[:min(len(jwtToken), 30)]+"..."),
		// )
		deviceID, err := s.authUc.ExtractDeviceIDFromJWT(jwtToken)
		if err != nil {
			s.logger.Warn("Failed to extract device ID from JWT",
				zap.Error(err),
			)
			return &v2.PullResponse{
				UMHMessages: []*v2.UMHMessage{},
			}, nil
		}
		// s.logger.Debug("Extracted device ID from JWT",
		// 	zap.String("device_id", deviceID),
		// )
		jwtDeviceID = deviceID
	} else {
		s.logger.Warn("No JWT token found in context")
	}

	if jwtDeviceID == "" {
		s.logger.Warn("Pull API: no device ID in JWT")
		return &v2.PullResponse{
			UMHMessages: []*v2.UMHMessage{},
		}, nil
	}

	// Retrieve queued messages for this device
	messageData, err := s.instanceUc.PullMessages(ctx, jwtDeviceID)
	if err != nil {
		s.logger.Error("Failed to pull messages",
			zap.String("device_id", jwtDeviceID),
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
		zap.String("device_id", jwtDeviceID),
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

	// Extract JWT from context (set by middleware) to determine device context
	// This is used as fallback if umhInstance field is not populated in messages
	jwtDeviceID := ""
	if jwtToken, ok := ctx.Value("jwt_token").(string); ok && jwtToken != "" {
		deviceID, err := s.authUc.ExtractDeviceIDFromJWT(jwtToken)
		if err != nil {
			s.logger.Warn("Failed to extract device ID from JWT, will rely on message umhInstance field",
				zap.Error(err),
			)
		} else {
			jwtDeviceID = deviceID
			s.logger.Debug("Extracted device ID from JWT",
				zap.String("device_id", deviceID),
			)
		}
	}

	s.logger.Debug("Processing push messages",
		zap.Int("message_count", len(req.UMHMessages)),
		zap.String("jwt_device_id_fallback", jwtDeviceID),
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

	// Process push messages with JWT device ID as fallback
	err := s.instanceUc.PushMessages(ctx, req.UMHMessages, jwtDeviceID)
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
// Response matches umh-core's UserCertificateResponse exactly
// (pkg/communicator/api/v2/pull/pull.go:147-151)
func (s *InstanceService) GetCertificate(ctx context.Context, req *v2.GetCertificateRequest) (*v2.Certificate, error) {
	// Extract device_id from JWT token (set by ExtractJWTTokenMiddleware)
	jwtToken, ok := ctx.Value("jwt_token").(string)
	if !ok || jwtToken == "" {
		s.logger.Warn("GetCertificate failed: missing JWT token")
		return nil, status.Error(codes.Unauthenticated, "missing JWT token")
	}

	// Decode JWT to get device_id
	deviceID, err := s.authUc.ExtractDeviceIDFromJWT(jwtToken)
	if err != nil {
		s.logger.Warn("GetCertificate failed: invalid JWT token",
			zap.Error(err),
		)
		return nil, status.Error(codes.Unauthenticated, "invalid JWT token")
	}

	s.logger.Debug("GetCertificate API called",
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
			zap.String("device_id", deviceID),
			zap.String("email", req.Email),
			zap.Error(err),
		)
		return nil, status.Error(codes.NotFound, err.Error())
	}

	s.logger.Info("Certificate retrieved successfully",
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
