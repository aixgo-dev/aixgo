# Aixgo Framework v2 Architecture

**Version**: 2.0.0-alpha
**Status**: Design Document
**Last Updated**: 2025-01-16

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Core Principles](#core-principles)
3. [Architecture Overview](#architecture-overview)
4. [Agent Interface Redesign](#agent-interface-redesign)
5. [Runtime Abstraction](#runtime-abstraction)
6. [Orchestration Patterns](#orchestration-patterns)
7. [Observability & Cost Tracking](#observability--cost-tracking)
8. [Package Structure](#package-structure)
9. [Implementation Roadmap](#implementation-roadmap)
10. [Migration Guide](#migration-guide)

---

## Executive Summary

Aixgo v2 introduces a **unified orchestration architecture** supporting 9 production-proven agent patterns while maintaining the ability to deploy as a **single binary** or **distributed system** without code changes.

### Key Improvements

**1. Unified Agent Interface**
- Supports both synchronous (request-response) and asynchronous (message-passing) execution
- Enables clean pattern implementations
- Single interface works for both local and distributed deployment

**2. Pattern-First Design**
- 9 orchestration patterns: Supervisor, Sequential, Parallel, Router, Swarm, Hierarchical, RAG, Reflection, Ensemble
- Each pattern is a first-class citizen with standardized interfaces
- Patterns compose and nest cleanly

**3. Production Observability**
- Automatic token and cost tracking for all LLM calls
- Pattern-specific metrics (e.g., ensemble agreement rate, parallel wait time)
- Langfuse integration with Generations API
- OpenTelemetry for backend flexibility

**4. Deployment Flexibility**
- `LocalRuntime`: In-process execution with Go channels (single binary)
- `DistributedRuntime`: gRPC-based multi-process deployment
- Same agent code, zero code changes between deployments

### Breaking Changes

⚠️ **This is a major version change (v2.0.0)** - backward compatibility is NOT maintained.

- Agent interface changed: Added `Execute()` method
- Runtime interface extended: Added `Call()`, `CallParallel()`, `Broadcast()`
- Config structure reorganized: Added `orchestration` section
- Observability enhanced: Automatic cost tracking

**Migration timeline**: Examples and documentation will be updated in this release.

---

## Core Principles

### 1. Deployment Agnostic
**Principle**: Write once, deploy anywhere.

```go
// Same agent code
type AnalysisAgent struct {}

func (a *AnalysisAgent) Execute(ctx context.Context, input *Message) (*Message, error) {
    // Business logic
    return &Message{Payload: "result"}, nil
}

// Local deployment (single binary)
runtime := NewLocalRuntime()
runtime.Register("analysis", analysisAgent)

// Distributed deployment (NO CODE CHANGES)
runtime := NewDistributedRuntime()
runtime.Connect("analysis", "analysis-service:50051")
```

### 2. Observability First
**Principle**: Every operation is traced, every LLM call is costed.

- Automatic instrumentation at provider level
- Zero manual tracking required
- Pattern-specific metrics built-in
- Langfuse integration out-of-the-box

### 3. Pattern Composability
**Principle**: Patterns should nest and compose cleanly.

```go
// Hierarchical supervisor with parallel sub-tasks
hierarchical := orchestration.NewHierarchical(
    supervisor,
    map[string]Orchestrator{
        "analysis": orchestration.NewParallel("agent1", "agent2", "agent3"),
        "synthesis": orchestration.NewSequential("step1", "step2"),
    },
)
```

### 4. Performance by Design
**Principle**: Local is fast, distributed scales.

- LocalRuntime: Direct function calls (no serialization)
- DistributedRuntime: Optimized gRPC with connection pooling
- Parallel patterns leverage goroutines efficiently

### 5. Security & Resource Limits
**Principle**: Sane defaults, user overrides.

- Max parallel agents: 100 (configurable)
- Max orchestration depth: 10 levels
- Timeouts on all operations
- Token budget limits

---

## Architecture Overview

### System Layers

```
┌─────────────────────────────────────────────────────────────┐
│                     User Application                        │
├─────────────────────────────────────────────────────────────┤
│                   Orchestration Layer                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │Supervisor│ │Parallel  │ │  Swarm   │ │   RAG    │ ...  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
├─────────────────────────────────────────────────────────────┤
│                      Runtime Layer                          │
│  ┌────────────────────┐  ┌──────────────────────┐         │
│  │  LocalRuntime      │  │ DistributedRuntime   │         │
│  │ (Go channels)      │  │ (gRPC)               │         │
│  └────────────────────┘  └──────────────────────┘         │
├─────────────────────────────────────────────────────────────┤
│                       Agent Layer                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │  ReAct   │ │Classifier│ │Aggregator│ │ Planner  │ ...  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
├─────────────────────────────────────────────────────────────┤
│                    Observability Layer                      │
│  ┌────────────────────┐  ┌──────────────────────┐         │
│  │ OpenTelemetry      │  │ Langfuse SDK         │         │
│  │ (OTLP Traces)      │  │ (Generations/Scores) │         │
│  └────────────────────┘  └──────────────────────┘         │
├─────────────────────────────────────────────────────────────┤
│                      Provider Layer                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │  OpenAI  │ │Anthropic │ │   xAI    │ │ Gemini   │ ...  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘      │
│  (All providers automatically instrumented for cost/tokens) │
└─────────────────────────────────────────────────────────────┘
```

### Component Interactions

```
User Request
    │
    ├─→ Orchestrator (selects pattern)
    │      │
    │      ├─→ Runtime.Call() or CallParallel()
    │      │      │
    │      │      ├─→ Agent.Execute()
    │      │      │      │
    │      │      │      ├─→ InstrumentedProvider
    │      │      │      │      │
    │      │      │      │      ├─→ LLM API Call
    │      │      │      │      │
    │      │      │      │      └─→ Observability (auto token/cost)
    │      │      │      │
    │      │      │      └─→ Return result
    │      │      │
    │      │      └─→ Aggregate results
    │      │
    │      └─→ Emit pattern metrics
    │
    └─→ Return final response
```

---

## Agent Interface Redesign

### Current (v1.x) - Message-Passing Only

```go
type Agent interface {
    Start(ctx context.Context) error
}

type Runtime interface {
    Send(target string, msg *Message) error
    Recv(source string) (<-chan *Message, error)
}
```

**Problems**:
- No `Name()` method - can't identify agents
- No `Execute()` method - awkward for synchronous patterns
- Parallel execution requires complex message coordination

### New (v2.0) - Unified Interface

```go
// internal/agent/types.go

package agent

import (
    "context"
    pb "github.com/aixgo-dev/aixgo/proto"
)

// Agent represents any executable agent
type Agent interface {
    // Metadata
    Name() string
    Role() string

    // Lifecycle
    Start(ctx context.Context) error  // For long-running reactive agents
    Stop(ctx context.Context) error   // Graceful shutdown

    // Execution (request-response)
    Execute(ctx context.Context, input *Message) (*Message, error)

    // Health check
    Ready() bool
}

// Message wraps protobuf message with convenience methods
type Message struct {
    *pb.Message

    // Metadata carries observability/routing info
    Metadata map[string]any
}

// AgentDef remains largely unchanged (YAML config)
type AgentDef struct {
    Name       string         `yaml:"name"`
    Role       string         `yaml:"role"`
    Model      string         `yaml:"model,omitempty"`
    Prompt     string         `yaml:"prompt,omitempty"`
    MCPServers []string       `yaml:"mcp_servers,omitempty"`
    Config     map[string]any `yaml:"config,omitempty"`
}
```

### Agent Implementation Patterns

**Pattern 1: Request-Response Agent** (most common)

```go
type ClassifierAgent struct {
    name     string
    provider provider.Provider
}

func (a *ClassifierAgent) Name() string { return a.name }
func (a *ClassifierAgent) Role() string { return "classifier" }

func (a *ClassifierAgent) Execute(ctx context.Context, input *Message) (*Message, error) {
    // Classify input
    result, err := a.provider.CreateCompletion(ctx, ...)
    if err != nil {
        return nil, err
    }

    return &Message{
        Payload: result.Content,
        Metadata: map[string]any{
            "category": result.Category,
            "confidence": result.Confidence,
        },
    }, nil
}

func (a *ClassifierAgent) Start(ctx context.Context) error {
    // No-op for stateless agents
    return nil
}

func (a *ClassifierAgent) Stop(ctx context.Context) error {
    // No-op for stateless agents
    return nil
}

func (a *ClassifierAgent) Ready() bool { return true }
```

**Pattern 2: Long-Running Reactive Agent**

```go
type ProducerAgent struct {
    name     string
    interval time.Duration
    runtime  Runtime
    stopCh   chan struct{}
}

func (a *ProducerAgent) Start(ctx context.Context) error {
    ticker := time.NewTicker(a.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Produce message
            msg := &Message{Payload: "tick"}
            a.runtime.Send("consumer", msg)

        case <-a.stopCh:
            return nil

        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (a *ProducerAgent) Execute(ctx context.Context, input *Message) (*Message, error) {
    // Producers don't support request-response
    return nil, errors.New("producer agents don't support Execute()")
}

func (a *ProducerAgent) Stop(ctx context.Context) error {
    close(a.stopCh)
    return nil
}

func (a *ProducerAgent) Ready() bool {
    select {
    case <-a.stopCh:
        return false
    default:
        return true
    }
}
```

**Pattern 3: Hybrid Agent** (supports both modes)

```go
type ReActAgent struct {
    name     string
    provider provider.Provider
    runtime  Runtime
}

// Can execute synchronously
func (a *ReActAgent) Execute(ctx context.Context, input *Message) (*Message, error) {
    return a.processMessage(ctx, input)
}

// Can also run reactively (listens for messages)
func (a *ReActAgent) Start(ctx context.Context) error {
    ch, _ := a.runtime.Recv(a.name)

    for {
        select {
        case msg := <-ch:
            result, _ := a.processMessage(ctx, msg)
            // Send result to output targets
            for _, target := range a.outputs {
                a.runtime.Send(target, result)
            }

        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (a *ReActAgent) processMessage(ctx context.Context, input *Message) (*Message, error) {
    // Shared logic for both modes
    // ...
}
```

### Benefits of Unified Interface

1. **Clarity**: `Execute()` makes synchronous use explicit
2. **Flexibility**: Agents choose which modes to support
3. **Composability**: Orchestrators use `Execute()` for clean composition
4. **Local/Distributed**: Same interface works for both runtimes

---

## Runtime Abstraction

### Runtime Interface (v2.0)

```go
// internal/runtime/runtime.go

package runtime

import (
    "context"
    "github.com/aixgo-dev/aixgo/internal/agent"
)

// Runtime handles agent execution and communication
type Runtime interface {
    // Agent registration and lifecycle
    Register(name string, ag agent.Agent) error
    Unregister(name string) error
    GetAgent(name string) (agent.Agent, error)
    ListAgents() []string

    // Async messaging (for reactive agents)
    Send(target string, msg *agent.Message) error
    Recv(source string) (<-chan *agent.Message, error)

    // Sync execution (for request-response agents)
    Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error)

    // Parallel execution (optimized per runtime)
    CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, error)

    // Broadcast
    Broadcast(ctx context.Context, targets []string, msg *agent.Message) error

    // Connection management (distributed only)
    Connect(name string, address string) error
    Disconnect(name string) error

    // Health and status
    Health(ctx context.Context) error
    Metrics() RuntimeMetrics
}

// RuntimeMetrics tracks runtime performance
type RuntimeMetrics struct {
    ActiveAgents     int
    MessagesSent     uint64
    MessagesReceived uint64
    CallsSucceeded   uint64
    CallsFailed      uint64
    AvgCallLatency   time.Duration
}
```

### LocalRuntime Implementation

```go
// internal/runtime/local.go

package runtime

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/aixgo-dev/aixgo/internal/agent"
)

// LocalRuntime executes agents in-process using Go channels
type LocalRuntime struct {
    agents   map[string]agent.Agent
    channels map[string]chan *agent.Message
    metrics  RuntimeMetrics
    mu       sync.RWMutex
}

func NewLocalRuntime() *LocalRuntime {
    return &LocalRuntime{
        agents:   make(map[string]agent.Agent),
        channels: make(map[string]chan *agent.Message),
    }
}

func (r *LocalRuntime) Register(name string, ag agent.Agent) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.agents[name]; exists {
        return fmt.Errorf("agent %s already registered", name)
    }

    r.agents[name] = ag
    r.channels[name] = make(chan *agent.Message, 100)
    r.metrics.ActiveAgents++

    return nil
}

func (r *LocalRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
    r.mu.RLock()
    ag, exists := r.agents[target]
    r.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("agent %s not found", target)
    }

    start := time.Now()

    // Direct function call - no serialization, no network overhead
    result, err := ag.Execute(ctx, input)

    latency := time.Since(start)
    r.updateMetrics(err == nil, latency)

    return result, err
}

func (r *LocalRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, error) {
    results := make(map[string]*agent.Message)
    errors := make(map[string]error)
    mu := sync.Mutex{}
    wg := sync.WaitGroup{}

    for _, target := range targets {
        wg.Add(1)
        go func(name string) {
            defer wg.Done()

            result, err := r.Call(ctx, name, input)

            mu.Lock()
            if err != nil {
                errors[name] = err
            } else {
                results[name] = result
            }
            mu.Unlock()
        }(target)
    }

    wg.Wait()

    // Return partial results even if some failed
    return results, nil
}

func (r *LocalRuntime) Send(target string, msg *agent.Message) error {
    r.mu.RLock()
    ch, exists := r.channels[target]
    r.mu.RUnlock()

    if !exists {
        return fmt.Errorf("agent %s not found", target)
    }

    select {
    case ch <- msg:
        r.metrics.MessagesSent++
        return nil
    default:
        return fmt.Errorf("channel %s is full", target)
    }
}

func (r *LocalRuntime) Recv(source string) (<-chan *agent.Message, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    ch, exists := r.channels[source]
    if !exists {
        return nil, fmt.Errorf("agent %s not found", source)
    }

    return ch, nil
}

func (r *LocalRuntime) Broadcast(ctx context.Context, targets []string, msg *agent.Message) error {
    for _, target := range targets {
        if err := r.Send(target, msg); err != nil {
            // Continue broadcasting even if one fails
            continue
        }
    }
    return nil
}

// Connect/Disconnect are no-ops for LocalRuntime
func (r *LocalRuntime) Connect(name string, address string) error {
    return fmt.Errorf("Connect() not supported in LocalRuntime")
}

func (r *LocalRuntime) Disconnect(name string) error {
    return fmt.Errorf("Disconnect() not supported in LocalRuntime")
}

func (r *LocalRuntime) Health(ctx context.Context) error {
    r.mu.RLock()
    defer r.mu.RUnlock()

    for name, ag := range r.agents {
        if !ag.Ready() {
            return fmt.Errorf("agent %s not ready", name)
        }
    }
    return nil
}

func (r *LocalRuntime) Metrics() RuntimeMetrics {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.metrics
}

func (r *LocalRuntime) updateMetrics(success bool, latency time.Duration) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if success {
        r.metrics.CallsSucceeded++
    } else {
        r.metrics.CallsFailed++
    }

    // Update average latency (simple moving average)
    total := r.metrics.CallsSucceeded + r.metrics.CallsFailed
    r.metrics.AvgCallLatency = time.Duration(
        (int64(r.metrics.AvgCallLatency) * int64(total-1) + int64(latency)) / int64(total),
    )
}
```

### DistributedRuntime Implementation (Skeleton)

```go
// internal/runtime/distributed.go

package runtime

import (
    "context"
    "google.golang.org/grpc"
    pb "github.com/aixgo-dev/aixgo/proto"
)

// DistributedRuntime executes agents across processes via gRPC
type DistributedRuntime struct {
    registry map[string]*grpc.ClientConn
    local    *LocalRuntime  // For local agents
    mu       sync.RWMutex
}

func NewDistributedRuntime() *DistributedRuntime {
    return &DistributedRuntime{
        registry: make(map[string]*grpc.ClientConn),
        local:    NewLocalRuntime(),
    }
}

func (r *DistributedRuntime) Connect(name string, address string) error {
    conn, err := grpc.Dial(address, grpc.WithInsecure())
    if err != nil {
        return err
    }

    r.mu.Lock()
    r.registry[name] = conn
    r.mu.Unlock()

    return nil
}

func (r *DistributedRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
    // Check if agent is local
    if _, err := r.local.GetAgent(target); err == nil {
        return r.local.Call(ctx, target, input)
    }

    // Agent is remote - use gRPC
    r.mu.RLock()
    conn, exists := r.registry[target]
    r.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("agent %s not found", target)
    }

    client := pb.NewAgentServiceClient(conn)
    resp, err := client.Execute(ctx, input.Message)
    if err != nil {
        return nil, err
    }

    return &agent.Message{Message: resp}, nil
}

func (r *DistributedRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, error) {
    // Use goroutines for parallel gRPC calls
    // Same concurrency model as LocalRuntime, different transport
    // ... implementation similar to LocalRuntime.CallParallel
}
```

### Runtime Selection

```go
// User selects runtime via config or programmatically

// Option 1: Config-driven
deployment:
  mode: local  # or "distributed"

// Option 2: Programmatic
func main() {
    var rt runtime.Runtime

    if os.Getenv("DEPLOYMENT_MODE") == "distributed" {
        rt = runtime.NewDistributedRuntime()
        rt.Connect("agent1", "agent1-service:50051")
        rt.Connect("agent2", "agent2-service:50051")
    } else {
        rt = runtime.NewLocalRuntime()
        rt.Register("agent1", agent1)
        rt.Register("agent2", agent2)
    }

    // Use runtime (code is identical for both)
    orchestrator := orchestration.NewParallel(rt, []string{"agent1", "agent2"})
    result := orchestrator.Execute(ctx, input)
}
```

---

## Orchestration Patterns

### Orchestrator Interface

```go
// internal/orchestration/orchestrator.go

package orchestration

import (
    "context"
    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/runtime"
)

// Orchestrator defines the interface for all orchestration patterns
type Orchestrator interface {
    // Execute runs the orchestration pattern
    Execute(ctx context.Context, input *agent.Message) (*agent.Message, error)

    // Name returns the orchestrator name
    Name() string

    // Pattern returns the pattern type (e.g., "parallel", "sequential")
    Pattern() string

    // Agents returns the list of agent names managed by this orchestrator
    Agents() []string
}

// OrchestratorDef defines orchestrator configuration
type OrchestratorDef struct {
    Name    string            `yaml:"name"`
    Pattern string            `yaml:"pattern"`
    Agents  []string          `yaml:"agents"`
    Config  map[string]any    `yaml:"config,omitempty"`
    Limits  OrchestratorLimits `yaml:"limits,omitempty"`
}

// OrchestratorLimits defines resource limits
type OrchestratorLimits struct {
    MaxAgents     int           `yaml:"max_agents,omitempty"`      // Default: 100
    MaxDepth      int           `yaml:"max_depth,omitempty"`       // Default: 10
    MaxIterations int           `yaml:"max_iterations,omitempty"`  // Default: 10
    Timeout       time.Duration `yaml:"timeout,omitempty"`         // Default: 5m
    MaxTokens     int           `yaml:"max_tokens,omitempty"`      // Default: 100000
}
```

### Pattern 1: Parallel Orchestrator

```go
// internal/orchestration/parallel.go

package orchestration

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/observability"
    "github.com/aixgo-dev/aixgo/internal/runtime"
)

// ParallelOrchestrator executes agents concurrently and aggregates results
type ParallelOrchestrator struct {
    name       string
    rt         runtime.Runtime
    agents     []string
    aggregator string  // Optional aggregator agent
    limits     OrchestratorLimits
}

func NewParallel(name string, rt runtime.Runtime, agents []string, opts ...ParallelOption) *ParallelOrchestrator {
    o := &ParallelOrchestrator{
        name:   name,
        rt:     rt,
        agents: agents,
        limits: DefaultLimits(),
    }

    for _, opt := range opts {
        opt(o)
    }

    return o
}

type ParallelOption func(*ParallelOrchestrator)

func WithAggregator(name string) ParallelOption {
    return func(o *ParallelOrchestrator) {
        o.aggregator = name
    }
}

func WithLimits(limits OrchestratorLimits) ParallelOption {
    return func(o *ParallelOrchestrator) {
        o.limits = limits
    }
}

func (o *ParallelOrchestrator) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Validation
    if len(o.agents) > o.limits.MaxAgents {
        return nil, fmt.Errorf("too many agents: %d (max: %d)", len(o.agents), o.limits.MaxAgents)
    }

    // Create span for observability
    ctx, span := observability.StartSpanWithContext(ctx, "orchestration.parallel", map[string]any{
        "pattern":     "parallel",
        "orchestrator": o.name,
        "agent_count": len(o.agents),
    })
    defer span.End()

    // Add timeout if configured
    if o.limits.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, o.limits.Timeout)
        defer cancel()
    }

    start := time.Now()

    // Execute agents in parallel using runtime
    results, err := o.rt.CallParallel(ctx, o.agents, input)

    waitTime := time.Since(start)

    // Emit pattern-specific metrics
    span.SetAttribute("parallel.agents_succeeded", len(results))
    span.SetAttribute("parallel.agents_failed", len(o.agents)-len(results))
    span.SetAttribute("parallel.wait_time_ms", waitTime.Milliseconds())

    // Aggregate costs from all agents
    totalCost := 0.0
    totalTokens := 0
    for _, result := range results {
        if cost, ok := result.Metadata["cost_usd"].(float64); ok {
            totalCost += cost
        }
        if tokens, ok := result.Metadata["tokens_total"].(int); ok {
            totalTokens += tokens
        }
    }
    span.SetAttribute("cost.total_usd", totalCost)
    span.SetAttribute("tokens.total", totalTokens)

    // If no results, return error
    if len(results) == 0 {
        err := fmt.Errorf("all agents failed")
        span.SetError(err)
        return nil, err
    }

    // Aggregate results if aggregator specified
    if o.aggregator != "" {
        aggregateInput := &agent.Message{
            Payload: results,
            Metadata: map[string]any{
                "parallel_results": results,
            },
        }

        return o.rt.Call(ctx, o.aggregator, aggregateInput)
    }

    // Otherwise, return results map
    return &agent.Message{
        Payload: results,
        Metadata: map[string]any{
            "pattern":     "parallel",
            "agent_count": len(o.agents),
            "cost_usd":    totalCost,
            "tokens_total": totalTokens,
        },
    }, nil
}

func (o *ParallelOrchestrator) Name() string    { return o.name }
func (o *ParallelOrchestrator) Pattern() string { return "parallel" }
func (o *ParallelOrchestrator) Agents() []string { return o.agents }

func DefaultLimits() OrchestratorLimits {
    return OrchestratorLimits{
        MaxAgents:     100,
        MaxDepth:      10,
        MaxIterations: 10,
        Timeout:       5 * time.Minute,
        MaxTokens:     100000,
    }
}
```

### Pattern 2: Sequential Orchestrator

```go
// internal/orchestration/sequential.go

package orchestration

// SequentialOrchestrator executes agents one after another
type SequentialOrchestrator struct {
    name   string
    rt     runtime.Runtime
    agents []string
    limits OrchestratorLimits
}

func NewSequential(name string, rt runtime.Runtime, agents []string, opts ...SequentialOption) *SequentialOrchestrator {
    o := &SequentialOrchestrator{
        name:   name,
        rt:     rt,
        agents: agents,
        limits: DefaultLimits(),
    }

    for _, opt := range opts {
        opt(o)
    }

    return o
}

type SequentialOption func(*SequentialOrchestrator)

func (o *SequentialOrchestrator) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    ctx, span := observability.StartSpanWithContext(ctx, "orchestration.sequential", map[string]any{
        "pattern":     "sequential",
        "orchestrator": o.name,
        "step_count":  len(o.agents),
    })
    defer span.End()

    // Add timeout if configured
    if o.limits.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, o.limits.Timeout)
        defer cancel()
    }

    currentOutput := input
    totalCost := 0.0
    totalTokens := 0

    for i, agentName := range o.agents {
        // Create span for each step
        stepCtx, stepSpan := observability.StartSpanWithContext(ctx, fmt.Sprintf("step.%d.%s", i+1, agentName), map[string]any{
            "agent": agentName,
            "step":  i + 1,
        })

        // Execute agent
        result, err := o.rt.Call(stepCtx, agentName, currentOutput)

        if err != nil {
            stepSpan.SetError(err)
            stepSpan.End()
            span.SetError(err)
            return nil, fmt.Errorf("step %d (%s) failed: %w", i+1, agentName, err)
        }

        // Track costs
        if cost, ok := result.Metadata["cost_usd"].(float64); ok {
            totalCost += cost
            stepSpan.SetAttribute("cost.usd", cost)
        }
        if tokens, ok := result.Metadata["tokens_total"].(int); ok {
            totalTokens += tokens
            stepSpan.SetAttribute("tokens.total", tokens)
        }

        stepSpan.End()

        // Pass result to next step
        currentOutput = result
    }

    // Emit total metrics
    span.SetAttribute("cost.total_usd", totalCost)
    span.SetAttribute("tokens.total", totalTokens)

    // Add metadata to final result
    currentOutput.Metadata["pattern"] = "sequential"
    currentOutput.Metadata["step_count"] = len(o.agents)
    currentOutput.Metadata["cost_usd"] = totalCost
    currentOutput.Metadata["tokens_total"] = totalTokens

    return currentOutput, nil
}

func (o *SequentialOrchestrator) Name() string    { return o.name }
func (o *SequentialOrchestrator) Pattern() string { return "sequential" }
func (o *SequentialOrchestrator) Agents() []string { return o.agents }
```

### Pattern 3: Router Orchestrator

```go
// internal/orchestration/router.go

package orchestration

// RouterOrchestrator routes requests to specialized agents based on classification
type RouterOrchestrator struct {
    name       string
    rt         runtime.Runtime
    classifier string  // Classifier agent name
    routes     map[string]string  // category -> agent mapping
    fallback   string  // Fallback agent if category not found
    limits     OrchestratorLimits
}

func NewRouter(name string, rt runtime.Runtime, classifier string, routes map[string]string, opts ...RouterOption) *RouterOrchestrator {
    o := &RouterOrchestrator{
        name:       name,
        rt:         rt,
        classifier: classifier,
        routes:     routes,
        limits:     DefaultLimits(),
    }

    for _, opt := range opts {
        opt(o)
    }

    return o
}

type RouterOption func(*RouterOrchestrator)

func WithFallback(agent string) RouterOption {
    return func(o *RouterOrchestrator) {
        o.fallback = agent
    }
}

func (o *RouterOrchestrator) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    ctx, span := observability.StartSpanWithContext(ctx, "orchestration.router", map[string]any{
        "pattern":     "router",
        "orchestrator": o.name,
        "route_count": len(o.routes),
    })
    defer span.End()

    // Step 1: Classify input
    classifyStart := time.Now()
    classifyResult, err := o.rt.Call(ctx, o.classifier, input)
    if err != nil {
        span.SetError(err)
        return nil, fmt.Errorf("classification failed: %w", err)
    }

    classifyLatency := time.Since(classifyStart)
    span.SetAttribute("router.classify_latency_ms", classifyLatency.Milliseconds())

    // Extract category from classification result
    category, ok := classifyResult.Metadata["category"].(string)
    if !ok {
        return nil, fmt.Errorf("classifier did not return category")
    }

    confidence, _ := classifyResult.Metadata["confidence"].(float64)
    span.SetAttribute("router.category", category)
    span.SetAttribute("router.confidence", confidence)

    // Step 2: Route to appropriate agent
    targetAgent, ok := o.routes[category]
    if !ok {
        if o.fallback != "" {
            targetAgent = o.fallback
            span.SetAttribute("router.used_fallback", true)
        } else {
            err := fmt.Errorf("no route found for category: %s", category)
            span.SetError(err)
            return nil, err
        }
    } else {
        span.SetAttribute("router.used_fallback", false)
    }

    span.SetAttribute("router.target_agent", targetAgent)

    // Step 3: Execute target agent
    routeStart := time.Now()
    result, err := o.rt.Call(ctx, targetAgent, input)
    if err != nil {
        span.SetError(err)
        return nil, fmt.Errorf("routing to %s failed: %w", targetAgent, err)
    }

    routeLatency := time.Since(routeStart)
    span.SetAttribute("router.route_latency_ms", routeLatency.Milliseconds())

    // Add routing metadata to result
    result.Metadata["pattern"] = "router"
    result.Metadata["routed_to"] = targetAgent
    result.Metadata["category"] = category
    result.Metadata["confidence"] = confidence

    return result, nil
}

func (o *RouterOrchestrator) Name() string    { return o.name }
func (o *RouterOrchestrator) Pattern() string { return "router" }
func (o *RouterOrchestrator) Agents() []string {
    agents := []string{o.classifier}
    for _, agent := range o.routes {
        agents = append(agents, agent)
    }
    if o.fallback != "" {
        agents = append(agents, o.fallback)
    }
    return agents
}
```

### Remaining Patterns (Summary)

**Pattern 4: Swarm** - `internal/orchestration/swarm.go`
- Decentralized handoffs between agents
- Each agent can transfer to any other agent
- Conversational flow or rules guide transitions

**Pattern 5: Hierarchical** - `internal/orchestration/hierarchical.go`
- Multi-level delegation (manager → sub-managers → workers)
- Dynamic manager assignment
- Hierarchical state management

**Pattern 6: Reflection** - `internal/orchestration/reflection.go`
- Generator-critic loops
- Iterative refinement until quality threshold
- Max iterations with early stopping

**Pattern 7: Ensemble** - `internal/orchestration/ensemble.go`
- Multiple models vote on output
- Voting strategies: majority, weighted, unanimous
- Agreement metrics and fallback handling

**Pattern 8: RAG** - `internal/orchestration/rag.go`
- Retrieval → Generation pipeline
- Vector store integration
- Context aggregation and relevance filtering

**Pattern 9: Debate** - `internal/orchestration/debate.go`
- Multi-agent adversarial collaboration
- Multiple rounds with termination criteria
- Perspective diversity and consensus tracking

---

## Observability & Cost Tracking

### Automatic Instrumentation

All LLM providers are automatically wrapped with instrumentation:

```go
// internal/llm/provider/instrumented.go

package provider

import (
    "context"
    "github.com/aixgo-dev/aixgo/internal/llm/cost"
    "github.com/aixgo-dev/aixgo/internal/observability"
)

// InstrumentedProvider wraps any provider with automatic observability
type InstrumentedProvider struct {
    base     Provider
    costCalc *cost.Calculator
}

// Instrument wraps a provider with automatic token and cost tracking
func Instrument(base Provider) Provider {
    return &InstrumentedProvider{
        base:     base,
        costCalc: cost.NewCalculator(),
    }
}

func (p *InstrumentedProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    // Start LLM generation span (dual OTEL + Langfuse)
    gen, ctx := observability.StartLLMGeneration(ctx, "llm.completion", observability.GenerationParams{
        Name:        "completion",
        Model:       req.Model,
        Input:       req.Prompt,
        Temperature: req.Temperature,
        MaxTokens:   req.MaxTokens,
    })

    // Call base provider
    resp, err := p.base.CreateCompletion(ctx, req)

    if err != nil {
        gen.End(nil, err)
        return nil, err
    }

    // Calculate cost
    costUSD := p.costCalc.CalculateCost(req.Model, resp.Usage)

    // End generation with metadata
    gen.End(&observability.GenerationResult{
        Output:           resp.Content,
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        TotalTokens:      resp.Usage.TotalTokens,
        CostUSD:          costUSD,
    }, nil)

    // Add metadata to response for upstream aggregation
    if resp.Metadata == nil {
        resp.Metadata = make(map[string]any)
    }
    resp.Metadata["cost_usd"] = costUSD
    resp.Metadata["tokens_total"] = resp.Usage.TotalTokens
    resp.Metadata["tokens_prompt"] = resp.Usage.PromptTokens
    resp.Metadata["tokens_completion"] = resp.Usage.CompletionTokens

    return resp, nil
}

// Similar for CreateChatCompletion and CreateStructuredCompletion
```

### Cost Calculator

```go
// internal/llm/cost/calculator.go

package cost

import (
    "github.com/aixgo-dev/aixgo/internal/llm/provider"
)

// ModelPricing defines per-model token pricing
type ModelPricing struct {
    PromptPricePer1K     float64
    CompletionPricePer1K float64
}

// DefaultPricing contains current pricing for major models
var DefaultPricing = map[string]ModelPricing{
    // OpenAI
    "gpt-4-turbo":           {0.01, 0.03},
    "gpt-4-turbo-preview":   {0.01, 0.03},
    "gpt-4":                 {0.03, 0.06},
    "gpt-3.5-turbo":         {0.0005, 0.0015},
    "gpt-3.5-turbo-16k":     {0.003, 0.004},

    // Anthropic
    "claude-3-5-sonnet-20241022": {0.003, 0.015},
    "claude-3-opus-20240229":     {0.015, 0.075},
    "claude-3-sonnet-20240229":   {0.003, 0.015},
    "claude-3-haiku-20240307":    {0.00025, 0.00125},

    // xAI
    "grok-beta": {0.005, 0.015},

    // Google
    "gemini-1.5-pro":   {0.00125, 0.005},
    "gemini-1.5-flash": {0.000075, 0.0003},

    // Deepseek
    "deepseek-chat": {0.00014, 0.00028},
    "deepseek-coder": {0.00014, 0.00028},
}

// Calculator calculates LLM costs
type Calculator struct {
    pricing map[string]ModelPricing
}

func NewCalculator() *Calculator {
    return &Calculator{
        pricing: DefaultPricing,
    }
}

// AddModel adds custom model pricing
func (c *Calculator) AddModel(model string, pricing ModelPricing) {
    c.pricing[model] = pricing
}

// CalculateCost computes cost in USD for a given model and usage
func (c *Calculator) CalculateCost(model string, usage provider.Usage) float64 {
    pricing, ok := c.pricing[model]
    if !ok {
        // Unknown model - return 0 (log warning in production)
        return 0.0
    }

    promptCost := (float64(usage.PromptTokens) / 1000.0) * pricing.PromptPricePer1K
    completionCost := (float64(usage.CompletionTokens) / 1000.0) * pricing.CompletionPricePer1K

    return promptCost + completionCost
}
```

### Langfuse Integration

```go
// internal/observability/langfuse.go

package observability

import (
    "context"
    "github.com/langfuse/langfuse-go"
)

var langfuseClient *langfuse.Langfuse

// InitLangfuse initializes Langfuse SDK (in addition to OTEL)
func InitLangfuse(publicKey, secretKey string) error {
    if publicKey == "" || secretKey == "" {
        return nil  // Langfuse disabled
    }

    client, err := langfuse.New(langfuse.Config{
        PublicKey: publicKey,
        SecretKey: secretKey,
    })
    if err != nil {
        return err
    }

    langfuseClient = client
    return nil
}

// GenerationParams defines LLM generation parameters
type GenerationParams struct {
    Name        string
    Model       string
    Input       string
    Temperature float64
    MaxTokens   int
}

// GenerationResult defines LLM generation result
type GenerationResult struct {
    Output           string
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    CostUSD          float64
}

// LLMGeneration wraps both OTEL span and Langfuse generation
type LLMGeneration struct {
    otelSpan *Span
    lfGen    *langfuse.Generation
}

// StartLLMGeneration creates a dual-tracked LLM generation
func StartLLMGeneration(ctx context.Context, name string, params GenerationParams) (*LLMGeneration, context.Context) {
    // OTEL span (works with any backend)
    ctx, otelSpan := StartSpanWithContext(ctx, name, map[string]any{
        "model":       params.Model,
        "temperature": params.Temperature,
        "max_tokens":  params.MaxTokens,
    })

    // Langfuse generation (if enabled)
    var lfGen *langfuse.Generation
    if langfuseClient != nil {
        lfGen = langfuseClient.Generation(langfuse.GenerationParams{
            Name:        params.Name,
            Model:       params.Model,
            Input:       params.Input,
            ModelParameters: map[string]any{
                "temperature": params.Temperature,
                "max_tokens":  params.MaxTokens,
            },
        })
    }

    return &LLMGeneration{
        otelSpan: otelSpan,
        lfGen:    lfGen,
    }, ctx
}

// End finishes both OTEL span and Langfuse generation
func (g *LLMGeneration) End(result *GenerationResult, err error) {
    // End OTEL span
    if err != nil {
        g.otelSpan.SetError(err)
    } else if result != nil {
        g.otelSpan.SetAttribute("tokens.prompt", result.PromptTokens)
        g.otelSpan.SetAttribute("tokens.completion", result.CompletionTokens)
        g.otelSpan.SetAttribute("tokens.total", result.TotalTokens)
        g.otelSpan.SetAttribute("cost.usd", result.CostUSD)
    }
    g.otelSpan.End()

    // End Langfuse generation
    if g.lfGen != nil {
        if err != nil {
            g.lfGen.End(langfuse.GenerationEndParams{
                StatusMessage: err.Error(),
            })
        } else if result != nil {
            g.lfGen.End(langfuse.GenerationEndParams{
                Output:           result.Output,
                Usage: &langfuse.Usage{
                    PromptTokens:     result.PromptTokens,
                    CompletionTokens: result.CompletionTokens,
                    TotalTokens:      result.TotalTokens,
                },
                CalculatedTotalCost: result.CostUSD,
            })
        }
    }
}

// RecordQualityScore records a quality metric (e.g., accuracy, relevance)
func RecordQualityScore(span *Span, dimension string, score float64, comment string) {
    span.SetAttribute(fmt.Sprintf("quality.%s", dimension), score)

    // Send to Langfuse if enabled
    if langfuseClient != nil && span.TraceID() != "" {
        langfuseClient.Score(langfuse.ScoreParams{
            TraceID: span.TraceID(),
            Name:    dimension,
            Value:   score,
            Comment: comment,
        })
    }
}
```

---

## Package Structure

```
aixgo/
├── internal/
│   ├── agent/
│   │   ├── types.go              # Agent interface (NEW)
│   │   ├── factory.go            # Agent factory (updated)
│   │   └── testutil.go           # Test utilities
│   │
│   ├── runtime/                  # NEW PACKAGE
│   │   ├── runtime.go            # Runtime interface
│   │   ├── local.go              # LocalRuntime implementation
│   │   ├── distributed.go        # DistributedRuntime implementation
│   │   ├── local_test.go         # LocalRuntime tests
│   │   └── distributed_test.go   # DistributedRuntime tests
│   │
│   ├── orchestration/            # NEW PACKAGE
│   │   ├── orchestrator.go       # Orchestrator interface
│   │   ├── parallel.go           # Parallel pattern
│   │   ├── parallel_test.go      # Parallel tests
│   │   ├── sequential.go         # Sequential pattern
│   │   ├── sequential_test.go    # Sequential tests
│   │   ├── router.go             # Router pattern
│   │   ├── router_test.go        # Router tests
│   │   ├── swarm.go              # Swarm pattern
│   │   ├── swarm_test.go         # Swarm tests
│   │   ├── hierarchical.go       # Hierarchical pattern
│   │   ├── hierarchical_test.go  # Hierarchical tests
│   │   ├── reflection.go         # Reflection pattern
│   │   ├── reflection_test.go    # Reflection tests
│   │   ├── ensemble.go           # Ensemble pattern
│   │   ├── ensemble_test.go      # Ensemble tests
│   │   ├── rag.go                # RAG pattern
│   │   ├── rag_test.go           # RAG tests
│   │   ├── debate.go             # Debate pattern
│   │   └── debate_test.go        # Debate tests
│   │
│   ├── llm/
│   │   ├── cost/                 # NEW PACKAGE
│   │   │   ├── calculator.go     # Cost calculator
│   │   │   └── calculator_test.go
│   │   ├── provider/
│   │   │   ├── provider.go       # Provider interface (updated)
│   │   │   ├── instrumented.go   # NEW: Auto-instrumentation wrapper
│   │   │   ├── openai.go
│   │   │   ├── anthropic.go
│   │   │   └── ...
│   │   └── inference/
│   │       └── ...
│   │
│   ├── observability/
│   │   ├── observability.go      # OTEL integration (existing)
│   │   ├── langfuse.go           # NEW: Langfuse SDK integration
│   │   └── observability_test.go
│   │
│   ├── supervisor/               # LEGACY - kept for compatibility
│   │   └── supervisor.go
│   │
│   └── workflow/                 # LEGACY - kept for compatibility
│       └── executor.go
│
├── agents/
│   ├── react.go                  # Updated to new Agent interface
│   ├── classifier.go             # Updated to new Agent interface
│   ├── aggregator.go             # Updated to new Agent interface
│   ├── planner.go                # Updated to new Agent interface
│   ├── producer.go               # Updated to new Agent interface
│   └── logger.go                 # Updated to new Agent interface
│
├── examples/
│   ├── parallel-analysis/        # NEW: Parallel pattern example
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── README.md
│   ├── router-costopt/           # NEW: Router pattern for cost optimization
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── README.md
│   ├── swarm-customer-service/   # NEW: Swarm pattern for handoffs
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── README.md
│   ├── ensemble-medical/         # NEW: Ensemble for high-stakes decisions
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── README.md
│   ├── rag-enterprise/           # NEW: RAG pattern
│   │   ├── main.go
│   │   ├── config.yaml
│   │   └── README.md
│   └── ...
│
├── docs/
│   ├── ARCHITECTURE_V2.md        # This document
│   ├── MIGRATION_V1_TO_V2.md     # NEW: Migration guide
│   ├── PATTERNS.md               # NEW: Pattern catalog
│   ├── OBSERVABILITY.md          # Updated with cost tracking
│   └── ...
│
└── tests/
    └── e2e/
        ├── patterns/             # NEW: E2E tests for each pattern
        │   ├── parallel_test.go
        │   ├── router_test.go
        │   └── ...
        └── ...
```

---

## Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)

**Goal**: Core interfaces and local runtime

**Tasks**:
1. ✅ Update `Agent` interface with `Execute()`, `Name()`, `Role()`, `Stop()`, `Ready()`
2. ✅ Create `Runtime` interface
3. ✅ Implement `LocalRuntime`
4. ✅ Create `Orchestrator` interface
5. ✅ Update existing agents to new interface (ReAct, Classifier, Planner, Aggregator)
6. ✅ Add comprehensive tests

**Deliverables**:
- `internal/agent/types.go` (updated)
- `internal/runtime/runtime.go`
- `internal/runtime/local.go`
- `internal/orchestration/orchestrator.go`
- Updated agents in `agents/`
- Tests for all new components

**Validation**: All existing examples work with new agent interface.

---

### Phase 2: Observability & Cost Tracking (Weeks 2-3)

**Goal**: Automatic token and cost tracking

**Tasks**:
1. ✅ Create `internal/llm/cost/calculator.go`
2. ✅ Create `internal/llm/provider/instrumented.go`
3. ✅ Create `internal/observability/langfuse.go`
4. ✅ Wrap all existing providers with `Instrument()`
5. ✅ Update observability documentation

**Deliverables**:
- `internal/llm/cost/calculator.go`
- `internal/llm/provider/instrumented.go`
- `internal/observability/langfuse.go`
- Updated `docs/OBSERVABILITY.md`

**Validation**:
- Every LLM call emits token and cost metrics
- Langfuse dashboard shows Generations with costs
- Cost aggregation works across patterns

---

### Phase 3: Core Patterns (Weeks 3-5)

**Goal**: Implement 6 core patterns

**Tasks**:
1. ✅ Parallel orchestrator + tests
2. ✅ Sequential orchestrator + tests (refactor existing workflow)
3. ✅ Router orchestrator + tests
4. ✅ Swarm orchestrator + tests
5. ✅ RAG orchestrator + tests
6. ✅ Reflection orchestrator + tests

**Deliverables**:
- `internal/orchestration/parallel.go` + tests
- `internal/orchestration/sequential.go` + tests
- `internal/orchestration/router.go` + tests
- `internal/orchestration/swarm.go` + tests
- `internal/orchestration/rag.go` + tests
- `internal/orchestration/reflection.go` + tests

**Validation**: Each pattern has working example and E2E tests.

---

### Phase 4: Advanced Patterns (Weeks 5-7)

**Goal**: Implement 3 advanced patterns

**Tasks**:
1. ✅ Hierarchical orchestrator + tests
2. ✅ Ensemble orchestrator + tests
3. ✅ Debate orchestrator + tests

**Deliverables**:
- `internal/orchestration/hierarchical.go` + tests
- `internal/orchestration/ensemble.go` + tests
- `internal/orchestration/debate.go` + tests

**Validation**: Each pattern has working example and E2E tests.

---

### Phase 5: Distributed Runtime (Weeks 7-9)

**Goal**: gRPC-based distributed deployment

**Tasks**:
1. ✅ Implement `DistributedRuntime`
2. ✅ Create gRPC protobuf definitions
3. ✅ Add agent service server
4. ✅ Add connection management
5. ✅ Test local → distributed migration

**Deliverables**:
- `internal/runtime/distributed.go`
- `proto/agent_service.proto`
- `internal/runtime/grpc_server.go`
- Example: `examples/distributed-deployment/`

**Validation**:
- Same code runs in single binary and distributed
- No code changes required for deployment switch

---

### Phase 6: Documentation & Examples (Weeks 9-10)

**Goal**: Complete documentation and examples

**Tasks**:
1. ✅ Write `PATTERNS.md` (pattern catalog)
2. ✅ Write `MIGRATION_V1_TO_V2.md`
3. ✅ Create example for each pattern (9 examples)
4. ✅ Update all existing examples
5. ✅ Update website documentation (`../web`)
6. ✅ Create video tutorials (optional)

**Deliverables**:
- `docs/PATTERNS.md`
- `docs/MIGRATION_V1_TO_V2.md`
- 9 pattern examples in `examples/`
- Updated website in `../web/content/`

---

### Phase 7: Testing & Validation (Weeks 10-11)

**Goal**: Comprehensive testing

**Tasks**:
1. ✅ Unit tests for all components (>80% coverage)
2. ✅ Integration tests for all patterns
3. ✅ E2E tests with real LLMs
4. ✅ Race condition tests (`go test -race`)
5. ✅ Performance benchmarks
6. ✅ Load testing

**Deliverables**:
- Comprehensive test suite
- Benchmark results
- Performance report

**Validation**: CI passes, no race conditions, benchmarks meet targets.

---

### Phase 8: Release (Week 12)

**Goal**: v2.0.0-alpha release

**Tasks**:
1. ✅ Final documentation review
2. ✅ Example verification
3. ✅ Security audit
4. ✅ Performance tuning
5. ✅ Release notes
6. ✅ Tag v2.0.0-alpha

**Deliverables**:
- v2.0.0-alpha release
- Release notes
- Migration guide
- Announcement blog post

---

## Migration Guide

### Breaking Changes Summary

**1. Agent Interface**
```go
// OLD (v1.x)
type Agent interface {
    Start(ctx context.Context) error
}

// NEW (v2.0)
type Agent interface {
    Name() string
    Role() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Execute(ctx context.Context, input *Message) (*Message, error)
    Ready() bool
}
```

**Migration**: All agent implementations must add new methods.

**2. Runtime Interface**
```go
// OLD (v1.x)
type Runtime interface {
    Send(target string, msg *Message) error
    Recv(source string) (<-chan *Message, error)
}

// NEW (v2.0)
type Runtime interface {
    // All old methods plus:
    Register(name string, ag Agent) error
    Call(ctx context.Context, target string, input *Message) (*Message, error)
    CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, error)
    Broadcast(ctx context.Context, targets []string, msg *Message) error
    // ... more methods
}
```

**Migration**: Use `NewLocalRuntime()` or `NewDistributedRuntime()` instead of `NewSimpleRuntime()`.

**3. Configuration**
```yaml
# OLD (v1.x)
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  - name: agent1
    role: react

# NEW (v2.0)
orchestration:
  pattern: supervisor  # or parallel, sequential, router, etc.
  config:
    max_rounds: 10
  agents:
    - agent1
    - agent2

agents:
  - name: agent1
    role: react
```

**Migration**: Move supervisor config to `orchestration` section. Add `pattern` field.

### Step-by-Step Migration

**Step 1**: Update agent implementations

```go
// Before
type MyAgent struct {
    name string
}

func (a *MyAgent) Start(ctx context.Context) error {
    // Implementation
}

// After
type MyAgent struct {
    name string
}

func (a *MyAgent) Name() string { return a.name }
func (a *MyAgent) Role() string { return "custom" }

func (a *MyAgent) Start(ctx context.Context) error {
    // Implementation
}

func (a *MyAgent) Stop(ctx context.Context) error {
    return nil  // Add cleanup if needed
}

func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Add request-response logic
    return &agent.Message{Payload: "result"}, nil
}

func (a *MyAgent) Ready() bool {
    return true  // Or check actual readiness
}
```

**Step 2**: Update runtime usage

```go
// Before
rt := aixgo.NewSimpleRuntime()

// After
rt := runtime.NewLocalRuntime()
rt.Register("agent1", agent1)
rt.Register("agent2", agent2)
```

**Step 3**: Update orchestration

```go
// Before
supervisor, _ := supervisor.New(supervisorDef, agents, rt)
result, _ := supervisor.Run(ctx, input)

// After
orchestrator := orchestration.NewParallel("parallel-workflow", rt, []string{"agent1", "agent2"})
result, _ := orchestrator.Execute(ctx, inputMsg)
```

**Step 4**: Update configuration

See configuration section above.

**Step 5**: Test and validate

```bash
go test ./...
go test -race ./...
```

---

## Next Steps

1. **Review this architecture document**
2. **Approve the approach**
3. **Begin Phase 1 implementation**
4. **Iterate based on feedback**

---

**Questions?** See `docs/FAQ.md` or open an issue on GitHub.

**Feedback?** We're pre-v1.0.0 - your input shapes the framework!
