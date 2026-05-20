package biz

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/data"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// generateUUID generates a proper UUID v4
func generateUUID() string {
	return uuid.New().String()
}

// encodeBase64 encodes a string to base64
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// redactHash returns a safe preview of a hash for logging (first 6 + last 6 chars with ... in between)
// Useful for debugging without exposing the full credential
func redactHash(s string) string {
	if len(s) <= 12 {
		return "***REDACTED***"
	}
	return s[:6] + "..." + s[len(s)-6:]
}

// decompressIfNeeded detects and decompresses gzip or zstd compressed data.
// Returns the decompressed bytes if compression is detected, otherwise returns the original data.
// Supports:
//   - zstd: magic bytes 0x28 0xb5 0x2f 0xfd
//   - gzip: magic bytes 0x1f 0x8b
func decompressIfNeeded(data []byte) ([]byte, error) {
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

// actionFailedFallbackMessage is persisted when umh-core reports action-failure without parseable text.
const actionFailedFallbackMessage = "Action failed on device, but no error text was found in the reply payload. Check umh-core communicator logs on the edge."

// extractActionFailureMessageFromReplyPayload extracts human-readable failure text from a decoded action-reply
// Payload map (same shape for trace_id and action-UUID correlation).
// Canonical order (single place — both correlation paths use this): structured actionReplyPayloadV2.message from
// umh-core SendActionReplyV2, then legacy actionReplyPayload (string or {message}). When V2 exposes a non-empty
// errorCode string, it is appended as " [<code>]" after the message.
func extractActionFailureMessageFromReplyPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}
	if v2, ok := payload["actionReplyPayloadV2"].(map[string]interface{}); ok {
		if m, ok := v2["message"].(string); ok {
			msg := strings.TrimSpace(m)
			if msg != "" {
				code := jsonMapString(v2["errorCode"])
				if code != "" {
					return msg + " [" + code + "]"
				}
				return msg
			}
		}
	}
	if replyPayload, ok := payload["actionReplyPayload"]; ok {
		if s, ok := replyPayload.(string); ok {
			msg := strings.TrimSpace(s)
			if msg != "" {
				return msg
			}
		}
		if replyMap, ok := replyPayload.(map[string]interface{}); ok {
			if m, ok := replyMap["message"].(string); ok {
				msg := strings.TrimSpace(m)
				if msg != "" {
					return msg
				}
			}
		}
	}
	return ""
}

func jsonMapString(v interface{}) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// persistActionErrorMessageIfFailed stores error_message for terminal failure statuses (shared by correlateResponseByTraceId and correlateResponseByActionUUID).
func (uc *InstanceUsecase) persistActionErrorMessageIfFailed(ctx context.Context, tenantID, actionID string, finalStatus models.ActionStatus, errorMessage string) {
	if finalStatus != models.ActionStatusFailed && finalStatus != models.ActionStatusFailedParsingResponse {
		return
	}
	msg := errorMessage
	if msg == "" && finalStatus == models.ActionStatusFailed {
		msg = actionFailedFallbackMessage
	}
	if msg == "" {
		return
	}
	if err := uc.store.Actions().UpdateErrorMessage(ctx, tenantID, actionID, msg); err != nil {
		fmt.Printf("Warning: Failed to store error message: %v\n", err)
	}
}

const (
	// subscribeActionType is a taksa-edge-umh compatible message that registers a "subscriber" so the edge emits periodic status heartbeats.
	// In the taksa-edge-umh fork, status heartbeats are only emitted while at least one subscriber exists.
	subscribeActionType = "subscribe"
	// subscribeActionDefaultTTLSeconds should be <= edge subscriber TTL to avoid stale subscribe actions piling up.
	subscribeActionDefaultTTLSeconds int32 = 120
	subscribeActionPayloadTypeURL          = "type.googleapis.com/taksa.edge.SubscribeMessagePayload"
)

// EnsureStatusSubscription queues a lightweight "subscribe" action for the device (if not already pending).
// This is used so that taksa-edge-umh will emit MessageType "status" heartbeats even when no UI is open.
func (uc *InstanceUsecase) EnsureStatusSubscription(ctx context.Context, deviceID string) {
	if deviceID == "" {
		return
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return
	}

	payloadJSON, err := json.Marshal(map[string]any{
		"resubscribed": true,
	})
	if err != nil {
		return
	}

	now := time.Now()
	action := &models.Action{
		Id:         generateUUID(),
		DeviceId:   deviceID,
		Type:       subscribeActionType,
		Payload:    &anypb.Any{TypeUrl: subscribeActionPayloadTypeURL, Value: payloadJSON},
		MaxRetries: 0,
		RetryCount: 0,
		Status:     models.ActionStatusQueued,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(subscribeActionDefaultTTLSeconds) * time.Second),
	}
	if err := uc.store.Actions().Save(ctx, tenantID, action); err != nil {
		// Ignore "already exists" for idempotency across replicas (db-level uniqueness).
		if err.Error() == "record already exists" {
			return
		}
		// Non-fatal: health still works without summaries.
		fmt.Printf("Warning: failed to queue subscribe action for device %s: %v\n", deviceID, err)
	}
}

// InstanceUsecase handles device-facing operations: authentication, message polling, and status reporting.
type InstanceUsecase struct {
	store                 storage.Store
	authUc                *AuthUsecase
	protocolConverterRepo *data.ProtocolConverterRepo
	dataModelRepo         *data.DataModelRepo
	streamProcessorRepo   *data.StreamProcessorRepo
}

// NewInstanceUsecase creates a new InstanceUsecase with the given storage and authentication backends.
func NewInstanceUsecase(store storage.Store, authUc *AuthUsecase, protocolConverterRepo *data.ProtocolConverterRepo, dataModelRepo *data.DataModelRepo, streamProcessorRepo *data.StreamProcessorRepo) *InstanceUsecase {
	return &InstanceUsecase{
		store:                 store,
		authUc:                authUc,
		protocolConverterRepo: protocolConverterRepo,
		dataModelRepo:         dataModelRepo,
		streamProcessorRepo:   streamProcessorRepo,
	}
}

