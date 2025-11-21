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

// ProviderFactory is a function that creates a new provider instance
type ProviderFactory func(config map[string]any) (Provider, error)

var (
	factories   = make(map[string]ProviderFactory)
	factoriesMu sync.RWMutex
)

// RegisterFactory registers a provider factory function
func RegisterFactory(name string, factory ProviderFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[name] = factory
}

// CreateProvider creates a provider from a factory
func CreateProvider(name string, config map[string]any) (Provider, error) {
	factoriesMu.RLock()
	factory, ok := factories[name]
	factoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("provider factory '%s' not found", name)
	}
	return factory(config)
}

// DetectProvider detects the provider type from model name
func DetectProvider(model string) string {
	// Check for HuggingFace model patterns
	hfPatterns := []string{
		"meta-llama/", "mistralai/", "tiiuae/", "EleutherAI/",
		"bigscience/", "facebook/", "google/", "microsoft/",
	}

	for _, pattern := range hfPatterns {
		if len(model) >= len(pattern) && model[:len(pattern)] == pattern {
			return "huggingface"
		}
	}

	// Check for OpenAI models
	openaiModels := []string{"gpt-3.5", "gpt-4", "text-davinci", "text-curie"}
	for _, prefix := range openaiModels {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return "openai"
		}
	}

	// Check for Anthropic models
	if len(model) >= 6 && model[:6] == "claude" {
		return "anthropic"
	}

	// Default to OpenAI-compatible
	return "openai"
}
