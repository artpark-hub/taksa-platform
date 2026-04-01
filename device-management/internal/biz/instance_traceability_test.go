package biz

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "taksa-platform-dm/api/devicemgmt/v1"
	"taksa-platform-dm/internal/data"
	"taksa-platform-dm/internal/storage"
	"taksa-platform-dm/internal/storage/sqlite"
)

// setupTestStore creates an in-memory store for testing
func setupTestStore(t *testing.T) storage.Store {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create all tables
	createTablesSQL := `
	CREATE TABLE IF NOT EXISTS devices (
	  id TEXT PRIMARY KEY,
	  uuid TEXT UNIQUE NOT NULL,
	  name TEXT NOT NULL,
	  serial_number TEXT UNIQUE NOT NULL,
	  status INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  last_seen TEXT,
	  last_login_at TEXT
	);

	CREATE TABLE IF NOT EXISTS actions (
	  id TEXT PRIMARY KEY,
	  device_id TEXT NOT NULL,
	  action_type TEXT NOT NULL,
	  payload_type TEXT,
	  payload_data TEXT,
	  max_retries INTEGER DEFAULT 3,
	  retry_count INTEGER DEFAULT 0,
	  status INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  expires_at TEXT,
	  delivered_at TEXT,
	  completed_at TEXT,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS messages (
	  id TEXT PRIMARY KEY,
	  device_id TEXT NOT NULL,
	  message_type TEXT NOT NULL,
	  content TEXT,
	  trace_id TEXT,
	  request_id TEXT,
	  correlation_id TEXT,
	  direction INTEGER DEFAULT 0,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  expires_at TEXT,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS action_message_tracking (
	  id TEXT PRIMARY KEY,
	  action_id TEXT NOT NULL,
	  device_id TEXT NOT NULL,
	  trace_id TEXT NOT NULL,
	  trace_generated_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  response_trace_id TEXT,
	  response_received_at TEXT,
	  correlation_status INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  completed_at TEXT,
	  UNIQUE(action_id),
	  FOREIGN KEY(action_id) REFERENCES actions(id) ON DELETE CASCADE,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS message_tracking (
	  id TEXT PRIMARY KEY,
	  device_id TEXT NOT NULL,
	  trace_id TEXT NOT NULL UNIQUE,
	  action_id TEXT,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE,
	  FOREIGN KEY(action_id) REFERENCES actions(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS action_responses (
	  id TEXT PRIMARY KEY,
	  action_id TEXT NOT NULL,
	  device_id TEXT NOT NULL,
	  trace_id TEXT,
	  response_content TEXT,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  FOREIGN KEY(action_id) REFERENCES actions(id) ON DELETE CASCADE,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS auth_tokens (
	  id TEXT PRIMARY KEY,
	  token TEXT UNIQUE NOT NULL,
	  device_id TEXT NOT NULL,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  expires_at TEXT NOT NULL,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS certificates (
	  id TEXT PRIMARY KEY,
	  device_id TEXT NOT NULL,
	  user_email TEXT,
	  certificate TEXT NOT NULL,
	  private_key TEXT,
	  is_active INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  expires_at TEXT,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS device_certificates (
	  device_id TEXT PRIMARY KEY,
	  certificate TEXT NOT NULL,
	  private_key TEXT,
	  is_active INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  expires_at TEXT,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS user_certificates (
	  id TEXT PRIMARY KEY,
	  device_id TEXT NOT NULL,
	  user_email TEXT NOT NULL,
	  certificate TEXT NOT NULL,
	  private_key TEXT,
	  is_active INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
	  expires_at TEXT,
	  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS settings (
	  key TEXT PRIMARY KEY,
	  value TEXT NOT NULL,
	  description TEXT,
	  updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(createTablesSQL); err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	store, err := sqlite.NewStore(db)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	return store
}

func TestTraceability_PullStoresTraceID(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create a device
	device := &v1.Device{
		Id:           "device-123",
		Name:         "Test Device",
		SerialNumber: "SN-123",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, device)

	// Create an action
	action := &v1.Action{
		Id:       "action-456",
		DeviceId: "device-123",
		Type:     "setConfigFile",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/config",
			Value:   []byte(`{"config":"value"}`),
		},
		MaxRetries: 3,
		Status:     v1.ActionStatus_QUEUED,
		CreatedAt:  timestamppb.Now(),
	}
	_ = store.Actions().Save(ctx, action)

	// Create instance usecase
	authUc := &AuthUsecase{}
	instanceUc := NewInstanceUsecase(store, authUc, nil)

	// Pull messages
	messages, err := instanceUc.PullMessages(ctx, "device-123")
	if err != nil {
		t.Fatalf("Failed to pull messages: %v", err)
	}

	if messages == nil {
		t.Fatal("Expected messages, got nil")
	}

	msgSlice, ok := messages.([]map[string]interface{})
	if !ok || len(msgSlice) == 0 {
		t.Fatal("Expected non-empty message slice")
	}

	// Extract trace_id from response
	metadata := msgSlice[0]["metadata"].(map[string]interface{})
	traceID := metadata["traceId"].(string)

	if traceID == "" {
		t.Fatal("Expected trace_id in metadata, got empty")
	}

	// Verify trace_id is stored in action_message_tracking
	track, err := store.ActionMessageTracking().GetByTraceID(ctx, traceID)
	if err != nil {
		t.Fatalf("Failed to get tracking by trace_id: %v", err)
	}
	if track == nil {
		t.Fatal("Expected tracking record, got nil")
	}

	// Verify tracking record details
	if track.ActionID != "action-456" {
		t.Errorf("Expected action_id 'action-456', got '%s'", track.ActionID)
	}
	if track.DeviceID != "device-123" {
		t.Errorf("Expected device_id 'device-123', got '%s'", track.DeviceID)
	}
	if track.TraceID != traceID {
		t.Errorf("Expected trace_id to match, got mismatch")
	}
	if track.CorrelationStatus != 1 {
		t.Errorf("Expected correlation_status 1 (PENDING), got %d", track.CorrelationStatus)
	}
}

func TestTraceability_PushCorrelatesByTraceID(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create device and action
	device := &v1.Device{
		Id:           "device-123",
		Name:         "Test Device",
		SerialNumber: "SN-123",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, device)

	action := &v1.Action{
		Id:       "action-456",
		DeviceId: "device-123",
		Type:     "setConfigFile",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/config",
			Value:   []byte(`{"config":"value"}`),
		},
		MaxRetries: 3,
		Status:     v1.ActionStatus_QUEUED,
		CreatedAt:  timestamppb.Now(),
	}
	_ = store.Actions().Save(ctx, action)

	// Create instance usecase
	authUc := &AuthUsecase{}
	instanceUc := NewInstanceUsecase(store, authUc, nil)

	// Pull messages to generate trace_id
	messages, _ := instanceUc.PullMessages(ctx, "device-123")
	msgSlice := messages.([]map[string]interface{})
	metadata := msgSlice[0]["metadata"].(map[string]interface{})
	traceID := metadata["traceId"].(string)

	// Prepare response message with trace_id echo
	responsePayload := map[string]interface{}{
		"MessageType": "action-result",
		"Payload": map[string]interface{}{
			"actionUUID": "action-456",
			"result":     "success",
		},
	}
	responseContent, _ := json.Marshal(responsePayload)
	responseMsg := map[string]interface{}{
		"umhInstance": "device-123",
		"metadata": map[string]interface{}{
			"traceId": traceID, // Echo trace_id back
		},
		"content": base64.StdEncoding.EncodeToString(responseContent),
	}

	// Push response
	err := instanceUc.PushMessages(ctx, []map[string]interface{}{responseMsg}, "device-123")
	if err != nil {
		t.Fatalf("Failed to push messages: %v", err)
	}

	// Verify trace_id was correlated
	track, err := store.ActionMessageTracking().GetByTraceID(ctx, traceID)
	if err != nil {
		t.Fatalf("Failed to get tracking: %v", err)
	}
	if track == nil {
		t.Fatal("Expected tracking record after push")
	}

	// Verify response_trace_id was set
	if track.ResponseTraceID != traceID {
		t.Errorf("Expected response_trace_id to match trace_id")
	}

	// Verify correlation_status updated
	if track.CorrelationStatus != 3 {
		t.Errorf("Expected correlation_status 3 (RESPONSE_RECEIVED), got %d", track.CorrelationStatus)
	}

	// Verify response_received_at is set
	if track.ResponseReceivedAt == nil {
		t.Fatal("Expected response_received_at to be set")
	}

	// Verify action was marked COMPLETED
	act, _ := store.Actions().GetByID(ctx, "action-456")
	if act.Status != v1.ActionStatus_COMPLETED {
		t.Errorf("Expected action status COMPLETED, got %d", act.Status)
	}
}

func TestTraceability_CompleteEndToEndFlow(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Setup
	device := &v1.Device{
		Id:           "device-xyz",
		Name:         "Production Device",
		SerialNumber: "SN-XYZ",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, device)

	action := &v1.Action{
		Id:       "action-789",
		DeviceId: "device-xyz",
		Type:     "updateFirmware",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/firmware",
			Value:   []byte(`{"version":"1.2.3"}`),
		},
		MaxRetries: 3,
		Status:     v1.ActionStatus_QUEUED,
		CreatedAt:  timestamppb.Now(),
	}
	_ = store.Actions().Save(ctx, action)

	authUc := &AuthUsecase{}
	instanceUc := NewInstanceUsecase(store, authUc, nil)

	// PULL: Generate and store trace_id
	messages, _ := instanceUc.PullMessages(ctx, "device-xyz")
	msgSlice := messages.([]map[string]interface{})
	metadata := msgSlice[0]["metadata"].(map[string]interface{})
	traceID := metadata["traceId"].(string)

	t.Logf("Generated trace_id: %s", traceID)

	// Verify initial tracking state
	track1, _ := store.ActionMessageTracking().GetByTraceID(ctx, traceID)
	if track1.CorrelationStatus != 1 {
		t.Errorf("Expected PENDING status after Pull, got %d", track1.CorrelationStatus)
	}
	if track1.ResponseReceivedAt != nil {
		t.Error("Expected ResponseReceivedAt to be nil before Push")
	}

	// SIMULATE DEVICE EXECUTION
	time.Sleep(10 * time.Millisecond) // Simulate work

	// PUSH: Device sends response with trace_id echo
	responsePayload := map[string]interface{}{
		"MessageType": "action-result",
		"Payload": map[string]interface{}{
			"actionUUID": "action-789",
			"result":     "success",
			"version":    "1.2.3",
		},
	}
	responseContent, _ := json.Marshal(responsePayload)
	responseMsg := map[string]interface{}{
		"umhInstance": "device-xyz",
		"metadata": map[string]interface{}{
			"traceId": traceID, // Device echoes trace_id
		},
		"content": base64.StdEncoding.EncodeToString(responseContent),
	}

	_ = instanceUc.PushMessages(ctx, []map[string]interface{}{responseMsg}, "device-xyz")

	// Verify final tracking state
	track2, _ := store.ActionMessageTracking().GetByTraceID(ctx, traceID)
	if track2.CorrelationStatus != 3 {
		t.Errorf("Expected RESPONSE_RECEIVED status after Push, got %d", track2.CorrelationStatus)
	}
	if track2.ResponseTraceID != traceID {
		t.Error("Expected response_trace_id to be set to trace_id")
	}
	if track2.ResponseReceivedAt == nil {
		t.Error("Expected response_received_at to be set")
	}

	// Verify action completion
	act, _ := store.Actions().GetByID(ctx, "action-789")
	if act.Status != v1.ActionStatus_COMPLETED {
		t.Errorf("Expected action COMPLETED, got status %d", act.Status)
	}

	// Verify complete audit trail exists
	auditTrack, _ := store.ActionMessageTracking().GetByActionID(ctx, "action-789")
	if auditTrack == nil {
		t.Fatal("Expected audit trail for action")
	}

	// Calculate latency
	if auditTrack.ResponseReceivedAt != nil {
		latency := auditTrack.ResponseReceivedAt.Sub(auditTrack.TraceGeneratedAt)
		t.Logf("Action execution latency: %v", latency)
		if latency < 0 {
			t.Error("Latency should be positive")
		}
	}
}

func TestTraceability_DeviceMismatchDetected(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create two devices
	dev1 := &v1.Device{
		Id:           "device-1",
		Name:         "Device 1",
		SerialNumber: "SN-1",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, dev1)

	dev2 := &v1.Device{
		Id:           "device-2",
		Name:         "Device 2",
		SerialNumber: "SN-2",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, dev2)

	// Create action for device-1
	action := &v1.Action{
		Id:       "action-1",
		DeviceId: "device-1",
		Type:     "test",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/test",
			Value:   []byte(`{}`),
		},
		Status:    v1.ActionStatus_QUEUED,
		CreatedAt: timestamppb.Now(),
	}
	_ = store.Actions().Save(ctx, action)

	authUc := &AuthUsecase{}
	instanceUc := NewInstanceUsecase(store, authUc, nil)

	// Pull from device-1
	messages, _ := instanceUc.PullMessages(ctx, "device-1")
	msgSlice := messages.([]map[string]interface{})
	metadata := msgSlice[0]["metadata"].(map[string]interface{})
	traceID := metadata["traceId"].(string)

	// Try to push response from device-2 with same trace_id
	responseMsg := map[string]interface{}{
		"umhInstance": "device-2", // Different device!
		"metadata": map[string]interface{}{
			"traceId": traceID,
		},
		"content": base64.StdEncoding.EncodeToString([]byte(`{"MessageType":"action-result","Payload":{"actionUUID":"action-1"}}`)),
	}

	// This should fail with device mismatch
	err := instanceUc.PushMessages(ctx, []map[string]interface{}{responseMsg}, "device-2")

	// Should get an error about device mismatch
	if err == nil {
		t.Error("Expected error due to device mismatch")
	}
}

// TestTraceability_CompressedMessageDecompression tests that correlation works with zstandard-compressed messages
// This verifies the compression fix we implemented for extracting message types from compressed payloads
func TestTraceability_CompressedMessageDecompression(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create device
	device := &v1.Device{
		Id:           "device-compressed",
		Name:         "Compressed Test Device",
		SerialNumber: "SN-ZSTD",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, device)

	// Create action
	action := &v1.Action{
		Id:       "action-compressed",
		DeviceId: "device-compressed",
		Type:     "get-logs",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/config",
			Value:   []byte(`{"logType":"agent"}`),
		},
		MaxRetries: 3,
		Status:     v1.ActionStatus_QUEUED,
		CreatedAt:  timestamppb.Now(),
	}
	_ = store.Actions().Save(ctx, action)

	authUc := &AuthUsecase{}
	instanceUc := NewInstanceUsecase(store, authUc, nil)

	// Pull to generate trace_id
	messages, _ := instanceUc.PullMessages(ctx, "device-compressed")
	msgSlice := messages.([]map[string]interface{})
	metadata := msgSlice[0]["metadata"].(map[string]interface{})
	traceID := metadata["traceId"].(string)

	// Create a large payload that would be compressed by umh-core (>= 1KB)
	largePayload := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionUUID":         "action-compressed",
			"actionReplyState":   "action-success",
			"actionReplyPayload": bytes.Repeat([]byte("log entry "), 200), // ~2KB
		},
	}
	largeJSON, _ := json.Marshal(largePayload)

	// Compress with zstandard (same as umh-core does)
	encoder, _ := zstd.NewWriter(nil)
	compressedData := encoder.EncodeAll(largeJSON, nil)

	// Base64 encode (same as umh-core wire format)
	encodedContent := base64.StdEncoding.EncodeToString(compressedData)

	// Send compressed message with trace_id echo
	compressedMsg := map[string]interface{}{
		"umhInstance": "device-compressed",
		"metadata": map[string]interface{}{
			"traceId": traceID,
		},
		"content": encodedContent,
	}

	// Push compressed message
	err := instanceUc.PushMessages(ctx, []map[string]interface{}{compressedMsg}, "device-compressed")
	if err != nil {
		t.Fatalf("Failed to push compressed message: %v", err)
	}

	// Verify decompression and correlation worked
	track, err := store.ActionMessageTracking().GetByTraceID(ctx, traceID)
	if err != nil {
		t.Fatalf("Failed to get tracking: %v", err)
	}

	// Verify message was correctly decompressed and type extracted
	if track.CorrelationStatus != 3 {
		t.Errorf("Expected correlation_status 3 (RESPONSE_RECEIVED), got %d - decompression likely failed", track.CorrelationStatus)
	}

	// Verify action was completed
	act, _ := store.Actions().GetByID(ctx, "action-compressed")
	if act.Status != v1.ActionStatus_COMPLETED {
		t.Errorf("Expected action COMPLETED, got status %d - correlation failed with compressed message", act.Status)
	}

	t.Logf("✅ Zstandard compression test passed - correlation works with 99.9%% compressed payload")
}

// TestTraceability_GzipCompressedMessage tests that correlation works with gzip-compressed messages
func TestTraceability_GzipCompressedMessage(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create device
	device := &v1.Device{
		Id:           "device-gzip",
		Name:         "Gzip Test Device",
		SerialNumber: "SN-GZIP",
		Status:       v1.DeviceStatus_ACTIVE,
		CreatedAt:    timestamppb.Now(),
		LastSeen:     timestamppb.Now(),
	}
	_ = store.Devices().Save(ctx, device)

	// Create action
	action := &v1.Action{
		Id:       "action-gzip",
		DeviceId: "device-gzip",
		Type:     "get-metrics",
		Payload: &anypb.Any{
			TypeUrl: "type.googleapis.com/config",
			Value:   []byte(`{"metricType":"redpanda"}`),
		},
		MaxRetries: 3,
		Status:     v1.ActionStatus_QUEUED,
		CreatedAt:  timestamppb.Now(),
	}
	_ = store.Actions().Save(ctx, action)

	authUc := &AuthUsecase{}
	instanceUc := NewInstanceUsecase(store, authUc, nil)

	// Pull to generate trace_id
	messages, _ := instanceUc.PullMessages(ctx, "device-gzip")
	msgSlice := messages.([]map[string]interface{})
	metadata := msgSlice[0]["metadata"].(map[string]interface{})
	traceID := metadata["traceId"].(string)

	// Create payload and compress with gzip
	payload := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionUUID":       "action-gzip",
			"actionReplyState": "action-success",
		},
	}
	payloadJSON, _ := json.Marshal(payload)

	// Compress with gzip
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	gzipWriter.Write(payloadJSON)
	gzipWriter.Close()
	compressedData := buf.Bytes()

	// Base64 encode
	encodedContent := base64.StdEncoding.EncodeToString(compressedData)

	// Send gzip-compressed message
	gzipMsg := map[string]interface{}{
		"umhInstance": "device-gzip",
		"metadata": map[string]interface{}{
			"traceId": traceID,
		},
		"content": encodedContent,
	}

	err := instanceUc.PushMessages(ctx, []map[string]interface{}{gzipMsg}, "device-gzip")
	if err != nil {
		t.Fatalf("Failed to push gzip message: %v", err)
	}

	// Verify correlation worked
	track, err := store.ActionMessageTracking().GetByTraceID(ctx, traceID)
	if err != nil {
		t.Fatalf("Failed to get tracking: %v", err)
	}

	if track.CorrelationStatus != 3 {
		t.Errorf("Expected correlation_status 3, got %d", track.CorrelationStatus)
	}

	act, _ := store.Actions().GetByID(ctx, "action-gzip")
	if act.Status != v1.ActionStatus_COMPLETED {
		t.Errorf("Expected action COMPLETED, got status %d", act.Status)
	}

	t.Logf("✅ Gzip compression test passed - correlation works with gzip-compressed payload")
}
