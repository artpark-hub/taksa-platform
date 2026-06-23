package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	uuidgen "github.com/google/uuid"

	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter"
)

// convertPlaceholders converts SQLite ? placeholders to PostgreSQL $1, $2 style
func convertPlaceholders(query string) string {
	counter := 1
	return regexp.MustCompile(`\?`).ReplaceAllStringFunc(query, func(string) string {
		placeholder := fmt.Sprintf("$%d", counter)
		counter++
		return placeholder
	})
}

// ProtocolConverterRepo handles database operations for protocol converters
type ProtocolConverterRepo struct {
	data *Data
}

// ProtocolConverterModel represents a protocol converter in the database
type ProtocolConverterModel struct {
	ID                  string
	DeviceID            string
	UUID                string
	Name                string
	Type                string
	ConnectionUUID      string
	InputYAML           sql.NullString
	ProcessorYAML       sql.NullString
	InjectYAML          sql.NullString
	IgnoreErrors        bool
	Metadata            sql.NullString // JSON
	DeploymentStatus    string         // PENDING, ACTIVE, FAILED
	HealthStatus        string         // ONLINE, OFFLINE, UNKNOWN
	ErrorMessage        sql.NullString
	LastSynced          sql.NullString // Store as TEXT for SQLite compatibility
	CreatedAt           sql.NullString // Store as TEXT for SQLite compatibility
	UpdatedAt           sql.NullString // Store as TEXT for SQLite compatibility
}

// Insert creates a new protocol converter record
func (r *ProtocolConverterRepo) Insert(ctx context.Context, tenantID, deviceID, uuid, name, converterType, connectionUUID string) error {
	query := `
		INSERT INTO protocol_converters (
			id, tenant_id, device_id, uuid, name, type, connection_uuid,
			deployment_status, health_status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, 'PENDING', 'UNKNOWN', ?, ?)
	`

	id := uuidgen.New().String()
	now := time.Now()

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	_, err := r.data.db.ExecContext(ctx, query,
		id, tenantID, deviceID, uuid, name, converterType, connectionUUID, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert protocol converter: %w", err)
	}

	return nil
}

// Update updates an existing protocol converter record
func (r *ProtocolConverterRepo) Update(ctx context.Context, tenantID, deviceID, uuid string, updates map[string]interface{}) error {
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

	args = append(args, tenantID, deviceID, uuid)

	query := fmt.Sprintf(`
		UPDATE protocol_converters 
		SET %s
		WHERE tenant_id = ? AND device_id = ? AND uuid = ?
	`, updateClause)

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	_, err := r.data.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update protocol converter: %w", err)
	}

	return nil
}

