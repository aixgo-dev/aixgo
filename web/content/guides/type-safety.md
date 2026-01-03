---
title: 'Type Safety & LLM Integration'
description: "Leverage Go's type system for compile-time guarantees in LLM interactions and tool definitions."
breadcrumb: 'Core Concepts'
category: 'Core Concepts'
weight: 6
---

One of Aixgo's most powerful advantages over Python frameworks is compile-time type safety. This guide shows how Go's type system catches errors before deployment and ensures
reliable LLM interactions.

## The Runtime Error Problem

Python frameworks discover type errors in production:

```python
# Python - Runtime error (discovered in production)
agent = Agent(
    name="analyzer",
    model=123,  # Should be string, but Python allows it
    temperature="high"  # Should be float, but Python allows it
)

# Error only appears when code runs in production
# TypeError: expected str, got int
```

This leads to:

- Production incidents
- Difficult debugging
- Customer-facing errors
- Wasted time and resources

## Aixgo's Solution: Compile-Time Safety

Go catches these errors before you deploy:

```go
// Go - Compile error (caught before deployment)
agent := aixgo.NewAgent(
    aixgo.WithName("analyzer"),
    aixgo.WithModel(123),  // ❌ Compile error: expected string, got int
    aixgo.WithTemperature("high"),  // ❌ Compile error: expected float64, got string
)

// Code won't compile until fixed
```

Your IDE tells you what's broken before your customers do.

## Type-Safe Agent Configuration

### Strong Typing for Agent Options

```go
package main

import "github.com/aixgo-dev/aixgo"

func main() {
    // All types are checked at compile time
    agent := aixgo.NewAgent(
        aixgo.WithName("data-analyzer"),           // string
        aixgo.WithRole(aixgo.RoleReAct),           // enum
        aixgo.WithModel("gpt-4-turbo"),              // string
        aixgo.WithTemperature(0.7),                // float64
        aixgo.WithMaxTokens(1000),                 // int
        aixgo.WithTimeout(30 * time.Second),       // time.Duration
    )

    // Won't compile if types are wrong
    // aixgo.WithTemperature("0.7")  // ❌ Compile error
    // aixgo.WithMaxTokens(1000.5)   // ❌ Compile error
}
```

### Enum-Based Role Safety

```go
// Roles are compile-time constants
const (
    RoleProducer   aixgo.AgentRole = "producer"
    RoleReAct      aixgo.AgentRole = "react"
    RoleLogger     aixgo.AgentRole = "logger"
    RoleClassifier aixgo.AgentRole = "classifier"
    RoleAggregator aixgo.AgentRole = "aggregator"
    RolePlanner    aixgo.AgentRole = "planner"
)

// Type-safe role assignment
agent := aixgo.NewAgent(
    aixgo.WithRole(aixgo.RoleReAct),       // ✅ Valid
    aixgo.WithRole(aixgo.RoleClassifier),  // ✅ Valid
    // aixgo.WithRole("analyser"),         // ❌ Compile error (typo)
)
```

## Type-Safe Tool Definitions

ReAct agents use tools to interact with external systems. Aixgo enforces type safety for tool schemas.

### Defining Tools with Struct Types

```go
package main

import "github.com/aixgo-dev/aixgo/tools"

// Define tool input as a struct
type DatabaseQueryInput struct {
    Query   string   `json:"query" required:"true"`
    Limit   int      `json:"limit" required:"false"`
    Filters []string `json:"filters" required:"false"`
}

// Define tool output as a struct
type DatabaseQueryOutput struct {
    Results []map[string]interface{} `json:"results"`
    Count   int                      `json:"count"`
}

// Implement the tool with type-safe inputs/outputs
func queryDatabase(input DatabaseQueryInput) (DatabaseQueryOutput, error) {
    // Implementation here
    // Type system guarantees input structure
    query := input.Query  // Guaranteed to be string
    limit := input.Limit  // Guaranteed to be int

    results, err := db.Query(query, limit)
    if err != nil {
        return DatabaseQueryOutput{}, err
    }

    return DatabaseQueryOutput{
        Results: results,
        Count:   len(results),
    }, nil
}

func main() {
    // Register tool with type-safe schema
    tool := tools.NewTool(
        "query_database",
        "Query the database with filters",
        queryDatabase,  // Type-checked at compile time
    )

    agent := aixgo.NewAgent(
        aixgo.WithName("analyst"),
        aixgo.WithRole(aixgo.RoleReAct),
        aixgo.WithTools(tool),
    )
}
```

