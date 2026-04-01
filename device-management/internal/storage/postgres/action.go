package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	"taksa-platform-dm/internal/models"
	"taksa-platform-dm/internal/storage"
)

// ActionStore implements storage.ActionStore for PostgreSQL
type ActionStore struct {
	db *sql.DB
}

// Save persists an action to storage
func (s *ActionStore) Save(ctx context.Context, action *models.Action) error {
	if action == nil || action.DeviceId == "" {
		return ErrInvalidInput
	}

	// Marshal payload
	var payloadType, payloadData string
	if action.Payload != nil {
		payloadType = action.Payload.TypeUrl
		payloadData = string(action.Payload.Value)
	}

	query := `
	INSERT INTO actions (
		id, device_id, action_type, payload_type, payload_data,
		max_retries, retry_count, status, created_at, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := s.db.ExecContext(ctx, query,
		action.Id, action.DeviceId, action.Type,
		payloadType, payloadData,
		action.MaxRetries, action.RetryCount, int32(action.Status),
		action.CreatedAt.Format(time.RFC3339),
		optionalTimeValue(action.ExpiresAt),
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to save action: %w", err)
	}

	return nil
}

// GetByID retrieves an action by its ID
func (s *ActionStore) GetByID(ctx context.Context, id string) (*models.Action, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}

	action := &models.Action{}
	var payloadType, payloadData string
	var status int32
	var createdAt, expiresAt, deliveredAt, completedAt sql.NullString

	row := s.db.QueryRowContext(ctx,
		`SELECT id, device_id, action_type, payload_type, payload_data,
		        max_retries, retry_count, status,
		        created_at, expires_at, delivered_at, completed_at
		 FROM actions WHERE id = $1`, id)

	err := row.Scan(
		&action.Id, &action.DeviceId, &action.Type,
		&payloadType, &payloadData,
		&action.MaxRetries, &action.RetryCount, &status,
		&createdAt, &expiresAt, &deliveredAt, &completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get action: %w", err)
	}

	// Unmarshal payload
	if payloadType != "" && payloadData != "" {
		action.Payload = &anypb.Any{
			TypeUrl: payloadType,
			Value:   []byte(payloadData),
		}
	}

	// Convert status
	action.Status = models.ActionStatus(status)

	// Parse timestamps
	if createdAt.Valid {
		t, _ := time.Parse(time.RFC3339, createdAt.String)
		action.CreatedAt = t
	}
	if expiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAt.String)
		action.ExpiresAt = t
	}
	if deliveredAt.Valid {
		t, _ := time.Parse(time.RFC3339, deliveredAt.String)
		action.DeliveredAt = t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		action.CompletedAt = t
	}

	return action, nil
}

// GetByDeviceID retrieves all actions for a device
func (s *ActionStore) GetByDeviceID(ctx context.Context, deviceID string) ([]*models.Action, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	return s.listActions(ctx,
		"WHERE device_id = $1 ORDER BY created_at DESC", deviceID)
}

// ListForDevice retrieves actions for a device with filtering
func (s *ActionStore) ListForDevice(ctx context.Context, deviceID string, filters *storage.ActionListFilter) ([]*models.Action, int32, error) {
	if deviceID == "" {
		return nil, 0, ErrInvalidInput
	}

	if filters == nil {
		filters = &storage.ActionListFilter{Page: 1, PageSize: 50}
	}

	whereClause := "WHERE device_id = $1"
	args := []interface{}{deviceID}
	paramCounter := 2

	// Add status filter
	if filters.StatusFilter != nil {
		whereClause += fmt.Sprintf(" AND status = $%d", paramCounter)
		args = append(args, int32(*filters.StatusFilter))
		paramCounter++
	}

	// Add history filter
	if !filters.IncludeHistory {
		whereClause += fmt.Sprintf(" AND status IN ($%d, $%d)", paramCounter, paramCounter+1)
		args = append(args, int32(models.ActionStatusQueued), int32(models.ActionStatusDelivered))
		paramCounter += 2
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM actions " + whereClause
	var total int32
	err := s.db.QueryRowContext(ctx, countQuery, args[:paramCounter-1]...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count actions: %w", err)
	}

	// Pagination
	offset := (filters.Page - 1) * filters.PageSize
	orderBy := "created_at"
	if filters.SortBy != "" {
		orderBy = filters.SortBy
	}
	if filters.SortDesc {
		orderBy += " DESC"
	} else {
		orderBy += " ASC"
	}

	query := fmt.Sprintf(`SELECT id, device_id, action_type, payload_type, payload_data,
		max_retries, retry_count, status,
		created_at, expires_at, delivered_at, completed_at
		FROM actions %s ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		whereClause, orderBy, paramCounter, paramCounter+1)

	args = append(args, filters.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list actions: %w", err)
	}
	defer rows.Close()

	var actions []*models.Action
	for rows.Next() {
		action := &models.Action{}
		var payloadType, payloadData string
		var status int32
		var createdAt, expiresAt, deliveredAt, completedAt sql.NullString

		err := rows.Scan(
			&action.Id, &action.DeviceId, &action.Type,
			&payloadType, &payloadData,
			&action.MaxRetries, &action.RetryCount, &status,
			&createdAt, &expiresAt, &deliveredAt, &completedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan action: %w", err)
		}

		// Unmarshal payload
		if payloadType != "" && payloadData != "" {
			action.Payload = &anypb.Any{
				TypeUrl: payloadType,
				Value:   []byte(payloadData),
			}
		}

		action.Status = models.ActionStatus(status)

		// Parse timestamps
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			action.CreatedAt = t
		}
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			action.ExpiresAt = t
		}
		if deliveredAt.Valid {
			t, _ := time.Parse(time.RFC3339, deliveredAt.String)
			action.DeliveredAt = t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			action.CompletedAt = t
		}

		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return actions, total, nil
}

