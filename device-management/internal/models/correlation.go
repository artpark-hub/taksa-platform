package models

import "time"

// RequestCorrelation tracks a user's request to queue an action
// INTERNAL ONLY: Not sent to device or umh-core
// Used by device-management to track request-response correlation
type RequestCorrelation struct {
	RequestID string    // Unique per API request
	TraceID   string    // For distributed tracing
	DeviceID  string    // Target device
	ActionID  string    // The action that was created
	ActionType string   // Type of action
	Timestamp time.Time // When request was made
}

// MessageTracking maps device messages back to original actions
// INTERNAL ONLY: Not sent over wire
// Used when device sends responses (push) to correlate which action they relate to
type MessageTracking struct {
	MessageTraceID string    // The traceId in UMHMessage sent to device
	ActionID       string    // Which action this message contains
	DeviceID       string    // Which device
	PulledAt       time.Time // When device pulled this action
}

// ActionResponse tracks device responses to actions
// INTERNAL ONLY: Not sent over wire
// Stored when device sends response via Push API
type ActionResponse struct {
	ID            string    // Unique response ID
	ActionID      string    // Which action this responds to
	DeviceID      string    // Which device sent response
	MessageTraceID string   // The traceId that linked it back
	Content       string    // Response content (usually JSON)
	Status        string    // Response status (success, error, etc)
	CompletedAt   time.Time // When device completed action
}

// MessageDirection indicates the direction of message flow
// INTERNAL ONLY: Not sent over wire
type MessageDirection int

const (
	MessageDirectionUnspecified MessageDirection = iota
	MessageDirectionInbound                      // Device → Console (push)
	MessageDirectionOutbound                     // Console → Device (pull)
)

// Message represents communication between device and console
// INTERNAL ONLY: Not sent over wire
// Used for auditing and traceability of device communications
type Message struct {
	ID            string             // Unique message ID
	DeviceID      string             // Target/source device
	Type          string             // "status", "telemetry", "action", "error"
	Metadata      map[string]string  // Message metadata
	Content       string             // Base64 encoded payload
	TraceID       string             // Distributed tracing ID for correlation
	RequestID     string             // Request ID
	CorrelationID string             // Correlation ID
	Direction     MessageDirection   // Direction of message (INBOUND/OUTBOUND)
	CreatedAt     time.Time          // When message was created
	ExpiresAt     time.Time          // When message expires
}
