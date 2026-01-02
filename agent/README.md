# Agent Package

The `agent` package provides the public interfaces for building agents with Aixgo. This package exports the core `Agent`, `Message`, and `Runtime` interfaces that external projects need to build custom agents or interact with the Aixgo framework.

## Installation

```bash
go get github.com/aixgo-dev/aixgo/agent
```

## Quick Start

### Creating a Custom Agent

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/agent"
)

type MyAgent struct {
    name  string
    ready bool
}

func NewMyAgent(name string) *MyAgent {
    return &MyAgent{name: name}
}

func (a *MyAgent) Name() string { return a.name }
func (a *MyAgent) Role() string { return "custom" }
func (a *MyAgent) Ready() bool  { return a.ready }

func (a *MyAgent) Start(ctx context.Context) error {
    a.ready = true
    // Start any background processing
    <-ctx.Done()
    return nil
}

func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Process the input and return a response
    return agent.NewMessage("response", map[string]string{
        "status": "processed",
    }), nil
}

func (a *MyAgent) Stop(ctx context.Context) error {
    a.ready = false
    return nil
}
```

### Using the Runtime

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/agent"
)

func main() {
    // Create a local runtime
    rt := agent.NewLocalRuntime()

    // Register agents
    rt.Register(NewMyAgent("agent1"))
    rt.Register(NewMyAgent("agent2"))

    // Start the runtime - blocks until all agents are started and ready
    ctx := context.Background()
    if err := rt.Start(ctx); err != nil {
        panic(err)
    }

    // Call an agent synchronously
    input := agent.NewMessage("request", map[string]string{"action": "analyze"})
    response, err := rt.Call(ctx, "agent1", input)
    if err != nil {
        panic(err)
    }

    // Process response
    var result map[string]string
    response.UnmarshalPayload(&result)

    // Call multiple agents in parallel
    results, errors := rt.CallParallel(ctx, []string{"agent1", "agent2"}, input)

    // Stop the runtime
    rt.Stop(ctx)
}
```

## Core Interfaces

### Runtime

The `Runtime` interface provides agent coordination and lifecycle management:

```go
type Runtime interface {
    Register(agent Agent) error
    Unregister(name string) error
    Get(name string) (Agent, error)
    List() []string
    Call(ctx context.Context, target string, input *Message) (*Message, error)
    CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, map[string]error)
    Send(target string, msg *Message) error
    Recv(source string) (<-chan *Message, error)
    Broadcast(msg *Message) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

#### LocalRuntime Behavior

The `LocalRuntime` implementation provides important guarantees for agent startup:

**Start() Method:**

- Starts all registered agents concurrently (in parallel for performance)
- Blocks until all agent `Start()` calls complete
- Verifies all agents report `Ready() == true` before returning
- Returns an error if any agent fails to start or doesn't become ready
- Ensures the runtime is in a fully-initialized state before accepting calls

**Example:**

```go
rt := agent.NewLocalRuntime()
rt.Register(agent1)
rt.Register(agent2)

// Start() blocks here until both agents are ready
if err := rt.Start(ctx); err != nil {
    log.Fatalf("Failed to start runtime: %v", err)
}

// At this point, all agents are guaranteed to be ready
response, err := rt.Call(ctx, "agent1", input)
```

**Error Handling:**

```go
// Agent fails to start
type FailingAgent struct{ /* ... */ }

func (a *FailingAgent) Start(ctx context.Context) error {
    return fmt.Errorf("database connection failed")
}

rt.Register(&FailingAgent{})

// Returns: "agent failing-agent failed to start: database connection failed"
err := rt.Start(ctx)
```

**Performance Note:** While agents are started concurrently for performance, the
`Start()` method itself blocks. This ensures deterministic behavior and prevents
race conditions when calling agents immediately after startup.

### Agent

The `Agent` interface must be implemented by all agents:

```go
type Agent interface {
    Name() string
    Role() string
    Start(ctx context.Context) error
    Execute(ctx context.Context, input *Message) (*Message, error)
    Stop(ctx context.Context) error
    Ready() bool
}
```

### Message

Messages are the standard unit of communication:

```go
type Message struct {
    ID        string
    Type      string
    Payload   string
    Timestamp string
    Metadata  map[string]interface{}
}
```

## Message Creation and Usage

```go
// Create a message with structured payload
type Request struct {
    Action string `json:"action"`
    Data   string `json:"data"`
}

msg := agent.NewMessage("request", Request{
    Action: "analyze",
    Data:   "sample text",
}).WithMetadata("priority", "high").
  WithMetadata("user_id", "user-123")

// Unmarshal payload
var req Request
if err := msg.UnmarshalPayload(&req); err != nil {
    panic(err)
}

// Access metadata
priority := msg.GetMetadataString("priority", "normal")
```

## Communication Patterns

### Synchronous (Request-Response)

```go
// Call a single agent
response, err := rt.Call(ctx, "analyzer", input)

// Call multiple agents in parallel
results, errors := rt.CallParallel(ctx, []string{"agent1", "agent2"}, input)
```

### Asynchronous (Message Passing)

```go
// Send a message asynchronously
rt.Send("agent1", msg)

// Receive messages from an agent
recvCh, _ := rt.Recv("agent1")
for msg := range recvCh {
    // Process message
}

// Broadcast to all agents
rt.Broadcast(msg)
```

## Examples

See the `agent_test.go` file for comprehensive examples including:

- Creating custom agents
- Message serialization/deserialization
- Runtime registration and coordination
- Synchronous and asynchronous communication
- Parallel agent execution

## Integration with Aixgo

This package is designed to be used standalone or integrated with the full Aixgo framework. For advanced features like:

- Built-in agent types (ReAct, Classifier, Aggregator, etc.)
- MCP (Model Context Protocol) integration
- LLM provider abstraction
- Orchestration patterns
- Observability and metrics

See the main Aixgo documentation at https://aixgo.dev

## License

See the main Aixgo repository for license information.
