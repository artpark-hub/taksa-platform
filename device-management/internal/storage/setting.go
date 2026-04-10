package storage

import "context"

// SettingStore defines the interface for settings storage operations
type SettingStore interface {
	// Get retrieves a setting value by key
	Get(ctx context.Context, key string) (string, error)

	// GetOrDefault retrieves a setting value with a default fallback
	GetOrDefault(ctx context.Context, key, defaultValue string) (string, error)

	// Set persists a setting
	Set(ctx context.Context, key, value string) error

	// SetWithDescription persists a setting with description
	SetWithDescription(ctx context.Context, key, value, description string) error

	// Delete removes a setting
	Delete(ctx context.Context, key string) error

	// GetAll retrieves all settings
	GetAll(ctx context.Context) (map[string]string, error)

	// GetAllWithDescription retrieves all settings with descriptions
	GetAllWithDescription(ctx context.Context) (map[string]SettingValue, error)

	// Exists checks if a setting exists
	Exists(ctx context.Context, key string) (bool, error)

	// GetInt retrieves a setting as integer
	GetInt(ctx context.Context, key string, defaultValue int) (int, error)

	// GetBool retrieves a setting as boolean
	GetBool(ctx context.Context, key string, defaultValue bool) (bool, error)

	// SetInt sets an integer setting
	SetInt(ctx context.Context, key string, value int) error

	// SetBool sets a boolean setting
	SetBool(ctx context.Context, key string, value bool) error
}

// SettingValue represents a setting with its metadata
type SettingValue struct {
	Key         string
	Value       string
	Description string
}
