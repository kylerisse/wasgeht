// Package check defines the core interfaces and types for monitoring checks.
//
// A Check represents a single monitoring probe that can be executed against
// a target. Different check types (ping, HTTP, TCP, DNS, etc.) implement
// the Check interface with their own logic and configuration.
//
// Results from check execution are captured in a Result struct, which
// provides a uniform shape regardless of check type: success/failure,
// a set of named metrics, and an optional error.
//
// Each check type provides a Descriptor that declares what metrics it
// produces, allowing the system to generically wire up storage and
// visualization without per-type knowledge.
//
// The Registry provides type discovery, allowing check types to be
// registered by name and instantiated from configuration at runtime.
package check

import (
	"context"
)

// Check is the interface that all monitoring check types must implement.
type Check interface {
	// Type returns the registered name of this check type (e.g. "ping", "http").
	Type() string

	// Run executes the check and returns a Result.
	// The provided context can be used for cancellation and timeouts.
	Run(ctx context.Context) Result
}
