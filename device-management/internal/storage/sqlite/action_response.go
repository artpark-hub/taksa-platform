package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"taksa-platform-dm/internal/models"
	"taksa-platform-dm/internal/storage"
)

type actionResponseStore struct {
	db *sql.DB
}

// NewActionResponseStore creates a new action response store
func newActionResponseStore(db *sql.DB) storage.ActionResponseStore {
	return &actionResponseStore{db: db}
}

// Save stores an action response
func (s *actionResponseStore) Save(ctx context.Context, response *models.ActionResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	query := `
		INSERT INTO action_responses (id, action_id, device_id, message_trace_id, content, status, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	completedAt := ""
	if !response.CompletedAt.IsZero() {
		completedAt = response.CompletedAt.Format("2006-01-02 15:04:05")
	}

	_, err := s.db.ExecContext(ctx, query,
		response.ID,
		response.ActionID,
		response.DeviceID,
		response.MessageTraceID,
		response.Content,
		response.Status,
		completedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save action response: %w", err)
	}

	return nil
}

// GetByActionId retrieves all responses for an action
func (s *actionResponseStore) GetByActionId(ctx context.Context, actionId string) ([]*models.ActionResponse, error) {
	if actionId == "" {
		return nil, fmt.Errorf("action ID cannot be empty")
	}

	query := `
		SELECT id, action_id, device_id, message_trace_id, content, status, completed_at
		FROM action_responses
		WHERE action_id = ?
		ORDER BY completed_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, actionId)
	if err != nil {
		return nil, fmt.Errorf("failed to query action responses: %w", err)
	}
	defer rows.Close()

	var responses []*models.ActionResponse

	for rows.Next() {
		var response models.ActionResponse
		var completedAtStr string

		err := rows.Scan(
			&response.ID,
			&response.ActionID,
			&response.DeviceID,
			&response.MessageTraceID,
			&response.Content,
			&response.Status,
			&completedAtStr,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan action response: %w", err)
		}

		if completedAtStr != "" {
			completedAt, _ := time.Parse("2006-01-02 15:04:05", completedAtStr)
			response.CompletedAt = completedAt
		}

		responses = append(responses, &response)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating action response rows: %w", err)
	}

	return responses, nil
}

// GetByTraceId retrieves response by message trace ID
func (s *actionResponseStore) GetByTraceId(ctx context.Context, messageTraceId string) (*models.ActionResponse, error) {
	if messageTraceId == "" {
		return nil, fmt.Errorf("message trace ID cannot be empty")
	}

	query := `
		SELECT id, action_id, device_id, message_trace_id, content, status, completed_at
		FROM action_responses
		WHERE message_trace_id = ?
	`

	var response models.ActionResponse
	var completedAtStr string

	err := s.db.QueryRowContext(ctx, query, messageTraceId).Scan(
		&response.ID,
		&response.ActionID,
		&response.DeviceID,
		&response.MessageTraceID,
		&response.Content,
		&response.Status,
		&completedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query action response: %w", err)
	}

	if completedAtStr != "" {
		completedAt, _ := time.Parse("2006-01-02 15:04:05", completedAtStr)
		response.CompletedAt = completedAt
	}

	return &response, nil
}

// GetByDeviceId retrieves all responses from a device
func (s *actionResponseStore) GetByDeviceId(ctx context.Context, deviceId string) ([]*models.ActionResponse, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	query := `
		SELECT id, action_id, device_id, message_trace_id, content, status, completed_at
		FROM action_responses
		WHERE device_id = ?
		ORDER BY completed_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceId)
	if err != nil {
		return nil, fmt.Errorf("failed to query action responses: %w", err)
	}
	defer rows.Close()

	var responses []*models.ActionResponse

	for rows.Next() {
		var response models.ActionResponse
		var completedAtStr string

		err := rows.Scan(
			&response.ID,
			&response.ActionID,
			&response.DeviceID,
			&response.MessageTraceID,
			&response.Content,
			&response.Status,
			&completedAtStr,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan action response: %w", err)
		}

		if completedAtStr != "" {
			completedAt, _ := time.Parse("2006-01-02 15:04:05", completedAtStr)
			response.CompletedAt = completedAt
		}

		responses = append(responses, &response)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating action response rows: %w", err)
	}

	return responses, nil
}

// Delete removes an action response
func (s *actionResponseStore) Delete(ctx context.Context, responseId string) error {
	if responseId == "" {
		return fmt.Errorf("response ID cannot be empty")
	}

	query := `DELETE FROM action_responses WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, responseId)
	if err != nil {
		return fmt.Errorf("failed to delete action response: %w", err)
	}

	return nil
}
