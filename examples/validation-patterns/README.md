# Validation Patterns Example

Demonstrates comprehensive validation capabilities including empty array validation, enum constraints, range validation, cross-field validation, and nested struct validation.

## Overview

Aixgo provides two complementary validation mechanisms:

1. **Struct Tags**: Built-in validators (`required`, `min`, `max`, `email`, `url`, `oneof`, etc.)
2. **Validatable Interface**: Custom validation logic for complex scenarios

## Quick Start

```bash
# Run with mock provider
go run main.go

# Run with OpenAI
export OPENAI_API_KEY=your_key
export PROVIDER=openai
export MODEL=gpt-4
go run main.go
```

## Empty Array Validation - The 10-Line Solution

```go
type DataCollection struct {
    Items []string `json:"items" validate:"required"`
}

// Implement Validatable interface
func (d DataCollection) Validate() error {
    if len(d.Items) == 0 {
        return fmt.Errorf("items array cannot be empty")
    }
    return nil
}
```

The framework automatically:

1. Calls `Validate()` after unmarshaling
2. Retries the LLM with validation error feedback
3. Continues until validation passes or max retries reached

## Validation Patterns

### 1. Empty Array Validation (Validatable Interface)

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
```

**How it works**: LLM generates → Framework unmarshals → Calls `Validate()` → If fail, retry with error message

### 2. Enum Validation (Struct Tags)

```go
type OrderStatus struct {
    Status string `json:"status" validate:"required,oneof=pending processing shipped delivered cancelled"`
}
```

The `oneof` tag ensures status is one of the allowed values.

### 3. Range Validation (Struct Tags)

```go
type UserProfile struct {
    Name  string `json:"name" validate:"required,min=1,max=100"`
    Age   int    `json:"age" validate:"required,min=1,max=150"`
    Email string `json:"email" validate:"required,email"`
}
```

### 4. Cross-Field Validation (Validatable Interface)

```go
type DateRange struct {
    StartDate string `json:"start_date" validate:"required"`
    EndDate   string `json:"end_date" validate:"required"`
}

func (d DateRange) Validate() error {
    if d.StartDate > d.EndDate {
        return fmt.Errorf("start_date must be before or equal to end_date")
    }
    return nil
}
```

### 5. Nested Struct Validation

```go
type Address struct {
    Street  string `json:"street" validate:"required,min=1"`
    City    string `json:"city" validate:"required,min=1"`
    ZipCode string `json:"zip_code" validate:"required,len=5"`
}

type Customer struct {
    Name    string   `json:"name" validate:"required,min=1,max=100"`
    Address Address  `json:"address" validate:"required"`
    Tags    []string `json:"tags"`
}

func (c Customer) Validate() error {
    if len(c.Tags) == 0 {
        return fmt.Errorf("customer must have at least one tag")
    }
    return nil
}
```

## When to Use Each Approach

### Use Struct Tags When:

- Validating single field constraints
- Using built-in validators (email, url, uuid)
- Simple, declarative validation

### Use Validatable Interface When:

- Validating empty arrays/slices
- Cross-field validation
- Complex business logic
- Custom validation rules

### Combine Both:

```go
type Product struct {
    Name   string   `json:"name" validate:"required,min=1"`  // Struct tag
    Price  float64  `json:"price" validate:"required,gt=0"`  // Struct tag
    Tags   []string `json:"tags" validate:"required"`        // Struct tag
}

func (p Product) Validate() error {  // Validatable interface
    if len(p.Tags) == 0 {
        return fmt.Errorf("product must have at least one tag")
    }
    return nil
}
```

## Automatic Retry Flow

When validation fails:

1. **Capture error**: e.g., "items array cannot be empty"
2. **Add to context**: Send error message to LLM
3. **Retry request**: LLM sees mistake and corrects
4. **Repeat until success**: Up to `MaxRetries` attempts (default: 3)

### Example:

```text
Attempt 1:
  LLM Response: {"items": []}
  Validation: FAIL - "items array cannot be empty"

Attempt 2:
  LLM sees error
  LLM Response: {"items": ["Go", "Python", "TypeScript"]}
  Validation: SUCCESS
```

## Available Struct Tag Validators

| Tag | Description | Example |
|-----|-------------|---------|
| `required` | Non-zero value required | `validate:"required"` |
| `min` | Minimum value/length | `validate:"min=1"` |
| `max` | Maximum value/length | `validate:"max=100"` |
| `len` | Exact length | `validate:"len=5"` |
| `gt` | Greater than | `validate:"gt=0"` |
| `gte` | Greater than or equal | `validate:"gte=0"` |
| `oneof` | One of specified values | `validate:"oneof=red blue green"` |
| `email` | Valid email format | `validate:"email"` |
| `url` | Valid URL format | `validate:"url"` |
| `uuid` | Valid UUID format | `validate:"uuid"` |

## Learn More

- [Pydantic-Style Validation Example](../pydantic-style-validation/) - More validation patterns
- [Validation Guide](/web/content/guides/validation-with-retry.md) - Full documentation
- [Structured Output Guide](/docs/structured-output.md) - Best practices
