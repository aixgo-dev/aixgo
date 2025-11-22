package agents

import (
	"fmt"
	"os"

	"github.com/aixgo-dev/aixgo/internal/llm/provider"
)

// initializeProvider creates the appropriate LLM provider based on model name
// This is a simplified version that uses the factory pattern from the provider package
func initializeProvider(model string) (provider.Provider, error) {
	// Detect provider type from model name
	providerType := provider.DetectProvider(model)

	// Build configuration based on provider type
	config := make(map[string]any)

	switch providerType {
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		config["api_key"] = apiKey
		if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
			config["base_url"] = baseURL
		}

	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		config["api_key"] = apiKey
		if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
			config["base_url"] = baseURL
		}

	case "xai":
		apiKey := os.Getenv("XAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("XAI_API_KEY not set")
		}
		config["api_key"] = apiKey
		if baseURL := os.Getenv("XAI_BASE_URL"); baseURL != "" {
			config["base_url"] = baseURL
		}

	case "vertexai", "gemini":
		projectID := os.Getenv("VERTEX_PROJECT_ID")
		if projectID == "" {
			return nil, fmt.Errorf("VERTEX_PROJECT_ID not set")
		}
		config["project_id"] = projectID
		config["location"] = os.Getenv("VERTEX_LOCATION")
		if config["location"] == "" {
			config["location"] = "us-central1"
		}

	case "huggingface":
		apiKey := os.Getenv("HUGGINGFACE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("HUGGINGFACE_API_KEY not set")
		}
		config["api_key"] = apiKey
		config["model"] = model
		if endpoint := os.Getenv("HUGGINGFACE_ENDPOINT"); endpoint != "" {
			config["endpoint"] = endpoint
		}

	default:
		// Try OpenAI as fallback
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("no API key found for provider: %s", providerType)
		}
		config["api_key"] = apiKey
	}

	// Try to create provider using factory if available
	if provider.Has(providerType) {
		return provider.Get(providerType)
	}

	// Otherwise, create a mock provider for testing
	// In production, this would use provider.CreateProvider with proper factories
	return nil, fmt.Errorf("provider %s not initialized in registry", providerType)
}