package sqlite

import (
	"database/sql"
	"sync"

	"taksa-platform-dm/internal/storage"
)

// SQLiteStore implements storage.Store interface using SQLite backend
type SQLiteStore struct {
	db *sql.DB

	// Individual stores (lazy-loaded)
	devicesMu                      sync.RWMutex
	devicesStore                   storage.DeviceStore
	authTokensMu                   sync.RWMutex
	authTokensStore                storage.AuthTokenStore
	actionsMu                      sync.RWMutex
	actionsStore                   storage.ActionStore
	messagesMu                     sync.RWMutex
	messagesStore                  storage.MessageStore
	certificatesMu                 sync.RWMutex
	certificatesStore              storage.CertificateStore
	settingsMu                     sync.RWMutex
	settingsStore                  storage.SettingStore
	messageTrackingMu              sync.RWMutex
	messageTrackingStore           storage.MessageTrackingStore
	actionResponsesMu              sync.RWMutex
	actionResponsesStore           storage.ActionResponseStore
	actionMessageTrackingMu        sync.RWMutex
	actionMessageTrackingStore     storage.ActionMessageTracker
}

// NewStore creates a new SQLiteStore
func NewStore(db *sql.DB) (storage.Store, error) {
	if db == nil {
		return nil, ErrNilDatabase
	}

	return &SQLiteStore{
		db: db,
	}, nil
}

// Devices returns the device store (lazy-loaded)
func (s *SQLiteStore) Devices() storage.DeviceStore {
	s.devicesMu.Lock()
	defer s.devicesMu.Unlock()

	if s.devicesStore == nil {
		s.devicesStore = &DeviceStore{db: s.db}
	}
	return s.devicesStore
}

// AuthTokens returns the auth token store (lazy-loaded)
func (s *SQLiteStore) AuthTokens() storage.AuthTokenStore {
	s.authTokensMu.Lock()
	defer s.authTokensMu.Unlock()

	if s.authTokensStore == nil {
		s.authTokensStore = &AuthTokenStore{db: s.db}
	}
	return s.authTokensStore
}

// Actions returns the action store (lazy-loaded)
func (s *SQLiteStore) Actions() storage.ActionStore {
	s.actionsMu.Lock()
	defer s.actionsMu.Unlock()

	if s.actionsStore == nil {
		s.actionsStore = &ActionStore{db: s.db}
	}
	return s.actionsStore
}

// Messages returns the message store (lazy-loaded)
func (s *SQLiteStore) Messages() storage.MessageStore {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	if s.messagesStore == nil {
		s.messagesStore = &MessageStore{db: s.db}
	}
	return s.messagesStore
}

// Certificates returns the certificate store (lazy-loaded)
func (s *SQLiteStore) Certificates() storage.CertificateStore {
	s.certificatesMu.Lock()
	defer s.certificatesMu.Unlock()

	if s.certificatesStore == nil {
		s.certificatesStore = NewCertificateStore(s.db)
	}
	return s.certificatesStore
}

// Settings returns the settings store (lazy-loaded)
func (s *SQLiteStore) Settings() storage.SettingStore {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	if s.settingsStore == nil {
		s.settingsStore = &SettingStore{db: s.db}
	}
	return s.settingsStore
}

// MessageTracking returns the message tracking store (lazy-loaded)
func (s *SQLiteStore) MessageTracking() storage.MessageTrackingStore {
	s.messageTrackingMu.Lock()
	defer s.messageTrackingMu.Unlock()

	if s.messageTrackingStore == nil {
		s.messageTrackingStore = newMessageTrackingStore(s.db)
	}
	return s.messageTrackingStore
}

// ActionResponses returns the action responses store (lazy-loaded)
func (s *SQLiteStore) ActionResponses() storage.ActionResponseStore {
	s.actionResponsesMu.Lock()
	defer s.actionResponsesMu.Unlock()

	if s.actionResponsesStore == nil {
		s.actionResponsesStore = newActionResponseStore(s.db)
	}
	return s.actionResponsesStore
}

// ActionMessageTracking returns the action message tracking store (lazy-loaded)
func (s *SQLiteStore) ActionMessageTracking() storage.ActionMessageTracker {
	s.actionMessageTrackingMu.Lock()
	defer s.actionMessageTrackingMu.Unlock()

	if s.actionMessageTrackingStore == nil {
		s.actionMessageTrackingStore = newActionMessageTrackingStore(s.db)
	}
	return s.actionMessageTrackingStore
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func init() {
	// Register the SQLite store factory
	storage.RegisterSQLiteStore(NewStore)
}
