package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InitializeSchema initializes the database schema from schema.sqlite3.sql
// This creates all base tables and indexes. Migrations are applied after if available.
func InitializeSchema(db *sql.DB) error {
	schemaSQL, err := readSchemaFile("db/schema.sqlite3.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	if _, err := db.Exec(string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Apply any available migrations (for version upgrades)
	if err := applyMigrations(db); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// readSchemaFile reads schema.sqlite3.sql from multiple possible locations
func readSchemaFile(filename string) ([]byte, error) {
	possiblePaths := []string{
		filename,                                                     // Relative path
		"../" + filename,                                             // One level up
		"../../" + filename,                                          // Two levels up
		"/home/rajeevb/projects/taksa-platform-dm/" + filename,      // Absolute path
	}

	for _, path := range possiblePaths {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("could not find %s in any expected location", filename)
}

// applyMigrations applies all .sql migration files sequentially from the migrations directory
// Migrations are optional (for fresh installs) and are typically used for version upgrades
// Migration files should be named sequentially (e.g., 001_*.sql, 002_*.sql, etc.)
func applyMigrations(db *sql.DB) error {
	migrationDirs := []string{
		"migrations",
		"../migrations",
		"../../migrations",
		"db/migrations",
		"../db/migrations",
		"../../db/migrations",
	}

	var files []os.FileInfo
	var foundDir string

	// Find the migrations directory
	for _, dir := range migrationDirs {
		entries, err := os.ReadDir(dir)
		if err == nil {
			foundDir = dir
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
					info, _ := entry.Info()
					files = append(files, info)
				}
			}
			break
		}
	}

	// If no migrations directory found, that's okay (fresh install)
	if foundDir == "" || len(files) == 0 {
		return nil
	}

	// Sort migration files by name
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Apply each migration
	for _, file := range files {
		migPath := filepath.Join(foundDir, file.Name())
		migSQL, err := os.ReadFile(migPath)
		if err != nil {
			log.Printf("WARNING: Failed to read migration file %s: %v", migPath, err)
			continue
		}

		log.Printf("Applying migration: %s", file.Name())
		// Execute migration - errors are expected (table might exist, etc)
		// Continue on error rather than failing
		if _, err := db.Exec(string(migSQL)); err != nil {
			log.Printf("WARNING: Migration %s failed: %v (may already be applied)", file.Name(), err)
			continue
		}
		log.Printf("Migration %s applied successfully", file.Name())
	}

	return nil
}

// CheckSchema verifies that all required tables exist
func CheckSchema(db *sql.DB) error {
	tables := []string{
		"devices",
		"auth_tokens",
		"actions",
		"messages",
		"certificates",
		"device_certificates",
		"user_certificates",
		"settings",
	}

	for _, table := range tables {
		var exists bool
		err := db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)",
			table,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	return nil
}
