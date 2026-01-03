---
title: 'Using Public Interfaces'
description: 'Build custom agents and integrate Aixgo into existing Go applications with the public agent package'
weight: 8
---

# Using Public Interfaces

Aixgo v0.2.2 introduces the `agent` package—a standalone, minimal-dependency package that exports core interfaces for building custom agents. This enables library-style integration into existing Go applications without requiring the full Aixgo framework.

## Overview

The public agent package provides:

- **Agent Interface**: Define custom agent behavior
- **Message Struct**: Standard communication format between agents
- **Runtime Interface**: Coordinate multiple agents with synchronous and asynchronous communication
- **LocalRuntime**: Production-ready single-process runtime implementation

### When to Use Public Interfaces

| Use Case | Approach |
|----------|----------|
| Building standalone AI applications | Full Aixgo framework |
| Integrating agents into existing services | Public agent package |
| Custom runtime implementations | Public agent package |
| Lightweight agent prototypes | Public agent package |
| Multi-provider orchestration with built-in patterns | Full Aixgo framework |

## Installation

Add the agent package to your project:

```bash
go get github.com/aixgo-dev/aixgo/agent
```

The package has minimal dependencies (only `github.com/google/uuid`), making it suitable for projects where dependency management is a concern.

## Core Interfaces

### Agent Interface

All agents must implement the `Agent` interface:

```go
type Agent interface {
    // Name returns the unique identifier for this agent instance
    Name() string

    // Role returns the agent's role type (e.g., "analyzer", "processor")
    Role() string

    // Start initializes the agent and prepares it to receive messages
    // Blocks until context is cancelled or a fatal error occurs
    Start(ctx context.Context) error

    // Execute processes an input message and returns a response synchronously
    Execute(ctx context.Context, input *Message) (*Message, error)

    // Stop gracefully shuts down the agent
    Stop(ctx context.Context) error

    // Ready returns true if the agent is ready to process messages
    Ready() bool
}
```

### Message Struct

Messages are the standard unit of communication:

```go
type Message struct {
    ID        string                 // Unique identifier (auto-generated)
    Type      string                 // Message type for routing
    Payload   string                 // JSON-serialized data
    Timestamp string                 // ISO 8601 creation time
    Metadata  map[string]interface{} // Optional key-value pairs
}
```

### Runtime Interface

The Runtime coordinates agent communication:

```go
type Runtime interface {
    // Registration
    Register(agent Agent) error
    Unregister(name string) error
    Get(name string) (Agent, error)
    List() []string

    // Synchronous communication
    Call(ctx context.Context, target string, input *Message) (*Message, error)
    CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, map[string]error)

    // Asynchronous communication
    Send(target string, msg *Message) error
    Recv(source string) (<-chan *Message, error)
    Broadcast(msg *Message) error

    // Lifecycle
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

## Creating a Custom Agent

Here's a complete example of a custom agent:

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/agent"
)

type AnalyzerAgent struct {
    name  string
    ready bool
}

func NewAnalyzerAgent(name string) *AnalyzerAgent {
    return &AnalyzerAgent{name: name}
}

func (a *AnalyzerAgent) Name() string { return a.name }
func (a *AnalyzerAgent) Role() string { return "analyzer" }
func (a *AnalyzerAgent) Ready() bool  { return a.ready }

func (a *AnalyzerAgent) Start(ctx context.Context) error {
    a.ready = true
    <-ctx.Done() // Block until context cancelled
    return nil
}

func (a *AnalyzerAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Unmarshal the input
    var request struct {
        Text string `json:"text"`
    }
    if err := input.UnmarshalPayload(&request); err != nil {
        return nil, err
    }

    // Process the request
    result := struct {
        WordCount int    `json:"word_count"`
        Status    string `json:"status"`
    }{
        WordCount: len(request.Text),
        Status:    "analyzed",
    }

    return agent.NewMessage("analysis_result", result), nil
}

func (a *AnalyzerAgent) Stop(ctx context.Context) error {
    a.ready = false
    return nil
}
```

## Using the LocalRuntime

