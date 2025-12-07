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

// HuggingFaceEmbeddings implements EmbeddingService using HuggingFace Inference API.
type HuggingFaceEmbeddings struct {
	apiKey       string
	model        string
	endpoint     string
	waitForModel bool
	useCache     bool
	dimensions   int
	client       *http.Client
}

// hfRequest represents the request format for HuggingFace API.
type hfRequest struct {
	Inputs  interface{}       `json:"inputs"`
	Options *hfRequestOptions `json:"options,omitempty"`
}

type hfRequestOptions struct {
	WaitForModel bool `json:"wait_for_model"`
	UseCache     bool `json:"use_cache"`
}

func init() {
	// Register the HuggingFace provider
	Register("huggingface", NewHuggingFace)
}

// NewHuggingFace creates a new HuggingFaceEmbeddings instance.
func NewHuggingFace(config Config) (EmbeddingService, error) {
	if config.HuggingFace == nil {
		return nil, fmt.Errorf("huggingface configuration is required")
	}

	// Validate required fields
	if config.HuggingFace.Model == "" {
		return nil, fmt.Errorf("HuggingFace model is required")
	}

	// Apply defaults
	endpoint := config.HuggingFace.Endpoint
	if endpoint == "" {
		endpoint = "https://api-inference.huggingface.co"
	}

	hf := &HuggingFaceEmbeddings{
		apiKey:       config.HuggingFace.APIKey,
		model:        config.HuggingFace.Model,
		endpoint:     endpoint,
		waitForModel: config.HuggingFace.WaitForModel,
		useCache:     config.HuggingFace.UseCache,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Determine dimensions based on model
	hf.dimensions = getHuggingFaceModelDimensions(config.HuggingFace.Model)

	return hf, nil
}

// Embed generates embeddings for a single text.
func (h *HuggingFaceEmbeddings) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := hfRequest{
		Inputs: text,
		Options: &hfRequestOptions{
			WaitForModel: h.waitForModel,
			UseCache:     h.useCache,
		},
	}

	embeddings, err := h.makeRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (h *HuggingFaceEmbeddings) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	reqBody := hfRequest{
		Inputs: texts,
		Options: &hfRequestOptions{
			WaitForModel: h.waitForModel,
			UseCache:     h.useCache,
		},
	}

	return h.makeRequest(ctx, reqBody)
}

// Dimensions returns the dimension size of the embeddings.
func (h *HuggingFaceEmbeddings) Dimensions() int {
	return h.dimensions
}

// ModelName returns the name of the embedding model.
func (h *HuggingFaceEmbeddings) ModelName() string {
	return h.model
}

// Close closes any resources held by the service.
func (h *HuggingFaceEmbeddings) Close() error {
	h.client.CloseIdleConnections()
	return nil
}

// makeRequest makes an HTTP request to the HuggingFace API.
func (h *HuggingFaceEmbeddings) makeRequest(ctx context.Context, reqBody hfRequest) ([][]float32, error) {
	// Serialize request body
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL - use the correct API endpoint for HuggingFace Inference API
	url := fmt.Sprintf("%s/models/%s", h.endpoint, h.model)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	// Make request
	resp, err := h.client.Do(req)
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
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embeddings [][]float32
	if err := json.Unmarshal(body, &embeddings); err != nil {
		// Try parsing as single embedding
		var singleEmbedding []float32
		if err2 := json.Unmarshal(body, &singleEmbedding); err2 != nil {
			return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
		}
		embeddings = [][]float32{singleEmbedding}
	}

	return embeddings, nil
}

// getHuggingFaceModelDimensions returns the known dimensions for popular HuggingFace models.
func getHuggingFaceModelDimensions(model string) int {
	// Map of known model dimensions
	knownDimensions := map[string]int{
		"sentence-transformers/all-MiniLM-L6-v2":        384,
		"sentence-transformers/all-MiniLM-L12-v2":       384,
		"sentence-transformers/all-mpnet-base-v2":       768,
		"sentence-transformers/paraphrase-MiniLM-L6-v2": 384,
		"BAAI/bge-small-en-v1.5":                        384,
		"BAAI/bge-base-en-v1.5":                         768,
		"BAAI/bge-large-en-v1.5":                        1024,
		"thenlper/gte-small":                            384,
		"thenlper/gte-base":                             768,
		"thenlper/gte-large":                            1024,
		"intfloat/e5-small-v2":                          384,
		"intfloat/e5-base-v2":                           768,
		"intfloat/e5-large-v2":                          1024,
		"jinaai/jina-embeddings-v2-small-en":            512,
		"jinaai/jina-embeddings-v2-base-en":             768,
	}

	if dim, ok := knownDimensions[model]; ok {
		return dim
	}

	// Default to 768 if unknown
	return 768
}
