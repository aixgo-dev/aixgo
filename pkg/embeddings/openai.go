package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIEmbeddings implements EmbeddingService using OpenAI's API.
type OpenAIEmbeddings struct {
	apiKey     string
	model      string
	baseURL    string
	dimensions int
	client     *http.Client
}

// openAIRequest represents the request format for OpenAI API.
type openAIRequest struct {
	Input      any `json:"input"`
	Model      string      `json:"model"`
	Dimensions *int        `json:"dimensions,omitempty"`
}

// openAIResponse represents the response format from OpenAI API.
type openAIResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func init() {
	// Register the OpenAI provider
	Register("openai", NewOpenAI)
}

// NewOpenAI creates a new OpenAIEmbeddings instance.
func NewOpenAI(config Config) (EmbeddingService, error) {
	if config.OpenAI == nil {
		return nil, fmt.Errorf("openai configuration is required")
	}

	// Apply defaults before validation
	if config.OpenAI.Model == "" {
		config.OpenAI.Model = "text-embedding-3-small"
	}
	if config.OpenAI.BaseURL == "" {
		config.OpenAI.BaseURL = "https://api.openai.com/v1"
	}

	// Validate required fields
	if config.OpenAI.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	// Determine dimensions based on model
	dims := getOpenAIModelDimensions(config.OpenAI.Model)
	if config.OpenAI.Dimensions > 0 {
		// Custom dimensions specified (only supported for text-embedding-3 models)
		if !isTextEmbedding3Model(config.OpenAI.Model) {
			return nil, fmt.Errorf("custom dimensions only supported for text-embedding-3 models, got model: %s", config.OpenAI.Model)
		}
		dims = config.OpenAI.Dimensions
	}

	return &OpenAIEmbeddings{
		apiKey:     config.OpenAI.APIKey,
		model:      config.OpenAI.Model,
		baseURL:    config.OpenAI.BaseURL,
		dimensions: dims,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Embed generates embeddings for a single text.
func (o *OpenAIEmbeddings) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := openAIRequest{
		Input: text,
		Model: o.model,
	}

	// Only set dimensions for text-embedding-3 models
	if isTextEmbedding3Model(o.model) && o.dimensions > 0 {
		reqBody.Dimensions = &o.dimensions
	}

	embeddings, err := o.makeRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (o *OpenAIEmbeddings) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	reqBody := openAIRequest{
		Input: texts,
		Model: o.model,
	}

	// Only set dimensions for text-embedding-3 models
	if isTextEmbedding3Model(o.model) && o.dimensions > 0 {
		reqBody.Dimensions = &o.dimensions
	}

	return o.makeRequest(ctx, reqBody)
}

// Dimensions returns the dimension size of the embeddings.
func (o *OpenAIEmbeddings) Dimensions() int {
	return o.dimensions
}

// ModelName returns the name of the embedding model.
func (o *OpenAIEmbeddings) ModelName() string {
	return o.model
}

// Close closes any resources held by the service.
func (o *OpenAIEmbeddings) Close() error {
	o.client.CloseIdleConnections()
	return nil
}

// makeRequest makes an HTTP request to the OpenAI API.
func (o *OpenAIEmbeddings) makeRequest(ctx context.Context, reqBody openAIRequest) ([][]float32, error) {
	// Serialize request body
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("%s/embeddings", o.baseURL)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	// Make request
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return nil, fmt.Errorf("OpenAI API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract embeddings in order with proper validation
	if len(apiResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned from API")
	}

	embeddings := make([][]float32, len(apiResp.Data))
	seen := make(map[int]bool, len(apiResp.Data))

	for i, item := range apiResp.Data {
		// Check for nil embedding
		if item.Embedding == nil {
			return nil, fmt.Errorf("embedding at response index %d is nil", i)
		}

		// Validate index bounds
		if item.Index < 0 {
			return nil, fmt.Errorf("invalid negative embedding index: %d", item.Index)
		}
		if item.Index >= len(embeddings) {
			return nil, fmt.Errorf("embedding index out of bounds: %d (expected 0-%d)", item.Index, len(embeddings)-1)
		}

		// Check for duplicate indices
		if seen[item.Index] {
			return nil, fmt.Errorf("duplicate embedding index: %d", item.Index)
		}
		seen[item.Index] = true

		embeddings[item.Index] = item.Embedding
	}

	// Verify all indices were filled (no gaps)
	for i := range embeddings {
		if !seen[i] {
			return nil, fmt.Errorf("missing embedding at index %d", i)
		}
	}

	return embeddings, nil
}

// getOpenAIModelDimensions returns the default dimensions for OpenAI models.
func getOpenAIModelDimensions(model string) int {
	switch model {
	case "text-embedding-ada-002":
		return 1536
	case "text-embedding-3-small":
		return 1536
	case "text-embedding-3-large":
		return 3072
	default:
		// Default to 1536 for unknown models
		return 1536
	}
}

// isTextEmbedding3Model checks if the model is a text-embedding-3 model.
// Only these models support custom dimensions.
func isTextEmbedding3Model(model string) bool {
	return model == "text-embedding-3-small" || model == "text-embedding-3-large"
}
