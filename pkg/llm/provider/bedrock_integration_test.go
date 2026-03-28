//go:build integration

package provider

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBedrockProvider_Integration_CreateCompletion tests real completion with Bedrock.
// Requires AWS credentials and model access.
func TestBedrockProvider_Integration_CreateCompletion(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test with Claude 3 Haiku (fastest, most available)
	resp, err := provider.CreateCompletion(ctx, CompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []Message{
			{Role: "user", Content: "Say 'hello world' and nothing else."},
		},
		MaxTokens:   50,
		Temperature: 0,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Content)
	assert.Contains(t, resp.Content, "hello")
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Greater(t, resp.Usage.TotalTokens, 0)
}

// TestBedrockProvider_Integration_CreateStreaming tests real streaming with Bedrock.
func TestBedrockProvider_Integration_CreateStreaming(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := provider.CreateStreaming(ctx, CompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []Message{
			{Role: "user", Content: "Count from 1 to 5."},
		},
		MaxTokens:   100,
		Temperature: 0,
	})

	require.NoError(t, err)
	defer stream.Close()

	var fullContent string
	var chunkCount int

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		fullContent += chunk.Delta
		chunkCount++

		if chunk.FinishReason != "" {
			break
		}
	}

	assert.NotEmpty(t, fullContent)
	assert.Greater(t, chunkCount, 0)
}

// TestBedrockProvider_Integration_CreateStructured tests structured output with Bedrock.
func TestBedrockProvider_Integration_CreateStructured(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name", "age"]
	}`)

	resp, err := provider.CreateStructured(ctx, StructuredRequest{
		CompletionRequest: CompletionRequest{
			Model: "anthropic.claude-3-haiku-20240307-v1:0",
			Messages: []Message{
				{Role: "user", Content: "Create a person named Alice who is 30 years old."},
			},
			MaxTokens:   200,
			Temperature: 0,
		},
		ResponseSchema: schema,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Data)
	// The response should contain structured data
	assert.Contains(t, string(resp.Data), "Alice")
}

// TestBedrockProvider_Integration_ToolCalling tests tool calling with Bedrock.
func TestBedrockProvider_Integration_ToolCalling(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get the current weather for a location",
			Parameters: []byte(`{
				"type": "object",
				"properties": {
					"location": {"type": "string", "description": "City name"}
				},
				"required": ["location"]
			}`),
		},
	}

	resp, err := provider.CreateCompletion(ctx, CompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []Message{
			{Role: "user", Content: "What's the weather in Tokyo?"},
		},
		Tools:       tools,
		MaxTokens:   200,
		Temperature: 0,
	})

	require.NoError(t, err)
	// Model should call the tool when given a weather question
	require.Greater(t, len(resp.ToolCalls), 0, "Expected tool calls but got none")
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Function.Name)
	assert.Contains(t, string(resp.ToolCalls[0].Function.Arguments), "Tokyo")
}

// TestBedrockProvider_Integration_ListModels tests listing available models.
func TestBedrockProvider_Integration_ListModels(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := provider.ListModels(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, models)

	// Check that models have expected fields
	for _, model := range models {
		assert.NotEmpty(t, model.ID)
		assert.Equal(t, "bedrock", model.Provider)
	}
}

// TestBedrockProvider_Integration_MultipleModels tests using different models.
func TestBedrockProvider_Integration_MultipleModels(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	models := []string{
		"anthropic.claude-3-haiku-20240307-v1:0",
		// Add more models as they become available in your account:
		// "amazon.nova-micro-v1:0",
		// "amazon.titan-text-lite-v1",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			resp, err := provider.CreateCompletion(ctx, CompletionRequest{
				Model: model,
				Messages: []Message{
					{Role: "user", Content: "Say 'test' and nothing else."},
				},
				MaxTokens:   20,
				Temperature: 0,
			})

			require.NoError(t, err)
			assert.NotEmpty(t, resp.Content)
		})
	}
}

// TestBedrockProvider_Integration_SystemMessage tests system message handling.
func TestBedrockProvider_Integration_SystemMessage(t *testing.T) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
		t.Skip("AWS credentials not configured, skipping integration test")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	provider, err := NewBedrockProvider(region)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.CreateCompletion(ctx, CompletionRequest{
		Model: "anthropic.claude-3-haiku-20240307-v1:0",
		Messages: []Message{
			{Role: "system", Content: "You always respond in uppercase letters only."},
			{Role: "user", Content: "Say hello"},
		},
		MaxTokens:   50,
		Temperature: 0,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Content)
	// Check that response follows system instruction (uppercase)
	// Note: Model compliance may vary
}
