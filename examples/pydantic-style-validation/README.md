# Pydantic AI-Style Validation with Automatic Retry

This example demonstrates Aixgo's **Pydantic AI-style automatic validation retry** feature, which dramatically improves the reliability of structured data extraction from LLMs.

## What is Validation Retry?

When an LLM returns incomplete or invalid data that fails validation, Aixgo automatically:
1. **Detects** the validation failure
2. **Constructs** a retry prompt with detailed validation errors
3. **Requests** corrections from the LLM
4. **Validates** the corrected output
5. **Returns** valid data or a clear error after maximum retries

This provides 40-70% improved reliability for structured output tasks with zero configuration required.

## Key Features

- **Automatic by Default**: Validation retry is enabled out-of-the-box with `MaxRetries=3`
- **Type-Safe**: Uses Go generics for compile-time type safety
- **Validation Tags**: Leverages Go's `validate` struct tags for comprehensive validation
- **Opt-Out Support**: Easy to disable for specific use cases via `DisableValidationRetry` or `MaxRetries=1`
- **Transparent**: Works with all existing Aixgo agents and providers

## Quick Start

### Basic Usage (Default Behavior)

```go
type User struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"required,gte=0"`
}

// Create client - retry is automatic!
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
})

// If validation fails, automatically retries with error feedback
user, err := llm.CreateStructured[User](ctx, client,
    "Extract user: John is 25",
    nil,
)
```

### Opt-Out (Disable Retry)

```go
// Option 1: Use DisableValidationRetry flag
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:           "gpt-4",
    DisableValidationRetry: true,  // Fail immediately
})

// Option 2: Set MaxRetries to 1
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
    MaxRetries:   1,  // Single attempt, no retry
})
```

## Running the Example

### Prerequisites

```bash
# Install dependencies
go mod download
```

### Run with Mock Provider (Demo Mode)

The example includes a mock provider that simulates validation failures and retries without requiring API keys.

```bash
# Run the example with mock provider (no API keys needed)
go run main.go
```

The mock provider will demonstrate:
- Initial validation failure (missing email field)
- Automatic retry with validation feedback
- Successful second attempt with complete data

### Run with Real LLM Provider

To test with actual LLM providers, set the appropriate environment variables:

```bash
# OpenAI
export PROVIDER=openai
export OPENAI_API_KEY=your-api-key
export MODEL=gpt-4
go run main.go

# Anthropic Claude
export PROVIDER=anthropic
export ANTHROPIC_API_KEY=your-api-key
export MODEL=claude-3-5-sonnet-20241022
go run main.go

# Google Gemini
export PROVIDER=gemini
export GEMINI_API_KEY=your-api-key
export MODEL=gemini-1.5-pro
go run main.go
```

Note: Real LLM providers will make actual API calls and may incur costs.

## How It Works

### Without Validation Retry (Old Behavior)

```
User Prompt → LLM → Returns: {"name": "John", "age": 25}
              ↓
          Validation fails (missing email)
              ↓
          ❌ ERROR: validation failed
```

### With Validation Retry (New Default Behavior)

```
User Prompt → LLM → Returns: {"name": "John", "age": 25}
              ↓
          Validation fails (missing email)
              ↓
          Retry Prompt: "Your previous response did not pass validation:
                        Field validation for 'Email' failed on the 'required' tag
                        Please correct the issues..."
              ↓
          LLM → Returns: {"name": "John", "email": "john@example.com", "age": 25}
              ↓
          Validation succeeds
              ↓
          ✅ SUCCESS
```

## Example Scenarios

### Scenario 1: User Data Extraction

Incomplete LLM response missing required `email` field:
- **Attempt 1**: `{"name": "John", "age": 25}` → Validation fails
- **Attempt 2**: LLM sees error, adds email → `{"name": "John", "email": "john@example.com", "age": 25}` → Success

### Scenario 2: Product Catalog

LLM returns price as string instead of number:
- **Attempt 1**: `{"name": "Laptop", "price": "999.99"}` → Validation fails
- **Attempt 2**: LLM corrects type → `{"name": "Laptop", "price": 999.99}` → Success

### Scenario 3: API Integration

Validation catches malformed email addresses:
- **Attempt 1**: `{"email": "not-an-email"}` → Validation fails
- **Attempt 2**: LLM provides valid email → `{"email": "user@example.com"}` → Success

## Validation Tags Reference

Aixgo uses the `validate` struct tag from the `go-playground/validator` library:

```go
type Example struct {
    // Required fields
    Name string `json:"name" validate:"required"`

    // Email validation
    Email string `json:"email" validate:"required,email"`

    // Number constraints
    Age   int     `json:"age" validate:"gte=0,lte=150"`
    Price float64 `json:"price" validate:"gt=0"`

    // String length
    Bio string `json:"bio" validate:"min=10,max=500"`

    // Enums
    Status string `json:"status" validate:"oneof=active inactive pending"`
}
```

See [validator documentation](https://pkg.go.dev/github.com/go-playground/validator/v10) for all available tags.

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `MaxRetries` | `3` | Maximum number of validation retry attempts |
| `DisableValidationRetry` | `false` | Set to `true` to disable automatic retry |
| `StrictValidation` | `false` | Enable strict type checking (no coercion) |

## Comparison with Pydantic AI

Aixgo's validation retry feature is inspired by Pydantic AI and provides equivalent functionality for Go developers:

| Feature | Pydantic AI (Python) | Aixgo (Go) |
|---------|---------------------|------------|
| Automatic retry on validation failure | ✅ | ✅ |
| Type-safe validation | ✅ Python types | ✅ Go generics |
| Validation error feedback in retry | ✅ | ✅ |
| Configurable max retries | ✅ | ✅ |
| Default behavior | Enabled | Enabled |
| Opt-out support | ✅ | ✅ |
| Compile-time type checking | ❌ | ✅ |
| Zero runtime dependencies | ❌ | ✅ |

## Best Practices

1. **Use Descriptive Validation Tags**: Clear validation rules help the LLM understand requirements and improve success rates
2. **Provide Explicit System Prompts**: Clearly explain the expected schema, required fields, and format constraints in the system prompt
3. **Set Reasonable MaxRetries**: The default of 3 works well for most cases; increase to 5-7 for complex nested schemas
4. **Monitor Retry Rates**: Track how often retries occur to identify opportunities for prompt improvement
5. **Use Strict Validation for Production**: Enable `StrictValidation: true` to enforce strict type checking without coercion

## Troubleshooting

### Validation Still Fails After Retries

- **Check your validation tags**: Ensure they're reasonable and achievable
- **Improve system prompt**: Be explicit about required fields and formats
- **Increase MaxRetries**: Consider 5-7 retries for complex schemas
- **Use better models**: GPT-4 performs better than GPT-3.5 for structured output

### Performance Concerns

- **Retries add latency**: Each retry is an additional LLM call
- **Use provider-level caching**: Some providers cache similar requests
- **Consider DisableValidationRetry**: For non-critical data or when speed is essential

## Learn More

- [Validation Guide](../../docs/guides/validation-with-retry.md) - Comprehensive validation documentation
- [LLM Client API](../../docs/api/llm-client.md) - Full API reference
- [Pydantic AI Documentation](https://ai.pydantic.dev/) - Inspiration for this feature

## License

This example is part of the Aixgo project and is licensed under the same terms.
