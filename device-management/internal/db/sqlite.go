package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Config holds database configuration
type Config struct {
	Path            string        // SQLite database file path
	MaxOpenConns    int           // Maximum open connections
	MaxIdleConns    int           // Maximum idle connections
	ConnMaxLifetime time.Duration // Maximum connection lifetime
	ConnMaxIdleTime time.Duration // Maximum idle time
}

// DefaultConfig returns default database configuration
func DefaultConfig() Config {
	return Config{
		Path:            "./data/taksa.db",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 10 * time.Second,
	}
}

// Database wraps *sql.DB with connection pooling and initialization
type Database struct {
	*sql.DB
	config Config
	once   sync.Once
	err    error
}

var (
	dbOnce sync.Once
	dbInst *Database
)

// NewSQLiteDatabase creates a new SQLite database connection with the given config
func NewSQLiteDatabase(config Config) (*Database, error) {
	// Open connection
	db, err := sql.Open("sqlite3", config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enable foreign keys (important for referential integrity)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode (better for concurrent access)
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	return &Database{
		DB:     db,
		config: config,
	}, nil
}

// Singleton returns the global database instance
func Singleton() *Database {
	return dbInst
}

// InitializeSingleton initializes the global database instance
func InitializeSingleton(config Config) (*Database, error) {
	var err error
	dbOnce.Do(func() {
		dbInst, err = NewSQLiteDatabase(config)
	})
	return dbInst, err
}

// Close closes the database connection
func (d *Database) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}

// BeginTx starts a new transaction
func (d *Database) BeginTx() (*sql.Tx, error) {
	return d.DB.Begin()
}

// HealthCheck verifies database connectivity with context timeout
func (d *Database) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.DB.PingContext(ctx)
}

// Stats returns connection pool statistics
func (d *Database) Stats() sql.DBStats {
	return d.DB.Stats()
}
