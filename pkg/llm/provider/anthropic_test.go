package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropicProvider("test-key", "https://api.anthropic.com/v1")
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic', got %s", p.Name())
	}
}

func TestAnthropicProvider_CreateCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") != anthropicVersion {
			t.Error("missing anthropic-version header")
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		if req["model"] != "claude-3-sonnet-20240229" {
			t.Errorf("expected claude model, got %v", req["model"])
		}

		// Verify system message is extracted
		if _, ok := req["system"]; !ok {
			t.Log("system message not found, may not have been provided")
		}

		resp := anthropicResponse{
			ID:   "msg_test",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Hello from Claude!"},
			},
			StopReason: "end_turn",
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from Claude!" {
		t.Errorf("expected 'Hello from Claude!', got %s", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("expected 'stop', got %s", resp.FinishReason)
	}
}

func TestAnthropicProvider_SystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		system, ok := req["system"].(string)
		if !ok || system != "You are helpful" {
			t.Errorf("expected system message 'You are helpful', got %v", req["system"])
		}

		messages, ok := req["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Errorf("expected 1 message (system should be separate), got %v", len(messages))
		}

		resp := anthropicResponse{
			ID:      "msg_test",
			Type:    "message",
			Role:    "assistant",
			Content: []anthropicContentBlock{{Type: "text", Text: "OK"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	_, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 100,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnthropicProvider_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_test",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Let me check the weather."},
				{
					Type:  "tool_use",
					ID:    "tool_1",
					Name:  "get_weather",
					Input: json.RawMessage(`{"location":"NYC"}`),
				},
			},
			StopReason: "tool_use",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", server.URL)
	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages:  []Message{{Role: "user", Content: "Weather?"}},
		MaxTokens: 100,
		Tools: []Tool{
			{Name: "get_weather", Description: "Get weather", Parameters: json.RawMessage(`{}`)},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected 'get_weather', got %s", resp.ToolCalls[0].Function.Name)
	}
	if resp.FinishReason != "tool_calls" {
		t.Errorf("expected 'tool_calls', got %s", resp.FinishReason)
	}
}

func TestAnthropicProvider_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
	}{
		{"auth error", 401, ErrorCodeAuthentication},
		{"rate limit", 429, ErrorCodeRateLimit},
		{"bad request", 400, ErrorCodeInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"message": "test error",
						"type":    "test_type",
					},
				})
			}))
			defer server.Close()

			p := NewAnthropicProvider("test-key", server.URL)
			_, err := p.CreateCompletion(context.Background(), CompletionRequest{
				Messages:  []Message{{Role: "user", Content: "Hi"}},
				MaxTokens: 100,
			})

			if err == nil {
				t.Fatal("expected error")
			}

			provErr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("expected ProviderError, got %T", err)
			}
			if provErr.Code != tt.errorCode {
				t.Errorf("expected code %s, got %s", tt.errorCode, provErr.Code)
			}
		})
	}
}

func TestAnthropicProvider_Factory(t *testing.T) {
	// Test factory without API key
	_, err := CreateProvider("anthropic", map[string]any{})
	if err == nil {
		t.Error("expected error without API key")
	}

	// Test factory with API key
	p, err := CreateProvider("anthropic", map[string]any{"api_key": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic', got %s", p.Name())
	}
}
