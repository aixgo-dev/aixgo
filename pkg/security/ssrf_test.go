package security

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
)

func TestSSRFValidator_ValidateURL(t *testing.T) {
	tests := []struct {
		name      string
		config    SSRFConfig
		url       string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid localhost URL",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowedSchemes: []string{"http", "https"},
				AllowLocalhost: true,
			},
			url:       "http://localhost:11434",
			shouldErr: false,
		},
		{
			name: "valid 127.0.0.1 URL",
			config: SSRFConfig{
				AllowedHosts:   []string{"127.0.0.1"},
				AllowedSchemes: []string{"http", "https"},
				AllowLocalhost: true,
			},
			url:       "http://127.0.0.1:11434",
			shouldErr: false,
		},
		{
			name: "valid IPv6 loopback",
			config: SSRFConfig{
				AllowedHosts:   []string{"::1"},
				AllowedSchemes: []string{"http", "https"},
				AllowLocalhost: true,
			},
			url:       "http://[::1]:11434",
			shouldErr: false,
		},
		{
			name: "invalid private IP 10.0.0.1",
			config: SSRFConfig{
				AllowedHosts:    []string{"10.0.0.1"},
				AllowedSchemes:  []string{"http", "https"},
				BlockPrivateIPs: true,
			},
			url:       "http://10.0.0.1:11434",
			shouldErr: true,
			errMsg:    "private IP",
		},
		{
			name: "invalid private IP 192.168.1.1",
			config: SSRFConfig{
				AllowedHosts:    []string{"192.168.1.1"},
				AllowedSchemes:  []string{"http", "https"},
				BlockPrivateIPs: true,
			},
			url:       "http://192.168.1.1:11434",
			shouldErr: true,
			errMsg:    "private IP",
		},
		{
			name: "invalid private IP 172.16.0.1",
			config: SSRFConfig{
				AllowedHosts:    []string{"172.16.0.1"},
				AllowedSchemes:  []string{"http", "https"},
				BlockPrivateIPs: true,
			},
			url:       "http://172.16.0.1:11434",
			shouldErr: true,
			errMsg:    "private IP",
		},
		{
			name: "invalid metadata service URL",
			config: SSRFConfig{
				AllowedHosts:   []string{"169.254.169.254"},
				AllowedSchemes: []string{"http", "https"},
				BlockMetadata:  true,
			},
			url:       "http://169.254.169.254/latest/meta-data/",
			shouldErr: true,
			errMsg:    "metadata service",
		},
		{
			name: "host not in allowlist",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowedSchemes: []string{"http", "https"},
			},
			url:       "http://evil.com",
			shouldErr: true,
			errMsg:    "not in allowlist",
		},
		{
			name: "invalid scheme file://",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowedSchemes: []string{"http", "https"},
			},
			url:       "file:///etc/passwd",
			shouldErr: true,
			errMsg:    "invalid URL scheme",
		},
		{
			name: "invalid scheme ftp://",
			config: SSRFConfig{
				AllowedHosts:   []string{"example.com"},
				AllowedSchemes: []string{"http", "https"},
			},
			url:       "ftp://example.com/",
			shouldErr: true,
			errMsg:    "invalid URL scheme",
		},
		{
			name: "invalid scheme gopher://",
			config: SSRFConfig{
				AllowedHosts:   []string{"example.com"},
				AllowedSchemes: []string{"http", "https"},
			},
			url:       "gopher://example.com/",
			shouldErr: true,
			errMsg:    "invalid URL scheme",
		},
		{
			name: "URL with port",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowedSchemes: []string{"http", "https"},
				AllowLocalhost: true,
			},
			url:       "http://localhost:8080",
			shouldErr: false,
		},
		{
			name: "case insensitive hostname",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowedSchemes: []string{"http", "https"},
				AllowLocalhost: true,
			},
			url:       "http://LOCALHOST:11434",
			shouldErr: false,
		},
		{
			name: "case insensitive scheme",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowedSchemes: []string{"http", "https"},
				AllowLocalhost: true,
			},
			url:       "HTTP://localhost:11434",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSSRFValidator(tt.config)
			err := validator.ValidateURL(tt.url)

			if tt.shouldErr {
				if err == nil {
					t.Error("expected error but got nil")
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

func TestSSRFValidator_ValidateHost(t *testing.T) {
	tests := []struct {
		name      string
		config    SSRFConfig
		host      string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "allowed localhost",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowLocalhost: true,
			},
			host:      "localhost",
			shouldErr: false,
		},
		{
			name: "allowed 127.0.0.1",
			config: SSRFConfig{
				AllowedHosts:   []string{"127.0.0.1"},
				AllowLocalhost: true,
			},
			host:      "127.0.0.1",
			shouldErr: false,
		},
		{
			name: "allowed ::1",
			config: SSRFConfig{
				AllowedHosts:   []string{"::1"},
				AllowLocalhost: true,
			},
			host:      "::1",
			shouldErr: false,
		},
		{
			name: "allowed ollama",
			config: SSRFConfig{
				AllowedHosts: []string{"ollama"},
			},
			host:      "ollama",
			shouldErr: false,
		},
		{
			name: "host not in allowlist",
			config: SSRFConfig{
				AllowedHosts: []string{"localhost"},
			},
			host:      "google.com",
			shouldErr: true,
			errMsg:    "not in allowlist",
		},
		{
			name: "case insensitive allowlist",
			config: SSRFConfig{
				AllowedHosts:   []string{"localhost"},
				AllowLocalhost: true,
			},
			host:      "LOCALHOST",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSSRFValidator(tt.config)
			err := validator.ValidateHost(tt.host)

			if tt.shouldErr {
				if err == nil {
					t.Error("expected error but got nil")
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

func TestSSRFValidator_ValidateIP(t *testing.T) {
	tests := []struct {
		name      string
		config    SSRFConfig
		ip        string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "loopback allowed",
			config: SSRFConfig{
				AllowLocalhost: true,
			},
			ip:        "127.0.0.1",
			shouldErr: false,
		},
		{
			name: "IPv6 loopback allowed",
			config: SSRFConfig{
				AllowLocalhost: true,
			},
			ip:        "::1",
			shouldErr: false,
		},
		{
			name: "private IP blocked",
			config: SSRFConfig{
				BlockPrivateIPs: true,
			},
			ip:        "192.168.1.1",
			shouldErr: true,
			errMsg:    "private IP",
		},
		{
			name: "private IP 10.x blocked",
			config: SSRFConfig{
				BlockPrivateIPs: true,
			},
			ip:        "10.0.0.1",
			shouldErr: true,
			errMsg:    "private IP",
		},
		{
			name: "private IP 172.16-31.x blocked",
			config: SSRFConfig{
				BlockPrivateIPs: true,
			},
			ip:        "172.16.0.1",
			shouldErr: true,
			errMsg:    "private IP",
		},
		{
			name: "metadata service blocked",
			config: SSRFConfig{
				BlockMetadata: true,
			},
			ip:        "169.254.169.254",
			shouldErr: true,
			errMsg:    "metadata service",
		},
		{
			name: "link-local blocked",
			config: SSRFConfig{
				BlockLinkLocal: true,
			},
			ip:        "169.254.1.1",
			shouldErr: true,
			errMsg:    "link-local",
		},
		{
			name: "IPv6 link-local blocked",
			config: SSRFConfig{
				BlockLinkLocal: true,
			},
			ip:        "fe80::1",
			shouldErr: true,
			errMsg:    "link-local",
		},
		{
			name: "multicast blocked",
			config: SSRFConfig{},
			ip:        "224.0.0.1",
			shouldErr: true,
			errMsg:    "multicast",
		},
		{
			name: "IPv6 multicast blocked",
			config: SSRFConfig{},
			ip:        "ff02::1",
			shouldErr: true,
			errMsg:    "multicast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSSRFValidator(tt.config)
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}

			err := validator.ValidateIP(ip)

			if tt.shouldErr {
				if err == nil {
					t.Error("expected error but got nil")
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

func TestSSRFValidator_CreateSecureTransport(t *testing.T) {
	config := SSRFConfig{
		AllowedHosts:    []string{"localhost"},
		AllowedSchemes:  []string{"http", "https"},
		AllowLocalhost:  true,
		BlockPrivateIPs: true,
	}
	validator := NewSSRFValidator(config)
	transport := validator.CreateSecureTransport()

	if transport == nil {
		t.Fatal("expected non-nil transport")
	}

	if transport.DialContext == nil {
		t.Fatal("expected DialContext to be configured")
	}

	ctx := context.Background()

	// Test that localhost passes validation
	// Connection may fail if no server, but should not be blocked by validator
	_, err := transport.DialContext(ctx, "tcp", "localhost:11434")
	if err != nil {
		// Connection refused is expected when no server is running
		if !strings.Contains(err.Error(), "connection refused") &&
			!strings.Contains(err.Error(), "connect: connection refused") {
			t.Errorf("localhost connection should pass validation (got: %v)", err)
		}
	}

	// Test that private IPs are blocked
	blockedAddresses := []string{
		"169.254.169.254:80",
		"10.0.0.1:80",
		"192.168.1.1:80",
		"172.16.0.1:80",
	}

	for _, addr := range blockedAddresses {
		t.Run("block_"+addr, func(t *testing.T) {
			_, err := transport.DialContext(ctx, "tcp", addr)
			if err == nil {
				t.Errorf("connection to %s should be blocked", addr)
			}
			if !strings.Contains(err.Error(), "connection blocked") {
				t.Errorf("unexpected error for %s: %v", addr, err)
			}
		})
	}
}

func TestGetOllamaAllowedHosts(t *testing.T) {
	// Save original env var
	originalEnv := os.Getenv("OLLAMA_ALLOWED_HOSTS")
	defer func() {
		if originalEnv != "" {
			_ = os.Setenv("OLLAMA_ALLOWED_HOSTS", originalEnv)
		} else {
			_ = os.Unsetenv("OLLAMA_ALLOWED_HOSTS")
		}
	}()

	tests := []struct {
		name     string
		envValue string
		want     []string
	}{
		{
			name:     "default hosts only",
			envValue: "",
			want:     DefaultOllamaAllowlist,
		},
		{
			name:     "additional single host",
			envValue: "ollama.example.com",
			want:     append(DefaultOllamaAllowlist, "ollama.example.com"),
		},
		{
			name:     "additional multiple hosts",
			envValue: "ollama1.example.com,ollama2.example.com",
			want: append(DefaultOllamaAllowlist,
				"ollama1.example.com",
				"ollama2.example.com"),
		},
		{
			name:     "hosts with whitespace",
			envValue: " ollama1.example.com , ollama2.example.com ",
			want: append(DefaultOllamaAllowlist,
				"ollama1.example.com",
				"ollama2.example.com"),
		},
		{
			name:     "empty values ignored",
			envValue: "ollama1.example.com,,ollama2.example.com",
			want: append(DefaultOllamaAllowlist,
				"ollama1.example.com",
				"ollama2.example.com"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				_ = os.Setenv("OLLAMA_ALLOWED_HOSTS", tt.envValue)
			} else {
				_ = os.Unsetenv("OLLAMA_ALLOWED_HOSTS")
			}

			got := GetOllamaAllowedHosts()

			if len(got) != len(tt.want) {
				t.Errorf("GetOllamaAllowedHosts() length = %v, want %v", len(got), len(tt.want))
				return
			}

			// Convert to map for easier comparison
			gotMap := make(map[string]bool)
			for _, h := range got {
				gotMap[h] = true
			}

			for _, wantHost := range tt.want {
				if !gotMap[wantHost] {
					t.Errorf("GetOllamaAllowedHosts() missing host %v", wantHost)
				}
			}
		})
	}
}

func TestNewOllamaSSRFValidator(t *testing.T) {
	validator := NewOllamaSSRFValidator()

	if validator == nil {
		t.Fatal("expected non-nil validator")
	}

	// Test that default Ollama hosts are allowed
	for _, host := range DefaultOllamaAllowlist {
		t.Run("allow_"+host, func(t *testing.T) {
			err := validator.ValidateHost(host)
			if err != nil {
				t.Errorf("default Ollama host %s should be allowed: %v", host, err)
			}
		})
	}

	// Test that non-allowlisted hosts are blocked
	blockedHosts := []string{
		"evil.com",
		"8.8.8.8",
		"example.com",
	}

	for _, host := range blockedHosts {
		t.Run("block_"+host, func(t *testing.T) {
			err := validator.ValidateHost(host)
			if err == nil {
				t.Errorf("host %s should be blocked", host)
			}
		})
	}
}

func TestDefaultSSRFConfig(t *testing.T) {
	config := DefaultSSRFConfig()

	if len(config.AllowedSchemes) == 0 {
		t.Error("expected default allowed schemes")
	}

	if !config.AllowLocalhost {
		t.Error("expected AllowLocalhost to be true by default")
	}

	if !config.BlockPrivateIPs {
		t.Error("expected BlockPrivateIPs to be true by default")
	}

	if !config.BlockMetadata {
		t.Error("expected BlockMetadata to be true by default")
	}

	if !config.BlockLinkLocal {
		t.Error("expected BlockLinkLocal to be true by default")
	}
}

func TestNewSSRFValidator_DefaultSchemes(t *testing.T) {
	// Test that default schemes are set if not provided
	config := SSRFConfig{
		AllowedHosts: []string{"localhost"},
	}
	validator := NewSSRFValidator(config)

	// Should allow http and https by default
	err := validator.ValidateURL("http://localhost:8080")
	if err != nil {
		t.Errorf("http should be allowed by default: %v", err)
	}

	err = validator.ValidateURL("https://localhost:8080")
	if err != nil {
		t.Errorf("https should be allowed by default: %v", err)
	}
}

// Benchmark tests
func BenchmarkValidateURL(b *testing.B) {
	config := DefaultSSRFConfig()
	config.AllowedHosts = []string{"localhost"}
	validator := NewSSRFValidator(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateURL("http://localhost:11434")
	}
}

func BenchmarkValidateHost(b *testing.B) {
	config := DefaultSSRFConfig()
	config.AllowedHosts = []string{"localhost"}
	validator := NewSSRFValidator(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateHost("localhost")
	}
}

func BenchmarkValidateIP(b *testing.B) {
	config := DefaultSSRFConfig()
	validator := NewSSRFValidator(config)
	ip := net.ParseIP("127.0.0.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateIP(ip)
	}
}
