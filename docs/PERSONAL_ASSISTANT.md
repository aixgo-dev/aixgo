# Personal Assistant: Task Continuation & Planning

- **Epic**: [#103](https://github.com/aixgo-dev/aixgo/issues/103) **Status**: 🚧 In Progress
- **Target Release**: v0.4.0 (March 15, 2026) **Milestones**: v0.4.0-alpha (Feb 28), v0.4.0 (Mar 15), v0.4.1 (Mar 31)

## Overview

The Personal Assistant feature transforms Aixgo agents from stateless processors into intelligent assistants that maintain task context, track progress, and continue work across
sessions. This builds on the existing v0.3.3 session system by adding working memory, task management, and intelligent planning capabilities.

**Design Philosophy**: Focus on the 20% that drives 80% of value - task completion and planning.

## Architecture

### Data Flow

```text
┌─────────────────────────────────────────────────────────────────┐
│                     User Interaction Layer                      │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Session System (v0.3.3)                      │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ SessionManager - JSONL/Redis/Firestore backends            │ │
│  │ • Checkpoint creation & restoration                        │ │
│  │ • Conversation history persistence                         │ │
│  └────────────────────────────────────────────────────────────┘ │
└────────────────┬────────────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────────────┐
│              Working Memory (NEW - Issue #105)                │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ WorkingMemory Interface:                                   │ │
│  │ • Store/Retrieve task state, context, artifacts            │ │
│  │ • Automatic cleanup policies (time/size limits)            │ │
│  │ • Context window optimization                              │ │
│  └────────────────────────────────────────────────────────────┘ │
└────────────┬────────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────────┐
│              Task Manager (NEW - Issues #104, #106)         │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ TaskState: pending → in_progress → blocked → completed     │ │
│  │ TaskQueue: Priority-based task scheduling                  │ │
│  │ • Dependencies & prerequisite tracking                     │ │
│  │ • Automatic retry policies                                 │ │
│  └────────────────────────────────────────────────────────────┘ │
└────────────┬────────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────────┐
│         Progress Tracker (NEW - Issue #107)                   │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Progress: percentage, milestones, estimated completion     │ │
│  │ • Real-time status updates                                 │ │
│  │ • Multi-task progress aggregation                          │ │
│  └────────────────────────────────────────────────────────────┘ │
└────────────┬────────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────────┐
│        Planning Agent (ENHANCED - Issues #108, #110)        │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ Existing: agents/planner.go + task continuation            │ │
│  │ • Task decomposition into sub-tasks                        │ │
│  │ • Dynamic re-planning based on progress                    │ │
│  │ • Integration with working memory & task queue             │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│           Runtime Integration (runtime.go)                      │
│  • Session-aware agent execution                                │
│  • Automatic checkpoint creation on state changes               │
│  • Task queue polling & execution                               │
└─────────────────────────────────────────────────────────────────┘
```

### Integration Points

**Session System** (`pkg/session/`):

- `Session.Checkpoint()` - Create state snapshots before task execution
- `Session.Restore()` - Resume from previous checkpoint on failure
- `Session.AppendMessage()` - Track task-related messages
- Storage backends: JSONL (default), Redis, Firestore

**Planner Agent** (`agents/planner.go`):

- Enhanced with task continuation awareness
- Reads from WorkingMemory to understand context
- Updates TaskQueue with decomposed sub-tasks
- Reports progress via Progress interface

**Runtime** (`runtime.go`):

- `Runtime.SessionManager` - Manages session lifecycle
- `Runtime.Call()` - Session-aware agent invocation
- New: `Runtime.TaskManager` - Task execution orchestration

## Core Data Schemas

### TaskState (Issue #104)

```go
// TaskState represents the complete state of a task
type TaskState struct {
    // Identity
    ID          string    `json:"id"`
    SessionID   string    `json:"sessionId"`
    ParentID    string    `json:"parentId,omitempty"` // For sub-tasks

    // Status
    Status      TaskStatus `json:"status"`
    Priority    int        `json:"priority"` // 1-10, higher = more urgent

    // Timing
    CreatedAt   time.Time  `json:"createdAt"`
    UpdatedAt   time.Time  `json:"updatedAt"`
    StartedAt   *time.Time `json:"startedAt,omitempty"`
    CompletedAt *time.Time `json:"completedAt,omitempty"`

    // Content
    Description string              `json:"description"`
    Context     map[string]any      `json:"context"`
    Dependencies []string           `json:"dependencies"` // Task IDs
    Artifacts   []TaskArtifact      `json:"artifacts"`

    // Progress
    Progress    *Progress          `json:"progress,omitempty"`

    // Retry Policy
    RetryCount  int                `json:"retryCount"`
    MaxRetries  int                `json:"maxRetries"`
    LastError   string             `json:"lastError,omitempty"`
}

type TaskStatus string

const (
    TaskPending     TaskStatus = "pending"
    TaskInProgress  TaskStatus = "in_progress"
    TaskBlocked     TaskStatus = "blocked"
    TaskCompleted   TaskStatus = "completed"
    TaskFailed      TaskStatus = "failed"
    TaskCancelled   TaskStatus = "cancelled"
)

type TaskArtifact struct {
    Type      string         `json:"type"`      // "file", "data", "url"
    Content   string         `json:"content"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}
