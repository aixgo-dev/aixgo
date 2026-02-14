package provider

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test registration
	mock1 := NewMockProvider("mock1")
	registry.Register("mock1", mock1)

	// Test retrieval
	provider, err := registry.Get("mock1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if provider.Name() != "mock1" {
		t.Errorf("Provider name = %s, want 'mock1'", provider.Name())
	}

	// Test Has
	if !registry.Has("mock1") {
		t.Error("Has('mock1') = false, want true")
	}

	if registry.Has("nonexistent") {
		t.Error("Has('nonexistent') = true, want false")
	}

	// Test List
	mock2 := NewMockProvider("mock2")
	registry.Register("mock2", mock2)

	names := registry.List()
	if len(names) != 2 {
		t.Errorf("List() length = %d, want 2", len(names))
	}

	// Test error for non-existent provider
	_, err = registry.Get("nonexistent")
	if err == nil {
		t.Error("Get('nonexistent') error = nil, want error")
	}
}

func TestMockProvider_CreateCompletion(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider("test")

	// Test default response
	response, err := mock.CreateCompletion(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if response.Content != "Mock response" {
		t.Errorf("Response content = %s, want 'Mock response'", response.Content)
	}

	// Check that call was tracked
	if len(mock.CompletionCalls) != 1 {
		t.Errorf("CompletionCalls length = %d, want 1", len(mock.CompletionCalls))
	}

	// Test custom response
	mock.Reset()
	mock.AddCompletionResponse(&CompletionResponse{
		Content:      "Custom response",
		FinishReason: "stop",
	})

	response, err = mock.CreateCompletion(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if response.Content != "Custom response" {
		t.Errorf("Response content = %s, want 'Custom response'", response.Content)
	}

	// Test error
	mock.Reset()
	mock.AddError(NewProviderError("test", ErrorCodeRateLimit, "rate limited", nil))

	_, err = mock.CreateCompletion(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err == nil {
		t.Error("CreateCompletion() error = nil, want error")
	}

	provErr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("Error type = %T, want *ProviderError", err)
	}

	if provErr.Code != ErrorCodeRateLimit {
		t.Errorf("Error code = %s, want %s", provErr.Code, ErrorCodeRateLimit)
	}

	if !provErr.IsRetryable {
		t.Error("Error IsRetryable = false, want true for rate limit")
	}
}

func TestMockProvider_CreateStructured(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider("test")

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// Test default response
	response, err := mock.CreateStructured(ctx, StructuredRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Create a user"},
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateStructured() error = %v", err)
	}

	if len(response.Data) == 0 {
		t.Error("Response data is empty")
	}

	// Test custom response
	mock.Reset()
	userData := User{Name: "Alice", Age: 30}
	mock.AddStructuredResponse(MockStructuredResponse(userData))

	response, err = mock.CreateStructured(ctx, StructuredRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{
				{Role: "user", Content: "Create a user"},
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateStructured() error = %v", err)
	}

	var user User
	if err := json.Unmarshal(response.Data, &user); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if user.Name != "Alice" {
		t.Errorf("User name = %s, want 'Alice'", user.Name)
	}

	if user.Age != 30 {
		t.Errorf("User age = %d, want 30", user.Age)
	}
}

func TestMockProvider_CreateStreaming(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider("test")

	// Test default stream
	stream, err := mock.CreateStreaming(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Stream this"},
		},
	})

	if err != nil {
		t.Fatalf("CreateStreaming() error = %v", err)
	}

	// Receive chunks
	chunks := []string{}
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		chunks = append(chunks, chunk.Delta)
	}

	if len(chunks) != 3 {
		t.Errorf("Received %d chunks, want 3", len(chunks))
	}

	// Test custom stream
	mock.Reset()
	mock.AddStreamChunks([]*StreamChunk{
		{Delta: "Hello "},
		{Delta: "world!", FinishReason: "stop"},
	})

	stream, err = mock.CreateStreaming(ctx, CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Stream this"},
		},
	})

	if err != nil {
		t.Fatalf("CreateStreaming() error = %v", err)
	}

	chunks = []string{}
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		chunks = append(chunks, chunk.Delta)
		if chunk.FinishReason != "" {
			break
		}
	}

	fullText := ""
	for _, chunk := range chunks {
		fullText += chunk
	}

	if fullText != "Hello world!" {
		t.Errorf("Stream content = %s, want 'Hello world!'", fullText)
	}

	// Test close
	if err := stream.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Test receiving after close
	_, err = stream.Recv()
	if err == nil {
		t.Error("Recv() after Close() error = nil, want error")
	}
}

func TestProviderError(t *testing.T) {
	// Test error creation
	err := NewProviderError("test", ErrorCodeRateLimit, "Too many requests", nil)

	if err.Provider != "test" {
		t.Errorf("Provider = %s, want 'test'", err.Provider)
	}

	if err.Code != ErrorCodeRateLimit {
		t.Errorf("Code = %s, want %s", err.Code, ErrorCodeRateLimit)
	}

	if !err.IsRetryable {
		t.Error("IsRetryable = false, want true for rate limit")
	}

	// Test error message
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	// Test non-retryable error
	authErr := NewProviderError("test", ErrorCodeAuthentication, "Invalid API key", nil)
	if authErr.IsRetryable {
		t.Error("IsRetryable = true, want false for authentication error")
	}

	// Test server error is retryable
	serverErr := NewProviderError("test", ErrorCodeServerError, "Internal error", nil)
	if !serverErr.IsRetryable {
		t.Error("IsRetryable = false, want true for server error")
	}

	// Test timeout is retryable
	timeoutErr := NewProviderError("test", ErrorCodeTimeout, "Request timeout", nil)
	if !timeoutErr.IsRetryable {
		t.Error("IsRetryable = false, want true for timeout")
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Note: Global registry is shared across tests, so we need to be careful
	providerName := "test-global-" + t.Name()

	mock := NewMockProvider(providerName)
	Register(providerName, mock)

	if !Has(providerName) {
		t.Errorf("Has('%s') = false, want true", providerName)
	}

	provider, err := Get(providerName)
	if err != nil {
		t.Fatalf("Get('%s') error = %v", providerName, err)
	}

	if provider.Name() != providerName {
		t.Errorf("Provider name = %s, want '%s'", provider.Name(), providerName)
	}

	names := List()
	found := false
	for _, name := range names {
		if name == providerName {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("List() does not contain '%s'", providerName)
	}
}
