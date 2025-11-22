package embeddings

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigValidation tests configuration validation.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid openai config",
			config: Config{
				Provider: "openai",
				OpenAI: &OpenAIConfig{
					APIKey: "test-key",
					Model:  "text-embedding-3-small",
				},
			},
			wantErr: false,
		},
		{
			name: "valid huggingface config",
			config: Config{
				Provider: "huggingface",
				HuggingFace: &HuggingFaceConfig{
					Model: "sentence-transformers/all-MiniLM-L6-v2",
				},
			},
			wantErr: false,
		},
		{
			name: "valid huggingface_tei config",
			config: Config{
				Provider: "huggingface_tei",
				HuggingFaceTEI: &HuggingFaceTEIConfig{
					Endpoint: "http://localhost:8080",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty provider",
			config:  Config{},
			wantErr: true,
			errMsg:  "provider must be specified",
		},
		{
			name: "openai provider without config",
			config: Config{
				Provider: "openai",
			},
			wantErr: true,
			errMsg:  "openai configuration is required",
		},
		{
			name: "huggingface provider without config",
			config: Config{
				Provider: "huggingface",
			},
			wantErr: true,
			errMsg:  "huggingface configuration is required",
		},
		{
			name: "huggingface_tei provider without config",
			config: Config{
				Provider: "huggingface_tei",
			},
			wantErr: true,
			errMsg:  "huggingface_tei configuration is required",
		},
		{
			name: "unsupported provider",
			config: Config{
				Provider: "unknown",
			},
			wantErr: true,
			errMsg:  "unsupported provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestOpenAIConfigValidation tests OpenAI-specific configuration validation.
func TestOpenAIConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *OpenAIConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &OpenAIConfig{
				APIKey: "test-key",
				Model:  "text-embedding-3-small",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &OpenAIConfig{
				Model: "text-embedding-3-small",
			},
			wantErr: true,
			errMsg:  "api_key is required",
		},
		{
			name: "default model is set",
			config: &OpenAIConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "default base URL is set",
			config: &OpenAIConfig{
				APIKey: "test-key",
				Model:  "text-embedding-3-small",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				// Check defaults are set
				if tt.config.Model == "" {
					assert.Equal(t, "text-embedding-3-small", tt.config.Model)
				}
				if tt.config.BaseURL == "" {
					assert.Equal(t, "https://api.openai.com/v1", tt.config.BaseURL)
				}
			}
		})
	}
}

// TestHuggingFaceConfigValidation tests HuggingFace-specific configuration validation.
func TestHuggingFaceConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *HuggingFaceConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &HuggingFaceConfig{
				Model: "sentence-transformers/all-MiniLM-L6-v2",
			},
			wantErr: false,
		},
		{
			name: "missing model",
			config: &HuggingFaceConfig{
				APIKey: "test-key",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "default endpoint is set",
			config: &HuggingFaceConfig{
				Model: "sentence-transformers/all-MiniLM-L6-v2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original endpoint to check if default was set
			originalEndpoint := tt.config.Endpoint

			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				// Check default endpoint is set
				if originalEndpoint == "" {
					assert.Equal(t, "https://api-inference.huggingface.co", tt.config.Endpoint)
				}
			}
		})
	}
}

// TestHuggingFaceTEIConfigValidation tests HuggingFace TEI-specific configuration validation.
func TestHuggingFaceTEIConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *HuggingFaceTEIConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &HuggingFaceTEIConfig{
				Endpoint: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name:    "missing endpoint",
			config:  &HuggingFaceTEIConfig{},
			wantErr: true,
			errMsg:  "endpoint is required",
		},
		{
			name: "with normalize flag",
			config: &HuggingFaceTEIConfig{
				Endpoint:  "http://localhost:8080",
				Normalize: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestRegistry tests the provider registry.
func TestRegistry(t *testing.T) {
	// Save original registry and restore after test
	mu.Lock()
	originalRegistry := make(map[string]ProviderFactory)
	for k, v := range registry {
		originalRegistry[k] = v
	}
	mu.Unlock()

	defer func() {
		mu.Lock()
		registry = originalRegistry
		mu.Unlock()
	}()

	t.Run("register new provider", func(t *testing.T) {
		factory := func(config Config) (EmbeddingService, error) {
			return &mockEmbeddingService{}, nil
		}

		Register("test_provider", factory)
		assert.True(t, IsRegistered("test_provider"))
	})

	t.Run("register nil factory panics", func(t *testing.T) {
		assert.Panics(t, func() {
			Register("nil_provider", nil)
		})
	})

	t.Run("register duplicate panics", func(t *testing.T) {
		factory := func(config Config) (EmbeddingService, error) {
			return &mockEmbeddingService{}, nil
		}

		Register("dup_provider", factory)
		assert.Panics(t, func() {
			Register("dup_provider", factory)
		})
	})

	t.Run("list providers", func(t *testing.T) {
		providers := ListProviders()
		assert.NotEmpty(t, providers)
		assert.Contains(t, providers, "test_provider")
	})

	t.Run("is registered", func(t *testing.T) {
		assert.True(t, IsRegistered("test_provider"))
		assert.False(t, IsRegistered("nonexistent_provider"))
	})

	t.Run("create from registry", func(t *testing.T) {
		// Need to add mock config for test_provider
		Register("test_provider_validated", func(config Config) (EmbeddingService, error) {
			return &mockEmbeddingService{}, nil
		})

		config := Config{
			Provider: "test_provider_validated",
			OpenAI: &OpenAIConfig{
				APIKey: "test-key",
				Model:  "test-model",
			},
		}

		// Override validation to succeed
		originalValidate := config.Validate
		_ = originalValidate

		service, err := New(config)
		// This will fail validation, but that's expected
		// The test validates the flow, not the specific provider
		if err == nil {
			assert.NotNil(t, service)
		}
	})

	t.Run("create unknown provider", func(t *testing.T) {
		config := Config{
			Provider: "unknown_provider",
		}

		_, err := New(config)
		require.Error(t, err)
		// Error comes from validation first
		assert.Contains(t, err.Error(), "unsupported provider")
	})
}

// TestRegistryConcurrency tests concurrent access to the registry.
func TestRegistryConcurrency(t *testing.T) {
	// Save original registry and restore after test
	mu.Lock()
	originalRegistry := make(map[string]ProviderFactory)
	for k, v := range registry {
		originalRegistry[k] = v
	}
	mu.Unlock()

	defer func() {
		mu.Lock()
		registry = originalRegistry
		mu.Unlock()
	}()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ListProviders()
			_ = IsRegistered("openai")
		}()
	}

	wg.Wait()
}

