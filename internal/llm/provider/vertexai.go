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

	"golang.org/x/oauth2/google"
)

const (
	vertexAIMaxRetries = 3
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

// VertexAIProvider implements Provider for Google Vertex AI API
type VertexAIProvider struct {
	projectID       string
	location        string
	client          *http.Client
	tokenFunc       func(context.Context) (string, error)
	baseURLOverride string // for testing
}

// NewVertexAIProvider creates a new Vertex AI provider using Application Default Credentials
func NewVertexAIProvider(projectID, location string) (*VertexAIProvider, error) {
	tokenFunc := func(ctx context.Context) (string, error) {
		creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return "", fmt.Errorf("failed to find default credentials: %w", err)
		}
		token, err := creds.TokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("failed to get token: %w", err)
		}
		return token.AccessToken, nil
	}

	return &VertexAIProvider{
		projectID: projectID,
		location:  location,
		client:    &http.Client{Timeout: 120 * time.Second},
		tokenFunc: tokenFunc,
	}, nil
}

// Name returns the provider name
func (p *VertexAIProvider) Name() string {
	return "vertexai"
}

func (p *VertexAIProvider) endpoint(model string) string {
	if p.baseURLOverride != "" {
		return p.baseURLOverride
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.location, p.projectID, p.location, model)
}

func (p *VertexAIProvider) streamEndpoint(model string) string {
	if p.baseURLOverride != "" {
		return p.baseURLOverride
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		p.location, p.projectID, p.location, model)
}

// CreateCompletion creates a completion
func (p *VertexAIProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	geminiReq := p.buildRequest(req)

	var resp geminiResponse
	if err := p.doRequestWithRetry(ctx, p.endpoint(model), geminiReq, &resp); err != nil {
		return nil, err
	}

	return p.parseResponse(&resp)
}

// CreateStructured creates a structured response
func (p *VertexAIProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	geminiReq := p.buildRequest(req.CompletionRequest)

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

	var resp geminiResponse
	if err := p.doRequestWithRetry(ctx, p.endpoint(model), geminiReq, &resp); err != nil {
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
func (p *VertexAIProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	geminiReq := p.buildRequest(req)

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, err
	}

	token, err := p.tokenFunc(ctx)
	if err != nil {
		return nil, NewProviderError("vertexai", ErrorCodeAuthentication, err.Error(), err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.streamEndpoint(model), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, NewProviderError("vertexai", ErrorCodeTimeout, err.Error(), err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			_ = resp.Body.Close()
		}()
		return nil, p.handleErrorResponse(resp)
	}

	return &vertexAIStream{reader: bufio.NewReader(resp.Body), closer: resp.Body}, nil
}

func (p *VertexAIProvider) buildRequest(req CompletionRequest) geminiRequest {
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

func (p *VertexAIProvider) doRequestWithRetry(ctx context.Context, endpoint string, reqBody any, result any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < vertexAIMaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		token, err := p.tokenFunc(ctx)
		if err != nil {
			return NewProviderError("vertexai", ErrorCodeAuthentication, err.Error(), err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = NewProviderError("vertexai", ErrorCodeTimeout, err.Error(), err)
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

func (p *VertexAIProvider) handleErrorResponse(resp *http.Response) error {
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
			Provider:    "vertexai",
			Code:        code,
			Message:     errResp.Error.Message,
			Type:        errResp.Error.Status,
			StatusCode:  resp.StatusCode,
			IsRetryable: code == ErrorCodeRateLimit || code == ErrorCodeServerError,
		}
	}

	return NewProviderError("vertexai", ErrorCodeUnknown, string(body), nil)
}

func (p *VertexAIProvider) parseResponse(resp *geminiResponse) (*CompletionResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, NewProviderError("vertexai", ErrorCodeUnknown, "no candidates in response", nil)
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

// vertexAIStream implements Stream for Vertex AI
type vertexAIStream struct {
	reader *bufio.Reader
	closer io.Closer
}

func (s *vertexAIStream) Recv() (*StreamChunk, error) {
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

func (s *vertexAIStream) Close() error {
	return s.closer.Close()
}
