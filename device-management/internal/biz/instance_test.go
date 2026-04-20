package biz

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// TestExtractMessageTypeUncompressed tests extraction from uncompressed messages
func TestExtractMessageTypeUncompressed(t *testing.T) {
	msg := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionUUID": "test-123",
		},
	}
	msgJSON, _ := json.Marshal(msg)
	msgB64 := base64.StdEncoding.EncodeToString(msgJSON)

	result := extractMessageType(msgB64)
	if result != "action-reply" {
		t.Errorf("Expected 'action-reply', got '%s'", result)
	}
}

// TestExtractMessageTypeZstdCompressed tests extraction from zstd-compressed messages
func TestExtractMessageTypeZstdCompressed(t *testing.T) {
	msg := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionReplyState": "action-success",
			"actionUUID":       "test-456",
			"actionReplyPayload": bytes.Repeat([]byte("log data "), 200), // ~2KB
		},
	}
	msgJSON, _ := json.Marshal(msg)

	// Compress with zstd
	encoder, _ := zstd.NewWriter(nil)
	compressed := encoder.EncodeAll(msgJSON, nil)
	msgB64 := base64.StdEncoding.EncodeToString(compressed)

	result := extractMessageType(msgB64)
	if result != "action-reply" {
		t.Errorf("Expected 'action-reply', got '%s'", result)
	}
}

// TestExtractMessageTypeGzipCompressed tests extraction from gzip-compressed messages
func TestExtractMessageTypeGzipCompressed(t *testing.T) {
	msg := map[string]interface{}{
		"MessageType": "status-message",
	}
	msgJSON, _ := json.Marshal(msg)

	// Compress with gzip
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	gzipWriter.Write(msgJSON)
	gzipWriter.Close()
	compressed := buf.Bytes()
	msgB64 := base64.StdEncoding.EncodeToString(compressed)

	result := extractMessageType(msgB64)
	if result != "status-message" {
		t.Errorf("Expected 'status-message', got '%s'", result)
	}
}

// TestExtractActionReplyStateZstdCompressed tests state extraction from compressed messages
func TestExtractActionReplyStateZstdCompressed(t *testing.T) {
	msg := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionReplyState": "action-success",
			"actionUUID":       "test-789",
		},
	}
	msgJSON, _ := json.Marshal(msg)

	// Compress with zstd
	encoder, _ := zstd.NewWriter(nil)
	compressed := encoder.EncodeAll(msgJSON, nil)
	msgB64 := base64.StdEncoding.EncodeToString(compressed)

	result := extractActionReplyState(msgB64)
	if result != "action-success" {
		t.Errorf("Expected 'action-success', got '%s'", result)
	}
}

// TestExtractActionUUIDZstdCompressed tests UUID extraction from compressed messages
func TestExtractActionUUIDZstdCompressed(t *testing.T) {
	msg := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionUUID": "my-test-action-id",
		},
	}
	msgJSON, _ := json.Marshal(msg)

	// Compress with zstd
	encoder, _ := zstd.NewWriter(nil)
	compressed := encoder.EncodeAll(msgJSON, nil)
	msgB64 := base64.StdEncoding.EncodeToString(compressed)

	result := extractActionUUID(msgB64)
	if result != "my-test-action-id" {
		t.Errorf("Expected 'my-test-action-id', got '%s'", result)
	}
}

// TestExtractPayloadFromMessageZstdCompressed tests payload extraction from compressed messages
func TestExtractPayloadFromMessageZstdCompressed(t *testing.T) {
	msg := map[string]interface{}{
		"MessageType": "action-reply",
		"Payload": map[string]interface{}{
			"actionUUID": "action-999",
			"actionReplyPayload": []string{
				"[2026-03-12T05:57:46Z] INFO: Test log line",
				"[2026-03-12T05:57:47Z] WARN: Another log line",
			},
		},
	}
	msgJSON, _ := json.Marshal(msg)

	// Compress with zstd
	encoder, _ := zstd.NewWriter(nil)
	compressed := encoder.EncodeAll(msgJSON, nil)
	msgB64 := base64.StdEncoding.EncodeToString(compressed)

	result, err := extractPayloadFromMessage(msgB64, "action-999")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	var payload []string
	json.Unmarshal(result, &payload)
	if len(payload) != 2 || payload[0] != "[2026-03-12T05:57:46Z] INFO: Test log line" {
		t.Errorf("Unexpected payload extracted: %v", payload)
	}
}

// TestDecompressIfNeededUncompressed tests that uncompressed data passes through
func TestDecompressIfNeededUncompressed(t *testing.T) {
	data := []byte(`{"test": "data"}`)
	result, err := decompressIfNeeded(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Data should pass through uncompressed")
	}
}

// TestDecompressIfNeededZstd tests zstd decompression detection
func TestDecompressIfNeededZstd(t *testing.T) {
	data := []byte(`{"test": "data"}`)
	encoder, _ := zstd.NewWriter(nil)
	compressed := encoder.EncodeAll(data, nil)
	
	// Verify magic bytes
	if len(compressed) < 4 || compressed[0] != 0x28 || compressed[1] != 0xb5 || 
	   compressed[2] != 0x2f || compressed[3] != 0xfd {
		t.Errorf("Zstd magic bytes not found")
	}

	result, err := decompressIfNeeded(compressed)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Data should be decompressed correctly")
	}
}

// TestDecompressIfNeededGzip tests gzip decompression detection
func TestDecompressIfNeededGzip(t *testing.T) {
	data := []byte(`{"test": "data"}`)
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	gzipWriter.Write(data)
	gzipWriter.Close()
	compressed := buf.Bytes()

	// Verify magic bytes
	if len(compressed) < 2 || compressed[0] != 0x1f || compressed[1] != 0x8b {
		t.Errorf("Gzip magic bytes not found")
	}

	result, err := decompressIfNeeded(compressed)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Errorf("Data should be decompressed correctly")
	}
}
