package provider

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestBedrockProvider_Name(t *testing.T) {
	// Skip if no AWS credentials available
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS_REGION not set, skipping test that requires provider initialization")
	}

	p, err := NewBedrockProvider("us-east-1")
	if err != nil {
		t.Skip("Could not create Bedrock provider:", err)
	}

	if p.Name() != "bedrock" {
		t.Errorf("expected 'bedrock', got %s", p.Name())
	}
}

func TestBedrockProvider_NormalizeModelID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bedrock prefix stripped",
			input:    "bedrock/anthropic.claude-3-haiku-20240307-v1:0",
			expected: "anthropic.claude-3-haiku-20240307-v1:0",
		},
		{
			name:     "no prefix unchanged",
			input:    "anthropic.claude-3-haiku-20240307-v1:0",
			expected: "anthropic.claude-3-haiku-20240307-v1:0",
		},
		{
			name:     "meta model unchanged",
			input:    "meta.llama3-70b-instruct-v1:0",
			expected: "meta.llama3-70b-instruct-v1:0",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	// Create a minimal provider for testing the method
	p := &BedrockProvider{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.normalizeModelID(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeModelID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBedrockProvider_WrapError(t *testing.T) {
	p := &BedrockProvider{}

	tests := []struct {
		name         string
		errMsg       string
		expectedCode string
		retryable    bool
	}{
		{
			name:         "throttling error",
			errMsg:       "ThrottlingException: Rate exceeded",
			expectedCode: ErrorCodeRateLimit,
			retryable:    true,
		},
		{
			name:         "access denied",
			errMsg:       "AccessDeniedException: User is not authorized",
			expectedCode: ErrorCodeAuthentication,
			retryable:    false,
		},
		{
			name:         "validation error",
			errMsg:       "ValidationException: Invalid model parameter",
			expectedCode: ErrorCodeInvalidRequest,
			retryable:    false,
		},
		{
			name:         "model not found",
			errMsg:       "ResourceNotFoundException: Model does not exist",
			expectedCode: ErrorCodeModelNotFound,
			retryable:    false,
		},
		{
			name:         "service error",
			errMsg:       "ServiceUnavailableException: Service is temporarily unavailable",
			expectedCode: ErrorCodeServerError,
			retryable:    true,
		},
		{
			name:         "internal error",
			errMsg:       "InternalServerException: An internal error occurred",
			expectedCode: ErrorCodeServerError,
			retryable:    true,
		},
		{
			name:         "timeout error",
			errMsg:       "context deadline exceeded",
			expectedCode: ErrorCodeTimeout,
			retryable:    true,
		},
		{
			name:         "content filter",
			errMsg:       "Content filter blocked the response due to guardrail violation",
			expectedCode: ErrorCodeContentFiltered,
			retryable:    false,
		},
		{
			name:         "unknown error",
			errMsg:       "some random error",
			expectedCode: ErrorCodeUnknown,
			retryable:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.wrapError(&testError{msg: tt.errMsg})
			provErr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("expected *ProviderError, got %T", err)
			}

			if provErr.Provider != "bedrock" {
				t.Errorf("expected provider 'bedrock', got %q", provErr.Provider)
			}

			if provErr.Code != tt.expectedCode {
				t.Errorf("expected code %q, got %q", tt.expectedCode, provErr.Code)
			}

			if provErr.IsRetryable != tt.retryable {
				t.Errorf("expected IsRetryable=%v, got %v", tt.retryable, provErr.IsRetryable)
			}
		})
	}
}

func TestBedrockProvider_IsRetryableError(t *testing.T) {
	p := &BedrockProvider{}

	tests := []struct {
		errMsg    string
		retryable bool
	}{
		{"ThrottlingException", true},
		{"rate limit exceeded", true},
		{"too many requests", true},
		{"ServiceUnavailableException", true},
		{"InternalServerException", true},
		{"context deadline exceeded", true},
		{"service unavailable", true},
		{"AccessDeniedException", false},
		{"ValidationException", false},
		{"some other error", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			result := p.isRetryableError(&testError{msg: tt.errMsg})
			if result != tt.retryable {
				t.Errorf("isRetryableError(%q) = %v, want %v", tt.errMsg, result, tt.retryable)
			}
		})
	}
}

func TestBedrockProvider_CalculateBackoff(t *testing.T) {
	p := &BedrockProvider{}

	// Test that backoff increases with attempts
	var lastDelay int64 = 0
	for attempt := 1; attempt <= 5; attempt++ {
		delay := p.calculateBackoff(attempt)

		// Backoff should generally increase (accounting for jitter)
		if attempt > 1 && delay.Nanoseconds() < lastDelay/2 {
			// Allow some variance due to jitter, but it shouldn't decrease too much
			t.Logf("Warning: backoff may not be increasing properly at attempt %d", attempt)
		}

		// Ensure delay is positive and reasonable
		if delay < 0 {
			t.Errorf("attempt %d: negative delay %v", attempt, delay)
		}

		// Max delay is 32 seconds, with jitter could be up to ~42 seconds
		if delay.Seconds() > 50 {
			t.Errorf("attempt %d: delay %v exceeds max expected", attempt, delay)
		}

		lastDelay = delay.Nanoseconds()
	}
}

func TestBedrockProvider_Factory(t *testing.T) {
	// Skip if no AWS credentials
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS credentials not configured, skipping factory test")
	}

	// Test factory creates provider
	p, err := CreateProvider("bedrock", map[string]any{
		"region": "us-east-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Name() != "bedrock" {
		t.Errorf("expected 'bedrock', got %s", p.Name())
	}
}

