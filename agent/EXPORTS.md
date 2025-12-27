# Agent Package - Public API

This document lists all public exports from the `github.com/aixgo-dev/aixgo/agent` package.

## Interfaces

### Agent
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

The core interface that all agents must implement. External packages should implement this interface for custom agents.

### Runtime
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

Provides agent coordination, message routing, and lifecycle management.

## Types

### Message
```go
type Message struct {
    ID        string
    Type      string
    Payload   string
    Timestamp string
    Metadata  map[string]interface{}
}
```

The standard message format for agent communication.

### LocalRuntime
```go
type LocalRuntime struct {
    // unexported fields
}
```

A single-process runtime implementation for agent coordination using in-memory channels.

## Functions

### NewMessage
```go
func NewMessage(msgType string, payload interface{}) *Message
```

Creates a new message with the given type and payload. The payload is automatically serialized to JSON.

### NewLocalRuntime
```go
func NewLocalRuntime() *LocalRuntime
```

Creates a new local runtime for single-process agent coordination.

## Message Methods

### WithMetadata
```go
func (m *Message) WithMetadata(key string, value interface{}) *Message
```

Adds metadata to the message and returns it for method chaining.

### GetMetadata
```go
func (m *Message) GetMetadata(key string, defaultValue interface{}) interface{}
```

Retrieves metadata by key, returning the default value if not found.

### GetMetadataString
```go
func (m *Message) GetMetadataString(key, defaultValue string) string
```

Convenience method to get metadata as a string.

### UnmarshalPayload
```go
func (m *Message) UnmarshalPayload(v interface{}) error
```

Deserializes the message payload into the provided value.

### MarshalPayload
```go
func (m *Message) MarshalPayload() []byte
```

Returns the payload as JSON bytes.

### Clone
```go
func (m *Message) Clone() *Message
```

Creates a deep copy of the message.

### String
```go
func (m *Message) String() string
```

Returns a human-readable representation of the message.

## LocalRuntime Methods

All methods of the Runtime interface are implemented by LocalRuntime. See the Runtime interface documentation above for details.

## Import Path

```go
import "github.com/aixgo-dev/aixgo/agent"
```

## Usage Example

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/agent"
)

// Implement custom agent
type MyAgent struct {
    name  string
    ready bool
}

func (a *MyAgent) Name() string { return a.name }
func (a *MyAgent) Role() string { return "custom" }
func (a *MyAgent) Ready() bool  { return a.ready }

func (a *MyAgent) Start(ctx context.Context) error {
    a.ready = true
    <-ctx.Done()
    return nil
}

func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    return agent.NewMessage("response", map[string]string{"status": "ok"}), nil
}

func (a *MyAgent) Stop(ctx context.Context) error {
    a.ready = false
    return nil
}

func main() {
    rt := agent.NewLocalRuntime()
    rt.Register(&MyAgent{name: "myagent"})

    ctx := context.Background()
    rt.Start(ctx)

    input := agent.NewMessage("request", map[string]string{"action": "process"})
    response, _ := rt.Call(ctx, "myagent", input)

    rt.Stop(ctx)
}
```

## Version Compatibility

This package is designed to be stable and maintain backward compatibility. Breaking changes will only be introduced in major version updates.

## See Also

- [README.md](README.md) - Package overview and examples
- [INTEGRATION.md](INTEGRATION.md) - Integration guide for external projects
- [Aixgo Documentation](https://aixgo.dev) - Full framework documentation
