package data

import (
	"database/sql"
	"fmt"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	dbpkg "github.com/artpark-hub/taksa-platform/device-management/internal/db"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage/postgres"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage/sqlite"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// Data holds the database connection and storage layer
type Data struct {
	db *sql.DB
	store storage.Store
}

// NewData creates a new data instance with the configured database driver
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	if c == nil || c.Database == nil {
		return nil, nil, fmt.Errorf("data configuration is required")
	}

	// Create database connection using factory (supports both SQLite and PostgreSQL)
	db, err := dbpkg.NewDatabase(c)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize database schema for SQLite (PostgreSQL uses migrations)
	if c.Database.Driver == "sqlite3" {
		if err := dbpkg.InitializeSchema(db); err != nil {
			_ = db.Close()
			return nil, nil, fmt.Errorf("failed to initialize SQLite schema: %w", err)
		}
	}

	// Create appropriate storage implementation
	var store storage.Store
	switch c.Database.Driver {
	case "sqlite3":
		store, err = sqlite.NewStore(db)
	case "postgres":
		store, err = postgres.NewStore(db)
	default:
		_ = db.Close()
		return nil, nil, fmt.Errorf("unsupported database driver: %s (use 'sqlite3' or 'postgres')", c.Database.Driver)
	}

	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("failed to create store: %w", err)
	}

	d := &Data{
		db: db,
		store: store,
	}

	cleanup := func() {
		_ = d.Close()
	}

	return d, cleanup, nil
}

// Store returns the storage layer
func (d *Data) Store() storage.Store {
	return d.store
}

// Close closes the database connection
func (d *Data) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// NewStore extracts the store from Data for Wire injection
func NewStore(d *Data) storage.Store {
	return d.Store()
}

// NewProtocolConverterRepo creates a new protocol converter repository
func NewProtocolConverterRepo(d *Data) *ProtocolConverterRepo {
	return &ProtocolConverterRepo{data: d}
}

// NewDataModelRepo creates a new data model repository
func NewDataModelRepo(d *Data) *DataModelRepo {
	return &DataModelRepo{data: d}
}

// NewStreamProcessorRepo creates a new stream processor repository
func NewStreamProcessorRepo(d *Data) *StreamProcessorRepo {
	return &StreamProcessorRepo{data: d}
}

// ProviderSet is data providers
var ProviderSet = wire.NewSet(
	NewData,
	NewStore,
	NewProtocolConverterRepo,
	NewDataModelRepo,
	NewStreamProcessorRepo,
)
