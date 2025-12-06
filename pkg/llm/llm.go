package llm

import (
	"context"
	"fmt"
)

// Client provides a unified interface for LLM providers
type Client interface {
	// Complete generates a completion for the given prompt
	Complete(ctx context.Context, prompt string, opts ...Option) (*Response, error)

	// Chat performs a chat completion with message history
	Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error)

	// Embed generates embeddings for the given texts
	Embed(ctx context.Context, texts []string) ([][]float64, error)

	// Close closes the client and releases resources
	Close() error
}

// Message represents a chat message
type Message struct {
	Role    string // system, user, assistant
	Content string
}

// Response represents an LLM response
type Response struct {
	Content      string
	FinishReason string
	Usage        Usage
	Model        string
}

// Usage tracks token usage
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Options holds generation options
type Options struct {
	Model       string
	MaxTokens   int
	Temperature float64
	TopP        float64
	Stop        []string
}

// Option is a functional option for LLM requests
type Option func(*Options)

// WithModel sets the model to use
func WithModel(model string) Option {
	return func(o *Options) {
		o.Model = model
	}
}

// WithMaxTokens sets the maximum tokens to generate
func WithMaxTokens(tokens int) Option {
	return func(o *Options) {
		o.MaxTokens = tokens
	}
}

// WithTemperature sets the temperature
func WithTemperature(temp float64) Option {
	return func(o *Options) {
		o.Temperature = temp
	}
}

// WithTopP sets the top-p sampling parameter
func WithTopP(p float64) Option {
	return func(o *Options) {
		o.TopP = p
	}
}

// WithStop sets stop sequences
func WithStop(stop ...string) Option {
	return func(o *Options) {
		o.Stop = stop
	}
}

// NewClient creates a new LLM client based on the provider
func NewClient(provider string, apiKey string) (Client, error) {
	switch provider {
	case "openai":
		return NewOpenAIClient(apiKey)
	case "anthropic":
		return NewAnthropicClient(apiKey)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// OpenAIClient implements Client for OpenAI
type OpenAIClient struct {
	apiKey string
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}
	return &OpenAIClient{apiKey: apiKey}, nil
}

func (c *OpenAIClient) Complete(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	return nil, fmt.Errorf("not implemented: use Chat instead")
}

func (c *OpenAIClient) Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// Stub implementation - would integrate with OpenAI API
	return &Response{
		Content:      "Mock response from OpenAI",
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Model: "gpt-4",
	}, nil
}

func (c *OpenAIClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	// Stub implementation
	embeddings := make([][]float64, len(texts))
	for i := range texts {
		embeddings[i] = make([]float64, 1536) // OpenAI embedding dimension
	}
	return embeddings, nil
}

func (c *OpenAIClient) Close() error {
	return nil
}

// AnthropicClient implements Client for Anthropic
type AnthropicClient struct {
	apiKey string
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}
	return &AnthropicClient{apiKey: apiKey}, nil
}

func (c *AnthropicClient) Complete(ctx context.Context, prompt string, opts ...Option) (*Response, error) {
	// Stub implementation - would integrate with Anthropic API
	return &Response{
		Content:      "Mock response from Anthropic",
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		Model: "claude-3-sonnet",
	}, nil
}

func (c *AnthropicClient) Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	return c.Complete(ctx, messages[len(messages)-1].Content, opts...)
}

func (c *AnthropicClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, fmt.Errorf("Anthropic does not support embeddings")
}

func (c *AnthropicClient) Close() error {
	return nil
}
