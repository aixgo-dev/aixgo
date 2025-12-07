package cost

import (
	"fmt"
	"strings"
	"sync"
)

// ModelPricing contains pricing information for a specific model
type ModelPricing struct {
	Model           string
	InputPer1M      float64 // Cost per 1M input tokens in USD
	OutputPer1M     float64 // Cost per 1M output tokens in USD
	CachedPer1M     float64 // Cost per 1M cached input tokens (if supported)
	SupportsCaching bool
}

// Usage represents token usage for a single LLM call
type Usage struct {
	Model        string
	InputTokens  int
	OutputTokens int
	CachedTokens int // Cached input tokens (if model supports caching)
	TotalTokens  int
}

// Cost represents the calculated cost for LLM usage
type Cost struct {
	InputCost  float64
	OutputCost float64
	CachedCost float64
	TotalCost  float64
	Currency   string
}

// Calculator provides cost calculation for LLM usage
type Calculator struct {
	pricing map[string]*ModelPricing
	mu      sync.RWMutex
}

// NewCalculator creates a new cost calculator with default pricing
func NewCalculator() *Calculator {
	c := &Calculator{
		pricing: make(map[string]*ModelPricing),
	}

	// Load default pricing
	c.loadDefaultPricing()

	return c
}

// loadDefaultPricing initializes pricing for common models
// Prices as of January 2025 - update periodically
func (c *Calculator) loadDefaultPricing() {
	models := []*ModelPricing{
		// OpenAI GPT-4 models
		{Model: "gpt-4", InputPer1M: 30.0, OutputPer1M: 60.0},
		{Model: "gpt-4-turbo", InputPer1M: 10.0, OutputPer1M: 30.0},
		{Model: "gpt-4-turbo-preview", InputPer1M: 10.0, OutputPer1M: 30.0},
		{Model: "gpt-4o", InputPer1M: 2.5, OutputPer1M: 10.0, CachedPer1M: 1.25, SupportsCaching: true},
		{Model: "gpt-4o-mini", InputPer1M: 0.15, OutputPer1M: 0.60, CachedPer1M: 0.075, SupportsCaching: true},

		// OpenAI GPT-3.5 models
		{Model: "gpt-3.5-turbo", InputPer1M: 0.5, OutputPer1M: 1.5},
		{Model: "gpt-3.5-turbo-16k", InputPer1M: 3.0, OutputPer1M: 4.0},

		// OpenAI O1 models
		{Model: "o1-preview", InputPer1M: 15.0, OutputPer1M: 60.0},
		{Model: "o1-mini", InputPer1M: 3.0, OutputPer1M: 12.0},

		// Anthropic Claude models
		{Model: "claude-3-opus-20240229", InputPer1M: 15.0, OutputPer1M: 75.0, CachedPer1M: 1.5, SupportsCaching: true},
		{Model: "claude-3-5-sonnet-20241022", InputPer1M: 3.0, OutputPer1M: 15.0, CachedPer1M: 0.3, SupportsCaching: true},
		{Model: "claude-3-5-sonnet-20240620", InputPer1M: 3.0, OutputPer1M: 15.0, CachedPer1M: 0.3, SupportsCaching: true},
		{Model: "claude-3-5-haiku-20241022", InputPer1M: 1.0, OutputPer1M: 5.0, CachedPer1M: 0.1, SupportsCaching: true},
		{Model: "claude-3-haiku-20240307", InputPer1M: 0.25, OutputPer1M: 1.25, CachedPer1M: 0.03, SupportsCaching: true},

		// Google Gemini models
		{Model: "gemini-1.5-pro", InputPer1M: 1.25, OutputPer1M: 5.0, CachedPer1M: 0.3125, SupportsCaching: true},
		{Model: "gemini-1.5-flash", InputPer1M: 0.075, OutputPer1M: 0.3, CachedPer1M: 0.01875, SupportsCaching: true},
		{Model: "gemini-2.0-flash-exp", InputPer1M: 0.0, OutputPer1M: 0.0}, // Free during preview

		// Ollama (local models - no cost)
		{Model: "ollama/llama3.1", InputPer1M: 0.0, OutputPer1M: 0.0},
		{Model: "ollama/llama3.2", InputPer1M: 0.0, OutputPer1M: 0.0},
		{Model: "ollama/llama3.3", InputPer1M: 0.0, OutputPer1M: 0.0},
		{Model: "ollama/qwen2.5", InputPer1M: 0.0, OutputPer1M: 0.0},
		{Model: "ollama/mistral", InputPer1M: 0.0, OutputPer1M: 0.0},
		{Model: "ollama/phi", InputPer1M: 0.0, OutputPer1M: 0.0},

		// vLLM (local models - no cost)
		{Model: "vllm/meta-llama/Llama-3.1-8B", InputPer1M: 0.0, OutputPer1M: 0.0},
		{Model: "vllm/meta-llama/Llama-3.2-3B", InputPer1M: 0.0, OutputPer1M: 0.0},
	}

	for _, pricing := range models {
		c.pricing[pricing.Model] = pricing
	}
}

