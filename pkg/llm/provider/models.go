package provider

import (
	"context"
	"os"
	"sort"
	"sync"
	"time"
)

// ModelAggregator fetches and caches models from all available providers.
type ModelAggregator struct {
	cache      []ModelInfo
	cachedAt   time.Time
	cacheTTL   time.Duration
	mu         sync.RWMutex
}

// NewModelAggregator creates a new model aggregator with the specified cache TTL.
func NewModelAggregator(cacheTTL time.Duration) *ModelAggregator {
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}
	return &ModelAggregator{
		cacheTTL: cacheTTL,
	}
}

// DefaultModelAggregator is a global aggregator with 5-minute cache.
var DefaultModelAggregator = NewModelAggregator(5 * time.Minute)

// ListAllModels fetches models from all configured providers and returns a unified list.
// Results are cached for the configured TTL to minimize API calls.
func (a *ModelAggregator) ListAllModels(ctx context.Context) ([]ModelInfo, error) {
	// Check cache first
	a.mu.RLock()
	if len(a.cache) > 0 && time.Since(a.cachedAt) < a.cacheTTL {
		cached := make([]ModelInfo, len(a.cache))
		copy(cached, a.cache)
		a.mu.RUnlock()
		return cached, nil
	}
	a.mu.RUnlock()

	// Fetch from all providers
	var allModels []ModelInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	providers := a.getAvailableProviders()

	for name, config := range providers {
		wg.Add(1)
		go func(providerName string, cfg map[string]any) {
			defer wg.Done()

			provider, err := CreateProvider(providerName, cfg)
			if err != nil {
				return // Skip unavailable providers
			}

			models, err := provider.ListModels(ctx)
			if err != nil {
				return // Skip providers that fail to list models
			}

			mu.Lock()
			allModels = append(allModels, models...)
			mu.Unlock()
		}(name, config)
	}

	wg.Wait()

	// Sort models by provider, then by ID
	sort.Slice(allModels, func(i, j int) bool {
		if allModels[i].Provider != allModels[j].Provider {
			return allModels[i].Provider < allModels[j].Provider
		}
		return allModels[i].ID < allModels[j].ID
	})

	// Update cache
	a.mu.Lock()
	a.cache = allModels
	a.cachedAt = time.Now()
	a.mu.Unlock()

	return allModels, nil
}

// getAvailableProviders returns provider configs for all providers with API keys set.
func (a *ModelAggregator) getAvailableProviders() map[string]map[string]any {
	providers := make(map[string]map[string]any)

	// Check Anthropic
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		providers["anthropic"] = map[string]any{}
	}

	// Check OpenAI
	if os.Getenv("OPENAI_API_KEY") != "" {
		providers["openai"] = map[string]any{}
	}

	// Check Google/Gemini
	if os.Getenv("GOOGLE_API_KEY") != "" {
		providers["gemini"] = map[string]any{}
	}

	// Check xAI
	if os.Getenv("XAI_API_KEY") != "" {
		providers["xai"] = map[string]any{}
	}

	return providers
}

// ClearCache clears the cached models, forcing a refresh on next ListAllModels call.
func (a *ModelAggregator) ClearCache() {
	a.mu.Lock()
	a.cache = nil
	a.cachedAt = time.Time{}
	a.mu.Unlock()
}

// ListAllModels is a convenience function using the default aggregator.
func ListAllModels(ctx context.Context) ([]ModelInfo, error) {
	return DefaultModelAggregator.ListAllModels(ctx)
}

// GetAvailableProviderNames returns the names of providers with valid API keys.
func GetAvailableProviderNames() []string {
	var names []string

	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		names = append(names, "anthropic")
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		names = append(names, "openai")
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		names = append(names, "gemini")
	}
	if os.Getenv("XAI_API_KEY") != "" {
		names = append(names, "xai")
	}

	return names
}

// FilterModelsByProvider filters a list of models to only include those from the specified provider.
func FilterModelsByProvider(models []ModelInfo, providerName string) []ModelInfo {
	var filtered []ModelInfo
	for _, m := range models {
		if m.Provider == providerName {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// FilterChatModels filters models to only include those suitable for chat.
// This filters based on common naming patterns.
func FilterChatModels(models []ModelInfo) []ModelInfo {
	var filtered []ModelInfo
	for _, m := range models {
		// Include most models by default, exclude known non-chat models
		// (e.g., embedding models, moderation models)
		if isChatModel(m.ID) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// isChatModel checks if a model ID is likely a chat-capable model.
func isChatModel(id string) bool {
	// Exclude known non-chat model patterns
	nonChatPrefixes := []string{
		"text-embedding-",
		"text-moderation-",
		"whisper-",
		"tts-",
		"dall-e-",
	}

	for _, prefix := range nonChatPrefixes {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			return false
		}
	}

	return true
}
