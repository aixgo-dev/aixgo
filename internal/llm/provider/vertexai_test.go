package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVertexAIProvider_Name(t *testing.T) {
	p := &VertexAIProvider{
		projectID: "test-project",
		location:  "us-central1",
	}
	if p.Name() != "vertexai" {
		t.Errorf("expected 'vertexai', got %s", p.Name())
	}
}

func newTestVertexAIProvider(serverURL string) *VertexAIProvider {
	return &VertexAIProvider{
		projectID: "test-project",
		location:  "us-central1",
		client:    http.DefaultClient,
		tokenFunc: func(ctx context.Context) (string, error) {
			return "test-token", nil
		},
		baseURLOverride: serverURL,
	}
}

func TestVertexAIProvider_CreateCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected 'Bearer test-token', got %s", auth)
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
						Parts: []geminiPart{{Text: "Hello from Vertex AI!"}},
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

	p := newTestVertexAIProvider(server.URL)

	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "gemini-1.5-flash",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from Vertex AI!" {
		t.Errorf("expected 'Hello from Vertex AI!', got %s", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("expected 'stop', got %s", resp.FinishReason)
	}
}

func TestVertexAIProvider_SystemInstruction(t *testing.T) {
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

	p := newTestVertexAIProvider(server.URL)

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

func TestVertexAIProvider_FunctionCalling(t *testing.T) {
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

	p := newTestVertexAIProvider(server.URL)

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

func TestVertexAIProvider_ErrorHandling(t *testing.T) {
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

			p := newTestVertexAIProvider(server.URL)

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

func TestVertexAIProvider_RoleMapping(t *testing.T) {
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

	p := newTestVertexAIProvider(server.URL)

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

func TestVertexAIProvider_Endpoint(t *testing.T) {
	p := &VertexAIProvider{
		projectID: "my-project",
		location:  "us-central1",
	}

	expected := "https://us-central1-aiplatform.googleapis.com/v1/projects/my-project/locations/us-central1/publishers/google/models/gemini-1.5-flash:generateContent"
	actual := p.endpoint("gemini-1.5-flash")
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestVertexAIProvider_StreamEndpoint(t *testing.T) {
	p := &VertexAIProvider{
		projectID: "my-project",
		location:  "europe-west1",
	}

	expected := "https://europe-west1-aiplatform.googleapis.com/v1/projects/my-project/locations/europe-west1/publishers/google/models/gemini-1.5-pro:streamGenerateContent?alt=sse"
	actual := p.streamEndpoint("gemini-1.5-pro")
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestDetectProvider_VertexAI(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"vertex/gemini-1.5-flash", "vertexai"},
		{"vertex/gemini-1.5-pro", "vertexai"},
		{"gemini-1.5-flash", "gemini"},
		{"gpt-4", "openai"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProvider(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProvider(%s) = %s, want %s", tt.model, result, tt.expected)
			}
		})
	}
}
