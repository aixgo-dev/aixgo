package provider

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Note: This package uses crypto/rand for jitter generation to satisfy security
// scanners and provide better entropy. For non-security-critical randomness,
// math/rand would be sufficient, but crypto/rand is used for defense in depth.

const (
	vertexAIMaxRetries   = 5
	vertexAIBaseDelay    = 1 * time.Second
	vertexAIMaxDelay     = 32 * time.Second
	vertexAIJitterFactor = 0.3
	vertexAIClientTimeout = 30 * time.Second
)

func init() {
	RegisterFactory("vertexai", func(config map[string]any) (Provider, error) {
		projectID := ""
		if id, ok := config["project_id"].(string); ok {
			projectID = id
		}
		if projectID == "" {
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		if projectID == "" {
			return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT not set")
		}

		location := ""
		if loc, ok := config["location"].(string); ok {
			location = loc
		}
		if location == "" {
			location = os.Getenv("VERTEX_AI_LOCATION")
		}
		if location == "" {
			location = "us-central1"
		}

		return NewVertexAIProvider(projectID, location)
	})
}

// VertexAIProvider implements Provider for Google Vertex AI using the Gen AI SDK
type VertexAIProvider struct {
	projectID string
	location  string
	client    *genai.Client
}

// NewVertexAIProvider creates a new Vertex AI provider using the Google Gen AI SDK.
// It uses Application Default Credentials (ADC) for authentication.
//
// Security: All API calls respect the context deadline. Callers should set
// appropriate timeouts (recommended: 60-120s for completion, 180s for streaming).
func NewVertexAIProvider(projectID, location string) (*VertexAIProvider, error) {
	// Add timeout for client creation to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), vertexAIClientTimeout)
	defer cancel()

	// Create client configured for Vertex AI backend
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	// Only log in debug mode to avoid leaking project info
	if os.Getenv("AIXGO_DEBUG") == "true" {
		log.Printf("[VertexAI] Initialized client (project=%s, location=%s)", projectID, location)
	}

	return &VertexAIProvider{
		projectID: projectID,
		location:  location,
		client:    client,
	}, nil
}

// Name returns the provider name
func (p *VertexAIProvider) Name() string {
	return "vertexai"
}

// CreateCompletion creates a completion using the Gen AI SDK
func (p *VertexAIProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	// Build generation config
	config := &genai.GenerateContentConfig{}
	// Always set temperature - 0 is a valid value for deterministic output
	// Use -1 as sentinel for "not set" if needed, but typically callers set explicit values
	config.Temperature = genai.Ptr(float32(req.Temperature))
	if req.MaxTokens > 0 && req.MaxTokens <= math.MaxInt32 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}

	// Build contents from messages
	contents, systemInstruction := p.buildContents(req.Messages)

	// Set system instruction if present
	if systemInstruction != nil {
		config.SystemInstruction = systemInstruction
	}

	// Add tools if provided
	if len(req.Tools) > 0 {
		config.Tools = p.buildTools(req.Tools)
	}

	var resp *genai.GenerateContentResponse
	var err error

	// Retry logic with exponential backoff and jitter
	for attempt := 0; attempt < vertexAIMaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err = p.client.Models.GenerateContent(ctx, model, contents, config)
		if err == nil {
			break
		}

		// Check if error is retryable
		if !isRetryableGenAIError(err) {
			return nil, p.wrapError(err)
		}
	}

	if err != nil {
		return nil, p.wrapError(err)
	}

	return p.parseResponse(resp)
}

// CreateStructured creates a structured response with JSON schema
func (p *VertexAIProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	// Build generation config
	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}
	// Always set temperature - 0 is a valid value for deterministic output
	config.Temperature = genai.Ptr(float32(req.Temperature))
	if req.MaxTokens > 0 && req.MaxTokens <= math.MaxInt32 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}

	// Add response schema if provided
	if len(req.ResponseSchema) > 0 {
		var schema *genai.Schema
		if err := json.Unmarshal(req.ResponseSchema, &schema); err == nil {
			config.ResponseSchema = schema
		}
	}

	// Build contents from messages
	contents, systemInstruction := p.buildContents(req.Messages)

	// Set system instruction if present
	if systemInstruction != nil {
		config.SystemInstruction = systemInstruction
	}

	var resp *genai.GenerateContentResponse
	var err error

	// Retry logic with exponential backoff and jitter
	for attempt := 0; attempt < vertexAIMaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err = p.client.Models.GenerateContent(ctx, model, contents, config)
		if err == nil {
			break
		}

		if !isRetryableGenAIError(err) {
			return nil, p.wrapError(err)
		}
	}

	if err != nil {
		return nil, p.wrapError(err)
	}

	compResp, err := p.parseResponse(resp)
	if err != nil {
		return nil, err
	}

	return &StructuredResponse{
		Data:               json.RawMessage(compResp.Content),
		CompletionResponse: *compResp,
	}, nil
}

