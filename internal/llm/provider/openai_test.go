package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_Name(t *testing.T) {
	p := NewOpenAIProvider("test-key", "https://api.openai.com/v1")
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got %s", p.Name())
	}
}

func TestOpenAIProvider_CreateCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing authorization header")
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		if req["model"] != "gpt-4" {
			t.Errorf("expected model gpt-4, got %v", req["model"])
		}

		resp := openaiResponse{
			ID: "test-id",
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openaiMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      openaiMessage{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", server.URL)
	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "gpt-4",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got %s", resp.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAIProvider_CreateCompletion_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		tools, ok := req["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Error("expected 1 tool")
		}

		resp := openaiResponse{
			ID: "test-id",
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openaiMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: openaiMessage{
						Role: "assistant",
						ToolCalls: []openaiToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "get_weather",
									Arguments: `{"location":"NYC"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", server.URL)
	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "What's the weather?"}},
		Model:    "gpt-4",
		Tools: []Tool{
			{
				Name:        "get_weather",
				Description: "Get weather",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
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
}

func TestOpenAIProvider_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
	}{
		{"auth error", 401, ErrorCodeAuthentication},
		{"rate limit", 429, ErrorCodeRateLimit},
		{"bad request", 400, ErrorCodeInvalidRequest},
		{"not found", 404, ErrorCodeModelNotFound},
		{"server error", 500, ErrorCodeServerError},
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

			p := NewOpenAIProvider("test-key", server.URL)
			_, err := p.CreateCompletion(context.Background(), CompletionRequest{
				Messages: []Message{{Role: "user", Content: "Hi"}},
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

func TestOpenAIProvider_Factory(t *testing.T) {
	// Test factory without API key
	_, err := CreateProvider("openai", map[string]any{})
	if err == nil {
		t.Error("expected error without API key")
	}

	// Test factory with API key
	p, err := CreateProvider("openai", map[string]any{"api_key": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got %s", p.Name())
	}
}
