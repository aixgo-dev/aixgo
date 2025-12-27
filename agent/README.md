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

    // Start the runtime
    ctx := context.Background()
    rt.Start(ctx)

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

### Runtime

The `Runtime` interface provides agent coordination:

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
