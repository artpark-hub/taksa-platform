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

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	return db, nil
}
