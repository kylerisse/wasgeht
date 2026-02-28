package check

import (
	"time"
)

// Result captures the outcome of a single check execution.
type Result struct {
	// Timestamp is when the check was executed.
	Timestamp time.Time

	// Success indicates whether the check passed.
	Success bool

	// Metrics holds named measurements from the check execution.
	// A nil pointer value for a key means the target was attempted but failed.
	// An absent key or nil map means no measurement was attempted.
	Metrics map[string]*int64

	// Err holds any error encountered during check execution.
	// A non-nil Err generally corresponds to Success being false,
	// but the check implementation decides the semantics.
	Err error
}
