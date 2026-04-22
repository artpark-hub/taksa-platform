package models

import (
	"time"
)

// AuthToken represents a device authentication credential
// Internal use only - used for storage and business logic
type AuthToken struct {
	Token     string
	DeviceID  string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// AuthTokenInfo holds device_id and tenant_id resolved from an auth token.
// Used by system-wide token lookups where tenant context is not yet known (e.g., login).
type AuthTokenInfo struct {
	DeviceID string
	TenantID string
}
