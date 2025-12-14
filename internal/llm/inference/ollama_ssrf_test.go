package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test SSRF Prevention - Private IP Blocking
func TestOllamaService_SSRFPreventionPrivateIPs(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "allowed localhost",
			baseURL:   "http://localhost:11434",
			shouldErr: false,
		},
		{
			name:      "allowed 127.0.0.1",
			baseURL:   "http://127.0.0.1:11434",
			shouldErr: false,
		},
		{
			name:      "allowed IPv6 loopback",
			baseURL:   "http://[::1]:11434",
			shouldErr: false,
		},
		{
			name:      "blocked private IP 10.0.0.1",
			baseURL:   "http://10.0.0.1:11434",
			shouldErr: true,
			errMsg:    "not in allowlist",
		},
		{
			name:      "blocked private IP 192.168.1.1",
			baseURL:   "http://192.168.1.1:11434",
			shouldErr: true,
			errMsg:    "not in allowlist",
		},
		{
			name:      "blocked private IP 172.16.0.1",
			baseURL:   "http://172.16.0.1:11434",
			shouldErr: true,
			errMsg:    "not in allowlist",
		},
		{
			name:      "blocked external IP",
			baseURL:   "http://8.8.8.8:11434",
			shouldErr: true,
			errMsg:    "not in allowlist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOllamaService(tt.baseURL)

			if tt.shouldErr {
				if err == nil {
					t.Error("expected error for blocked URL")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for allowed URL: %v", err)
				}
			}
		})
	}
}

// Test SSRF Prevention - Metadata Service Blocking
func TestOllamaService_SSRFPreventionMetadataService(t *testing.T) {
	metadataURLs := []string{
		"http://169.254.169.254/latest/meta-data/", // AWS
		"http://metadata.google.internal/",         // GCP
		"http://169.254.169.254/metadata/v1/",      // DigitalOcean
		"http://169.254.169.254/metadata/instance", // Azure (old)
		"http://169.254.169.254/",                  // Generic metadata
	}

	for _, metadataURL := range metadataURLs {
		t.Run(metadataURL, func(t *testing.T) {
			_, err := NewOllamaService(metadataURL)

			if err == nil {
				t.Error("expected error for metadata service URL")
			}

			if !strings.Contains(err.Error(), "not in allowlist") &&
				!strings.Contains(err.Error(), "metadata service") {
				t.Errorf("error = %v, want 'not in allowlist' or 'metadata service'", err)
			}
		})
	}
}

// Test SSRF Prevention - Invalid Schemes
func TestOllamaService_SSRFPreventionInvalidSchemes(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
	}{
		{
			name:    "file scheme",
			baseURL: "file:///etc/passwd",
		},
		{
			name:    "ftp scheme",
			baseURL: "ftp://example.com/",
		},
		{
			name:    "gopher scheme",
			baseURL: "gopher://example.com/",
		},
		{
			name:    "data scheme",
			baseURL: "data:text/plain,hello",
		},
		{
			name:    "javascript scheme",
			baseURL: "javascript:alert(1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOllamaService(tt.baseURL)

			if err == nil {
				t.Error("expected error for invalid scheme")
			}

			if !strings.Contains(err.Error(), "invalid URL scheme") {
				t.Errorf("error = %v, want 'invalid URL scheme'", err)
			}
		})
	}
}

// Test SSRF Prevention - Redirect Following Disabled
func TestOllamaService_SSRFPreventionNoRedirects(t *testing.T) {
	// Create a test server that redirects to metadata service
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirectServer.Close()

	// This should succeed (127.0.0.1 is allowed)
	client, err := NewOllamaService("http://localhost:11434")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Check that redirects are disabled via CheckRedirect function
	if client.httpClient.CheckRedirect == nil {
		t.Error("CheckRedirect should be configured to prevent redirects")
	}

	// Test that CheckRedirect returns an error
	testReq, _ := http.NewRequest("GET", "http://localhost:11434", nil)
	err = client.httpClient.CheckRedirect(testReq, nil)
	if err != http.ErrUseLastResponse {
		t.Errorf("CheckRedirect should return http.ErrUseLastResponse, got %v", err)
	}
}

