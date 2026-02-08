# Session Persistence

**Comprehensive guide to session management in Aixgo**

## Overview

The session persistence feature enables AI agents to maintain conversation history, checkpoint state, and resume from
previous interactions. Sessions provide durable storage for agent conversations with automatic checkpointing and
restoration capabilities.

**Key Features**:

- **Persistent Conversation History**: Store and retrieve complete message histories
- **Checkpoint Creation & Restoration**: Save state snapshots and rollback when needed
- **Multiple Storage Backends**: File-based (JSONL), Firestore, PostgreSQL support
- **User-Scoped Sessions**: Associate sessions with specific users for multi-tenant applications
- **Concurrent-Safe Operations**: Thread-safe session management and storage
- **Context Integration**: Seamless context.Context-based session passing

## Quick Start

### Installation

Sessions are included in the core Aixgo package.

```bash
go get github.com/aixgo-dev/aixgo
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/aixgo-dev/aixgo/agent"
    "github.com/aixgo-dev/aixgo/pkg/session"
)

func main() {
    ctx := context.Background()

    // Create file-based storage backend
    backend, err := session.NewFileBackend("")
    if err != nil {
        log.Fatal(err)
    }
    defer backend.Close()

    // Create session manager
    mgr := session.NewManager(backend)
    defer mgr.Close()

    // Create a new session
    sess, err := mgr.Create(ctx, "assistant", session.CreateOptions{
        UserID: "user-123",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Append messages
    msg := agent.NewMessage("user", map[string]string{
        "content": "Hello, AI!",
    })
    if err := sess.AppendMessage(ctx, msg); err != nil {
        log.Fatal(err)
    }

    // Retrieve message history
    messages, err := sess.GetMessages(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Session has %d messages", len(messages))
}
```

## Configuration

### YAML Configuration

Add session configuration to your Aixgo YAML config:

```yaml
session:
  enabled: true
  store: file
  base_dir: ~/.aixgo/sessions
  checkpoint:
    auto_save: true
    interval: 5m
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `true` | Enable/disable session persistence |
| `store` | `string` | `"file"` | Storage backend type (`file`, `firestore`, `postgres`) |
| `base_dir` | `string` | `~/.aixgo/sessions` | Base directory for file storage |
| `checkpoint.auto_save` | `bool` | `false` | Automatically create checkpoints |
| `checkpoint.interval` | `string` | `"5m"` | Auto-checkpoint interval |

### Environment Variables

```bash
# Session storage location (file backend)
export AIXGO_SESSION_DIR=~/.aixgo/sessions

# Firestore configuration (if using Firestore backend)
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
export FIRESTORE_PROJECT_ID=my-project

# PostgreSQL configuration (if using PostgreSQL backend)
export POSTGRES_CONNECTION_STRING=postgres://user:pass@localhost/aixgo
```

## Session Modes

### Enabled Mode (Default)

Sessions are active and conversation history is persisted.

```yaml
session:
  enabled: true
```

**Use Cases**:

- Multi-turn conversations requiring context
- Customer support chatbots
- Research assistants with long-running tasks
- Applications requiring conversation replay

### Disabled Mode

Sessions are turned off, agents operate statelessly.

```yaml
session:
  enabled: false