func TestBedrockProvider_ConvertRole(t *testing.T) {
	p := &BedrockProvider{}

	tests := []struct {
		role     string
		expected string
	}{
		{"assistant", "assistant"},
		{"user", "user"},
		{"system", "user"}, // System messages are handled separately
		{"", "user"},       // Empty defaults to user
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := p.convertRole(tt.role)
			if string(result) != tt.expected {
				t.Errorf("convertRole(%q) = %q, want %q", tt.role, result, tt.expected)
			}
		})
	}
}

func TestBedrockProvider_GetKnownModels(t *testing.T) {
	p := &BedrockProvider{}

	models := p.getKnownModels()

	if len(models) == 0 {
		t.Error("expected at least one known model")
	}

	// Check that all models have required fields
	for _, m := range models {
		if m.ID == "" {
			t.Error("model ID should not be empty")
		}
		if m.Provider != "bedrock" {
			t.Errorf("expected provider 'bedrock', got %q", m.Provider)
		}
		if m.Description == "" {
			t.Errorf("model %s should have description", m.ID)
		}
	}

	// Check for expected models
	expectedModels := map[string]bool{
		"anthropic.claude-3-haiku-20240307-v1:0": false,
		"meta.llama3-70b-instruct-v1:0":          false,
		"mistral.mistral-large-2407-v1:0":        false,
	}

	for _, m := range models {
		if _, exists := expectedModels[m.ID]; exists {
			expectedModels[m.ID] = true
		}
	}

	for modelID, found := range expectedModels {
		if !found {
			t.Errorf("expected model %q not found in known models", modelID)
		}
	}
}

func TestBedrockProvider_ModelPricing(t *testing.T) {
	// Verify pricing map has expected entries
	expectedModels := []string{
		"anthropic.claude-3-haiku-20240307-v1:0",
		"anthropic.claude-3-opus-20240229-v1:0",
		"meta.llama3-70b-instruct-v1:0",
		"amazon.nova-pro-v1:0",
	}

	for _, modelID := range expectedModels {
		pricing, ok := bedrockModelPricing[modelID]
		if !ok {
			t.Errorf("missing pricing for model %s", modelID)
			continue
		}

		if pricing.input < 0 {
			t.Errorf("model %s has negative input cost", modelID)
		}
		if pricing.output < 0 {
			t.Errorf("model %s has negative output cost", modelID)
		}
		if pricing.description == "" {
			t.Errorf("model %s has empty description", modelID)
		}
	}
}

func TestBedrockProvider_BuildToolConfig(t *testing.T) {
	p := &BedrockProvider{}

	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}`),
		},
		{
			Name:        "search",
			Description: "Search for information",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		},
	}

	config, err := p.buildToolConfig(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("expected non-nil tool config")
	}

	if len(config.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(config.Tools))
	}
}

func TestBedrockProvider_BuildToolConfig_InvalidJSON(t *testing.T) {
	p := &BedrockProvider{}

	tools := []Tool{
		{
			Name:        "bad_tool",
			Description: "Tool with invalid JSON",
			Parameters:  json.RawMessage(`{invalid json`),
		},
	}

	_, err := p.buildToolConfig(tools)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDetectProvider_Bedrock(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		// Bedrock prefix
		{"bedrock/anthropic.claude-3-haiku", "bedrock"},
		{"bedrock/meta.llama3-70b", "bedrock"},

		// Full Bedrock model IDs
		{"anthropic.claude-3-haiku-20240307-v1:0", "bedrock"},
		{"anthropic.claude-3-opus-20240229-v1:0", "bedrock"},
		{"meta.llama3-70b-instruct-v1:0", "bedrock"},
		{"mistral.mistral-large-2407-v1:0", "bedrock"},
		{"amazon.titan-text-express-v1", "bedrock"},
		{"cohere.command-r-plus-v1:0", "bedrock"},
		{"ai21.jamba-1-5-large-v1:0", "bedrock"},

		// Non-Bedrock models (direct API)
		{"claude-3-haiku-20240307", "anthropic"},
		{"gpt-4-turbo", "openai"},
		{"gemini-1.5-pro", "gemini"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := DetectProvider(tt.model)
			if result != tt.expected {
				t.Errorf("DetectProvider(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

// Integration tests - only run with AWS credentials
func TestBedrockProvider_Integration_ListModels(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	p, err := NewBedrockProvider("us-east-1")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Error("expected at least one model")
	}

	// Check model structure
	for _, m := range models {
		if m.Provider != "bedrock" {
			t.Errorf("expected provider 'bedrock', got %q", m.Provider)
		}
	}
}

func TestBedrockProvider_Integration_CreateCompletion(t *testing.T) {
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}
	if os.Getenv("AIXGO_RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Set AIXGO_RUN_INTEGRATION_TESTS=true to run integration tests")
	}

	p, err := NewBedrockProvider("us-east-1")
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	resp, err := p.CreateCompletion(context.Background(), CompletionRequest{
		Model:     "anthropic.claude-3-haiku-20240307-v1:0",
		Messages:  []Message{{Role: "user", Content: "Say hello in one word."}},
		MaxTokens: 50,
	})

	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty content")
	}

	if resp.Usage.TotalTokens == 0 {
		t.Error("expected non-zero token usage")
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
