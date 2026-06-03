package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

const (
	perActionTTLExpiredMessage = "Per-action TTL exceeded"
	autoExpireQueuedMessage    = "Queued action auto-expired (device did not pull in time)"
)

// ActionStore implements storage.ActionStore for PostgreSQL
type ActionStore struct {
	db *sql.DB
}

// Save persists an action to storage
func (s *ActionStore) Save(ctx context.Context, tenantID string, action *models.Action) error {
	if action == nil || action.DeviceId == "" || tenantID == "" {
		return ErrInvalidInput
	}

	// Marshal payload
	var payloadType, payloadData string
	if action.Payload != nil {
		payloadType = action.Payload.TypeUrl
		payloadData = string(action.Payload.Value)
	}

	args := []interface{}{
		action.Id, tenantID, action.DeviceId, action.Type,
		payloadType, payloadData,
		action.MaxRetries, action.RetryCount, int32(action.Status),
		action.CreatedAt.Format(time.RFC3339),
		optionalTimeValue(action.ExpiresAt),
	}

	// For subscribe actions we want DB-level idempotency across replicas: only one QUEUED subscribe per device.
	// Predicate must exactly match partial unique index uq_actions_subscribe_queued in db/schema.postgres.sql.
	if action.Type == "subscribe" && action.Status == models.ActionStatusQueued {
		query := `
	INSERT INTO actions (
		id, tenant_id, device_id, action_type, payload_type, payload_data,
		max_retries, retry_count, status, created_at, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	ON CONFLICT (tenant_id, device_id, action_type)
	WHERE action_type = 'subscribe' AND status = 1
	DO NOTHING
	`
		result, err := s.db.ExecContext(ctx, query, args...)
		if err == nil {
			rows, raErr := result.RowsAffected()
			if raErr == nil && rows == 0 {
				return ErrAlreadyExists
			}
			return nil
		}
		// Rolling deploy before migration, or mismatched DDL: Postgres returns 42P10 when no matching constraint exists.
		if pqErrCode(err) == "42P10" {
			return s.saveActionPlainInsert(ctx, args)
		}
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to save action: %w", err)
	}

	return s.saveActionPlainInsert(ctx, args)
}

func (s *ActionStore) saveActionPlainInsert(ctx context.Context, args []interface{}) error {
	query := `
	INSERT INTO actions (
		id, tenant_id, device_id, action_type, payload_type, payload_data,
		max_retries, retry_count, status, created_at, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to save action: %w", err)
	}
	return nil
}

func pqErrCode(err error) string {
	for err != nil {
		var pe *pq.Error
		if errors.As(err, &pe) {
			return string(pe.Code)
		}
		err = errors.Unwrap(err)
	}
	return ""
}

// GetByID retrieves an action by its ID
func (s *ActionStore) GetByID(ctx context.Context, tenantID, id string) (*models.Action, error) {
	if id == "" || tenantID == "" {
		return nil, ErrInvalidInput
	}

	action := &models.Action{}
	var payloadType, payloadData string
	var status int32
	var createdAt, expiresAt, deliveredAt, completedAt sql.NullString
	var errorMessage sql.NullString

	row := s.db.QueryRowContext(ctx,
		`SELECT id, device_id, action_type, payload_type, payload_data,
		        max_retries, retry_count, status,
		        created_at, expires_at, delivered_at, completed_at,
		        error_message
		 FROM actions WHERE tenant_id = $1 AND id = $2`, tenantID, id)

	err := row.Scan(
		&action.Id, &action.DeviceId, &action.Type,
		&payloadType, &payloadData,
		&action.MaxRetries, &action.RetryCount, &status,
		&createdAt, &expiresAt, &deliveredAt, &completedAt,
		&errorMessage,
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
	if errorMessage.Valid {
		action.ErrorMessage = errorMessage.String
	}

	return action, nil
}

// GetByDeviceID retrieves all actions for a device
func (s *ActionStore) GetByDeviceID(ctx context.Context, tenantID, deviceID string) ([]*models.Action, error) {
	if deviceID == "" || tenantID == "" {
		return nil, ErrInvalidInput
	}

	return s.listActions(ctx,
		"WHERE tenant_id = $1 AND device_id = $2 ORDER BY created_at DESC", tenantID, deviceID)
}

// ListForDevice retrieves actions for a device with filtering
func (s *ActionStore) ListForDevice(ctx context.Context, deviceID string, filters *storage.ActionListFilter) ([]*models.Action, int32, error) {
	if deviceID == "" || filters == nil || filters.TenantID == "" {
		return nil, 0, ErrInvalidInput
	}

	whereClause := "WHERE tenant_id = $1 AND device_id = $2"
	args := []interface{}{filters.TenantID, deviceID}
	paramCounter := 3

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
		created_at, expires_at, delivered_at, completed_at,
		error_message
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
		var errorMessage sql.NullString

		err := rows.Scan(
			&action.Id, &action.DeviceId, &action.Type,
			&payloadType, &payloadData,
			&action.MaxRetries, &action.RetryCount, &status,
			&createdAt, &expiresAt, &deliveredAt, &completedAt,
			&errorMessage,
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
		if errorMessage.Valid {
			action.ErrorMessage = errorMessage.String
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
func (s *ActionStore) ListPending(ctx context.Context, tenantID, deviceID string) ([]*models.Action, error) {
	if deviceID == "" || tenantID == "" {
		return nil, ErrInvalidInput
	}

	return s.listActions(ctx,
		"WHERE tenant_id = $1 AND device_id = $2 AND status = $3 ORDER BY created_at ASC",
		tenantID, deviceID, int32(models.ActionStatusQueued))
}

// ExpireQueuedPastDeadline marks QUEUED actions whose per-action expires_at has passed as EXPIRED.
func (s *ActionStore) ExpireQueuedPastDeadline(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE actions
SET status = $1, completed_at = CURRENT_TIMESTAMP, error_message = $2
WHERE status = $3
  AND expires_at IS NOT NULL
  AND expires_at < CURRENT_TIMESTAMP`,
		int32(models.ActionStatusExpired), perActionTTLExpiredMessage, int32(models.ActionStatusQueued))
	if err != nil {
		return fmt.Errorf("failed to expire queued actions past deadline: %w", err)
	}
	return nil
}