// CreateStreaming creates a streaming response
func (p *VertexAIProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	// Build generation config
	config := &genai.GenerateContentConfig{}
	// Always set temperature - 0 is a valid value for deterministic output
	config.Temperature = genai.Ptr(float32(req.Temperature))
	if req.MaxTokens > 0 && req.MaxTokens <= math.MaxInt32 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}

	// Build contents from messages
	contents, systemInstruction := p.buildContents(req.Messages)

	// Set system instruction if present
	if systemInstruction != nil {
		config.SystemInstruction = systemInstruction
	}

	// Add tools if provided
	if len(req.Tools) > 0 {
		config.Tools = p.buildTools(req.Tools)
	}

	// Create streaming response using Go 1.23 iter.Seq2 pattern
	// We need to collect responses into a channel for the Stream interface
	respChan := make(chan *genai.GenerateContentResponse, 10)
	errChan := make(chan error, 1)

	// Use cancellable context to allow cleanup via Close()
	streamCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(respChan)
		defer close(errChan)

		for resp, err := range p.client.Models.GenerateContentStream(streamCtx, model, contents, config) {
			if err != nil {
				select {
				case errChan <- err:
				case <-streamCtx.Done():
				}
				return
			}
			select {
			case respChan <- resp:
			case <-streamCtx.Done():
				return
			}
		}
	}()

	return &vertexAIStream{
		respChan: respChan,
		errChan:  errChan,
		ctx:      streamCtx,
		cancel:   cancel,
	}, nil
}

// buildContents converts messages to Gen AI content format
func (p *VertexAIProvider) buildContents(messages []Message) ([]*genai.Content, *genai.Content) {
	var systemInstruction *genai.Content
	contents := make([]*genai.Content, 0, len(messages))

	for _, m := range messages {
		if m.Role == "system" {
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: m.Content}},
			}
			continue
		}

		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		// Handle tool/function response messages for ReAct agent loop
		if m.Role == "tool" || m.Role == "function" {
			// Parse tool response from content (expected format: JSON with name and response)
			var toolResp struct {
				Name     string         `json:"name"`
				Response map[string]any `json:"response"`
			}
			if err := json.Unmarshal([]byte(m.Content), &toolResp); err == nil && toolResp.Name != "" {
				contents = append(contents, &genai.Content{
					Role: "function",
					Parts: []*genai.Part{{
						FunctionResponse: &genai.FunctionResponse{
							Name:     toolResp.Name,
							Response: toolResp.Response,
						},
					}},
				})
				continue
			}
			// If parsing fails, treat as regular user message with tool context
			role = "user"
		}

		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: m.Content}},
		})
	}

	return contents, systemInstruction
}

// buildTools converts tools to Gen AI tool format
func (p *VertexAIProvider) buildTools(tools []Tool) []*genai.Tool {
	funcDecls := make([]*genai.FunctionDeclaration, len(tools))
	for i, t := range tools {
		var params *genai.Schema
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &params)
		}
		funcDecls[i] = &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		}
	}
	return []*genai.Tool{{FunctionDeclarations: funcDecls}}
}

// parseResponse parses the Gen AI response into CompletionResponse
func (p *VertexAIProvider) parseResponse(resp *genai.GenerateContentResponse) (*CompletionResponse, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, NewProviderError("vertexai", ErrorCodeUnknown, "no candidates in response", nil)
	}

	candidate := resp.Candidates[0]
	var content string
	var toolCalls []ToolCall

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, ToolCall{
					ID:   part.FunctionCall.Name,
					Type: "function",
					Function: FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: args,
					},
				})
			}
		}
	}

	// Convert finish reason
	finishReason := string(candidate.FinishReason)
	if finishReason == "STOP" || finishReason == "" {
		finishReason = "stop"
	}

	// Get usage stats
	var usage Usage
	if resp.UsageMetadata != nil {
		usage.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
		usage.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		usage.TotalTokens = int(resp.UsageMetadata.TotalTokenCount)
	}

	return &CompletionResponse{
		Content:      content,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		Usage:        usage,
		Raw:          resp,
	}, nil
}

