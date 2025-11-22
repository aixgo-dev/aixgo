package embeddings

import (
	"context"
	"fmt"
	"sync"
)

// EmbeddingService is the main interface for generating text embeddings.
type EmbeddingService interface {
	// Embed generates embeddings for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the dimension size of the embeddings
	Dimensions() int

	// ModelName returns the name of the embedding model
	ModelName() string

	// Close closes any resources held by the service
	Close() error
}

// Config holds configuration for embedding providers.
type Config struct {
	// Provider specifies which embedding service to use
	// Supported values: "openai", "huggingface", "huggingface_tei"
	Provider string `yaml:"provider" json:"provider"`

	// OpenAI-specific configuration
	OpenAI *OpenAIConfig `yaml:"openai,omitempty" json:"openai,omitempty"`

	// HuggingFace-specific configuration
	HuggingFace *HuggingFaceConfig `yaml:"huggingface,omitempty" json:"huggingface,omitempty"`

	// HuggingFaceTEI-specific configuration (Text Embeddings Inference)
	HuggingFaceTEI *HuggingFaceTEIConfig `yaml:"huggingface_tei,omitempty" json:"huggingface_tei,omitempty"`
}

// OpenAIConfig contains OpenAI-specific embedding settings.
type OpenAIConfig struct {
	// APIKey for authentication
	APIKey string `yaml:"api_key" json:"api_key"`

	// Model specifies which OpenAI embedding model to use
	// Options: "text-embedding-3-small" (1536 dims), "text-embedding-3-large" (3072 dims)
	Model string `yaml:"model" json:"model"`

	// BaseURL is the API endpoint (default: https://api.openai.com/v1)
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// Dimensions allows reducing embedding dimensions (only for text-embedding-3 models)
	Dimensions int `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
}

// HuggingFaceConfig contains HuggingFace Inference API settings.
type HuggingFaceConfig struct {
	// APIKey for authentication (optional for public models)
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"`

	// Model specifies which HuggingFace model to use
	// Popular options:
	//   - "sentence-transformers/all-MiniLM-L6-v2" (384 dims, fast)
	//   - "BAAI/bge-small-en-v1.5" (384 dims)
	//   - "BAAI/bge-large-en-v1.5" (1024 dims)
	//   - "thenlper/gte-large" (1024 dims)
	Model string `yaml:"model" json:"model"`

	// Endpoint is the API endpoint (default: https://api-inference.huggingface.co)
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`

	// WaitForModel waits if model is loading (default: true)
	WaitForModel bool `yaml:"wait_for_model" json:"wait_for_model"`

	// UseCache uses cached results (default: true)
	UseCache bool `yaml:"use_cache" json:"use_cache"`
}

// HuggingFaceTEIConfig contains HuggingFace Text Embeddings Inference settings.
// TEI is a self-hosted, high-performance embedding server.
type HuggingFaceTEIConfig struct {
	// Endpoint is the TEI server URL (e.g., "http://localhost:8080")
	Endpoint string `yaml:"endpoint" json:"endpoint"`

	// Model name (informational, server determines actual model)
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// Normalize returns normalized embeddings (default: true)
	Normalize bool `yaml:"normalize" json:"normalize"`
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must be specified")
	}

	switch c.Provider {
	case "openai":
		if c.OpenAI == nil {
			return fmt.Errorf("openai configuration is required when provider is 'openai'")
		}
		return c.OpenAI.Validate()
	case "huggingface":
		if c.HuggingFace == nil {
			return fmt.Errorf("huggingface configuration is required when provider is 'huggingface'")
		}
		return c.HuggingFace.Validate()
	case "huggingface_tei":
		if c.HuggingFaceTEI == nil {
			return fmt.Errorf("huggingface_tei configuration is required when provider is 'huggingface_tei'")
		}
		return c.HuggingFaceTEI.Validate()
	default:
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}

// Validate checks if OpenAI configuration is valid.
func (oc *OpenAIConfig) Validate() error {
	if oc.APIKey == "" {
		return fmt.Errorf("openai api_key is required")
	}
	if oc.Model == "" {
		oc.Model = "text-embedding-3-small" // Default
	}
	if oc.BaseURL == "" {
		oc.BaseURL = "https://api.openai.com/v1"
	}
	return nil
}

// Validate checks if HuggingFace configuration is valid.
func (hc *HuggingFaceConfig) Validate() error {
	if hc.Model == "" {
		return fmt.Errorf("huggingface model is required")
	}
	if hc.Endpoint == "" {
		hc.Endpoint = "https://api-inference.huggingface.co"
	}
	return nil
}

// Validate checks if HuggingFaceTEI configuration is valid.
func (tc *HuggingFaceTEIConfig) Validate() error {
	if tc.Endpoint == "" {
		return fmt.Errorf("huggingface_tei endpoint is required")
	}
	return nil
}

// ProviderFactory is a function that creates an EmbeddingService from a Config.
type ProviderFactory func(config Config) (EmbeddingService, error)

// registry holds all registered embedding providers.
var (
	registry = make(map[string]ProviderFactory)
	mu       sync.RWMutex
)

// Register adds a new embedding provider to the registry.
func Register(name string, factory ProviderFactory) {
	mu.Lock()
	defer mu.Unlock()

	if factory == nil {
		panic("embeddings: Register factory is nil")
	}
	if _, dup := registry[name]; dup {
		panic("embeddings: Register called twice for provider " + name)
	}
	registry[name] = factory
}

// New creates a new EmbeddingService based on the provider specified in the config.
func New(config Config) (EmbeddingService, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	mu.RLock()
	factory, ok := registry[config.Provider]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown embedding provider: %s (available: %v)", config.Provider, ListProviders())
	}

	return factory(config)
}

// ListProviders returns a list of all registered embedding providers.
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
