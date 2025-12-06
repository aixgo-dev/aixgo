# Migration Guide: v1.x to v2.0

**Migrating from Aixgo v1.x to v2.0**

## Overview

Aixgo v2.0 is a **major version** with breaking changes. This guide walks through migrating your existing v1.x code to v2.0.

### Why Breaking Changes?

v2.0 introduces fundamental improvements that require API changes:

1. **Unified Agent Interface**: Supports both sync and async execution
2. **Runtime Abstraction**: Enables local and distributed deployment without code changes
3. **Pattern-First Design**: 9 orchestration patterns as first-class citizens
4. **Automatic Observability**: Built-in cost and token tracking

### Migration Timeline

- **Preparation**: 1-2 hours (read this guide)
- **Code Migration**: 2-4 hours (update agents and runtime)
- **Testing**: 2-4 hours (validate changes)
- **Total**: **5-10 hours** for typical project

---

## Breaking Changes Summary

### 1. Agent Interface

**v1.x**:
```go
type Agent interface {
    Start(ctx context.Context) error
}
```

**v2.0**:
```go
type Agent interface {
    Name() string
    Role() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Execute(ctx context.Context, input *Message) (*Message, error)
    Ready() bool
}
```

**Impact**: All custom agents must implement new methods.

### 2. Runtime Interface

**v1.x**:
```go
type Runtime interface {
    Send(target string, msg *Message) error
    Recv(source string) (<-chan *Message, error)
}

// Usage
rt := aixgo.NewSimpleRuntime()
```

**v2.0**:
```go
type Runtime interface {
    // Old methods (kept)
    Send(target string, msg *Message) error
    Recv(source string) (<-chan *Message, error)

    // New methods
    Register(name string, ag Agent) error
    Call(ctx context.Context, target string, input *Message) (*Message, error)
    CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, error)
    Broadcast(ctx context.Context, targets []string, msg *Message) error
    // ... more methods
}

// Usage
rt := runtime.NewLocalRuntime()
rt.Register("agent1", agent1)
```

**Impact**: Change runtime creation, add agent registration.

### 3. Configuration Structure

**v1.x**:
```yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  - name: agent1
    role: react
```

**v2.0**:
```yaml
orchestration:
  pattern: supervisor  # NEW: pattern selection
  config:
    model: gpt-4-turbo
    max_rounds: 10
  agents:
    - agent1
    - agent2

agents:
  - name: agent1
    role: react
```

**Impact**: Restructure config, add `orchestration` section.

### 4. Observability

**v1.x**: Manual token tracking in agents
```go
result.TokensUsed = resp.Usage.TotalTokens  // Manual
```

**v2.0**: Automatic tracking via instrumented providers
```go
// No manual tracking needed - automatic!
```

