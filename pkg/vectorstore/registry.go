package vectorstore

import (
	"fmt"
	"sync"
)

// ProviderFactory is a function that creates a VectorStore from a Config.
type ProviderFactory func(config Config) (VectorStore, error)

// registry holds all registered vector store providers.
var (
	registry = make(map[string]ProviderFactory)
	mu       sync.RWMutex
)

// Register adds a new vector store provider to the registry.
// This allows you to add custom vector store implementations.
//
// Example:
//
//	func init() {
//	    vectorstore.Register("custom", func(config vectorstore.Config) (vectorstore.VectorStore, error) {
//	        return NewCustomVectorStore(config)
//	    })
//	}
func Register(name string, factory ProviderFactory) {
	mu.Lock()
	defer mu.Unlock()

	if factory == nil {
		panic("vectorstore: Register factory is nil")
	}
	if _, dup := registry[name]; dup {
		panic("vectorstore: Register called twice for provider " + name)
	}
	registry[name] = factory
}

// New creates a new VectorStore based on the provider specified in the config.
//
// Example:
//
//	config := vectorstore.Config{
//	    Provider: "firestore",
//	    EmbeddingDimensions: 768,
//	    Firestore: &vectorstore.FirestoreConfig{
//	        ProjectID: "my-project",
//	        Collection: "embeddings",
//	    },
//	}
//	store, err := vectorstore.New(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
func New(config Config) (VectorStore, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	mu.RLock()
	factory, ok := registry[config.Provider]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown vector store provider: %s (available: %v)", config.Provider, ListProviders())
	}

	return factory(config)
}

// ListProviders returns a list of all registered vector store providers.
func ListProviders() []string {
	mu.RLock()
	defer mu.RUnlock()

	providers := make([]string, 0, len(registry))
	for name := range registry {
		providers = append(providers, name)
	}
	return providers
}

// IsRegistered checks if a provider is registered.
func IsRegistered(name string) bool {
	mu.RLock()
	defer mu.RUnlock()

	_, ok := registry[name]
	return ok
}

// Unregister removes a provider from the registry.
// This is primarily useful for testing.
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()

	delete(registry, name)
}