// ExpireQueuedOlderThan marks stale QUEUED actions as EXPIRED (cross-tenant auto-expire sweep).
// Infrastructure actions (subscribe, UNS→NATS mirror deploy/edit) are excluded; they use per-action TTL
// and dedicated re-queue paths (pull staleness, login, fleet reconcile).
func (s *ActionStore) ExpireQueuedOlderThan(ctx context.Context, before time.Time, errorMessage string) error {
	if errorMessage == "" {
		errorMessage = autoExpireQueuedMessage
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE actions
SET status = $1, completed_at = CURRENT_TIMESTAMP, error_message = $2
WHERE status = $3 AND created_at < $4
  AND action_type <> $5
  AND NOT (
    action_type IN ('deploy-data-flow-component', 'edit-data-flow-component')
    AND payload_data::jsonb ->> 'name' = $6
  )`,
		int32(models.ActionStatusExpired), errorMessage, int32(models.ActionStatusQueued), before.Format(time.RFC3339),
		models.ActionTypeSubscribe, models.NATSMirrorPayloadMarker)
	if err != nil {
		return fmt.Errorf("failed to auto-expire queued actions: %w", err)
	}
	return nil
}

// ClaimQueuedForDevice atomically claims QUEUED actions for delivery (QUEUED → DELIVERED).
func (s *ActionStore) ClaimQueuedForDevice(ctx context.Context, tenantID, deviceID string) ([]*models.Action, error) {
	if deviceID == "" || tenantID == "" {
		return nil, ErrInvalidInput
	}

	query := `
WITH to_claim AS (
  SELECT id FROM actions
  WHERE tenant_id = $2 AND device_id = $3 AND status = $4
    AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
  ORDER BY created_at ASC
),
updated AS (
  UPDATE actions AS a
  SET status = $1, delivered_at = CURRENT_TIMESTAMP
  FROM to_claim
  WHERE a.id = to_claim.id
  RETURNING a.id, a.device_id, a.action_type, a.payload_type, a.payload_data,
            a.max_retries, a.retry_count, a.status,
            a.created_at, a.expires_at, a.delivered_at, a.completed_at,
            a.error_message
)
SELECT id, device_id, action_type, payload_type, payload_data,
       max_retries, retry_count, status,
       created_at, expires_at, delivered_at, completed_at,
       error_message
FROM updated
ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query,
		int32(models.ActionStatusDelivered), tenantID, deviceID, int32(models.ActionStatusQueued))
	if err != nil {
		return nil, fmt.Errorf("failed to claim queued actions: %w", err)
	}
	defer rows.Close()

	return scanActionRows(rows)
}

