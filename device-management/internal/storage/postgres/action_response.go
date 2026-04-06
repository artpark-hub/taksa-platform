package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

type ActionResponseStore struct {
	db *sql.DB
}

func (s *ActionResponseStore) Save(ctx context.Context, response *models.ActionResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	query := `
		INSERT INTO action_responses (id, action_id, device_id, message_trace_id, content, status, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	completedAt := ""
	if !response.CompletedAt.IsZero() {
		completedAt = response.CompletedAt.Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, query,
		response.ID, response.ActionID, response.DeviceID,
		response.MessageTraceID, response.Content,
		response.Status, completedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save action response: %w", err)
	}

	return nil
}

func (s *ActionResponseStore) GetByActionID(ctx context.Context, actionID string) ([]*models.ActionResponse, error) {
	if actionID == "" {
		return nil, fmt.Errorf("action ID cannot be empty")
	}

	query := `
		SELECT id, action_id, device_id, message_trace_id, content, status, completed_at
		FROM action_responses
		WHERE action_id = $1
		ORDER BY completed_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query action responses: %w", err)
	}
	defer rows.Close()

	var responses []*models.ActionResponse

	for rows.Next() {
		var response models.ActionResponse
		var completedAtStr string

		err := rows.Scan(
			&response.ID, &response.ActionID, &response.DeviceID,
			&response.MessageTraceID, &response.Content,
			&response.Status, &completedAtStr,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan action response: %w", err)
		}

		if completedAtStr != "" {
			completedAt, err := time.Parse(time.RFC3339, completedAtStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse completed_at %q for action response %q: %w", completedAtStr, response.ID, err)
			}
			response.CompletedAt = completedAt
		}

		responses = append(responses, &response)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating action response rows: %w", err)
	}

	return responses, nil
}

func (s *ActionResponseStore) GetByTraceID(ctx context.Context, messageTraceID string) (*models.ActionResponse, error) {
	if messageTraceID == "" {
		return nil, fmt.Errorf("message trace ID cannot be empty")
	}

	query := `
		SELECT id, action_id, device_id, message_trace_id, content, status, completed_at
		FROM action_responses
		WHERE message_trace_id = $1
	`

	var response models.ActionResponse
	var completedAtStr string

	err := s.db.QueryRowContext(ctx, query, messageTraceID).Scan(
		&response.ID, &response.ActionID, &response.DeviceID,
		&response.MessageTraceID, &response.Content,
		&response.Status, &completedAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query action response: %w", err)
	}

	if completedAtStr != "" {
		completedAt, err := time.Parse(time.RFC3339, completedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse completed_at %q for action response %q: %w", completedAtStr, response.ID, err)
		}
		response.CompletedAt = completedAt
	}

	return &response, nil
}

func (s *ActionResponseStore) GetByDeviceID(ctx context.Context, deviceID string) ([]*models.ActionResponse, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	query := `
		SELECT id, action_id, device_id, message_trace_id, content, status, completed_at
		FROM action_responses
		WHERE device_id = $1
		ORDER BY completed_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query action responses: %w", err)
	}
	defer rows.Close()

	var responses []*models.ActionResponse

	for rows.Next() {
		var response models.ActionResponse
		var completedAtStr string

		err := rows.Scan(
			&response.ID, &response.ActionID, &response.DeviceID,
			&response.MessageTraceID, &response.Content,
			&response.Status, &completedAtStr,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan action response: %w", err)
		}

		if completedAtStr != "" {
			completedAt, err := time.Parse(time.RFC3339, completedAtStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse completed_at %q for action response %q: %w", completedAtStr, response.ID, err)
			}
			response.CompletedAt = completedAt
		}

		responses = append(responses, &response)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating action response rows: %w", err)
	}

	return responses, nil
}

func (s *ActionResponseStore) Delete(ctx context.Context, responseID string) error {
	if responseID == "" {
		return fmt.Errorf("response ID cannot be empty")
	}

	query := `DELETE FROM action_responses WHERE id = $1`

	_, err := s.db.ExecContext(ctx, query, responseID)
	if err != nil {
		return fmt.Errorf("failed to delete action response: %w", err)
	}

	return nil
}