// Test SSRF Prevention - Localhost Validation
func TestOllamaService_LocalhostValidation(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		shouldErr bool
	}{
		{
			name:      "localhost",
			baseURL:   "http://localhost:11434",
			shouldErr: false,
		},
		{
			name:      "127.0.0.1",
			baseURL:   "http://127.0.0.1:11434",
			shouldErr: false,
		},
		{
			name:      "::1 IPv6",
			baseURL:   "http://[::1]:11434",
			shouldErr: false,
		},
		{
			name:      "ollama docker service",
			baseURL:   "http://ollama:11434",
			shouldErr: false,
		},
		{
			name:      "localhost with different port",
			baseURL:   "http://localhost:8080",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOllamaService(tt.baseURL)

			if tt.shouldErr && err == nil {
				t.Error("expected error")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test Empty Base URL
func TestOllamaService_EmptyBaseURL(t *testing.T) {
	client, err := NewOllamaService("")
	if err != nil {
		t.Fatalf("failed to create client with empty URL: %v", err)
	}

	// Should default to localhost
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %s, want http://localhost:11434", client.baseURL)
	}
}

// Test Invalid URL Format
func TestOllamaService_InvalidURLFormat(t *testing.T) {
	invalidURLs := []string{
		"not a url",
		"http://[invalid",
		"://missing-scheme",
		"http://",
	}

	for _, invalidURL := range invalidURLs {
		t.Run(invalidURL, func(t *testing.T) {
			_, err := NewOllamaService(invalidURL)
			if err == nil {
				t.Error("expected error for invalid URL format")
			}
		})
	}
}

// Test Connection Blocking
func TestOllamaService_ConnectionBlocking(t *testing.T) {
	client, err := NewOllamaService("http://localhost:11434")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Get the transport
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	// Test that the dialer validates addresses
	ctx := context.Background()

	// This should pass validation (localhost is allowed)
	// Connection may fail if no Ollama server is running, which is expected
	conn, err := transport.DialContext(ctx, "tcp", "localhost:11434")
	if err != nil {
		// Connection refused is expected when no server is running
		// Validation errors would contain "connection blocked" or "not in allowlist"
		if !strings.Contains(err.Error(), "connection refused") &&
			!strings.Contains(err.Error(), "connect: connection refused") {
			t.Errorf("localhost connection should pass validation (got: %v)", err)
		}
	} else if conn != nil {
		_ = conn.Close()
	}

	// These should be blocked by validation
	blockedAddresses := []string{
		"169.254.169.254:80",
		"10.0.0.1:80",
		"192.168.1.1:80",
		"172.16.0.1:80",
	}

	for _, addr := range blockedAddresses {
		_, err = transport.DialContext(ctx, "tcp", addr)
		if err == nil {
			t.Errorf("connection to %s should be blocked", addr)
		}
		if !strings.Contains(err.Error(), "connection blocked") && !strings.Contains(err.Error(), "not in allowlist") {
			t.Errorf("unexpected error for %s: %v", addr, err)
		}
	}
}

// Test Case Sensitivity
func TestOllamaService_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		shouldErr bool
	}{
		{
			name:      "localhost lowercase",
			baseURL:   "http://localhost:11434",
			shouldErr: false,
		},
		{
			name:      "LOCALHOST uppercase",
			baseURL:   "http://LOCALHOST:11434",
			shouldErr: false,
		},
		{
			name:      "Localhost mixed case",
			baseURL:   "http://Localhost:11434",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOllamaService(tt.baseURL)

			if tt.shouldErr && err == nil {
				t.Error("expected error")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test URL Parsing Edge Cases
func TestOllamaService_URLParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		shouldErr bool
	}{
		{
			name:      "URL with path",
			baseURL:   "http://localhost:11434/v1",
			shouldErr: false,
		},
		{
			name:      "URL with query",
			baseURL:   "http://localhost:11434?param=value",
			shouldErr: false,
		},
		{
			name:      "URL with fragment",
			baseURL:   "http://localhost:11434#section",
			shouldErr: false,
		},
		{
			name:      "HTTPS scheme",
			baseURL:   "https://localhost:11434",
			shouldErr: false,
		},
		{
			name:      "URL with username (should work but unusual)",
			baseURL:   "http://user@localhost:11434",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOllamaService(tt.baseURL)

			if tt.shouldErr && err == nil {
				t.Error("expected error")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test Integration with httptest server
func TestOllamaService_SSRFProtectionWithHTTPTest(t *testing.T) {
	// Create a mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response":          "test response",
				"done":              true,
				"prompt_eval_count": 5,
				"eval_count":        3,
			})
		}
	}))
	defer server.Close()

	// httptest.NewServer creates a server on 127.0.0.1, which is in the allowlist
	svc, err := NewOllamaService(server.URL)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Test that normal operations work with SSRF protection enabled
	resp, err := svc.Generate(context.Background(), GenerateRequest{
		Model:  "test-model",
		Prompt: "test prompt",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "test response" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
}

// Benchmark tests
func BenchmarkNewOllamaService(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewOllamaService("http://localhost:11434")
	}
}

func BenchmarkOllamaService_SSRFValidation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewOllamaService("http://127.0.0.1:11434")
	}
}