// AddPricing adds or updates pricing for a model
func (c *Calculator) AddPricing(pricing *ModelPricing) {
	if pricing == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pricing[pricing.Model] = pricing
}

// GetPricing retrieves pricing for a model
func (c *Calculator) GetPricing(model string) (*ModelPricing, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var pricing *ModelPricing

	// Exact match first
	if p, ok := c.pricing[model]; ok {
		pricing = p
	} else {
		// Prefix match (sort keys by length for determinism)
		var keys []string
		for k := range c.pricing {
			keys = append(keys, k)
		}
		// Sort by length (longest first) for deterministic matching
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if len(keys[i]) < len(keys[j]) {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}

		for _, key := range keys {
			if strings.HasPrefix(model, key) {
				pricing = c.pricing[key]
				break
			}
		}
	}

	if pricing == nil {
		return nil, false
	}

	// Return a copy to prevent concurrent modification
	pricingCopy := *pricing
	return &pricingCopy, true
}

// Calculate computes the cost for the given usage
func (c *Calculator) Calculate(usage *Usage) (*Cost, error) {
	pricing, ok := c.GetPricing(usage.Model)
	if !ok {
		return nil, fmt.Errorf("no pricing found for model: %s", usage.Model)
	}

	cost := &Cost{
		Currency: "USD",
	}

	// Calculate input cost
	if usage.InputTokens > 0 {
		cost.InputCost = (float64(usage.InputTokens) / 1_000_000) * pricing.InputPer1M
	}

	// Calculate output cost
	if usage.OutputTokens > 0 {
		cost.OutputCost = (float64(usage.OutputTokens) / 1_000_000) * pricing.OutputPer1M
	}

	// Calculate cached token cost (if supported)
	if usage.CachedTokens > 0 && pricing.SupportsCaching {
		cost.CachedCost = (float64(usage.CachedTokens) / 1_000_000) * pricing.CachedPer1M
	}

	cost.TotalCost = cost.InputCost + cost.OutputCost + cost.CachedCost

	return cost, nil
}

// CalculateMultiple computes total cost for multiple usage records
func (c *Calculator) CalculateMultiple(usages []*Usage) (*Cost, error) {
	total := &Cost{
		Currency: "USD",
	}

	for i, usage := range usages {
		if usage == nil {
			return nil, fmt.Errorf("usage at index %d is nil", i)
		}
		cost, err := c.Calculate(usage)
		if err != nil {
			return nil, err
		}

		total.InputCost += cost.InputCost
		total.OutputCost += cost.OutputCost
		total.CachedCost += cost.CachedCost
		total.TotalCost += cost.TotalCost
	}

	return total, nil
}

// EstimateCost estimates cost for a given number of tokens
func (c *Calculator) EstimateCost(model string, inputTokens, outputTokens int) (*Cost, error) {
	return c.Calculate(&Usage{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	})
}

// ListModels returns all models with pricing information
func (c *Calculator) ListModels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	models := make([]string, 0, len(c.pricing))
	for model := range c.pricing {
		models = append(models, model)
	}

	return models
}

// DefaultCalculator is the global cost calculator instance
var DefaultCalculator = NewCalculator()
