package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"time"
)

const (
	anthropicBaseURL    = "https://api.anthropic.com/v1"
	anthropicVersion    = "2023-06-01"
	anthropicMaxRetries = 3
)

func init() {
	RegisterFactory("anthropic", func(config map[string]any) (Provider, error) {
		apiKey := ""
		if key, ok := config["api_key"].(string); ok {
			apiKey = key
		}
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}

		baseURL := anthropicBaseURL
		if url, ok := config["base_url"].(string); ok && url != "" {
			baseURL = url
		}

		return NewAnthropicProvider(apiKey, baseURL), nil
	})
}

// AnthropicProvider implements Provider for Anthropic API
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CreateCompletion creates a completion
func (p *AnthropicProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}

	anthropicReq := p.buildRequest(req, model, false)

	var resp anthropicResponse
	if err := p.doRequestWithRetry(ctx, "/messages", anthropicReq, &resp); err != nil {
		return nil, err
	}

	return p.parseResponse(&resp)
}

// CreateStructured creates a structured response
func (p *AnthropicProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	// Anthropic uses tool_use for structured output
	model := req.Model
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}

	// Add schema as a tool if provided
	modReq := req.CompletionRequest
	if len(req.ResponseSchema) > 0 {
		modReq.Tools = append(modReq.Tools, Tool{
			Name:        "structured_output",
			Description: "Use this tool to provide your structured response",
			Parameters:  req.ResponseSchema,
		})
		// Append instruction to use the tool
		if len(modReq.Messages) > 0 {
			lastIdx := len(modReq.Messages) - 1
			modReq.Messages[lastIdx].Content += "\n\nPlease use the structured_output tool to provide your response in the required format."
		}
	}

	anthropicReq := p.buildRequest(modReq, model, false)

	var resp anthropicResponse
	if err := p.doRequestWithRetry(ctx, "/messages", anthropicReq, &resp); err != nil {
		return nil, err
	}

	compResp, err := p.parseResponse(&resp)
	if err != nil {
		return nil, err
	}

	// Extract structured data from tool_use
	var data json.RawMessage
	for _, block := range resp.Content {
		if block.Type == "tool_use" && block.Name == "structured_output" {
			data = block.Input
			break
		}
	}

	if len(data) == 0 {
		data = json.RawMessage(compResp.Content)
	}

	return &StructuredResponse{
		Data:               data,
		CompletionResponse: *compResp,
	}, nil
}

// CreateStreaming creates a streaming response
func (p *AnthropicProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	model := req.Model
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}

	anthropicReq := p.buildRequest(req, model, true)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, NewProviderError("anthropic", ErrorCodeTimeout, err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		return nil, p.handleErrorResponse(resp)
	}

	return &anthropicStream{reader: bufio.NewReader(resp.Body), closer: resp.Body}, nil
}

func (p *AnthropicProvider) buildRequest(req CompletionRequest, model string, stream bool) anthropicRequest {
	var system string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		messages = append(messages, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	aReq := anthropicRequest{
		Model:       model,
		Messages:    messages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		Stream:      stream,
	}

	if len(req.Tools) > 0 {
		aReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, t := range req.Tools {
			aReq.Tools[i] = anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.Parameters,
			}
		}
	}

	return aReq
}

func (p *AnthropicProvider) doRequestWithRetry(ctx context.Context, endpoint string, reqBody any, result any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < anthropicMaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = NewProviderError("anthropic", ErrorCodeTimeout, err.Error(), err)
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = p.handleErrorResponse(resp)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return p.handleErrorResponse(resp)
		}

		return json.NewDecoder(resp.Body).Decode(result)
	}

	return lastErr
}

func (p *AnthropicProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp anthropicResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		code := ErrorCodeUnknown
		switch resp.StatusCode {
		case 401:
			code = ErrorCodeAuthentication
		case 429:
			code = ErrorCodeRateLimit
		case 400:
			code = ErrorCodeInvalidRequest
		case 404:
			code = ErrorCodeModelNotFound
		default:
			if resp.StatusCode >= 500 {
				code = ErrorCodeServerError
			}
		}
		return &ProviderError{
			Provider:    "anthropic",
			Code:        code,
			Message:     errResp.Error.Message,
			Type:        errResp.Error.Type,
			StatusCode:  resp.StatusCode,
			IsRetryable: code == ErrorCodeRateLimit || code == ErrorCodeServerError,
		}
	}

	return NewProviderError("anthropic", ErrorCodeUnknown, string(body), nil)
}