// CancelQueued atomically cancels a QUEUED action for the given device.
func (s *ActionStore) CancelQueued(ctx context.Context, tenantID, deviceID, id, errorMessage string) error {
	if id == "" || tenantID == "" || deviceID == "" {
		return ErrInvalidInput
	}
	if errorMessage == "" {
		errorMessage = "Cancelled by user"
	}

	result, err := s.db.ExecContext(ctx, `
UPDATE actions
SET status = $1, completed_at = CURRENT_TIMESTAMP, error_message = $2
WHERE tenant_id = $3 AND device_id = $4 AND id = $5 AND status = $6`,
		int32(models.ActionStatusCancelled), errorMessage, tenantID, deviceID, id, int32(models.ActionStatusQueued))
	if err != nil {
		return fmt.Errorf("failed to cancel action: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to cancel action: %w", err)
	}
	if rows == 0 {
		return storage.ErrActionNotCancellable
	}
	return nil
}

// UpdateStatus updates an action's status
func (s *ActionStore) UpdateStatus(ctx context.Context, tenantID, id string, status models.ActionStatus) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE actions SET status = $1 WHERE tenant_id = $2 AND id = $3", int32(status), tenantID, id)
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
func (s *ActionStore) UpdateErrorMessage(ctx context.Context, tenantID, id string, errorMessage string) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE actions SET error_message = $1 WHERE tenant_id = $2 AND id = $3", errorMessage, tenantID, id)
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
func (s *ActionStore) MarkDelivered(ctx context.Context, tenantID, id string) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE actions 
		 SET status = $1, delivered_at = CURRENT_TIMESTAMP 
		 WHERE tenant_id = $2 AND id = $3 AND status = $4`,
		int32(models.ActionStatusDelivered), tenantID, id, int32(models.ActionStatusQueued))
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
func (s *ActionStore) MarkCompleted(ctx context.Context, tenantID, id string) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE actions 
		 SET status = $1, completed_at = CURRENT_TIMESTAMP 
		 WHERE tenant_id = $2 AND id = $3`,
		int32(models.ActionStatusCompleted), tenantID, id)
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
func (s *ActionStore) MarkFailed(ctx context.Context, tenantID, id string) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE actions 
		 SET status = $1, completed_at = CURRENT_TIMESTAMP 
		 WHERE tenant_id = $2 AND id = $3`,
		int32(models.ActionStatusFailed), tenantID, id)
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
func (s *ActionStore) IncrementRetry(ctx context.Context, tenantID, id string) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE actions SET retry_count = retry_count + 1 WHERE tenant_id = $1 AND id = $2", tenantID, id)
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
func (s *ActionStore) Delete(ctx context.Context, tenantID, id string) error {
	if id == "" || tenantID == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM actions WHERE tenant_id = $1 AND id = $2", tenantID, id)
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
func (s *ActionStore) DeleteByDeviceID(ctx context.Context, tenantID, deviceID string) error {
	if deviceID == "" || tenantID == "" {
		return ErrInvalidInput
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM actions WHERE tenant_id = $1 AND device_id = $2", tenantID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete actions for device: %w", err)
	}

	return nil
}

// HasRecentSubscribeForDevice reports whether a subscribe action was created at or after since.
func (s *ActionStore) HasRecentSubscribeForDevice(ctx context.Context, tenantID, deviceID string, since time.Time) (bool, error) {
	if tenantID == "" || deviceID == "" {
		return false, ErrInvalidInput
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM actions
  WHERE tenant_id = $1 AND device_id = $2 AND action_type = 'subscribe'
    AND created_at >= $3
)`, tenantID, deviceID, since).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has recent subscribe: %w", err)
	}
	return exists, nil
}

