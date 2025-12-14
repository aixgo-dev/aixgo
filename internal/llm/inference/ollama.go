package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/security"
)

// OllamaService implements InferenceService for Ollama
type OllamaService struct {
	baseURL    string
	httpClient *http.Client
	validator  *security.SSRFValidator
}

// NewOllamaService creates a new Ollama inference service with SSRF protection
func NewOllamaService(baseURL string) (*OllamaService, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Create SSRF validator
	validator := security.NewOllamaSSRFValidator()

	// Validate URL
	if err := validator.ValidateURL(baseURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	// Create secure HTTP client with SSRF-protected transport
	httpClient := &http.Client{
		Timeout:   5 * time.Minute,
		Transport: validator.CreateSecureTransport(),
		// Disable following redirects to prevent SSRF via redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &OllamaService{
		baseURL:    parsedURL.String(),
		httpClient: httpClient,
		validator:  validator,
	}, nil
}

// Generate performs inference using Ollama
func (o *OllamaService) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	ollamaReq := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"stream": false,
	}

	options := make(map[string]any)
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		options["temperature"] = req.Temperature
	}
	if len(req.Stop) > 0 {
		options["stop"] = req.Stop
	}
	if len(options) > 0 {
		ollamaReq["options"] = options
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Response        string `json:"response"`
		Done            bool   `json:"done"`
		Context         []int  `json:"context"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &GenerateResponse{
		Text:         ollamaResp.Response,
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// Chat performs chat completion using Ollama
func (o *OllamaService) Chat(ctx context.Context, model string, messages []ChatMessage) (*GenerateResponse, error) {
	ollamaReq := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama chat error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Done            bool `json:"done"`
		PromptEvalCount int  `json:"prompt_eval_count"`
		EvalCount       int  `json:"eval_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &GenerateResponse{
		Text:         chatResp.Message.Content,
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     chatResp.PromptEvalCount,
			CompletionTokens: chatResp.EvalCount,
			TotalTokens:      chatResp.PromptEvalCount + chatResp.EvalCount,
		},
	}, nil
}

// ListModels returns available models
func (o *OllamaService) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, len(result.Models))
	for i, m := range result.Models {
		models[i] = ModelInfo{
			Name: m.Name,
			Size: m.Size,
		}
	}
	return models, nil
}

// Available checks if Ollama is available
func (o *OllamaService) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use root endpoint for lightweight health check
	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/", nil)
	if err != nil {
		return false
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ModelInfo represents model information
type ModelInfo struct {
	Name string
	Size int64
}
