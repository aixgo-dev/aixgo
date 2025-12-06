package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// LangfuseClient provides direct integration with Langfuse SDK features
// beyond what's available through OpenTelemetry.
// This includes LLM-specific features like Generations, Scores, and Feedback.
type LangfuseClient struct {
	baseURL    string
	publicKey  string
	secretKey  string
	httpClient *http.Client
	enabled    bool
	mu         sync.Mutex
}

// LangfuseConfig contains configuration for Langfuse integration
type LangfuseConfig struct {
	// BaseURL is the Langfuse API endpoint (defaults to cloud.langfuse.com)
	BaseURL string

	// PublicKey is the Langfuse public key
	PublicKey string

	// SecretKey is the Langfuse secret key
	SecretKey string

	// Enabled controls whether Langfuse integration is active
	Enabled bool
}

// Generation represents an LLM generation event in Langfuse
type Generation struct {
	ID              string                 `json:"id,omitempty"`
	Name            string                 `json:"name,omitempty"`
	StartTime       time.Time              `json:"startTime"`
	EndTime         time.Time              `json:"endTime,omitempty"`
	Model           string                 `json:"model"`
	ModelParameters map[string]interface{} `json:"modelParameters,omitempty"`
	Input           interface{}            `json:"input,omitempty"`
	Output          interface{}            `json:"output,omitempty"`
	Usage           *LangfuseUsage         `json:"usage,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	Level           string                 `json:"level,omitempty"` // "DEBUG", "DEFAULT", "WARNING", "ERROR"
	StatusMessage   string                 `json:"statusMessage,omitempty"`
	Version         string                 `json:"version,omitempty"`
	TraceID         string                 `json:"traceId,omitempty"`
	ParentID        string                 `json:"parentObservationId,omitempty"`
}

// LangfuseUsage represents token usage and cost
type LangfuseUsage struct {
	PromptTokens     int     `json:"promptTokens,omitempty"`
	CompletionTokens int     `json:"completionTokens,omitempty"`
	TotalTokens      int     `json:"totalTokens,omitempty"`
	Unit             string  `json:"unit,omitempty"` // "TOKENS", "CHARACTERS", "MILLISECONDS", etc.
	InputCost        float64 `json:"inputCost,omitempty"`
	OutputCost       float64 `json:"outputCost,omitempty"`
	TotalCost        float64 `json:"totalCost,omitempty"`
}

// Score represents a score/evaluation for a trace or generation
type Score struct {
	ID          string                 `json:"id,omitempty"`
	TraceID     string                 `json:"traceId"`
	Name        string                 `json:"name"`
	Value       float64                `json:"value"`
	Comment     string                 `json:"comment,omitempty"`
	ObservationID string               `json:"observationId,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

var (
	// DefaultLangfuseClient is the global Langfuse client instance
	DefaultLangfuseClient *LangfuseClient
	langfuseInitOnce      sync.Once
)

// InitLangfuse initializes the Langfuse client from environment variables
func InitLangfuse() error {
	config := LangfuseConfig{
		BaseURL:   getEnv("LANGFUSE_BASE_URL", "https://cloud.langfuse.com"),
		PublicKey: os.Getenv("LANGFUSE_PUBLIC_KEY"),
		SecretKey: os.Getenv("LANGFUSE_SECRET_KEY"),
		Enabled:   getEnv("LANGFUSE_ENABLED", "true") == "true",
	}

	// Disable if no credentials provided
	if config.PublicKey == "" || config.SecretKey == "" {
		config.Enabled = false
	}

	langfuseInitOnce.Do(func() {
		DefaultLangfuseClient = NewLangfuseClient(config)
	})

	return nil
}

// NewLangfuseClient creates a new Langfuse client
func NewLangfuseClient(config LangfuseConfig) *LangfuseClient {
	return &LangfuseClient{
		baseURL:   config.BaseURL,
		publicKey: config.PublicKey,
		secretKey: config.SecretKey,
		enabled:   config.Enabled,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TrackGeneration tracks an LLM generation in Langfuse
func (c *LangfuseClient) TrackGeneration(ctx context.Context, gen *Generation) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create the request payload
	payload := map[string]interface{}{
		"type": "generation-create",
		"body": gen,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal generation: %w", err)
	}

	// Send to Langfuse API
	url := fmt.Sprintf("%s/api/public/ingestion", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	req.SetBasicAuth(c.publicKey, c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send generation: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("langfuse API returned status %d", resp.StatusCode)
	}

	return nil
}

// TrackScore tracks a score/evaluation in Langfuse
func (c *LangfuseClient) TrackScore(ctx context.Context, score *Score) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create the request payload
	payload := map[string]interface{}{
		"type": "score-create",
		"body": score,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal score: %w", err)
	}

	// Send to Langfuse API
	url := fmt.Sprintf("%s/api/public/ingestion", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	req.SetBasicAuth(c.publicKey, c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send score: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("langfuse API returned status %d", resp.StatusCode)
	}

	return nil
}

// Flush ensures all pending events are sent (no-op for HTTP client)
func (c *LangfuseClient) Flush(ctx context.Context) error {
	// HTTP client sends immediately, so nothing to flush
	return nil
}

// Close closes the Langfuse client
func (c *LangfuseClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// Helper functions for creating Langfuse events

// NewGeneration creates a new Generation event
func NewGeneration(name, model string, startTime time.Time) *Generation {
	return &Generation{
		Name:      name,
		Model:     model,
		StartTime: startTime,
		Level:     "DEFAULT",
	}
}

// WithInput adds input to a generation
func (g *Generation) WithInput(input interface{}) *Generation {
	g.Input = input
	return g
}

// WithOutput adds output to a generation
func (g *Generation) WithOutput(output interface{}) *Generation {
	g.Output = output
	return g
}

// WithUsage adds usage information to a generation
func (g *Generation) WithUsage(promptTokens, completionTokens int, inputCost, outputCost float64) *Generation {
	g.Usage = &LangfuseUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		Unit:             "TOKENS",
		InputCost:        inputCost,
		OutputCost:       outputCost,
		TotalCost:        inputCost + outputCost,
	}
	return g
}

// WithMetadata adds metadata to a generation
func (g *Generation) WithMetadata(metadata map[string]interface{}) *Generation {
	g.Metadata = metadata
	return g
}

// WithTraceID associates the generation with a trace
func (g *Generation) WithTraceID(traceID string) *Generation {
	g.TraceID = traceID
	return g
}

// Finish marks the generation as complete
func (g *Generation) Finish() *Generation {
	g.EndTime = time.Now()
	return g
}

// NewScore creates a new Score event
func NewScore(traceID, name string, value float64) *Score {
	return &Score{
		TraceID: traceID,
		Name:    name,
		Value:   value,
	}
}

// WithComment adds a comment to a score
func (s *Score) WithComment(comment string) *Score {
	s.Comment = comment
	return s
}

// WithObservationID associates the score with a specific observation
func (s *Score) WithObservationID(observationID string) *Score {
	s.ObservationID = observationID
	return s
}
