---
title: "Session Persistence"
description: "Build AI agents that remember with built-in session persistence, checkpoints, and multiple storage backends"
weight: 8
---

AI agents that remember. Aixgo sessions provide durable conversation history with automatic persistence, checkpoints, and seamless resumption across restarts.

## Quick Start

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/pkg/session"
)

func main() {
    ctx := context.Background()

    // Create file-based storage
    backend, _ := session.NewFileBackend("~/.aixgo/sessions")
    mgr := session.NewManager(backend)
    defer mgr.Close()

    // Create or resume a session
    sess, _ := mgr.GetOrCreate(ctx, "assistant", "user-123")

    // Append a message (role: "user", "assistant", or "system")
    sess.AppendEntry(ctx, session.EntryTypeMessage, map[string]any{
        "role":    "user",
        "content": "Hello!",
    })

    // Get full history
    entries, _ := sess.GetEntries(ctx)
}
```

## Why Sessions?

| Problem | Solution |
|---------|----------|
| Context lost on restart | Persistent JSONL/Redis storage |
| No rollback capability | Checkpoint and restore |
| Complex state management | Automatic session lifecycle |
| Multi-node deployments | Redis backend for shared state |

## Storage Backends

### File Backend (JSONL)

Best for single-node deployments and development:

```go
backend, err := session.NewFileBackend("~/.aixgo/sessions")
```

**Features:**
- Append-only JSONL format (human-readable)
- Automatic directory creation
- Restrictive permissions (0700/0600)
- File locking for concurrent access

### Redis Backend

Best for distributed deployments:

```go
backend, err := session.NewRedisBackend(
    "localhost:6379",
    session.RedisOptions{
        Password:  os.Getenv("REDIS_PASSWORD"),
        DB:        0,
        KeyPrefix: "aixgo:sessions:",
    },
)
```

**Features:**
- Shared state across nodes
- Connection pooling
- Configurable key prefix
- TTL support for session expiration

## Checkpoints

Save and restore conversation state:

```go
// Create checkpoint before risky operation
checkpoint, err := sess.Checkpoint(ctx)
if err != nil {
    log.Fatal(err)
}

// ... perform operation ...

// Restore if needed
if needRollback {
    err = sess.Restore(ctx, checkpoint.ID)
}
```

Checkpoints include:
- Full message history reference
- Timestamp and checksum
- Custom metadata

## Runtime Integration

Use `CallWithSession()` for automatic session management:

```go
rt := aixgo.NewSimpleRuntime()
rt.SetSessionManager(sessionMgr)

// Messages automatically persisted
result, err := rt.CallWithSession(ctx, "assistant", msg, sessionID)
```

## Context Helpers

Pass sessions through context:

```go
// Add session to context
ctx = session.ContextWithSession(ctx, sess)

// Retrieve in downstream code
sess, ok := session.SessionFromContext(ctx)
if ok {
    messages, _ := sess.GetMessages(ctx)
}
```

## Configuration

YAML configuration for session-enabled agents:

```yaml
session:
  enabled: true
  store: file  # or redis
  base_dir: ~/.aixgo/sessions

agents:
  - name: assistant
    role: react
    model: gpt-4-turbo
    prompt: "You are a helpful assistant with memory."
```

## Performance

| Operation | Latency |
|-----------|---------|
| Create session | <1ms |
| Append message | <1ms |
| Get 100 messages | <5ms |
| Checkpoint | <1ms |

## Examples

Two complete examples are included:

1. **session-basic** - CRUD operations, checkpoints, context helpers
2. **session-react** - ReAct agent with full conversation history

```bash
cd examples/session-basic && go run main.go
cd examples/session-react && go run main.go
```

## Best Practices

1. **Use GetOrCreate()** - Handles both new and existing sessions
2. **Checkpoint before risky operations** - Enable rollback capability
3. **Use Redis for multi-node** - File backend is single-node only
4. **Clean up old sessions** - Implement retention policies
5. **Pass sessions via context** - Cleaner than parameter threading

## Next Steps

- [Multi-Agent Orchestration](/guides/multi-agent-orchestration/) - Coordinate multiple agents
- [Production Deployment](/guides/production-deployment/) - Deploy with sessions
- [Observability](/guides/observability/) - Monitor session operations
