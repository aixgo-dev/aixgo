package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		wantBaseURL string
		wantErr     bool
	}{
		{
			name:        "with custom URL not in allowlist",
			baseURL:     "http://custom:8080",
			wantBaseURL: "",
			wantErr:     true, // "custom" is not in the allowlist
		},
		{
			name:        "with empty URL defaults to localhost",
			baseURL:     "",
			wantBaseURL: "http://localhost:11434",
			wantErr:     false,
		},
		{
			name:        "with localhost URL",
			baseURL:     "http://localhost:8080",
			wantBaseURL: "http://localhost:8080",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.baseURL)

			if tt.wantErr {
				if err == nil {
					t.Fatal("NewClient() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			if client == nil {
				t.Fatal("NewClient() returned nil")
				return
			}

			if client.baseURL != tt.wantBaseURL {
				t.Errorf("client.baseURL = %q, want %q", client.baseURL, tt.wantBaseURL)
			}

			if client.httpClient == nil {
				t.Error("client.httpClient is nil")
			}

			if client.httpClient.Timeout != 5*time.Minute {
				t.Errorf("client.httpClient.Timeout = %v, want 5m", client.httpClient.Timeout)
			}
		})
	}
}

func TestClient_Generate_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Request method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Errorf("Request path = %s, want /api/generate", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}

		// Parse request body
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		// Verify request fields
		if reqBody["model"] != "test-model" {
			t.Errorf("Request model = %v, want 'test-model'", reqBody["model"])
		}
		if reqBody["prompt"] != "Test prompt" {
			t.Errorf("Request prompt = %v, want 'Test prompt'", reqBody["prompt"])
		}
		if reqBody["stream"] != false {
			t.Errorf("Request stream = %v, want false", reqBody["stream"])
		}

		// Send response
		resp := map[string]any{
			"response": "Generated text",
			"done":     true,
			"context":  []int{1, 2, 3, 4, 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()

	req := inference.GenerateRequest{
		Model:       "test-model",
		Prompt:      "Test prompt",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := client.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}

	if resp.Text != "Generated text" {
		t.Errorf("Generate() text = %q, want 'Generated text'", resp.Text)
	}

	if resp.FinishReason != "stop" {
		t.Errorf("Generate() finish_reason = %q, want 'stop'", resp.FinishReason)
	}

	if resp.Usage.TotalTokens != 5 {
		t.Errorf("Generate() total_tokens = %d, want 5", resp.Usage.TotalTokens)
	}
}

func TestClient_Generate_WithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		// Verify options are set
		options, ok := reqBody["options"].(map[string]any)
		if !ok {
			t.Error("Request options not found")
		}

		if options["num_predict"] != 100.0 {
			t.Errorf("options.num_predict = %v, want 100", options["num_predict"])
		}

		if options["temperature"] != 0.8 {
			t.Errorf("options.temperature = %v, want 0.8", options["temperature"])
		}

		resp := map[string]any{
			"response": "Response",
			"done":     true,
			"context":  []int{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()

	req := inference.GenerateRequest{
		Model:       "test-model",
		Prompt:      "Test",
		MaxTokens:   100,
		Temperature: 0.8,
	}

	_, err = client.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
}

func TestClient_Generate_WithStopSequences(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		options, ok := reqBody["options"].(map[string]any)
		if !ok {
			t.Error("Request options not found")
		}

		stop, ok := options["stop"].([]any)
		if !ok {
			t.Error("options.stop not found or wrong type")
		}

		if len(stop) != 2 {
			t.Errorf("len(options.stop) = %d, want 2", len(stop))
		}

		resp := map[string]any{
			"response": "Response",
			"done":     true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()

	req := inference.GenerateRequest{
		Model:  "test-model",
		Prompt: "Test",
		Stop:   []string{"END", "STOP"},
	}

	_, err = client.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate() error = %v, want nil", err)
	}
}

func TestClient_Generate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()

	req := inference.GenerateRequest{
		Model:  "test-model",
		Prompt: "Test",
	}

	_, err = client.Generate(ctx, req)
	if err == nil {
		t.Error("Generate() error = nil, want error")
	}
}

func TestClient_Generate_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx := context.Background()

	req := inference.GenerateRequest{
		Model:  "test-model",
		Prompt: "Test",
	}

	_, err = client.Generate(ctx, req)
	if err == nil {
		t.Error("Generate() error = nil, want error")
	}
}

func TestClient_Generate_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		resp := map[string]any{
			"response": "Response",
			"done":     true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	req := inference.GenerateRequest{
		Model:  "test-model",
		Prompt: "Test",
	}

	_, err = client.Generate(ctx, req)
	if err == nil {
		t.Error("Generate() with cancelled context error = nil, want error")
	}
}

func TestClient_Available_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("Available() path = %s, want /api/tags", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	available := client.Available()

	if !available {
		t.Error("Available() = false, want true")
	}
}

func TestClient_Available_ServerDown(t *testing.T) {
	// Use an invalid URL
	client, err := NewClient("http://localhost:99999")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	available := client.Available()

	if available {
		t.Error("Available() = true, want false for down server")
	}
}

func TestClient_Available_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	available := client.Available()

	if available {
		t.Error("Available() = true, want false for server error")
	}
}

func TestClient_HasModel_ModelExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"models": []map[string]any{
				{"name": "llama2"},
				{"name": "mistral"},
				{"name": "codellama"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{
			name:  "model exists",
			model: "llama2",
			want:  true,
		},
		{
			name:  "model does not exist",
			model: "gpt-4",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has := client.HasModel(tt.model)
			if has != tt.want {
				t.Errorf("HasModel(%q) = %v, want %v", tt.model, has, tt.want)
			}
		})
	}
}

func TestClient_HasModel_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	has := client.HasModel("llama2")

	if has {
		t.Error("HasModel() = true, want false for server error")
	}
}

func TestClient_HasModel_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	has := client.HasModel("llama2")

	if has {
		t.Error("HasModel() = true, want false for invalid response")
	}
}

func TestClient_HasModel_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate timeout
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	has := client.HasModel("llama2")

	if has {
		t.Error("HasModel() = true, want false for timeout")
	}
}
