package storage

import (
	"context"
	"database/sql"
	"time"
)

// ActionMessageTracking represents a request-response correlation record for traceability
type ActionMessageTracking struct {
	ID                 string     // Primary key
	ActionID           string     // Foreign key to actions
	DeviceID           string     // Foreign key to devices
	TraceID            string     // Generated trace_id sent to device
	TraceGeneratedAt   time.Time  // When trace was created
	ResponseTraceID    string     // Echo from device response (should match TraceID)
	ResponseMessageID  string     // ID of the response message (direct pointer for O(1) lookup)
	ResponseReceivedAt *time.Time // When response arrived
	CorrelationStatus  int        // 1=PENDING, 2=IN_FLIGHT, 3=RESPONSE_RECEIVED, 4=COMPLETED
	CreatedAt          time.Time
	CompletedAt        *time.Time
}

// ActionMessageTracker handles request-response correlation tracking for traceability
type ActionMessageTracker interface {
	// Create stores a new action message tracking record
	// Called during Pull when action is sent to device
	Create(ctx context.Context, track *ActionMessageTracking) error

	// GetByTraceID retrieves tracking record by trace_id
	// Called during Push to correlate response back to action
	GetByTraceID(ctx context.Context, traceID string) (*ActionMessageTracking, error)

	// GetByActionID retrieves tracking record by action_id
	GetByActionID(ctx context.Context, actionID string) (*ActionMessageTracking, error)

	// UpdateResponse updates response side of tracking record
	// Called when device response is received (Push)
	// Sets response_trace_id, response_received_at, correlation_status
	UpdateResponse(ctx context.Context, id string, responseTraceID string, correlationStatus int) error

	// UpdateResponseWithMessageID updates response side with message ID (O(1) lookup pointer)
	// Called when device response is received (Push) with actionUUID correlation
	// Sets response_message_id, response_received_at, correlation_status
	UpdateResponseWithMessageID(ctx context.Context, id string, messageID string, correlationStatus int) error

	// UpdateCompleted marks tracking as completed
	UpdateCompleted(ctx context.Context, id string) error

	// ListPendingCorrelations returns uncorrelated responses
	// Used for debugging and cleanup
	ListPendingCorrelations(ctx context.Context, deviceID string) ([]*ActionMessageTracking, error)

	// ListByDevice returns all tracking records for a device
	// Used for audit trails and diagnostics
	ListByDevice(ctx context.Context, deviceID string) ([]*ActionMessageTracking, error)
}

// Store is the root interface that holds all storage operations
type Store interface {
	// Device store
	Devices() DeviceStore

	// Auth token store
	AuthTokens() AuthTokenStore

	// Action store
	Actions() ActionStore

	// Message store
	Messages() MessageStore

	// Certificate store
	Certificates() CertificateStore

	// Setting store
	Settings() SettingStore

	// Message tracking store - for request-response correlation
	MessageTracking() MessageTrackingStore

	// Action response store - for storing correlated device responses
	ActionResponses() ActionResponseStore

	// Action message tracking store - for traceability
	ActionMessageTracking() ActionMessageTracker

	// Action workflows - composite orchestration (facade deploy + configure).
	ActionWorkflows() ActionWorkflowStore

	// Close closes the underlying database connection
	Close() error
}

// NewStore creates a new Store with PostgreSQL backend
// This is initialized by data package after postgres is imported
func NewStore(db *sql.DB) (Store, error) {
	return newPostgresStore(db)
}

// newPostgresStore is the factory function set by postgres subpackage
var newPostgresStore func(db *sql.DB) (Store, error)

// RegisterPostgresStore registers the PostgreSQL store factory
// Called from postgres subpackage's init
func RegisterPostgresStore(factory func(db *sql.DB) (Store, error)) {
	newPostgresStore = factory
}