```

**Use Cases**:

- Single-shot API requests
- Stateless microservices
- High-throughput batch processing
- Development/testing with no persistence needed

## Core Operations

### Create a Session

Create a new session for an agent.

```go
sess, err := mgr.Create(ctx, "assistant", session.CreateOptions{
    UserID: "user-123",
    Metadata: map[string]any{
        "source": "mobile-app",
        "locale": "en-US",
    },
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Created session %s", sess.ID())
```

### Get an Existing Session

Retrieve a session by its ID.

```go
sess, err := mgr.Get(ctx, sessionID)
if err != nil {
    if errors.Is(err, session.ErrSessionNotFound) {
        log.Println("Session not found")
    } else {
        log.Fatal(err)
    }
}
```

### Get or Create Session

Convenient method to retrieve an existing session or create a new one.

```go
// Returns existing session for user-123 or creates new one
sess, err := mgr.GetOrCreate(ctx, "assistant", "user-123")
if err != nil {
    log.Fatal(err)
}
```

**Behavior**:

- If `userID` is provided, searches for existing sessions for that user
- Returns the most recently updated session if found
- Creates a new session if none exists
- If `userID` is empty, always creates a new session

### Append Messages

Add messages to the session history.

```go
// User message
userMsg := agent.NewMessage("user", map[string]string{
    "content": "What's the weather today?",
})
if err := sess.AppendMessage(ctx, userMsg); err != nil {
    log.Fatal(err)
}

// Agent response
agentMsg := agent.NewMessage("assistant", map[string]string{
    "content": "I'll check the weather for you.",
    "temperature": "72°F",
})
if err := sess.AppendMessage(ctx, agentMsg); err != nil {
    log.Fatal(err)
}
```

### Retrieve Message History

Get all messages from a session.

```go
messages, err := sess.GetMessages(ctx)
if err != nil {
    log.Fatal(err)
}

for _, msg := range messages {
    log.Printf("[%s] %s", msg.Type, msg.Payload)
}
```

### Create Checkpoints

Checkpoints allow you to save session state and restore later.

```go
// Create checkpoint
checkpoint, err := sess.Checkpoint(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Created checkpoint %s at entry %s",
    checkpoint.ID, checkpoint.EntryID)

// Later: restore to checkpoint
if err := sess.Restore(ctx, checkpoint.ID); err != nil {
    log.Fatal(err)
}

log.Println("Session restored to checkpoint")
```

**Use Cases**:

- Undo/redo functionality
- Rollback after failed operations
- A/B testing different conversation paths
- Debugging conversation flows

### List Sessions

Query sessions by agent and filter criteria.

```go
// List all sessions for an agent
sessions, err := mgr.List(ctx, "assistant", session.ListOptions{})
if err != nil {
    log.Fatal(err)
}

// List sessions for specific user with pagination
sessions, err := mgr.List(ctx, "assistant", session.ListOptions{
    UserID: "user-123",
    Limit:  10,
    Offset: 0,
})
if err != nil {
    log.Fatal(err)
}

for _, meta := range sessions {
    log.Printf("Session %s: %d messages, updated %s",
        meta.ID, meta.MessageCount, meta.UpdatedAt)
}
```

### Delete Sessions

Remove a session and all its data.

```go
if err := mgr.Delete(ctx, sessionID); err != nil {
    log.Fatal(err)
}

log.Println("Session deleted")
```

### Context Helpers

Pass sessions through context for easy access across function calls.

```go
// Add session to context
ctxWithSession := session.ContextWithSession(ctx, sess)

// Retrieve session from context
sess, ok := session.SessionFromContext(ctxWithSession)
if !ok {
    log.Fatal("Session not found in context")
}

// Use in agent execution
func handleRequest(ctx context.Context, req *Request) error {
    sess, ok := session.SessionFromContext(ctx)
    if !ok {
        return session.ErrSessionNotInContext
    }

    // Process with session...
    return nil
}
```

## Storage Backends

### File Backend (Default)

JSONL-based file storage for sessions.

**Features**:

- Simple filesystem-based storage
- JSONL format for easy inspection and debugging
- Append-only writes for performance
- Organized by agent name

**Storage Layout**:

```text
~/.aixgo/sessions/
  └── <agent-name>/
      ├── sessions.json              # Session index
      ├── <session-id>.jsonl         # Session entries
      └── checkpoints/
          └── <checkpoint-id>.json   # Checkpoint data
```

**Configuration**:

```go
backend, err := session.NewFileBackend("~/.aixgo/sessions")
if err != nil {
    log.Fatal(err)
}
defer backend.Close()
```

**Pros**:

- No external dependencies
- Easy to inspect and debug
- Fast for moderate session counts (<10,000)
- Portable across environments

**Cons**:

- Not suitable for distributed systems
- Limited query capabilities
- File locking overhead at scale

### Firestore Backend

Google Cloud Firestore-based storage (future).

**Configuration**:

```yaml
session:
  store: firestore
```

**Pros**:

- Fully managed, scalable storage
- Real-time synchronization
- Multi-region replication
- Built-in security rules

**Cons**:

- External dependency
- Network latency
- Cost at scale

### PostgreSQL Backend

Relational database storage (future).

**Configuration**:

```yaml
session:
  store: postgres
```

**Pros**:

- ACID transactions
- Complex queries
- Mature ecosystem
- Self-hostable

**Cons**:

- Requires database setup
- Higher operational complexity

### Custom Storage Backend

Implement the `StorageBackend` interface for custom storage:

```go
type PostgresBackend struct {
    db *sql.DB
}

func (b *PostgresBackend) SaveSession(ctx context.Context, meta *session.SessionMetadata) error {
    _, err := b.db.ExecContext(ctx, `
        INSERT INTO sessions (id, agent_name, user_id, created_at, updated_at, message_count)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (id) DO UPDATE SET
            updated_at = EXCLUDED.updated_at,
            message_count = EXCLUDED.message_count
    `, meta.ID, meta.AgentName, meta.UserID, meta.CreatedAt, meta.UpdatedAt, meta.MessageCount)
    return err
}

func (b *PostgresBackend) LoadSession(ctx context.Context, sessionID string) (*session.SessionMetadata, error) {
    var meta session.SessionMetadata
    err := b.db.QueryRowContext(ctx, `
        SELECT id, agent_name, user_id, created_at, updated_at, message_count
        FROM sessions WHERE id = $1
    `, sessionID).Scan(
        &meta.ID, &meta.AgentName, &meta.UserID,
        &meta.CreatedAt, &meta.UpdatedAt, &meta.MessageCount,
    )
    if err == sql.ErrNoRows {
        return nil, session.ErrSessionNotFound
    }
    return &meta, err
}

// Implement remaining StorageBackend methods...
```

Register the custom backend:

```go
backend := &PostgresBackend{db: db}
mgr := session.NewManager(backend)
```

## API Reference

### SessionManager Interface

```go
type Manager interface {
    // Create creates a new session for an agent.
    Create(ctx context.Context, agentName string, opts CreateOptions) (Session, error)

    // Get retrieves an existing session by ID.
    // Returns ErrSessionNotFound if the session doesn't exist.
    Get(ctx context.Context, sessionID string) (Session, error)

    // GetOrCreate returns an existing session or creates a new one.
    // If userID is provided, it looks for existing sessions for that user.
    GetOrCreate(ctx context.Context, agentName, userID string) (Session, error)

    // List returns sessions for an agent matching the filter options.
    List(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error)

    // Delete removes a session and all its data.
    Delete(ctx context.Context, sessionID string) error

    // Close releases resources held by the manager.
    Close() error
}
```

### Session Interface

```go
type Session interface {
    // ID returns the unique session identifier.
    ID() string

    // AgentName returns the name of the agent this session belongs to.
    AgentName() string

    // UserID returns the user identifier (may be empty).
    UserID() string

    // AppendMessage adds a message to the session history.
    AppendMessage(ctx context.Context, msg *agent.Message) error

    // GetMessages retrieves all messages in the session.
    GetMessages(ctx context.Context) ([]*agent.Message, error)

    // Checkpoint creates a restorable state snapshot.
    Checkpoint(ctx context.Context) (*Checkpoint, error)

    // Restore reverts the session to a previous checkpoint.
    Restore(ctx context.Context, checkpointID string) error

    // Close releases any resources held by the session.
    Close(ctx context.Context) error
}
```

### StorageBackend Interface

```go
type StorageBackend interface {
    // SaveSession creates or updates session metadata.
    SaveSession(ctx context.Context, meta *SessionMetadata) error

    // LoadSession retrieves session metadata by ID.
    LoadSession(ctx context.Context, sessionID string) (*SessionMetadata, error)

    // DeleteSession removes a session and all its data.
    DeleteSession(ctx context.Context, sessionID string) error

    // ListSessions returns sessions for an agent matching filter options.
    ListSessions(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error)

    // AppendEntry adds an entry to a session (append-only).
    AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error

    // LoadEntries retrieves all entries for a session in order.
    LoadEntries(ctx context.Context, sessionID string) ([]*SessionEntry, error)

    // SaveCheckpoint stores a checkpoint.
    SaveCheckpoint(ctx context.Context, checkpoint *Checkpoint) error

    // LoadCheckpoint retrieves a checkpoint by ID.
    LoadCheckpoint(ctx context.Context, checkpointID string) (*Checkpoint, error)

    // Close releases any resources held by the backend.
    Close() error
}
```

### Types

**SessionMetadata**:

```go
type SessionMetadata struct {
    ID           string            // Unique session identifier
    AgentName    string            // Agent this session belongs to
    UserID       string            // User identifier (optional)
    CreatedAt    time.Time         // Creation timestamp
    UpdatedAt    time.Time         // Last update timestamp
    MessageCount int               // Number of messages
    CurrentLeaf  string            // Current leaf entry ID
}
```

**SessionEntry**:

```go
type SessionEntry struct {
    ID        string         // Unique entry identifier
    ParentID  string         // Previous entry ID (for branching)
    Timestamp time.Time      // Entry creation time
    Type      EntryType      // message, checkpoint, metadata
    Data      map[string]any // Entry payload
}
```

**Checkpoint**:

```go
type Checkpoint struct {
    ID        string         // Unique checkpoint identifier
    SessionID string         // Parent session ID
    Timestamp time.Time      // Checkpoint creation time
    EntryID   string         // Session entry at checkpoint
    Checksum  string         // Integrity checksum
    Metadata  map[string]any // Optional checkpoint metadata
}
```

### Errors

```go
var (
    // ErrSessionNotFound is returned when a session doesn't exist.
    ErrSessionNotFound = errors.New("session not found")

    // ErrCheckpointNotFound is returned when a checkpoint doesn't exist.
    ErrCheckpointNotFound = errors.New("checkpoint not found")

    // ErrStorageClosed is returned when operating on closed backend.
    ErrStorageClosed = errors.New("storage backend is closed")

    // ErrSessionNotInContext is returned when no session in context.
    ErrSessionNotInContext = errors.New("session not found in context")
)
```

## Runtime Integration

### Setting Up Session-Enabled Runtime

```go
import (
    "github.com/aixgo-dev/aixgo"
    "github.com/aixgo-dev/aixgo/pkg/session"
)

func main() {
    ctx := context.Background()

    // 1. Create storage backend
    backend, err := session.NewFileBackend("~/.aixgo/sessions")
    if err != nil {
        log.Fatal(err)
    }
    defer backend.Close()

    // 2. Create session manager
    mgr := session.NewManager(backend)
    defer mgr.Close()

    // 3. Create and configure runtime
    rt := aixgo.NewRuntime()
    rt.SetSessionManager(mgr)

    if err := rt.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer rt.Stop(ctx)

    // Runtime is now session-enabled
}
```

### Using CallWithSession

The runtime provides `CallWithSession` for automatic session management:

```go
// Create or get a session
sess, _ := mgr.GetOrCreate(ctx, "assistant", "user-123")

// Call agent with session - input/output automatically logged
result, err := rt.CallWithSession(ctx, "assistant", input, sess.ID())
```

This method:

1. Retrieves the session by ID
2. Appends the input message to session history
3. Adds session to context for agent access
4. Executes the agent
5. Appends the output message to session history

### Accessing Session Manager from Runtime

```go
// Get session manager from runtime
mgr := rt.SessionManager()

// List sessions for an agent
sessions, _ := mgr.List(ctx, "assistant", session.ListOptions{})

// Delete old sessions
for _, s := range sessions {
    if s.UpdatedAt.Before(cutoff) {
        mgr.Delete(ctx, s.ID)
    }
}
```

## Integration with Agents

### Agent Configuration

Enable sessions in agent configuration:

```yaml
agents:
  - name: assistant
    role: react
    model: gpt-4-turbo
    prompt: "You are a helpful assistant..."
    session:
      enabled: true
```

### Session-Aware Agents

Agents can implement `SessionAwareAgent` for direct access to conversation history:

```go
import "github.com/aixgo-dev/aixgo/pkg/session"

type MyAgent struct {
    // ...
}

// ExecuteWithSession is called when session context is available
func (a *MyAgent) ExecuteWithSession(
    ctx context.Context,
    input *agent.Message,
    sess session.Session,
) (*agent.Message, error) {
    // Get conversation history
    history, err := sess.GetMessages(ctx)
    if err != nil {
        return nil, err
    }

    // Use history for context-aware processing
    response := a.processWithHistory(input, history)

    return response, nil
}
```

The built-in ReAct agent implements `SessionAwareAgent`:

```go
// ReActAgent automatically includes history in prompts
func (r *ReActAgent) ExecuteWithSession(
    ctx context.Context,
    input *agent.Message,
    sess session.Session,
) (*agent.Message, error) {
    // Gets history from session
    history, _ := sess.GetMessages(ctx)

    // Includes history in LLM context
    return r.thinkWithHistory(ctx, input, history)
}
```

### Checking Agent Capabilities

```go
import "github.com/aixgo-dev/aixgo/pkg/session"

// Check if agent supports sessions
if session.IsSessionAware(myAgent) {
    // Use ExecuteWithSession
}

// Check checkpoint support
if session.IsCheckpointable(myAgent) {
    // Agent can save/restore internal state
}

// Check memory support
if session.IsMemoryAware(myAgent) {
    // Agent can access long-term memory
}
```

### Accessing Sessions in Standard Agents

```go
func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Get session from context
    sess, ok := session.SessionFromContext(ctx)
    if !ok {
        // Create new session if needed
        sess, err := a.sessionMgr.GetOrCreate(ctx, a.name, input.UserID())
        if err != nil {
            return nil, fmt.Errorf("get session: %w", err)
        }
        ctx = session.ContextWithSession(ctx, sess)
    }

    // Append incoming message
    if err := sess.AppendMessage(ctx, input); err != nil {
        return nil, fmt.Errorf("append message: %w", err)
    }

    // Get conversation history for context
    history, err := sess.GetMessages(ctx)
    if err != nil {
        return nil, fmt.Errorf("get history: %w", err)
    }

    // Process with history...
    response := a.processWithHistory(ctx, input, history)

    // Append response
    if err := sess.AppendMessage(ctx, response); err != nil {
        return nil, fmt.Errorf("append response: %w", err)
    }

    return response, nil
}
```

### Checkpointable Agents

For agents with internal state that should be saved:

```go
type StatefulAgent struct {
    memory   map[string]string
    counter  int
    lastSeen time.Time
}

func (a *StatefulAgent) CreateCheckpoint(ctx context.Context) (map[string]any, error) {
    return map[string]any{
        "memory":    a.memory,
        "counter":   a.counter,
        "last_seen": a.lastSeen.Format(time.RFC3339),
    }, nil
}

func (a *StatefulAgent) RestoreFromCheckpoint(ctx context.Context, state map[string]any) error {
    if mem, ok := state["memory"].(map[string]string); ok {
        a.memory = mem
    }
    if cnt, ok := state["counter"].(int); ok {
        a.counter = cnt
    }
    if ts, ok := state["last_seen"].(string); ok {
        a.lastSeen, _ = time.Parse(time.RFC3339, ts)
    }
    return nil
}
```

Using checkpoints with agent state:

```go
// Create checkpoint that includes agent state
if checkpointable, ok := myAgent.(session.CheckpointableAgent); ok {
    // Get agent's internal state
    agentState, _ := checkpointable.CreateCheckpoint(ctx)

    // Create session checkpoint with agent state
    checkpoint, _ := sess.Checkpoint(ctx)
    // Agent state is stored in checkpoint.Metadata
}

// Restore checkpoint
if err := sess.Restore(ctx, checkpointID); err != nil {
    log.Fatal(err)
}

// Restore agent state
if checkpointable, ok := myAgent.(session.CheckpointableAgent); ok {
    checkpoint, _ := backend.LoadCheckpoint(ctx, checkpointID)
    checkpointable.RestoreFromCheckpoint(ctx, checkpoint.Metadata)
}
```

### Memory-Aware Agents

For agents that need access to long-term memory:

```go
type MemoryAwareProcessor struct {
    memory session.MemoryReader
}

func (p *MemoryAwareProcessor) SetMemory(ctx context.Context, memory session.MemoryReader) error {
    p.memory = memory
    return nil
}

func (p *MemoryAwareProcessor) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Search memory for relevant context
    entries, _ := p.memory.Search(ctx, input.Payload["query"], 5)

    // Use memory entries in processing
    context := buildContextFromMemory(entries)
    return p.processWithContext(input, context)
}
```

## HTTP Middleware

### Session Middleware Pattern

```go
func SessionMiddleware(mgr session.Manager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Get session ID from header or cookie
            sessionID := r.Header.Get("X-Session-ID")
            if sessionID == "" {
                // Create new session
                sess, _ := mgr.Create(r.Context(), "web-agent", session.CreateOptions{})
                sessionID = sess.ID()
                w.Header().Set("X-Session-ID", sessionID)
            }

            // Get session
            sess, err := mgr.Get(r.Context(), sessionID)
            if err != nil {
                http.Error(w, "Invalid session", http.StatusBadRequest)
                return
            }

            // Add to context
            ctx := session.ContextWithSession(r.Context(), sess)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

## Performance Considerations

### Memory Management

**File Backend**:

- Messages are cached in memory after first load
- Use `sess.Close()` to flush and release memory
- Consider pagination for very long sessions (>1000 messages)

**Best Practices**:

```go
// Close sessions when done
defer sess.Close(ctx)

// For long-running agents, periodically close idle sessions
if time.Since(lastActivity) > 30*time.Minute {
    sess.Close(ctx)
}
```

### Storage Performance

**File Backend Benchmarks** (typical workloads):

- Create session: <1ms
- Append message: <5ms
- Get messages (100 msgs): <10ms
- Create checkpoint: <5ms
- List sessions: <20ms per 100 sessions

**Optimization Tips**:

1. **Batch Message Appends**: Minimize storage I/O by grouping messages when possible
2. **Use Checkpoints Sparingly**: Only create checkpoints when state preservation is critical
3. **Clean Up Old Sessions**: Implement retention policies to remove expired sessions
4. **Consider Database Backend**: For >10,000 active sessions, use PostgreSQL or Firestore

### Concurrency

All session operations are thread-safe:

```go
// Safe to call from multiple goroutines
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        msg := agent.NewMessage("user", map[string]string{
            "content": fmt.Sprintf("Message %d", n),
        })
        sess.AppendMessage(ctx, msg)
    }(i)
}
wg.Wait()
```

## Security Considerations

### Access Control

**User Isolation**:

```go
// Ensure users can only access their own sessions
sess, err := mgr.GetOrCreate(ctx, "assistant", authenticatedUserID)

// Validate ownership before operations
if sess.UserID() != authenticatedUserID {
    return errors.New("unauthorized session access")
}
```

**Session ID Security**:

- Session IDs are UUIDs (cryptographically random)
- Do not expose session IDs in URLs or logs
- Use secure storage for session ID mapping

### Data Privacy

**File Permissions**:

```bash
# File backend uses restrictive permissions
chmod 700 ~/.aixgo/sessions          # Directory: owner only
chmod 600 ~/.aixgo/sessions/**/*.jsonl  # Files: owner read/write only
```

**Encryption**:

For sensitive data, consider:

1. Encrypting message payloads before storage
2. Using encrypted filesystems (LUKS, FileVault)
3. Database encryption at rest (PostgreSQL, Firestore)

**PII Handling**:

```go
// Sanitize PII before storing
msg := agent.NewMessage("user", map[string]string{
    "content": sanitizer.RemovePII(rawContent),
})
sess.AppendMessage(ctx, msg)
```

### Retention Policies

Implement automatic cleanup:

```go
// Delete sessions older than 30 days
func cleanupOldSessions(mgr session.Manager, agentName string) error {
    sessions, err := mgr.List(ctx, agentName, session.ListOptions{})
    if err != nil {
        return err
    }

    cutoff := time.Now().Add(-30 * 24 * time.Hour)
    for _, meta := range sessions {
        if meta.UpdatedAt.Before(cutoff) {
            if err := mgr.Delete(ctx, meta.ID); err != nil {
                log.Printf("Failed to delete session %s: %v", meta.ID, err)
            }
        }
    }
    return nil
}
```

## Best Practices

### 1. Always Close Sessions

```go
sess, _ := mgr.Create(ctx, "agent", opts)
defer sess.Close(ctx)
```

### 2. Use GetOrCreate for User Sessions

```go
// Prefer GetOrCreate over manual Create/Get
sess, _ := mgr.GetOrCreate(ctx, "agent", userID)
```

### 3. Checkpoint Before Risky Operations

```go
checkpoint, _ := sess.Checkpoint(ctx)

result, err := performRiskyOperation(sess)
if err != nil {
    sess.Restore(ctx, checkpoint.ID)
    return err
}
```

### 4. Handle Session Not Found

```go
sess, err := mgr.Get(ctx, sessionID)
if errors.Is(err, session.ErrSessionNotFound) {
    // Create new session or return error
    sess, err = mgr.Create(ctx, agentName, opts)
}
```

### 5. Clean Up Old Sessions Periodically

```go
// Periodically clean up old sessions
sessions, _ := mgr.List(ctx, agentName, session.ListOptions{})
for _, meta := range sessions {
    if time.Since(meta.UpdatedAt) > 30*24*time.Hour {
        mgr.Delete(ctx, meta.ID)
    }
}
```

## Examples

### Complete Example

See [`examples/session-basic/main.go`](../examples/session-basic/main.go) for a complete working example demonstrating:

- Session creation and management
- Message appending and retrieval
- Checkpoint creation and restoration
- Session resumption across restarts
- Context helper usage

Run the example:

```bash
cd examples/session-basic
go run main.go
```

### Common Patterns

**Multi-Tenant Chat Application**:

```go
func handleChatMessage(w http.ResponseWriter, r *http.Request) {
    userID := getUserIDFromAuth(r)
    agentName := "assistant"

    // Get or create session for user
    sess, err := sessionMgr.GetOrCreate(r.Context(), agentName, userID)
    if err != nil {
        http.Error(w, "Failed to get session", 500)
        return
    }

    // Process message with session context
    ctx := session.ContextWithSession(r.Context(), sess)
    response, err := agent.Execute(ctx, userMessage)
    if err != nil {
        http.Error(w, "Failed to process message", 500)
        return
    }

    json.NewEncoder(w).Encode(response)
}
```

**Conversation Branching**:

```go
// Save checkpoint before trying different approaches
checkpoint, err := sess.Checkpoint(ctx)
if err != nil {
    return err
}

// Try approach A
responseA, err := tryApproachA(ctx, sess)
if err != nil || !isSatisfactory(responseA) {
    // Rollback and try approach B
    sess.Restore(ctx, checkpoint.ID)
    responseB, err := tryApproachB(ctx, sess)
    return responseB, err
}

return responseA, nil
```

## Troubleshooting

### Common Issues

**"session not found" error**:

```go
// Ensure session exists before calling Get()
sess, err := mgr.Get(ctx, sessionID)
if errors.Is(err, session.ErrSessionNotFound) {
    // Create new session instead
    sess, err = mgr.Create(ctx, agentName, session.CreateOptions{})
}
```

**Permission denied errors**:

```bash
# Check file permissions
ls -la ~/.aixgo/sessions

# Fix permissions if needed
chmod -R 700 ~/.aixgo/sessions
```

**"storage backend is closed" error**:

```go
// Don't call operations after Close()
mgr.Close()
// mgr.Get(ctx, id) // Error: backend closed

// Create new manager instead
backend, _ := session.NewFileBackend("")
mgr = session.NewManager(backend)
```

**Memory growth with long sessions**:

```go
// Periodically close and reload sessions
if len(messages) > 1000 {
    sess.Close(ctx)
    sess, _ = mgr.Get(ctx, sess.ID()) // Reload fresh
}
```

**Session not persisting**:

1. Ensure `backend.Close()` is called on shutdown
2. Check file permissions for JSONL storage directory
3. Verify session ID is consistent across calls

**History not available in agent**:

1. Check agent implements `SessionAwareAgent`
2. Verify `CallWithSession` is being used
3. Confirm session exists and has messages

**Checkpoint restore fails**:

1. Check checkpoint ID is valid
2. Verify checkpoint hasn't been cleaned up
3. Ensure session hasn't been deleted

### Debug Mode

Enable verbose logging:

```go
// Log all session operations
type loggingBackend struct {
    session.StorageBackend
}

func (l *loggingBackend) AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error {
    log.Printf("AppendEntry: session=%s type=%s", sessionID, entry.Type)
    return l.StorageBackend.AppendEntry(ctx, sessionID, entry)
}
```

## Resources

- **Code**: [`pkg/session/`](../pkg/session/)
- **Examples**: [`examples/session-basic/`](../examples/session-basic/)
- **Feature Catalog**: [FEATURES.md](FEATURES.md)
- **Testing Guide**: [TESTING_GUIDE.md](TESTING_GUIDE.md)
- **Discussions**: [GitHub Discussions](https://github.com/orgs/aixgo-dev/discussions)

For questions or issues, open a discussion or file an issue on GitHub.