The `LocalRuntime` provides single-process agent coordination:

```go
package main

import (
    "context"
    "fmt"
    "github.com/aixgo-dev/aixgo/agent"
)

func main() {
    ctx := context.Background()

    // Create runtime
    rt := agent.NewLocalRuntime()

    // Register agents
    rt.Register(NewAnalyzerAgent("analyzer-1"))
    rt.Register(NewAnalyzerAgent("analyzer-2"))

    // Start runtime (launches all agents)
    go rt.Start(ctx)

    // Create a request message
    input := agent.NewMessage("analyze_request", map[string]string{
        "text": "Hello, Aixgo!",
    })

    // Synchronous call to single agent
    response, err := rt.Call(ctx, "analyzer-1", input)
    if err != nil {
        panic(err)
    }

    var result map[string]interface{}
    response.UnmarshalPayload(&result)
    fmt.Printf("Result: %v\n", result)

    // Parallel call to multiple agents
    results, errors := rt.CallParallel(ctx, []string{"analyzer-1", "analyzer-2"}, input)
    for name, resp := range results {
        fmt.Printf("Agent %s responded\n", name)
    }
    for name, err := range errors {
        fmt.Printf("Agent %s failed: %v\n", name, err)
    }

    // Clean up
    rt.Stop(ctx)
}
```

## Message Patterns

### Creating Messages with Metadata

```go
// Create a message with structured payload
msg := agent.NewMessage("request", map[string]interface{}{
    "action": "analyze",
    "data":   "sample text",
}).WithMetadata("priority", "high").
   WithMetadata("user_id", "user-123").
   WithMetadata("correlation_id", "req-456")

// Access metadata
priority := msg.GetMetadataString("priority", "normal")
```

### Asynchronous Communication

```go
// Send a message without waiting for response
rt.Send("analyzer-1", msg)

// Receive messages from an agent
recvCh, _ := rt.Recv("analyzer-1")
go func() {
    for msg := range recvCh {
        fmt.Printf("Received: %s\n", msg.Type)
    }
}()

// Broadcast to all registered agents
rt.Broadcast(agent.NewMessage("shutdown", nil))
```

## Integration Patterns

### Embedding in an HTTP Service

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "github.com/aixgo-dev/aixgo/agent"
)

type AgentService struct {
    runtime agent.Runtime
}

func NewAgentService() *AgentService {
    rt := agent.NewLocalRuntime()
    rt.Register(NewAnalyzerAgent("analyzer"))

    ctx := context.Background()
    go rt.Start(ctx)

    return &AgentService{runtime: rt}
}

func (s *AgentService) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Text string `json:"text"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    input := agent.NewMessage("analyze", req).
        WithMetadata("request_id", r.Header.Get("X-Request-ID"))

    response, err := s.runtime.Call(r.Context(), "analyzer", input)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(response.MarshalPayload())
}
```

### Wrapping Existing Code

Migrate existing logic incrementally by wrapping it in an agent:

```go
// Your existing service
type LegacyProcessor struct {
    // existing fields
}

func (p *LegacyProcessor) Process(data string) (string, error) {
    // existing logic
    return "processed: " + data, nil
}

// Wrap it in an agent
type LegacyWrapper struct {
    processor *LegacyProcessor
    name      string
    ready     bool
}

func (w *LegacyWrapper) Name() string { return w.name }
func (w *LegacyWrapper) Role() string { return "legacy-processor" }
func (w *LegacyWrapper) Ready() bool  { return w.ready }

func (w *LegacyWrapper) Start(ctx context.Context) error {
    w.ready = true
    <-ctx.Done()
    return nil
}

func (w *LegacyWrapper) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    var req struct {
        Data string `json:"data"`
    }
    input.UnmarshalPayload(&req)

    result, err := w.processor.Process(req.Data)
    if err != nil {
        return nil, err
    }

    return agent.NewMessage("result", map[string]string{"output": result}), nil
}

func (w *LegacyWrapper) Stop(ctx context.Context) error {
    w.ready = false
    return nil
}
```

## Building an Orchestrator

