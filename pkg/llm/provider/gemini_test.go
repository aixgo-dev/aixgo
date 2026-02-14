package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiProvider_Name(t *testing.T) {
	p := NewGeminiProvider("test-key", "https://generativelanguage.googleapis.com/v1beta")
	if p.Name() != "gemini" {
		t.Errorf("expected 'gemini', got %s", p.Name())
	}
}

func TestGeminiProvider_CreateCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key in URL
		if !strings.Contains(r.URL.RawQuery, "key=test-key") {
			t.Error("missing API key in URL")
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		contents, ok := req["contents"].([]any)
		if !ok || len(contents) == 0 {
			t.Error("expected contents in request")
		}

		resp := geminiResponse{
			Candidates: []struct {
				Content       geminiContent `json:"content"`
				FinishReason  string        `json:"finishReason"`
				SafetyRatings []struct {
					Category    string `json:"category"`
					Probability string `json:"probability"`
				} `json:"safetyRatings"`
			}{
				{
					Content: geminiContent{
						Role:  "model",
						Parts: []geminiPart{{Text: "Hello from Gemini!"}},
					},
					FinishReason: "STOP",
				},
			},
			UsageMetadata: struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			}{
				PromptTokenCount:     10,
				CandidatesTokenCount: 5,
				TotalTokenCount:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", server.URL)
	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "gemini-1.5-flash",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from Gemini!" {
		t.Errorf("expected 'Hello from Gemini!', got %s", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("expected 'stop', got %s", resp.FinishReason)
	}
}

func TestGeminiProvider_SystemInstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		sysInst, ok := req["systemInstruction"].(map[string]any)
		if !ok {
			t.Error("expected systemInstruction in request")
		} else {
			parts, _ := sysInst["parts"].([]any)
			if len(parts) == 0 {
				t.Error("expected parts in systemInstruction")
			}
		}

		resp := geminiResponse{
			Candidates: []struct {
				Content       geminiContent `json:"content"`
				FinishReason  string        `json:"finishReason"`
				SafetyRatings []struct {
					Category    string `json:"category"`
					Probability string `json:"probability"`
				} `json:"safetyRatings"`
			}{
				{
					Content:      geminiContent{Parts: []geminiPart{{Text: "OK"}}},
					FinishReason: "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", server.URL)
	_, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hi"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeminiProvider_FunctionCalling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		tools, ok := req["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Error("expected tools in request")
		}

		resp := geminiResponse{
			Candidates: []struct {
				Content       geminiContent `json:"content"`
				FinishReason  string        `json:"finishReason"`
				SafetyRatings []struct {
					Category    string `json:"category"`
					Probability string `json:"probability"`
				} `json:"safetyRatings"`
			}{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{
								FunctionCall: &geminiFuncCall{
									Name: "get_weather",
									Args: map[string]any{"location": "NYC"},
								},
							},
						},
					},
					FinishReason: "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", server.URL)
	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Weather?"}},
		Tools: []Tool{
			{Name: "get_weather", Description: "Get weather", Parameters: json.RawMessage(`{"type":"object"}`)},
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

func TestGeminiProvider_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
	}{
		{"auth error", 401, ErrorCodeAuthentication},
		{"rate limit", 429, ErrorCodeRateLimit},
		{"bad request", 400, ErrorCodeInvalidRequest},
		{"not found", 404, ErrorCodeModelNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    tt.statusCode,
						"message": "test error",
						"status":  "TEST_ERROR",
					},
				})
			}))
			defer server.Close()

			p := NewGeminiProvider("test-key", server.URL)
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

func TestGeminiProvider_Factory(t *testing.T) {
	// Test factory without API key
	_, err := CreateProvider("gemini", map[string]any{})
	if err == nil {
		t.Error("expected error without API key")
	}

	// Test factory with API key
	p, err := CreateProvider("gemini", map[string]any{"api_key": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "gemini" {
		t.Errorf("expected 'gemini', got %s", p.Name())
	}
}

func TestGeminiProvider_RoleMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		contents, _ := req["contents"].([]any)
		for _, c := range contents {
			content, _ := c.(map[string]any)
			role := content["role"].(string)
			if role == "assistant" {
				t.Error("assistant role should be mapped to model")
			}
		}

		resp := geminiResponse{
			Candidates: []struct {
				Content       geminiContent `json:"content"`
				FinishReason  string        `json:"finishReason"`
				SafetyRatings []struct {
					Category    string `json:"category"`
					Probability string `json:"probability"`
				} `json:"safetyRatings"`
			}{
				{
					Content:      geminiContent{Parts: []geminiPart{{Text: "OK"}}},
					FinishReason: "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewGeminiProvider("test-key", server.URL)
	_, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello"},
			{Role: "user", Content: "How are you?"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
