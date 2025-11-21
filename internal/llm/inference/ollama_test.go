package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaService_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req["model"] != "llama2" {
			t.Errorf("unexpected model: %v", req["model"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"response":          "Hello, world!",
			"done":              true,
			"prompt_eval_count": 5,
			"eval_count":        3,
		})
	}))
	defer server.Close()

	svc := NewOllamaService(server.URL)
	resp, err := svc.Generate(context.Background(), GenerateRequest{
		Model:  "llama2",
		Prompt: "Say hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Hello, world!" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
	if resp.Usage.PromptTokens != 5 {
		t.Errorf("unexpected prompt tokens: %d", resp.Usage.PromptTokens)
	}
}

func TestOllamaService_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": "I am doing well!",
			},
			"done":              true,
			"prompt_eval_count": 10,
			"eval_count":        5,
		})
	}))
	defer server.Close()

	svc := NewOllamaService(server.URL)
	resp, err := svc.Chat(context.Background(), "llama2", []ChatMessage{
		{Role: "user", Content: "How are you?"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "I am doing well!" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
}

func TestOllamaService_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "llama2", "size": 3826793472},
				{"name": "mistral", "size": 4109854432},
			},
		})
	}))
	defer server.Close()

	svc := NewOllamaService(server.URL)
	models, err := svc.ListModels(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("unexpected number of models: %d", len(models))
	}
	if models[0].Name != "llama2" {
		t.Errorf("unexpected model name: %s", models[0].Name)
	}
}

func TestOllamaService_Available(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
	}))
	defer server.Close()

	svc := NewOllamaService(server.URL)
	if !svc.Available() {
		t.Error("expected service to be available")
	}

	svc2 := NewOllamaService("http://localhost:99999")
	if svc2.Available() {
		t.Error("expected service to be unavailable")
	}
}
