package db

import (
	"database/sql"
	"fmt"
	"time"

	"taksa-platform-dm/internal/conf"
)

// NewDatabase creates a database connection based on the driver specified in config
// Supports both SQLite and PostgreSQL
func NewDatabase(dataConfig *conf.Data) (*sql.DB, error) {
	if dataConfig == nil || dataConfig.Database == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	dbConfig := dataConfig.Database

	switch dbConfig.Driver {
	case "sqlite3":
		database, err := NewSQLiteDatabase(Config{
			Path:            dbConfig.Source,
			MaxOpenConns:    int(dbConfig.MaxOpenConns),
			MaxIdleConns:    int(dbConfig.MaxIdleConns),
			ConnMaxLifetime: time.Duration(dbConfig.ConnMaxLifetime) * time.Second,
			ConnMaxIdleTime: time.Duration(dbConfig.ConnMaxIdleTime) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		return database.DB, nil

	case "postgres":
		db, err := NewPostgresDatabase(PostgresConfig{
			DSN:             dbConfig.Source,
			MaxOpenConns:    int(dbConfig.MaxOpenConns),
			MaxIdleConns:    int(dbConfig.MaxIdleConns),
			ConnMaxLifetime: int(dbConfig.ConnMaxLifetime),
			ConnMaxIdleTime: int(dbConfig.ConnMaxIdleTime),
		})
		if err != nil {
			return nil, err
		}
		// Note: PostgreSQL schema should be initialized via migration tool (Flyway, golang-migrate)
		// For development, schema.postgres.sql can be imported manually
		return db, nil

	default:
		return nil, fmt.Errorf("unsupported database driver: %s (use 'sqlite3' or 'postgres')", dbConfig.Driver)
	}
}
