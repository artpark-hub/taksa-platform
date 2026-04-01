package models

import (
	"time"
)

// AuthToken represents a device authentication credential
// Internal use only - used for storage and business logic
type AuthToken struct {
	Token     string
	DeviceId  string
	ExpiresAt time.Time
	CreatedAt time.Time
}
