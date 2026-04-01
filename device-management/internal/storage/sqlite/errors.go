package sqlite

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNilDatabase is returned when database is nil
	ErrNilDatabase = errors.New("database is nil")

	// ErrNotFound is returned when a record is not found
	ErrNotFound = errors.New("record not found")

	// ErrAlreadyExists is returned when trying to create a duplicate record
	ErrAlreadyExists = errors.New("record already exists")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")

	// ErrDatabaseError is returned for database operation errors
	ErrDatabaseError = errors.New("database error")

	// ErrTokenExpired is returned when a token has expired
	ErrTokenExpired = errors.New("token expired")

	// ErrInvalidToken is returned when a token is invalid
	ErrInvalidToken = errors.New("invalid token")
)

// Helper functions for storage layer

// generateUUID generates a UUID string (placeholder)
// TODO: Use github.com/google/uuid in production
func generateUUID() string {
	return fmt.Sprintf("gen-%d", time.Now().UnixNano())
}

// optionalTimeValue converts a time.Time to RFC3339 format, or empty string if zero
// Returns string so it can be passed to database/sql
func optionalTimeValue(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339)
}
