package inference

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVLLMService_Generate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"text": "Generated text", "finish_reason": "stop"},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer server.Close()

	svc := NewVLLMService(server.URL, "test-key")
	resp, err := svc.Generate(context.Background(), GenerateRequest{
		Model:  "mistral-7b",
		Prompt: "Hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Generated text" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("unexpected total tokens: %d", resp.Usage.TotalTokens)
	}
}

func TestVLLMService_ChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message":       map[string]any{"content": "Chat response"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     8,
				"completion_tokens": 4,
				"total_tokens":      12,
			},
		})
	}))
	defer server.Close()

	svc := NewVLLMService(server.URL, "test-key")
	resp, err := svc.ChatCompletion(context.Background(), "mistral-7b", []ChatMessage{
		{Role: "user", Content: "Hi"},
	}, 100)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "Chat response" {
		t.Errorf("unexpected text: %s", resp.Text)
	}
}

func TestVLLMService_BatchGenerate(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"text": "Response", "finish_reason": "stop"},
			},
			"usage": map[string]any{"total_tokens": 10},
		})
	}))
	defer server.Close()

	svc := NewVLLMService(server.URL, "")
	reqs := []GenerateRequest{
		{Model: "model", Prompt: "A"},
		{Model: "model", Prompt: "B"},
		{Model: "model", Prompt: "C"},
	}

	results, err := svc.BatchGenerate(context.Background(), reqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("unexpected results count: %d", len(results))
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestVLLMService_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "model-1"},
				{"id": "model-2"},
			},
		})
	}))
	defer server.Close()

	svc := NewVLLMService(server.URL, "")
	models, err := svc.ListModels(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("unexpected models count: %d", len(models))
	}
}

func TestVLLMService_Available(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer server.Close()

	svc := NewVLLMService(server.URL, "")
	if !svc.Available() {
		t.Error("expected service to be available")
	}
}
