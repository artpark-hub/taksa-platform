package data

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	uuidgen "github.com/google/uuid"
)

// convertPlaceholdersSP converts SQLite ? placeholders to PostgreSQL $1, $2 style
func convertPlaceholdersSP(query string) string {
	counter := 1
	return regexp.MustCompile(`\?`).ReplaceAllStringFunc(query, func(string) string {
		placeholder := fmt.Sprintf("$%d", counter)
		counter++
		return placeholder
	})
}

// StreamProcessorRepo handles database operations for stream processors
type StreamProcessorRepo struct {
	data *Data
}

// StreamProcessorModel represents a stream processor in the database
type StreamProcessorModel struct {
	ID                string
	DeviceID          string
	UUID              string
	Name              string
	ModelName         sql.NullString
	ModelVersion      sql.NullString
	EncodedConfig     sql.NullString
	LocationJSON      sql.NullString // JSON map of location levels
	IgnoreHealthCheck bool
	MetadataJSON      sql.NullString // JSON map of metadata
	CreatedAt         sql.NullString // Store as TEXT for SQLite compatibility
	UpdatedAt         sql.NullString // Store as TEXT for SQLite compatibility
}

// Insert creates a new stream processor record
func (r *StreamProcessorRepo) Insert(ctx context.Context, deviceID, uuid, name string) error {
	query := `
		INSERT INTO stream_processors (
			id, device_id, uuid, name,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	id := uuidgen.New().String()
	now := time.Now()

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersSP(query)

	_, err := r.data.db.ExecContext(ctx, query,
		id, deviceID, uuid, name, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert stream processor: %w", err)
	}

	return nil
}

// Update updates an existing stream processor record
func (r *StreamProcessorRepo) Update(ctx context.Context, deviceID, uuid string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
	updateClause := "updated_at = ?"
	args := []interface{}{time.Now()}

	for key, val := range updates {
		updateClause += ", " + key + " = ?"
		args = append(args, val)
	}

	args = append(args, deviceID, uuid)

	query := fmt.Sprintf(`
		UPDATE stream_processors 
		SET %s
		WHERE device_id = ? AND uuid = ?
	`, updateClause)

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersSP(query)

	_, err := r.data.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update stream processor: %w", err)
	}

	return nil
}

// GetByUUID retrieves a stream processor by UUID
func (r *StreamProcessorRepo) GetByUUID(ctx context.Context, deviceID, uuid string) (*StreamProcessorModel, error) {
	query := `
		SELECT id, device_id, uuid, name, model_name, model_version,
		       encoded_config, location_json, ignore_health_check, metadata_json,
		       created_at, updated_at
		FROM stream_processors
		WHERE device_id = ? AND uuid = ?
	`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersSP(query)

	row := r.data.db.QueryRowContext(ctx, query, deviceID, uuid)

	var sp StreamProcessorModel
	var ignoreHealthCheck sql.NullBool // Handle both SQLite INT and PostgreSQL BOOLEAN
	err := row.Scan(
		&sp.ID, &sp.DeviceID, &sp.UUID, &sp.Name, &sp.ModelName, &sp.ModelVersion,
		&sp.EncodedConfig, &sp.LocationJSON, &ignoreHealthCheck, &sp.MetadataJSON,
		&sp.CreatedAt, &sp.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get stream processor: %w", err)
	}

	sp.IgnoreHealthCheck = ignoreHealthCheck.Bool

	return &sp, nil
}

// ListQuery represents query parameters for listing stream processors
type StreamProcessorListQuery struct {
	DeviceID   string
	NameFilter string // Substring match
	Offset     int64
	Limit      int64
}

// List retrieves stream processors with optional filtering and pagination
func (r *StreamProcessorRepo) List(ctx context.Context, query *StreamProcessorListQuery) ([]*StreamProcessorModel, error) {
	sqlQuery := `
		SELECT id, device_id, uuid, name, model_name, model_version,
		       encoded_config, location_json, ignore_health_check, metadata_json,
		       created_at, updated_at
		FROM stream_processors
		WHERE device_id = ?
	`

	args := []interface{}{query.DeviceID}

	// Add optional filters
	if query.NameFilter != "" {
		sqlQuery += " AND name LIKE ?"
		args = append(args, "%"+query.NameFilter+"%")
	}

	// Order and paginate
	sqlQuery += " ORDER BY created_at DESC OFFSET ? LIMIT ?"
	args = append(args, query.Offset, query.Limit)

	// Convert placeholders for PostgreSQL compatibility
	sqlQuery = convertPlaceholdersSP(sqlQuery)

	rows, err := r.data.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list stream processors: %w", err)
	}
	defer rows.Close()

	processors := make([]*StreamProcessorModel, 0) // Initialize as empty slice, not nil

	for rows.Next() {
		var sp StreamProcessorModel
		var ignoreHealthCheck sql.NullBool // Handle both SQLite INT and PostgreSQL BOOLEAN
		err := rows.Scan(
			&sp.ID, &sp.DeviceID, &sp.UUID, &sp.Name, &sp.ModelName, &sp.ModelVersion,
			&sp.EncodedConfig, &sp.LocationJSON, &ignoreHealthCheck, &sp.MetadataJSON,
			&sp.CreatedAt, &sp.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stream processor: %w", err)
		}

		sp.IgnoreHealthCheck = ignoreHealthCheck.Bool
		processors = append(processors, &sp)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating stream processors: %w", err)
	}

	return processors, nil
}

// Delete removes a stream processor record
func (r *StreamProcessorRepo) Delete(ctx context.Context, deviceID, uuid string) error {
	query := `DELETE FROM stream_processors WHERE device_id = ? AND uuid = ?`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersSP(query)

	_, err := r.data.db.ExecContext(ctx, query, deviceID, uuid)
	if err != nil {
		return fmt.Errorf("failed to delete stream processor: %w", err)
	}

	return nil
}

// DeleteByDevice removes all stream processors for a device
func (r *StreamProcessorRepo) DeleteByDevice(ctx context.Context, deviceID string) error {
	query := `DELETE FROM stream_processors WHERE device_id = ?`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersSP(query)

	_, err := r.data.db.ExecContext(ctx, query, deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete stream processors for device: %w", err)
	}

	return nil
}

// Upsert creates or updates a stream processor (for sync operations)
func (r *StreamProcessorRepo) Upsert(ctx context.Context, deviceID, uuid, name, modelName, modelVersion, encodedConfig string, ignoreHealthCheck bool, locationJSON, metadataJSON string) error {
	// Check if exists
	existing, err := r.GetByUUID(ctx, deviceID, uuid)
	if err != nil {
		return err
	}

	// Convert bool to int for SQLite
	ignoreHealthCheckInt := 0
	if ignoreHealthCheck {
		ignoreHealthCheckInt = 1
	}

	if existing != nil {
		// Update existing
		return r.Update(ctx, deviceID, uuid, map[string]interface{}{
			"name":                  name,
			"model_name":            modelName,
			"model_version":         modelVersion,
			"encoded_config":        encodedConfig,
			"ignore_health_check":   ignoreHealthCheckInt,
			"location_json":         locationJSON,
			"metadata_json":         metadataJSON,
		})
	}

	// Insert new
	if err := r.Insert(ctx, deviceID, uuid, name); err != nil {
		return err
	}

	// Update with full details
	return r.Update(ctx, deviceID, uuid, map[string]interface{}{
		"model_name":            modelName,
		"model_version":         modelVersion,
		"encoded_config":        encodedConfig,
		"ignore_health_check":   ignoreHealthCheckInt,
		"location_json":         locationJSON,
		"metadata_json":         metadataJSON,
	})
}
