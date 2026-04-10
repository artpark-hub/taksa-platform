package postgres

import (
	"database/sql"
	"fmt"

	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
)

// Store implements storage.Store for PostgreSQL backend
type Store struct {
	db                       *sql.DB
	devices                  *DeviceStore
	authTokens               *AuthTokenStore
	actions                  *ActionStore
	actionResponses          *ActionResponseStore
	messages                 *MessageStore
	messageTracking          storage.MessageTrackingStore
	actionMessageTracking    *ActionMessageTrackingStore
	certificates             *CertificateStore
	settings                 *SettingStore
}

// NewStore creates a new PostgreSQL-backed Store
func NewStore(db *sql.DB) (storage.Store, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	return &Store{
		db:                    db,
		devices:               &DeviceStore{db: db},
		authTokens:            &AuthTokenStore{db: db},
		actions:               &ActionStore{db: db},
		actionResponses:       &ActionResponseStore{db: db},
		messages:              &MessageStore{db: db},
		messageTracking:       newMessageTrackingStore(db),
		actionMessageTracking: newActionMessageTrackingStore(db),
		certificates:          NewCertificateStore(db),
		settings:              &SettingStore{db: db},
	}, nil
}

// Devices returns the device store
func (s *Store) Devices() storage.DeviceStore {
	return s.devices
}

// AuthTokens returns the auth token store
func (s *Store) AuthTokens() storage.AuthTokenStore {
	return s.authTokens
}

// Actions returns the action store
func (s *Store) Actions() storage.ActionStore {
	return s.actions
}

// Messages returns the message store
func (s *Store) Messages() storage.MessageStore {
	return s.messages
}

// MessageTracking returns the message tracking store
func (s *Store) MessageTracking() storage.MessageTrackingStore {
	return s.messageTracking
}

// ActionResponses returns the action response store
func (s *Store) ActionResponses() storage.ActionResponseStore {
	return s.actionResponses
}

// Certificates returns the certificate store
func (s *Store) Certificates() storage.CertificateStore {
	return s.certificates
}

// Settings returns the settings store
func (s *Store) Settings() storage.SettingStore {
	return s.settings
}

// ActionMessageTracking returns the action message tracking store
func (s *Store) ActionMessageTracking() storage.ActionMessageTracker {
	return s.actionMessageTracking
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// init registers the PostgreSQL store factory with the storage package
func init() {
	storage.RegisterPostgresStore(NewStore)
}
