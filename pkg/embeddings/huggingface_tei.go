package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/security"
)

// HuggingFaceTEIEmbeddings implements EmbeddingService using HuggingFace Text Embeddings Inference (TEI).
// TEI is a self-hosted, high-performance embedding server optimized for production use.
// See: https://github.com/huggingface/text-embeddings-inference
type HuggingFaceTEIEmbeddings struct {
	endpoint   string
	model      string
	normalize  bool
	dimensions int32 // Changed to int32 for atomic operations
	client     *http.Client
}

// teiRequest represents the request format for TEI API.
type teiRequest struct {
	Inputs    interface{} `json:"inputs"`
	Normalize *bool       `json:"normalize,omitempty"`
}

// teiResponse represents the response format from TEI API.
// TEI returns embeddings directly as an array of float arrays.
type teiResponse [][]float32

func init() {
	// Register the HuggingFace TEI provider
	Register("huggingface_tei", NewHuggingFaceTEI)
}

// NewHuggingFaceTEI creates a new HuggingFaceTEIEmbeddings instance.
func NewHuggingFaceTEI(config Config) (EmbeddingService, error) {
	if config.HuggingFaceTEI == nil {
		return nil, fmt.Errorf("huggingface_tei configuration is required")
	}

	tei := &HuggingFaceTEIEmbeddings{
		endpoint:  config.HuggingFaceTEI.Endpoint,
		model:     config.HuggingFaceTEI.Model,
		normalize: config.HuggingFaceTEI.Normalize,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Probe the TEI server to get model dimensions
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dims, err := tei.probeDimensions(ctx)
	if err != nil {
		// If probe fails, log warning but don't fail initialization
		// Dimensions will be determined on first embedding
		log.Printf("Warning: HuggingFace TEI dimension probe failed: %v (dimensions will be determined on first embedding)", err)
		atomic.StoreInt32(&tei.dimensions, 0)
	} else {
		// G115: Safe conversion with bounds checking to prevent integer overflow
		dims32, err := security.SafeIntToInt32(dims)
		if err != nil {
			return nil, fmt.Errorf("dimension size out of range: %w", err)
		}
		atomic.StoreInt32(&tei.dimensions, dims32)
	}

	return tei, nil
}

// Embed generates embeddings for a single text.
func (t *HuggingFaceTEIEmbeddings) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	reqBody := teiRequest{
		Inputs: text,
	}

	if t.normalize {
		reqBody.Normalize = &t.normalize
	}

	embeddings, err := t.makeRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Update dimensions if not set (using atomic operation to prevent race condition)
	if atomic.LoadInt32(&t.dimensions) == 0 && len(embeddings) > 0 && len(embeddings[0]) > 0 {
		// G115: Safe conversion - embedding dimensions are bounded by model architecture (typically < 10000)
		dim := len(embeddings[0])
		if dim > 32767 { // int32 max safe value for embeddings
			return nil, fmt.Errorf("dimension size %d exceeds maximum supported value", dim)
		}
		atomic.StoreInt32(&t.dimensions, int32(dim)) //nolint:gosec
	}

	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (t *HuggingFaceTEIEmbeddings) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	reqBody := teiRequest{
		Inputs: texts,
	}

	if t.normalize {
		reqBody.Normalize = &t.normalize
	}

	embeddings, err := t.makeRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	// Update dimensions if not set (using atomic operation to prevent race condition)
	if atomic.LoadInt32(&t.dimensions) == 0 && len(embeddings) > 0 && len(embeddings[0]) > 0 {
		// G115: Safe conversion with bounds checking to prevent integer overflow
		dim32, err := security.SafeIntToInt32(len(embeddings[0]))
		if err != nil {
			return nil, fmt.Errorf("dimension size out of range: %w", err)
		}
		atomic.StoreInt32(&t.dimensions, dim32)
	}

	return embeddings, nil
}

// Dimensions returns the dimension size of the embeddings.
func (t *HuggingFaceTEIEmbeddings) Dimensions() int {
	return int(atomic.LoadInt32(&t.dimensions))
}

// ModelName returns the name of the embedding model.
func (t *HuggingFaceTEIEmbeddings) ModelName() string {
	if t.model != "" {
		return t.model
	}
	return "huggingface-tei"
}

// Close closes any resources held by the service.
func (t *HuggingFaceTEIEmbeddings) Close() error {
	t.client.CloseIdleConnections()
	return nil
}

// makeRequest makes an HTTP request to the TEI server.
func (t *HuggingFaceTEIEmbeddings) makeRequest(ctx context.Context, reqBody teiRequest) ([][]float32, error) {
	// Serialize request body
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build URL (TEI endpoint is typically just the base URL)
	url := fmt.Sprintf("%s/embed", t.endpoint)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := t.client.Do(req)
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
		return nil, fmt.Errorf("TEI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embeddings teiResponse
	if err := json.Unmarshal(body, &embeddings); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return embeddings, nil
}

// probeDimensions sends a test request to determine the embedding dimensions.
func (t *HuggingFaceTEIEmbeddings) probeDimensions(ctx context.Context) (int, error) {
	// Send a simple test embedding
	embedding, err := t.Embed(ctx, "test")
	if err != nil {
		return 0, err
	}
	return len(embedding), nil
}
