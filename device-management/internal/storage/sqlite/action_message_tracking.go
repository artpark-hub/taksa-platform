package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"taksa-platform-dm/internal/storage"
)

// ActionMessageTrackingStore implements storage.ActionMessageTracker for SQLite
type ActionMessageTrackingStore struct {
	db *sql.DB
}

// newActionMessageTrackingStore creates a new ActionMessageTrackingStore
func newActionMessageTrackingStore(db *sql.DB) *ActionMessageTrackingStore {
	return &ActionMessageTrackingStore{db: db}
}

// Create stores a new action message tracking record
func (s *ActionMessageTrackingStore) Create(ctx context.Context, track *storage.ActionMessageTracking) error {
	if track == nil || track.ID == "" || track.ActionID == "" || track.DeviceID == "" || track.TraceID == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO action_message_tracking (
		id, action_id, device_id, trace_id, trace_generated_at,
		response_message_id, correlation_status, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		track.ID,
		track.ActionID,
		track.DeviceID,
		track.TraceID,
		track.TraceGeneratedAt.Format(time.RFC3339),
		track.ResponseMessageID,
		track.CorrelationStatus,
		track.CreatedAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to create action message tracking: %w", err)
	}

	return nil
}

// GetByTraceID retrieves tracking record by trace_id (PRIMARY correlation lookup)
func (s *ActionMessageTrackingStore) GetByTraceID(ctx context.Context, traceID string) (*storage.ActionMessageTracking, error) {
	if traceID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT 
		id, action_id, device_id, trace_id, trace_generated_at,
		response_trace_id, response_message_id, response_received_at, correlation_status,
		created_at, completed_at
	FROM action_message_tracking
	WHERE trace_id = ?
	LIMIT 1
	`

	row := s.db.QueryRowContext(ctx, query, traceID)
	return s.scanActionMessageTracking(row)
}

// GetByActionID retrieves tracking record by action_id
func (s *ActionMessageTrackingStore) GetByActionID(ctx context.Context, actionID string) (*storage.ActionMessageTracking, error) {
	if actionID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT 
		id, action_id, device_id, trace_id, trace_generated_at,
		response_trace_id, response_message_id, response_received_at, correlation_status,
		created_at, completed_at
	FROM action_message_tracking
	WHERE action_id = ?
	LIMIT 1
	`

	row := s.db.QueryRowContext(ctx, query, actionID)
	return s.scanActionMessageTracking(row)
}

