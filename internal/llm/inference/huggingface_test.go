package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHuggingFaceService_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
		}

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"generated_text": "HF response"},
		})
	}))
	defer server.Close()

	svc := NewHuggingFaceService(server.URL, "test-token")
	resp, err := svc.Generate(context.Background(), GenerateRequest{
		Prompt:    "Hello",
		MaxTokens: 50,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "HF response" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
}

func TestHuggingFaceService_Generate_SingleObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"generated_text": "Single response",
		})
	}))
	defer server.Close()

	svc := NewHuggingFaceService(server.URL, "test-token")
	resp, err := svc.Generate(context.Background(), GenerateRequest{
		Prompt: "Hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Single response" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
}

func TestHuggingFaceService_Generate_NoToken(t *testing.T) {
	svc := NewHuggingFaceService("http://example.com", "")
	_, err := svc.Generate(context.Background(), GenerateRequest{
		Prompt: "Hello",
	})

	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestHuggingFaceService_Available(t *testing.T) {
	svc := NewHuggingFaceService("", "test-token")
	if !svc.Available() {
		t.Error("expected available with token")
	}

	svc2 := NewHuggingFaceService("", "")
	if svc2.Available() {
		t.Error("expected unavailable without token")
	}
}

func TestHuggingFaceService_TextGenerationInference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"generated_text": "TGI response",
		})
	}))
	defer server.Close()

	svc := NewHuggingFaceService("", "test-token")
	svc.skipURLValidate = true // Allow test server URL
	resp, err := svc.TextGenerationInference(context.Background(), server.URL, GenerateRequest{
		Prompt:    "Hello",
		MaxTokens: 100,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "TGI response" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
}

func TestHuggingFaceService_TextGenerationInference_SSRFPrevention(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{"valid_https_hf", "https://api-inference.huggingface.co/models/test", false},
		{"http_not_allowed", "http://api-inference.huggingface.co/models/test", true},
		{"internal_ip", "https://192.168.1.1/models/test", true},
		{"localhost", "https://localhost/models/test", true},
		{"arbitrary_host", "https://evil.com/models/test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpointURL(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEndpointURL(%q) error = %v, wantErr %v", tt.endpoint, err, tt.wantErr)
			}
		})
	}
}

func TestHuggingFaceService_Generate_ModelValidation(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{"valid_model", "bert-base-uncased", false},
		{"valid_org_model", "google/bert-base", false},
		{"path_traversal", "../../../etc/passwd", true},
		{"with_dots", "model..name", true},
		{"empty", "", false},
		{"too_long", string(make([]byte, 300)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateModelName(tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateModelName(%q) error = %v, wantErr %v", tt.model, err, tt.wantErr)
			}
		})
	}
}