Create complex workflows by composing agents:

```go
type WorkflowOrchestrator struct {
    runtime agent.Runtime
    name    string
    ready   bool
}

func (o *WorkflowOrchestrator) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Step 1: Validate input
    validated, err := o.runtime.Call(ctx, "validator", input)
    if err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Step 2: Process in parallel
    targets := []string{"processor-1", "processor-2", "processor-3"}
    results, errors := o.runtime.CallParallel(ctx, targets, validated)

    // Check for errors
    for name, err := range errors {
        if err != nil {
            return nil, fmt.Errorf("processor %s failed: %w", name, err)
        }
    }

    // Step 3: Aggregate results
    aggregateInput := agent.NewMessage("aggregate", results)
    return o.runtime.Call(ctx, "aggregator", aggregateInput)
}
```

## Testing Agents

```go
package myagent_test

import (
    "context"
    "testing"
    "github.com/aixgo-dev/aixgo/agent"
)

func TestAnalyzerAgent(t *testing.T) {
    // Create agent
    analyzer := NewAnalyzerAgent("test-analyzer")
    analyzer.ready = true

    // Create input
    input := agent.NewMessage("analyze", map[string]string{
        "text": "Hello, World!",
    })

    // Execute
    ctx := context.Background()
    response, err := analyzer.Execute(ctx, input)
    if err != nil {
        t.Fatalf("Execute failed: %v", err)
    }

    // Verify response
    var result struct {
        WordCount int `json:"word_count"`
    }
    if err := response.UnmarshalPayload(&result); err != nil {
        t.Fatalf("Unmarshal failed: %v", err)
    }

    if result.WordCount == 0 {
        t.Error("Expected non-zero word count")
    }
}

func TestWithRuntime(t *testing.T) {
    ctx := context.Background()
    rt := agent.NewLocalRuntime()

    // Register test agents
    rt.Register(NewAnalyzerAgent("analyzer"))

    // Start runtime
    go rt.Start(ctx)
    defer rt.Stop(ctx)

    // Test call
    input := agent.NewMessage("test", map[string]string{"text": "test"})
    _, err := rt.Call(ctx, "analyzer", input)
    if err != nil {
        t.Fatalf("Call failed: %v", err)
    }
}
```

## Comparison: Framework vs. Library

| Feature | Full Framework | Public Package |
|---------|----------------|----------------|
| Built-in agent types (ReAct, Classifier, etc.) | Yes | No |
| LLM provider abstraction | Yes | No |
| MCP integration | Yes | No |
| Orchestration patterns | Yes | Build your own |
| Observability (OpenTelemetry, Langfuse) | Yes | Add manually |
| Dependencies | Full framework | Only uuid |
| Use case | Standalone AI apps | Integration into existing services |

## Migrating to Full Framework

When you're ready for advanced features, migration is straightforward:

```go
// Using public interfaces
import "github.com/aixgo-dev/aixgo/agent"

// Add full framework capabilities
import (
    "github.com/aixgo-dev/aixgo/agent"
    "github.com/aixgo-dev/aixgo/pkg/llm"
    "github.com/aixgo-dev/aixgo/pkg/agents"
)

// Your custom agents continue to work
rt := agent.NewLocalRuntime()
rt.Register(yourCustomAgent)

// Add framework agents alongside
reactAgent := agents.NewReActAgent(config)
rt.Register(reactAgent)
```

## Best Practices

1. **Keep agents focused**: Each agent should have a single responsibility
1. **Use metadata for tracing**: Add correlation IDs and request context to messages
1. **Handle context cancellation**: Always respect `ctx.Done()` in long-running operations
1. **Test in isolation**: Test agents independently before integrating with the runtime
1. **Graceful shutdown**: Always call `runtime.Stop()` to clean up resources

## Next Steps

- [Core Concepts](/guides/core-concepts) - Understand agent fundamentals
- [Multi-Agent Orchestration](/guides/multi-agent-orchestration) - Advanced coordination patterns
- [Extending Aixgo](/guides/extending-aixgo) - Add custom LLM providers and vector stores
