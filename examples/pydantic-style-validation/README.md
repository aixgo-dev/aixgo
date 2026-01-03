# Pydantic AI-Style Validation with Automatic Retry

Demonstrates Aixgo's automatic validation retry feature for reliable structured data extraction from LLMs.

## Quick Start

```bash
cd examples/pydantic-style-validation
export OPENAI_API_KEY="your-key"
go run main.go
```

## Overview

When LLMs return invalid structured data, Aixgo automatically:

- Detects validation failures using Go struct tags
- Constructs retry prompts with clear error messages
- Requests corrections from the LLM
- Returns valid data or clear error after max retries

**Result: 40-70% improved reliability** with zero configuration.

**Comprehensive Guide**: See [Validation with Retry](https://aixgo.dev/guides/validation-with-retry/) for complete documentation, advanced patterns, and best practices.

## Key Features

- **Automatic by default** - Enabled with `MaxRetries=3` (configurable)
- **Type-safe** - Uses Go generics for structured outputs
- **Validation tags** - Comprehensive rules via `validate` struct tags
- **Opt-out support** - Disable via `DisableValidationRetry` or `MaxRetries=1`

## Example

```go
type User struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"required,gte=0"`
}

// Automatic retry on validation failure
user, err := llm.CreateStructured[User](ctx, client, prompt, nil)
```

## Files

- `main.go` - Mock and real LLM provider examples

## Related

- [LLM Client API](../../docs/api/llm-client.md) - Full API reference
