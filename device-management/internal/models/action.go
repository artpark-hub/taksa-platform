package models

import (
	"time"

	"google.golang.org/protobuf/types/known/anypb"
)

// Action represents an instruction queued for a device
// Internal use only - never exposed in RPC endpoints
type Action struct {
	Id             string
	DeviceId       string
	Type           string // "command", "config", "update", "restart", etc.
	Payload        *anypb.Any
	Status         ActionStatus
	CreatedAt      time.Time
	ExpiresAt      time.Time
	DeliveredAt    time.Time
	CompletedAt    time.Time
	RetryCount     int32
	MaxRetries     int32
	ResultPayload  []byte       // JSON-serialized result from device (optional)
	ErrorMessage   string       // Error details if action failed (optional)
}

// ActionStatus tracks action state
type ActionStatus int32

const (
	ActionStatusUnspecified ActionStatus = 0
	ActionStatusQueued      ActionStatus = 1
	ActionStatusDelivered   ActionStatus = 2
	ActionStatusProcessing  ActionStatus = 3
	ActionStatusCompleted   ActionStatus = 4
	ActionStatusFailed      ActionStatus = 5
	ActionStatusExpired     ActionStatus = 6
	ActionStatusCancelled   ActionStatus = 7
	ActionStatusFailedParsingResponse ActionStatus = 8
)

// String returns the string representation of ActionStatus
func (s ActionStatus) String() string {
	switch s {
	case ActionStatusQueued:
		return "QUEUED"
	case ActionStatusDelivered:
		return "DELIVERED"
	case ActionStatusProcessing:
		return "PROCESSING"
	case ActionStatusCompleted:
		return "COMPLETED"
	case ActionStatusFailed:
		return "FAILED"
	case ActionStatusExpired:
		return "EXPIRED"
	case ActionStatusCancelled:
		return "CANCELLED"
	case ActionStatusFailedParsingResponse:
		return "FAILED_PARSING_RESPONSE"
	default:
		return "UNSPECIFIED"
	}
}