**Impact**: Remove manual token tracking code (it's automatic now).

---

## Step-by-Step Migration

### Step 1: Update Agent Implementations

#### Before (v1.x)

```go
package agents

type MyAgent struct {
    name string
    provider provider.Provider
}

func (a *MyAgent) Start(ctx context.Context) error {
    // Long-running loop
    for {
        // Process messages
    }
    return nil
}
```

#### After (v2.0)

```go
package agents

import "github.com/aixgo-dev/aixgo/internal/agent"

type MyAgent struct {
    name string
    provider provider.Provider
}

// NEW: Metadata methods
func (a *MyAgent) Name() string { return a.name }
func (a *MyAgent) Role() string { return "custom" }

// NEW: Request-response execution
func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Synchronous processing
    result, err := a.processMessage(ctx, input)
    if err != nil {
        return nil, err
    }

    return &agent.Message{
        Payload: result,
        Metadata: map[string]any{
            "processed_by": a.name,
        },
    }, nil
}

// EXISTING: Long-running mode
func (a *MyAgent) Start(ctx context.Context) error {
    // Long-running loop (unchanged)
    for {
        // Process messages
    }
    return nil
}

// NEW: Graceful shutdown
func (a *MyAgent) Stop(ctx context.Context) error {
    // Cleanup resources
    return nil
}

// NEW: Health check
func (a *MyAgent) Ready() bool {
    return a.provider != nil
}

// HELPER: Shared processing logic
func (a *MyAgent) processMessage(ctx context.Context, input *agent.Message) (string, error) {
    // Business logic here
    return "result", nil
}
```

**Key Changes**:
1. Add `Name()`, `Role()`, `Stop()`, `Ready()` methods
2. Add `Execute()` for request-response pattern
3. Keep `Start()` for long-running reactive pattern
4. Extract common logic to helper methods

#### Migration Checklist

For each custom agent:
- [ ] Add `Name() string` method
- [ ] Add `Role() string` method
- [ ] Add `Execute(ctx, input) (output, error)` method
- [ ] Add `Stop(ctx) error` method
- [ ] Add `Ready() bool` method
- [ ] Test both `Start()` and `Execute()` modes

### Step 2: Update Runtime Usage

#### Before (v1.x)

```go
package main

import "github.com/aixgo-dev/aixgo"

func main() {
    // Create simple runtime
    rt := aixgo.NewSimpleRuntime()

    // Agents auto-registered?
    // (implicit in v1.x)
}
```

#### After (v2.0)

```go
package main

import (
    "github.com/aixgo-dev/aixgo/internal/runtime"
    "github.com/aixgo-dev/aixgo/internal/agent"
)

func main() {
    // Create local runtime
    rt := runtime.NewLocalRuntime()

    // EXPLICITLY register agents
    rt.Register("agent1", agent1)
    rt.Register("agent2", agent2)
    rt.Register("agent3", agent3)
}
```

**Key Changes**:
1. Import `internal/runtime` instead of using `aixgo.NewSimpleRuntime()`
2. Explicitly call `rt.Register(name, agent)` for each agent
3. Agent names are used for routing

#### Migration Checklist

- [ ] Replace `aixgo.NewSimpleRuntime()` with `runtime.NewLocalRuntime()`
- [ ] Add `rt.Register(name, agent)` for each agent
- [ ] Ensure agent names match config references

### Step 3: Update Orchestration

#### Before (v1.x)

```go
package main

import "github.com/aixgo-dev/aixgo/internal/supervisor"

func main() {
    // Create supervisor
    supervisor := supervisor.New(supervisor.SupervisorDef{
        Name:      "coordinator",
        Model:     "gpt-4-turbo",
        MaxRounds: 10,
    }, agents, runtime)

    // Run
    result, _ := supervisor.Run(ctx, "user input")
}
```

#### After (v2.0) - Option A: Keep Supervisor

```go
package main

import (
    "github.com/aixgo-dev/aixgo/internal/supervisor"
    "github.com/aixgo-dev/aixgo/internal/runtime"
)

func main() {
    // Runtime with registered agents
    rt := runtime.NewLocalRuntime()
    rt.Register("agent1", agent1)
    rt.Register("agent2", agent2)

    // Create supervisor (API unchanged)
    supervisor := supervisor.New(supervisor.SupervisorDef{
        Name:      "coordinator",
        Model:     "gpt-4-turbo",
        MaxRounds: 10,
    }, agents, rt)  // Pass registered agents map

    // Run (API unchanged)
    result, _ := supervisor.Run(ctx, "user input")
}
```

**Good news**: Supervisor API is mostly unchanged! Just update runtime.

#### After (v2.0) - Option B: Use New Patterns

```go
package main

import (
    "github.com/aixgo-dev/aixgo/internal/runtime"
    "github.com/aixgo-dev/aixgo/internal/orchestration"
)

func main() {
    // Runtime with registered agents
    rt := runtime.NewLocalRuntime()
    rt.Register("agent1", agent1)
    rt.Register("agent2", agent2)
    rt.Register("agent3", agent3)

    // Use parallel pattern for concurrent execution
    orchestrator := orchestration.NewParallel(
        "parallel-workflow",
        rt,
        []string{"agent1", "agent2", "agent3"},
    )

    // Execute
    result, _ := orchestrator.Execute(ctx, input)
}
```

**Benefit**: Access to new patterns (Parallel, Router, Swarm, etc.)

#### Migration Checklist

- [ ] Decide: Keep Supervisor or migrate to new patterns?
- [ ] If keeping Supervisor: Update runtime only
- [ ] If migrating: Choose appropriate pattern (see [Pattern Selection](./PATTERNS.md#pattern-selection-guide))
- [ ] Update orchestration code
- [ ] Test end-to-end workflow

### Step 4: Update Configuration

#### Before (v1.x)

```yaml
# config.yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  - name: agent1
    role: react
    model: gpt-3.5-turbo
    prompt: |
      You are a helpful assistant.

  - name: agent2
    role: classifier
    model: gpt-3.5-turbo
```

#### After (v2.0)

```yaml
# config.yaml
orchestration:
  pattern: supervisor  # or parallel, sequential, router, etc.
  config:
    model: gpt-4-turbo
    max_rounds: 10
  agents:
    - agent1
    - agent2

agents:
  - name: agent1
    role: react
    model: gpt-3.5-turbo
    prompt: |
      You are a helpful assistant.

  - name: agent2
    role: classifier
    model: gpt-3.5-turbo
```

**Key Changes**:
1. Add `orchestration` section
2. Specify `pattern` (supervisor, parallel, sequential, etc.)
3. Move orchestration config to `orchestration.config`
4. List agent names in `orchestration.agents`

#### Pattern-Specific Configs

**Parallel Pattern**:
```yaml
orchestration:
  pattern: parallel
  config:
    timeout: 60s
    aggregation: voting
  agents:
    - agent1
    - agent2
    - agent3
```

**Router Pattern**:
```yaml
orchestration:
  pattern: router
  config:
    classifier: intent-classifier
    fallback: general-agent
    routes:
      technical: tech-agent
      billing: billing-agent
  agents:
    - intent-classifier
    - tech-agent
    - billing-agent
    - general-agent
```

**Sequential Pattern**:
```yaml
orchestration:
  pattern: sequential
  agents:
    - step1-agent
    - step2-agent
    - step3-agent
```

#### Migration Checklist

- [ ] Add `orchestration` section to config
- [ ] Set `pattern` field
- [ ] Move orchestration-specific config to `orchestration.config`
- [ ] List agent names in `orchestration.agents`
- [ ] Validate config with `go run main.go --validate-config`

### Step 5: Remove Manual Token Tracking

#### Before (v1.x)

```go
package agents

func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    resp, err := a.provider.CreateCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    // MANUAL token tracking
    result := &agent.Message{
        Payload: resp.Content,
        Metadata: map[string]any{
            "tokens_used": resp.Usage.TotalTokens,  // ❌ Manual
        },
    }

    return result, nil
}
```

#### After (v2.0)

```go
package agents

func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Provider is automatically instrumented!
    resp, err := a.provider.CreateCompletion(ctx, req)
    if err != nil {
        return nil, err
    }

    // NO manual tracking needed - automatic via instrumentation!
    result := &agent.Message{
        Payload: resp.Content,
        // ✅ tokens_used, cost_usd added automatically by instrumented provider
    }

    return result, nil
}
```

**Key Changes**:
1. Remove manual token tracking
2. Providers are auto-instrumented (wrapped by v2.0 framework)
3. Token and cost metadata added automatically

#### Migration Checklist

- [ ] Remove all manual `tokens_used` tracking
- [ ] Remove all manual cost calculations
- [ ] Verify automatic tracking in Langfuse dashboard
- [ ] Check metrics in observability UI

### Step 6: Update Tests

#### Before (v1.x)

```go
package agents

import "testing"

func TestMyAgent(t *testing.T) {
    agent := NewMyAgent("test-agent")

    // Only Start() method tested
    err := agent.Start(context.Background())
    if err != nil {
        t.Errorf("Start failed: %v", err)
    }
}
```

#### After (v2.0)

```go
package agents

import (
    "context"
    "testing"
    "github.com/aixgo-dev/aixgo/internal/agent"
)

func TestMyAgent_Execute(t *testing.T) {
    agent := NewMyAgent("test-agent")

    // Test Execute() method
    input := &agent.Message{Payload: "test input"}
    output, err := agent.Execute(context.Background(), input)

    if err != nil {
        t.Errorf("Execute failed: %v", err)
    }

    if output.Payload == "" {
        t.Error("Expected non-empty output")
    }
}

func TestMyAgent_Start(t *testing.T) {
    agent := NewMyAgent("test-agent")

    // Test Start() method (if applicable)
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    err := agent.Start(ctx)
    if err != context.DeadlineExceeded {
        t.Errorf("Expected timeout, got: %v", err)
    }
}

func TestMyAgent_Metadata(t *testing.T) {
    agent := NewMyAgent("test-agent")

    // Test new methods
    if agent.Name() != "test-agent" {
        t.Errorf("Name() = %s, want test-agent", agent.Name())
    }

    if agent.Role() != "custom" {
        t.Errorf("Role() = %s, want custom", agent.Role())
    }

    if !agent.Ready() {
        t.Error("Ready() = false, want true")
    }
}
```

**Key Changes**:
1. Test both `Execute()` and `Start()` methods
2. Test metadata methods (`Name()`, `Role()`, `Ready()`)
3. Test graceful shutdown (`Stop()`)

#### Migration Checklist

- [ ] Add tests for `Execute()` method
- [ ] Add tests for metadata methods
- [ ] Add tests for `Stop()` method
- [ ] Run `go test ./...` to validate
- [ ] Run `go test -race ./...` to check for race conditions

---

## Common Migration Scenarios

### Scenario 1: Simple Reactive Agent

**v1.x Code**:
```go
type LoggerAgent struct {
    name string
}

func (a *LoggerAgent) Start(ctx context.Context) error {
    ch, _ := runtime.Recv(a.name)
    for msg := range ch {
        log.Println(msg.Payload)
    }
    return nil
}
```

**v2.0 Migration**:
```go
type LoggerAgent struct {
    name   string
    stopCh chan struct{}
}

func (a *LoggerAgent) Name() string { return a.name }
func (a *LoggerAgent) Role() string { return "logger" }

func (a *LoggerAgent) Start(ctx context.Context) error {
    rt, _ := agent.RuntimeFromContext(ctx)
    ch, _ := rt.Recv(a.name)

    for {
        select {
        case msg := <-ch:
            log.Println(msg.Payload)
        case <-a.stopCh:
            return nil
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (a *LoggerAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Loggers don't support request-response
    return nil, fmt.Errorf("logger doesn't support Execute()")
}

func (a *LoggerAgent) Stop(ctx context.Context) error {
    close(a.stopCh)
    return nil
}

func (a *LoggerAgent) Ready() bool {
    select {
    case <-a.stopCh:
        return false
    default:
        return true
    }
}
```

**Effort**: ~30 minutes

### Scenario 2: Request-Response Agent

**v1.x Code**:
```go
type ClassifierAgent struct {
    name     string
    provider provider.Provider
}

func (a *ClassifierAgent) Start(ctx context.Context) error {
    // Awkward: Listening for messages, then calling provider
    ch, _ := runtime.Recv(a.name)
    for msg := range ch {
        resp, _ := a.provider.CreateCompletion(ctx, ...)
        runtime.Send(msg.ReplyTo, resp)
    }
    return nil
}
```

**v2.0 Migration**:
```go
type ClassifierAgent struct {
    name     string
    provider provider.Provider
}

func (a *ClassifierAgent) Name() string { return a.name }
func (a *ClassifierAgent) Role() string { return "classifier" }

func (a *ClassifierAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Clean request-response!
    resp, err := a.provider.CreateCompletion(ctx, provider.CompletionRequest{
        Model:  "gpt-3.5-turbo",
        Prompt: input.Payload,
    })
    if err != nil {
        return nil, err
    }

    return &agent.Message{
        Payload: resp.Content,
        Metadata: map[string]any{
            "category":   resp.Category,
            "confidence": resp.Confidence,
        },
    }, nil
}

func (a *ClassifierAgent) Start(ctx context.Context) error {
    // Optional: Also support reactive mode
    rt, _ := agent.RuntimeFromContext(ctx)
    ch, _ := rt.Recv(a.name)

    for {
        select {
        case msg := <-ch:
            result, _ := a.Execute(ctx, msg)
            rt.Send(msg.ReplyTo, result)
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (a *ClassifierAgent) Stop(ctx context.Context) error { return nil }
func (a *ClassifierAgent) Ready() bool { return true }
```

**Effort**: ~45 minutes

**Benefit**: Much cleaner API with `Execute()`!

### Scenario 3: Migrating Supervisor Config

**v1.x Config**:
```yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10
  routing_strategy: best_match

agents:
  - name: billing-agent
    role: react
    model: gpt-3.5-turbo

  - name: tech-agent
    role: react
    model: gpt-4-turbo
```

**v2.0 Config**:
```yaml
orchestration:
  pattern: supervisor
  config:
    model: gpt-4-turbo
    max_rounds: 10
    routing_strategy: best_match
  agents:
    - billing-agent
    - tech-agent

agents:
  - name: billing-agent
    role: react
    model: gpt-3.5-turbo

  - name: tech-agent
    role: react
    model: gpt-4-turbo
```

**Effort**: 10 minutes

---

## Migration Checklist

### Preparation
- [ ] Read this migration guide
- [ ] Review [Architecture v2.0](./ARCHITECTURE_V2.md)
- [ ] Review [Pattern Catalog](./PATTERNS.md)
- [ ] Back up existing code
- [ ] Create migration branch

### Code Changes
- [ ] Update all agent implementations (add new methods)
- [ ] Update runtime creation (`NewLocalRuntime()`)
- [ ] Update agent registration (`rt.Register()`)
- [ ] Update orchestration (supervisor or new patterns)
- [ ] Update configuration files
- [ ] Remove manual token tracking
- [ ] Update all tests

### Testing
- [ ] Run `go test ./...`
- [ ] Run `go test -race ./...`
- [ ] Test each agent in isolation
- [ ] Test end-to-end workflows
- [ ] Verify observability in Langfuse dashboard
- [ ] Performance testing

### Deployment
- [ ] Update documentation
- [ ] Update deployment scripts
- [ ] Deploy to staging
- [ ] Smoke test in staging
- [ ] Deploy to production

---

## Rollback Plan

If migration fails or causes issues:

### Option 1: Revert to v1.x

```bash
# In go.mod
require github.com/aixgo-dev/aixgo v1.x.x  # Pin to v1.x

go mod tidy
```

### Option 2: Gradual Migration

Migrate one component at a time:

1. **Week 1**: Update agent interfaces only
2. **Week 2**: Update runtime and registration
3. **Week 3**: Update orchestration
4. **Week 4**: Update configs and deploy

### Option 3: Dual-Version Support

Run v1.x and v2.0 side-by-side:
- v1.x for production traffic
- v2.0 for shadow traffic (testing)

---

## Getting Help

**Stuck on migration?**

1. **Check examples**: `examples/` directory has v2.0 examples
2. **Read docs**: [Architecture](./ARCHITECTURE_V2.md), [Patterns](./PATTERNS.md)
3. **GitHub Issues**: [Report migration issues](https://github.com/aixgo-dev/aixgo/issues)
4. **Discord**: [Join community](#) (coming soon)

**Common Issues**:

**Issue**: `Agent doesn't implement Execute()`
**Fix**: Add `Execute()` method to your agent

**Issue**: `Runtime not found in context`
**Fix**: Ensure runtime is registered and passed via context

**Issue**: `Agent not found` in runtime
**Fix**: Call `rt.Register(name, agent)` before using agent

---

## Benefits After Migration

After migrating to v2.0, you gain:

✅ **9 Orchestration Patterns**: Parallel, Router, Swarm, RAG, Reflection, Ensemble, etc.

✅ **Automatic Cost Tracking**: No more manual token counting

✅ **Deployment Flexibility**: Single binary OR distributed with zero code changes

✅ **Better Testing**: Clean `Execute()` API for unit tests

✅ **Production Observability**: Langfuse integration out-of-the-box

✅ **Performance**: Parallel execution, optimized routing

---

**Questions?** See [FAQ](./FAQ.md) or [open an issue](https://github.com/aixgo-dev/aixgo/issues).

**Ready to migrate?** Let's go! 🚀