```

### WorkingMemory (Issue #105)

```go
// WorkingMemory provides short-term context storage for agents
type WorkingMemory interface {
    // Store saves a key-value pair with optional TTL
    Store(ctx context.Context, sessionID, key string, value any, ttl time.Duration) error

    // Retrieve gets a value by key
    Retrieve(ctx context.Context, sessionID, key string) (any, error)

    // List returns all keys for a session
    List(ctx context.Context, sessionID string) ([]string, error)

    // Delete removes a key
    Delete(ctx context.Context, sessionID, key string) error

    // Clear removes all data for a session
    Clear(ctx context.Context, sessionID string) error

    // GetSummary returns optimized context for LLM
    GetSummary(ctx context.Context, sessionID string, maxTokens int) (string, error)
}

// WorkingMemoryConfig controls memory behavior
type WorkingMemoryConfig struct {
    MaxEntries      int           `yaml:"max_entries"`       // Default: 1000
    DefaultTTL      time.Duration `yaml:"default_ttl"`       // Default: 24h
    CleanupInterval time.Duration `yaml:"cleanup_interval"`  // Default: 1h
    Backend         string        `yaml:"backend"`           // "memory", "redis"

    // Context window optimization
    SummaryStrategy string        `yaml:"summary_strategy"`  // "recent", "important", "semantic"
    MaxSummarySize  int           `yaml:"max_summary_size"`  // Tokens, default: 2000
}
```

### TaskQueue (Issue #106)

```go
// TaskQueue manages prioritized task execution
type TaskQueue interface {
    // Enqueue adds a task to the queue
    Enqueue(ctx context.Context, task *TaskState) error

    // Dequeue retrieves the highest priority task
    Dequeue(ctx context.Context, sessionID string) (*TaskState, error)

    // Peek views next task without removing it
    Peek(ctx context.Context, sessionID string) (*TaskState, error)

    // Update modifies an existing task
    Update(ctx context.Context, task *TaskState) error

    // List returns all tasks for a session
    List(ctx context.Context, sessionID string, filter TaskFilter) ([]*TaskState, error)

    // Size returns queue length
    Size(ctx context.Context, sessionID string) (int, error)
}

type TaskFilter struct {
    Status    []TaskStatus `json:"status,omitempty"`
    MinPriority int        `json:"min_priority,omitempty"`
    ParentID  string       `json:"parent_id,omitempty"`
}
```

### Progress (Issue #107)

```go
// Progress tracks task completion status
type Progress struct {
    // Percentage
    Percent       float64    `json:"percent"`        // 0.0 - 100.0

    // Milestones
    CurrentStep   int        `json:"currentStep"`
    TotalSteps    int        `json:"totalSteps"`
    Milestones    []Milestone `json:"milestones"`

    // Estimation
    EstimatedCompletion *time.Time `json:"estimatedCompletion,omitempty"`
    TimeElapsed   time.Duration `json:"timeElapsed"`

    // Status
    Message       string     `json:"message"`
    LastUpdated   time.Time  `json:"lastUpdated"`
}

type Milestone struct {
    Name        string    `json:"name"`
    Completed   bool      `json:"completed"`
    CompletedAt *time.Time `json:"completedAt,omitempty"`
}
```

## Configuration Example

Complete YAML configuration demonstrating all Personal Assistant features:

```yaml
supervisor:
  name: personal-assistant
  model: gpt-4-turbo

  # Session configuration (v0.3.3)
  session:
    enabled: true
    backend: jsonl
    storage_path: ./data/sessions

    # Working memory configuration (NEW)
    working_memory:
      max_entries: 1000
      default_ttl: 24h
      cleanup_interval: 1h
      backend: memory # or "redis"
      summary_strategy: semantic
      max_summary_size: 2000

    # Task queue configuration (NEW)
    task_queue:
      max_queue_size: 100
      priority_scheduling: true
      checkpoint_interval: 5m
      persistence: true

