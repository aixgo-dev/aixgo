package provider

import (
	"context"
	"encoding/json"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// CreateCompletion creates a completion (unstructured text response)
	CreateCompletion(ctx context.Context, request CompletionRequest) (*CompletionResponse, error)

	// CreateStructured creates a structured response with schema validation
	CreateStructured(ctx context.Context, request StructuredRequest) (*StructuredResponse, error)

	// CreateStreaming creates a streaming response
	CreateStreaming(ctx context.Context, request CompletionRequest) (Stream, error)

	// Name returns the provider name (e.g., "openai", "anthropic")
	Name() string
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"` // The message content
}

// Tool represents a function/tool that can be called by the LLM
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema for parameters
}

// CompletionRequest represents a completion request
type CompletionRequest struct {
	// Messages is the conversation history
	Messages []Message `json:"messages"`

	// Model is the model to use (e.g., "gpt-4", "claude-3-opus")
	Model string `json:"model,omitempty"`

	// Temperature controls randomness (0.0-2.0)
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens is the maximum number of tokens to generate
	MaxTokens int `json:"max_tokens,omitempty"`

	// Tools available for the model to call
	Tools []Tool `json:"tools,omitempty"`

	// MaxIterations is the maximum number of ReAct iterations (for ReAct providers)
	MaxIterations int `json:"max_iterations,omitempty"`

	// TokenBudget is the total token budget for the entire conversation/loop
	TokenBudget int `json:"token_budget,omitempty"`

	// Additional provider-specific options
	Extra map[string]any `json:"extra,omitempty"`
}

// CompletionResponse represents a completion response
type CompletionResponse struct {
	// Content is the generated text
	Content string `json:"content"`

	// FinishReason explains why generation stopped
	FinishReason string `json:"finish_reason"`

	// Usage contains token usage information
	Usage Usage `json:"usage"`

	// ToolCalls if the model called any tools
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// Raw is the raw provider response for debugging
	Raw any `json:"raw,omitempty"`
}

// StructuredRequest represents a request for structured output
type StructuredRequest struct {
	CompletionRequest

	// ResponseSchema is the JSON Schema for the expected response
	ResponseSchema json.RawMessage `json:"response_schema"`

	// ResponseFormat specifies the format (e.g., "json_object", "json_schema")
	ResponseFormat string `json:"response_format,omitempty"`

	// StrictSchema enables strict schema adherence (provider-dependent)
	StrictSchema bool `json:"strict_schema,omitempty"`
}

// StructuredResponse represents a structured response
type StructuredResponse struct {
	// Data is the parsed structured data
	Data json.RawMessage `json:"data"`

	// Raw completion response
	CompletionResponse
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ToolCall represents a function call made by the model
type ToolCall struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"` // "function"
	Function  FunctionCall    `json:"function"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Stream represents a streaming response
type Stream interface {
	// Recv receives the next chunk
	Recv() (*StreamChunk, error)

	// Close closes the stream
	Close() error
}

// StreamChunk represents a chunk in a streaming response
type StreamChunk struct {
	// Delta is the incremental content
	Delta string `json:"delta"`

	// FinishReason if this is the last chunk
	FinishReason string `json:"finish_reason,omitempty"`

	// ToolCallDeltas if tools are being called
	ToolCallDeltas []ToolCallDelta `json:"tool_call_deltas,omitempty"`
}

// ToolCallDelta represents an incremental tool call update
type ToolCallDelta struct {
	Index         int    `json:"index"`
	ID            string `json:"id,omitempty"`
	Type          string `json:"type,omitempty"`
	FunctionName  string `json:"function_name,omitempty"`
	ArgumentDelta string `json:"argument_delta,omitempty"`
}

// ProviderError represents a provider-specific error
type ProviderError struct {
	Provider      string `json:"provider"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	Type          string `json:"type,omitempty"`
	StatusCode    int    `json:"status_code,omitempty"`
	IsRetryable   bool   `json:"is_retryable"`
	OriginalError error  `json:"-"`
}

// Error implements the error interface
func (e *ProviderError) Error() string {
	return e.Provider + " error: " + e.Message
}

// Unwrap returns the original error
func (e *ProviderError) Unwrap() error {
	return e.OriginalError
}

// Common error codes
const (
	ErrorCodeInvalidRequest  = "invalid_request"
	ErrorCodeAuthentication  = "authentication_error"
	ErrorCodeRateLimit       = "rate_limit_exceeded"
	ErrorCodeQuotaExceeded   = "quota_exceeded"
	ErrorCodeServerError     = "server_error"
	ErrorCodeTimeout         = "timeout"
	ErrorCodeModelNotFound   = "model_not_found"
	ErrorCodeContentFiltered = "content_filtered"
	ErrorCodeUnknown         = "unknown_error"
)

// NewProviderError creates a new provider error
func NewProviderError(provider, code, message string, original error) *ProviderError {
	return &ProviderError{
		Provider:      provider,
		Code:          code,
		Message:       message,
		OriginalError: original,
		IsRetryable:   isRetryableError(code),
	}
}

// isRetryableError determines if an error code is retryable
func isRetryableError(code string) bool {
	switch code {
	case ErrorCodeRateLimit, ErrorCodeServerError, ErrorCodeTimeout:
		return true
	default:
		return false
	}
}