// UpdateResponse updates response side of tracking record
// Called when device response is received (Push)
func (s *ActionMessageTrackingStore) UpdateResponse(ctx context.Context, id string, responseTraceID string, correlationStatus int) error {
	if id == "" {
		return ErrInvalidInput
	}

	query := `
	UPDATE action_message_tracking
	SET response_trace_id = ?, response_received_at = ?, correlation_status = ?
	WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		responseTraceID,
		time.Now().Format(time.RFC3339),
		correlationStatus,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update response: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateResponseWithMessageID updates response side with message ID (direct pointer for O(1) lookup)
// Called when device response is received (Push) with actionUUID correlation
func (s *ActionMessageTrackingStore) UpdateResponseWithMessageID(ctx context.Context, id string, messageID string, correlationStatus int) error {
	if id == "" {
		return ErrInvalidInput
	}

	query := `
	UPDATE action_message_tracking
	SET response_message_id = ?, response_received_at = ?, correlation_status = ?
	WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		messageID,
		time.Now().Format(time.RFC3339),
		correlationStatus,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update response with message_id: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateCompleted marks tracking as completed
func (s *ActionMessageTrackingStore) UpdateCompleted(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	query := `
	UPDATE action_message_tracking
	SET completed_at = ?, correlation_status = 4
	WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query, time.Now().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("failed to mark completed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// ListPendingCorrelations returns uncorrelated responses
// Used for debugging and cleanup
func (s *ActionMessageTrackingStore) ListPendingCorrelations(ctx context.Context, deviceID string) ([]*storage.ActionMessageTracking, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT 
		id, action_id, device_id, trace_id, trace_generated_at,
		response_trace_id, response_message_id, response_received_at, correlation_status,
		created_at, completed_at
	FROM action_message_tracking
	WHERE device_id = ? AND correlation_status < 3
	ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending correlations: %w", err)
	}
	defer rows.Close()

	var results []*storage.ActionMessageTracking
	for rows.Next() {
		track, err := s.scanActionMessageTrackingRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, track)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// ListByDevice returns all tracking records for a device
// Used for audit trails and diagnostics
func (s *ActionMessageTrackingStore) ListByDevice(ctx context.Context, deviceID string) ([]*storage.ActionMessageTracking, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	query := `
	SELECT 
		id, action_id, device_id, trace_id, trace_generated_at,
		response_trace_id, response_message_id, response_received_at, correlation_status,
		created_at, completed_at
	FROM action_message_tracking
	WHERE device_id = ?
	ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query by device: %w", err)
	}
	defer rows.Close()

	var results []*storage.ActionMessageTracking
	for rows.Next() {
		track, err := s.scanActionMessageTrackingRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, track)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// Helper: scanActionMessageTracking scans a single row into ActionMessageTracking
func (s *ActionMessageTrackingStore) scanActionMessageTracking(row *sql.Row) (*storage.ActionMessageTracking, error) {
	track := &storage.ActionMessageTracking{}
	var traceGeneratedAt, createdAt, responseReceivedAt, completedAt sql.NullString
	var responseTraceID, responseMessageID sql.NullString

	err := row.Scan(
		&track.ID,
		&track.ActionID,
		&track.DeviceID,
		&track.TraceID,
		&traceGeneratedAt,
		&responseTraceID,
		&responseMessageID,
		&responseReceivedAt,
		&track.CorrelationStatus,
		&createdAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan action message tracking: %w", err)
	}

	// Parse timestamps
	if traceGeneratedAt.Valid {
		t, err := time.Parse(time.RFC3339, traceGeneratedAt.String)
		if err == nil {
			track.TraceGeneratedAt = t
		}
	}
	if createdAt.Valid {
		t, err := time.Parse(time.RFC3339, createdAt.String)
		if err == nil {
			track.CreatedAt = t
		}
	}
	if responseTraceID.Valid {
		track.ResponseTraceID = responseTraceID.String
	}
	if responseMessageID.Valid {
		track.ResponseMessageID = responseMessageID.String
	}
	if responseReceivedAt.Valid {
		t, err := time.Parse(time.RFC3339, responseReceivedAt.String)
		if err == nil {
			track.ResponseReceivedAt = &t
		}
	}
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err == nil {
			track.CompletedAt = &t
		}
	}

	return track, nil
}

// Helper: scanActionMessageTrackingRow scans from rows.Scan
func (s *ActionMessageTrackingStore) scanActionMessageTrackingRow(rows *sql.Rows) (*storage.ActionMessageTracking, error) {
	track := &storage.ActionMessageTracking{}
	var traceGeneratedAt, createdAt, responseReceivedAt, completedAt sql.NullString
	var responseTraceID, responseMessageID sql.NullString

	err := rows.Scan(
		&track.ID,
		&track.ActionID,
		&track.DeviceID,
		&track.TraceID,
		&traceGeneratedAt,
		&responseTraceID,
		&responseMessageID,
		&responseReceivedAt,
		&track.CorrelationStatus,
		&createdAt,
		&completedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	// Parse timestamps
	if traceGeneratedAt.Valid {
		t, err := time.Parse(time.RFC3339, traceGeneratedAt.String)
		if err == nil {
			track.TraceGeneratedAt = t
		}
	}
	if createdAt.Valid {
		t, err := time.Parse(time.RFC3339, createdAt.String)
		if err == nil {
			track.CreatedAt = t
		}
	}
	if responseTraceID.Valid {
		track.ResponseTraceID = responseTraceID.String
	}
	if responseMessageID.Valid {
		track.ResponseMessageID = responseMessageID.String
	}
	if responseReceivedAt.Valid {
		t, err := time.Parse(time.RFC3339, responseReceivedAt.String)
		if err == nil {
			track.ResponseReceivedAt = &t
		}
	}
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err == nil {
			track.CompletedAt = &t
		}
	}

	return track, nil
}
