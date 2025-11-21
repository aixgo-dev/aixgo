package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// allowedHuggingFaceHosts contains the allowed hostnames for HuggingFace API requests
var allowedHuggingFaceHosts = map[string]bool{
	"api-inference.huggingface.co": true,
	"huggingface.co":               true,
}

// modelNamePattern restricts model names to prevent path traversal and injection
var modelNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+(/[a-zA-Z0-9_.-]+)?$`)

// validateModelName validates that a model name is safe
func validateModelName(model string) error {
	if model == "" {
		return nil // Empty model is allowed (uses default)
	}
	if len(model) > 256 {
		return fmt.Errorf("model name too long")
	}
	if !modelNamePattern.MatchString(model) {
		return fmt.Errorf("invalid model name format")
	}
	// Additional check for path traversal
	if strings.Contains(model, "..") {
		return fmt.Errorf("model name contains path traversal")
	}
	return nil
}

// validateEndpointURL validates that an endpoint URL is safe to use
func validateEndpointURL(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Must be HTTPS
	if parsed.Scheme != "https" {
		return fmt.Errorf("only HTTPS endpoints are allowed")
	}

	// Check against allowed hosts
	host := strings.ToLower(parsed.Host)
	if !allowedHuggingFaceHosts[host] {
		return fmt.Errorf("endpoint host not in allowlist: %s", host)
	}

	return nil
}

// HuggingFaceService implements InferenceService for HuggingFace Inference API
type HuggingFaceService struct {
	apiToken        string
	endpoint        string
	httpClient      *http.Client
	skipURLValidate bool // Only for testing - bypasses URL validation
}

// NewHuggingFaceService creates a new HuggingFace inference service
// Uses HF_TOKEN environment variable if token is empty
func NewHuggingFaceService(endpoint, token string) *HuggingFaceService {
	if token == "" {
		token = os.Getenv("HF_TOKEN")
	}
	if endpoint == "" {
		endpoint = "https://api-inference.huggingface.co/models"
	}
	return &HuggingFaceService{
		apiToken: token,
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

// Generate performs text generation using HuggingFace Inference API
func (h *HuggingFaceService) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if h.apiToken == "" {
		return nil, fmt.Errorf("HF_TOKEN not set")
	}

	// Validate model name to prevent SSRF
	if err := validateModelName(req.Model); err != nil {
		return nil, fmt.Errorf("invalid model name: %w", err)
	}

	hfReq := map[string]any{
		"inputs": req.Prompt,
		"parameters": map[string]any{
			"return_full_text": false,
		},
	}

	params := hfReq["parameters"].(map[string]any)
	if req.MaxTokens > 0 {
		params["max_new_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		params["temperature"] = req.Temperature
	}
	if len(req.Stop) > 0 {
		params["stop_sequences"] = req.Stop
	}

	reqBody, err := json.Marshal(hfReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := h.endpoint
	if req.Model != "" {
		url = h.endpoint + "/" + req.Model
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.apiToken)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("huggingface error (status %d): %s", resp.StatusCode, string(body))
	}

	// HF API returns an array of generated texts
	var hfResp []struct {
		GeneratedText string `json:"generated_text"`
	}

	if err := json.Unmarshal(body, &hfResp); err != nil {
		// Try single object response format
		var singleResp struct {
			GeneratedText string `json:"generated_text"`
		}
		if err := json.Unmarshal(body, &singleResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		return &GenerateResponse{
			Text:         singleResp.GeneratedText,
			FinishReason: "stop",
		}, nil
	}

	if len(hfResp) == 0 {
		return nil, fmt.Errorf("empty response from huggingface")
	}

	return &GenerateResponse{
		Text:         hfResp[0].GeneratedText,
		FinishReason: "stop",
	}, nil
}

// Available checks if HuggingFace API is available
func (h *HuggingFaceService) Available() bool {
	return h.apiToken != ""
}

// TextGenerationInference sends request to a dedicated TGI endpoint
func (h *HuggingFaceService) TextGenerationInference(ctx context.Context, endpoint string, req GenerateRequest) (*GenerateResponse, error) {
	if h.apiToken == "" {
		return nil, fmt.Errorf("HF_TOKEN not set")
	}

	// Validate endpoint URL to prevent SSRF (skip in test mode)
	if !h.skipURLValidate {
		if err := validateEndpointURL(endpoint); err != nil {
			return nil, fmt.Errorf("invalid endpoint: %w", err)
		}
	}

	tgiReq := map[string]any{
		"inputs": req.Prompt,
		"parameters": map[string]any{
			"return_full_text": false,
		},
	}

	params := tgiReq["parameters"].(map[string]any)
	if req.MaxTokens > 0 {
		params["max_new_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		params["temperature"] = req.Temperature
	}
	if len(req.Stop) > 0 {
		params["stop_sequences"] = req.Stop
	}

	reqBody, err := json.Marshal(tgiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.apiToken)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tgi error (status %d): %s", resp.StatusCode, string(body))
	}

	var tgiResp struct {
		GeneratedText string `json:"generated_text"`
	}

	if err := json.Unmarshal(body, &tgiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &GenerateResponse{
		Text:         tgiResp.GeneratedText,
		FinishReason: "stop",
	}, nil
}
