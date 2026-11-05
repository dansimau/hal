package hal

import "errors"

var (
	ErrEntityNotRegistered = errors.New("entity not registered")

	// ErrNegativeFlushInterval is returned by parseFlushInterval when the
	// FlushIntervalEnv value is a negative duration.
	ErrNegativeFlushInterval = errors.New("flush interval must not be negative")
)
