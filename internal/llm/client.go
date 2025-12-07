package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/internal/llm/validator"
)

// Client provides high-level LLM operations with validation
type Client struct {
	provider provider.Provider
	config   ClientConfig
}

// ClientConfig configures the LLM client
type ClientConfig struct {
	// DefaultModel to use if not specified in requests
	DefaultModel string

	// DefaultTemperature to use if not specified
	DefaultTemperature float64

	// MaxRetries for validation failures (default: 3, set to 1 to disable retry)
	MaxRetries int

	// DisableValidationRetry disables automatic retry on validation failures
	// When true, validation errors will fail immediately without retry
	DisableValidationRetry bool

	// StrictValidation enables strict mode (no type coercion)
	StrictValidation bool
}

// NewClient creates a new LLM client
func NewClient(prov provider.Provider, config ClientConfig) *Client {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.DefaultTemperature == 0 {
		config.DefaultTemperature = 0.7
	}

	return &Client{
		provider: prov,
		config:   config,
	}
}

// CreateOptions contains options for creation requests
type CreateOptions struct {
	// Model to use (overrides default)
	Model string

	// Temperature (overrides default)
	Temperature float64

	// MaxTokens limits response length
	MaxTokens int

	// SystemPrompt sets the system message
	SystemPrompt string

	// Schema is the JSON Schema for the response (optional - will be generated from type if not provided)
	Schema json.RawMessage

	// ValidationMode can be "strict" or "lax"
	ValidationMode string
}

// CreateStructured creates a structured response of type T with automatic validation retry
func CreateStructured[T any](ctx context.Context, client *Client, prompt string, options *CreateOptions) (*T, error) {
	if options == nil {
		options = &CreateOptions{}
	}

	// Build initial messages
	messages := []provider.Message{}

	if options.SystemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: options.SystemPrompt,
		})
	}

	messages = append(messages, provider.Message{
		Role:    "user",
		Content: prompt,
	})

	// Determine model
	model := options.Model
	if model == "" {
		model = client.config.DefaultModel
	}

	// Determine temperature
	temperature := options.Temperature
	if temperature == 0 {
		temperature = client.config.DefaultTemperature
	}

	// Determine max retries (default: 3 for Pydantic AI-style behavior)
	maxRetries := client.config.MaxRetries
	if client.config.DisableValidationRetry {
		maxRetries = 1 // Single attempt if retry is disabled
	}
	if maxRetries == 0 {
		maxRetries = 1 // At least one attempt
	}

	// Retry loop for validation failures
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create request with current messages (includes retry feedback if retrying)
		request := provider.StructuredRequest{
			CompletionRequest: provider.CompletionRequest{
				Messages:    messages,
				Model:       model,
				Temperature: temperature,
				MaxTokens:   options.MaxTokens,
			},
			ResponseSchema: options.Schema,
			StrictSchema:   client.config.StrictValidation || options.ValidationMode == "strict",
		}

		// Make request
		response, err := client.provider.CreateStructured(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("provider error: %w", err)
		}

		// Parse response data
		var data map[string]any
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Validate and convert to target type
		var result *T
		var validationErr error
		if client.config.StrictValidation || options.ValidationMode == "strict" {
			result, validationErr = validator.ValidateStrict[T](data)
		} else {
			result, validationErr = validator.Validate[T](data)
		}

		// Success! Return result
		if validationErr == nil {
			return result, nil
		}

		// Last attempt failed - return error
		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("validation failed after %d attempts: %w", maxRetries, validationErr)
		}

		// Retry with validation feedback - append assistant's response and user's feedback
		feedbackMsg := formatValidationFeedback(validationErr, response.Content)
		messages = append(messages,
			provider.Message{Role: "assistant", Content: response.Content},
			provider.Message{Role: "user", Content: feedbackMsg},
		)
	}

	// Unreachable (loop always returns)
	return nil, fmt.Errorf("unreachable")
}

