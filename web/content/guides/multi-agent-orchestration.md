---
title: 'Multi-Agent Orchestration'
description: 'Learn how to coordinate multiple agents with supervisor patterns and message routing.'
breadcrumb: 'Core Concepts'
category: 'Core Concepts'
weight: 4
---

Multi-agent systems unlock powerful capabilities: divide complex tasks across specialized agents, enable parallel processing, and create sophisticated workflows. Aixgo provides 13 production-proven orchestration patterns that make building these systems straightforward.

## Overview

Aixgo supports **13 production-proven orchestration patterns** for building AI agent systems. Each pattern solves specific coordination challenges and is backed by real-world production usage.

### Pattern Status

| Pattern | Status | Production Usage | Key Benefit |
|---------|--------|-----------------|-------------|
| Supervisor | ✅ Stable | High | Central coordination & lifecycle management |
| Sequential | ✅ Stable | High | Ordered task execution |
| Parallel | ✅ Stable | High | Concurrent processing |
| Router | ✅ Stable | High | Intelligent task routing |
| Swarm | ✅ Stable | Medium | Autonomous collaboration |
| Hierarchical | ✅ Stable | Medium | Multi-level delegation |
| RAG | ✅ Stable | High | Knowledge-augmented responses |
| Reflection | ✅ Stable | Medium | Self-improvement & critique |
| Ensemble | ✅ Stable | Medium | Multi-model consensus |
| Classifier | ✅ Stable | High | Intelligent categorization |
| Aggregation | ✅ Stable | High | Multi-source synthesis |
| Planning | ✅ Stable | Medium | Strategic task decomposition |
| MapReduce | ✅ Stable | Medium | Distributed data processing |

All 13 patterns are production-ready and actively used in real-world applications.

## Pattern Catalog

For comprehensive pattern details, see [PATTERNS.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md).

### 1. Supervisor Pattern

The supervisor coordinates agent lifecycle, message routing, and execution constraints. It's the foundational layer for all multi-agent systems.

**When to use**: Every multi-agent system requiring centralized control, lifecycle management, and message routing.

**Quick example**:

```yaml
supervisor:
  name: coordinator
  max_rounds: 10

agents:
  - name: worker-1
    outputs: [{ target: worker-2 }]
  - name: worker-2
    inputs: [{ source: worker-1 }]
```

