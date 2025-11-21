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

func TestXAIProvider_Name(t *testing.T) {
	p := NewXAIProvider("test-key", "grok-beta", "")
	if p.Name() != "xai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "xai")
	}
}

func TestXAIProvider_CreateCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Path = %q, want /chat/completions", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-key")
		}

		// Parse request body
		var req xaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "grok-beta" {
			t.Errorf("Model = %q, want grok-beta", req.Model)
		}

		// Return response
		resp := xaiResponse{
			ID: "chatcmpl-123",
			Choices: []struct {
				Index        int        `json:"index"`
				Message      xaiMessage `json:"message"`
				FinishReason string     `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      xaiMessage{Role: "assistant", Content: "Hello from Grok!"},
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
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := NewXAIProvider("test-key", "grok-beta", server.URL)

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := p.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	if resp.Content != "Hello from Grok!" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello from Grok!")
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", resp.FinishReason, "stop")
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, 15)
	}
}

func TestXAIProvider_CreateCompletion_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req xaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Verify tools were passed
		if len(req.Tools) != 1 {
			t.Errorf("len(Tools) = %d, want 1", len(req.Tools))
		}

		// Return tool call response
		resp := xaiResponse{
			ID: "chatcmpl-456",
			Choices: []struct {
				Index        int        `json:"index"`
				Message      xaiMessage `json:"message"`
				FinishReason string     `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: xaiMessage{
						Role: "assistant",
						ToolCalls: []xaiToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
									Name:      "get_weather",
									Arguments: `{"location": "San Francisco"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := NewXAIProvider("test-key", "grok-beta", server.URL)

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "What's the weather in SF?"},
		},
		Tools: []Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Parameters:  json.RawMessage(`{"type": "object", "properties": {"location": {"type": "string"}}}`),
			},
		},
	}

	resp, err := p.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCall name = %q, want %q", resp.ToolCalls[0].Function.Name, "get_weather")
	}
}

func TestXAIProvider_CreateStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req xaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if !req.Stream {
			t.Error("Stream should be true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter doesn't support flushing")
		}

		chunks := []string{
			`{"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":" from"},"finish_reason":null}]}`,
			`{"choices":[{"delta":{"content":" Grok!"},"finish_reason":"stop"}]}`,
		}

		for _, chunk := range chunks {
			if _, err := w.Write([]byte("data: " + chunk + "\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
		if _, err := w.Write([]byte("data: [DONE]\n\n")); err != nil {
			return
		}
		flusher.Flush()
	}))
	defer server.Close()

	p := NewXAIProvider("test-key", "grok-beta", server.URL)

	stream, err := p.CreateStreaming(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("CreateStreaming failed: %v", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	var content strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		content.WriteString(chunk.Delta)
	}

	if content.String() != "Hello from Grok!" {
		t.Errorf("Content = %q, want %q", content.String(), "Hello from Grok!")
	}
}

func TestXAIProvider_CreateStructured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req xaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.ResponseFormat == nil {
			t.Error("ResponseFormat should be set")
		}

		resp := xaiResponse{
			ID: "chatcmpl-789",
			Choices: []struct {
				Index        int        `json:"index"`
				Message      xaiMessage `json:"message"`
				FinishReason string     `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      xaiMessage{Role: "assistant", Content: `{"name": "John", "age": 30}`},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := NewXAIProvider("test-key", "grok-beta", server.URL)

	resp, err := p.CreateStructured(context.Background(), StructuredRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{{Role: "user", Content: "Give me a person"}},
		},
		ResponseSchema: json.RawMessage(`{"type": "object", "properties": {"name": {"type": "string"}, "age": {"type": "integer"}}}`),
	})
	if err != nil {
		t.Fatalf("CreateStructured failed: %v", err)
	}

	var data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data.Name != "John" || data.Age != 30 {
		t.Errorf("Data = %+v, want {John, 30}", data)
	}
}

func TestXAIProvider_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   string
	}{
		{
			name:       "authentication error",
			statusCode: 401,
			body:       `{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`,
			wantCode:   ErrorCodeAuthentication,
		},
		{
			name:       "rate limit",
			statusCode: 429,
			body:       `{"error": {"message": "Rate limit exceeded", "type": "rate_limit_error"}}`,
			wantCode:   ErrorCodeRateLimit,
		},
		{
			name:       "model not found",
			statusCode: 404,
			body:       `{"error": {"message": "Model not found", "type": "invalid_request_error"}}`,
			wantCode:   ErrorCodeModelNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.body)); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			}))
			defer server.Close()

			p := NewXAIProvider("test-key", "grok-beta", server.URL)

			_, err := p.CreateCompletion(context.Background(), CompletionRequest{
				Messages: []Message{{Role: "user", Content: "Hi"}},
			})

			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			provErr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("Expected ProviderError, got %T", err)
			}

			if provErr.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", provErr.Code, tt.wantCode)
			}
		})
	}
}

func TestXAIProvider_DefaultModel(t *testing.T) {
	p := NewXAIProvider("test-key", "", "")
	if p.model != "grok-beta" {
		t.Errorf("Default model = %q, want %q", p.model, "grok-beta")
	}
}

func TestXAIProvider_DefaultBaseURL(t *testing.T) {
	p := NewXAIProvider("test-key", "grok-beta", "")
	if p.baseURL != xaiBaseURL {
		t.Errorf("Default baseURL = %q, want %q", p.baseURL, xaiBaseURL)
	}
}
