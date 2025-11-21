package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VLLMService implements InferenceService for vLLM (OpenAI-compatible API)
type VLLMService struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewVLLMService creates a new vLLM inference service
func NewVLLMService(baseURL, apiKey string) *VLLMService {
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	return &VLLMService{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Generate performs inference using vLLM completions endpoint
func (v *VLLMService) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	vllmReq := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}

	if req.MaxTokens > 0 {
		vllmReq["max_tokens"] = req.MaxTokens
	}
	if req.Temperature >= 0 {
		vllmReq["temperature"] = req.Temperature
	}
	if len(req.Stop) > 0 {
		vllmReq["stop"] = req.Stop
	}

	reqBody, err := json.Marshal(vllmReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", v.baseURL+"/v1/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if v.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+v.apiKey)
	}

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("vllm error (status %d): %s", resp.StatusCode, string(body))
	}

	var vllmResp struct {
		Choices []struct {
			Text         string `json:"text"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vllmResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(vllmResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &GenerateResponse{
		Text:         vllmResp.Choices[0].Text,
		FinishReason: vllmResp.Choices[0].FinishReason,
		Usage: Usage{
			PromptTokens:     vllmResp.Usage.PromptTokens,
			CompletionTokens: vllmResp.Usage.CompletionTokens,
			TotalTokens:      vllmResp.Usage.TotalTokens,
		},
	}, nil
}

// BatchGenerate performs batch inference
func (v *VLLMService) BatchGenerate(ctx context.Context, reqs []GenerateRequest) ([]*GenerateResponse, error) {
	results := make([]*GenerateResponse, len(reqs))
	for i, req := range reqs {
		resp, err := v.Generate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("batch item %d: %w", i, err)
		}
		results[i] = resp
	}
	return results, nil
}

// ChatCompletion performs chat completion using vLLM
func (v *VLLMService) ChatCompletion(ctx context.Context, model string, messages []ChatMessage, maxTokens int) (*GenerateResponse, error) {
	vllmReq := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if maxTokens > 0 {
		vllmReq["max_tokens"] = maxTokens
	}

	reqBody, err := json.Marshal(vllmReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", v.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if v.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+v.apiKey)
	}

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("vllm chat error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &GenerateResponse{
		Text:         chatResp.Choices[0].Message.Content,
		FinishReason: chatResp.Choices[0].FinishReason,
		Usage: Usage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		},
	}, nil
}

// Available checks if vLLM is available
func (v *VLLMService) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", v.baseURL+"/v1/models", nil)
	if err != nil {
		return false
	}
	if v.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+v.apiKey)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// ListModels returns available models from vLLM
func (v *VLLMService) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", v.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if v.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+v.apiKey)
	}

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("vllm error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, len(result.Data))
	for i, m := range result.Data {
		models[i] = ModelInfo{Name: m.ID}
	}
	return models, nil
}
