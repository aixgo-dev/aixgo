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
	openaiBaseURL    = "https://api.openai.com/v1"
	openaiMaxRetries = 3
)

func init() {
	RegisterFactory("openai", func(config map[string]any) (Provider, error) {
		apiKey := ""
		if key, ok := config["api_key"].(string); ok {
			apiKey = key
		}
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}

		baseURL := openaiBaseURL
		if url, ok := config["base_url"].(string); ok && url != "" {
			baseURL = url
		}

		return NewOpenAIProvider(apiKey, baseURL), nil
	})
}

// OpenAIProvider implements Provider for OpenAI API
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// openaiRequest represents the OpenAI API request format
type openaiRequest struct {
	Model          string          `json:"model"`
	Messages       []openaiMessage `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Tools          []openaiTool    `json:"tools,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *openaiRespFmt  `json:"response_format,omitempty"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiRespFmt struct {
	Type       string            `json:"type"`
	JSONSchema *openaiJSONSchema `json:"json_schema,omitempty"`
}

type openaiJSONSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict,omitempty"`
	Schema json.RawMessage `json:"schema"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// CreateCompletion creates a completion
func (p *OpenAIProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = "gpt-4"
	}

	openaiReq := p.buildRequest(req, model, false)

	var resp openaiResponse
	if err := p.doRequestWithRetry(ctx, "/chat/completions", openaiReq, &resp); err != nil {
		return nil, err
	}

	return p.parseResponse(&resp)
}

// CreateStructured creates a structured response
func (p *OpenAIProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	model := req.Model
	if model == "" {
		model = "gpt-4"
	}

	openaiReq := p.buildRequest(req.CompletionRequest, model, false)

	// Add response format for structured output
	if len(req.ResponseSchema) > 0 {
		openaiReq.ResponseFormat = &openaiRespFmt{
			Type: "json_schema",
			JSONSchema: &openaiJSONSchema{
				Name:   "response",
				Strict: req.StrictSchema,
				Schema: req.ResponseSchema,
			},
		}
	} else {
		openaiReq.ResponseFormat = &openaiRespFmt{Type: "json_object"}
	}

	var resp openaiResponse
	if err := p.doRequestWithRetry(ctx, "/chat/completions", openaiReq, &resp); err != nil {
		return nil, err
	}

	compResp, err := p.parseResponse(&resp)
	if err != nil {
		return nil, err
	}

	return &StructuredResponse{
		Data:               json.RawMessage(compResp.Content),
		CompletionResponse: *compResp,
	}, nil
}

// CreateStreaming creates a streaming response
func (p *OpenAIProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	model := req.Model
	if model == "" {
		model = "gpt-4"
	}

	openaiReq := p.buildRequest(req, model, true)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, NewProviderError("openai", ErrorCodeTimeout, err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		return nil, p.handleErrorResponse(resp)
	}

	return &openaiStream{reader: bufio.NewReader(resp.Body), closer: resp.Body}, nil
}

func (p *OpenAIProvider) buildRequest(req CompletionRequest, model string, stream bool) openaiRequest {
	messages := make([]openaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = openaiMessage{Role: m.Role, Content: m.Content}
	}

	oReq := openaiRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      stream,
	}

	if len(req.Tools) > 0 {
		oReq.Tools = make([]openaiTool, len(req.Tools))
		for i, t := range req.Tools {
			oReq.Tools[i] = openaiTool{
				Type:     "function",
				Function: openaiFunction(t),
			}
		}
	}

	return oReq
}

func (p *OpenAIProvider) doRequestWithRetry(ctx context.Context, endpoint string, reqBody any, result any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < openaiMaxRetries; attempt++ {
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
		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = NewProviderError("openai", ErrorCodeTimeout, err.Error(), err)
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

func (p *OpenAIProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp openaiResponse
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
			Provider:    "openai",
			Code:        code,
			Message:     errResp.Error.Message,
			Type:        errResp.Error.Type,
			StatusCode:  resp.StatusCode,
			IsRetryable: code == ErrorCodeRateLimit || code == ErrorCodeServerError,
		}
	}

	return NewProviderError("openai", ErrorCodeUnknown, string(body), nil)
}

func (p *OpenAIProvider) parseResponse(resp *openaiResponse) (*CompletionResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, NewProviderError("openai", ErrorCodeUnknown, "no choices in response", nil)
	}

	choice := resp.Choices[0]
	result := &CompletionResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Raw: resp,
	}

	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: json.RawMessage(tc.Function.Arguments),
				},
			}
		}
	}

	return result, nil
}

// openaiStream implements Stream for OpenAI
type openaiStream struct {
	reader *bufio.Reader
	closer io.Closer
}

func (s *openaiStream) Recv() (*StreamChunk, error) {
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
		if string(data) == "[DONE]" {
			return &StreamChunk{FinishReason: "stop"}, io.EOF
		}

		var event struct {
			Choices []struct {
				Delta struct {
					Content   string           `json:"content"`
					ToolCalls []openaiToolCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}

		if len(event.Choices) == 0 {
			continue
		}

		choice := event.Choices[0]
		chunk := &StreamChunk{
			Delta:        choice.Delta.Content,
			FinishReason: choice.FinishReason,
		}

		if len(choice.Delta.ToolCalls) > 0 {
			chunk.ToolCallDeltas = make([]ToolCallDelta, len(choice.Delta.ToolCalls))
			for i, tc := range choice.Delta.ToolCalls {
				chunk.ToolCallDeltas[i] = ToolCallDelta{
					Index:         i,
					ID:            tc.ID,
					Type:          tc.Type,
					FunctionName:  tc.Function.Name,
					ArgumentDelta: tc.Function.Arguments,
				}
			}
		}

		return chunk, nil
	}
}

func (s *openaiStream) Close() error {
	return s.closer.Close()
}

// openaiModelsResponse represents the response from GET /v1/models
type openaiModelsResponse struct {
	Data []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// openaiModelPricing contains known pricing for OpenAI models (per 1M tokens)
var openaiModelPricing = map[string]struct {
	input       float64
	output      float64
	description string
}{
	"gpt-4o":           {2.50, 10.00, "Latest GPT-4 with vision"},
	"gpt-4o-mini":      {0.15, 0.60, "Smaller, faster GPT-4o"},
	"gpt-4-turbo":      {10.00, 30.00, "GPT-4 optimized for speed"},
	"gpt-4":            {30.00, 60.00, "Original GPT-4"},
	"gpt-3.5-turbo":    {0.50, 1.50, "Fast and cost-effective"},
	"o1":               {15.00, 60.00, "Reasoning model for complex problems"},
	"o1-mini":          {3.00, 12.00, "Faster reasoning model"},
	"o1-preview":       {15.00, 60.00, "Preview reasoning model"},
	"gpt-4.5-turbo":    {75.00, 150.00, "Enhanced GPT-4"},
	"gpt-4.1":          {2.00, 8.00, "GPT-4.1 base model"},
	"gpt-4.1-mini":     {0.40, 1.60, "Smaller GPT-4.1"},
	"gpt-4.1-nano":     {0.10, 0.40, "Fastest GPT-4.1"},
	"gpt-5":            {5.00, 20.00, "GPT-5 model"},
	"gpt-5-mini":       {1.00, 4.00, "Smaller GPT-5"},
	"gpt-5.1":          {5.00, 20.00, "GPT-5.1 improved"},
	"gpt-5.1-codex":    {6.00, 24.00, "GPT-5.1 for coding"},
	"gpt-5.2":          {5.00, 20.00, "Latest GPT-5 series"},
}

// ListModels fetches available models from OpenAI API
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, NewProviderError("openai", ErrorCodeTimeout, err.Error(), err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp)
	}

	var modelsResp openaiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, NewProviderError("openai", ErrorCodeUnknown, "failed to decode models response", err)
	}

	var models []ModelInfo
	for _, m := range modelsResp.Data {
		// Filter to only chat models
		if !isOpenAIChatModel(m.ID) {
			continue
		}

		info := ModelInfo{
			ID:        m.ID,
			Name:      m.ID,
			Provider:  "openai",
			CreatedAt: m.Created,
		}

		// Add pricing and description if known
		if pricing, ok := openaiModelPricing[m.ID]; ok {
			info.InputCost = pricing.input
			info.OutputCost = pricing.output
			info.Description = pricing.description
		} else {
			info.Description = "OpenAI model"
		}

		models = append(models, info)
	}

	return models, nil
}

// isOpenAIChatModel checks if a model ID is a chat-capable model
func isOpenAIChatModel(id string) bool {
	chatPrefixes := []string{"gpt-", "o1", "chatgpt-"}
	for _, prefix := range chatPrefixes {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
