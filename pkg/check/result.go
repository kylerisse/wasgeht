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
	// For example, a ping check might set {"latency_us": 1234.0}.
	// An empty or nil map is valid for checks that only report success/failure.
	Metrics map[string]float64

	// Err holds any error encountered during check execution.
	// A non-nil Err generally corresponds to Success being false,
	// but the check implementation decides the semantics.
	Err error
}
