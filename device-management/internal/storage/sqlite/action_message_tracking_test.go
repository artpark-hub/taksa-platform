package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"taksa-platform-dm/internal/storage"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create tables
	createTableSQL := `
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

	CREATE TABLE IF NOT EXISTS devices (
	  id TEXT PRIMARY KEY,
	  uuid TEXT UNIQUE NOT NULL,
	  name TEXT NOT NULL,
	  serial_number TEXT UNIQUE NOT NULL,
	  status INTEGER DEFAULT 1,
	  created_at TEXT DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	return db
}

func TestActionMessageTrackingCreate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	track := &storage.ActionMessageTracking{
		ID:                generateTestUUID(),
		ActionID:          "action-123",
		DeviceID:          "device-456",
		TraceID:           "trace-789",
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 1, // PENDING
		CreatedAt:         time.Now(),
	}

	err := store.Create(ctx, track)
	if err != nil {
		t.Fatalf("Failed to create tracking: %v", err)
	}

	// Verify it was created
	retrieved, err := store.GetByTraceID(ctx, "trace-789")
	if err != nil {
		t.Fatalf("Failed to retrieve tracking: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved tracking is nil")
	}
	if retrieved.ActionID != "action-123" {
		t.Errorf("Expected action_id 'action-123', got '%s'", retrieved.ActionID)
	}
	if retrieved.DeviceID != "device-456" {
		t.Errorf("Expected device_id 'device-456', got '%s'", retrieved.DeviceID)
	}
	if retrieved.TraceID != "trace-789" {
		t.Errorf("Expected trace_id 'trace-789', got '%s'", retrieved.TraceID)
	}
	if retrieved.CorrelationStatus != 1 {
		t.Errorf("Expected correlation_status 1, got %d", retrieved.CorrelationStatus)
	}
}

func TestActionMessageTrackingGetByTraceID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	// Create a tracking record
	track := &storage.ActionMessageTracking{
		ID:                generateTestUUID(),
		ActionID:          "action-123",
		DeviceID:          "device-456",
		TraceID:           "trace-abc",
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 1,
		CreatedAt:         time.Now(),
	}
	_ = store.Create(ctx, track)

	// Test GetByTraceID
	retrieved, err := store.GetByTraceID(ctx, "trace-abc")
	if err != nil {
		t.Fatalf("Failed to get by trace_id: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved tracking is nil")
	}
	if retrieved.TraceID != "trace-abc" {
		t.Errorf("Expected trace_id 'trace-abc', got '%s'", retrieved.TraceID)
	}

	// Test non-existent trace_id
	notFound, err := store.GetByTraceID(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error for non-existent trace_id: %v", err)
	}
	if notFound != nil {
		t.Fatal("Expected nil for non-existent trace_id")
	}
}

func TestActionMessageTrackingGetByActionID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	// Create a tracking record
	track := &storage.ActionMessageTracking{
		ID:                generateTestUUID(),
		ActionID:          "action-xyz",
		DeviceID:          "device-456",
		TraceID:           "trace-123",
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 1,
		CreatedAt:         time.Now(),
	}
	_ = store.Create(ctx, track)

	// Test GetByActionID
	retrieved, err := store.GetByActionID(ctx, "action-xyz")
	if err != nil {
		t.Fatalf("Failed to get by action_id: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved tracking is nil")
	}
	if retrieved.ActionID != "action-xyz" {
		t.Errorf("Expected action_id 'action-xyz', got '%s'", retrieved.ActionID)
	}
}

func TestActionMessageTrackingUpdateResponse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	// Create a tracking record
	trackID := generateTestUUID()
	track := &storage.ActionMessageTracking{
		ID:                trackID,
		ActionID:          "action-123",
		DeviceID:          "device-456",
		TraceID:           "trace-789",
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 1,
		CreatedAt:         time.Now(),
	}
	_ = store.Create(ctx, track)

	// Update response
	err := store.UpdateResponse(ctx, trackID, "trace-789", 3) // 3 = RESPONSE_RECEIVED
	if err != nil {
		t.Fatalf("Failed to update response: %v", err)
	}

	// Verify update
	retrieved, _ := store.GetByTraceID(ctx, "trace-789")
	if retrieved.ResponseTraceID != "trace-789" {
		t.Errorf("Expected response_trace_id 'trace-789', got '%s'", retrieved.ResponseTraceID)
	}
	if retrieved.CorrelationStatus != 3 {
		t.Errorf("Expected correlation_status 3, got %d", retrieved.CorrelationStatus)
	}
	if retrieved.ResponseReceivedAt == nil {
		t.Fatal("Expected response_received_at to be set")
	}
}

func TestActionMessageTrackingUpdateCompleted(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	// Create a tracking record
	trackID := generateTestUUID()
	track := &storage.ActionMessageTracking{
		ID:                trackID,
		ActionID:          "action-123",
		DeviceID:          "device-456",
		TraceID:           "trace-789",
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 3, // Already RESPONSE_RECEIVED
		CreatedAt:         time.Now(),
	}
	_ = store.Create(ctx, track)

	// Update completed
	err := store.UpdateCompleted(ctx, trackID)
	if err != nil {
		t.Fatalf("Failed to update completed: %v", err)
	}

	// Verify update
	retrieved, _ := store.GetByActionID(ctx, "action-123")
	if retrieved.CorrelationStatus != 4 {
		t.Errorf("Expected correlation_status 4, got %d", retrieved.CorrelationStatus)
	}
	if retrieved.CompletedAt == nil {
		t.Fatal("Expected completed_at to be set")
	}
}

func TestActionMessageTrackingListPendingCorrelations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	deviceID := "device-456"

	// Create multiple tracking records with different statuses
	for i := 0; i < 3; i++ {
		track := &storage.ActionMessageTracking{
			ID:                generateTestUUID(),
			ActionID:          "action-" + string(rune(i)),
			DeviceID:          deviceID,
			TraceID:           "trace-" + string(rune(i)),
			TraceGeneratedAt:  time.Now(),
			CorrelationStatus: 1, // PENDING
			CreatedAt:         time.Now(),
		}
		_ = store.Create(ctx, track)
	}

	// Create a completed one (status 4)
	track := &storage.ActionMessageTracking{
		ID:                generateTestUUID(),
		ActionID:          "action-completed",
		DeviceID:          deviceID,
		TraceID:           "trace-completed",
		TraceGeneratedAt:  time.Now(),
		CorrelationStatus: 4, // COMPLETED
		CreatedAt:         time.Now(),
	}
	_ = store.Create(ctx, track)

	// List pending (should only get status < 3)
	pending, err := store.ListPendingCorrelations(ctx, deviceID)
	if err != nil {
		t.Fatalf("Failed to list pending correlations: %v", err)
	}

	// Should have 3 pending (not 4)
	if len(pending) != 3 {
		t.Errorf("Expected 3 pending correlations, got %d", len(pending))
	}

	// All should have status < 3
	for _, p := range pending {
		if p.CorrelationStatus >= 3 {
			t.Errorf("Expected correlation_status < 3, got %d", p.CorrelationStatus)
		}
	}
}

func TestActionMessageTrackingListByDevice(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := newActionMessageTrackingStore(db)
	ctx := context.Background()

	deviceID := "device-456"

	// Create multiple tracking records for same device
	for i := 0; i < 5; i++ {
		track := &storage.ActionMessageTracking{
			ID:                generateTestUUID(),
			ActionID:          "action-" + string(rune(i)),
			DeviceID:          deviceID,
			TraceID:           "trace-" + string(rune(i)),
			TraceGeneratedAt:  time.Now(),
			CorrelationStatus: 1,
			CreatedAt:         time.Now(),
		}
		_ = store.Create(ctx, track)
	}

	// List by device
	records, err := store.ListByDevice(ctx, deviceID)
	if err != nil {
		t.Fatalf("Failed to list by device: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("Expected 5 records, got %d", len(records))
	}

	// All should have the same device_id
	for _, r := range records {
		if r.DeviceID != deviceID {
			t.Errorf("Expected device_id '%s', got '%s'", deviceID, r.DeviceID)
		}
	}
}

// Helper: generateTestUUID generates a simple test UUID
func generateTestUUID() string {
	return "test-" + time.Now().Format("20060102150405") + "-" + string(rune(time.Now().Nanosecond()%1000))
}