### Automatic Schema Generation

Aixgo generates JSON schemas from Go structs automatically:

```go
type SearchInput struct {
    Query      string   `json:"query" required:"true" description:"Search query"`
    MaxResults int      `json:"max_results" required:"false" description:"Maximum results to return"`
    Categories []string `json:"categories" required:"false" description:"Filter by categories"`
}

// Aixgo generates this JSON schema automatically:
// {
//   "type": "object",
//   "properties": {
//     "query": { "type": "string", "description": "Search query" },
//     "max_results": { "type": "integer", "description": "Maximum results to return" },
//     "categories": {
//       "type": "array",
//       "items": { "type": "string" },
//       "description": "Filter by categories"
//     }
//   },
//   "required": ["query"]
// }
```

No manual schema writing. No drift between code and schema.

## Compile-Time Error Detection

### Configuration Errors

```go
// ❌ Won't compile - wrong type
agent := aixgo.NewAgent(
    aixgo.WithMaxTokens("1000"),  // Expected int, got string
)

// ✅ Compiles - correct type
agent := aixgo.NewAgent(
    aixgo.WithMaxTokens(1000),
)
```

### Missing Required Fields

```go
type ToolInput struct {
    Query string `json:"query" required:"true"`
}

func searchTool(input ToolInput) (string, error) {
    // Input.Query is guaranteed to exist
    // No need for nil checks
    return search(input.Query), nil
}
```

### Type Mismatches in Message Passing

```go
// Define message types
type DataMessage struct {
    Content string
    Score   float64
}

// Agent expects specific message type
func processMessage(msg DataMessage) {
    // msg.Content guaranteed to be string
    // msg.Score guaranteed to be float64
}

// Type system prevents wrong message types
// processMessage("wrong type")  // ❌ Won't compile
```

## Refactoring with Confidence

### Safe Across Large Codebases

```go
// Change a tool input structure
type QueryInput struct {
    Query  string
    Limit  int
    Offset int  // New field added
}

// Compiler finds ALL places that need updating:
// - Tool implementations using QueryInput
// - Tests that create QueryInput
// - Documentation examples (if in Go)

// No hidden runtime errors in production
```

### Interface Changes are Tracked

```go
// Change an interface
type Analyzer interface {
    Analyze(data string) (Result, error)
    // GetConfidence() float64  // New method added
}

// Compiler identifies all implementations that need the new method
// Can't deploy until all implementations are updated
```

## LLM Output Validation

### Structured Output Parsing

```go
// Define expected LLM output structure
type AnalysisResult struct {
    Sentiment  string  `json:"sentiment" validate:"oneof=positive negative neutral"`
    Confidence float64 `json:"confidence" validate:"gte=0,lte=1"`
    Keywords   []string `json:"keywords" validate:"min=1"`
}

// Parse and validate LLM response
func parseAnalysis(llmOutput string) (*AnalysisResult, error) {
    var result AnalysisResult
    if err := json.Unmarshal([]byte(llmOutput), &result); err != nil {
        return nil, fmt.Errorf("invalid JSON: %w", err)
    }

    // Validate struct
    if err := validator.Validate(result); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return &result, nil
}
```

### Retry on Type Errors

```go
// Automatic retry if LLM returns invalid type
agent := aixgo.NewAgent(
    aixgo.WithName("classifier"),
    aixgo.WithRetryOnValidationError(true),
    aixgo.WithMaxRetries(3),
)

// If LLM returns wrong type:
// 1. Aixgo detects type mismatch
// 2. Automatically retries with corrected prompt
// 3. Returns error only after max retries
```

## Python vs Go: Type Safety Comparison

| Scenario               | Python                                 | Aixgo (Go)                              |
| ---------------------- | -------------------------------------- | --------------------------------------- |
| **Configuration typo** | Runtime error in production            | Compile error, caught before deployment |
| **Tool input type**    | Validated at runtime (if you remember) | Validated at compile time (automatic)   |
| **Refactoring**        | Hope you find all usages               | Compiler finds all usages               |
| **LLM output parsing** | Manual validation, easy to miss        | Struct-based, validated automatically   |
| **Schema drift**       | Code and schema can diverge            | Schema generated from code              |