// ListPending retrieves all pending (QUEUED) actions for a device
// CRITICAL: Used by Pull endpoint
func (s *ActionStore) ListPending(ctx context.Context, deviceID string) ([]*models.Action, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	return s.listActions(ctx,
		"WHERE device_id = $1 AND status = $2 ORDER BY created_at ASC",
		deviceID, int32(models.ActionStatusQueued))
}

// UpdateStatus updates an action's status
func (s *ActionStore) UpdateStatus(ctx context.Context, id string, status models.ActionStatus) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE actions SET status = $1 WHERE id = $2", int32(status), id)
	if err != nil {
		return fmt.Errorf("failed to update action status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateErrorMessage updates the error message for an action
func (s *ActionStore) UpdateErrorMessage(ctx context.Context, id string, errorMessage string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE actions SET error_message = $1 WHERE id = $2", errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to update action error message: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// MarkDelivered marks an action as delivered
func (s *ActionStore) MarkDelivered(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE actions 
		 SET status = $1, delivered_at = CURRENT_TIMESTAMP 
		 WHERE id = $2`,
		int32(models.ActionStatusDelivered), id)
	if err != nil {
		return fmt.Errorf("failed to mark action delivered: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// MarkCompleted marks an action as completed
func (s *ActionStore) MarkCompleted(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE actions 
		 SET status = $1, completed_at = CURRENT_TIMESTAMP 
		 WHERE id = $2`,
		int32(models.ActionStatusCompleted), id)
	if err != nil {
		return fmt.Errorf("failed to mark action completed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// MarkFailed marks an action as failed
func (s *ActionStore) MarkFailed(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE actions 
		 SET status = $1, completed_at = CURRENT_TIMESTAMP 
		 WHERE id = $2`,
		int32(models.ActionStatusFailed), id)
	if err != nil {
		return fmt.Errorf("failed to mark action failed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// IncrementRetry increments the retry count for an action
func (s *ActionStore) IncrementRetry(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE actions SET retry_count = retry_count + 1 WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to increment retry: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes an action
func (s *ActionStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM actions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete action: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDeviceID removes all actions for a device
func (s *ActionStore) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM actions WHERE device_id = $1", deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete actions for device: %w", err)
	}

	return nil
}

// CleanupExpired removes all expired actions
func (s *ActionStore) CleanupExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM actions WHERE expires_at < CURRENT_TIMESTAMP AND expires_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired actions: %w", err)
	}

	return nil
}

// listActions is a helper to retrieve actions with a where clause
func (s *ActionStore) listActions(ctx context.Context, whereClause string, args ...interface{}) ([]*models.Action, error) {
	query := `
	SELECT id, device_id, action_type, payload_type, payload_data,
	       max_retries, retry_count, status,
	       created_at, expires_at, delivered_at, completed_at
	FROM actions ` + whereClause

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query actions: %w", err)
	}
	defer rows.Close()

	var actions []*models.Action
	for rows.Next() {
		action := &models.Action{}
		var payloadType, payloadData string
		var status int32
		var createdAt, expiresAt, deliveredAt, completedAt sql.NullString

		err := rows.Scan(
			&action.Id, &action.DeviceId, &action.Type,
			&payloadType, &payloadData,
			&action.MaxRetries, &action.RetryCount, &status,
			&createdAt, &expiresAt, &deliveredAt, &completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan action: %w", err)
		}

		// Unmarshal payload
		if payloadType != "" && payloadData != "" {
			action.Payload = &anypb.Any{
				TypeUrl: payloadType,
				Value:   []byte(payloadData),
			}
		}

		action.Status = models.ActionStatus(status)

		// Parse timestamps
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			action.CreatedAt = t
		}
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			action.ExpiresAt = t
		}
		if deliveredAt.Valid {
			t, _ := time.Parse(time.RFC3339, deliveredAt.String)
			action.DeliveredAt = t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			action.CompletedAt = t
		}

		actions = append(actions, action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return actions, nil
}