// Login authenticates a device using a double-hashed token.
// CRITICAL: Used by /v2/instance/login endpoint
//
// Validates the token hash, retrieves device information, and issues a JWT token.
// Device must send: Authorization: Bearer <SHA3(SHA3(token))>
// On successful login, renews the auth token's expiry and includes tenant_id in JWT for multi-tenancy isolation.
//
// DEVICE RETRY PATTERN:
// If Pull returns 401 Unauthenticated (expired/invalid JWT), device should:
//  1. Call Login with stored auth token to refresh JWT
//  2. Store returned JWT in "token" cookie
//  3. Retry the failed Pull request
//
// This handles service restarts and token expiry gracefully.
//
// MULTI-TENANCY FLOW:
//  1. ValidateAuthToken does system-wide search (tokens are cryptographically unique)
//  2. Get device to extract tenant_id from device.CreatedBy
//  3. Inject tenant_id into context
//  4. Pass context to RenewAuthToken and GenerateJWT with tenant isolation
func (uc *InstanceUsecase) Login(ctx context.Context, tokenHash string) (*LoginResponse, error) {

	if tokenHash == "" {
		return nil, fmt.Errorf("token hash is empty")
	}

	// Keep an unscoped context for device reads.
	// Login starts without tenant context; only after validating the auth token (system-wide)
	// do we learn the device_id and tenant_id. Before that, reads must be by device id only.
	ctxUnscoped := ctx

	// Log hash preview (first 6 + last 6 chars) for debugging without exposing full credential
	hashPreview := redactHash(tokenHash)
	fmt.Printf("DEBUG: Login attempt with tokenHash (preview: %s, length: %d)\n", hashPreview, len(tokenHash))

	// ValidateAuthToken resolves device_id and tenant_id from the auth token in one lookup
	deviceID, tenantID, token, err := uc.authUc.ValidateAuthToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Fetch device BEFORE setting tenant context.
	// This ensures we can load device details using the validated device_id even though
	// the request did not begin with tenant context.
	device, err := uc.store.Devices().GetByID(ctxUnscoped, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Now inject tenant_id and device_id into context for downstream operations
	ctx = middleware.SetTenantID(ctx, tenantID)
	ctx = middleware.SetDeviceID(ctx, deviceID)

	// Invariant check: after tenant context is established, the device must be readable
	// with tenant scoping. If not, we treat it as a data-integrity issue.
	if _, err := uc.store.Devices().GetByID(ctx, deviceID); err != nil {
		return nil, fmt.Errorf("device not found for tenant (tenant_id=%s device_id=%s): %w", tenantID, deviceID, err)
	}

	// Record login activity and transition the device to ACTIVE on successful login,
	// unless it is currently SUSPENDED or DECOMMISSIONED.
	now := time.Now()
	if err := uc.store.Devices().UpdateLastLogin(ctx, deviceID, now); err != nil {
		fmt.Printf("ERROR: login: failed to update last_login_at for device %s (tenant_id=%s): %v\n", deviceID, tenantID, err)
	}
	if err := uc.store.Devices().UpdateLastSeen(ctx, deviceID, now); err != nil {
		fmt.Printf("ERROR: login: failed to update last_seen for device %s (tenant_id=%s): %v\n", deviceID, tenantID, err)
	}
	if device.Status != v1.DeviceStatus_SUSPENDED && device.Status != v1.DeviceStatus_DECOMMISSIONED {
		if err := uc.store.Devices().UpdateStatus(ctx, deviceID, v1.DeviceStatus_ACTIVE); err != nil {
			fmt.Printf("ERROR: login: failed to set status ACTIVE for device %s (tenant_id=%s): %v\n", deviceID, tenantID, err)
		}
	}

	// Re-read device row after login updates so later updates don't overwrite
	// status/last_seen/last_login_at with stale values from the pre-login read.
	var refreshedDevice *v1.Device
	if refreshed, err := uc.store.Devices().GetByID(ctxUnscoped, deviceID); err != nil {
		fmt.Printf("ERROR: login: failed to re-read device %s (tenant_id=%s): %v\n", deviceID, tenantID, err)
	} else if refreshed != nil {
		refreshedDevice = refreshed
	}

	// Renew auth token expiry with tenant isolation
	if err := uc.authUc.RenewAuthToken(ctx, tenantID, token); err != nil {
		// Log error but don't fail login - JWT is still valid
		fmt.Printf("ERROR: Failed to renew auth token for device %s: %v\n", deviceID, err)
	} else {
		// Update device's auth token expiry to match the renewed token
		newExpiryTime := time.Now().AddDate(50, 0, 0)
		if err := uc.store.Devices().UpdateAuthTokenExpiresAt(ctx, deviceID, newExpiryTime); err != nil {
			fmt.Printf("ERROR: Failed to update device auth token expiry for device %s: %v\n", deviceID, err)
		}

		// Keep in-memory device consistent for callers of Login.
		// If refresh succeeded, use it; otherwise update the known field only.
		if refreshedDevice != nil {
			device = refreshedDevice
		}
		device.AuthTokenExpiresAt = timestamppb.New(newExpiryTime)
	}

	// GenerateJWT reads tenant_id from context (set above) and includes it in JWT claims
	jwtToken, err := uc.authUc.GenerateJWT(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Queue a subscribe so the edge starts emitting status heartbeats.
	uc.EnsureStatusSubscription(ctx, deviceID)

	return &LoginResponse{
		JwtToken:            jwtToken,
		DeviceId:            device.Id,
		Name:                device.Name,
		Device:              device,
		Certificate:         device.Certificate,
		EncryptedPrivateKey: device.EncryptedPrivateKey,
		ExpiresAt:           timestamppb.New(time.Now().Add(DeviceJWTTTL)),
	}, nil
}

// PullMessages retrieves pending actions converted to UMHMessage format.
// CRITICAL: Used by /v2/instance/pull endpoint
//
// Device polls for queued actions. Each action is converted to a UMHMessage structure
// with umh-core compatible fields:
//   - umhInstance: device ID
//   - content: Base64-encoded JSON with MessageType="action" and Action payload
//   - metadata: ONLY traceId (wire protocol, umh-core compatible)
//   - email: sender email
//
// TRACEABILITY IMPLEMENTATION:
//  1. Generates unique trace_id for each action being sent (identifies this request)
//  2. Stores trace_id in action_message_tracking table, linked to action_id
//  3. Sends trace_id in message.metadata.traceId to device
//  4. When device responds via Push, echo trace_id for correlation
//  5. Creates complete request-response audit trail
//
// Multi-tenancy: tenant_id and device_id obtained from JWT context (via middleware)
// Returns empty slice if no actions pending.
func (uc *InstanceUsecase) PullMessages(ctx context.Context, deviceID string) (interface{}, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is empty")
	}

	// Get tenant_id from context (set by middleware from JWT)
	tenantID := middleware.GetTenantID(ctx)

	actions, err := uc.store.Actions().ListPending(ctx, tenantID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending actions: %w", err)
	}

	_ = uc.store.Devices().UpdateLastSeen(ctx, deviceID, time.Now())

	if len(actions) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Convert each action to UMHMessage format
	messages := make([]map[string]interface{}, 0, len(actions))
	for _, action := range actions {
		// Get JSON content and base64 encode it
		jsonContent, err := actionToJSONContent(action)
		if err != nil {
			// Log error but continue processing other actions
			fmt.Printf("Warning: Failed to convert action %s: %v\n", action.Id, err)
			continue
		}
		base64Content := encodeBase64(jsonContent)

		// Generate unique trace_id for this message (request identifier)
		// This trace_id will be echoed back by device in response for correlation
		traceId := generateUUID()

		// TRACEABILITY: Store trace_id mapping in action_message_tracking
		// This creates the request side of the audit trail
		if err := uc.createActionMessageTracking(ctx, action.Id, deviceID, traceId); err != nil {
			fmt.Printf("Warning: Failed to create message tracking for action %s: %v\n", action.Id, err)
			// Continue even if tracking fails - action still needs to be sent
		}

		// Wire protocol (UMHMessage) - CLEAN and umh-core compatible
		// Only includes metadata.traceId, matching umh-core's MessageMetadata
		messages = append(messages, map[string]interface{}{
			"umhInstance": deviceID,
			"email":       "console@taksa.app",
			"content":     base64Content,
			"metadata": map[string]interface{}{
				"traceId": traceId, // ← Wire protocol: send trace_id, expect it echoed in response
			},
		})

		// Mark action as DELIVERED now that we're returning it to the device
		// This prevents ListPending from returning the same action repeatedly
		// State progression: QUEUED → DELIVERED (on Pull) → COMPLETED (on Push)
		if err := uc.store.Actions().MarkDelivered(ctx, tenantID, action.Id); err != nil {
			fmt.Printf("Warning: Failed to mark action %s as delivered: %v\n", action.Id, err)
			// Continue even if marking fails - action still needs to be sent
			// Error will be logged for troubleshooting but won't block Pull response
		}
	}

	return messages, nil
}

// extractMessageType extracts MessageType field from base64-encoded or plain JSON content
// Handles both uncompressed and compressed (zstd/gzip) messages from umh-core
func extractMessageType(content string) string {
	if content == "" {
		return "unknown"
	}

	// Try to base64 decode first (messages from Pull are base64 encoded)
	decodedContent := content
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil {
		// Successfully decoded - use decoded content
		decodedContent = string(decoded)
	}
	// If decode fails, treat content as plain JSON (for testing or direct pushes)

	// Decompress if needed (zstd or gzip)
	decompressed, err := decompressIfNeeded([]byte(decodedContent))
	if err != nil {
		// Log warning but don't fail - try to parse as-is
		fmt.Printf("Warning: Failed to decompress message content: %v\n", err)
		// Fall back to decodedContent
	} else {
		// Successfully decompressed (or wasn't compressed)
		decodedContent = string(decompressed)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(decodedContent), &data); err != nil {
		return "unknown"
	}

	// Extract MessageType field
	if msgType, ok := data["MessageType"].(string); ok && msgType != "" {
		return msgType
	}

	return "unknown"
}

// extractActionUUID extracts actionUUID from action response payload
// Used to correlate device responses back to original actions
// Handles both uncompressed and compressed (zstd/gzip) messages from umh-core
func extractActionUUID(content string) string {
	if content == "" {
		return ""
	}

	// Try to base64 decode first (messages from Pull are base64 encoded)
	decodedContent := content
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil {
		// Successfully decoded - use decoded content
		decodedContent = string(decoded)
	}

	// Decompress if needed (zstd or gzip)
	decompressed, err := decompressIfNeeded([]byte(decodedContent))
	if err != nil {
		// Log warning but don't fail - try to parse as-is
		fmt.Printf("Warning: Failed to decompress message content: %v\n", err)
		// Fall back to decodedContent
	} else {
		// Successfully decompressed (or wasn't compressed)
		decodedContent = string(decompressed)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(decodedContent), &data); err != nil {
		return ""
	}

	// Extract Payload.actionUUID field
	if payload, ok := data["Payload"].(map[string]interface{}); ok {
		if actionUUID, ok := payload["actionUUID"].(string); ok && actionUUID != "" {
			return actionUUID
		}
	}

	return ""
}

// extractActionReplyState extracts actionReplyState from action response payload
// Terminal states: "action-success", "action-failure"
// Intermediate states: "action-confirmed", "action-executing"
// Returns empty string if not found
// Handles both uncompressed and compressed (zstd/gzip) messages from umh-core
func extractActionReplyState(content string) string {
	if content == "" {
		return ""
	}

	// Try to base64 decode first
	decodedContent := content
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil {
		decodedContent = string(decoded)
	}

	// Decompress if needed (zstd or gzip)
	decompressed, err := decompressIfNeeded([]byte(decodedContent))
	if err != nil {
		// Log warning but don't fail - try to parse as-is
		fmt.Printf("Warning: Failed to decompress message content: %v\n", err)
		// Fall back to decodedContent
	} else {
		// Successfully decompressed (or wasn't compressed)
		decodedContent = string(decompressed)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(decodedContent), &data); err != nil {
		return ""
	}

	// Extract Payload.actionReplyState field
	if payload, ok := data["Payload"].(map[string]interface{}); ok {
		if state, ok := payload["actionReplyState"].(string); ok && state != "" {
			return state
		}
	}

	return ""
}

// isTerminalActionState checks if an action reply state is terminal
// Terminal states mean the action is fully complete
func isTerminalActionState(state string) bool {
	return state == "action-success" || state == "action-failure"
}

// isIntermediateActionState checks if an action reply state is intermediate
// Intermediate states mean the action is still processing
func isIntermediateActionState(state string) bool {
	return state == "action-confirmed" || state == "action-executing"
}

// actionToJSONContent converts an Action to JSON string in umh-core format.
// Expected format: {"MessageType":"action","Payload":{actionType, actionUUID, actionPayload}}
// Returns error if action or payload is nil - action must have valid payload.
func actionToJSONContent(action *models.Action) (string, error) {
	if action == nil {
		return "", fmt.Errorf("action is nil")
	}

	// Parse payload if present - some actions like get-config-file don't need payloads
	var payloadData interface{}
	if action.Payload != nil && action.Payload.Value != nil {
		if err := json.Unmarshal(action.Payload.Value, &payloadData); err != nil {
			return "", fmt.Errorf("failed to unmarshal payload for action %s: %w", action.Id, err)
		}
	}

	// Create the action message structure expected by umh-core
	// Special handling for subscribe actions - they use a different MessageType
	var messageContent map[string]interface{}
	if action.Type == "subscribe" {
		// Subscribe actions are converted to subscribe messages for the router
		messageContent = map[string]interface{}{
			"MessageType": "subscribe",
			"Payload":     payloadData,
		}
	} else {
		// All other actions use the standard action format
		messageContent = map[string]interface{}{
			"MessageType": "action",
			"Payload": map[string]interface{}{
				"actionType":    action.Type,
				"actionUUID":    action.Id,
				"actionPayload": payloadData,
			},
		}
	}

	// Marshal to JSON string (not base64 - we return the plain JSON)
	content, err := json.Marshal(messageContent)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message for action %s: %w", action.Id, err)
	}

	return string(content), nil
}