// UpdateStatus updates deployment and health status
func (r *ProtocolConverterRepo) UpdateStatus(ctx context.Context, tenantID, deviceID, uuid, deploymentStatus, healthStatus, errorMessage string) error {
	query := `
		UPDATE protocol_converters 
		SET deployment_status = ?, health_status = ?, error_message = ?, updated_at = ?
		WHERE tenant_id = ? AND device_id = ? AND uuid = ?
	`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	_, err := r.data.db.ExecContext(ctx, query,
		deploymentStatus, healthStatus, errorMessage, time.Now(), tenantID, deviceID, uuid)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// MarkSynced updates the last_synced timestamp
func (r *ProtocolConverterRepo) MarkSynced(ctx context.Context, tenantID, deviceID, uuid string) error {
	query := `
		UPDATE protocol_converters 
		SET last_synced = ?, updated_at = ?
		WHERE tenant_id = ? AND device_id = ? AND uuid = ?
	`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	_, err := r.data.db.ExecContext(ctx, query,
		time.Now(), time.Now(), tenantID, deviceID, uuid)
	if err != nil {
		return fmt.Errorf("failed to mark synced: %w", err)
	}

	return nil
}

// GetByUUID retrieves a protocol converter by UUID
func (r *ProtocolConverterRepo) GetByUUID(ctx context.Context, tenantID, deviceID, uuid string) (*ProtocolConverterModel, error) {
	query := `
		SELECT id, device_id, uuid, name, type, connection_uuid,
		       input_yaml, processor_yaml, inject_yaml, ignore_errors, metadata,
		       deployment_status, health_status, error_message, last_synced,
		       created_at, updated_at
		FROM protocol_converters
		WHERE tenant_id = ? AND device_id = ? AND uuid = ?
	`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	row := r.data.db.QueryRowContext(ctx, query, tenantID, deviceID, uuid)

	var pc ProtocolConverterModel
	err := row.Scan(
		&pc.ID, &pc.DeviceID, &pc.UUID, &pc.Name, &pc.Type, &pc.ConnectionUUID,
		&pc.InputYAML, &pc.ProcessorYAML, &pc.InjectYAML, &pc.IgnoreErrors, &pc.Metadata,
		&pc.DeploymentStatus, &pc.HealthStatus, &pc.ErrorMessage, &pc.LastSynced,
		&pc.CreatedAt, &pc.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get protocol converter: %w", err)
	}

	return &pc, nil
}

// ListQuery represents query parameters for listing protocol converters
type ListQuery struct {
	DeviceID              string
	UUIDFilter            string
	NameFilter            string
	TypeFilter            string
	DeploymentStatusFilter string
	ConnectionUUIDFilter   string
	HealthStatusFilter     string
	Offset                int64
	Limit                 int64
}

// List retrieves protocol converters with optional filtering and pagination
func (r *ProtocolConverterRepo) List(ctx context.Context, tenantID string, query *ListQuery) ([]*ProtocolConverterModel, error) {
	sqlQuery := `
		SELECT id, device_id, uuid, name, type, connection_uuid,
		       input_yaml, processor_yaml, inject_yaml, ignore_errors, metadata,
		       deployment_status, health_status, error_message, last_synced,
		       created_at, updated_at
		FROM protocol_converters
		WHERE tenant_id = ? AND device_id = ?
	`

	args := []interface{}{tenantID, query.DeviceID}

	// Add optional filters
	if query.UUIDFilter != "" {
		sqlQuery += " AND uuid = ?"
		args = append(args, query.UUIDFilter)
	}

	if query.NameFilter != "" {
		sqlQuery += " AND name LIKE ?"
		args = append(args, "%"+query.NameFilter+"%")
	}

	if query.TypeFilter != "" {
		sqlQuery += " AND type = ?"
		args = append(args, query.TypeFilter)
	}

	if query.DeploymentStatusFilter != "" {
		sqlQuery += " AND deployment_status = ?"
		args = append(args, query.DeploymentStatusFilter)
	}

	if query.ConnectionUUIDFilter != "" {
		sqlQuery += " AND connection_uuid = ?"
		args = append(args, query.ConnectionUUIDFilter)
	}

	if query.HealthStatusFilter != "" {
		sqlQuery += " AND health_status = ?"
		args = append(args, query.HealthStatusFilter)
	}

	// Order and paginate
	sqlQuery += " ORDER BY created_at DESC OFFSET ? LIMIT ?"
	args = append(args, query.Offset, query.Limit)

	// Convert placeholders for PostgreSQL compatibility
	sqlQuery = convertPlaceholders(sqlQuery)

	rows, err := r.data.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list protocol converters: %w", err)
	}
	defer rows.Close()

	converters := make([]*ProtocolConverterModel, 0) // Initialize as empty slice, not nil

	for rows.Next() {
		var pc ProtocolConverterModel
		err := rows.Scan(
			&pc.ID, &pc.DeviceID, &pc.UUID, &pc.Name, &pc.Type, &pc.ConnectionUUID,
			&pc.InputYAML, &pc.ProcessorYAML, &pc.InjectYAML, &pc.IgnoreErrors, &pc.Metadata,
			&pc.DeploymentStatus, &pc.HealthStatus, &pc.ErrorMessage, &pc.LastSynced,
			&pc.CreatedAt, &pc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan protocol converter: %w", err)
		}

		converters = append(converters, &pc)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating protocol converters: %w", err)
	}

	return converters, nil
}

