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
	xaiBaseURL    = "https://api.x.ai/v1"
	xaiMaxRetries = 3
)

func init() {
	RegisterFactory("xai", func(config map[string]any) (Provider, error) {
		apiKey := ""
		if key, ok := config["api_key"].(string); ok {
			apiKey = key
		}
		if apiKey == "" {
			apiKey = os.Getenv("XAI_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("XAI_API_KEY not set")
		}

		model := "grok-beta"
		if m, ok := config["model"].(string); ok && m != "" {
			model = m
		}

		baseURL := xaiBaseURL
		if url, ok := config["base_url"].(string); ok && url != "" {
			baseURL = url
		}

		return NewXAIProvider(apiKey, model, baseURL), nil
	})
}

// XAIProvider implements Provider for X.AI (Grok) API
type XAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewXAIProvider creates a new X.AI provider
func NewXAIProvider(apiKey, model, baseURL string) *XAIProvider {
	if baseURL == "" {
		baseURL = xaiBaseURL
	}
	if model == "" {
		model = "grok-beta"
	}
	return &XAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns the provider name
func (p *XAIProvider) Name() string {
	return "xai"
}

// xaiRequest represents the X.AI API request format (OpenAI-compatible)
type xaiRequest struct {
	Model          string         `json:"model"`
	Messages       []xaiMessage   `json:"messages"`
	Temperature    float64        `json:"temperature,omitempty"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	Tools          []xaiTool      `json:"tools,omitempty"`
	Stream         bool           `json:"stream,omitempty"`
	ResponseFormat *xaiRespFmt    `json:"response_format,omitempty"`
}

type xaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []xaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type xaiTool struct {
	Type     string      `json:"type"`
	Function xaiFunction `json:"function"`
}

type xaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type xaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type xaiRespFmt struct {
	Type       string         `json:"type"`
	JSONSchema *xaiJSONSchema `json:"json_schema,omitempty"`
}

type xaiJSONSchema struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict,omitempty"`
	Schema json.RawMessage `json:"schema"`
}

type xaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index        int        `json:"index"`
		Message      xaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
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
func (p *XAIProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	xaiReq := p.buildRequest(req, model, false)

	var resp xaiResponse
	if err := p.doRequestWithRetry(ctx, "/chat/completions", xaiReq, &resp); err != nil {
		return nil, err
	}

	return p.parseResponse(&resp)
}

// CreateStructured creates a structured response
func (p *XAIProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	xaiReq := p.buildRequest(req.CompletionRequest, model, false)

	// Add response format for structured output
	if len(req.ResponseSchema) > 0 {
		xaiReq.ResponseFormat = &xaiRespFmt{
			Type: "json_schema",
			JSONSchema: &xaiJSONSchema{
				Name:   "response",
				Strict: req.StrictSchema,
				Schema: req.ResponseSchema,
			},
		}
	} else {
		xaiReq.ResponseFormat = &xaiRespFmt{Type: "json_object"}
	}

	var resp xaiResponse
	if err := p.doRequestWithRetry(ctx, "/chat/completions", xaiReq, &resp); err != nil {
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
func (p *XAIProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	xaiReq := p.buildRequest(req, model, true)

	body, err := json.Marshal(xaiReq)
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
		return nil, NewProviderError("xai", ErrorCodeTimeout, err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		return nil, p.handleErrorResponse(resp)
	}

	return &xaiStream{reader: bufio.NewReader(resp.Body), closer: resp.Body}, nil
}

func (p *XAIProvider) buildRequest(req CompletionRequest, model string, stream bool) xaiRequest {
	messages := make([]xaiMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = xaiMessage{Role: m.Role, Content: m.Content}
	}

	xReq := xaiRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      stream,
	}

	if len(req.Tools) > 0 {
		xReq.Tools = make([]xaiTool, len(req.Tools))
		for i, t := range req.Tools {
			xReq.Tools[i] = xaiTool{
				Type:     "function",
				Function: xaiFunction(t),
			}
		}
	}

	return xReq
}

func (p *XAIProvider) doRequestWithRetry(ctx context.Context, endpoint string, reqBody any, result any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < xaiMaxRetries; attempt++ {
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
			lastErr = NewProviderError("xai", ErrorCodeTimeout, err.Error(), err)
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

func (p *XAIProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp xaiResponse
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
			Provider:    "xai",
			Code:        code,
			Message:     errResp.Error.Message,
			Type:        errResp.Error.Type,
			StatusCode:  resp.StatusCode,
			IsRetryable: code == ErrorCodeRateLimit || code == ErrorCodeServerError,
		}
	}

	return NewProviderError("xai", ErrorCodeUnknown, string(body), nil)
}

func (p *XAIProvider) parseResponse(resp *xaiResponse) (*CompletionResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, NewProviderError("xai", ErrorCodeUnknown, "no choices in response", nil)
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

// xaiStream implements Stream for X.AI
type xaiStream struct {
	reader *bufio.Reader
	closer io.Closer
}

func (s *xaiStream) Recv() (*StreamChunk, error) {
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
					Content   string        `json:"content"`
					ToolCalls []xaiToolCall `json:"tool_calls"`
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

func (s *xaiStream) Close() error {
	return s.closer.Close()
}