// PushMessages stores device status/telemetry messages with correlation tracking.
// CRITICAL: Used by /v2/instance/push endpoint
//
// Device sends UMHMessage array containing status updates and responses to actions.
// Messages are persisted for audit/debugging.
//
// TRACEABILITY IMPLEMENTATION:
//  1. Extracts trace_id from metadata.traceId (echo from device)
//  2. Looks up action_message_tracking by trace_id to find original action
//  3. Also extracts actionUUID from content as secondary correlation
//  4. Updates action_message_tracking with response_trace_id and completion time
//  5. Marks action as COMPLETED when response received
//  6. Creates complete request-response audit trail
//
// Expected message structure:
//   - umhInstance: device UUID (optional, identifies sender)
//   - metadata.traceId: Echo of original trace_id (primary correlation)
//   - content: JSON string with MessageType and Payload
//   - Payload.actionUUID: Secondary correlation identifier
//
// resolvePushDeviceID binds push handling to the authenticated device. When jwtDeviceID is set,
// umhInstance must match or be empty; the JWT device id is always used for persistence and sync.
func resolvePushDeviceID(jwtDeviceID, umhInstance string) (string, error) {
	if jwtDeviceID == "" {
		if umhInstance == "" {
			return "", fmt.Errorf("device instance UUID is empty: not found in message umhInstance or JWT token")
		}
		return umhInstance, nil
	}
	if umhInstance != "" && umhInstance != jwtDeviceID {
		return "", fmt.Errorf("umhInstance %q does not match authenticated device %q", umhInstance, jwtDeviceID)
	}
	return jwtDeviceID, nil
}

func (uc *InstanceUsecase) PushMessages(ctx context.Context, messages interface{}, jwtDeviceID string) error {
	if messages == nil {
		return nil
	}

	// Handle both proto messages and map types
	var deviceID string

	// Try proto message type first
	if protoMsgs, ok := messages.([]*v2.UMHMessage); ok {
		if len(protoMsgs) == 0 {
			return nil
		}

		var err error
		deviceID, err = resolvePushDeviceID(jwtDeviceID, protoMsgs[0].UmhInstance)
		if err != nil {
			return err
		}

		// Get tenant_id from context (set by middleware from JWT)
		tenantID := middleware.GetTenantID(ctx)

		_ = uc.store.Devices().UpdateLastSeen(ctx, deviceID, time.Now())

		// Track message types for summary
		messageTypeCount := make(map[string]int)

		// Persist each message and handle correlation
		for _, msg := range protoMsgs {
			// Extract MessageType from content JSON
			msgType := extractMessageType(msg.Content)
			messageTypeCount[msgType]++

			// DEBUG: Log incoming messages
			fmt.Printf("DEBUG: Push received message - Type: %s, Content (first 200 chars): %.200s\n", msgType, msg.Content)

			message := &models.Message{
				ID:        generateUUID(),
				DeviceID:  deviceID,
				Type:      msgType,
				Content:   msg.Content,
				CreatedAt: time.Now(),
			}

			if tenantID == "" {
				fmt.Printf("ERROR: missing tenant ID in context; skipping persistence for message %s on device %s\n", message.ID, deviceID)
				continue
			}
			if err := uc.store.Messages().Save(ctx, tenantID, message); err != nil {
				fmt.Printf("ERROR: failed to persist message %s for tenant %s on device %s: %v\n", message.ID, tenantID, deviceID, err)
				continue
			}

			fmt.Printf("INFO: successfully persisted message %s for tenant %s on device %s\n", message.ID, tenantID, deviceID)

			// TRACEABILITY: Correlate response to original action request
			if msgType == "action-result" || msgType == "action-reply" {
				// Extract action reply state to determine if this is an intermediate or terminal update
				actionReplyState := extractActionReplyState(msg.Content)

				// Only correlate on TERMINAL states (action-success, action-failure)
				// Intermediate states (action-confirmed, action-executing) should NOT mark action complete
				if isTerminalActionState(actionReplyState) {
					// Extract trace_id from response metadata (echo from device)
					// This is PRIMARY correlation - trace_id sent in request, echoed in response
					responseTraceId := ""
					if msg.Metadata != nil {
						responseTraceId = msg.Metadata.TraceId
					}

					// Secondary correlation: Extract actionUUID from payload
					actionUUID := extractActionUUID(msg.Content)

					// Try to correlate using trace_id first (primary method)
					if responseTraceId != "" {
						if err := uc.correlateResponseByTraceId(ctx, deviceID, responseTraceId, message.ID, msg.Content); err != nil {
							fmt.Printf("Warning: Failed to correlate by trace_id %s: %v\n", responseTraceId, err)
							// Fall through to secondary correlation
						} else {
							// Successfully correlated by trace_id - continue to next message
							continue
						}
					}

					// Fallback to secondary correlation using actionUUID
					if actionUUID != "" {
						if err := uc.correlateResponseByActionUUID(ctx, tenantID, actionUUID, message.ID); err != nil {
							fmt.Printf("Note: Failed to correlate by actionUUID %s: %v\n", actionUUID, err)
						}
					}
				} else if isIntermediateActionState(actionReplyState) {
					// For intermediate states, just log for debugging (don't finalize)
					actionUUID := extractActionUUID(msg.Content)
					fmt.Printf("DEBUG: Received intermediate action state '%s' for action %s - not finalizing yet\n", actionReplyState, actionUUID)
				}
			}

			// TASK 2: Sync protocol converters and stream processors from StatusMessage
			// taksa-edge-umh sends MessageType "status" (not "status-message")
			if msgType == "status-message" || msgType == "StatusMessage" || msgType == "status" {
				_ = uc.syncProtocolConvertersFromStatusMessage(ctx, tenantID, deviceID, msg.Content)
				_ = uc.syncStreamProcessorsFromStatusMessage(ctx, tenantID, deviceID, msg.Content)
			}
		}

		// Log summary of message types received in this push batch
		fmt.Printf("DEBUG: Push batch summary - Total messages: %d, Type breakdown: %v\n", len(protoMsgs), messageTypeCount)
		if messageTypeCount["status-message"] == 0 && messageTypeCount["StatusMessage"] == 0 && messageTypeCount["status"] == 0 {
			fmt.Printf("WARNING: No heartbeat status message in push batch. Device must send one of status-message, StatusMessage, or status (1-second heartbeat) to update health_status. Message types: %v\n", messageTypeCount)
		}

		return nil
	}

	// Fallback to map type for testing
	msgSlice, ok := messages.([]map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid messages type: expected []*UMHMessage or []map[string]interface{}")
	}

	if len(msgSlice) == 0 {
		return nil
	}

	umhInstance := ""
	if v, ok := msgSlice[0]["umhInstance"].(string); ok {
		umhInstance = v
	}
	var err error
	deviceID, err = resolvePushDeviceID(jwtDeviceID, umhInstance)
	if err != nil {
		return err
	}

	// Verify device exists
	device, err := uc.store.Devices().GetByID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("failed to get device %s: %w", deviceID, err)
	}
	if device == nil {
		return fmt.Errorf("device %s not found", deviceID)
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("missing tenant ID in context")
	}

	_ = uc.store.Devices().UpdateLastSeen(ctx, deviceID, time.Now())

	// Persist each message
	for _, msg := range msgSlice {
		content := ""
		if c, ok := msg["content"].(string); ok {
			content = c
		}

		// Extract MessageType from content JSON
		msgType := extractMessageType(content)

		// Extract trace_id from metadata for correlation
		traceID := ""
		if metadata, ok := msg["metadata"].(map[string]interface{}); ok {
			if traceId, ok := metadata["traceId"].(string); ok {
				traceID = traceId
			}
		}

		message := &models.Message{
			ID:        generateUUID(),
			DeviceID:  deviceID,
			Type:      msgType,
			Content:   content,
			TraceID:   traceID,
			CreatedAt: time.Now(),
		}

		if err := uc.store.Messages().Save(ctx, tenantID, message); err != nil {
			return fmt.Errorf("failed to persist message %s for tenant %s on device %s: %w", message.ID, tenantID, deviceID, err)
		}

		// TRACEABILITY: Correlate response to original action request
		if msgType == "action-result" || msgType == "action-reply" {
			// Extract action reply state to determine if this is an intermediate or terminal update
			actionReplyState := extractActionReplyState(content)
			fmt.Printf("DEBUG: Extracted actionReplyState: '%s' for message type: %s\n", actionReplyState, msgType)

			// Only correlate on TERMINAL states (action-success, action-failure)
			// Intermediate states (action-confirmed, action-executing) should NOT mark action complete
			if isTerminalActionState(actionReplyState) {
				// Extract trace_id from response metadata (echo from device)
				responseTraceId := ""
				if metadata, ok := msg["metadata"].(map[string]interface{}); ok {
					if traceId, ok := metadata["traceId"].(string); ok {
						responseTraceId = traceId
					}
				}

				// Secondary correlation: Extract actionUUID from payload
				actionUUID := extractActionUUID(content)

				// Try to correlate using trace_id first (primary method)
				if responseTraceId != "" {
					if err := uc.correlateResponseByTraceId(ctx, deviceID, responseTraceId, message.ID, content); err != nil {
						fmt.Printf("Warning: Failed to correlate by trace_id %s: %v\n", responseTraceId, err)
						// Fall through to secondary correlation
					} else {
						// Successfully correlated by trace_id - continue to next message
						continue
					}
				}

				// Fallback to secondary correlation using actionUUID
				if actionUUID != "" {
					if err := uc.correlateResponseByActionUUID(ctx, tenantID, actionUUID, message.ID); err != nil {
						fmt.Printf("Note: Failed to correlate by actionUUID %s: %v\n", actionUUID, err)
					}
				}
			} else if isIntermediateActionState(actionReplyState) {
				// For intermediate states, just log for debugging (don't finalize)
				actionUUID := extractActionUUID(content)
				fmt.Printf("DEBUG: Received intermediate action state '%s' for action %s - not finalizing yet\n", actionReplyState, actionUUID)
			}
		}
	}

	return nil
}

// GetCertificate retrieves the device certificate for secure communication.
// Used by /v2/instance/user/certificate endpoint
//
// If email is provided, returns the certificate for that email; otherwise returns
// the device's default certificate.
func (uc *InstanceUsecase) GetCertificate(ctx context.Context, deviceID, email string) (*v2.Certificate, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is empty")
	}

	if email != "" {
		cert, err := uc.store.Certificates().GetByDeviceAndEmail(ctx, deviceID, email)
		if err != nil {
			return nil, fmt.Errorf("certificate not found: %w", err)
		}
		return cert, nil
	}

	cert, err := uc.store.Certificates().GetByDevice(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device certificate not found: %w", err)
	}

	return cert, nil
}

// LoginResponse contains the successful login response data.
type LoginResponse struct {
	JwtToken            string
	DeviceId            string
	Name                string
	Device              *v1.Device
	Certificate         string
	EncryptedPrivateKey string
	ExpiresAt           *timestamppb.Timestamp
}

// RegisterDevice creates a new device and generates its initial auth token.
// Deprecated: Use DeviceUsecase.RegisterDevice instead.
func (uc *InstanceUsecase) RegisterDevice(ctx context.Context, req *RegisterDeviceRequest) (*RegisterDeviceResponse, error) {
	return nil, fmt.Errorf("use DeviceUsecase.RegisterDevice")
}