func (p *AnthropicProvider) parseResponse(resp *anthropicResponse) (*CompletionResponse, error) {
	var content string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      block.Name,
					Arguments: block.Input,
				},
			})
		}
	}

	finishReason := resp.StopReason
	switch finishReason {
	case "end_turn":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	}

	return &CompletionResponse{
		Content:      content,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
		Raw: resp,
	}, nil
}

// anthropicStream implements Stream for Anthropic
type anthropicStream struct {
	reader *bufio.Reader
	closer io.Closer
}

func (s *anthropicStream) Recv() (*StreamChunk, error) {
	for {
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return &StreamChunk{FinishReason: "stop"}, io.EOF
			}
			return nil, err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			ContentBlock *anthropicContentBlock `json:"content_block"`
			Index        int                    `json:"index"`
		}

		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				return &StreamChunk{Delta: event.Delta.Text}, nil
			}
		case "message_delta":
			if event.Delta.StopReason != "" {
				reason := event.Delta.StopReason
				if reason == "end_turn" {
					reason = "stop"
				}
				return &StreamChunk{FinishReason: reason}, nil
			}
		case "message_stop":
			return &StreamChunk{FinishReason: "stop"}, io.EOF
		}
	}
}

func (s *anthropicStream) Close() error {
	return s.closer.Close()
}

// anthropicModelsResponse represents the response from GET /v1/models
type anthropicModelsResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		DisplayName string `json:"display_name"`
		CreatedAt   string `json:"created_at"`
	} `json:"data"`
	FirstID string `json:"first_id"`
	LastID  string `json:"last_id"`
	HasMore bool   `json:"has_more"`
}

// anthropicModelPricing contains known pricing for Anthropic models (per 1M tokens)
var anthropicModelPricing = map[string]struct {
	input       float64
	output      float64
	description string
}{
	// Claude 4 series
	"claude-opus-4-6":         {15.00, 75.00, "Powerful, large model for complex challenges"},
	"claude-sonnet-4-6":       {3.00, 15.00, "Smart, efficient model for everyday use"},
	"claude-opus-4-5-20251101": {15.00, 75.00, "Previous Opus version"},
	"claude-haiku-4-5-20251001": {0.25, 1.25, "Fastest model for daily tasks"},
	// Legacy Claude 3.5 series
	"claude-3-5-sonnet-20241022": {3.00, 15.00, "Claude 3.5 Sonnet"},
	"claude-3-5-haiku-20241022":  {0.25, 1.25, "Claude 3.5 Haiku"},
	// Claude 3 series
	"claude-3-opus-20240229":   {15.00, 75.00, "Most capable Claude 3"},
	"claude-3-sonnet-20240229": {3.00, 15.00, "Balanced Claude 3"},
	"claude-3-haiku-20240307":  {0.25, 1.25, "Fast Claude 3"},
}

// ListModels fetches available models from Anthropic API
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, NewProviderError("anthropic", ErrorCodeTimeout, err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp)
	}

	var modelsResp anthropicModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, NewProviderError("anthropic", ErrorCodeUnknown, "failed to decode models response", err)
	}

	var models []ModelInfo
	for _, m := range modelsResp.Data {
		info := ModelInfo{
			ID:       m.ID,
			Name:     m.DisplayName,
			Provider: "anthropic",
		}

		if info.Name == "" {
			info.Name = m.ID
		}

		// Add pricing and description if known
		if pricing, ok := anthropicModelPricing[m.ID]; ok {
			info.InputCost = pricing.input
			info.OutputCost = pricing.output
			info.Description = pricing.description
		} else {
			info.Description = "Anthropic Claude model"
		}

		models = append(models, info)
	}

	return models, nil
}
