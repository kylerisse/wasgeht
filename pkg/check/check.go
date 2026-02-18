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

// MetricDef describes a single metric produced by a check type.
type MetricDef struct {
	// ResultKey is the key used in Result.Metrics (e.g. "latency_us").
	ResultKey string

	// DSName is the RRD data source name (e.g. "latency").
	DSName string
}

// Descriptor declares static metadata about a check type, including
// what metrics it produces. This is registered alongside the Factory
// so the system can generically wire up storage and graphs without
// per-type knowledge.
type Descriptor struct {
	// Metrics lists the metrics this check type produces.
	Metrics []MetricDef
}

// Factory is a function that creates a Check from a raw configuration map.
// Each check type registers a Factory with the Registry.
type Factory func(config map[string]any) (Check, error)

// registration bundles a Factory with its Descriptor.
type registration struct {
	factory    Factory
	descriptor Descriptor
}

// Registry holds registered check types and their factories.
// It is safe for concurrent use.
type Registry struct {
	mu            sync.RWMutex
	registrations map[string]registration
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		registrations: make(map[string]registration),
	}
}

// Register adds a check type factory and descriptor under the given name.
// Returns an error if the name is already registered.
func (r *Registry) Register(name string, factory Factory, desc Descriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.registrations[name]; exists {
		return fmt.Errorf("check type %q is already registered", name)
	}
	r.registrations[name] = registration{factory: factory, descriptor: desc}
	return nil
}

// Create instantiates a Check of the given type using the provided config.
// Returns an error if the type is not registered or the factory fails.
func (r *Registry) Create(name string, config map[string]any) (Check, error) {
	r.mu.RLock()
	reg, exists := r.registrations[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown check type %q", name)
	}
	return reg.factory(config)
}

// Describe returns the Descriptor for the given check type.
// Returns an error if the type is not registered.
func (r *Registry) Describe(name string) (Descriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reg, exists := r.registrations[name]
	if !exists {
		return Descriptor{}, fmt.Errorf("unknown check type %q", name)
	}
	return reg.descriptor, nil
}

// Types returns the names of all registered check types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.registrations))
	for name := range r.registrations {
		types = append(types, name)
	}
	return types
}