## Best Practices

### 1. Use Structs for Complex Inputs

```go
// ❌ Avoid: untyped maps
func processTool(input map[string]interface{}) {
    query := input["query"].(string)  // Runtime panic if wrong type
}

// ✅ Prefer: typed structs
type ToolInput struct {
    Query string `json:"query"`
}

func processTool(input ToolInput) {
    query := input.Query  // Compile-time guaranteed
}
```

### 2. Define Custom Types for Enums

```go
type Sentiment string

const (
    SentimentPositive Sentiment = "positive"
    SentimentNegative Sentiment = "negative"
    SentimentNeutral  Sentiment = "neutral"
)

type AnalysisOutput struct {
    Sentiment Sentiment `json:"sentiment"`
}

// Type-safe sentiment assignment
output := AnalysisOutput{
    Sentiment: SentimentPositive,  // ✅ Type-checked
    // Sentiment: "postive",       // ❌ Won't compile (typo)
}
```

### 3. Validate at Boundaries

```go
import "github.com/go-playground/validator/v10"

type UserInput struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=150"`
}

func handleInput(input UserInput) error {
    validate := validator.New()
    if err := validate.Struct(input); err != nil {
        return fmt.Errorf("invalid input: %w", err)
    }
    // Proceed with validated input
}
```

### 4. Use Pointer Types for Optional Fields

```go
type AgentConfig struct {
    Name        string   `json:"name" required:"true"`
    Model       string   `json:"model" required:"true"`
    Temperature *float64 `json:"temperature" required:"false"`  // Optional
}

// Can distinguish between "not provided" and "zero value"
if config.Temperature != nil {
    // Temperature was explicitly set
}
```

## Real-World Example: Type-Safe Research Agent

```go
package main

import (
    "github.com/aixgo-dev/aixgo"
    "github.com/aixgo-dev/aixgo/tools"
)

// Define research query input
type ResearchInput struct {
    Topic      string   `json:"topic" required:"true" description:"Research topic"`
    MaxSources int      `json:"max_sources" required:"false" description:"Maximum sources to search"`
    Languages  []string `json:"languages" required:"false" description:"Preferred languages"`
}

// Define research output
type ResearchOutput struct {
    Summary string   `json:"summary"`
    Sources []string `json:"sources"`
    Tags    []string `json:"tags"`
}

// Type-safe research tool
func conductResearch(input ResearchInput) (ResearchOutput, error) {
    // Input types guaranteed at compile time
    topic := input.Topic
    maxSources := input.MaxSources
    if maxSources == 0 {
        maxSources = 10  // Default
    }

    // Implement research logic
    results := performSearch(topic, maxSources)

    return ResearchOutput{
        Summary: summarize(results),
        Sources: extractSources(results),
        Tags:    extractTags(results),
    }, nil
}

func main() {
    // Register tool with type safety
    researchTool := tools.NewTool(
        "conduct_research",
        "Research a topic and return summary with sources",
        conductResearch,
    )

    // Create agent with type-safe configuration
    agent := aixgo.NewAgent(
        aixgo.WithName("research-assistant"),
        aixgo.WithRole(aixgo.RoleReAct),
        aixgo.WithModel("gpt-4-turbo"),
        aixgo.WithTemperature(0.3),
        aixgo.WithTools(researchTool),
    )

    // All types checked at compile time
    // No runtime surprises in production
}
```

## Key Takeaways

1. **Compile-time safety** - Errors caught before deployment, not in production
2. **Automatic schema generation** - No manual JSON schema writing
3. **Refactoring confidence** - Compiler tracks all changes across codebase
4. **Type-safe tools** - LLM tool inputs/outputs validated automatically
5. **IDE support** - Autocomplete, type hints, and error detection

Type safety is not just a nice-to-have—it's a production necessity for reliable AI systems.

## Next Steps

- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Build complex type-safe workflows
- **[Production Deployment](/guides/production-deployment)** - Deploy with confidence
- **[Provider Integration](/guides/provider-integration)** - Integrate LLM providers with type safety
