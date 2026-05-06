package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// MessageStore implements storage.MessageStore for PostgreSQL
type MessageStore struct {
	db *sql.DB
}

// Save persists a message to storage with tenant isolation
func (s *MessageStore) Save(ctx context.Context, tenantID string, message *models.Message) error {
	if message == nil || message.DeviceID == "" || tenantID == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO messages (
		id, device_id, tenant_id, message_type, content,
		trace_id, request_id, correlation_id, direction,
		created_at, expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	directionValue := int32(message.Direction)
	_, err := s.db.ExecContext(ctx, query,
		message.ID, message.DeviceID, tenantID, message.Type,
		message.Content,
		"", "", "", directionValue,
		message.CreatedAt.Format(time.RFC3339),
		optionalTimeValue(message.ExpiresAt),
	)

	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to save message: %w", err)
	}

	return nil
}

// GetByID retrieves a message by its ID with tenant isolation
func (s *MessageStore) GetByID(ctx context.Context, id string) (*models.Message, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	message := &models.Message{}
	var createdAt, expiresAt sql.NullString

	var row *sql.Row
	if tenantID != "" {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, device_id, message_type, content, created_at, expires_at
			 FROM messages WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	} else {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, device_id, message_type, content, created_at, expires_at
			 FROM messages WHERE id = $1`, id)
	}

	err := row.Scan(
		&message.ID, &message.DeviceID, &message.Type,
		&message.Content,
		&createdAt, &expiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Metadata is not currently used in generated protobuf
	// if metadataJSON.Valid && metadataJSON.String != "" {
	// 	message.Metadata = ... need to deserialize properly
	// }

	// Parse timestamps
	if createdAt.Valid {
		t, _ := time.Parse(time.RFC3339, createdAt.String)
		message.CreatedAt = t
	}
	if expiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAt.String)
		message.ExpiresAt = t
	}

	return message, nil
}

// GetByDeviceID retrieves all messages for a device with tenant isolation
func (s *MessageStore) GetByDeviceID(ctx context.Context, deviceID string) ([]*models.Message, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		return s.listMessages(ctx,
			"WHERE device_id = $1 AND tenant_id = $2 ORDER BY created_at DESC", deviceID, tenantID)
	}
	return s.listMessages(ctx,
		"WHERE device_id = $1 ORDER BY created_at DESC", deviceID)
}

// ListHistory retrieves message history with filtering and pagination with tenant isolation
func (s *MessageStore) ListHistory(ctx context.Context, filters *storage.MessageListFilter) ([]*models.Message, int32, error) {
	if filters == nil || filters.DeviceID == "" {
		return nil, 0, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	whereClause := "WHERE device_id = $1"
	args := []interface{}{filters.DeviceID}
	argIndex := 2
	if tenantID != "" {
		whereClause += fmt.Sprintf(" AND tenant_id = $%d", argIndex)
		args = append(args, tenantID)
		argIndex++
	}

	// Add message type filter
	if filters.MessageType != "" {
		whereClause += fmt.Sprintf(" AND message_type = $%d", argIndex)
		args = append(args, filters.MessageType)
		argIndex++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM messages " + whereClause
	var total int32
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	// Pagination
	if filters.Page == 0 {
		filters.Page = 1
	}
	if filters.PageSize == 0 {
		filters.PageSize = 50
	}

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

	query := fmt.Sprintf("SELECT id, device_id, message_type, content, created_at, expires_at "+
		"FROM messages %s ORDER BY %s LIMIT $%d OFFSET $%d",
		whereClause, orderBy, argIndex, argIndex+1)

	args = append(args, filters.PageSize, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		message := &models.Message{}
		var createdAt, expiresAt sql.NullString

		err := rows.Scan(
			&message.ID, &message.DeviceID, &message.Type,
			&message.Content,
			&createdAt, &expiresAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan message: %w", err)
		}

		// Metadata is not currently used in generated protobuf
		// if metadataJSON.Valid && metadataJSON.String != "" {
		// 	message.Metadata = ... need to deserialize properly
		// }

		// Parse timestamps
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			message.CreatedAt = t
		}
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			message.ExpiresAt = t
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}

	return messages, total, nil
}

// GetRecentByDevice retrieves recent messages for a device with tenant isolation
func (s *MessageStore) GetRecentByDevice(ctx context.Context, deviceID string, limit int32) ([]*models.Message, error) {
	if deviceID == "" {
		return nil, ErrInvalidInput
	}

	if limit <= 0 {
		limit = 10
	}

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		return s.listMessages(ctx,
			"WHERE device_id = $1 AND tenant_id = $2 ORDER BY created_at DESC LIMIT $3",
			deviceID, tenantID, limit)
	}
	return s.listMessages(ctx,
		"WHERE device_id = $1 ORDER BY created_at DESC LIMIT $2",
		deviceID, limit)
}

// Delete removes a message by ID with tenant isolation
func (s *MessageStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM messages WHERE id = $1"
	args := []interface{}{id}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM messages WHERE id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteByDeviceID removes all messages for a device with tenant isolation
func (s *MessageStore) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM messages WHERE device_id = $1"
	args := []interface{}{deviceID}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM messages WHERE device_id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete messages for device: %w", err)
	}

	return nil
}

// CleanupOld removes messages created before the specified time
func (s *MessageStore) CleanupOld(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM messages WHERE created_at < $1",
		before.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to cleanup old messages: %w", err)
	}

	return nil
}

// CleanupExpired removes all expired messages
func (s *MessageStore) CleanupExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM messages WHERE expires_at < NOW() AND expires_at IS NOT NULL")
	if err != nil {
		return fmt.Errorf("failed to cleanup expired messages: %w", err)
	}

	return nil
}

// CountByDevice returns the number of messages for a device with tenant isolation
func (s *MessageStore) CountByDevice(ctx context.Context, deviceID string) (int32, error) {
	if deviceID == "" {
		return 0, ErrInvalidInput
	}

	query := "SELECT COUNT(*) FROM messages WHERE device_id = $1"
	args := []interface{}{deviceID}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "SELECT COUNT(*) FROM messages WHERE device_id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	var count int32
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}

	return count, nil
}

// CountByDeviceAndDirection returns the number of messages by direction with tenant isolation
func (s *MessageStore) CountByDeviceAndDirection(ctx context.Context, deviceID string, direction int32) (int32, error) {
	if deviceID == "" {
		return 0, ErrInvalidInput
	}

	query := "SELECT COUNT(*) FROM messages WHERE device_id = $1 AND direction = $2"
	args := []interface{}{deviceID, direction}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "SELECT COUNT(*) FROM messages WHERE device_id = $1 AND tenant_id = $2 AND direction = $3"
		args = []interface{}{deviceID, tenantID, direction}
	}
	var count int32
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}

	return count, nil
}

// listMessages is a helper to retrieve messages with a where clause
func (s *MessageStore) listMessages(ctx context.Context, whereClause string, args ...interface{}) ([]*models.Message, error) {
	query := `
	SELECT id, device_id, message_type, content, created_at, expires_at
	FROM messages ` + whereClause

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		message := &models.Message{}
		var createdAt, expiresAt sql.NullString

		err := rows.Scan(
			&message.ID, &message.DeviceID, &message.Type,
			&message.Content,
			&createdAt, &expiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// Metadata is not currently used in generated protobuf
		// if metadataJSON.Valid && metadataJSON.String != "" {
		// 	message.Metadata = ... need to deserialize properly
		// }

		// Parse timestamps
		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			message.CreatedAt = t
		}
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			message.ExpiresAt = t
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return messages, nil
}

// GetByTraceID retrieves messages by trace ID with tenant isolation
func (s *MessageStore) GetByTraceID(ctx context.Context, traceID string) ([]*models.Message, error) {
	if traceID == "" {
		return nil, ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	query := `
	SELECT id, device_id, message_type, content,
	       trace_id, request_id, correlation_id, direction,
	       created_at, expires_at
	FROM messages
	WHERE trace_id = $1
	`
	args := []interface{}{traceID}
	if tenantID != "" {
		query = `
		SELECT id, device_id, message_type, content,
		       trace_id, request_id, correlation_id, direction,
		       created_at, expires_at
		FROM messages
		WHERE trace_id = $1 AND tenant_id = $2
		`
		args = append(args, tenantID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages by trace_id: %w", err)
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		message := &models.Message{}
		var createdAt, expiresAt sql.NullString
		var direction int32

		err := rows.Scan(
			&message.ID, &message.DeviceID, &message.Type,
			&message.Content,
			&message.TraceID, &message.RequestID, &message.CorrelationID,
			&direction,
			&createdAt, &expiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		message.Direction = models.MessageDirection(direction)

		if createdAt.Valid {
			t, _ := time.Parse(time.RFC3339, createdAt.String)
			message.CreatedAt = t
		}
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			message.ExpiresAt = t
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return messages, nil
}
