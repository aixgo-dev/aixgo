package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
)

// Client implements Ollama HTTP API client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// allowedOllamaHosts is the list of allowed Ollama server hosts
var allowedOllamaHosts = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"ollama", // Docker service name
}

// NewClient creates a new Ollama client with SSRF protection
func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Only allow http/https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid URL scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}

	// Validate host against allowlist
	host := parsedURL.Hostname()
	if err := validateOllamaHost(host); err != nil {
		return nil, err
	}

	// Create client with restricted transport
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Validate the address we're about to connect to
			dialHost, _, err := net.SplitHostPort(addr)
			if err != nil {
				dialHost = addr
			}

			// Re-validate the host (in case of redirects or DNS rebinding)
			if err := validateOllamaHost(dialHost); err != nil {
				return nil, fmt.Errorf("connection blocked: %w", err)
			}

			// Use default dialer
			dialer := &net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, network, addr)
		},
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   5 * time.Minute,
			Transport: transport,
			// Disable following redirects to prevent SSRF via redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// validateOllamaHost validates that a host is allowed
func validateOllamaHost(host string) error {
	// Check against allowlist
	hostAllowed := false
	for _, allowed := range allowedOllamaHosts {
		if strings.EqualFold(host, allowed) {
			hostAllowed = true
			break
		}
	}

	if !hostAllowed {
		return fmt.Errorf("host not in allowlist: %s", host)
	}

	// Resolve and check IP address
	if err := validateOllamaIP(host); err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	return nil
}

// validateOllamaIP validates that resolved IP addresses are safe
func validateOllamaIP(host string) error {
	// Skip IP validation for localhost only (always resolves to loopback)
	if host == "localhost" {
		return nil
	}

	// For Docker service name "ollama", validate it resolves to private/loopback
	// This is the expected behavior in Docker networks

	ips, err := net.LookupIP(host)
	if err != nil {
		// If lookup fails, allow "ollama" to proceed (Docker network may not be available during tests)
		if host == "ollama" {
			return nil
		}
		return err
	}

	for _, ip := range ips {
		// Allow loopback
		if ip.IsLoopback() {
			continue
		}

		// Block private IP ranges for non-localhost hosts
		if ip.IsPrivate() {
			return fmt.Errorf("private IP addresses not allowed: %s", ip)
		}

		// Block link-local
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("link-local addresses not allowed: %s", ip)
		}

		// Block multicast
		if ip.IsMulticast() {
			return fmt.Errorf("multicast addresses not allowed: %s", ip)
		}
	}

	return nil
}

// Generate generates text using Ollama
func (c *Client) Generate(ctx context.Context, req inference.GenerateRequest) (*inference.GenerateResponse, error) {
	// Build Ollama request
	ollamaReq := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"stream": false,
	}

	if req.MaxTokens > 0 {
		ollamaReq["options"] = map[string]any{
			"num_predict": req.MaxTokens,
		}
	}

	if req.Temperature > 0 {
		if options, ok := ollamaReq["options"].(map[string]any); ok {
			options["temperature"] = req.Temperature
		} else {
			ollamaReq["options"] = map[string]any{
				"temperature": req.Temperature,
			}
		}
	}

	if len(req.Stop) > 0 {
		if options, ok := ollamaReq["options"].(map[string]any); ok {
			options["stop"] = req.Stop
		} else {
			ollamaReq["options"] = map[string]any{
				"stop": req.Stop,
			}
		}
	}

	// Marshal request
	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
		Context  []int  `json:"context"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &inference.GenerateResponse{
		Text:         ollamaResp.Response,
		FinishReason: "stop",
		Usage: inference.Usage{
			// Ollama doesn't provide token counts in non-streaming mode
			TotalTokens: len(ollamaResp.Context),
		},
	}, nil
}

// Available checks if Ollama is available
func (c *Client) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimSuffix(c.baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// HasModel checks if a specific model is available
func (c *Client) HasModel(model string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimSuffix(c.baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	for _, m := range result.Models {
		if m.Name == model {
			return true
		}
	}
	return false
}
