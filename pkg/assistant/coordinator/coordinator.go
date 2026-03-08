// Package coordinator provides agent orchestration for the chat assistant.
package coordinator

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aixgo-dev/aixgo/pkg/assistant/session"
	"github.com/aixgo-dev/aixgo/pkg/llm/cost"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
)

// Config holds configuration for the coordinator.
type Config struct {
	Model       string
	Streaming   bool
	MaxTokens   int
	Temperature float64
	SystemPrompt string
}

// Response represents a chat response.
type Response struct {
	Content      string
	Cost         float64
	InputTokens  int
	OutputTokens int
	Model        string
	FinishReason string
}

// Coordinator orchestrates LLM interactions for chat.
type Coordinator struct {
	config       Config
	provider     provider.Provider
	calculator   *cost.Calculator
	history      []provider.Message
	systemPrompt string
}

// New creates a new coordinator with the given configuration.
func New(config Config) (*Coordinator, error) {
	// Detect provider from model name
	providerName := provider.DetectProvider(config.Model)

	// Get or create provider
	prov, err := getOrCreateProvider(providerName, config.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Set default system prompt
	systemPrompt := config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt()
	}

	return &Coordinator{
		config:       config,
		provider:     prov,
		calculator:   cost.DefaultCalculator,
		history:      []provider.Message{},
		systemPrompt: systemPrompt,
	}, nil
}

// Chat sends a message and returns the response.
func (c *Coordinator) Chat(ctx context.Context, messages []session.Message) (*Response, error) {
	// Convert session messages to provider messages
	providerMsgs := c.buildMessages(messages)

	if c.config.Streaming {
		return c.chatStreaming(ctx, providerMsgs)
	}

	return c.chatCompletion(ctx, providerMsgs)
}

// chatCompletion sends a non-streaming completion request.
func (c *Coordinator) chatCompletion(ctx context.Context, messages []provider.Message) (*Response, error) {
	req := provider.CompletionRequest{
		Messages:    messages,
		Model:       c.config.Model,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
	}

	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	resp, err := c.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Calculate cost
	costResult, _ := c.calculator.Calculate(&cost.Usage{
		Model:        c.config.Model,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		TotalTokens:  resp.Usage.TotalTokens,
	})

	var totalCost float64
	if costResult != nil {
		totalCost = costResult.TotalCost
	}

	return &Response{
		Content:      resp.Content,
		Cost:         totalCost,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		Model:        c.config.Model,
		FinishReason: resp.FinishReason,
	}, nil
}

// chatStreaming sends a streaming completion request.
func (c *Coordinator) chatStreaming(ctx context.Context, messages []provider.Message) (*Response, error) {
	req := provider.CompletionRequest{
		Messages:    messages,
		Model:       c.config.Model,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
	}

	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	stream, err := c.provider.CreateStreaming(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("streaming failed: %w", err)
	}
	defer stream.Close()

	var content strings.Builder
	var finishReason string

	fmt.Println() // Start output on new line

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("stream recv failed: %w", err)
		}

		if chunk.Delta != "" {
			content.WriteString(chunk.Delta)
			fmt.Print(chunk.Delta)
			os.Stdout.Sync()
		}

		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}
	}

	fmt.Println() // End output with newline

	// Estimate cost (streaming doesn't provide usage stats)
	inputTokens := estimateTokens(messages)
	outputTokens := estimateTokens([]provider.Message{{Content: content.String()}})

	costResult, _ := c.calculator.Calculate(&cost.Usage{
		Model:        c.config.Model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	})

	var totalCost float64
	if costResult != nil {
		totalCost = costResult.TotalCost
	}

	return &Response{
		Content:      content.String(),
		Cost:         totalCost,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Model:        c.config.Model,
		FinishReason: finishReason,
	}, nil
}

// SetModel changes the current model.
func (c *Coordinator) SetModel(model string) error {
	providerName := provider.DetectProvider(model)

	prov, err := getOrCreateProvider(providerName, model)
	if err != nil {
		return fmt.Errorf("failed to get provider for model %s: %w", model, err)
	}

	c.provider = prov
	c.config.Model = model
	return nil
}

// ClearHistory clears the conversation history.
func (c *Coordinator) ClearHistory() {
	c.history = []provider.Message{}
}

// buildMessages converts session messages to provider messages.
func (c *Coordinator) buildMessages(messages []session.Message) []provider.Message {
	providerMsgs := make([]provider.Message, 0, len(messages)+1)

	// Add system prompt
	if c.systemPrompt != "" {
		providerMsgs = append(providerMsgs, provider.Message{
			Role:    "system",
			Content: c.systemPrompt,
		})
	}

	// Add conversation history
	for _, msg := range messages {
		providerMsgs = append(providerMsgs, provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return providerMsgs
}

// getOrCreateProvider gets an existing provider or creates a new one.
func getOrCreateProvider(providerName, model string) (provider.Provider, error) {
	// Try to get from registry first
	prov, err := provider.Get(providerName)
	if err == nil {
		return prov, nil
	}

	// Create new provider based on type
	config := map[string]any{
		"model": model,
	}

	// Add API key from environment
	switch providerName {
	case "openai":
		config["api_key"] = os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		config["api_key"] = os.Getenv("ANTHROPIC_API_KEY")
	case "gemini":
		config["api_key"] = os.Getenv("GOOGLE_API_KEY")
	case "xai":
		config["api_key"] = os.Getenv("XAI_API_KEY")
	case "vertexai":
		config["project_id"] = os.Getenv("GCP_PROJECT_ID")
	case "huggingface":
		config["api_key"] = os.Getenv("HUGGINGFACE_API_KEY")
	}

	return provider.CreateProvider(providerName, config)
}

// estimateTokens provides a rough token count estimate.
// ~4 chars per token is a reasonable approximation.
func estimateTokens(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4
	}
	if total == 0 {
		total = 1
	}
	return total
}

// defaultSystemPrompt returns the default system prompt for the assistant.
func defaultSystemPrompt() string {
	return `You are Aixgo Assistant, a helpful AI coding assistant.

You help users with software engineering tasks including:
- Writing and explaining code
- Debugging and fixing issues
- Code reviews and best practices
- Architecture and design decisions

Be concise and direct. Provide code examples when helpful.
Use markdown formatting for code blocks and structure.`
}
