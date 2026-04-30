package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// DeviceStore implements storage.DeviceStore for PostgreSQL
type DeviceStore struct {
	db *sql.DB
}

// Save persists a device to storage
func (s *DeviceStore) Save(ctx context.Context, device *v1.Device) error {
	if device == nil {
		return ErrInvalidInput
	}

	// Initialize nested structures if nil
	if device.Metadata == nil {
		device.Metadata = &v1.DeviceMetadata{}
	}

	// Extract location from device.Location.Levels map
	// ISA-95 hierarchy (7 levels): 0=company, 1=plant, 2=area, 3=zone, 4=line, 5=workCell, 6=workUnit
	locCompany := ""
	locPlant := ""
	locArea := ""
	locZone := ""
	locLine := ""
	locWorkCell := ""
	locWorkUnit := ""
	if device.Location != nil && device.Location.Levels != nil {
		if val, ok := device.Location.Levels["0"]; ok {
			locCompany = val
		}
		if val, ok := device.Location.Levels["1"]; ok {
			locPlant = val
		}
		if val, ok := device.Location.Levels["2"]; ok {
			locArea = val
		}
		if val, ok := device.Location.Levels["3"]; ok {
			locZone = val
		}
		if val, ok := device.Location.Levels["4"]; ok {
			locLine = val
		}
		if val, ok := device.Location.Levels["5"]; ok {
			locWorkCell = val
		}
		if val, ok := device.Location.Levels["6"]; ok {
			locWorkUnit = val
		}
	}

	// Multi-tenancy: extract tenant_id from JWT context
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return fmt.Errorf("tenant_id not found in context - ensure JWT middleware is properly configured")
	}

	query := `
	INSERT INTO devices (
		id, uuid, tenant_id, created_by, name,
		hardware_version, operating_system, manufacturer, firmware_version, ip_address, mac_address,
		location_company, location_plant, location_area, location_zone, location_line, location_work_cell, location_work_unit,
		certificate, encrypted_private_key,
		status, created_at, last_seen, last_login_at, auth_token_expires_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
	`

	_, err := s.db.ExecContext(ctx, query,
		device.Id, device.Id, tenantID, device.CreatedBy, device.Name,
		device.Metadata.HardwareVersion, device.Metadata.OperatingSystem, device.Metadata.Manufacturer,
		device.Metadata.FirmwareVersion, device.Metadata.IpAddress, device.Metadata.MacAddress,
		locCompany, locPlant, locArea, locZone, locLine, locWorkCell, locWorkUnit,  // 7-level location hierarchy
		device.Certificate, device.EncryptedPrivateKey,
		device.Status, device.CreatedAt.AsTime().Format(time.RFC3339),
		optionalTime(device.LastSeen),
		optionalTime(device.LastLogin), optionalTime(device.AuthTokenExpiresAt),
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to save device: %w", err)
	}

	return nil
}

// GetByID retrieves a device by its ID
func (s *DeviceStore) GetByID(ctx context.Context, id string) (*v1.Device, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}

	device := &v1.Device{}
	tenantID := middleware.GetTenantID(ctx)
	var err error
	if tenantID != "" {
		err = s.getDevice(ctx, "WHERE id = $1 AND tenant_id = $2", []interface{}{id, tenantID}, device)
	} else {
		err = s.getDevice(ctx, "WHERE id = $1", id, device)
	}
	if err != nil {
		return nil, err
	}
	return device, nil
}

// GetByCreatedByAndName retrieves a device by created_by (tenant) and name
func (s *DeviceStore) GetByCreatedByAndName(ctx context.Context, createdBy, name string) (*v1.Device, error) {
	if createdBy == "" || name == "" {
		return nil, ErrInvalidInput
	}

	device := &v1.Device{}
	tenantID := middleware.GetTenantID(ctx)
	var err error
	if tenantID != "" {
		err = s.getDevice(ctx, "WHERE created_by = $1 AND name = $2 AND tenant_id = $3", []interface{}{createdBy, name, tenantID}, device)
	} else {
		err = s.getDevice(ctx, "WHERE created_by = $1 AND name = $2", []interface{}{createdBy, name}, device)
	}
	if err != nil {
		return nil, err
	}
	return device, nil
}

