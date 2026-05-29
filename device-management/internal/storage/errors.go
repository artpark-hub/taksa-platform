package storage

import "errors"

var (
	// ErrActionNotCancellable is returned when cancel is attempted on a non-QUEUED action.
	ErrActionNotCancellable = errors.New("action not cancellable")
)
