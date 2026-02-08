# Integration Guide for External Projects

This guide shows how to integrate the Aixgo agent package into your Go projects.

## Installation

Add the dependency to your project:

```bash
go get github.com/aixgo-dev/aixgo/agent
```

Or add to your `go.mod`:

```go
require github.com/aixgo-dev/aixgo/agent latest
```

## Basic Integration Example

### 1. Import the Package

```go
import "github.com/aixgo-dev/aixgo/agent"
```

### 2. Implement Custom Agents

Create domain-specific agents for your application:

```go
package myapp

import (
    "context"
    "github.com/aixgo-dev/aixgo/agent"
)

// DataProcessorAgent processes incoming data
type DataProcessorAgent struct {
    name      string
    processor DataProcessor // Your processing abstraction
    ready     bool
}

func NewDataProcessorAgent(name string, proc DataProcessor) *DataProcessorAgent {
    return &DataProcessorAgent{
        name:      name,
        processor: proc,
        ready:     false,
    }
}

func (a *DataProcessorAgent) Name() string { return a.name }
func (a *DataProcessorAgent) Role() string { return "data-processor" }
func (a *DataProcessorAgent) Ready() bool  { return a.ready }

func (a *DataProcessorAgent) Start(ctx context.Context) error {
    a.ready = true
    <-ctx.Done()
    return nil
}

func (a *DataProcessorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Unmarshal request
    type Request struct {
        ID      string `json:"id"`
        Content string `json:"content"`
    }

    var req Request
    if err := input.UnmarshalPayload(&req); err != nil {
        return nil, err
    }

    // Perform processing
    result, err := a.processor.Process(ctx, req.Content)
    if err != nil {
        return nil, err
    }

    // Return result
    return agent.NewMessage("process_result", result), nil
}

func (a *DataProcessorAgent) Stop(ctx context.Context) error {
    a.ready = false
    return nil
}
```

### 3. Create an Orchestrator

```go
package myapp

import (
    "context"
    "github.com/aixgo-dev/aixgo"
    "github.com/aixgo-dev/aixgo/agent"
)

// WorkflowOrchestrator coordinates multiple agents
type WorkflowOrchestrator struct {
    runtime agent.Runtime
    name    string
    ready   bool
}

func NewWorkflowOrchestrator(name string, rt agent.Runtime) *WorkflowOrchestrator {
    return &WorkflowOrchestrator{
        runtime: rt,
        name:    name,
        ready:   false,
    }
}

func (a *WorkflowOrchestrator) Name() string { return a.name }
func (a *WorkflowOrchestrator) Role() string { return "orchestrator" }
func (a *WorkflowOrchestrator) Ready() bool  { return a.ready }

func (a *WorkflowOrchestrator) Start(ctx context.Context) error {
    a.ready = true
    <-ctx.Done()
    return nil
}

func (a *WorkflowOrchestrator) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Step 1: Process data
    processResult, err := a.runtime.Call(ctx, "data-processor", input)
    if err != nil {
        return nil, err
    }

    // Step 2: Validate results
    validationResult, err := a.runtime.Call(ctx, "validator", processResult)
    if err != nil {
        return nil, err
    }

    // Step 3: Format output
    outputResult, err := a.runtime.Call(ctx, "formatter", validationResult)
    if err != nil {
        return nil, err
    }

    return outputResult, nil
}

func (a *WorkflowOrchestrator) Stop(ctx context.Context) error {
    a.ready = false
    return nil
}
```

### 4. Wire Up in Your Service

```go
package myapp

import (
    "context"
    "github.com/aixgo-dev/aixgo"
    "github.com/aixgo-dev/aixgo/agent"
)

type ProcessingService struct {
    runtime      agent.Runtime
    orchestrator agent.Agent
}

func NewProcessingService(processor DataProcessor) *ProcessingService {
    // Create runtime
    rt := aixgo.NewRuntime()

    // Register agents
    rt.Register(NewDataProcessorAgent("data-processor", processor))
    rt.Register(NewValidatorAgent("validator"))
    rt.Register(NewFormatterAgent("formatter"))

    // Create orchestrator
    orchestrator := NewWorkflowOrchestrator("orchestrator", rt)
    rt.Register(orchestrator)

    return &ProcessingService{
        runtime:      rt,
        orchestrator: orchestrator,
    }
}

func (s *ProcessingService) Start(ctx context.Context) error {
    // Start() blocks until all agents are started and ready
    // Returns error if any agent fails to start
    return s.runtime.Start(ctx)
}

func (s *ProcessingService) Process(ctx context.Context, req *ProcessRequest) (*ProcessResult, error) {
    // Convert request to message
    input := agent.NewMessage("process_request", req).
        WithMetadata("correlation_id", req.CorrelationID).
        WithMetadata("user_id", req.UserID)

    // Call orchestrator
    response, err := s.runtime.Call(ctx, "orchestrator", input)
    if err != nil {
        return nil, err
    }

    // Convert response
    var result ProcessResult
    if err := response.UnmarshalPayload(&result); err != nil {
        return nil, err
    }

    return &result, nil
}

func (s *ProcessingService) Stop(ctx context.Context) error {
    return s.runtime.Stop(ctx)
}
```

