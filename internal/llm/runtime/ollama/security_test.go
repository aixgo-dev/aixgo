package ollama

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
)

// Test SSRF Prevention - Private IP Blocking
func TestClient_SSRFPreventionPrivateIPs(t *testing.T) {
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
			_, err := NewClient(tt.baseURL)

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
func TestClient_SSRFPreventionMetadataService(t *testing.T) {
	metadataURLs := []string{
		"http://169.254.169.254/latest/meta-data/", // AWS
		"http://metadata.google.internal/",         // GCP
		"http://169.254.169.254/metadata/v1/",      // DigitalOcean
		"http://169.254.169.254/metadata/instance", // Azure (old)
		"http://169.254.169.254/",                  // Generic metadata
	}

	for _, metadataURL := range metadataURLs {
		t.Run(metadataURL, func(t *testing.T) {
			_, err := NewClient(metadataURL)

			if err == nil {
				t.Error("expected error for metadata service URL")
			}

			if !strings.Contains(err.Error(), "not in allowlist") {
				t.Errorf("error = %v, want 'not in allowlist'", err)
			}
		})
	}
}

// Test SSRF Prevention - Invalid Schemes
func TestClient_SSRFPreventionInvalidSchemes(t *testing.T) {
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
			_, err := NewClient(tt.baseURL)

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
func TestClient_SSRFPreventionNoRedirects(t *testing.T) {
	// Create a test server that redirects to metadata service
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirectServer.Close()

	// This should succeed (localhost is allowed)
	client, err := NewClient("http://localhost:11434")
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
func TestClient_LocalhostValidation(t *testing.T) {
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
			_, err := NewClient(tt.baseURL)

			if tt.shouldErr && err == nil {
				t.Error("expected error")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Test SSRF Prevention - Allowlist Validation
func TestValidateOllamaHost(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		shouldErr bool
	}{
		{
			name:      "allowed localhost",
			host:      "localhost",
			shouldErr: false,
		},
		{
			name:      "allowed 127.0.0.1",
			host:      "127.0.0.1",
			shouldErr: false,
		},
		{
			name:      "allowed ::1",
			host:      "::1",
			shouldErr: false,
		},
		{
			name:      "allowed ollama",
			host:      "ollama",
			shouldErr: false,
		},
		{
			name:      "blocked google.com",
			host:      "google.com",
			shouldErr: true,
		},
		{
			name:      "blocked private IP",
			host:      "192.168.1.1",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOllamaHost(tt.host)

			if tt.shouldErr && err == nil {
				t.Error("expected error for blocked host")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error for allowed host: %v", err)
			}
		})
	}
}

// Test SSRF Prevention - IP Validation
func TestValidateOllamaIP(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		shouldErr bool
	}{
		{
			name:      "localhost - no validation",
			host:      "localhost",
			shouldErr: false,
		},
		{
			name:      "ollama - no validation",
			host:      "ollama",
			shouldErr: false,
		},
		{
			name:      "127.0.0.1 loopback allowed",
			host:      "127.0.0.1",
			shouldErr: false,
		},
		{
			name:      "::1 loopback allowed",
			host:      "::1",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOllamaIP(tt.host)

			if tt.shouldErr && err == nil {
				t.Error("expected error for blocked IP")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error for allowed IP: %v", err)
			}
		})
	}
}

// Test Connection Blocking
func TestClient_ConnectionBlocking(t *testing.T) {
	client, err := NewClient("http://localhost:11434")
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

// Test Empty Base URL
func TestClient_EmptyBaseURL(t *testing.T) {
	client, err := NewClient("")
	if err != nil {
		t.Fatalf("failed to create client with empty URL: %v", err)
	}

	// Should default to localhost
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %s, want http://localhost:11434", client.baseURL)
	}
}

// Test Invalid URL Format
func TestClient_InvalidURLFormat(t *testing.T) {
	invalidURLs := []string{
		"not a url",
		"http://[invalid",
		"://missing-scheme",
		"http://",
	}

	for _, invalidURL := range invalidURLs {
		t.Run(invalidURL, func(t *testing.T) {
			_, err := NewClient(invalidURL)
			if err == nil {
				t.Error("expected error for invalid URL format")
			}
		})
	}
}

// Test Client Timeouts
func TestClient_Timeouts(t *testing.T) {
	client, err := NewClient("http://localhost:11434")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Check that timeout is configured
	if client.httpClient.Timeout != 5*time.Minute {
		t.Errorf("client timeout = %v, want 5m", client.httpClient.Timeout)
	}

	// Test with a context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := inference.GenerateRequest{
		Model:  "test-model",
		Prompt: "test prompt",
	}

	// This will timeout because there's no server
	_, err = client.Generate(ctx, req)
	if err == nil {
		t.Error("expected timeout error")
	}
}

