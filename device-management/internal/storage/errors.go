package storage

import "errors"

var (
	// ErrNotFound is returned when a record is not found.
	ErrNotFound = errors.New("record not found")

	// ErrActionNotCancellable is returned when cancel is attempted on a non-QUEUED action.
	ErrActionNotCancellable = errors.New("action not cancellable")
)
