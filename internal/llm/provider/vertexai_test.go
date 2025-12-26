package provider

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestVertexAIProvider_Name(t *testing.T) {
	// Skip if we can't create a provider (no credentials)
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("GOOGLE_CLOUD_PROJECT not set, skipping unit test that requires provider creation")
	}

	p, err := NewVertexAIProvider("test-project", "us-central1")
	if err != nil {
		t.Skipf("Could not create provider (likely no credentials): %v", err)
	}

	if p.Name() != "vertexai" {
		t.Errorf("expected 'vertexai', got %s", p.Name())
	}
}

func TestVertexAIProvider_Factory(t *testing.T) {
	// Test that factory is registered
	factory, ok := factories["vertexai"]
	if !ok {
		t.Fatal("vertexai factory not registered")
	}

	// Test factory requires project ID
	_, err := factory(map[string]any{})
	if err == nil {
		t.Error("expected error when GOOGLE_CLOUD_PROJECT not set")
	}

	// Test factory with config
	_, err = factory(map[string]any{
		"project_id": "test-project",
		"location":   "us-central1",
	})
	// This may fail if no credentials, which is fine
	if err != nil {
		t.Logf("Factory creation failed (expected if no credentials): %v", err)
	}
}

func TestVertexAIProvider_DefaultLocation(t *testing.T) {
	// Verify default location is used when not specified
	factory, ok := factories["vertexai"]
	if !ok {
		t.Fatal("vertexai factory not registered")
	}

	// Clear env vars for test
	oldProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
	oldLocation := os.Getenv("VERTEX_AI_LOCATION")
	defer func() {
		if oldProject != "" {
			_ = os.Setenv("GOOGLE_CLOUD_PROJECT", oldProject)
		}
		if oldLocation != "" {
			_ = os.Setenv("VERTEX_AI_LOCATION", oldLocation)
		}
	}()

	_ = os.Unsetenv("VERTEX_AI_LOCATION")

	// Factory should use us-central1 as default
	_, err := factory(map[string]any{
		"project_id": "test-project",
	})
	// May fail due to no credentials, but should not fail due to missing location
	if err != nil && err.Error() == "VERTEX_AI_LOCATION not set" {
		t.Error("Factory should use default location 'us-central1'")
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

// Integration tests - require GOOGLE_CLOUD_PROJECT and valid credentials
// Run with: go test -v -run Integration -tags=integration

func skipIfNoCredentials(t *testing.T) {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("GOOGLE_CLOUD_PROJECT not set, skipping integration test")
	}
	// Also check for ADC
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		// Try to detect if gcloud auth is configured
		// This is a best-effort check
		t.Log("GOOGLE_APPLICATION_CREDENTIALS not set, using Application Default Credentials")
	}
}

func TestVertexAIProvider_Integration_CreateCompletion(t *testing.T) {
	skipIfNoCredentials(t)

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	provider, err := NewVertexAIProvider(projectID, location)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.CreateCompletion(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Say 'hello' and nothing else."},
		},
		Model:       "gemini-1.5-flash",
		Temperature: 0.1,
		MaxTokens:   50,
	})

	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Expected non-empty content")
	}

	t.Logf("Response: %s", resp.Content)
	t.Logf("Usage: prompt=%d, completion=%d, total=%d",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func TestVertexAIProvider_Integration_SystemInstruction(t *testing.T) {
	skipIfNoCredentials(t)

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	provider, err := NewVertexAIProvider(projectID, location)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.CreateCompletion(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant. Always respond with exactly one word."},
			{Role: "user", Content: "What color is the sky?"},
		},
		Model:       "gemini-1.5-flash",
		Temperature: 0.1,
		MaxTokens:   10,
	})

	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Expected non-empty content")
	}

	t.Logf("Response: %s", resp.Content)
}

func TestVertexAIProvider_Integration_FunctionCalling(t *testing.T) {
	skipIfNoCredentials(t)

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	provider, err := NewVertexAIProvider(projectID, location)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.CreateCompletion(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "What's the weather in New York?"},
		},
		Model:       "gemini-1.5-flash",
		Temperature: 0.1,
		MaxTokens:   100,
		Tools: []Tool{
			{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				Parameters:  json.RawMessage(`{"type": "object", "properties": {"location": {"type": "string", "description": "The city name"}}, "required": ["location"]}`),
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	// Should either have tool calls or text content
	if len(resp.ToolCalls) == 0 && resp.Content == "" {
		t.Error("Expected either tool calls or content")
	}

	if len(resp.ToolCalls) > 0 {
		t.Logf("Tool calls: %+v", resp.ToolCalls)
	} else {
		t.Logf("Content: %s", resp.Content)
	}
}

func TestVertexAIProvider_Integration_Structured(t *testing.T) {
	skipIfNoCredentials(t)

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	provider, err := NewVertexAIProvider(projectID, location)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.CreateStructured(ctx, StructuredRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "List 3 primary colors."},
			},
			Model:       "gemini-1.5-flash",
			Temperature: 0.1,
			MaxTokens:   200,
		},
		ResponseSchema: json.RawMessage(`{"type": "object", "properties": {"colors": {"type": "array", "items": {"type": "string"}}}, "required": ["colors"]}`),
	})

	if err != nil {
		t.Fatalf("CreateStructured failed: %v", err)
	}

	// Parse the response
	var result struct {
		Colors []string `json:"colors"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		t.Fatalf("Failed to parse response: %v (raw: %s)", err, string(resp.Data))
	}

	if len(result.Colors) == 0 {
		t.Error("Expected colors in response")
	}

	t.Logf("Colors: %v", result.Colors)
}

func TestVertexAIProvider_Integration_Streaming(t *testing.T) {
	skipIfNoCredentials(t)

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("VERTEX_AI_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	provider, err := NewVertexAIProvider(projectID, location)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := provider.CreateStreaming(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Count from 1 to 5."},
		},
		Model:       "gemini-1.5-flash",
		Temperature: 0.1,
		MaxTokens:   50,
	})

	if err != nil {
		t.Fatalf("CreateStreaming failed: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var fullContent string
	chunkCount := 0

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("Recv failed: %v", err)
		}

		fullContent += chunk.Delta
		chunkCount++

		if chunk.FinishReason == "stop" {
			break
		}
	}

	if fullContent == "" {
		t.Error("Expected non-empty streamed content")
	}

	t.Logf("Received %d chunks, full content: %s", chunkCount, fullContent)
}