// createActionMessageTracking creates a tracking record for an action being sent to device.
// This is the REQUEST side of the traceability flow.
// Links trace_id to action_id for later correlation when device responds.
func (uc *InstanceUsecase) createActionMessageTracking(ctx context.Context, actionID, deviceID, traceId string) error {
	if actionID == "" || deviceID == "" || traceId == "" {
		return fmt.Errorf("missing required fields for tracking: action_id=%s, device_id=%s, trace_id=%s", actionID, deviceID, traceId)
	}

	track := &storage.ActionMessageTracking{
		ID:                generateUUID(),
		ActionID:          actionID,
		DeviceID:          deviceID,
		TraceID:           traceId,
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 1, // PENDING
		CreatedAt:         time.Now(),
	}

	// Store tracking record
	if err := uc.store.ActionMessageTracking().Create(ctx, track); err != nil {
		return fmt.Errorf("failed to create action message tracking: %w", err)
	}

	return nil
}

// correlateResponseByTraceId correlates an incoming response to the original action
// using the trace_id echo (primary correlation method).
// Updates action_message_tracking with response_trace_id and marks action as COMPLETED.
// resultPayload is the response content from device (used for sync operations like protocol converters)
func (uc *InstanceUsecase) correlateResponseByTraceId(ctx context.Context, deviceID, responseTraceId, messageID, resultPayload string) error {
	fmt.Printf("DEBUG: correlateResponseByTraceId called - TraceID: %s, ResultPayload (first 200 chars): %.200s\n", responseTraceId, resultPayload)

	if responseTraceId == "" {
		return fmt.Errorf("response trace_id is empty")
	}

	// 1. Find tracking record by trace_id
	track, err := uc.store.ActionMessageTracking().GetByTraceID(ctx, responseTraceId)
	if err != nil {
		return fmt.Errorf("failed to lookup tracking by trace_id: %w", err)
	}
	if track == nil {
		return fmt.Errorf("no tracking found for trace_id: %s", responseTraceId)
	}

	// 2. Verify device match (security check)
	if track.DeviceID != deviceID {
		return fmt.Errorf("device mismatch: tracking device %s vs request device %s", track.DeviceID, deviceID)
	}

	// 2b. Verify device exists (GetByID applies tenant scoping when JWT tenant is present)
	if _, err := uc.store.Devices().GetByID(ctx, deviceID); err != nil {
		return fmt.Errorf("device not found: %w", err)
	}
	tenantID := middleware.GetTenantID(ctx)

	// 3. Get the action to check its type and payload (for protocol converter sync)
	action, err := uc.store.Actions().GetByID(ctx, tenantID, track.ActionID)
	if err != nil || action == nil {
		fmt.Printf("Warning: Could not retrieve action %s for protocol converter sync: %v\n", track.ActionID, err)
		// Continue anyway - action can still be marked complete
	}

	// 4. Update tracking with response info (store message_id for O(1) lookup)
	if err := uc.store.ActionMessageTracking().UpdateResponseWithMessageID(ctx, track.ID, messageID, 3); err != nil {
		fmt.Printf("Warning: Failed to store message_id in tracking: %v\n", err)
		// Don't fail - action still marked complete
	}

	// 5. Determine action final status based on actionReplyState
	finalStatus := models.ActionStatusCompleted
	errorMessage := ""
	stateFound := false

	// Parse the result payload to extract actionReplyState and error info
	// First, handle potential base64 encoding and compression
	decodedPayload := resultPayload
	if decoded, err := base64.StdEncoding.DecodeString(resultPayload); err == nil {
		decodedPayload = string(decoded)
	}

	// Decompress if needed
	if decompressed, err := decompressIfNeeded([]byte(decodedPayload)); err == nil {
		decodedPayload = string(decompressed)
	}

	var payloadData map[string]interface{}
	if err := json.Unmarshal([]byte(decodedPayload), &payloadData); err != nil {
		// Failed to parse, can't determine if success or failure
	} else {
		if payload, ok := payloadData["Payload"].(map[string]interface{}); ok {
			// Check actionReplyState
			if state, ok := payload["actionReplyState"].(string); ok {
				stateFound = true
				if state == "action-failure" {
					finalStatus = models.ActionStatusFailed
					errorMessage = extractActionFailureMessageFromReplyPayload(payload)
				} else if state == "action-success" {
					finalStatus = models.ActionStatusCompleted
				}
			}
		}
	}

	// If we couldn't extract the state, mark as failed parsing
	if !stateFound {
		finalStatus = models.ActionStatusFailedParsingResponse
		errorMessage = "Failed to parse action response from device"
		fmt.Printf("Warning: Could not extract actionReplyState for action %s, payload (first 500 chars): %.500s\n", track.ActionID, decodedPayload)
	}

	// Update action status based on actionReplyState
	if err := uc.store.Actions().UpdateStatus(ctx, tenantID, track.ActionID, finalStatus); err != nil {
		return fmt.Errorf("failed to update action status: %w", err)
	}

	uc.persistActionErrorMessageIfFailed(ctx, tenantID, track.ActionID, finalStatus, errorMessage)

	// 6. Sync protocol converter and data model state if action completed
	// Use RESULT payload from device (resultPayload), not the REQUEST payload (action.Payload)
	// NOTE: Use finalStatus (computed above), not action.Status (stale in-memory object)
	// NOTE: Call sync even for delete operations which may have empty resultPayload
	if action != nil && finalStatus == models.ActionStatusCompleted {
		// Create a copy of action with updated status for sync
		actionWithStatus := *action
		actionWithStatus.Status = finalStatus

		// For non-delete operations, only sync if we have result payload
		// For delete operations, sync even with empty payload (empty payload means successful delete)
		shouldSync := len(resultPayload) > 0 || action.Type == "delete-protocol-converter" || action.Type == "delete-datamodel" || action.Type == "delete-stream-processor"

		if shouldSync {
			_ = uc.syncProtocolConverterActionResult(ctx, &actionWithStatus, []byte(resultPayload))
			_ = uc.syncDataModelActionResult(ctx, &actionWithStatus, []byte(resultPayload))
			_ = uc.syncStreamProcessorActionResult(ctx, &actionWithStatus, []byte(resultPayload))
		}
	}

	// 7. Mark tracking as completed
	if err := uc.store.ActionMessageTracking().UpdateCompleted(ctx, track.ID); err != nil {
		fmt.Printf("Warning: Failed to mark tracking completed: %v\n", err)
		// Don't fail if this step fails - action is already marked complete
	}

	return nil
}

// correlateResponseByActionUUID correlates an incoming response to the original action
// using the actionUUID from payload (secondary/fallback correlation method).
// Also updates the action_message_tracking table to record the correlation with the response message ID.
// Marks action as COMPLETED.
func (uc *InstanceUsecase) correlateResponseByActionUUID(ctx context.Context, tenantID, actionUUID string, messageID string) error {
	// Look up action by UUID
	action, err := uc.store.Actions().GetByID(ctx, tenantID, actionUUID)
	if err != nil {
		return fmt.Errorf("action not found: %w", err)
	}
	if action == nil {
		return fmt.Errorf("action with UUID %s not found", actionUUID)
	}

	// TRACEABILITY: Update tracking record to reflect response correlation
	// Find the tracking record for this action
	track, err := uc.store.ActionMessageTracking().GetByActionID(ctx, action.Id)
	if err != nil {
		fmt.Printf("Warning: Failed to find tracking record for action %s: %v\n", action.Id, err)
		// Continue even if tracking fails - action still needs to be marked completed
	} else if track != nil {
		// Store the response message ID for O(1) lookup in GetActionResultPayload
		// This is the KEY FIX: instead of searching 100+ messages, we have a direct pointer
		if err := uc.store.ActionMessageTracking().UpdateResponseWithMessageID(ctx, track.ID, messageID, 3); err != nil {
			fmt.Printf("Warning: Failed to update tracking with message_id for action %s: %v\n", action.Id, err)
			// Continue even if tracking update fails
		}
	}

	// Get the message content to extract actionReplyState and error info
	deviceMsg, err := uc.store.Messages().GetByID(ctx, messageID)
	finalStatus := models.ActionStatusCompleted
	errorMessage := ""
	stateFound := false

	if err == nil && deviceMsg != nil {
		// Parse message to extract state and error details
		decodedPayload := deviceMsg.Content
		if decoded, err := base64.StdEncoding.DecodeString(deviceMsg.Content); err == nil {
			decodedPayload = string(decoded)
		}
		if decompressed, err := decompressIfNeeded([]byte(decodedPayload)); err == nil {
			decodedPayload = string(decompressed)
		}

		var payloadData map[string]interface{}
		if err := json.Unmarshal([]byte(decodedPayload), &payloadData); err == nil {
			if payload, ok := payloadData["Payload"].(map[string]interface{}); ok {
				if state, ok := payload["actionReplyState"].(string); ok {
					stateFound = true
					if state == "action-failure" {
						finalStatus = models.ActionStatusFailed
						errorMessage = extractActionFailureMessageFromReplyPayload(payload)
					} else if state == "action-success" {
						finalStatus = models.ActionStatusCompleted
					}
				}
			}
		}
	}

	// If we couldn't extract the state, mark as failed parsing
	if !stateFound {
		finalStatus = models.ActionStatusFailedParsingResponse
		errorMessage = "Failed to parse action response from device"
		fmt.Printf("Warning: Could not extract actionReplyState for action %s via secondary correlation (actionUUID: %s)\n", action.Id, actionUUID)
	}

	// Mark action with appropriate status
	if err := uc.store.Actions().UpdateStatus(ctx, tenantID, action.Id, finalStatus); err != nil {
		return fmt.Errorf("failed to update action status: %w", err)
	}

	uc.persistActionErrorMessageIfFailed(ctx, tenantID, action.Id, finalStatus, errorMessage)

	// 6. Sync state if this is a protocol converter or data model action
	// This mirrors the logic in correlateResponseByTraceId to ensure DB persistence
	// regardless of which correlation method (trace_id vs actionUUID) is used
	if finalStatus == models.ActionStatusCompleted && action != nil {
		syncMsg := deviceMsg
		if syncMsg == nil {
			syncMsg, err = uc.store.Messages().GetByID(ctx, messageID)
			if err != nil {
				syncMsg = nil
			}
		}
		if syncMsg != nil {
			// Create a copy of action with updated status for sync
			actionWithStatus := *action
			actionWithStatus.Status = finalStatus
			_ = uc.syncProtocolConverterActionResult(ctx, &actionWithStatus, []byte(syncMsg.Content))
			_ = uc.syncDataModelActionResult(ctx, &actionWithStatus, []byte(syncMsg.Content))
			_ = uc.syncStreamProcessorActionResult(ctx, &actionWithStatus, []byte(syncMsg.Content))
		}
	}

	return nil
}

