package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// DeviceStore implements storage.DeviceStore for SQLite
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

	query := `
	INSERT INTO devices (
		id, uuid, created_by, name,
		hardware_version, operating_system, manufacturer, firmware_version, ip_address, mac_address,
		location_company, location_plant, location_area, location_zone, location_line, location_work_cell, location_work_unit,
		certificate, encrypted_private_key,
		status, created_at, last_seen, last_login_at, auth_token_expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		device.Id, device.Id, device.CreatedBy, device.Name,
		device.Metadata.HardwareVersion, device.Metadata.OperatingSystem, device.Metadata.Manufacturer,
		device.Metadata.FirmwareVersion, device.Metadata.IpAddress, device.Metadata.MacAddress,
		locCompany, locPlant, locArea, locZone, locLine, locWorkCell, locWorkUnit,  // 7-level location hierarchy
		device.Certificate, device.EncryptedPrivateKey,
		device.Status, device.CreatedAt.AsTime().Format(time.RFC3339),
		device.LastSeen.AsTime().Format(time.RFC3339),
		optionalTimeValue(device.LastLogin.AsTime()), optionalTimeValue(device.AuthTokenExpiresAt.AsTime()),
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
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
	err := s.getDevice(ctx, "WHERE id = ?", id, device)
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
	err := s.getDevice(ctx, "WHERE created_by = ? AND name = ?", []interface{}{createdBy, name}, device)
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
	err := s.getDevice(ctx, "WHERE uuid = ?", uuid, device)
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

	if len(filters.StatusFilters) > 0 {
		placeholders := strings.Repeat("?,", len(filters.StatusFilters)-1) + "?"
		where = append(where, "status IN ("+placeholders+")")
		for _, status := range filters.StatusFilters {
			args = append(args, int(status))
		}
	}

	if filters.LocationFilter != "" {
		where = append(where, "location_company LIKE ?")
		args = append(args, "%"+filters.LocationFilter+"%")
	}

	if filters.Search != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+filters.Search+"%")
	}

	if filters.CreatedBy != "" {
		where = append(where, "created_by LIKE ?")
		args = append(args, "%"+filters.CreatedBy+"%")
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
		LIMIT ? OFFSET ?
	`, whereClause, orderBy, orderDirection)

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

	// Build WHERE clause
	where := []string{}
	args := []interface{}{}

	if len(filters.StatusFilters) > 0 {
		placeholders := strings.Repeat("?,", len(filters.StatusFilters)-1) + "?"
		where = append(where, "status IN ("+placeholders+")")
		for _, status := range filters.StatusFilters {
			args = append(args, int(status))
		}
	}

	if filters.LocationFilter != "" {
		where = append(where, "location_company LIKE ?")
		args = append(args, "%"+filters.LocationFilter+"%")
	}

	if filters.Search != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+filters.Search+"%")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Build ORDER BY
	orderBy := "created_at"
	if filters.SortBy != "" {
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

	// Execute query with limit and offset - SELECT only summary fields
	query := fmt.Sprintf(`
		SELECT id, created_by, name, location_company, location_plant, location_area, 
		       location_zone, location_line, location_work_cell, location_work_unit,
		       status, created_at, last_seen
		FROM devices %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereClause, orderBy, orderDirection)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query device summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]*v1.DeviceSummary, 0, filters.PageSize+1)
	for rows.Next() {
		var id, createdBy, name, locCompany, locPlant, locArea, locZone, locLine, locWorkCell, locWorkUnit string
		var status int
		var createdAt, lastSeen string

		err := rows.Scan(&id, &createdBy, &name, &locCompany, &locPlant, &locArea, &locZone, &locLine, &locWorkCell, &locWorkUnit, &status, &createdAt, &lastSeen)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device summary: %w", err)
		}

		// Reconstruct location from individual level columns
		location := &v1.DeviceLocation{
			Levels: make(map[string]string),
		}
		if locCompany != "" {
			location.Levels["0"] = locCompany
		}
		if locPlant != "" {
			location.Levels["1"] = locPlant
		}
		if locArea != "" {
			location.Levels["2"] = locArea
		}
		if locZone != "" {
			location.Levels["3"] = locZone
		}
		if locLine != "" {
			location.Levels["4"] = locLine
		}
		if locWorkCell != "" {
			location.Levels["5"] = locWorkCell
		}
		if locWorkUnit != "" {
			location.Levels["6"] = locWorkUnit
		}

		var createdAtTime, lastSeenTime *timestamppb.Timestamp
		if createdAtTs, err := time.Parse(time.RFC3339, createdAt); err == nil {
			createdAtTime = timestamppb.New(createdAtTs)
		}
		if lastSeenTs, err := time.Parse(time.RFC3339, lastSeen); err == nil {
			lastSeenTime = timestamppb.New(lastSeenTs)
		}

		summary := &v1.DeviceSummary{
			Id:        id,
			CreatedBy: createdBy,
			Name:      name,
			Location:  location,
			Status:    v1.DeviceStatus(status),
			CreatedAt: createdAtTime,
			LastSeen:  lastSeenTime,
		}
		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading device summaries: %w", err)
	}

	return summaries, nil
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

	query := `
	UPDATE devices SET
		name = ?,
		hardware_version = ?, operating_system = ?, manufacturer = ?, firmware_version = ?,
		ip_address = ?, mac_address = ?,
		location_company = ?, location_plant = ?, location_area = ?, location_zone = ?, location_line = ?, location_work_cell = ?, location_work_unit = ?,
		certificate = ?, encrypted_private_key = ?,
		status = ?, last_seen = ?, last_login_at = ?, auth_token_expires_at = ?
	WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		device.Name,
		device.Metadata.HardwareVersion, device.Metadata.OperatingSystem, device.Metadata.Manufacturer,
		device.Metadata.FirmwareVersion, device.Metadata.IpAddress, device.Metadata.MacAddress,
		locCompany, locPlant, locArea, locZone, locLine, locWorkCell, locWorkUnit,  // 7-level location hierarchy
		device.Certificate, device.EncryptedPrivateKey,
		device.Status, device.LastSeen.AsTime().Format(time.RFC3339),
		optionalTimeValue(device.LastLogin.AsTime()), optionalTimeValue(device.AuthTokenExpiresAt.AsTime()),
		device.Id,
	)

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

// Delete removes a device by ID
func (s *DeviceStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM devices WHERE id = ?", id)
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

// UpdateStatus updates only the device status
func (s *DeviceStore) UpdateStatus(ctx context.Context, id string, status v1.DeviceStatus) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "UPDATE devices SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
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

// UpdateLastSeen updates the last_seen timestamp
func (s *DeviceStore) UpdateLastSeen(ctx context.Context, id string, timestamp time.Time) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "UPDATE devices SET last_seen = ? WHERE id = ?",
		timestamp.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("failed to update last_seen: %w", err)
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

// UpdateLastLogin updates the last login timestamp
func (s *DeviceStore) UpdateLastLogin(ctx context.Context, id string, timestamp time.Time) error {
	if id == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "UPDATE devices SET last_login_at = ? WHERE id = ?",
		timestamp.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("failed to update last_login_at: %w", err)
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
