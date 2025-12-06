package observability

import (
	"strings"
	"testing"
)

func TestNewLangfuseClient_EnforcesHTTPS(t *testing.T) {
	tests := []struct {
		name      string
		config    LangfuseConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "HTTP URL rejected",
			config: LangfuseConfig{
				BaseURL:   "http://langfuse.com",
				PublicKey: "pk_test",
				SecretKey: "sk_test_1234567890123456",
				Enabled:   true,
			},
			wantErr:   true,
			errSubstr: "must use HTTPS",
		},
		{
			name: "HTTPS URL accepted",
			config: LangfuseConfig{
				BaseURL:   "https://cloud.langfuse.com",
				PublicKey: "pk_test",
				SecretKey: "sk_test_1234567890123456",
				Enabled:   true,
			},
			wantErr: false,
		},
		{
			name: "Disabled client with HTTP is OK",
			config: LangfuseConfig{
				BaseURL: "http://langfuse.com",
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "Missing credentials",
			config: LangfuseConfig{
				BaseURL: "https://cloud.langfuse.com",
				Enabled: true,
			},
			wantErr:   true,
			errSubstr: "credentials required",
		},
		{
			name: "Short secret key",
			config: LangfuseConfig{
				BaseURL:   "https://cloud.langfuse.com",
				PublicKey: "pk_test",
				SecretKey: "short",
				Enabled:   true,
			},
			wantErr:   true,
			errSubstr: "too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLangfuseClient(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("expected error to contain %q, got %q", tt.errSubstr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client, got nil")
				}
			}
		})
	}
}

func TestNewLangfuseClient_TLSConfig(t *testing.T) {
	config := LangfuseConfig{
		BaseURL:   "https://cloud.langfuse.com",
		PublicKey: "pk_test",
		SecretKey: "sk_test_1234567890123456",
		Enabled:   true,
	}

	client, err := NewLangfuseClient(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.httpClient == nil {
		t.Fatal("expected httpClient, got nil")
	}

	// Check that Transport is configured
	if client.httpClient.Transport == nil {
		t.Error("expected Transport to be configured")
	}
}