**See**: [Supervisor Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#1-supervisor-pattern)

### 2. Sequential Pattern

Execute agents in order where each step depends on the previous output. Ideal for ETL pipelines and multi-stage workflows.

**When to use**: Ordered execution, stage dependencies, deterministic processing.

**Quick example**:

```yaml
agents:
  - name: ingest
    outputs: [{ target: process }]
  - name: process
    inputs: [{ source: ingest }]
    outputs: [{ target: store }]
  - name: store
    inputs: [{ source: process }]
```

**See**: [Sequential Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#2-sequential-pattern)

### 3. Parallel Pattern

Execute multiple agents concurrently for independent tasks. Reduces latency through parallelism.

**When to use**: Independent tasks, multi-perspective analysis, distributed processing.

**Quick example**:

```yaml
agents:
  - name: source
    outputs: [{ target: analyzer-1 }, { target: analyzer-2 }, { target: analyzer-3 }]
  - name: analyzer-1
    inputs: [{ source: source }]
  - name: analyzer-2
    inputs: [{ source: source }]
  - name: analyzer-3
    inputs: [{ source: source }]
```

**See**: [Parallel Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#3-parallel-pattern)

### 4. Router Pattern

Intelligently route messages to appropriate agents based on classification or routing logic. Achieves 60-80% cost savings in production.

**When to use**: Cost optimization, skill-based routing, conditional workflows.

**Quick example**:

```yaml
agents:
  - name: classifier
    role: classifier
    categories: [simple, moderate, complex]
    outputs:
      - target: simple-handler
        condition: 'category == simple'
      - target: complex-handler
        condition: 'category == complex'
```

**See**: [Router Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#4-router-pattern)

### 5. Swarm Pattern

Autonomous peer-to-peer agent collaboration without central coordination. Agents self-organize and communicate directly.

**When to use**: Brainstorming, consensus building, emergent problem solving.

**Quick example**:

```yaml
agents:
  - name: agent-1
    inputs: [{ source: agent-2 }, { source: agent-3 }]
    outputs: [{ target: agent-2 }, { target: agent-3 }]
  - name: agent-2
    inputs: [{ source: agent-1 }, { source: agent-3 }]
    outputs: [{ target: agent-1 }, { target: agent-3 }]
  - name: agent-3
    inputs: [{ source: agent-1 }, { source: agent-2 }]
    outputs: [{ target: agent-1 }, { target: agent-2 }]
```

**See**: [Swarm Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#5-swarm-pattern)

### 6. Hierarchical Pattern

Tree structure with managers delegating to workers. Enables multi-level task decomposition and scalable coordination.

**When to use**: Large agent teams (>10), organizational modeling, multi-level workflows.

**Quick example**:

```yaml
agents:
  - name: executive
    outputs: [{ target: manager-1 }, { target: manager-2 }]
  - name: manager-1
    inputs: [{ source: executive }]
    outputs: [{ target: worker-1a }, { target: worker-1b }]
  - name: worker-1a
    inputs: [{ source: manager-1 }]
```

**See**: [Hierarchical Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#6-hierarchical-pattern)

### 7. RAG Pattern

Combines knowledge retrieval with generation to ground responses in factual data. Essential for question answering over documents.

**When to use**: Knowledge base integration, reducing hallucinations, domain-specific information.

**Quick example**:

```yaml
agents:
  - name: retriever
    role: retriever
    vector_store: pinecone
    top_k: 5
    outputs: [{ target: generator }]
  - name: generator
    role: react
    inputs: [{ source: retriever }]
```

**See**: [RAG Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#7-rag-pattern)

### 8. Reflection Pattern

Agents critique and improve their outputs through iterative self-review. Improves quality through self-correction.

**When to use**: High-quality content generation, code review, iterative refinement.

**Quick example**:

```yaml
agents:
  - name: generator
    outputs: [{ target: critic }]
  - name: critic
    inputs: [{ source: generator }]
    outputs: [{ target: refiner }]
  - name: refiner
    inputs: [{ source: critic }]
```

**See**: [Reflection Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#8-reflection-pattern)

### 9. Ensemble Pattern

Combine outputs from multiple models to improve accuracy and reduce variance through voting.

**When to use**: High-stakes decisions, model comparison, uncertainty reduction.

**Quick example**:

```yaml
agents:
  - name: model-1
    outputs: [{ target: aggregator }]
  - name: model-2
    outputs: [{ target: aggregator }]
  - name: model-3
    outputs: [{ target: aggregator }]
  - name: aggregator
    role: aggregator
    strategy: majority-vote
    inputs: [{ source: model-1 }, { source: model-2 }, { source: model-3 }]
```

**See**: [Ensemble Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#9-ensemble-pattern)

### 10. Classifier Pattern

Categorize inputs into predefined categories for intelligent routing. Achieves 95%+ accuracy in production.

**When to use**: Content categorization, intent detection, automated triage.

**Quick example**:

```yaml
agents:
  - name: classifier
    role: classifier
    categories: [technical, business, legal, marketing]
    outputs:
      - target: technical-handler
        condition: 'has_category("technical")'
```

**See**: [Classifier Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#10-classifier-pattern)

### 11. Aggregation Pattern

Combine multiple inputs into coherent synthesis. Supports consensus, weighted, and semantic aggregation strategies.

**When to use**: Multi-source data fusion, consensus decision making, report generation.

**Quick example**:

```yaml
agents:
  - name: aggregator
    role: aggregator
    strategy: consensus
    min_inputs: 2
    timeout: 30s
    inputs: [{ source: source-1 }, { source: source-2 }, { source: source-3 }]
```

**See**: [Aggregation Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#11-aggregation-pattern)

### 12. Planning Pattern

Decompose complex tasks into executable steps for strategic problem solving.

**When to use**: Complex task decomposition, multi-step workflows, dynamic workflow generation.

**Quick example**:

```yaml
agents:
  - name: planner
    role: planner
    strategy: chain-of-thought
    max_steps: 10
    outputs: [{ target: executor }]
  - name: executor
    inputs: [{ source: planner }]
```

**See**: [Planning Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#12-planning-pattern)

### 13. MapReduce Pattern

Distribute processing across workers (map) and aggregate results (reduce) for large-scale data processing.

**When to use**: Batch document processing, large dataset analysis, distributed computation.

**Quick example**:

```yaml
agents:
  - name: splitter
    outputs: [{ target: mapper-1 }, { target: mapper-2 }, { target: mapper-3 }]
  - name: mapper-1
    inputs: [{ source: splitter }]
    outputs: [{ target: reducer }]
  - name: reducer
    role: aggregator
    strategy: concatenate
    inputs: [{ source: mapper-1 }, { source: mapper-2 }, { source: mapper-3 }]
```

**See**: [MapReduce Pattern Details](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#13-mapreduce-pattern)

## Pattern Comparison Matrix

| Pattern | Complexity | Latency | Cost | Accuracy | Use When |
|---------|-----------|---------|------|----------|----------|
| Sequential | Low | High | Low | Medium | Ordered execution needed |
| Parallel | Low | Low | Medium | Medium | Independent tasks |
| Router | Medium | Low | Low | Medium | Cost optimization critical |
| Supervisor | Low | Medium | Low | Medium | Always (foundation) |
| Swarm | High | High | High | High | Creative problem solving |
| Hierarchical | High | Medium | Medium | Medium | Large agent teams |
| RAG | Medium | Medium | Medium | High | Knowledge grounding needed |
| Reflection | Medium | High | High | High | Quality > speed |
| Ensemble | Medium | High | High | High | Critical decisions |
| Classifier | Low | Low | Low | High | Categorization needed |
| Aggregation | Medium | Medium | Medium | High | Multi-source synthesis |
| Planning | Medium | Medium | Medium | Medium | Complex task decomposition |
| MapReduce | Medium | Low | Medium | Medium | Large-scale processing |

## Pattern Selection Guide

### By Use Case

**Cost Optimization**

- Router: 60-80% cost savings by routing to appropriate model tiers
- RAG: Reduce hallucinations, fewer retries
- Classifier: Efficient categorization before expensive processing

**Speed/Performance**

- Parallel: Concurrent execution reduces latency
- Router: Fast routing to appropriate handlers
- MapReduce: Distributed processing for throughput

**Accuracy/Quality**

- Ensemble: Multi-model consensus improves accuracy
- Reflection: Iterative refinement enhances quality
- RAG: Grounded responses reduce errors

**Flexibility**

- Swarm: Emergent behavior adapts to scenarios
- Supervisor: Central control enables dynamic orchestration
- Hierarchical: Scalable multi-level delegation

**Complex Workflows**

- Hierarchical: Multi-level task decomposition
- Sequential: Ordered multi-stage processing
- Planning: Strategic task breakdown

### Decision Tree

```text
Start: What's your primary goal?

├─ Cost Optimization
│  ├─ Route by complexity? → Router Pattern
│  ├─ Ground in knowledge? → RAG Pattern
│  └─ Categorize first? → Classifier Pattern
│
├─ Speed/Performance
│  ├─ Independent tasks? → Parallel Pattern
│  ├─ Large dataset? → MapReduce Pattern
│  └─ Route efficiently? → Router Pattern
│
├─ Accuracy/Quality
│  ├─ Multi-model consensus? → Ensemble Pattern
│  ├─ Iterative improvement? → Reflection Pattern
│  └─ Knowledge grounding? → RAG Pattern
│
├─ Flexibility
│  ├─ Emergent behavior? → Swarm Pattern
│  ├─ Central control? → Supervisor Pattern
│  └─ Multi-level hierarchy? → Hierarchical Pattern
│
└─ Complex Workflows
   ├─ Ordered stages? → Sequential Pattern
   ├─ Task decomposition? → Planning Pattern
   └─ Large team? → Hierarchical Pattern
```

## Combining Patterns

Patterns can be composed to create sophisticated systems. Common combinations:

**Router + RAG**: Route queries by complexity, use RAG for complex queries requiring knowledge grounding.

**Classifier + Parallel**: Classify input, then run parallel analyses on each category.

**Sequential + Reflection**: Multi-stage pipeline where each stage includes reflection for quality.

**Hierarchical + MapReduce**: Hierarchical coordination of MapReduce workers for massive scale.

**Ensemble + Aggregation**: Multiple models with sophisticated aggregation strategies.

## Implementation Status

**Production Ready (13 patterns)**

All 13 patterns are stable and production-ready:

- Supervisor, Sequential, Parallel, Router
- Swarm, Hierarchical, RAG, Reflection
- Ensemble, Classifier, Aggregation
- Planning, MapReduce

**Future Patterns (2 patterns)**

Under research and development:

- Tool-Use Pattern: Agents with function calling capabilities
- Memory Pattern: Long-term memory and context retention

## Phased Agent Startup with Dependencies

**New in v0.2.3**: Aixgo now provides dependency-aware agent startup that eliminates race conditions and ensures agents initialize in the correct order.

### The Problem: Startup Race Conditions

In multi-agent systems, orchestrator agents often need to verify their dependencies are ready during startup. When all agents start concurrently, race conditions can occur where orchestrators try to use dependencies that haven't finished initializing.

### The Solution: depends_on Field

Declare explicit startup dependencies using the `depends_on` field:

```yaml
agents:
  # Phase 0: No dependencies
  - name: database
    role: producer

  # Phase 1: Depends on database
  - name: cache
    role: producer
    depends_on: [database]

  # Phase 2: Depends on database and cache
  - name: api
    role: react
    depends_on: [database, cache]
```

### How It Works

1. **Topological Sort**: Aixgo uses Kahn's algorithm to compute startup order
1. **Phase Grouping**: Agents are grouped into phases (0, 1, 2, ...) based on dependencies
1. **Concurrent Within Phases**: Agents in the same phase start concurrently for performance
1. **Ready Polling**: Each phase waits for all agents to be `Ready()` before proceeding
1. **Timeout Protection**: Configurable timeout (30s default) prevents infinite waiting

### Benefits

**Eliminates Race Conditions**

```yaml
# Before v0.2.3: Race condition possible
agents:
  - name: orchestrator  # Might start before workers
  - name: worker1
  - name: worker2

# After v0.2.3: Guaranteed order
agents:
  - name: worker1
  - name: worker2
  - name: orchestrator
    depends_on: [worker1, worker2]  # Starts only after workers are ready
```

**Parallel Performance**

Agents without dependencies on each other start concurrently:

```yaml
agents:
  # These start concurrently (Phase 0)
  - name: service-a
  - name: service-b
  - name: service-c

  # This starts after all above are ready (Phase 1)
  - name: orchestrator
    depends_on: [service-a, service-b, service-c]
```

**Clear Error Messages**

If startup fails, you get precise information about which agent didn't become ready:

```text
agent startup failed: agent 'database' not ready after 30s timeout
```

### Configuration

#### Startup Timeout

Control how long to wait for agents to become ready:

```yaml
config:
  agent_start_timeout: 45s  # Default: 30s
```

#### Complex Dependencies

Build multi-tier systems with complex dependency graphs:

```yaml
agents:
  # Tier 1: Foundation
  - name: config-service
    role: producer

  - name: database
    role: producer

  # Tier 2: Services depending on Tier 1
  - name: cache
    role: producer
    depends_on: [database, config-service]

  - name: auth-service
    role: react
    depends_on: [database]

  # Tier 3: Application layer
  - name: user-service
    role: react
    depends_on: [database, cache, auth-service]

  - name: order-service
    role: react
    depends_on: [database, cache, auth-service]

  # Tier 4: API Gateway
  - name: api-gateway
    role: react
    depends_on: [user-service, order-service]
```

**Startup sequence**:

1. Phase 0: `config-service`, `database` (concurrent)
1. Phase 1: `cache`, `auth-service` (concurrent, after Phase 0)
1. Phase 2: `user-service`, `order-service` (concurrent, after Phase 1)
1. Phase 3: `api-gateway` (after Phase 2)

### Backward Compatibility

Phased startup is **opt-in and backward compatible**:

- **Without `depends_on`**: All agents start concurrently (v0.2.2 behavior)
- **With `depends_on`**: Agents start in phases based on dependencies

### Best Practices

1. **Declare True Dependencies**: Only use `depends_on` for agents that must be ready before others start
1. **Minimize Phases**: Group independent agents at the same tier for faster startup
1. **Fast Ready() Checks**: Keep `Ready()` implementations lightweight (simple boolean checks)
1. **Set Appropriate Timeouts**: Increase `agent_start_timeout` if agents need time to initialize

### Cycle Detection

Aixgo automatically detects circular dependencies and fails fast with a clear error:

```yaml
# This configuration will fail validation
agents:
  - name: agent-a
    depends_on: [agent-b]
  - name: agent-b
    depends_on: [agent-a]  # Circular dependency!
```

Error message:

```text
circular dependency detected: agent-a -> agent-b -> agent-a
```

### Runtime Support

Phased startup works across all runtime implementations:

- **LocalRuntime**: Single-process deployments
- **Runtime**: Lightweight runtime
- **DistributedRuntime**: Multi-node gRPC-based deployments

### Message-Based Dependencies (Legacy)

Before v0.2.3, dependencies were implicit through inputs/outputs. This still works but doesn't guarantee startup order:

```yaml
# Legacy approach: message routing (doesn't guarantee startup order)
agents:
  - name: worker
    outputs: [{ target: orchestrator }]
  - name: orchestrator
    inputs: [{ source: worker }]
```

**Recommendation**: Use `depends_on` for startup dependencies, keep `inputs`/`outputs` for message routing.

## Execution Constraints

### Max Rounds

Limits the total number of workflow iterations:

```yaml
supervisor:
  max_rounds: 10 # Stop after 10 iterations
```

This prevents runaway workflows and ensures predictable resource usage.

### Agent-Level Timeouts

Set timeouts per agent for long-running operations:

```yaml
agents:
  - name: slow-processor
    role: react
    model: gpt-4-turbo
    timeout: 30s # Fail if processing takes >30s
```

## Error Handling

The supervisor provides built-in error handling:

### Automatic Retry

Configure retry behavior for transient failures:

```yaml
agents:
  - name: api-caller
    role: react
    retry:
      max_attempts: 3
      backoff: exponential
      initial_interval: 1s
```

### Graceful Degradation

Agents can fail without crashing the entire system:

```yaml
supervisor:
  failure_mode: continue # or 'stop' to halt on first error
```

## Best Practices

### 1. Keep Workflows Acyclic

Avoid circular dependencies—messages should flow in one direction:

**Bad:**

```yaml
# Creates a cycle: A → B → C → A
agents:
  - name: A
    outputs: [{ target: B }]
    inputs: [{ source: C }] # Circular!
```

**Good:**

```yaml
# Acyclic: A → B → C
agents:
  - name: A
    outputs: [{ target: B }]
  - name: B
    inputs: [{ source: A }]
    outputs: [{ target: C }]
  - name: C
    inputs: [{ source: B }]
```

### 2. Use Descriptive Names

Agent names should describe their role:

```yaml
agents:
  - name: customer-data-enricher # Clear
  - name: agent-1 # Unclear
```

### 3. Set Reasonable max_rounds

Too high = wasted resources, too low = incomplete workflows:

```yaml
supervisor:
  max_rounds: 10 # Typical: 5-20 rounds
```

### 4. Monitor Performance

Use observability to track:

- Message latency between agents
- Agent processing time
- Bottlenecks in the workflow
- Pattern-specific metrics

## Real-World Example: Content Moderation Pipeline

This example combines multiple patterns (Classifier, Parallel, Aggregation, Planning):

```yaml
supervisor:
  name: content-moderation
  max_rounds: 100

agents:
  # Ingest content submissions
  - name: content-ingestion
    role: producer
    interval: 100ms
    outputs:
      - target: content-classifier
      - target: text-analyzer
      - target: image-analyzer

  # Classify content type (Classifier Pattern)
  - name: content-classifier
    role: classifier
    model: gpt-4-turbo
    prompt: 'Classify content into categories'
    strategy: multi-label
    categories:
      - user-generated
      - commercial
      - news
      - entertainment
    inputs:
      - source: content-ingestion
    outputs:
      - target: decision-aggregator

  # Parallel analysis (Parallel Pattern)
  - name: text-analyzer
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze text for policy violations'
    inputs:
      - source: content-ingestion
    outputs:
      - target: decision-aggregator

  - name: image-analyzer
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze images for inappropriate content'
    inputs:
      - source: content-ingestion
    outputs:
      - target: decision-aggregator

  # Aggregate results (Aggregation Pattern)
  - name: decision-aggregator
    role: aggregator
    model: gpt-4-turbo
    prompt: 'Combine classification, text, and image analysis using consensus strategy'
    strategy: consensus
    inputs:
      - source: content-classifier
      - source: text-analyzer
      - source: image-analyzer
    outputs:
      - target: action-planner

  # Plan actions (Planning Pattern)
  - name: action-planner
    role: planner
    model: gpt-4-turbo
    prompt: 'Plan appropriate actions: approve, reject, or flag for human review'
    strategy: chain-of-thought
    inputs:
      - source: decision-aggregator
    outputs:
      - target: action-handler

  # Execute decision
  - name: action-handler
    role: logger
    inputs:
      - source: action-planner
```

This pipeline demonstrates:

1. Ingests content at 10 submissions/second
2. Classifies content type with multi-label strategy (Classifier Pattern)
3. Analyzes text and images in parallel (Parallel Pattern)
4. Aggregates results using consensus strategy (Aggregation Pattern)
5. Plans actions with chain-of-thought reasoning (Planning Pattern)
6. Executes appropriate action

## Next Steps

- **[Production Deployment](/guides/production-deployment)** - Deploy multi-agent systems to production
- **[Observability & Monitoring](/guides/observability)** - Track and debug complex workflows
- **[Type Safety & LLM Integration](/guides/type-safety)** - Build reliable agent interactions
- **[Examples Repository](https://github.com/aixgo-dev/aixgo/tree/main/examples)** - See working examples of all patterns
