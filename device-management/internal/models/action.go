package models

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// ActionTypeSubscribe is the edge status subscription keepalive action.
	ActionTypeSubscribe = "subscribe"
	// NATSMirrorPayloadMarker identifies UNS→NATS mirror deploy/edit action payloads.
	NATSMirrorPayloadMarker = "UNS-to-NATS-mirror"
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

// IsNATSMirrorDeployOrEditPayload reports whether JSON is a deploy/edit payload for the
// platform UNS→NATS mirror DFC (top-level "name" must equal NATSMirrorPayloadMarker).
func IsNATSMirrorDeployOrEditPayload(payloadJSON []byte) bool {
	if len(payloadJSON) == 0 {
		return false
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(payloadJSON, &top); err != nil {
		return false
	}
	rawName, ok := top["name"]
	if !ok {
		return false
	}
	var name string
	if err := json.Unmarshal(rawName, &name); err != nil {
		return false
	}
	return name == NATSMirrorPayloadMarker
}

// ExcludedFromAutoExpire reports whether TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES must not apply.
// Infrastructure keepalive actions use per-action TTL and their own re-queue logic instead.
func (a *Action) ExcludedFromAutoExpire() bool {
	if a == nil {
		return false
	}
	if a.Type == ActionTypeSubscribe {
		return true
	}
	if a.Type != "deploy-data-flow-component" && a.Type != "edit-data-flow-component" {
		return false
	}
	if a.Payload == nil || len(a.Payload.Value) == 0 {
		return false
	}
	return IsNATSMirrorDeployOrEditPayload(a.Payload.Value)
}

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
