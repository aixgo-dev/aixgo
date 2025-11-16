package provider

import (
	"fmt"
	"sync"
)

// Registry manages LLM providers
type Registry struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider
func (r *Registry) Register(name string, provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// Get retrieves a provider by name
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", name)
	}

	return provider, nil
}

// Has checks if a provider is registered
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[name]
	return ok
}

// List returns all registered provider names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Global registry
var globalRegistry = NewRegistry()

// Register registers a provider globally
func Register(name string, provider Provider) {
	globalRegistry.Register(name, provider)
}

// Get retrieves a provider from the global registry
func Get(name string) (Provider, error) {
	return globalRegistry.Get(name)
}

// Has checks if a provider exists in the global registry
func Has(name string) bool {
	return globalRegistry.Has(name)
}

// List returns all registered provider names from the global registry
func List() []string {
	return globalRegistry.List()
}