// GetByUUID retrieves a device by UUID
func (s *DeviceStore) GetByUUID(ctx context.Context, uuid string) (*v1.Device, error) {
	if uuid == "" {
		return nil, ErrInvalidInput
	}

	device := &v1.Device{}
	tenantID := middleware.GetTenantID(ctx)
	var err error
	if tenantID != "" {
		err = s.getDevice(ctx, "WHERE uuid = $1 AND tenant_id = $2", []interface{}{uuid, tenantID}, device)
	} else {
		err = s.getDevice(ctx, "WHERE uuid = $1", uuid, device)
	}
	if err != nil {
		return nil, err
	}
	return device, nil
}

// List retrieves devices with optional filtering and pagination
func (s *DeviceStore) List(ctx context.Context, filters *storage.DeviceListFilter) ([]*v1.Device, error) {
	if filters == nil {
		filters = storage.DefaultDeviceListFilter()
	}

	// Build WHERE clause
	where := []string{}
	args := []interface{}{}
	paramCounter := 1

	// Tenant isolation
	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		where = append(where, fmt.Sprintf("tenant_id = $%d", paramCounter))
		args = append(args, tenantID)
		paramCounter++
	}

	if len(filters.StatusFilters) > 0 {
		placeholders := ""
		for range filters.StatusFilters {
			if placeholders != "" {
				placeholders += ", "
			}
			placeholders += fmt.Sprintf("$%d", paramCounter)
			paramCounter++
		}
		where = append(where, "status IN ("+placeholders+")")
		for _, status := range filters.StatusFilters {
			args = append(args, int(status))
		}
	}

	if filters.LocationFilter != "" {
		where = append(where, fmt.Sprintf("location_company ILIKE $%d", paramCounter))
		args = append(args, "%"+filters.LocationFilter+"%")
		paramCounter++
	}

	if filters.Search != "" {
		where = append(where, fmt.Sprintf("name ILIKE $%d", paramCounter))
		args = append(args, "%"+filters.Search+"%")
		paramCounter++
	}

	if filters.CreatedBy != "" {
		where = append(where, fmt.Sprintf("created_by ILIKE $%d", paramCounter))
		args = append(args, "%"+filters.CreatedBy+"%")
		paramCounter++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Build ORDER BY with whitelist to prevent SQL injection
	orderBy := "created_at"
	allowedColumns := map[string]bool{
		"id":               true,
		"name":             true,
		"created_at":       true,
		"last_seen":        true,
		"created_by":       true,
		"status":           true,
		"location_company": true,
	}
	if filters.SortBy != "" && allowedColumns[filters.SortBy] {
		orderBy = filters.SortBy
	}
	orderDirection := "ASC"
	if filters.SortDesc {
		orderDirection = "DESC"
	}

	// Use offset from cursor pagination
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	// Fetch page_size + 1 to detect if more results exist
	limit := filters.PageSize + 1

	// Execute query with limit and offset
	query := fmt.Sprintf(`
		SELECT id, uuid, created_by, name, hardware_version, operating_system, manufacturer,
		       firmware_version, ip_address, mac_address, location_company, location_plant,
		       location_area, location_zone, location_line, location_work_cell, location_work_unit,
		       certificate, encrypted_private_key, status, created_at, last_seen,
		       last_login_at, auth_token_expires_at
		FROM devices %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, orderBy, orderDirection, paramCounter, paramCounter+1)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	devices := make([]*v1.Device, 0, filters.PageSize+1)
	for rows.Next() {
		device, err := s.scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading devices: %w", err)
	}

	return devices, nil
}

// ListSummaries retrieves device summaries with optional filtering and pagination
func (s *DeviceStore) ListSummaries(ctx context.Context, filters *storage.DeviceListFilter) ([]*v1.DeviceSummary, error) {
	if filters == nil {
		filters = storage.DefaultDeviceListFilter()
	}

	// Build WHERE clause (effective-status aware)
	where := []string{}
	args := []interface{}{}
	paramCounter := 1

	tenantID := middleware.GetTenantID(ctx)
	if tenantID != "" {
		where = append(where, fmt.Sprintf("tenant_id = $%d", paramCounter))
		args = append(args, tenantID)
		paramCounter++
	}

	// Effective status expression must match deriveEffectiveDeviceStatus rules.
	// Use status ints directly in SQL.
	activeWindowSeconds := int(deviceActiveWindow.Seconds())
	effectiveStatusExpr := fmt.Sprintf(`(
		CASE
			WHEN status IN (%d, %d, %d) THEN status
			WHEN last_seen IS NULL THEN %d
			WHEN (NOW() - last_seen) <= make_interval(secs => %d) THEN %d
			ELSE %d
		END
	)`, statusToInt(v1.DeviceStatus_PENDING), statusToInt(v1.DeviceStatus_SUSPENDED), statusToInt(v1.DeviceStatus_DECOMMISSIONED),
		statusToInt(v1.DeviceStatus_INACTIVE),
		activeWindowSeconds,
		statusToInt(v1.DeviceStatus_ACTIVE),
		statusToInt(v1.DeviceStatus_INACTIVE),
	)

	if len(filters.StatusFilters) > 0 {
		placeholders := ""
		for range filters.StatusFilters {
			if placeholders != "" {
				placeholders += ", "
			}
			placeholders += fmt.Sprintf("$%d", paramCounter)
			paramCounter++
		}
		where = append(where, effectiveStatusExpr+" IN ("+placeholders+")")
		for _, status := range filters.StatusFilters {
			args = append(args, int(status))
		}
	}

	if filters.LocationFilter != "" {
		where = append(where, fmt.Sprintf("location_company ILIKE $%d", paramCounter))
		args = append(args, "%"+filters.LocationFilter+"%")
		paramCounter++
	}

	if filters.Search != "" {
		where = append(where, fmt.Sprintf("name ILIKE $%d", paramCounter))
		args = append(args, "%"+filters.Search+"%")
		paramCounter++
	}

	if filters.CreatedBy != "" {
		where = append(where, fmt.Sprintf("created_by ILIKE $%d", paramCounter))
		args = append(args, "%"+filters.CreatedBy+"%")
		paramCounter++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// ORDER BY with whitelist; map status -> effective_status.
	orderBy := "created_at"
	allowedColumns := map[string]bool{
		"id":               true,
		"name":             true,
		"created_at":       true,
		"last_seen":        true,
		"created_by":       true,
		"status":           true,
		"location_company": true,
	}
	if filters.SortBy != "" && allowedColumns[filters.SortBy] {
		if filters.SortBy == "status" {
			orderBy = "effective_status"
		} else {
			orderBy = filters.SortBy
		}
	}
	orderDirection := "ASC"
	if filters.SortDesc {
		orderDirection = "DESC"
	}

	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}
	limit := filters.PageSize + 1

	query := fmt.Sprintf(`
		SELECT
			id,
			created_by,
			name,
			location_company, location_plant, location_area, location_zone, location_line, location_work_cell, location_work_unit,
			%s AS effective_status,
			created_at,
			last_seen
		FROM devices
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, effectiveStatusExpr, whereClause, orderBy, orderDirection, paramCounter, paramCounter+1)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query device summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]*v1.DeviceSummary, 0, filters.PageSize+1)
	for rows.Next() {
		summary := &v1.DeviceSummary{}
		var createdAt, lastSeen sql.NullString
		var locCompany, locPlant, locArea, locZone sql.NullString
		var locLine, locWorkCell, locWorkUnit sql.NullString
		var effectiveStatus int32

		if err := rows.Scan(
			&summary.Id,
			&summary.CreatedBy,
			&summary.Name,
			&locCompany, &locPlant, &locArea, &locZone, &locLine, &locWorkCell, &locWorkUnit,
			&effectiveStatus,
			&createdAt,
			&lastSeen,
		); err != nil {
			return nil, fmt.Errorf("failed to scan device summary: %w", err)
		}

		// Location
		summary.Location = &v1.DeviceLocation{Levels: map[string]string{}}
		if locCompany.Valid && locCompany.String != "" {
			summary.Location.Levels["0"] = locCompany.String
		}
		if locPlant.Valid && locPlant.String != "" {
			summary.Location.Levels["1"] = locPlant.String
		}
		if locArea.Valid && locArea.String != "" {
			summary.Location.Levels["2"] = locArea.String
		}
		if locZone.Valid && locZone.String != "" {
			summary.Location.Levels["3"] = locZone.String
		}
		if locLine.Valid && locLine.String != "" {
			summary.Location.Levels["4"] = locLine.String
		}
		if locWorkCell.Valid && locWorkCell.String != "" {
			summary.Location.Levels["5"] = locWorkCell.String
		}
		if locWorkUnit.Valid && locWorkUnit.String != "" {
			summary.Location.Levels["6"] = locWorkUnit.String
		}

		// Status
		summary.Status = v1.DeviceStatus(effectiveStatus)

		// Timestamps
		if createdAt.Valid {
			if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				summary.CreatedAt = timestamppb.New(t)
			}
		}
		if lastSeen.Valid {
			if t, err := time.Parse(time.RFC3339, lastSeen.String); err == nil {
				summary.LastSeen = timestamppb.New(t)
			}
		}

		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading device summaries: %w", err)
	}

	return summaries, nil
}

