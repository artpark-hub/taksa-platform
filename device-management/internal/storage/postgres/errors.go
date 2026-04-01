package postgres

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
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

// optionalTime converts a timestamp to RFC3339 format, or nil if nil
// Returns interface{} so it can be passed to database/sql which will convert it to SQL NULL
func optionalTime(ts *timestamppb.Timestamp) interface{} {
	if ts == nil {
		return nil
	}
	return ts.AsTime().Format(time.RFC3339)
}

// optionalTimeValue converts a time.Time to RFC3339 format, or nil if zero
// Returns interface{} so it can be passed to database/sql which will convert it to SQL NULL
func optionalTimeValue(ts time.Time) interface{} {
	if ts.IsZero() {
		return nil
	}
	return ts.Format(time.RFC3339)
}
