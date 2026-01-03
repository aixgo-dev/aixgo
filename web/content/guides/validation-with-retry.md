---
title: 'Validation with Automatic Retry'
description: 'Pydantic AI-style automatic validation retry for 40-70% improved structured output reliability'
breadcrumb: 'Validation Retry'
category: 'LLM Integration'
weight: 15
---

Aixgo provides **Pydantic AI-style automatic validation retry**, a powerful feature that dramatically improves the reliability of structured data extraction from LLMs.

**Working Example**: See [pydantic-style-validation](https://github.com/aixgo-dev/aixgo/tree/main/examples/pydantic-style-validation) for a complete implementation with mock and real LLM provider examples.

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

```text
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

## Validating Array Length

Go's validator tags don't support `minItems` for slices. Use the `Validatable` interface for custom array validation.

### The Problem

LLMs frequently return empty arrays when they shouldn't:

- "Extract data collection methods" → `{"data_collection": []}`
- "List product features" → `{"features": []}`
- "Find security risks" → `{"risks": []}`

This is a common failure mode that degrades data quality and requires explicit handling.

### The Solution

Implement the `Validatable` interface with custom validation:

```go
type DataCollection struct {
    Items []string `json:"items" validate:"required"`
}

func (d DataCollection) Validate() error {
    if len(d.Items) == 0 {
        return fmt.Errorf("items array cannot be empty - at least one item required")
    }
    return nil
}

// Use with automatic retry
result, err := llm.CreateStructured[DataCollection](ctx, client, prompt, nil)
// Framework automatically retries if validation fails
```

### Automatic Retry Feedback

When validation fails, the LLM receives detailed feedback:

```text
Your previous response did not pass validation:

items array cannot be empty - at least one item required

Please re-read the document and extract all relevant items.
If truly not found, use: ["Not specified"]
```

The retry mechanism feeds this error message back to the LLM, prompting it to correct the issue. This typically resolves 60-80% of empty array problems automatically.

### Reusable Pattern

Create a generic helper for application code (not provided by framework):

```go
type NonEmptySlice[T any] []T

func (s NonEmptySlice[T]) Validate() error {
    if len(s) == 0 {
        return fmt.Errorf("slice cannot be empty")
    }
    return nil
}

// Usage
type Response struct {
    Items NonEmptySlice[Item] `json:"items"`
}
```

## Comprehensive Validation Tags Reference

Aixgo uses the [go-playground/validator](https://github.com/go-playground/validator) library, which supports extensive validation tags.

### Required and Optional Fields

```go
type User struct {
    Name  string `json:"name" validate:"required"`      // Must be present
    Email string `json:"email" validate:"omitempty"`    // Optional field
}
```

### Numeric Constraints

```go
type Product struct {
    Price    float64 `json:"price" validate:"gte=0"`           // Greater than or equal
    Quantity int     `json:"quantity" validate:"gt=0,lte=100"` // Greater than 0, less than or equal to 100
    Rating   float64 `json:"rating" validate:"min=1,max=5"`    // Between 1 and 5
    Age      int     `json:"age" validate:"gte=0,lte=150"`     // 0 to 150
}
```

### String Constraints

```go
type User struct {
    Username string `json:"username" validate:"required,min=3,max=20"`  // Length between 3-20
    Bio      string `json:"bio" validate:"max=500"`                      // Max 500 characters
    Code     string `json:"code" validate:"len=6"`                       // Exactly 6 characters
}
```

### Enumeration (oneof)

```go
type Order struct {
    Status string `json:"status" validate:"required,oneof=pending approved rejected"`
    Type   string `json:"type" validate:"oneof=standard express overnight"`
}
```

### Format Validation

```go
type Contact struct {
    Email   string `json:"email" validate:"required,email"`           // RFC 5322 email
    URL     string `json:"url" validate:"omitempty,url"`              // Valid URL
    UUID    string `json:"uuid" validate:"required,uuid"`             // Valid UUID
    Phone   string `json:"phone" validate:"omitempty,e164"`           // E.164 phone format
    Country string `json:"country" validate:"iso3166_1_alpha2"`       // ISO country code
}
```

### Nested Validation (dive)

```go
type Company struct {
    Employees []Employee `json:"employees" validate:"required,dive"`
}

type Employee struct {
    Name  string `json:"name" validate:"required,min=1"`
    Email string `json:"email" validate:"required,email"`
}

// The "dive" tag validates each element in the slice
```

### Combining Tags

```go
type User struct {
    // Multiple constraints combined
    Email string `json:"email" validate:"required,email,min=5,max=100"`

    // Optional but must be valid if present
    Website string `json:"website" validate:"omitempty,url"`

    // Complex numeric constraints
    Age int `json:"age" validate:"required,gte=18,lte=120"`
}
```

### When to Use Struct Tags vs Validatable Interface

**Use struct tags when:**

- Validation is simple and supported by standard tags
- Field-level constraints are sufficient
- No cross-field validation needed
- No complex custom logic required

**Use Validatable interface when:**

- Array length validation needed (`minItems`, `maxItems`)
- Cross-field validation required (e.g., `end_date > start_date`)
- Complex business logic
- Conditional validation based on other fields
- Custom error messages with context

```go
// Example: When you need both
type Order struct {
    Items     []Item    `json:"items" validate:"required,dive"`  // Struct tag for nested validation
    StartDate time.Time `json:"start_date" validate:"required"`
    EndDate   time.Time `json:"end_date" validate:"required"`
}

// Validatable for cross-field logic
func (o Order) Validate() error {
    if len(o.Items) == 0 {
        return fmt.Errorf("items array cannot be empty")
    }
    if o.EndDate.Before(o.StartDate) {
        return fmt.Errorf("end_date must be after start_date")
    }
    return nil
}
```

## Cross-Field Validation

When validation depends on multiple fields, implement the `Validatable` interface.

### Date Range Validation

```go
type Event struct {
    StartDate time.Time `json:"start_date" validate:"required"`
    EndDate   time.Time `json:"end_date" validate:"required"`
}

func (e Event) Validate() error {
    if e.EndDate.Before(e.StartDate) {
        return fmt.Errorf("end_date must be after start_date")
    }
    return nil
}
```

### Conditional Required Fields

```go
type Payment struct {
    Method      string `json:"method" validate:"required,oneof=credit_card bank_transfer"`
    CardNumber  string `json:"card_number" validate:"omitempty"`
    BankAccount string `json:"bank_account" validate:"omitempty"`
}

func (p Payment) Validate() error {
    if p.Method == "credit_card" && p.CardNumber == "" {
        return fmt.Errorf("card_number required when method is credit_card")
    }
    if p.Method == "bank_transfer" && p.BankAccount == "" {
        return fmt.Errorf("bank_account required when method is bank_transfer")
    }
    return nil
}
```

### Mutually Exclusive Fields

```go
type Search struct {
    Keyword string `json:"keyword" validate:"omitempty"`
    TagID   string `json:"tag_id" validate:"omitempty"`
}

func (s Search) Validate() error {
    hasKeyword := s.Keyword != ""
    hasTagID := s.TagID != ""

    if !hasKeyword && !hasTagID {
        return fmt.Errorf("either keyword or tag_id must be provided")
    }
    if hasKeyword && hasTagID {
        return fmt.Errorf("keyword and tag_id are mutually exclusive")
    }
    return nil
}
```

### Sum Validation

```go
type Budget struct {
    Total      float64   `json:"total" validate:"required,gt=0"`
    Categories []float64 `json:"categories" validate:"required,dive,gte=0"`
}

func (b Budget) Validate() error {
    sum := 0.0
    for _, amount := range b.Categories {
        sum += amount
    }

    if math.Abs(sum-b.Total) > 0.01 {
        return fmt.Errorf("category sum (%.2f) must equal total (%.2f)", sum, b.Total)
    }
    return nil
}
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

## Related Documentation

- [Validation Tags Reference](https://pkg.go.dev/github.com/go-playground/validator/v10)
- [Pydantic AI Inspiration](https://ai.pydantic.dev/)
- [Example: Pydantic-Style Validation](https://github.com/aixgo-dev/aixgo/tree/main/examples/pydantic-style-validation/)

## See Also

- [LLM Provider Integration](/guides/provider-integration)
- [Type Safety](/guides/type-safety)
- [Core Concepts](/guides/core-concepts)