const deviceActiveWindow = 1 * time.Minute

func allowUnscopedDeviceUpdate(ctx context.Context, id string) bool {
	// Only allow tenant bypass for explicitly device-authenticated requests.
	// This prevents cross-tenant mutation from admin/user contexts that may carry
	// a tenant_id but are not scoped to a specific device identity.
	return id != "" && middleware.GetDeviceID(ctx) == id
}

// deriveEffectiveDeviceStatus derives the listing status according to product rules:
// - On registration: PENDING.
// - On first successful login: becomes ACTIVE (persisted).
// - Subsequently toggles between ACTIVE/INACTIVE based on last_seen activity.
// - SUSPENDED/DECOMMISSIONED are terminal admin states.
func deriveEffectiveDeviceStatus(device *v1.Device) v1.DeviceStatus {
	if device == nil {
		return v1.DeviceStatus_DEVICE_STATUS_UNSPECIFIED
	}

	switch device.Status {
	case v1.DeviceStatus_SUSPENDED, v1.DeviceStatus_DECOMMISSIONED:
		return device.Status
	case v1.DeviceStatus_PENDING:
		// Registration state always shows as PENDING until login flips status to ACTIVE.
		// Do NOT use last_seen to infer ACTIVE/INACTIVE while still administratively PENDING
		// (older rows or DB defaults may have last_seen initialized).
		return v1.DeviceStatus_PENDING
	default:
		// Derive only for statuses that represent "logged in at least once" state
		// (ACTIVE/INACTIVE) or legacy/unspecified rows. Keep other admin statuses as-is.
		if device.Status != v1.DeviceStatus_ACTIVE &&
			device.Status != v1.DeviceStatus_INACTIVE &&
			device.Status != v1.DeviceStatus_DEVICE_STATUS_UNSPECIFIED {
			return device.Status
		}
	}

	// If we have a heartbeat timestamp, use it as the primary activity signal.
	if device.LastSeen != nil {
		lastSeen := device.LastSeen.AsTime()
		if time.Since(lastSeen) <= deviceActiveWindow {
			return v1.DeviceStatus_ACTIVE
		}
		return v1.DeviceStatus_INACTIVE
	}

	// No last_seen, but has logged in at least once.
	return v1.DeviceStatus_INACTIVE
}