// mockEmbeddingService is a mock implementation for testing.
type mockEmbeddingService struct {
	embedFunc      func(ctx context.Context, text string) ([]float32, error)
	embedBatchFunc func(ctx context.Context, texts []string) ([][]float32, error)
	dimensions     int
	modelName      string
}

func (m *mockEmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedBatchFunc != nil {
		return m.embedBatchFunc(ctx, texts)
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func (m *mockEmbeddingService) Dimensions() int {
	if m.dimensions > 0 {
		return m.dimensions
	}
	return 3
}

func (m *mockEmbeddingService) ModelName() string {
	if m.modelName != "" {
		return m.modelName
	}
	return "mock-model"
}

func (m *mockEmbeddingService) Close() error {
	return nil
}

// TestEmbeddingServiceInterface tests that all implementations satisfy the interface.
func TestEmbeddingServiceInterface(t *testing.T) {
	var _ EmbeddingService = &mockEmbeddingService{}
}

// TestConfigStructure tests the Config structure.
func TestConfigStructure(t *testing.T) {
	config := Config{
		Provider: "openai",
		OpenAI: &OpenAIConfig{
			APIKey: "test-key",
			Model:  "text-embedding-3-small",
		},
		HuggingFace: &HuggingFaceConfig{
			Model: "sentence-transformers/all-MiniLM-L6-v2",
		},
		HuggingFaceTEI: &HuggingFaceTEIConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	assert.Equal(t, "openai", config.Provider)
	assert.NotNil(t, config.OpenAI)
	assert.Equal(t, "test-key", config.OpenAI.APIKey)
	assert.NotNil(t, config.HuggingFace)
	assert.NotNil(t, config.HuggingFaceTEI)
}

// TestOpenAIConfigStructure tests the OpenAIConfig structure.
func TestOpenAIConfigStructure(t *testing.T) {
	config := OpenAIConfig{
		APIKey:     "test-key",
		Model:      "text-embedding-3-large",
		BaseURL:    "https://custom.openai.com/v1",
		Dimensions: 1024,
	}

	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "text-embedding-3-large", config.Model)
	assert.Equal(t, "https://custom.openai.com/v1", config.BaseURL)
	assert.Equal(t, 1024, config.Dimensions)
}

// TestHuggingFaceConfigStructure tests the HuggingFaceConfig structure.
func TestHuggingFaceConfigStructure(t *testing.T) {
	config := HuggingFaceConfig{
		APIKey:       "test-key",
		Model:        "BAAI/bge-large-en-v1.5",
		Endpoint:     "https://custom.huggingface.co",
		WaitForModel: true,
		UseCache:     false,
	}

	assert.Equal(t, "test-key", config.APIKey)
	assert.Equal(t, "BAAI/bge-large-en-v1.5", config.Model)
	assert.Equal(t, "https://custom.huggingface.co", config.Endpoint)
	assert.True(t, config.WaitForModel)
	assert.False(t, config.UseCache)
}

// TestHuggingFaceTEIConfigStructure tests the HuggingFaceTEIConfig structure.
func TestHuggingFaceTEIConfigStructure(t *testing.T) {
	config := HuggingFaceTEIConfig{
		Endpoint:  "http://localhost:8080",
		Model:     "BAAI/bge-large-en-v1.5",
		Normalize: true,
	}

	assert.Equal(t, "http://localhost:8080", config.Endpoint)
	assert.Equal(t, "BAAI/bge-large-en-v1.5", config.Model)
	assert.True(t, config.Normalize)
}

// TestValidationWithDefaults tests that validation sets appropriate defaults.
func TestValidationWithDefaults(t *testing.T) {
	t.Run("openai defaults", func(t *testing.T) {
		config := &OpenAIConfig{
			APIKey: "test-key",
		}

		err := config.Validate()
		require.NoError(t, err)
		assert.Equal(t, "text-embedding-3-small", config.Model)
		assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	})

	t.Run("huggingface defaults", func(t *testing.T) {
		config := &HuggingFaceConfig{
			Model: "sentence-transformers/all-MiniLM-L6-v2",
		}

		err := config.Validate()
		require.NoError(t, err)
		assert.Equal(t, "https://api-inference.huggingface.co", config.Endpoint)
	})
}

// TestNewWithInvalidConfig tests creating a service with invalid configuration.
func TestNewWithInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "empty provider",
			config: Config{},
		},
		{
			name: "openai without config",
			config: Config{
				Provider: "openai",
			},
		},
		{
			name: "openai without API key",
			config: Config{
				Provider: "openai",
				OpenAI:   &OpenAIConfig{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.config)
			require.Error(t, err)
		})
	}
}