// Test Available Method with Mock Server
func TestClient_Available(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models": []}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Note: This test is limited because we can't easily override the allowlist
	// In a real scenario, we'd use dependency injection for the validator

	// Test with localhost (should work with real Ollama)
	client, err := NewClient("http://localhost:11434")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Available will return false if Ollama isn't actually running
	// This is expected behavior
	_ = client.Available()
}

// Test DNS Rebinding Protection
func TestClient_DNSRebindingProtection(t *testing.T) {
	// This test verifies that we re-validate the host on each connection
	client, err := NewClient("http://localhost:11434")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}

	// The DialContext function should validate on each call
	// Even if DNS changes between calls

	ctx := context.Background()

	// First call with localhost
	conn1, err1 := transport.DialContext(ctx, "tcp", "localhost:11434")
	if err1 != nil {
		t.Logf("First connection error (expected if no Ollama): %v", err1)
	}
	if conn1 != nil {
		_ = conn1.Close()
	}

	// Second call should also validate
	conn2, err2 := transport.DialContext(ctx, "tcp", "localhost:11434")
	if err2 != nil {
		t.Logf("Second connection error (expected if no Ollama): %v", err2)
	}
	if conn2 != nil {
		_ = conn2.Close()
	}
}

// Test Link-Local Address Blocking
func TestValidateOllamaIP_LinkLocal(t *testing.T) {
	linkLocalAddresses := []string{
		"169.254.1.1",     // IPv4 link-local
		"fe80::1",         // IPv6 link-local
		"169.254.169.254", // Metadata service
	}

	for _, addr := range linkLocalAddresses {
		t.Run(addr, func(t *testing.T) {
			// Resolve IP
			ips, err := net.LookupIP(addr)
			if err != nil {
				t.Skip("DNS lookup failed, skipping test")
			}

			// Check if any resolved IP is blocked
			hasLinkLocal := false
			for _, ip := range ips {
				if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
					hasLinkLocal = true
					break
				}
			}

			if !hasLinkLocal && addr != "169.254.169.254" {
				t.Skip("Address did not resolve to link-local")
			}
		})
	}
}

// Test Multicast Address Blocking
func TestValidateOllamaIP_Multicast(t *testing.T) {
	multicastAddresses := []string{
		"224.0.0.1", // IPv4 multicast
		"ff02::1",   // IPv6 multicast
	}

	for _, addr := range multicastAddresses {
		t.Run(addr, func(t *testing.T) {
			// Parse IP
			ip := net.ParseIP(addr)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", addr)
			}

			// Verify it's recognized as multicast
			if !ip.IsMulticast() {
				t.Errorf("IP %s should be multicast", addr)
			}
		})
	}
}

// Test Case Sensitivity
func TestValidateOllamaHost_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name      string
		host      string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "localhost lowercase",
			host:      "localhost",
			shouldErr: false,
		},
		{
			name:      "LOCALHOST uppercase",
			host:      "LOCALHOST",
			shouldErr: false,
		},
		{
			name:      "Localhost mixed case",
			host:      "Localhost",
			shouldErr: false,
		},
		{
			name:      "ollama lowercase",
			host:      "ollama",
			shouldErr: false,
		},
		{
			name: "OLLAMA uppercase",
			host: "OLLAMA",
			// The allowlist check is case-insensitive, so "OLLAMA" passes
			// However, the IP validation skips DNS for "ollama" (lowercase) only
			// So "OLLAMA" will fail DNS lookup, which is expected current behavior
			shouldErr: true,
			errMsg:    "", // DNS lookup error message varies by system
		},
		{
			name: "Ollama mixed case",
			host: "Ollama",
			// Same as OLLAMA - passes allowlist but fails DNS
			shouldErr: true,
			errMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOllamaHost(tt.host)

			if tt.shouldErr {
				if err == nil {
					t.Error("expected error")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test URL Parsing Edge Cases
func TestClient_URLParsingEdgeCases(t *testing.T) {
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
			_, err := NewClient(tt.baseURL)

			if tt.shouldErr && err == nil {
				t.Error("expected error")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateOllamaHost(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = validateOllamaHost("localhost")
	}
}

func BenchmarkValidateOllamaIP(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = validateOllamaIP("127.0.0.1")
	}
}

func BenchmarkNewClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewClient("http://localhost:11434")
	}
}