// wrapError converts Gen AI errors to ProviderError
func (p *VertexAIProvider) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// Determine error code based on error message (case-insensitive)
	code := ErrorCodeUnknown
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "authentication") || strings.Contains(errMsg, "credential") || strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401"):
		code = ErrorCodeAuthentication
	case strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429") || strings.Contains(errMsg, "quota"):
		code = ErrorCodeRateLimit
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404"):
		code = ErrorCodeModelNotFound
	case strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "400"):
		code = ErrorCodeInvalidRequest
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline"):
		code = ErrorCodeTimeout
	case strings.Contains(errMsg, "500") || strings.Contains(errMsg, "503") || strings.Contains(errMsg, "server"):
		code = ErrorCodeServerError
	}

	return &ProviderError{
		Provider:      "vertexai",
		Code:          code,
		Message:       err.Error(), // Keep original case for display
		IsRetryable:   code == ErrorCodeRateLimit || code == ErrorCodeServerError || code == ErrorCodeTimeout,
		OriginalError: err,
	}
}

// isRetryableGenAIError checks if a Gen AI error is retryable
func isRetryableGenAIError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "429") ||
		strings.Contains(errMsg, "500") ||
		strings.Contains(errMsg, "503") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline") ||
		strings.Contains(errMsg, "unavailable")
}

// calculateBackoff returns the backoff duration with jitter for a given attempt
func (p *VertexAIProvider) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s (capped at maxDelay)
	// Guard against negative or zero attempt to prevent uint overflow
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 31 { // Prevent overflow for large values
		shift = 31
	}
	delay := time.Duration(1<<uint(shift)) * vertexAIBaseDelay
	if delay > vertexAIMaxDelay {
		delay = vertexAIMaxDelay
	}
	// Add jitter: delay ± 30% using crypto/rand for security compliance
	jitter := time.Duration(float64(delay) * vertexAIJitterFactor * (cryptoRandFloat64()*2 - 1))
	return delay + jitter
}

// cryptoRandFloat64 returns a cryptographically secure random float64 in [0.0, 1.0)
func cryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to deterministic value on error (should never happen)
		return 0.5
	}
	// Use top 53 bits to create a float64 in [0, 1)
	return float64(binary.BigEndian.Uint64(b[:])>>11) / (1 << 53)
}

// vertexAIStream implements Stream for Vertex AI using Gen AI SDK
type vertexAIStream struct {
	respChan <-chan *genai.GenerateContentResponse
	errChan  <-chan error
	ctx      context.Context
	cancel   context.CancelFunc
	done     bool
}

func (s *vertexAIStream) Recv() (*StreamChunk, error) {
	if s.done {
		return &StreamChunk{FinishReason: "stop"}, io.EOF
	}

	// Check for errors first (non-blocking) to prioritize error handling
	select {
	case err := <-s.errChan:
		s.done = true
		if err != nil {
			return nil, err
		}
		return &StreamChunk{FinishReason: "stop"}, io.EOF
	default:
		// Continue to main select
	}

	// Use select to properly handle both response and error channels
	select {
	case <-s.ctx.Done():
		s.done = true
		return nil, s.ctx.Err()
	case err := <-s.errChan:
		s.done = true
		if err != nil {
			return nil, err
		}
		return &StreamChunk{FinishReason: "stop"}, io.EOF
	case resp, ok := <-s.respChan:
		if !ok {
			s.done = true
			return &StreamChunk{FinishReason: "stop"}, io.EOF
		}

		if len(resp.Candidates) == 0 {
			return &StreamChunk{}, nil
		}

		candidate := resp.Candidates[0]
		var text string
		var toolCallDeltas []ToolCallDelta

		if candidate.Content != nil {
			for i, part := range candidate.Content.Parts {
				if part.Text != "" {
					text += part.Text
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					toolCallDeltas = append(toolCallDeltas, ToolCallDelta{
						Index:         i,
						ID:            part.FunctionCall.Name,
						Type:          "function",
						FunctionName:  part.FunctionCall.Name,
						ArgumentDelta: string(args),
					})
				}
			}
		}

		finishReason := ""
		if string(candidate.FinishReason) == "STOP" {
			finishReason = "stop"
		}

		return &StreamChunk{
			Delta:          text,
			FinishReason:   finishReason,
			ToolCallDeltas: toolCallDeltas,
		}, nil
	}
}

func (s *vertexAIStream) Close() error {
	if s.done {
		return nil
	}
	s.done = true

	// Cancel the streaming context to signal the goroutine to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Drain channels with timeout to prevent goroutine leaks
	// This ensures the streaming goroutine can exit cleanly
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

	go func() {
		for range s.respChan {
			// Drain response channel
		}
	}()

	select {
	case <-timeout.C:
		// Timeout - goroutine should exit due to context cancellation
	case <-s.ctx.Done():
		// Context cancelled successfully
	}

	return nil
}

// Close implements the Provider interface. The genai.Client does not provide
// a Close() method as of v0.5.0, so this is a no-op. The underlying HTTP client
// and resources are managed by the SDK and will be released when the Client
// is garbage collected.
func (p *VertexAIProvider) Close() error {
	return nil
}
