package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresConfig holds PostgreSQL-specific configuration
type PostgresConfig struct {
	DSN             string // PostgreSQL connection string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds
	ConnMaxIdleTime int // seconds
}

// NewPostgresDatabase creates a new PostgreSQL database connection
func NewPostgresDatabase(config PostgresConfig) (*sql.DB, error) {
	// Open connection
	db, err := sql.Open("postgres", config.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)
	}
	if config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(config.ConnMaxIdleTime) * time.Second)
	}

	// Wait for PostgreSQL to accept connections (e.g. container startup / restarts).
	const (
		maxAttempts = 30
		interval    = time.Second
	)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = db.Ping()
		if lastErr == nil {
			return db, nil
		}
		if attempt < maxAttempts {
			time.Sleep(interval)
		}
	}
	_ = db.Close()
	return nil, fmt.Errorf("failed to ping PostgreSQL database after %d attempts: %w", maxAttempts, lastErr)
}