agents:
  - name: planner
    role: planner
    model: gpt-4-turbo

    planner_config:
      planning_strategy: chain_of_thought
      max_steps: 20
      enable_backtracking: true
      enable_self_critique: true

      # Task continuation (NEW)
      task_continuation:
        enabled: true
        auto_checkpoint: true
        checkpoint_interval: 10m
        resume_on_restart: true

      # Progress tracking (NEW)
      progress_tracking:
        enabled: true
        update_interval: 30s
        milestone_tracking: true

      # Working memory integration (NEW)
      working_memory:
        enabled: true
        context_size: 2000 # tokens
        include_artifacts: true

    outputs: [executor]

  - name: executor
    role: react
    model: gpt-4-turbo
    prompt: 'Execute tasks from the planning queue...'
    inputs: [planner]

    # Task execution configuration (NEW)
    task_execution:
      max_concurrent: 3
      retry_policy:
        max_retries: 3
        backoff: exponential
        initial_delay: 1s

      # Progress reporting (NEW)
      progress_reporting:
        enabled: true
        report_interval: 10s
```

## Cross-Issue Dependencies

Visual representation of issue relationships:

```text
Epic #103: Personal Assistant
    │
    ├─── Core State Management
    │    ├── #104: Task State Persistence ────────┐
    │    └── #105: Working Memory                 │
    │                                               │
    ├─── Task Orchestration                         ▼
    │    ├── #106: Task Queue ◄────── Dependencies from #104
    │    └── #107: Progress Tracking ◄─────────────┘
    │
    ├─── Intelligence Layer
    │    ├── #108: Enhanced Planning ◄── Uses #105 Working Memory
    │    └── #110: Task Decomposition ◄── Feeds #106 Task Queue
    │
    └─── Maintenance
         ├── #109: Cleanup Policies ◄── Applies to #105 + #106
         └── #111: Resume Logic ◄──────── Integrates all above

Implementation Order:
1. #104 (TaskState) - Foundation for all task features
2. #105 (WorkingMemory) - Parallel with #104
3. #106 (TaskQueue) - Depends on #104
4. #107 (Progress) - Depends on #104
5. #108 (Planning) - Depends on #105, #106
6. #110 (Decomposition) - Depends on #106, #108
7. #109 (Cleanup) - Depends on #105, #106
8. #111 (Resume) - Integration of all features
```

### Dependency Details

**Critical Path** (blocks other work):

- #104 (TaskState) → #106 (TaskQueue) → #110 (Decomposition)

**Parallel Work** (can develop simultaneously):

- #105 (WorkingMemory) + #104 (TaskState)
- #107 (Progress) + #106 (TaskQueue)

**Integration Phase** (requires multiple components):

- #108 (Enhanced Planning): Needs #105 + #106
- #111 (Resume Logic): Needs #104 + #105 + #106

## Testing Strategy

### End-to-End Test Scenarios

```go
// Test: Task continuation across sessions
func TestTaskContinuation(t *testing.T) {
    // 1. Create session with active task
    session := createSessionWithTask(t, "Write documentation")

    // 2. Execute partial work
    executeTaskSteps(t, session, 3) // Complete 3 of 10 steps

    // 3. Create checkpoint
    checkpoint, err := session.Checkpoint(ctx)
    require.NoError(t, err)

    // 4. Simulate restart
    runtime.Shutdown(ctx)
    runtime = NewRuntime(...)

    // 5. Restore session
    restored := runtime.SessionManager.Get(ctx, session.ID())
    err = restored.Restore(ctx, checkpoint.ID)
    require.NoError(t, err)

    // 6. Resume task execution
    progress := resumeTask(t, restored)
    assert.Equal(t, 30.0, progress.Percent) // 3/10 = 30%

    // 7. Complete remaining work
    executeTaskSteps(t, restored, 7)
    assert.Equal(t, 100.0, progress.Percent)
}

