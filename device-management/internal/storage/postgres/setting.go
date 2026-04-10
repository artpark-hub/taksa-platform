package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// SettingStore implements storage.SettingStore for PostgreSQL
type SettingStore struct {
	db *sql.DB
}

// Get retrieves a setting value by key
func (s *SettingStore) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", ErrInvalidInput
	}

	var value string
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM settings WHERE key = $1", key).Scan(&value)

	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get setting: %w", err)
	}

	return value, nil
}

// GetOrDefault retrieves a setting value with a default fallback
func (s *SettingStore) GetOrDefault(ctx context.Context, key, defaultValue string) (string, error) {
	if key == "" {
		return defaultValue, nil
	}

	value, err := s.Get(ctx, key)
	if err == ErrNotFound {
		return defaultValue, nil
	}
	if err != nil {
		return "", err
	}

	return value, nil
}

// Set persists a setting
func (s *SettingStore) Set(ctx context.Context, key, value string) (error) {
	return s.SetWithDescription(ctx, key, value, "")
}

// SetWithDescription persists a setting with description
func (s *SettingStore) SetWithDescription(ctx context.Context, key, value, description string) error {
	if key == "" {
		return ErrInvalidInput
	}

	query := `
	INSERT INTO settings (key, value, description)
	VALUES ($1, $2, $3)
	ON CONFLICT(key) DO UPDATE SET value = $2, description = $3
	`

	_, err := s.db.ExecContext(ctx, query, key, value, description)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	return nil
}

// Delete removes a setting
func (s *SettingStore) Delete(ctx context.Context, key string) error {
	if key == "" {
		return ErrInvalidInput
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM settings WHERE key = $1", key)
	if err != nil {
		return fmt.Errorf("failed to delete setting: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		return ErrNotFound
	}

	return nil
}

// GetAll retrieves all settings
func (s *SettingStore) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("failed to get all settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return settings, nil
}

// GetAllWithDescription retrieves all settings with descriptions
func (s *SettingStore) GetAllWithDescription(ctx context.Context) (map[string]storage.SettingValue, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value, description FROM settings")
	if err != nil {
		return nil, fmt.Errorf("failed to get all settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]storage.SettingValue)
	for rows.Next() {
		var key, value, description string
		err := rows.Scan(&key, &value, &description)
		if err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings[key] = storage.SettingValue{
			Key:         key,
			Value:       value,
			Description: description,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return settings, nil
}

// Exists checks if a setting exists
func (s *SettingStore) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, ErrInvalidInput
	}

	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM settings WHERE key = $1)", key).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check setting existence: %w", err)
	}

	return exists, nil
}

// GetInt retrieves a setting as integer
func (s *SettingStore) GetInt(ctx context.Context, key string, defaultValue int) (int, error) {
	value, err := s.Get(ctx, key)
	if err == ErrNotFound {
		return defaultValue, nil
	}
	if err != nil {
		return 0, err
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue, fmt.Errorf("failed to parse integer setting: %w", err)
	}

	return intVal, nil
}

// GetBool retrieves a setting as boolean
func (s *SettingStore) GetBool(ctx context.Context, key string, defaultValue bool) (bool, error) {
	value, err := s.Get(ctx, key)
	if err == ErrNotFound {
		return defaultValue, nil
	}
	if err != nil {
		return false, err
	}

	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue, fmt.Errorf("failed to parse boolean setting: %w", err)
	}

	return boolVal, nil
}

// SetInt sets an integer setting
func (s *SettingStore) SetInt(ctx context.Context, key string, value int) error {
	return s.Set(ctx, key, strconv.Itoa(value))
}

// SetBool sets a boolean setting
func (s *SettingStore) SetBool(ctx context.Context, key string, value bool) error {
	return s.Set(ctx, key, strconv.FormatBool(value))
}