// Delete removes a protocol converter record
func (r *ProtocolConverterRepo) Delete(ctx context.Context, tenantID, deviceID, uuid string) error {
	query := `DELETE FROM protocol_converters WHERE tenant_id = ? AND device_id = ? AND uuid = ?`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	_, err := r.data.db.ExecContext(ctx, query, tenantID, deviceID, uuid)
	if err != nil {
		return fmt.Errorf("failed to delete protocol converter: %w", err)
	}

	return nil
}

// DeleteByDevice removes all protocol converters for a device
func (r *ProtocolConverterRepo) DeleteByDevice(ctx context.Context, tenantID, deviceID string) error {
	query := `DELETE FROM protocol_converters WHERE tenant_id = ? AND device_id = ?`

	// Convert placeholders for PostgreSQL compatibility
	query = convertPlaceholders(query)

	_, err := r.data.db.ExecContext(ctx, query, tenantID, deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete protocol converters for device: %w", err)
	}

	return nil
}

// UpsertPending records a converter in the catalog before deploy/configure has finished.
func (r *ProtocolConverterRepo) UpsertPending(ctx context.Context, tenantID, deviceID, uuid, name, converterType, connectionUUID string) error {
	existing, err := r.GetByUUID(ctx, tenantID, deviceID, uuid)
	if err != nil {
		return err
	}
	if existing != nil {
		converterType = protocolconverter.ResolveCatalogWireType(converterType, existing.Type)
		return r.Update(ctx, tenantID, deviceID, uuid, map[string]interface{}{
			"name":              name,
			"type":              converterType,
			"connection_uuid":   connectionUUID,
			"deployment_status": "PENDING",
			"health_status":     "UNKNOWN",
			"error_message":     "",
			"last_synced":       time.Now(),
		})
	}
	return r.Insert(ctx, tenantID, deviceID, uuid, name, converterType, connectionUUID)
}

// PromoteDeployed marks a converter as fully deployed in the catalog.
// wireType, when non-empty, backfills the catalog wire protocol (opcua/modbus).
func (r *ProtocolConverterRepo) PromoteDeployed(ctx context.Context, tenantID, deviceID, uuid, wireType string) error {
	updates := map[string]interface{}{
		"deployment_status": "ACTIVE",
		"health_status":     "ONLINE",
		"error_message":     "",
		"last_synced":       time.Now(),
	}
	if wireType != "" && !protocolconverter.IsGenericCatalogType(wireType) {
		if existing, err := r.GetByUUID(ctx, tenantID, deviceID, uuid); err == nil && existing != nil {
			wireType = protocolconverter.ResolveCatalogWireType(wireType, existing.Type)
		}
		updates["type"] = wireType
	}
	return r.Update(ctx, tenantID, deviceID, uuid, updates)
}

// Upsert creates or updates a protocol converter as ACTIVE (device-confirmed deploy).
func (r *ProtocolConverterRepo) Upsert(ctx context.Context, tenantID, deviceID, uuid, name, converterType, connectionUUID string) error {
	existing, err := r.GetByUUID(ctx, tenantID, deviceID, uuid)
	if err != nil {
		return err
	}

	if existing != nil {
		converterType = protocolconverter.ResolveCatalogWireType(converterType, existing.Type)
		return r.Update(ctx, tenantID, deviceID, uuid, map[string]interface{}{
			"name":              name,
			"type":              converterType,
			"connection_uuid":   connectionUUID,
			"deployment_status": "ACTIVE",
			"health_status":     "ONLINE",
			"last_synced":       time.Now(),
		})
	}

	if err := r.Insert(ctx, tenantID, deviceID, uuid, name, converterType, connectionUUID); err != nil {
		return err
	}

	return r.PromoteDeployed(ctx, tenantID, deviceID, uuid, "")
}

// ParseMetadata parses the metadata JSON string into a map
func (r *ProtocolConverterRepo) ParseMetadata(metadataStr string) (map[string]string, error) {
	if metadataStr == "" {
		return make(map[string]string), nil
	}

	var metadata map[string]string
	if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return metadata, nil
}
