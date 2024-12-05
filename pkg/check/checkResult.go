package check

import (
	"time"
)

// CheckResult holds the result of a check execution.
type Result struct {
	// Status represents the outcome of the check.
	Status Status `json:"status"`

	// Metrics contains various metrics collected by the check.
	Metrics Metrics `json:"metrics"`

	// LastUpdated indicates when the check was last executed.
	LastUpdated time.Time `json:"last_updated"`
}