// GetActionResultPayload retrieves the device response payload for a completed action
// Uses three strategies in priority order:
// 1. Direct message ID lookup (O(1)) via action_message_tracking.response_message_id
// 2. Fallback: Linear search through recent messages for actionUUID
// Returns JSON bytes of the actionReplyPayload, or nil if not found/not completed
func (uc *InstanceUsecase) GetActionResultPayload(ctx context.Context, tenantID, actionID string) ([]byte, error) {
	// 1. Get the action to verify it exists
	action, err := uc.store.Actions().GetByID(ctx, tenantID, actionID)
	if err != nil {
		return nil, fmt.Errorf("action not found: %w", err)
	}
	if action == nil {
		return nil, fmt.Errorf("action with ID %s not found", actionID)
	}

	// 2. STRATEGY 1: Use response_message_id from action_message_tracking (PREFERRED - O(1))
	// This is the direct pointer to the response message
	track, err := uc.store.ActionMessageTracking().GetByActionID(ctx, actionID)
	if err == nil && track != nil && track.ResponseMessageID != "" {
		// We have a message ID - fetch it directly!
		msg, err := uc.store.Messages().GetByID(ctx, track.ResponseMessageID)
		if err == nil && msg != nil {
			// Extract and return the payload from this message
			payload, err := extractPayloadFromMessage(msg.Content, actionID)
			if err == nil && payload != nil {
				return payload, nil
			}
			// Log but fall through to fallback
			fmt.Printf("Warning: Failed to extract payload from cached message %s: %v\n", track.ResponseMessageID, err)
		}
	}

	// 3. STRATEGY 2 (FALLBACK): Linear search through recent messages
	// This is the old approach - slower but works when tracking wasn't recorded
	deviceID := action.DeviceId
	if deviceID == "" {
		return nil, fmt.Errorf("action has no device ID")
	}

	messages, err := uc.store.Messages().GetRecentByDevice(ctx, deviceID, 500) // Search more messages as fallback
	if err != nil || len(messages) == 0 {
		return nil, fmt.Errorf("no messages found for device: %w", err)
	}

	// 4. Find the message that contains this action's response by actionUUID
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}

		// Decode the base64 content
		decodedContent, err := base64.StdEncoding.DecodeString(msg.Content)
		if err != nil {
			continue // Skip if can't decode
		}

		// Parse the JSON and check if actionUUID matches
		var msgData map[string]interface{}
		if err := json.Unmarshal(decodedContent, &msgData); err != nil {
			continue // Skip if invalid JSON
		}

		// Check if this message is for our action
		if payload, ok := msgData["Payload"].(map[string]interface{}); ok {
			if msgActionUUID, ok := payload["actionUUID"].(string); ok && msgActionUUID == actionID {
				// Found our action's response!
				if replyPayload, ok := payload["actionReplyPayload"]; ok {
					// Convert to JSON bytes for proto unmarshaling
					payloadBytes, err := json.Marshal(replyPayload)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal payload: %w", err)
					}
					return payloadBytes, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no response payload found for action %s", actionID)
}

// extractPayloadFromMessage extracts actionReplyPayload from message content
// Helper for GetActionResultPayload
// Handles both uncompressed and compressed (zstd/gzip) messages from umh-core
func extractPayloadFromMessage(content string, actionID string) ([]byte, error) {
	if content == "" {
		return nil, fmt.Errorf("message content is empty")
	}

	// Decode the base64 content
	decodedContent, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Decompress if needed (zstd or gzip)
	decompressed, err := decompressIfNeeded(decodedContent)
	if err != nil {
		// Log warning but don't fail - try to parse as-is
		fmt.Printf("Warning: Failed to decompress message content: %v\n", err)
		// Fall back to decodedContent
		decompressed = decodedContent
	}

	// Parse the JSON
	var msgData map[string]interface{}
	if err := json.Unmarshal(decompressed, &msgData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Extract the payload
	if payload, ok := msgData["Payload"].(map[string]interface{}); ok {
		if msgActionUUID, ok := payload["actionUUID"].(string); ok && msgActionUUID == actionID {
			if replyPayload, ok := payload["actionReplyPayload"]; ok {
				// Convert to JSON bytes
				payloadBytes, err := json.Marshal(replyPayload)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal payload: %w", err)
				}
				return payloadBytes, nil
			}
		}
	}

	return nil, fmt.Errorf("no actionReplyPayload found in message for action %s", actionID)
}

// syncProtocolConverterActionResult syncs protocol converter state from action result
// Called when an action-result is received for protocol converter operations
func (uc *InstanceUsecase) syncProtocolConverterActionResult(ctx context.Context, action *models.Action, payload []byte) error {
	if uc.protocolConverterRepo == nil {
		// Skip if repo is not available
		return nil
	}

	// Extract tenant_id from context (set by middleware)
	tenantID := middleware.GetTenantID(ctx)

	// DEBUG: Log incoming payload
	fmt.Printf("DEBUG: syncProtocolConverterActionResult - ActionType: %s, Payload (first 500 chars): %.500s\n", action.Type, string(payload))

	// Determine action type and corresponding operation
	var actionType string
	switch action.Type {
	case "deploy-protocol-converter":
		actionType = "deploy"
	case "edit-protocol-converter":
		actionType = "edit"
	case "delete-protocol-converter":
		actionType = "delete"
	case "get-protocol-converter":
		actionType = "get"
	default:
		// Not a protocol converter action
		return nil
	}

	// Parse the payload to extract protocol converter info
	// First, decode Base64 if needed
	decodedPayload := payload
	if decoded, err := base64.StdEncoding.DecodeString(string(payload)); err == nil {
		decodedPayload = decoded
	}

	// Decompress if needed
	if decompressed, err := decompressIfNeeded(decodedPayload); err == nil {
		decodedPayload = decompressed
	}

	var responsePayload map[string]interface{}
	if err := json.Unmarshal(decodedPayload, &responsePayload); err != nil {
		fmt.Printf("Warning: Failed to unmarshal protocol converter action payload: %v\n", err)
		fmt.Printf("Payload (first 200 chars): %.200s\n", string(decodedPayload))
		return nil
	}

	// Extract result from payload - check nested Payload.actionReplyPayload structure
	var protocolConverter map[string]interface{}

	// Try extracting from Payload.actionReplyPayload (umh-core format)
	if payload, ok := responsePayload["Payload"].(map[string]interface{}); ok {
		if actionReplyPayload, ok := payload["actionReplyPayload"].(map[string]interface{}); ok {
			protocolConverter = actionReplyPayload
		}
	}

	// Fallback: Try result field
	if len(protocolConverter) == 0 {
		if result, ok := responsePayload["result"].(map[string]interface{}); ok {
			protocolConverter = result
		}
	}

	// Fallback: Try protocolConverter field
	if len(protocolConverter) == 0 {
		if converter, ok := responsePayload["protocolConverter"].(map[string]interface{}); ok {
			protocolConverter = converter
		}
	}

	if len(protocolConverter) == 0 {
		// For delete operations, there might be no result - extract UUID from original payload
		if actionType == "delete" {
			if action.Payload != nil {
				var origPayload map[string]interface{}
				if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
					if uuid, ok := origPayload["uuid"].(string); ok && uuid != "" {
						// If action succeeded, delete from DB
						if action.Status == models.ActionStatusCompleted {
							_ = uc.protocolConverterRepo.Delete(ctx, tenantID, action.DeviceId, uuid)
						} else if action.Status == models.ActionStatusFailed {
							// If action failed, mark as OFFLINE but keep record
							_ = uc.protocolConverterRepo.UpdateStatus(ctx, tenantID, action.DeviceId, uuid,
								"FAILED", "OFFLINE", "Failed to delete protocol converter")
						}
						return nil
					}
				}
			}
		}
		return nil
	}

	// Extract UUID from result
	uuid, ok := protocolConverter["uuid"].(string)
	if !ok || uuid == "" {
		// For deploy: UUID is generated from name (same as umh-core does)
		// UUID generation is deterministic: GenerateUUIDFromName(name) == same UUID every time for same name
		if actionType == "deploy" {
			if name, ok := protocolConverter["name"].(string); ok && name != "" {
				// Import would be needed: "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
				// For now, try to get from request payload
				if action.Payload != nil {
					var origPayload map[string]interface{}
					if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
						if name, ok := origPayload["name"].(string); ok && name != "" {
							// Use the name to look up the converter, or generate UUID from name
							// For deterministic UUID generation: would need to implement same logic as umh-core
							// Fallback: use name as temporary identifier
							uuid = name // Temporary - ideally generate proper UUID from name
						}
					}
				}
			}
		}

		// For delete with empty result, try to get UUID from original payload
		if actionType == "delete" && len(protocolConverter) == 0 {
			if action.Payload != nil {
				var origPayload map[string]interface{}
				if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
					if id, ok := origPayload["uuid"].(string); ok && id != "" {
						uuid = id
					}
				}
			}
		}
		if uuid == "" {
			return nil
		}
	}

	// Handle based on action type and status
	if action.Status == models.ActionStatusCompleted {
		switch actionType {
		case "deploy":
			// For deploy: UPSERT to create new record
			name, _ := protocolConverter["name"].(string)
			converterType := "protocol-converter"

			// Try to extract protocol type from response meta first
			if meta, ok := protocolConverter["meta"].(map[string]interface{}); ok {
				if protocol, ok := meta["protocol"].(string); ok && protocol != "" {
					converterType = protocol
				}
			}

			// If not in response, extract from request payload
			if converterType == "protocol-converter" && action.Payload != nil {
				var requestPayload map[string]interface{}
				if err := json.Unmarshal(action.Payload.Value, &requestPayload); err == nil {
					if readDFC, ok := requestPayload["readDFC"].(map[string]interface{}); ok {
						if inputs, ok := readDFC["inputs"].(map[string]interface{}); ok {
							if inputType, ok := inputs["type"].(string); ok && inputType != "" {
								converterType = inputType
							}
						}
					}
				}
			}

			connectionUUID := "" // Extract from connection if available
			if conn, ok := protocolConverter["connection"].(map[string]interface{}); ok {
				if uuid, ok := conn["uuid"].(string); ok && uuid != "" {
					connectionUUID = uuid
				}
			}

			err := uc.protocolConverterRepo.Upsert(ctx, tenantID, action.DeviceId, uuid, name, converterType, connectionUUID)
			if err != nil {
				fmt.Printf("Warning: Failed to upsert protocol converter: %v\n", err)
			}

		case "edit":
			// For edit: Extract old UUID from request, compare with new UUID from response
			var editRequest map[string]interface{}
			if action.Payload != nil && action.Payload.Value != nil {
				if err := json.Unmarshal(action.Payload.Value, &editRequest); err != nil {
					fmt.Printf("Warning: Failed to unmarshal edit request payload: %v\n", err)
					editRequest = make(map[string]interface{})
				}
			}

			oldUUID, _ := editRequest["uuid"].(string)
			newName, _ := editRequest["name"].(string)

			// If UUID changed (name changed), delete old and create new
			if oldUUID != "" && oldUUID != uuid {
				// Delete old record
				_ = uc.protocolConverterRepo.Delete(ctx, tenantID, action.DeviceId, oldUUID)

				// Create new record with new UUID and updated details
				converterType := "protocol-converter"
				connectionUUID := ""

				// Try to extract protocol type from response meta first
				if meta, ok := protocolConverter["meta"].(map[string]interface{}); ok {
					if protocol, ok := meta["protocol"].(string); ok && protocol != "" {
						converterType = protocol
					}
				}

				// If not in response, extract from request payload
				if converterType == "protocol-converter" && action.Payload != nil {
					var requestPayload map[string]interface{}
					if err := json.Unmarshal(action.Payload.Value, &requestPayload); err == nil {
						if readDFC, ok := requestPayload["readDFC"].(map[string]interface{}); ok {
							if inputs, ok := readDFC["inputs"].(map[string]interface{}); ok {
								if inputType, ok := inputs["type"].(string); ok && inputType != "" {
									converterType = inputType
								}
							}
						}
					}
				}

				// Extract connection UUID if available
				if conn, ok := protocolConverter["connection"].(map[string]interface{}); ok {
					if connUUID, ok := conn["uuid"].(string); ok && connUUID != "" {
						connectionUUID = connUUID
					}
				}

				// Upsert with new UUID
				if err := uc.protocolConverterRepo.Upsert(ctx, tenantID, action.DeviceId, uuid, newName, converterType, connectionUUID); err != nil {
					fmt.Printf("Warning: Failed to upsert renamed protocol converter: %v\n", err)
				}
			} else {
				// UUID unchanged (name unchanged) - just update status and details
				err := uc.protocolConverterRepo.UpdateStatus(ctx,
					tenantID,
					action.DeviceId,
					uuid,
					"ACTIVE", // deployment_status
					"ONLINE", // health_status
					"",       // error_message (clear any previous errors)
				)
				if err != nil {
					fmt.Printf("Warning: Failed to update protocol converter status: %v\n", err)
				}
			}

		case "delete":
			// For delete: REMOVE from database
			err := uc.protocolConverterRepo.Delete(ctx, tenantID, action.DeviceId, uuid)
			if err != nil {
				fmt.Printf("Warning: Failed to delete protocol converter: %v\n", err)
			}
		}
	} else if action.Status == models.ActionStatusFailed {
		// Failed action: mark as FAILED/OFFLINE
		errorMessage := ""
		if errMsg, ok := responsePayload["error"].(string); ok {
			errorMessage = errMsg
		} else if errMsg, ok := responsePayload["errorMessage"].(string); ok {
			errorMessage = errMsg
		}

		if actionType == "delete" {
			// Delete failure: mark as OFFLINE but keep record
			err := uc.protocolConverterRepo.UpdateStatus(ctx, tenantID, action.DeviceId, uuid,
				"FAILED", "OFFLINE", errorMessage)
			if err != nil {
				fmt.Printf("Warning: Failed to update protocol converter status on delete failure: %v\n", err)
			}
		} else {
			// Deploy/Edit failure: mark as FAILED
			err := uc.protocolConverterRepo.UpdateStatus(ctx, tenantID, action.DeviceId, uuid,
				"FAILED", "OFFLINE", errorMessage)
			if err != nil {
				fmt.Printf("Warning: Failed to update protocol converter status on failure: %v\n", err)
			}
		}
	}

	// Mark as synced (for all successful operations)
	if action.Status == models.ActionStatusCompleted {
		_ = uc.protocolConverterRepo.MarkSynced(ctx, tenantID, action.DeviceId, uuid)
	}

	return nil
}