// CreateList creates a list of structured responses of type T with automatic validation retry
func CreateList[T any](ctx context.Context, client *Client, prompt string, options *CreateOptions) ([]*T, error) {
	if options == nil {
		options = &CreateOptions{}
	}

	// Build initial messages
	messages := []provider.Message{}

	if options.SystemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: options.SystemPrompt,
		})
	}

	// Add instruction to return a list
	userPrompt := prompt + "\n\nReturn your response as a JSON array of objects."
	messages = append(messages, provider.Message{
		Role:    "user",
		Content: userPrompt,
	})

	// Determine model
	model := options.Model
	if model == "" {
		model = client.config.DefaultModel
	}

	// Determine temperature
	temperature := options.Temperature
	if temperature == 0 {
		temperature = client.config.DefaultTemperature
	}

	// Determine max retries (default: 3 for Pydantic AI-style behavior)
	maxRetries := client.config.MaxRetries
	if client.config.DisableValidationRetry {
		maxRetries = 1 // Single attempt if retry is disabled
	}
	if maxRetries == 0 {
		maxRetries = 1 // At least one attempt
	}

	// Retry loop for validation failures
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create request with current messages (includes retry feedback if retrying)
		request := provider.StructuredRequest{
			CompletionRequest: provider.CompletionRequest{
				Messages:    messages,
				Model:       model,
				Temperature: temperature,
				MaxTokens:   options.MaxTokens,
			},
			ResponseSchema: options.Schema,
			StrictSchema:   client.config.StrictValidation || options.ValidationMode == "strict",
		}

		// Make request
		response, err := client.provider.CreateStructured(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("provider error: %w", err)
		}

		// Parse response data
		var dataList []any
		if err := json.Unmarshal(response.Data, &dataList); err != nil {
			// Retry with parsing error feedback
			if attempt < maxRetries-1 {
				feedbackMsg := formatValidationFeedback(err, response.Content)
				messages = append(messages,
					provider.Message{Role: "assistant", Content: response.Content},
					provider.Message{Role: "user", Content: feedbackMsg},
				)
				continue
			}
			return nil, fmt.Errorf("failed to parse response as array: %w", err)
		}

		// Validate each item
		results := make([]*T, 0, len(dataList))
		var validationErr error

		for i, item := range dataList {
			mapData, ok := item.(map[string]any)
			if !ok {
				validationErr = fmt.Errorf("item %d is not an object", i)
				break
			}

			var result *T
			if client.config.StrictValidation || options.ValidationMode == "strict" {
				result, err = validator.ValidateStrict[T](mapData)
			} else {
				result, err = validator.Validate[T](mapData)
			}

			if err != nil {
				validationErr = fmt.Errorf("validation error for item %d: %w", i, err)
				break
			}

			results = append(results, result)
		}

		// Success! All items validated
		if validationErr == nil {
			return results, nil
		}

		// Last attempt failed - return error
		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("validation failed after %d attempts: %w", maxRetries, validationErr)
		}

		// Retry with validation feedback
		feedbackMsg := formatValidationFeedback(validationErr, response.Content)
		messages = append(messages,
			provider.Message{Role: "assistant", Content: response.Content},
			provider.Message{Role: "user", Content: feedbackMsg},
		)
	}

	// Unreachable (loop always returns)
	return nil, fmt.Errorf("unreachable")
}

// CreateCompletion creates a simple text completion
func CreateCompletion(ctx context.Context, client *Client, prompt string, options *CreateOptions) (string, error) {
	if options == nil {
		options = &CreateOptions{}
	}

	// Build messages
	messages := []provider.Message{}

	if options.SystemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: options.SystemPrompt,
		})
	}

	messages = append(messages, provider.Message{
		Role:    "user",
		Content: prompt,
	})

	// Determine model
	model := options.Model
	if model == "" {
		model = client.config.DefaultModel
	}

	// Determine temperature
	temperature := options.Temperature
	if temperature == 0 {
		temperature = client.config.DefaultTemperature
	}

	// Create request
	request := provider.CompletionRequest{
		Messages:    messages,
		Model:       model,
		Temperature: temperature,
		MaxTokens:   options.MaxTokens,
	}

	// Make request
	response, err := client.provider.CreateCompletion(ctx, request)
	if err != nil {
		return "", fmt.Errorf("provider error: %w", err)
	}

	return response.Content, nil
}

// formatValidationFeedback formats validation errors into a user-friendly retry prompt
func formatValidationFeedback(validationErr error, previousOutput string) string {
	return fmt.Sprintf(`Your previous response did not pass validation:

%s

Please correct the issues and provide a valid response that matches all requirements.`, validationErr.Error())
}

// Helper function to create a client with a provider name
func NewClientWithProvider(providerName string, config ClientConfig) (*Client, error) {
	prov, err := provider.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return NewClient(prov, config), nil
}
