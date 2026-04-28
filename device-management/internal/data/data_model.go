package data

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

// convertPlaceholders converts SQLite ? placeholders to PostgreSQL $1, $2 style
func convertPlaceholdersDM(query string) string {
	counter := 1
	return regexp.MustCompile(`\?`).ReplaceAllStringFunc(query, func(string) string {
		placeholder := fmt.Sprintf("$%d", counter)
		counter++
		return placeholder
	})
}

// DataModelRepo handles database operations for data models
type DataModelRepo struct {
	data *Data
}

// DataModelModel represents a data model in the database
type DataModelModel struct {
	ID                string
	DeviceID          string
	Name              string
	Version           string
	Description       sql.NullString
	EncodedStructure  sql.NullString
	CreatedAt         sql.NullString // Store as TEXT for SQLite compatibility
	UpdatedAt         sql.NullString // Store as TEXT for SQLite compatibility
}

// Upsert creates or updates a data model (for sync operations from umh-core actions)
func (r *DataModelRepo) Upsert(ctx context.Context, tenantID, deviceID, name, version, description, encodedStructure string) error {
	// Check if exists
	existing, err := r.GetByNameAndVersion(ctx, tenantID, deviceID, name, version)
	if err != nil {
		return err
	}

	if existing != nil {
		// Update existing
		query := `
			UPDATE data_models 
			SET description = ?, encoded_structure = ?, updated_at = ?
			WHERE tenant_id = ? AND device_id = ? AND name = ? AND version = ?
		`

		// Convert placeholders for PostgreSQL compatibility
		query = convertPlaceholdersDM(query)

		_, err := r.data.db.ExecContext(ctx, query,
			description, encodedStructure, time.Now(),
			tenantID, deviceID, name, version)
		if err != nil {
			return fmt.Errorf("failed to update data model: %w", err)
		}

		return nil
	}

	// Insert new
	query := `
		INSERT INTO data_models (tenant_id, device_id, name, version, description, encoded_structure, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersDM(query)

	now := time.Now()
	_, err = r.data.db.ExecContext(ctx, query,
		tenantID, deviceID, name, version, description, encodedStructure, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert data model: %w", err)
	}

	return nil
}

// GetByNameAndVersion retrieves a data model by name and version
func (r *DataModelRepo) GetByNameAndVersion(ctx context.Context, tenantID, deviceID, name, version string) (*DataModelModel, error) {
	query := `
		SELECT id, device_id, name, version, description, encoded_structure, created_at, updated_at
		FROM data_models
		WHERE tenant_id = ? AND device_id = ? AND name = ? AND version = ?
	`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholdersDM(query)

	row := r.data.db.QueryRowContext(ctx, query, tenantID, deviceID, name, version)

	var dm DataModelModel
	err := row.Scan(
		&dm.ID, &dm.DeviceID, &dm.Name, &dm.Version,
		&dm.Description, &dm.EncodedStructure, &dm.CreatedAt, &dm.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get data model: %w", err)
	}

	return &dm, nil
}

// ListQuery represents query parameters for listing data models
type DataModelListQuery struct {
	DeviceID      string
	NameFilter    string // Substring match
	VersionFilter string // Exact match
	Offset        int64
	Limit         int64
}

// List retrieves data models with optional filtering and cursor-based pagination
func (r *DataModelRepo) List(ctx context.Context, tenantID string, query *DataModelListQuery) ([]*DataModelModel, error) {
	sqlQuery := `
		SELECT id, device_id, name, version, description, encoded_structure, created_at, updated_at
		FROM data_models
		WHERE tenant_id = ? AND device_id = ?
	`

	args := []interface{}{tenantID, query.DeviceID}

	// Add optional filters
	if query.NameFilter != "" {
		sqlQuery += " AND name LIKE ?"
		args = append(args, "%"+query.NameFilter+"%")
	}

	if query.VersionFilter != "" {
		sqlQuery += " AND version = ?"
		args = append(args, query.VersionFilter)
	}

	// Order and paginate
	sqlQuery += " ORDER BY name ASC, version DESC OFFSET ? LIMIT ?"
	args = append(args, query.Offset, query.Limit)

	// Convert placeholders for PostgreSQL compatibility
	sqlQuery = convertPlaceholdersDM(sqlQuery)

	rows, err := r.data.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list data models: %w", err)
	}
	defer rows.Close()

	models := make([]*DataModelModel, 0) // Initialize as empty slice, not nil

	for rows.Next() {
		var dm DataModelModel
		err := rows.Scan(
			&dm.ID, &dm.DeviceID, &dm.Name, &dm.Version,
			&dm.Description, &dm.EncodedStructure, &dm.CreatedAt, &dm.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan data model: %w", err)
		}

		models = append(models, &dm)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating data models: %w", err)
	}

	return models, nil
}

// Delete removes a data model record
func (r *DataModelRepo) Delete(ctx context.Context, tenantID, deviceID, name, version string) error {
	query := `DELETE FROM data_models WHERE tenant_id = ? AND device_id = ? AND name = ? AND version = ?`
	query = convertPlaceholdersDM(query)

	_, err := r.data.db.ExecContext(ctx, query, tenantID, deviceID, name, version)
	if err != nil {
		return fmt.Errorf("failed to delete data model: %w", err)
	}

	return nil
}

// DeleteByDevice removes all data models for a device
func (r *DataModelRepo) DeleteByDevice(ctx context.Context, tenantID, deviceID string) error {
	query := `DELETE FROM data_models WHERE tenant_id = ? AND device_id = ?`
	query = convertPlaceholdersDM(query)

	_, err := r.data.db.ExecContext(ctx, query, tenantID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete data models for device: %w", err)
	}

	return nil
}

// DeleteByName removes all versions of a named data model
func (r *DataModelRepo) DeleteByName(ctx context.Context, tenantID, deviceID, name string) error {
	query := `DELETE FROM data_models WHERE tenant_id = ? AND device_id = ? AND name = ?`
	query = convertPlaceholdersDM(query)

	_, err := r.data.db.ExecContext(ctx, query, tenantID, deviceID, name)
	if err != nil {
		return fmt.Errorf("failed to delete data model by name: %w", err)
	}

	return nil
}

// GetLatestByName retrieves the latest version of a data model by name
// Used by GetDataModel RPC handler for enrichment and existence checks
func (r *DataModelRepo) GetLatestByName(ctx context.Context, tenantID, deviceID, name string) (*DataModelModel, error) {
	query := `
		SELECT id, device_id, name, version, description, encoded_structure, created_at, updated_at
		FROM data_models
		WHERE tenant_id = ? AND device_id = ? AND name = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	query = convertPlaceholdersDM(query)

	row := r.data.db.QueryRowContext(ctx, query, tenantID, deviceID, name)

	var dm DataModelModel
	err := row.Scan(
		&dm.ID, &dm.DeviceID, &dm.Name, &dm.Version,
		&dm.Description, &dm.EncodedStructure, &dm.CreatedAt, &dm.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest data model: %w", err)
	}

	return &dm, nil
}