// syncDataModelActionResult syncs data model state from action result
// Called when an action-result is received for data model operations (add, edit, delete, get)
// Extracts version from umh-core action response and updates local database
func (uc *InstanceUsecase) syncDataModelActionResult(ctx context.Context, action *models.Action, payload []byte) error {
	if uc.dataModelRepo == nil {
		return nil
	}

	tenantID := middleware.GetTenantID(ctx)

	// Determine action type and corresponding operation
	var actionType string
	switch action.Type {
	case "add-datamodel":
		actionType = "add"
	case "edit-datamodel":
		actionType = "edit"
	case "delete-datamodel":
		actionType = "delete"
	case "get-datamodel":
		actionType = "get"
	default:
		// Not a data model action
		return nil
	}

	// Parse the payload to extract data model info
	decodedPayload := payload
	if decoded, err := base64.StdEncoding.DecodeString(string(payload)); err == nil {
		decodedPayload = decoded
	}

	if decompressed, err := decompressIfNeeded(decodedPayload); err == nil {
		decodedPayload = decompressed
	}

	var responsePayload map[string]interface{}
	if err := json.Unmarshal(decodedPayload, &responsePayload); err != nil {
		return nil
	}

	// Extract result from payload
	var dataModel map[string]interface{}

	// Try extracting from Payload.actionReplyPayload (umh-core format)
	if payload, ok := responsePayload["Payload"].(map[string]interface{}); ok {
		if actionReplyPayload, ok := payload["actionReplyPayload"].(map[string]interface{}); ok {
			dataModel = actionReplyPayload
		}
	}

	// Fallback: Try result field
	if len(dataModel) == 0 {
		if result, ok := responsePayload["result"].(map[string]interface{}); ok {
			dataModel = result
		}
	}

	// Handle delete operations - umh-core returns {deleted: true, name: "ModelName"}
	if actionType == "delete" {
		// Only proceed if umh-core explicitly confirms deletion
		deleted, _ := dataModel["deleted"].(bool)
		if !deleted {
			// If deleted flag is false or missing, don't remove from DB
			return nil
		}

		// Try to get name from response payload first
		var nameToDelete string
		if name, ok := dataModel["name"].(string); ok && name != "" {
			nameToDelete = name
		}

		// If not in response, extract from original request payload
		if nameToDelete == "" && action.Payload != nil {
			var origPayload map[string]interface{}
			if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
				if name, ok := origPayload["name"].(string); ok && name != "" {
					nameToDelete = name
				}
			}
		}

		// If action succeeded and deletion confirmed, delete from DB
		if nameToDelete != "" && action.Status == models.ActionStatusCompleted {
			_ = uc.dataModelRepo.DeleteByName(ctx, tenantID, action.DeviceId, nameToDelete)
		}
		return nil
	}

	if len(dataModel) == 0 {
		// No data model returned and not a delete operation
		return nil
	}

	// Extract required fields
	// NOTE: umh-core returns these fields in the response:
	// - name: string
	// - description: string
	// - version: integer (1, 2, 3, ...)
	// - structure: map[string]interface{} (the parsed YAML structure)
	// NOT encodedStructure - that was the input, umh-core parses it and returns the structure

	name, _ := dataModel["name"].(string)
	description, _ := dataModel["description"].(string)

	// Version comes back as interface{} - could be float64 (JSON default for numbers) or string
	var version string
	if versionVal, ok := dataModel["version"]; ok {
		switch v := versionVal.(type) {
		case float64:
			version = fmt.Sprintf("%.0f", v) // "1", "2", etc
		case int:
			version = fmt.Sprintf("%d", v)
		case string:
			version = v
		}
	}

	if name == "" {
		// Skip if name is missing
		fmt.Printf("WARNING: Data model action (type=%s) missing name field. dataModel=%+v\n", actionType, dataModel)
		return nil
	}

	// Skip upsert if version is missing - this prevents creating records with empty version strings
	// Both Add and Edit operations should return a version from umh-core
	if version == "" {
		fmt.Printf("WARNING: Data model action (type=%s) missing version field for name=%s. dataModel=%+v\n", actionType, name, dataModel)
		return nil
	}

	// For successful operations, upsert the data model into DB
	// NOTE: encodedStructure is empty because umh-core returns the parsed "structure" map, not the encoded string
	// The dataModelRepo will store this empty encodedStructure, which is okay since we have the structure from StatusMessage
	if action.Status == models.ActionStatusCompleted {
		err := uc.dataModelRepo.Upsert(ctx, tenantID, action.DeviceId, name, version, description, "")
		if err != nil {
			fmt.Printf("Warning: Failed to upsert data model: %v\n", err)
		}
	}

	return nil
}