### 5. Update go.mod

```go
module myapp

go 1.21

require (
    github.com/aixgo-dev/aixgo/agent latest
    // ... other dependencies
)
```

## Migration Path from Existing Code

If you have existing code, you can migrate incrementally:

### Phase 1: Add Runtime
```go
// Old code still works
func (s *Service) Process(ctx context.Context, req *Request) (*Result, error) {
    // existing implementation
}

// New agent-based code alongside
rt := aixgo.NewRuntime()
```

### Phase 2: Wrap Existing Code
```go
type LegacyWrapper struct {
    service *Service
    name    string
    ready   bool
}

func (w *LegacyWrapper) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    var req Request
    input.UnmarshalPayload(&req)

    result, err := w.service.Process(ctx, &req)
    if err != nil {
        return nil, err
    }

    return agent.NewMessage("result", result), nil
}
```

### Phase 3: Refactor to Pure Agents
```go
// Gradually replace legacy code with new agent implementations
```

## Testing with the Agent Package

```go
package myapp_test

import (
    "context"
    "testing"
    "github.com/aixgo-dev/aixgo"
    "github.com/aixgo-dev/aixgo/agent"
)

func TestDataProcessor(t *testing.T) {
    // Create mock processor
    mockProc := NewMockProcessor()

    // Create agent
    processor := NewDataProcessorAgent("test-processor", mockProc)

    // Create runtime
    rt := aixgo.NewRuntime()
    rt.Register(processor)

    ctx := context.Background()
    processor.ready = true

    // Test
    input := agent.NewMessage("process", map[string]string{
        "id":      "test-123",
        "content": "Sample data...",
    })

    response, err := rt.Call(ctx, "test-processor", input)
    if err != nil {
        t.Fatalf("Processing failed: %v", err)
    }

    // Verify response
    var result map[string]interface{}
    response.UnmarshalPayload(&result)
    // assertions...
}
```

## Parallel Execution

Use `CallParallel` for concurrent agent execution:

```go
// Call multiple agents in parallel
targets := []string{"processor-1", "processor-2", "processor-3"}
results, errors := rt.CallParallel(ctx, targets, input)

// Handle results
for name, result := range results {
    fmt.Printf("Agent %s completed\n", name)
}

for name, err := range errors {
    fmt.Printf("Agent %s failed: %v\n", name, err)
}
```

## Runtime Lifecycle Management

The `Runtime` provides strong guarantees for agent lifecycle:

### Startup Behavior

```go
rt := aixgo.NewRuntime()
rt.Register(agent1)
rt.Register(agent2)
rt.Register(agent3)

// Start() performs these steps:
// 1. Starts all agents concurrently (in parallel)
// 2. Waits for all Start() calls to complete
// 3. Verifies all agents report Ready() == true
// 4. Returns error if any agent fails or isn't ready
if err := rt.Start(ctx); err != nil {
    log.Fatalf("Startup failed: %v", err)
}

// At this point, all agents are guaranteed to be ready
```

### Error Handling During Startup

```go
type DatabaseAgent struct {
    db *sql.DB
    ready bool
}

func (a *DatabaseAgent) Start(ctx context.Context) error {
    db, err := sql.Open("postgres", connectionString)
    if err != nil {
        return fmt.Errorf("failed to connect to database: %w", err)
    }
    a.db = db
    a.ready = true
    return nil
}

// If database connection fails, runtime.Start() returns:
// "agent database failed to start: failed to connect to database: ..."
```

### Shutdown Behavior

```go
// Stop() gracefully shuts down all agents concurrently
if err := rt.Stop(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

### Key Guarantees

1. **Deterministic Startup**: `Start()` blocks until all agents are ready
2. **Parallel Performance**: Agents start concurrently for fast initialization
3. **Error Propagation**: Any startup failure is immediately reported
4. **Ready State**: All agents guaranteed to be `Ready() == true` after `Start()`
5. **No Race Conditions**: Safe to call agents immediately after `Start()` returns

## Benefits of Agent-Based Architecture

1. **Testability**: Each agent can be tested in isolation
2. **Composability**: Agents can be composed into workflows
3. **Parallel Execution**: Use CallParallel for concurrent processing
4. **Clear Contracts**: Message-based communication with explicit types
5. **Tracing**: Built-in metadata for correlation and tracing
6. **Flexibility**: Easy to add new agents without changing existing code
7. **Lifecycle Management**: Strong startup and shutdown guarantees

## Next Steps

1. Review the [README.md](README.md) for detailed API documentation
2. Check [EXPORTS.md](EXPORTS.md) for the complete public API surface
3. See [example_test.go](example_test.go) for runnable examples
4. Refer to the main [Aixgo documentation](https://aixgo.dev) for advanced patterns

## Support

For questions or issues:
- Open an issue in the Aixgo repository
- Check the Aixgo documentation at https://aixgo.dev
- Review the test files for usage examples
