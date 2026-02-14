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
	geminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta"
	geminiMaxRetries = 3
)

func init() {
	RegisterFactory("gemini", func(config map[string]any) (Provider, error) {
		apiKey := ""
		if key, ok := config["api_key"].(string); ok {
			apiKey = key
		}
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY not set")
		}

		baseURL := geminiBaseURL
		if url, ok := config["base_url"].(string); ok && url != "" {
			baseURL = url
		}

		return NewGeminiProvider(apiKey, baseURL), nil
	})
}

// GeminiProvider implements Provider for Google Gemini API
type GeminiProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewGeminiProvider creates a new Gemini provider
func NewGeminiProvider(apiKey, baseURL string) *GeminiProvider {
	return &GeminiProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns the provider name
func (p *GeminiProvider) Name() string {
	return "gemini"
}

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	Tools             []geminiTool     `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string          `json:"text,omitempty"`
	FunctionCall *geminiFuncCall `json:"functionCall,omitempty"`
	FunctionResp *geminiFuncResp `json:"functionResponse,omitempty"`
}

type geminiFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFuncResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiGenConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
	ResponseSchema   any     `json:"responseSchema,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations,omitempty"`
}

type geminiFuncDecl struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content       geminiContent `json:"content"`
		FinishReason  string        `json:"finishReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// CreateCompletion creates a completion
func (p *GeminiProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	geminiReq := p.buildRequest(req)
	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", model, p.apiKey)

	var resp geminiResponse
	if err := p.doRequestWithRetry(ctx, endpoint, geminiReq, &resp); err != nil {
		return nil, err
	}

	return p.parseResponse(&resp)
}

// CreateStructured creates a structured response
func (p *GeminiProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	geminiReq := p.buildRequest(req.CompletionRequest)

	// Configure structured output
	if geminiReq.GenerationConfig == nil {
		geminiReq.GenerationConfig = &geminiGenConfig{}
	}
	geminiReq.GenerationConfig.ResponseMimeType = "application/json"

	if len(req.ResponseSchema) > 0 {
		var schema any
		if err := json.Unmarshal(req.ResponseSchema, &schema); err == nil {
			geminiReq.GenerationConfig.ResponseSchema = schema
		}
	}

	endpoint := fmt.Sprintf("/models/%s:generateContent?key=%s", model, p.apiKey)

	var resp geminiResponse
	if err := p.doRequestWithRetry(ctx, endpoint, geminiReq, &resp); err != nil {
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
func (p *GeminiProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	geminiReq := p.buildRequest(req)
	endpoint := fmt.Sprintf("/models/%s:streamGenerateContent?key=%s&alt=sse", model, p.apiKey)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, NewProviderError("gemini", ErrorCodeTimeout, err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		return nil, p.handleErrorResponse(resp)
	}

	return &geminiStream{reader: bufio.NewReader(resp.Body), closer: resp.Body}, nil
}

func (p *GeminiProvider) buildRequest(req CompletionRequest) geminiRequest {
	var systemContent *geminiContent
	contents := make([]geminiContent, 0, len(req.Messages))

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemContent = &geminiContent{
				Parts: []geminiPart{{Text: m.Content}},
			}
			continue
		}

		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	gReq := geminiRequest{
		Contents:          contents,
		SystemInstruction: systemContent,
	}

	if req.Temperature != 0 || req.MaxTokens != 0 {
		gReq.GenerationConfig = &geminiGenConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	if len(req.Tools) > 0 {
		funcDecls := make([]geminiFuncDecl, len(req.Tools))
		for i, t := range req.Tools {
			var params any
			if len(t.Parameters) > 0 {
				_ = json.Unmarshal(t.Parameters, &params)
			}
			funcDecls[i] = geminiFuncDecl{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			}
		}
		gReq.Tools = []geminiTool{{FunctionDeclarations: funcDecls}}
	}

	return gReq
}

func (p *GeminiProvider) doRequestWithRetry(ctx context.Context, endpoint string, reqBody any, result any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < geminiMaxRetries; attempt++ {
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

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = NewProviderError("gemini", ErrorCodeTimeout, err.Error(), err)
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

func (p *GeminiProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp geminiResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		code := ErrorCodeUnknown
		switch resp.StatusCode {
		case 401, 403:
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
			Provider:    "gemini",
			Code:        code,
			Message:     errResp.Error.Message,
			Type:        errResp.Error.Status,
			StatusCode:  resp.StatusCode,
			IsRetryable: code == ErrorCodeRateLimit || code == ErrorCodeServerError,
		}
	}

	return NewProviderError("gemini", ErrorCodeUnknown, string(body), nil)
}

func (p *GeminiProvider) parseResponse(resp *geminiResponse) (*CompletionResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, NewProviderError("gemini", ErrorCodeUnknown, "no candidates in response", nil)
	}

	candidate := resp.Candidates[0]
	var content string
	var toolCalls []ToolCall

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

	finishReason := candidate.FinishReason
	if finishReason == "STOP" {
		finishReason = "stop"
	}

	return &CompletionResponse{
		Content:      content,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		Usage: Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
		Raw: resp,
	}, nil
}

// geminiStream implements Stream for Gemini
type geminiStream struct {
	reader *bufio.Reader
	closer io.Closer
}

func (s *geminiStream) Recv() (*StreamChunk, error) {
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

		var resp geminiResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		candidate := resp.Candidates[0]
		var text string
		for _, part := range candidate.Content.Parts {
			text += part.Text
		}

		finishReason := ""
		if candidate.FinishReason == "STOP" {
			finishReason = "stop"
		}

		return &StreamChunk{
			Delta:        text,
			FinishReason: finishReason,
		}, nil
	}
}

func (s *geminiStream) Close() error {
	return s.closer.Close()
}
