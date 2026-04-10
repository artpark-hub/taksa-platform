package db

import (
	"database/sql"
	"fmt"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
)

// NewDatabase creates a database connection for PostgreSQL
func NewDatabase(dataConfig *conf.Data) (*sql.DB, error) {
	if dataConfig == nil || dataConfig.Database == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	dbConfig := dataConfig.Database

	if dbConfig.Driver != "postgres" {
		return nil, fmt.Errorf("unsupported database driver: %s (PostgreSQL only)", dbConfig.Driver)
	}

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

	// PostgreSQL schema is managed via migrations
	return db, nil
}
