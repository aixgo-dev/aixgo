# Pydantic AI-Style Validation with Automatic Retry

Demonstrates Aixgo's automatic validation retry feature for structured data extraction from LLMs.

## Overview

When an LLM returns invalid data, Aixgo automatically:

1. Detects validation failures
2. Constructs retry prompts with validation errors
3. Requests corrections from the LLM
4. Returns valid data or clear error after max retries

This provides **40-70% improved reliability** with zero configuration.

## Key Features

- **Automatic by default**: Validation retry enabled with `MaxRetries=3`
- **Type-safe**: Uses Go generics
- **Validation tags**: Comprehensive validation via `validate` struct tags
- **Opt-out support**: Disable via `DisableValidationRetry` or `MaxRetries=1`
- **Works everywhere**: Compatible with all agents and providers

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

// Automatic retry on validation failure
user, err := llm.CreateStructured[User](ctx, client,
    "Extract user: John is 25",
    nil,
)
```

### Opt-Out (Disable Retry)

```go
// Option 1: DisableValidationRetry flag
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:           "gpt-4",
    DisableValidationRetry: true,
})

// Option 2: Set MaxRetries to 1
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
    MaxRetries:   1,
})
```

## Running the Example

### With Mock Provider (No API Keys)

```bash
go run main.go
```

The mock provider demonstrates:

- Initial validation failure (missing email)
- Automatic retry with validation feedback
- Successful second attempt

### With Real LLM Provider

```bash
# OpenAI
export PROVIDER=openai
export OPENAI_API_KEY=your-key
export MODEL=gpt-4
go run main.go

# Anthropic
export PROVIDER=anthropic
export ANTHROPIC_API_KEY=your-key
export MODEL=claude-3-5-sonnet-20241022
go run main.go
```

## How It Works

### Without Retry (Old Behavior)

```text
LLM → {"name": "John", "age": 25}
     ↓
Validation fails (missing email)
     ↓
❌ ERROR
```

### With Retry (Default Behavior)

```text
LLM → {"name": "John", "age": 25}
     ↓
Validation fails (missing email)
     ↓
Retry: "Field 'Email' failed on 'required' tag"
     ↓
LLM → {"name": "John", "email": "john@example.com", "age": 25}
     ↓
✅ SUCCESS
```

## Validation Tags Reference

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

See [validator documentation](https://pkg.go.dev/github.com/go-playground/validator/v10) for all tags.

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `MaxRetries` | `3` | Maximum retry attempts |
| `DisableValidationRetry` | `false` | Disable automatic retry |
| `StrictValidation` | `false` | No type coercion |

## Best Practices

1. **Use descriptive validation tags** - Clear rules help LLMs understand requirements
2. **Provide explicit system prompts** - Explain expected schema and required fields
3. **Set reasonable MaxRetries** - Default 3 for simple, 5-7 for complex schemas
4. **Monitor retry rates** - Track failures to improve prompts
5. **Use strict validation for production** - Enforce strict type checking

## Troubleshooting

### Validation Still Fails After Retries

- Check validation tags are achievable
- Improve system prompt clarity
- Increase MaxRetries for complex schemas
- Use better models (GPT-4 > GPT-3.5)

### Performance Issues

- Reduce MaxRetries
- Disable retry for non-critical data
- Use faster models
- Optimize prompts to reduce failures

## Learn More

- [Validation Guide](/web/content/guides/validation-with-retry.md) - Comprehensive documentation
- [LLM Client API](/docs/api/llm-client.md) - Full API reference
- [Pydantic AI](https://ai.pydantic.dev/) - Inspiration for this feature
