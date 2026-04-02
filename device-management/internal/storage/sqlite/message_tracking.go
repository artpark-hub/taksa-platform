package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

type messageTrackingStore struct {
	db *sql.DB
}

// NewMessageTrackingStore creates a new message tracking store
func newMessageTrackingStore(db *sql.DB) storage.MessageTrackingStore {
	return &messageTrackingStore{db: db}
}

// Save stores a message tracking record
func (s *messageTrackingStore) Save(ctx context.Context, tracking *models.MessageTracking) error {
	if tracking == nil {
		return fmt.Errorf("tracking cannot be nil")
	}

	query := `
		INSERT INTO message_tracking (message_trace_id, action_id, device_id, pulled_at)
		VALUES (?, ?, ?, ?)
	`

	pulledAt := ""
	if !tracking.PulledAt.IsZero() {
		pulledAt = tracking.PulledAt.Format("2006-01-02 15:04:05")
	}

	_, err := s.db.ExecContext(ctx, query,
		tracking.MessageTraceID,
		tracking.ActionID,
		tracking.DeviceID,
		pulledAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save message tracking: %w", err)
	}

	return nil
}

// GetByTraceId retrieves message tracking by message trace ID
func (s *messageTrackingStore) GetByTraceId(ctx context.Context, messageTraceId string) (*models.MessageTracking, error) {
	if messageTraceId == "" {
		return nil, fmt.Errorf("message trace ID cannot be empty")
	}

	query := `
		SELECT message_trace_id, action_id, device_id, pulled_at
		FROM message_tracking
		WHERE message_trace_id = ?
	`

	var tracking models.MessageTracking
	var pulledAtStr string

	err := s.db.QueryRowContext(ctx, query, messageTraceId).Scan(
		&tracking.MessageTraceID,
		&tracking.ActionID,
		&tracking.DeviceID,
		&pulledAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found, return nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query message tracking: %w", err)
	}

	if pulledAtStr != "" {
		pulledAt, _ := time.Parse("2006-01-02 15:04:05", pulledAtStr)
		tracking.PulledAt = pulledAt
	}

	return &tracking, nil
}

// GetByActionId retrieves all message tracking records for an action
func (s *messageTrackingStore) GetByActionId(ctx context.Context, actionId string) ([]*models.MessageTracking, error) {
	if actionId == "" {
		return nil, fmt.Errorf("action ID cannot be empty")
	}

	query := `
		SELECT message_trace_id, action_id, device_id, pulled_at
		FROM message_tracking
		WHERE action_id = ?
		ORDER BY pulled_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, actionId)
	if err != nil {
		return nil, fmt.Errorf("failed to query message tracking: %w", err)
	}
	defer rows.Close()

	var trackings []*models.MessageTracking

	for rows.Next() {
		var tracking models.MessageTracking
		var pulledAtStr string

		err := rows.Scan(
			&tracking.MessageTraceID,
			&tracking.ActionID,
			&tracking.DeviceID,
			&pulledAtStr,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan message tracking: %w", err)
		}

		if pulledAtStr != "" {
			pulledAt, _ := time.Parse("2006-01-02 15:04:05", pulledAtStr)
			tracking.PulledAt = pulledAt
		}

		trackings = append(trackings, &tracking)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message tracking rows: %w", err)
	}

	return trackings, nil
}

// Delete removes a message tracking record
func (s *messageTrackingStore) Delete(ctx context.Context, messageTraceId string) error {
	if messageTraceId == "" {
		return fmt.Errorf("message trace ID cannot be empty")
	}

	query := `DELETE FROM message_tracking WHERE message_trace_id = ?`

	_, err := s.db.ExecContext(ctx, query, messageTraceId)
	if err != nil {
		return fmt.Errorf("failed to delete message tracking: %w", err)
	}

	return nil
}
