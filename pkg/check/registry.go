package check

import (
	"fmt"
	"sync"
)

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
// The returned Check's Describe() method provides the instance-specific
// Descriptor declaring what metrics it produces.
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