// syncStreamProcessorActionResult syncs stream processor state from action result
// Called when an action-result is received for stream processor operations
// Handles deploy, edit, delete, and get operations with proper error validation.
func (uc *InstanceUsecase) syncStreamProcessorActionResult(ctx context.Context, action *models.Action, payload []byte) error {
	if uc.streamProcessorRepo == nil {
		return nil
	}

	// Extract tenant_id from context (set by middleware)
	tenantID := middleware.GetTenantID(ctx)

	// Determine action type
	var actionType string
	switch action.Type {
	case "deploy-stream-processor":
		actionType = "deploy"
	case "edit-stream-processor":
		actionType = "edit"
	case "delete-stream-processor":
		actionType = "delete"
	case "get-stream-processor":
		actionType = "get"
	default:
		return nil
	}

	// Only sync COMPLETED actions
	if action.Status != models.ActionStatusCompleted {
		return nil
	}

	// Successful delete operations commonly return an empty result payload.
	// In that case, we must delete based on the ORIGINAL queued action payload.
	if actionType == "delete" && len(payload) == 0 {
		if action.Payload != nil && len(action.Payload.Value) > 0 {
			var origPayload map[string]interface{}
			if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
				// The queued payload is JSON-serialized from the proto request.
				// Keys are expected to be "uuid" (and "device_id").
				uuid := ""
				if u, ok := origPayload["uuid"].(string); ok && u != "" {
					uuid = u
				} else if u, ok := origPayload["Uuid"].(string); ok && u != "" {
					uuid = u
				}
				if uuid != "" {
					_ = uc.streamProcessorRepo.Delete(ctx, tenantID, action.DeviceId, uuid)
				}
			}
		}
		return nil
	}

	// Decode and decompress payload
	decodedPayload := payload
	if decoded, err := base64.StdEncoding.DecodeString(string(payload)); err == nil {
		decodedPayload = decoded
	}

	if decompressed, err := decompressIfNeeded(decodedPayload); err == nil {
		decodedPayload = decompressed
	}

	var responsePayload map[string]interface{}
	if err := json.Unmarshal(decodedPayload, &responsePayload); err != nil {
		return nil
	}

	// Check for error in response BEFORE processing
	// Device may return actionReplyPayload with error field indicating failure
	var streamProcessor map[string]interface{}
	if payload, ok := responsePayload["Payload"].(map[string]interface{}); ok {
		if actionReplyPayload, ok := payload["actionReplyPayload"].(map[string]interface{}); ok {
			streamProcessor = actionReplyPayload
		}
	}

	if len(streamProcessor) == 0 {
		if result, ok := responsePayload["result"].(map[string]interface{}); ok {
			streamProcessor = result
		}
	}

	// Validate no error in response before syncing
	// This prevents syncing failed operations
	if errorMsg, ok := streamProcessor["error"].(string); ok && errorMsg != "" {
		fmt.Printf("Warning: Stream processor action (%s) failed on device: %s\n", actionType, errorMsg)
		return nil
	}

	// Delete actions often return a minimal success payload (e.g. { "deleted_name": "..." })
	// with no uuid field. For delete, always fall back to the ORIGINAL queued action payload
	// to determine which record to remove from the DB.
	if actionType == "delete" {
		// Try to extract UUID from the action request payload first (canonical for delete).
		if action.Payload != nil && len(action.Payload.Value) > 0 {
			var origPayload map[string]interface{}
			if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
				uuid := ""
				if u, ok := origPayload["uuid"].(string); ok && u != "" {
					uuid = u
				} else if u, ok := origPayload["Uuid"].(string); ok && u != "" {
					uuid = u
				}
				if uuid != "" {
					_ = uc.streamProcessorRepo.Delete(ctx, tenantID, action.DeviceId, uuid)
					return nil
				}
			}
		}
	}

	// Handle delete with empty result (successful deletion returns no data)
	if actionType == "delete" && len(streamProcessor) == 0 {
		if action.Payload != nil {
			var origPayload map[string]interface{}
			if err := json.Unmarshal(action.Payload.Value, &origPayload); err == nil {
				// Try both snake_case and PascalCase for uuid field
				uuid := ""
				if u, ok := origPayload["uuid"].(string); ok && u != "" {
					uuid = u
				} else if u, ok := origPayload["Uuid"].(string); ok && u != "" {
					uuid = u
				}

				if uuid != "" {
					// Empty payload on delete = success; delete from DB
					_ = uc.streamProcessorRepo.Delete(ctx, tenantID, action.DeviceId, uuid)
					return nil
				}
			}
		}
		return nil
	}

	if len(streamProcessor) == 0 {
		return nil
	}

	// Extract UUID
	uuid, ok := streamProcessor["uuid"].(string)
	if !ok || uuid == "" {
		return nil
	}

	// Handle based on action type
	switch actionType {
	case "deploy":
		// Deploy returns full StreamProcessor: UUID, name, model, config, location, etc.
		// Extract all fields from response
		name, _ := streamProcessor["name"].(string)

		var modelName, modelVersion string
		if model, ok := streamProcessor["model"].(map[string]interface{}); ok {
			modelName, _ = model["name"].(string)
			modelVersion, _ = model["version"].(string)
		}

		encodedConfig, _ := streamProcessor["encodedConfig"].(string)

		var locationJSON, metadataJSON string
		if location, ok := streamProcessor["location_json"].(map[string]interface{}); ok {
			if b, err := json.Marshal(location); err == nil {
				locationJSON = string(b)
			}
		}
		if metadata, ok := streamProcessor["metadata_json"].(map[string]interface{}); ok {
			if b, err := json.Marshal(metadata); err == nil {
				metadataJSON = string(b)
			}
		}

		ignoreHealthCheck := false
		if val, ok := streamProcessor["ignoreHealthCheck"].(bool); ok {
			ignoreHealthCheck = val
		}

		// Upsert with full response data
		err := uc.streamProcessorRepo.Upsert(ctx, tenantID, action.DeviceId, uuid, name, modelName, modelVersion, encodedConfig, ignoreHealthCheck, locationJSON, metadataJSON)
		if err != nil {
			fmt.Printf("Warning: Failed to upsert stream processor (deploy): %v\n", err)
		}

	case "edit":
		// Edit returns only UUID from response
		// Must extract name, model, config from REQUEST payload since response doesn't have them
		var name, modelName, modelVersion, encodedConfig string
		var locationJSON string

		if action.Payload != nil && action.Payload.Value != nil {
			var editRequest map[string]interface{}
			if err := json.Unmarshal(action.Payload.Value, &editRequest); err == nil {
				// Debug: Log what we got
				fmt.Printf("DEBUG: Edit sync - editRequest keys: %v\n", func() []string {
					keys := make([]string, 0, len(editRequest))
					for k := range editRequest {
						keys = append(keys, k)
					}
					return keys
				}())

				// Extract name from request
				name, _ = editRequest["name"].(string)

				// Extract model from request (v2.StreamProcessor uses Model as nested object)
				if modelObj, ok := editRequest["model"].(map[string]interface{}); ok {
					if modelName_, ok := modelObj["name"].(string); ok {
						modelName = modelName_
					}
					if modelVersion_, ok := modelObj["version"].(string); ok {
						modelVersion = modelVersion_
					}
				}
				// Fallback: Try top-level fields (if request was original API format)
				if modelName == "" {
					if modelObj, ok := editRequest["modelName"].(string); ok {
						modelName = modelObj
					}
				}
				if modelVersion == "" {
					if versionObj, ok := editRequest["modelVersion"].(string); ok {
						modelVersion = versionObj
					}
				}

				fmt.Printf("DEBUG: Edit sync extracted - name: %s, modelName: %s, modelVersion: %s\n", name, modelName, modelVersion)

				// Extract and encode config from request
				if configObj, ok := editRequest["config"].(map[string]interface{}); ok {
					configYAML, err := yaml.Marshal(configObj)
					if err == nil {
						encodedConfig = base64.StdEncoding.EncodeToString(configYAML)
					}
				}

				// Extract location from request
				if locationObj, ok := editRequest["location"].(map[string]interface{}); ok {
					if b, err := json.Marshal(locationObj); err == nil {
						locationJSON = string(b)
					}
				}
			}
		}

		// Upsert with data from request payload (editor has confirmed the edit succeeded via UUID response)
		err := uc.streamProcessorRepo.Upsert(ctx, tenantID, action.DeviceId, uuid, name, modelName, modelVersion, encodedConfig, false, locationJSON, "")
		if err != nil {
			fmt.Printf("Warning: Failed to upsert stream processor (edit): %v\n", err)
		}

	case "get":
		// Get returns full StreamProcessor with current state/status from device
		// Extract name, model, config, and status information
		name, _ := streamProcessor["name"].(string)

		var modelName, modelVersion string
		if model, ok := streamProcessor["model"].(map[string]interface{}); ok {
			modelName, _ = model["name"].(string)
			modelVersion, _ = model["version"].(string)
		}

		encodedConfig, _ := streamProcessor["encodedConfig"].(string)

		var locationJSON, metadataJSON string
		if location, ok := streamProcessor["location"].(map[string]interface{}); ok {
			if b, err := json.Marshal(location); err == nil {
				locationJSON = string(b)
			}
		}
		if metadata, ok := streamProcessor["metadata"].(map[string]interface{}); ok {
			if b, err := json.Marshal(metadata); err == nil {
				metadataJSON = string(b)
			}
		}

		ignoreHealthCheck := false
		if val, ok := streamProcessor["ignoreHealthCheck"].(bool); ok {
			ignoreHealthCheck = val
		}

		// Upsert basic processor info
		err := uc.streamProcessorRepo.Upsert(ctx, tenantID, action.DeviceId, uuid, name, modelName, modelVersion, encodedConfig, ignoreHealthCheck, locationJSON, metadataJSON)
		if err != nil {
			fmt.Printf("Warning: Failed to upsert stream processor (get): %v\n", err)
		}

		// Extract and update status from Get response
		// GetStreamProcessor returns configuration (uuid, name, model, config, location, etc.)
		// It does NOT include runtime health state information.
		//
		// What we CAN infer from a successful Get:
		// - Processor EXISTS on device (can be retrieved) → deployment_status = "ACTIVE"
		//
		// What we CANNOT infer without StatusMessage:
		// - Processor's actual runtime state (running, stopped, failed, etc.)
		// - Processor's health condition (healthy, degraded, offline, etc.)
		//
		// Therefore:
		// - Set deployment_status = "ACTIVE" (exists and is deployed)
		// - Keep health_status = "UNKNOWN" (wait for StatusMessage with actual Health.Category)
		//
		// StatusMessage (from device's 1-second heartbeat) will provide the actual health via:
		//   Health.State → deployment_status mapping
		//   Health.Category → health_status mapping (active/healthy → ONLINE, degraded/neutral → OFFLINE)

		deploymentStatus := "ACTIVE" // Get succeeded = processor exists and is deployed
		healthStatus := "UNKNOWN"    // Wait for StatusMessage with actual Health.Category
		errorMessage := ""

		// Update status and mark as synced
		_ = uc.streamProcessorRepo.UpdateStatus(ctx, tenantID, action.DeviceId, uuid, deploymentStatus, healthStatus, errorMessage)
		_ = uc.streamProcessorRepo.MarkSynced(ctx, tenantID, action.DeviceId, uuid)
	}

	return nil
}

