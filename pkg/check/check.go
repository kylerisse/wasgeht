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
// The Registry provides type discovery, allowing check types to be
// registered by name and instantiated from configuration at runtime.
package check

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Check is the interface that all monitoring check types must implement.
type Check interface {
	// Type returns the registered name of this check type (e.g. "ping", "http").
	Type() string

	// Run executes the check and returns a Result.
	// The provided context can be used for cancellation and timeouts.
	Run(ctx context.Context) Result
}

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

// Factory is a function that creates a Check from a raw configuration map.
// Each check type registers a Factory with the Registry.
type Factory func(config map[string]any) (Check, error)

// Registry holds registered check types and their factories.
// It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a check type factory under the given name.
// Returns an error if the name is already registered.
func (r *Registry) Register(name string, factory Factory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("check type %q is already registered", name)
	}
	r.factories[name] = factory
	return nil
}

// Create instantiates a Check of the given type using the provided config.
// Returns an error if the type is not registered or the factory fails.
func (r *Registry) Create(name string, config map[string]any) (Check, error) {
	r.mu.RLock()
	factory, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown check type %q", name)
	}
	return factory(config)
}

// Types returns the names of all registered check types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.factories))
	for name := range r.factories {
		types = append(types, name)
	}
	return types
}