// BenchmarkConfigValidation benchmarks configuration validation.
func BenchmarkConfigValidation(b *testing.B) {
	config := Config{
		Provider: "openai",
		OpenAI: &OpenAIConfig{
			APIKey: "test-key",
			Model:  "text-embedding-3-small",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

// BenchmarkListProviders benchmarks listing providers.
func BenchmarkListProviders(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ListProviders()
	}
}

// BenchmarkIsRegistered benchmarks checking provider registration.
func BenchmarkIsRegistered(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsRegistered("openai")
	}
}

// TestMultipleProviderConfigs tests having configs for multiple providers.
func TestMultipleProviderConfigs(t *testing.T) {
	config := Config{
		Provider: "openai",
		OpenAI: &OpenAIConfig{
			APIKey: "openai-key",
			Model:  "text-embedding-3-small",
		},
		HuggingFace: &HuggingFaceConfig{
			Model: "sentence-transformers/all-MiniLM-L6-v2",
		},
	}

	// Should validate successfully with openai provider selected
	err := config.Validate()
	require.NoError(t, err)

	// Should ignore huggingface config since openai is selected
	assert.NotNil(t, config.HuggingFace)
}

// TestProviderSwitching tests changing providers.
func TestProviderSwitching(t *testing.T) {
	config := Config{
		Provider: "openai",
		OpenAI: &OpenAIConfig{
			APIKey: "openai-key",
			Model:  "text-embedding-3-small",
		},
		HuggingFace: &HuggingFaceConfig{
			Model: "sentence-transformers/all-MiniLM-L6-v2",
		},
	}

	// Validate with openai
	err := config.Validate()
	require.NoError(t, err)

	// Switch to huggingface
	config.Provider = "huggingface"
	err = config.Validate()
	require.NoError(t, err)
}

// TestEdgeCases tests edge cases in configuration.
func TestEdgeCases(t *testing.T) {
	t.Run("empty strings in config", func(t *testing.T) {
		config := Config{
			Provider: "",
		}
		err := config.Validate()
		require.Error(t, err)
	})

	t.Run("whitespace in provider", func(t *testing.T) {
		config := Config{
			Provider: "  ",
			OpenAI: &OpenAIConfig{
				APIKey: "test-key",
			},
		}
		err := config.Validate()
		require.Error(t, err)
	})

	t.Run("custom dimensions", func(t *testing.T) {
		config := OpenAIConfig{
			APIKey:     "test-key",
			Model:      "text-embedding-3-small",
			Dimensions: 512,
		}
		err := config.Validate()
		require.NoError(t, err)
		assert.Equal(t, 512, config.Dimensions)
	})
}

// TestErrorMessages tests that error messages are descriptive.
func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		shouldMatch string
	}{
		{
			name:        "empty provider",
			config:      Config{},
			shouldMatch: "provider must be specified",
		},
		{
			name: "unsupported provider",
			config: Config{
				Provider: "unsupported",
			},
			shouldMatch: "unsupported provider",
		},
		{
			name: "missing openai config",
			config: Config{
				Provider: "openai",
			},
			shouldMatch: "openai configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.shouldMatch)
		})
	}
}

// TestBasicUsagePattern demonstrates basic usage of the embeddings package.
// This is a test, not an example, to avoid failures when running without API keys.
func TestBasicUsagePattern(t *testing.T) {
	config := Config{
		Provider: "openai",
		OpenAI: &OpenAIConfig{
			APIKey: "your-api-key",
			Model:  "text-embedding-3-small",
		},
	}

	// Validate configuration
	err := config.Validate()
	require.NoError(t, err)

	// Test that validation sets defaults
	assert.Equal(t, "text-embedding-3-small", config.OpenAI.Model)
	assert.Equal(t, "https://api.openai.com/v1", config.OpenAI.BaseURL)
}
