# Validation with Automatic Retry

Aixgo provides **Pydantic AI-style automatic validation retry**, a powerful feature that dramatically improves the reliability of structured data extraction from LLMs.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [How It Works](#how-it-works)
- [Configuration](#configuration)
- [Use Cases](#use-cases)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [API Reference](#api-reference)

## Overview

### The Problem

LLMs are powerful but imperfect. When extracting structured data, they often:
- Omit required fields
- Return incorrect data types
- Violate validation constraints
- Produce malformed output

Traditional approaches fail immediately on validation errors, requiring manual retry logic and increasing development complexity.

### The Solution

Aixgo's validation retry feature automatically:
1. **Detects** validation failures
2. **Constructs** retry prompts with validation errors
3. **Requests** corrections from the LLM
4. **Validates** the corrected output
5. **Returns** valid data or a clear error after max retries

This is **enabled by default** with `MaxRetries=3`, providing Pydantic AI-style reliability out-of-the-box.

### Benefits

- **40-70% improvement** in structured output reliability
- **Zero configuration** required (works automatically)
- **Type-safe** using Go generics
- **Opt-out support** for performance-critical scenarios
- **Works with all agents** and providers

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aixgo-dev/aixgo/internal/llm"
    "github.com/aixgo-dev/aixgo/internal/llm/provider"
)

type User struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=150"`
}

func main() {
    ctx := context.Background()

    // Get provider
    prov, err := provider.Get("openai")
    if err != nil {
        log.Fatalf("Failed to get provider: %v", err)
    }

    // Create client - validation retry is AUTOMATIC!
    client := llm.NewClient(prov, llm.ClientConfig{
        DefaultModel: "gpt-4",
        // MaxRetries defaults to 3 - no configuration needed
    })

    // Extract data - automatic retry on validation failure
    user, err := llm.CreateStructured[User](
        ctx,
        client,
        "Extract user: John Smith is 30",
        nil,
    )

    if err != nil {
        log.Fatalf("Failed after retries: %v", err)
    }

    fmt.Printf("Success: %+v\n", user)
}
```

### What Happens Behind the Scenes

When you call `CreateStructured`, Aixgo automatically handles validation failures:

**Attempt 1**: LLM returns incomplete data
```json
{"name": "John Smith", "age": 30}
```
Validation fails: missing required field `email`

**Automatic Retry**: Aixgo sends validation feedback to the LLM
```text
Your previous response did not pass validation:

Field validation for 'Email' failed on the 'required' tag

Please correct the issues and provide a valid response that matches all requirements.
```

**Attempt 2**: LLM corrects the issue
```json
{"name": "John Smith", "email": "john.smith@example.com", "age": 30}
```
Validation succeeds - result returned to your application

## How It Works

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Your Code: llm.CreateStructured[T](...)                     │
└────────────┬────────────────────────────────────────────────┘
             │
             v
┌─────────────────────────────────────────────────────────────┐
│ LLM Client Layer (internal/llm/client.go)                   │
│ - Manages retry loop (up to MaxRetries attempts)            │
│ - Constructs feedback messages                              │
└────────────┬────────────────────────────────────────────────┘
             │
             v
┌─────────────────────────────────────────────────────────────┐
│ Provider Layer (internal/llm/provider/)                     │
│ - Calls LLM API                                              │
│ - Returns structured response                                │
└────────────┬────────────────────────────────────────────────┘
             │
             v
┌─────────────────────────────────────────────────────────────┐
│ Validator Layer (internal/llm/validator/)                   │
│ - Validates struct tags                                      │
│ - Returns validation errors if any                           │
└─────────────────────────────────────────────────────────────┘
```

### Retry Loop Logic

```go
for attempt := 0; attempt < maxRetries; attempt++ {
    // 1. Call LLM
    response := provider.CreateStructured(ctx, messages)

    // 2. Validate response
    result, validationErr := validator.Validate[T](response.Data)

    // 3. Success!
    if validationErr == nil {
        return result, nil
    }

    // 4. Last attempt failed - return error
    if attempt == maxRetries-1 {
        return nil, fmt.Errorf("validation failed after %d attempts: %w",
            maxRetries, validationErr)
    }

    // 5. Construct retry prompt with validation errors
    feedback := formatValidationFeedback(validationErr)
    messages = append(messages,
        Message{Role: "assistant", Content: response.Content},
        Message{Role: "user", Content: feedback},
    )
}
```

## Configuration

### ClientConfig Options

```go
type ClientConfig struct {
    DefaultModel string

    // MaxRetries for validation failures (default: 3)
    // Set to 1 to disable retry
    MaxRetries int

    // DisableValidationRetry disables automatic retry
    // When true, validation errors fail immediately
    DisableValidationRetry bool

    // StrictValidation enables strict type checking
    // No type coercion (e.g., "42" won't become int 42)
    StrictValidation bool
}
```

### Default Behavior

```go
// Default: MaxRetries=3, retry enabled
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
})
// ✅ Automatic retry with up to 3 attempts
```

### Disable Retry (Opt-Out)

#### Option 1: Use DisableValidationRetry Flag

```go
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:           "gpt-4",
    DisableValidationRetry: true,  // Fail immediately on validation error
})
```

#### Option 2: Set MaxRetries to 1

```go
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
    MaxRetries:   1,  // Single attempt, no retry
})
```

### Custom Retry Count

```go
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
    MaxRetries:   5,  // Allow up to 5 attempts for complex schemas
})
```

## Use Cases

### Use Case 1: User Data Extraction

```go
type User struct {
    Name     string `json:"name" validate:"required,min=1,max=100"`
    Email    string `json:"email" validate:"required,email"`
    Phone    string `json:"phone" validate:"omitempty,e164"`  // Optional, but must be valid E.164 if present
    Age      int    `json:"age" validate:"required,gte=0,lte=150"`
    Country  string `json:"country" validate:"required,iso3166_1_alpha2"`  // ISO country code
}

// LLM might initially miss fields or use invalid formats
// Auto-retry ensures all required fields are present and valid
user, err := llm.CreateStructured[User](ctx, client, prompt, nil)
```

### Use Case 2: API Response Parsing

```go
type APIResponse struct {
    Status   string   `json:"status" validate:"required,oneof=success error pending"`
    Message  string   `json:"message" validate:"required,min=1"`
    Code     int      `json:"code" validate:"required,gte=100,lte=599"`  // HTTP status codes
    Data     any      `json:"data"`
    Metadata Metadata `json:"metadata" validate:"required"`
}

type Metadata struct {
    RequestID string `json:"request_id" validate:"required,uuid"`
    Timestamp int64  `json:"timestamp" validate:"required,gt=0"`
}

// Complex nested validation with auto-retry
// If the LLM omits metadata or uses invalid values, it will be retried
response, err := llm.CreateStructured[APIResponse](ctx, client, prompt, nil)
```

### Use Case 3: Product Catalog

```go
type Product struct {
    SKU         string   `json:"sku" validate:"required,alphanum,len=8"`
    Name        string   `json:"name" validate:"required,min=1,max=200"`
    Description string   `json:"description" validate:"required,min=20,max=1000"`
    Price       float64  `json:"price" validate:"required,gt=0"`
    Currency    string   `json:"currency" validate:"required,iso4217"`  // ISO currency code
    Categories  []string `json:"categories" validate:"required,min=1,dive,required"`
    InStock     bool     `json:"in_stock"`
}

// Auto-retry helps with complex constraints and nested validation
product, err := llm.CreateStructured[Product](ctx, client, prompt, nil)
```

### Use Case 4: Form Data Extraction

```go
type ContactForm struct {
    FullName    string `json:"full_name" validate:"required,min=2,max=100"`
    Email       string `json:"email" validate:"required,email"`
    Phone       string `json:"phone" validate:"required,e164"`
    Company     string `json:"company" validate:"required,min=1,max=200"`
    Message     string `json:"message" validate:"required,min=10,max=2000"`
    ConsentGiven bool  `json:"consent_given" validate:"eq=true"`  // Must be true
}

// Ensures all form validation rules are met
form, err := llm.CreateStructured[ContactForm](ctx, client, prompt, nil)
```

## Best Practices

### 1. Use Descriptive Validation Tags

**Good:**
```go
type User struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"required,gte=0,lte=150"`
}
```

**Better:**
```go
// Also provide clear field documentation
type User struct {
    // Email must be a valid email address (required)
    Email string `json:"email" validate:"required,email"`

    // Age must be between 0 and 150 (required)
    Age int `json:"age" validate:"required,gte=0,lte=150"`
}
```

### 2. Provide Explicit System Prompts

```go
result, err := llm.CreateStructured[User](ctx, client, userPrompt, &llm.CreateOptions{
    SystemPrompt: `You are a data extraction assistant.

Extract user information and return it as JSON with these REQUIRED fields:
- name: full name (string, 1-100 characters)
- email: valid email address (string, RFC 5322 format)
- age: age in years (integer, 0-150)
- city: city of residence (string, 1-100 characters)

All fields are REQUIRED. If information is missing, make reasonable assumptions or ask for clarification.`,
})
```

### 3. Set Reasonable MaxRetries

```go
// Simple schema: 3 retries (default)
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
    MaxRetries:   3,  // Good for most cases
})

