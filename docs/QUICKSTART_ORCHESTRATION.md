# Orchestration Patterns - Quick Start Guide

Aixgo provides 7 production-proven orchestration patterns for building AI agent systems. This guide shows you how to get started with each pattern.

## Table of Contents

- [Installation](#installation)
- [Core Concepts](#core-concepts)
- [Pattern Quick Starts](#pattern-quick-starts)
  - [Parallel](#1-parallel---concurrent-execution)
  - [Router](#2-router---cost-optimization)
  - [Swarm](#3-swarm---decentralized-handoffs)
  - [RAG](#4-rag---retrieval-augmented-generation)
  - [Reflection](#5-reflection---iterative-refinement)
  - [Hierarchical](#6-hierarchical---multi-level-delegation)
  - [Ensemble](#7-ensemble---multi-model-voting)
- [Runtime Options](#runtime-options)
- [Observability](#observability)

## Installation

```bash
go get github.com/aixgo-dev/aixgo@latest
```

## Core Concepts

### Unified Agent Interface

All agents implement the same interface, supporting both sync and async execution:

```go
type Agent interface {
    Name() string
    Role() string
    Start(ctx context.Context) error    // Async execution
    Execute(ctx context.Context, input *Message) (*Message, error)  // Sync execution
    Stop(ctx context.Context) error
    Ready() bool
}
```

### Runtime Abstraction

Same code runs on both local and distributed runtimes:

```go
// Local deployment (single binary)
rt := runtime.NewLocalRuntime()

// Distributed deployment (gRPC)
rt := runtime.NewDistributedRuntime(":50051")
```

### Automatic Observability

All LLM calls are automatically tracked:
- Token usage
- Cost calculation
- Performance metrics
- Langfuse integration

---

## Pattern Quick Starts

### 1. Parallel - Concurrent Execution

**Use Case**: Multi-source research, batch processing, A/B testing

**Benefits**: 3-4× speedup for independent tasks

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/internal/orchestration"
    "github.com/aixgo-dev/aixgo/internal/runtime"
)

func main() {
    ctx := context.Background()

    // Create runtime
    rt := runtime.NewLocalRuntime()
    rt.Start(ctx)
    defer rt.Stop(ctx)

    // Register agents
    rt.Register(competitorAgent)
    rt.Register(marketSizeAgent)
    rt.Register(trendsAgent)

    // Create parallel orchestrator
    parallel := orchestration.NewParallel(
        "market-research",
        rt,
        []string{"competitor-agent", "market-size-agent", "trends-agent"},
    )

    // Execute in parallel
    result, err := parallel.Execute(ctx, input)
}
```

**Full Example**: `examples/parallel-research/main.go`

---

### 2. Router - Cost Optimization

**Use Case**: Cost optimization, intent-based routing, model selection

**Benefits**: 25-50% cost reduction in production

```go
// Create router
router := orchestration.NewRouter(
    "cost-optimizer",
    rt,
    "complexity-classifier",  // Classifies query complexity
    map[string]string{
        "simple":  "gpt-3.5-agent",  // Cheap model
        "complex": "gpt-4-agent",     // Expensive model
    },
    orchestration.WithDefaultRoute("gpt-3.5-agent"),
)

// Route queries automatically
result, err := router.Execute(ctx, query)
```

**Full Example**: `examples/router-cost-optimization/main.go`

---

### 3. Swarm - Decentralized Handoffs

**Use Case**: Customer service handoffs, adaptive routing, collaborative problem-solving

**Benefits**: Dynamic agent-to-agent handoffs based on conversational context

```go
// Create swarm
swarm := orchestration.NewSwarm(
    "customer-service",
    rt,
    "general-agent",  // Entry point
    []string{"general-agent", "billing-agent", "technical-agent"},
    orchestration.WithMaxHandoffs(5),
)

// Execute with dynamic handoffs
result, err := swarm.Execute(ctx, customerQuery)
```

---

### 4. RAG - Retrieval-Augmented Generation

**Use Case**: Enterprise chatbots, documentation Q&A, knowledge retrieval

**Benefits**: Grounded answers, 70% token reduction vs full context

```go
// Create RAG orchestrator
rag := orchestration.NewRAG(
    "docs-qa",
    rt,
    "doc-retriever",      // Retrieves relevant documents
    "answer-generator",   // Generates grounded answer
    orchestration.WithTopK(5),  // Top 5 documents
)

// Execute RAG pipeline
result, err := rag.Execute(ctx, question)
```

**Full Example**: `examples/rag-documentation/main.go`

---

### 5. Reflection - Iterative Refinement

**Use Case**: Code generation, content creation, complex reasoning

**Benefits**: 20-50% quality improvement through self-critique

```go
// Create reflection orchestrator
reflection := orchestration.NewReflection(
    "code-generator",
    rt,
    "code-generator",  // Generates code
    "code-critic",     // Critiques code
    orchestration.WithMaxIterations(3),
)

// Execute with iterative refinement
result, err := reflection.Execute(ctx, codeRequest)
```

**Full Example**: `examples/reflection-code-generation/main.go`

---

### 6. Hierarchical - Multi-Level Delegation

**Use Case**: Enterprise workflows, project management, complex decomposition

**Benefits**: Scalable task delegation across organizational hierarchies

```go
// Create hierarchical orchestrator
hierarchical := orchestration.NewHierarchical(
    "project-manager",
    rt,
    "pm-agent",  // Top-level manager
    map[string][]string{
        "frontend": {"ui-engineer", "ux-engineer"},
        "backend":  {"api-engineer", "db-engineer"},
    },
)

// Execute with hierarchical delegation
result, err := hierarchical.Execute(ctx, projectPlan)
```

---

### 7. Ensemble - Multi-Model Voting

**Use Case**: Medical diagnosis, financial forecasting, content moderation

**Benefits**: 25-50% error reduction for high-stakes decisions

```go
// Create ensemble orchestrator
ensemble := orchestration.NewEnsemble(
    "medical-diagnosis",
    rt,
    []string{"gpt-4-diagnostic", "claude-diagnostic", "gemini-diagnostic"},
    orchestration.WithVotingStrategy(orchestration.VotingMajority),
    orchestration.WithAgreementThreshold(0.75),  // 75% agreement required
)

// Execute with multi-model voting
result, err := ensemble.Execute(ctx, medicalCase)
```

---

## Runtime Options

### Local Runtime (Single Binary)

```go
rt := runtime.NewLocalRuntime(
    runtime.WithChannelBufferSize(100),
    runtime.WithMaxConcurrentCalls(10),
)
```

### Distributed Runtime (gRPC)

```go
// Server
rt := runtime.NewDistributedRuntime(":50051")
rt.Register(agent1)
rt.Start(ctx)

// Client
rt := runtime.NewDistributedRuntime("")
rt.Connect("agent1", "localhost:50051")
result, _ := rt.Call(ctx, "agent1", input)
```

---

## Observability

### Automatic Cost Tracking

All LLM calls are automatically tracked:

```go
import "github.com/aixgo-dev/aixgo/internal/llm/provider"

// Wrap provider for automatic tracking
prov := provider.WrapProvider(openaiProvider)

// All calls now tracked automatically
resp, _ := prov.CreateCompletion(ctx, req)
// ✓ Tokens tracked
// ✓ Cost calculated
// ✓ Metrics emitted to Langfuse
```

### Langfuse Integration

```bash
# Set environment variables
export LANGFUSE_PUBLIC_KEY="pk-..."
export LANGFUSE_SECRET_KEY="sk-..."

# Traces automatically sent to Langfuse
# View at https://cloud.langfuse.com
```

### Custom Metrics

```go
import "github.com/aixgo-dev/aixgo/internal/observability"

// Track custom metrics
ctx, span := observability.StartSpan(ctx, "custom-operation",
    trace.WithAttributes(
        attribute.String("operation", "data-processing"),
        attribute.Int("records_processed", 1000),
    ),
)
defer span.End()
```

---

## Pattern Selection Guide

Choose the right pattern for your use case:

| Goal | Pattern | Expected Benefit |
|------|---------|------------------|
| **Reduce Costs** | Router or RAG | 25-50% cost savings |
| **Improve Speed** | Parallel | 3-4× speedup |
| **Improve Accuracy** | Ensemble or Reflection | 20-50% error reduction |
| **Adaptive Routing** | Swarm or Router | Dynamic agent selection |
| **Complex Workflows** | Hierarchical | Multi-level delegation |
| **Knowledge-Intensive** | RAG | Grounded answers |

---

## Next Steps

1. **Try the examples**: Run examples in `examples/` directory
2. **Read the docs**: See `docs/ARCHITECTURE_V2.md` and `docs/PATTERNS.md`
3. **Build your first agent**: Follow `docs/MIGRATION_V1_TO_V2.md`
4. **Deploy**: Choose Local or Distributed runtime

## Support

- **Documentation**: [docs/](../docs/)
- **GitHub Issues**: [github.com/aixgo-dev/aixgo/issues](https://github.com/aixgo-dev/aixgo/issues)
- **Examples**: [examples/](../examples/)