func statusToInt(s v1.DeviceStatus) int32 {
	return int32(s)
}

// Update updates an existing device
func (s *DeviceStore) Update(ctx context.Context, device *v1.Device) error {
	if device == nil || device.Id == "" {
		return ErrInvalidInput
	}

	// Extract location from device.Location.Levels map
	// ISA-95 hierarchy (7 levels): 0=company, 1=plant, 2=area, 3=zone, 4=line, 5=workCell, 6=workUnit
	locCompany := ""
	locPlant := ""
	locArea := ""
	locZone := ""
	locLine := ""
	locWorkCell := ""
	locWorkUnit := ""
	if device.Location != nil && device.Location.Levels != nil {
		if val, ok := device.Location.Levels["0"]; ok {
			locCompany = val
		}
		if val, ok := device.Location.Levels["1"]; ok {
			locPlant = val
		}
		if val, ok := device.Location.Levels["2"]; ok {
			locArea = val
		}
		if val, ok := device.Location.Levels["3"]; ok {
			locZone = val
		}
		if val, ok := device.Location.Levels["4"]; ok {
			locLine = val
		}
		if val, ok := device.Location.Levels["5"]; ok {
			locWorkCell = val
		}
		if val, ok := device.Location.Levels["6"]; ok {
			locWorkUnit = val
		}
	}

	tenantID := middleware.GetTenantID(ctx)
	query := `
	UPDATE devices SET
		name = $1,
		hardware_version = $2, operating_system = $3, manufacturer = $4, firmware_version = $5,
		ip_address = $6, mac_address = $7,
		location_company = $8, location_plant = $9, location_area = $10, location_zone = $11, location_line = $12, location_work_cell = $13, location_work_unit = $14,
		certificate = $15, encrypted_private_key = $16,
		status = $17, last_seen = $18, last_login_at = $19, auth_token_expires_at = $20
	WHERE id = $21
	`
	args := []interface{}{
		device.Name,
		device.Metadata.HardwareVersion, device.Metadata.OperatingSystem, device.Metadata.Manufacturer,
		device.Metadata.FirmwareVersion, device.Metadata.IpAddress, device.Metadata.MacAddress,
		locCompany, locPlant, locArea, locZone, locLine, locWorkCell, locWorkUnit,
		device.Certificate, device.EncryptedPrivateKey,
		device.Status, optionalTime(device.LastSeen),
		optionalTime(device.LastLogin), optionalTime(device.AuthTokenExpiresAt),
		device.Id,
	}
	if tenantID != "" {
		query = `
		UPDATE devices SET
			name = $1,
			hardware_version = $2, operating_system = $3, manufacturer = $4, firmware_version = $5,
			ip_address = $6, mac_address = $7,
			location_company = $8, location_plant = $9, location_area = $10, location_zone = $11, location_line = $12, location_work_cell = $13, location_work_unit = $14,
			certificate = $15, encrypted_private_key = $16,
			status = $17, last_seen = $18, last_login_at = $19, auth_token_expires_at = $20
		WHERE id = $21 AND tenant_id = $22
		`
		args = append(args, tenantID)
	}

	result, err := s.db.ExecContext(ctx, query, args...)

	if err != nil {
		return fmt.Errorf("failed to update device: %w", err)
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

// UpdateAuthTokenExpiresAt updates only the auth_token_expires_at timestamp.
func (s *DeviceStore) UpdateAuthTokenExpiresAt(ctx context.Context, id string, timestamp time.Time) error {
	if id == "" {
		return ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	query := "UPDATE devices SET auth_token_expires_at = $1 WHERE id = $2"
	args := []interface{}{timestamp.Format(time.RFC3339), id}
	if tenantID != "" {
		query = "UPDATE devices SET auth_token_expires_at = $1 WHERE id = $2 AND tenant_id = $3"
		args = []interface{}{timestamp.Format(time.RFC3339), id, tenantID}
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update auth_token_expires_at: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Tenant mismatch fallback, gated to device-authenticated calls only.
	if rows == 0 && tenantID != "" && allowUnscopedDeviceUpdate(ctx, id) {
		fallbackResult, fbErr := s.db.ExecContext(ctx, "UPDATE devices SET auth_token_expires_at = $1 WHERE id = $2", timestamp.Format(time.RFC3339), id)
		if fbErr != nil {
			return fmt.Errorf("failed to update auth_token_expires_at (fallback): %w", fbErr)
		}
		fallbackRows, fbErr := fallbackResult.RowsAffected()
		if fbErr != nil {
			return fmt.Errorf("failed to get rows affected (fallback): %w", fbErr)
		}
		rows = fallbackRows
	}

	if rows == 0 {
		if tenantID != "" && !allowUnscopedDeviceUpdate(ctx, id) {
			return fmt.Errorf("%w: tenant mismatch or device not found", ErrNotFound)
		}
		return ErrNotFound
	}

	return nil
}

// UpdateStatus updates only the device status
func (s *DeviceStore) UpdateStatus(ctx context.Context, id string, status v1.DeviceStatus) error {
	if id == "" {
		return ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	query := "UPDATE devices SET status = $1 WHERE id = $2"
	args := []interface{}{status, id}
	if tenantID != "" {
		query = "UPDATE devices SET status = $1 WHERE id = $2 AND tenant_id = $3"
		args = []interface{}{status, id, tenantID}
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If the tenant_id in context is stale/mismatched (e.g. after migrations),
	// optionally fall back to updating by device id only, but ONLY for explicitly
	// device-authenticated calls.
	if rows == 0 && tenantID != "" && allowUnscopedDeviceUpdate(ctx, id) {
		fallbackResult, fbErr := s.db.ExecContext(ctx, "UPDATE devices SET status = $1 WHERE id = $2", status, id)
		if fbErr != nil {
			return fmt.Errorf("failed to update device status (fallback): %w", fbErr)
		}
		fallbackRows, fbErr := fallbackResult.RowsAffected()
		if fbErr != nil {
			return fmt.Errorf("failed to get rows affected (fallback): %w", fbErr)
		}
		rows = fallbackRows
	}

	if rows == 0 {
		if tenantID != "" && !allowUnscopedDeviceUpdate(ctx, id) {
			return fmt.Errorf("%w: tenant mismatch or device not found", ErrNotFound)
		}
		return ErrNotFound
	}

	return nil
}

// UpdateLastSeen updates the last_seen timestamp
func (s *DeviceStore) UpdateLastSeen(ctx context.Context, id string, timestamp time.Time) error {
	if id == "" {
		return ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	query := "UPDATE devices SET last_seen = $1 WHERE id = $2"
	args := []interface{}{timestamp.Format(time.RFC3339), id}
	if tenantID != "" {
		query = "UPDATE devices SET last_seen = $1 WHERE id = $2 AND tenant_id = $3"
		args = []interface{}{timestamp.Format(time.RFC3339), id, tenantID}
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update last_seen: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Tenant mismatch fallback, see UpdateStatus for rationale.
	if rows == 0 && tenantID != "" && allowUnscopedDeviceUpdate(ctx, id) {
		fallbackResult, fbErr := s.db.ExecContext(ctx, "UPDATE devices SET last_seen = $1 WHERE id = $2", timestamp.Format(time.RFC3339), id)
		if fbErr != nil {
			return fmt.Errorf("failed to update last_seen (fallback): %w", fbErr)
		}
		fallbackRows, fbErr := fallbackResult.RowsAffected()
		if fbErr != nil {
			return fmt.Errorf("failed to get rows affected (fallback): %w", fbErr)
		}
		rows = fallbackRows
	}

	if rows == 0 {
		if tenantID != "" && !allowUnscopedDeviceUpdate(ctx, id) {
			return fmt.Errorf("%w: tenant mismatch or device not found", ErrNotFound)
		}
		return ErrNotFound
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func (s *DeviceStore) UpdateLastLogin(ctx context.Context, id string, timestamp time.Time) error {
	if id == "" {
		return ErrInvalidInput
	}

	tenantID := middleware.GetTenantID(ctx)

	query := "UPDATE devices SET last_login_at = $1 WHERE id = $2"
	args := []interface{}{timestamp.Format(time.RFC3339), id}
	if tenantID != "" {
		query = "UPDATE devices SET last_login_at = $1 WHERE id = $2 AND tenant_id = $3"
		args = []interface{}{timestamp.Format(time.RFC3339), id, tenantID}
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update last_login_at: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Tenant mismatch fallback, see UpdateStatus for rationale.
	if rows == 0 && tenantID != "" && allowUnscopedDeviceUpdate(ctx, id) {
		fallbackResult, fbErr := s.db.ExecContext(ctx, "UPDATE devices SET last_login_at = $1 WHERE id = $2", timestamp.Format(time.RFC3339), id)
		if fbErr != nil {
			return fmt.Errorf("failed to update last_login_at (fallback): %w", fbErr)
		}
		fallbackRows, fbErr := fallbackResult.RowsAffected()
		if fbErr != nil {
			return fmt.Errorf("failed to get rows affected (fallback): %w", fbErr)
		}
		rows = fallbackRows
	}

	if rows == 0 {
		if tenantID != "" && !allowUnscopedDeviceUpdate(ctx, id) {
			return fmt.Errorf("%w: tenant mismatch or device not found", ErrNotFound)
		}
		return ErrNotFound
	}

	return nil
}

// Delete removes a device
func (s *DeviceStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	query := "DELETE FROM devices WHERE id = $1"
	args := []interface{}{id}
	if tenantID := middleware.GetTenantID(ctx); tenantID != "" {
		query = "DELETE FROM devices WHERE id = $1 AND tenant_id = $2"
		args = append(args, tenantID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
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

// Helper methods

func (s *DeviceStore) getDevice(ctx context.Context, where string, args interface{}, device *v1.Device) error {
	query := fmt.Sprintf(`
		SELECT id, uuid, created_by, name, hardware_version, operating_system, manufacturer,
		       firmware_version, ip_address, mac_address, location_company, location_plant,
		       location_area, location_zone, location_line, location_work_cell, location_work_unit,
		       certificate, encrypted_private_key, status, created_at, last_seen,
		       last_login_at, auth_token_expires_at
		FROM devices %s
	`, where)

	// Handle both single arg and multiple args
	var queryArgs []interface{}
	switch v := args.(type) {
	case []interface{}:
		queryArgs = v
	default:
		queryArgs = []interface{}{args}
	}

	return s.scanDeviceRow(s.db.QueryRowContext(ctx, query, queryArgs...), device)
}

func (s *DeviceStore) scanDeviceRow(row *sql.Row, device *v1.Device) error {
	var createdAt, lastSeen sql.NullString
	var lastLogin, authTokenExpires sql.NullString
	var locCompany, locPlant, locArea, locZone sql.NullString
	var locLine, locWorkCell, locWorkUnit sql.NullString  // 7-level location fields (nullable)

	// Initialize nested structures
	if device.Metadata == nil {
		device.Metadata = &v1.DeviceMetadata{}
	}
	
	err := row.Scan(
		&device.Id, &device.Id, &device.CreatedBy, &device.Name,
		&device.Metadata.HardwareVersion, &device.Metadata.OperatingSystem, &device.Metadata.Manufacturer,
		&device.Metadata.FirmwareVersion, &device.Metadata.IpAddress, &device.Metadata.MacAddress,
		&locCompany, &locPlant, &locArea, &locZone, &locLine, &locWorkCell, &locWorkUnit,  // 7-level location fields
		&device.Certificate, &device.EncryptedPrivateKey,
		&device.Status, &createdAt, &lastSeen, &lastLogin, &authTokenExpires,
	)
	
	// Initialize location map
	if device.Location == nil {
		device.Location = &v1.DeviceLocation{
			Levels: make(map[string]string),
		}
	}
	if device.Location.Levels == nil {
		device.Location.Levels = make(map[string]string)
	}

	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to scan device: %w", err)
	}

	// Populate location from columns (7-level ISA-95 hierarchy)
	// Levels: 0=company, 1=plant, 2=area, 3=zone, 4=line, 5=workCell, 6=workUnit
	if locCompany.Valid && locCompany.String != "" {
		device.Location.Levels["0"] = locCompany.String
	}
	if locPlant.Valid && locPlant.String != "" {
		device.Location.Levels["1"] = locPlant.String
	}
	if locArea.Valid && locArea.String != "" {
		device.Location.Levels["2"] = locArea.String
	}
	if locZone.Valid && locZone.String != "" {
		device.Location.Levels["3"] = locZone.String
	}
	if locLine.Valid && locLine.String != "" {
		device.Location.Levels["4"] = locLine.String
	}
	if locWorkCell.Valid && locWorkCell.String != "" {
		device.Location.Levels["5"] = locWorkCell.String
	}
	if locWorkUnit.Valid && locWorkUnit.String != "" {
		device.Location.Levels["6"] = locWorkUnit.String
	}

	// Parse timestamps
	if createdAt.Valid {
		if createdAtTime, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			device.CreatedAt = timestamppb.New(createdAtTime)
		}
	}
	if lastSeen.Valid {
		if lastSeenTime, err := time.Parse(time.RFC3339, lastSeen.String); err == nil {
			device.LastSeen = timestamppb.New(lastSeenTime)
		}
	}
	if lastLogin.Valid {
		if lastLoginTime, err := time.Parse(time.RFC3339, lastLogin.String); err == nil {
			device.LastLogin = timestamppb.New(lastLoginTime)
		}
	}
	if authTokenExpires.Valid {
		if authTokenExpiresTime, err := time.Parse(time.RFC3339, authTokenExpires.String); err == nil {
			device.AuthTokenExpiresAt = timestamppb.New(authTokenExpiresTime)
		}
	}

	return nil
}

func (s *DeviceStore) scanDevice(rows *sql.Rows) (*v1.Device, error) {
	device := &v1.Device{}
	var createdAt, lastSeen sql.NullString
	var lastLogin, authTokenExpires sql.NullString
	var locCompany, locPlant, locArea, locZone sql.NullString
	var locLine, locWorkCell, locWorkUnit sql.NullString  // 7-level location fields (nullable)

	// Initialize nested structures BEFORE scanning
	if device.Metadata == nil {
		device.Metadata = &v1.DeviceMetadata{}
	}

	err := rows.Scan(
		&device.Id, &device.Id, &device.CreatedBy, &device.Name,
		&device.Metadata.HardwareVersion, &device.Metadata.OperatingSystem, &device.Metadata.Manufacturer,
		&device.Metadata.FirmwareVersion, &device.Metadata.IpAddress, &device.Metadata.MacAddress,
		&locCompany, &locPlant, &locArea, &locZone, &locLine, &locWorkCell, &locWorkUnit,  // 7-level location fields
		&device.Certificate, &device.EncryptedPrivateKey,
		&device.Status, &createdAt, &lastSeen, &lastLogin, &authTokenExpires,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to scan device row: %w", err)
	}

	// Initialize location map
	if device.Location == nil {
		device.Location = &v1.DeviceLocation{
			Levels: make(map[string]string),
		}
	}
	if device.Location.Levels == nil {
		device.Location.Levels = make(map[string]string)
	}

	// Populate location from columns (7-level ISA-95 hierarchy)
	// Levels: 0=company, 1=plant, 2=area, 3=zone, 4=line, 5=workCell, 6=workUnit
	if locCompany.Valid && locCompany.String != "" {
		device.Location.Levels["0"] = locCompany.String
	}
	if locPlant.Valid && locPlant.String != "" {
		device.Location.Levels["1"] = locPlant.String
	}
	if locArea.Valid && locArea.String != "" {
		device.Location.Levels["2"] = locArea.String
	}
	if locZone.Valid && locZone.String != "" {
		device.Location.Levels["3"] = locZone.String
	}
	if locLine.Valid && locLine.String != "" {
		device.Location.Levels["4"] = locLine.String
	}
	if locWorkCell.Valid && locWorkCell.String != "" {
		device.Location.Levels["5"] = locWorkCell.String
	}
	if locWorkUnit.Valid && locWorkUnit.String != "" {
		device.Location.Levels["6"] = locWorkUnit.String
	}

	// Parse timestamps
	if createdAt.Valid {
		if createdAtTime, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			device.CreatedAt = timestamppb.New(createdAtTime)
		}
	}
	if lastSeen.Valid {
		if lastSeenTime, err := time.Parse(time.RFC3339, lastSeen.String); err == nil {
			device.LastSeen = timestamppb.New(lastSeenTime)
		}
	}
	if lastLogin.Valid {
		if lastLoginTime, err := time.Parse(time.RFC3339, lastLogin.String); err == nil {
			device.LastLogin = timestamppb.New(lastLoginTime)
		}
	}
	if authTokenExpires.Valid {
		if authTokenExpiresTime, err := time.Parse(time.RFC3339, authTokenExpires.String); err == nil {
			device.AuthTokenExpiresAt = timestamppb.New(authTokenExpiresTime)
		}
	}

	return device, nil
}

// Utility functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