// Complex nested schema: more retries
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel: "gpt-4",
    MaxRetries:   5,  // More attempts for complex validation
})

// Performance-critical: disable retry
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:           "gpt-4",
    DisableValidationRetry: true,  // Speed over reliability
})
```

### 4. Monitor Retry Rates

Track how often retries occur to identify prompt improvements:

```go
// Custom wrapper with metrics
func CreateStructuredWithMetrics[T any](ctx context.Context, client *llm.Client, prompt string) (*T, error) {
    startTime := time.Now()
    result, err := llm.CreateStructured[T](ctx, client, prompt, nil)
    duration := time.Since(startTime)

    // Log metrics
    if err != nil && strings.Contains(err.Error(), "after") {
        // Validation failed after retries - improve prompt
        log.Printf("RETRY_EXHAUSTED: %s (took %v)", prompt, duration)
    }

    return result, err
}
```

### 5. Use Strict Validation for Production

```go
// Development: allow type coercion
devClient := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:     "gpt-4",
    StrictValidation: false,  // "42" → int(42)
})

// Production: strict types
prodClient := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:     "gpt-4",
    StrictValidation: true,  // "42" → validation error
})
```

## Troubleshooting

### Validation Still Fails After Retries

**Problem**: Error message shows "validation failed after 3 attempts"

**Solutions**:

1. **Check validation tags are achievable**
   ```go
   // Bad: Too restrictive
   Email string `validate:"required,email,endswith=@company.com"`

   // Good: Reasonable
   Email string `validate:"required,email"`
   ```

2. **Improve system prompt clarity**
   ```go
   // Bad: Vague
   SystemPrompt: "Extract user data"

   // Good: Explicit
   SystemPrompt: `Extract user data as JSON with:
   - name: string (required)
   - email: valid email (required)
   - age: number 0-150 (required)`
   ```

3. **Increase MaxRetries**
   ```go
   MaxRetries: 7,  // More attempts for complex schemas
   ```

4. **Use better models**
   ```go
   DefaultModel: "gpt-4",  // Better than gpt-3.5-turbo
   ```

### Performance Issues

**Problem**: Requests are too slow

**Solutions**:

1. **Reduce MaxRetries**
   ```go
   MaxRetries: 2,  // Faster but less reliable
   ```

2. **Disable retry for non-critical data**
   ```go
   DisableValidationRetry: true,  // Speed over reliability
   ```

3. **Use faster models**
   ```go
   DefaultModel: "gpt-3.5-turbo",  // Faster but less accurate
   ```

4. **Optimize prompts to reduce failures**
   - Provide examples in system prompt
   - Use few-shot prompting
   - Simplify schema complexity

### Type Coercion Issues

**Problem**: Strict validation fails on type mismatches

**Solution**: Disable strict mode for development

```go
client := llm.NewClient(provider, llm.ClientConfig{
    DefaultModel:     "gpt-4",
    StrictValidation: false,  // Allow "42" → int(42)
})
```

## API Reference

### CreateStructured

```go
func CreateStructured[T any](
    ctx context.Context,
    client *Client,
    prompt string,
    options *CreateOptions,
) (*T, error)
```

**Generic Type Parameter:**
- `T`: Target struct type with `json` and `validate` tags

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `client`: LLM client with validation retry configuration
- `prompt`: User prompt for data extraction
- `options`: Optional configuration (system prompt, model override, etc.)

**Returns:**
- `*T`: Pointer to validated struct instance
- `error`: Error if validation fails after all retries

**Example:**
```go
user, err := llm.CreateStructured[User](ctx, client, "Extract user: John", nil)
```

### CreateList

```go
func CreateList[T any](
    ctx context.Context,
    client *Client,
    prompt string,
    options *CreateOptions,
) ([]*T, error)
```

**Generic Type Parameter:**
- `T`: Target struct type for list items

**Parameters:**
- Same as `CreateStructured`

**Returns:**
- `[]*T`: Slice of validated struct pointers
- `error`: Error if validation fails after all retries

**Example:**
```go
users, err := llm.CreateList[User](ctx, client, "Extract all users", nil)
```

### ClientConfig

```go
type ClientConfig struct {
    DefaultModel           string
    DefaultTemperature     float64
    MaxRetries             int   // Default: 3
    DisableValidationRetry bool  // Default: false
    StrictValidation       bool  // Default: false
}
```

## Related Documentation

- [Validation Tags Reference](https://pkg.go.dev/github.com/go-playground/validator/v10)
- [LLM Client API](../api/llm-client.md)
- [Pydantic AI Inspiration](https://ai.pydantic.dev/)
- [Example: Pydantic-Style Validation](../../examples/pydantic-style-validation/)

## See Also

- [Structured Output Guide](./structured-output.md)
- [Error Handling](./error-handling.md)
- [Performance Optimization](./performance.md)