// syncProtocolConvertersFromStatusMessage syncs protocol converters from device's StatusMessage
// Implements "eventual consistency" - device state wins, reconciles DB with actual device state
// Called periodically via device heartbeat StatusMessage
func (uc *InstanceUsecase) syncProtocolConvertersFromStatusMessage(ctx context.Context, tenantID, deviceID, messageContent string) error {
	if uc.protocolConverterRepo == nil {
		return nil // Skip if repo unavailable
	}

	// Decode message content if base64 encoded.
	decodedBytes := []byte(messageContent)
	if messageContent != "" {
		if b, err := base64.StdEncoding.DecodeString(messageContent); err == nil && len(b) > 0 {
			decodedBytes = b
		}
	}

	// Heartbeat payloads may be compressed (zstd/gzip). Decompress when detected.
	if b, err := decompressIfNeeded(decodedBytes); err != nil {
		fmt.Printf("Warning: Failed to decompress StatusMessage payload: %v\n", err)
		return nil
	} else if len(b) > 0 {
		decodedBytes = b
	}

	// Parse JSON StatusMessage (umh-core sends JSON; payload may be base64+compressed).
	var messageData map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(decodedBytes))
	dec.UseNumber()
	if err := dec.Decode(&messageData); err != nil || messageData == nil {
		// Preserve original warning format for log filters/dashboards.
		fmt.Printf("Warning: Failed to parse StatusMessage JSON: %v\n", err)
		return nil
	}

	// Extract DFCs from status message to find protocol converters
	var discoveredConverters []*ConverterInfo
	// The heartbeat may be either the StatusMessage object itself or a wrapper with "Payload".
	payload := messageData
	if p := jsonMap(messageData, "Payload", "payload"); p != nil {
		payload = p
	}

	core := jsonMap(payload, "core", "Core")
	if core == nil {
		return nil
	}

	dfcs, _ := core["dfcs"].([]interface{})
	if dfcs == nil {
		dfcs, _ = core["Dfcs"].([]interface{})
	}
	for _, dfc := range dfcs {
		dfcMap, ok := dfc.(map[string]interface{})
		if !ok {
			continue
		}
		dfcType := jsonString(dfcMap, "dfcType", "dfc_type", "DfcType")
		if dfcType != "protocol-converter" {
			continue
		}

		converter := &ConverterInfo{
			UUID: jsonString(dfcMap, "uuid", "Uuid", "UUID"),
			Name: jsonString(dfcMap, "name", "Name"),
			Type: dfcType,
		}
		if converter.UUID != "" && converter.Name != "" {
			discoveredConverters = append(discoveredConverters, converter)
		}
	}

	// Step 1: Create or update discovered converters
	for _, converter := range discoveredConverters {
		if converter.UUID == "" {
			continue // Skip if no UUID
		}

		// Use Upsert to create or update
		// This handles both new converters and converters transitioning from PENDING to ACTIVE
		err := uc.protocolConverterRepo.Upsert(ctx,
			tenantID,
			deviceID,
			converter.UUID,
			converter.Name,
			converter.Type,
			converter.ConnectionUUID,
		)
		if err != nil {
			fmt.Printf("Warning: Failed to sync protocol converter %s: %v\n", converter.UUID, err)
		}
	}

	// Step 2: Mark converters as OFFLINE if they were ACTIVE but are no longer in device status
	// This reconciles DB with device state when converters are removed
	allConverters, err := uc.protocolConverterRepo.List(ctx, tenantID, &data.ListQuery{
		DeviceID:               deviceID,
		DeploymentStatusFilter: "ACTIVE",
		Limit:                  1000,
	})
	if err != nil {
		fmt.Printf("Warning: Failed to list existing converters: %v\n", err)
		return nil
	}

	// Check which converters in DB are no longer in device state
	for _, dbConverter := range allConverters {
		found := false
		for _, statusConverter := range discoveredConverters {
			if statusConverter.UUID == dbConverter.UUID {
				found = true
				break
			}
		}

		// If converter was ACTIVE in DB but not in StatusMessage, mark as OFFLINE
		if !found && dbConverter.DeploymentStatus == "ACTIVE" {
			err := uc.protocolConverterRepo.UpdateStatus(ctx,
				tenantID,
				deviceID,
				dbConverter.UUID,
				dbConverter.DeploymentStatus,
				"OFFLINE", // Mark health as offline
				"",        // Clear error message
			)
			if err != nil {
				fmt.Printf("Warning: Failed to mark converter %s as offline: %v\n", dbConverter.UUID, err)
			}
		}
	}

	// Mark all updated converters as synced
	for _, converter := range discoveredConverters {
		if converter.UUID != "" {
			_ = uc.protocolConverterRepo.MarkSynced(ctx, tenantID, deviceID, converter.UUID)
		}
	}

	return nil
}

// ConverterInfo holds minimal protocol converter information extracted from StatusMessage
type ConverterInfo struct {
	UUID           string
	Name           string
	Type           string
	ConnectionUUID string
	Health         *v2.Health
}

// syncStreamProcessorsFromStatusMessage syncs stream processors from device's StatusMessage
// Implements "eventual consistency" - device state wins, reconciles DB with actual device state
// Called periodically via device heartbeat StatusMessage
func (uc *InstanceUsecase) syncStreamProcessorsFromStatusMessage(ctx context.Context, tenantID, deviceID, messageContent string) error {
	if uc.streamProcessorRepo == nil {
		return nil // Skip if repo unavailable
	}

	// Decode message content if base64 encoded.
	decodedBytes := []byte(messageContent)
	if messageContent != "" {
		if b, err := base64.StdEncoding.DecodeString(messageContent); err == nil && len(b) > 0 {
			decodedBytes = b
		}
	}

	// Heartbeat payloads may be compressed (zstd/gzip). Decompress when detected.
	if b, err := decompressIfNeeded(decodedBytes); err != nil {
		fmt.Printf("Warning: Failed to decompress StatusMessage payload: %v\n", err)
		return nil
	} else if len(b) > 0 {
		decodedBytes = b
	}

	// Parse JSON StatusMessage (umh-core sends JSON; payload may be base64+compressed).
	var messageData map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(decodedBytes))
	dec.UseNumber()
	if err := dec.Decode(&messageData); err != nil || messageData == nil {
		// Preserve original warning format for log filters/dashboards.
		fmt.Printf("Warning: Failed to parse StatusMessage JSON: %v\n", err)
		return nil
	}

	// Extract DFCs from status message to find stream processors
	var discoveredProcessors []*StreamProcessorStatusInfo
	// The heartbeat may be either the StatusMessage object itself or a wrapper with "Payload".
	payload := messageData
	if p := jsonMap(messageData, "Payload", "payload"); p != nil {
		payload = p
	}

	core := jsonMap(payload, "core", "Core")
	if core == nil {
		return nil
	}

	dfcs, _ := core["dfcs"].([]interface{})
	if dfcs == nil {
		dfcs, _ = core["Dfcs"].([]interface{})
	}
	for _, dfc := range dfcs {
		dfcMap, ok := dfc.(map[string]interface{})
		if !ok {
			continue
		}
		dfcType := jsonString(dfcMap, "dfcType", "dfc_type", "DfcType")
		if dfcType != "stream-processor" {
			continue
		}

		processor := &StreamProcessorStatusInfo{
			UUID: jsonString(dfcMap, "uuid", "Uuid", "UUID"),
			Name: jsonString(dfcMap, "name", "Name"),
			Type: dfcType,
		}

		// Extract Health from JSON
		if healthMap := jsonMap(dfcMap, "health", "Health"); healthMap != nil {
			processor.Health = &v2.Health{
				Message:      jsonString(healthMap, "message", "Message"),
				State:        jsonString(healthMap, "state", "State"),
				DesiredState: jsonString(healthMap, "desiredState", "desired_state", "DesiredState"),
				Category:     jsonString(healthMap, "category", "Category"),
			}
		}

		if processor.UUID != "" && processor.Name != "" {
			discoveredProcessors = append(discoveredProcessors, processor)
		}
	}

	// Step 1: Update status for discovered stream processors
	for _, processor := range discoveredProcessors {
		if processor.UUID == "" {
			continue // Skip if no UUID
		}

		// Map health state to deployment/health status
		deploymentStatus := "PENDING"
		healthStatus := "UNKNOWN"

		if processor.Health != nil {
			// Determine deployment status from health state
			// Common states: "active", "idle", "pending", "starting", "error", etc.
			state := processor.Health.State
			category := processor.Health.Category

			if state == "active" || state == "idle" {
				deploymentStatus = "ACTIVE"
			} else if state == "error" || state == "failed" {
				deploymentStatus = "FAILED"
			} else if state == "pending" || state == "starting" {
				deploymentStatus = "PENDING"
			}

			// Determine health status from category
			if category == "active" || category == "healthy" {
				healthStatus = "ONLINE"
			} else if category == "degraded" || category == "neutral" {
				healthStatus = "OFFLINE"
			}
		}

		// Update status in database
		err := uc.streamProcessorRepo.UpdateStatus(ctx,
			tenantID,
			deviceID,
			processor.UUID,
			deploymentStatus,
			healthStatus,
			"", // Clear error message on success
		)
		if err != nil {
			fmt.Printf("Warning: Failed to update stream processor status %s: %v\n", processor.UUID, err)
		}

		// Mark as synced
		_ = uc.streamProcessorRepo.MarkSynced(ctx, tenantID, deviceID, processor.UUID)
	}

	// Step 2: Mark stream processors as OFFLINE if they were ACTIVE but are no longer in device status
	// This reconciles DB with device state when processors are removed
	allProcessors, err := uc.streamProcessorRepo.List(ctx, tenantID, &data.StreamProcessorListQuery{
		DeviceID: deviceID,
		Limit:    1000,
	})
	if err != nil {
		fmt.Printf("Warning: Failed to list existing stream processors: %v\n", err)
		return nil
	}

	// Check which processors in DB are no longer in device state
	for _, dbProcessor := range allProcessors {
		found := false
		for _, statusProcessor := range discoveredProcessors {
			if statusProcessor.UUID == dbProcessor.UUID {
				found = true
				break
			}
		}

		// If processor was ACTIVE in DB but not in StatusMessage, mark as OFFLINE
		deploymentStatus := dbProcessor.DeploymentStatus.String
		if !found && deploymentStatus == "ACTIVE" {
			err := uc.streamProcessorRepo.UpdateStatus(ctx,
				tenantID,
				deviceID,
				dbProcessor.UUID,
				deploymentStatus,
				"OFFLINE", // Mark health as offline
				"",        // Clear error message
			)
			if err != nil {
				fmt.Printf("Warning: Failed to mark stream processor %s as offline: %v\n", dbProcessor.UUID, err)
			}
		}
	}

	return nil
}

// StreamProcessorStatusInfo holds minimal stream processor information extracted from StatusMessage
type StreamProcessorStatusInfo struct {
	UUID   string
	Name   string
	Type   string
	Health *v2.Health
}

// extractStringField safely extracts a string field from a map
func extractStringField(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}