// Test: Multi-task progress aggregation
func TestMultiTaskProgress(t *testing.T) {
    tasks := []string{
        "Research competitors",
        "Design architecture",
        "Implement prototype",
    }

    // Track overall progress across all tasks
    tracker := NewProgressAggregator(tasks)

    completeTask(t, tasks[0]) // 33%
    assert.InDelta(t, 33.3, tracker.OverallProgress(), 0.1)

    completeTask(t, tasks[1]) // 66%
    assert.InDelta(t, 66.6, tracker.OverallProgress(), 0.1)
}
```

### Performance Targets

**Working Memory**:

- Store operation: <5ms (p99)
- Retrieve operation: <3ms (p99)
- Summary generation: <100ms for 1000 entries

**Task Queue**:

- Enqueue operation: <10ms (p99)
- Dequeue operation: <10ms (p99)
- Queue persistence: Every 5 minutes or 100 operations

**Session Restoration**:

- Checkpoint creation: <50ms (p99)
- Restore from checkpoint: <200ms (p99)
- Full session load: <1s for 1000 messages

**Memory Limits**:

- Working memory: Max 100MB per session
- Task queue: Max 1000 pending tasks per session
- Session cache: Max 50 concurrent sessions in memory

## Issue Reference Table

| Issue                                                 | Component              | Status         | Milestone    | Dependencies |
| ----------------------------------------------------- | ---------------------- | -------------- | ------------ | ------------ |
| [#104](https://github.com/aixgo-dev/aixgo/issues/104) | Task State Persistence | 🚧 In Progress | v0.4.0-alpha | -            |
| [#105](https://github.com/aixgo-dev/aixgo/issues/105) | Working Memory         | 🚧 In Progress | v0.4.0-alpha | -            |
| [#106](https://github.com/aixgo-dev/aixgo/issues/106) | Task Queue             | 🔮 Roadmap     | v0.4.0-alpha | #104         |
| [#107](https://github.com/aixgo-dev/aixgo/issues/107) | Progress Tracking      | 🔮 Roadmap     | v0.4.0       | #104         |
| [#108](https://github.com/aixgo-dev/aixgo/issues/108) | Enhanced Planning      | 🔮 Roadmap     | v0.4.0       | #105, #106   |
| [#109](https://github.com/aixgo-dev/aixgo/issues/109) | Cleanup Policies       | 🔮 Roadmap     | v0.4.0       | #105, #106   |
| [#110](https://github.com/aixgo-dev/aixgo/issues/110) | Task Decomposition     | 🔮 Roadmap     | v0.4.0       | #106, #108   |
| [#111](https://github.com/aixgo-dev/aixgo/issues/111) | Resume Logic           | 🔮 Roadmap     | v0.4.1       | All above    |

**Legend**:

- 🚧 In Progress - Active development
- 🔮 Roadmap - Planned, not started
- ✅ Implemented - Complete

## Implementation Notes

### Backwards Compatibility

All Personal Assistant features are **opt-in** via configuration. Existing Aixgo applications continue to work without changes.

### File Structure

```text
pkg/
  session/              # Existing session system (v0.3.3)
    session.go
    types.go
    backends/
      jsonl.go
      redis.go
      firestore.go

  taskmanager/          # NEW - Task management
    taskstate.go        # Issue #104
    taskqueue.go        # Issue #106
    progress.go         # Issue #107

  workingmemory/        # NEW - Working memory
    memory.go           # Issue #105
    backends/
      memory.go         # In-memory backend
      redis.go          # Redis backend

agents/
  planner.go            # ENHANCED - Task continuation (#108, #110)

runtime.go              # ENHANCED - Task manager integration (#111)
```

### Security Considerations

- **Task Isolation**: Tasks from different sessions never access each other's data
- **Input Validation**: All task descriptions and context data validated before storage
- **Resource Limits**: Per-session quotas prevent memory exhaustion
- **Audit Logging**: All task state transitions logged for debugging

### Observability

OpenTelemetry spans for all operations:

- `task.create` - Task creation
- `task.execute` - Task execution
- `task.checkpoint` - Checkpoint creation
- `working_memory.store` - Memory operations
- `queue.enqueue` / `queue.dequeue` - Queue operations

Metrics tracked:

- `task_duration_seconds` - Task completion time
- `task_retry_count` - Retry attempts
- `queue_depth` - Current queue size
- `working_memory_size_bytes` - Memory usage

## Resources

- **Epic Discussion**: [#103](https://github.com/aixgo-dev/aixgo/discussions/103)
- **Session System**: [docs/SESSIONS.md](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/docs/SESSIONS.md)
- **Planner Agent**: [agents/planner.go](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/agents/planner.go)
- **Runtime**: [runtime.go](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/runtime.go)

## Changelog

| Version | Date       | Changes                                  |
| ------- | ---------- | ---------------------------------------- |
| 0.1.0   | 2026-02-12 | Initial specification based on Epic #103 |