// DeleteQueuedSubscribe removes the QUEUED subscribe action for a device so a new one can be inserted.
func (s *ActionStore) DeleteQueuedSubscribe(ctx context.Context, tenantID, deviceID string) (bool, error) {
	if tenantID == "" || deviceID == "" {
		return false, ErrInvalidInput
	}
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM actions WHERE tenant_id = $1 AND device_id = $2 AND action_type = 'subscribe' AND status = $3`,
		tenantID, deviceID, int32(models.ActionStatusQueued))
	if err != nil {
		return false, fmt.Errorf("delete queued subscribe: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
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

// CleanupTerminal removes terminal actions older than before.
// NOTE: This does not currently attempt to preserve actions for audit/history.
// It is intended for aggressive cleanup after the async UI loop completes.
func (s *ActionStore) CleanupTerminal(ctx context.Context, before time.Time) error {
	// Terminal statuses:
	// 4=COMPLETED, 5=FAILED, 6=EXPIRED, 7=CANCELLED, 8=FAILED_PARSING_RESPONSE
	//
	// Use a stable "terminal timestamp" so we can clean up even when completed_at is missing:
	// completed_at (when present) > delivered_at > created_at.
	_, err := s.db.ExecContext(ctx, `
DELETE FROM actions
WHERE status IN (4, 5, 6, 7, 8)
  AND COALESCE(completed_at, delivered_at, created_at) < $1
`, before.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to cleanup terminal actions: %w", err)
	}
	return nil
}

// listActions is a helper to retrieve actions with a where clause
func (s *ActionStore) listActions(ctx context.Context, whereClause string, args ...interface{}) ([]*models.Action, error) {
	query := `
	SELECT id, device_id, action_type, payload_type, payload_data,
	       max_retries, retry_count, status,
	       created_at, expires_at, delivered_at, completed_at,
	       error_message
	FROM actions ` + whereClause

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query actions: %w", err)
	}
	defer rows.Close()

	return scanActionRows(rows)
}

func scanActionRows(rows *sql.Rows) ([]*models.Action, error) {
	var actions []*models.Action
	for rows.Next() {
		action, err := scanActionRow(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return actions, nil
}

func scanActionRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.Action, error) {
	action := &models.Action{}
	var payloadType, payloadData string
	var status int32
	var createdAt, expiresAt, deliveredAt, completedAt sql.NullString
	var errorMessage sql.NullString

	err := scanner.Scan(
		&action.Id, &action.DeviceId, &action.Type,
		&payloadType, &payloadData,
		&action.MaxRetries, &action.RetryCount, &status,
		&createdAt, &expiresAt, &deliveredAt, &completedAt,
		&errorMessage,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan action: %w", err)
	}

	if payloadType != "" && payloadData != "" {
		action.Payload = &anypb.Any{
			TypeUrl: payloadType,
			Value:   []byte(payloadData),
		}
	}

	action.Status = models.ActionStatus(status)

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
	if errorMessage.Valid {
		action.ErrorMessage = errorMessage.String
	}

	return action, nil
}

// NATSMirrorDeployInflight reports whether a UNS-to-NATS mirror deploy is queued or delivered.
func (s *ActionStore) NATSMirrorDeployInflight(ctx context.Context, tenantID, deviceID string) (bool, error) {
	return s.natsMirrorInflight(ctx, tenantID, deviceID, "deploy-data-flow-component")
}

// NATSMirrorActionInflight reports whether a UNS-to-NATS mirror deploy or edit is queued, delivered, or processing.
func (s *ActionStore) NATSMirrorActionInflight(ctx context.Context, tenantID, deviceID string) (bool, error) {
	if tenantID == "" || deviceID == "" {
		return false, ErrInvalidInput
	}
	const (
		deployType = "deploy-data-flow-component"
		editType   = "edit-data-flow-component"
	)
	var inflight bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM actions
  WHERE tenant_id = $1 AND device_id = $2
    AND action_type IN ($3, $4)
    AND payload_data::jsonb ->> 'name' = $5
    AND status IN ($6, $7, $8)
)`, tenantID, deviceID, deployType, editType, models.NATSMirrorPayloadMarker,
		int(models.ActionStatusQueued),
		int(models.ActionStatusDelivered),
		int(models.ActionStatusProcessing),
	).Scan(&inflight)
	if err != nil {
		return false, fmt.Errorf("nats mirror action inflight query: %w", err)
	}
	return inflight, nil
}

func (s *ActionStore) natsMirrorInflight(ctx context.Context, tenantID, deviceID, actionType string) (bool, error) {
	if tenantID == "" || deviceID == "" || actionType == "" {
		return false, ErrInvalidInput
	}
	var inflight bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM actions
  WHERE tenant_id = $1 AND device_id = $2
    AND action_type = $3
    AND payload_data::jsonb ->> 'name' = $4
    AND status IN ($5, $6, $7)
)`, tenantID, deviceID, actionType, models.NATSMirrorPayloadMarker,
		int(models.ActionStatusQueued),
		int(models.ActionStatusDelivered),
		int(models.ActionStatusProcessing),
	).Scan(&inflight)
	if err != nil {
		return false, fmt.Errorf("nats mirror inflight query: %w", err)
	}
	return inflight, nil
}
