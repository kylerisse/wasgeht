package check

import (
	"fmt"
	"sync"
)

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
