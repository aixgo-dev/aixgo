---
title: "Orchestration Patterns"
description: "Production-proven agent orchestration patterns in Aixgo"
weight: 30
---

# Agent Orchestration Patterns

Aixgo provides **13 production-proven orchestration patterns** for building AI agent systems. Each pattern solves specific problems and is backed by real-world usage from industry-leading frameworks.

All patterns are **fully implemented and production-ready** in v0.2.0+.

> 📖 **Complete Technical Documentation:**
> - **[PATTERNS.md on GitHub](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md)** - **Complete pattern catalog** with detailed code examples, configuration templates, use cases, and implementation guides
> - **[FEATURES.md on GitHub](https://github.com/aixgo-dev/aixgo/blob/main/docs/FEATURES.md)** - Authoritative feature reference with all capabilities
> - **[GitHub Repository](https://github.com/aixgo-dev/aixgo)** - Source code and examples
>
> **This page provides a marketing-friendly overview for evaluation and planning.**

## Pattern Overview

<div class="pattern-grid">

### ✅ Implemented (v0.2.0+)

<div class="pattern-card implemented">

#### Supervisor Pattern
**Centralized orchestration with specialized agents**

Route tasks to expert agents, aggregate results, and maintain conversation state. Perfect for customer service and multi-agent workflows.

```yaml
orchestration:
  pattern: supervisor
  agents: [billing, tech-support, sales]
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#1-supervisor-pattern)

</div>

<div class="pattern-card implemented">

#### Sequential Pattern
**Ordered pipeline execution**

Execute agents in sequence where each step's output feeds the next. Ideal for ETL, content pipelines, and multi-stage workflows.

```yaml
orchestration:
  pattern: sequential
  agents: [extract, transform, validate, load]
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#2-sequential-pattern)

</div>

<div class="pattern-card implemented">

#### Parallel Pattern
**Concurrent execution with aggregation**

Execute multiple agents simultaneously and aggregate results. **3-4× speedup** for independent tasks.

**Use cases**: Multi-source research, batch processing, A/B testing

```yaml
orchestration:
  pattern: parallel
  agents: [competitive-analysis, market-sizing, tech-trends]
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#3-parallel-pattern)

</div>

<div class="pattern-card implemented">

#### Router Pattern
**Intelligent routing for cost optimization**

Route simple queries to cheap models, complex to expensive. **25-50% cost reduction** in production.

**Use cases**: Cost optimization, intent-based routing, model selection

```yaml
orchestration:
  pattern: router
  classifier: intent-classifier
  routes:
    simple: gpt-3.5-turbo
    complex: gpt-4-turbo
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#4-router-pattern)

</div>

<div class="pattern-card implemented">

#### Swarm Pattern
**Decentralized agent handoffs**

Dynamic agent-to-agent handoffs based on conversational context. Popularized by OpenAI Swarm.

**Use cases**: Customer service handoffs, adaptive routing, collaborative problem-solving

```yaml
orchestration:
  pattern: swarm
  agents: [general, billing, technical]
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#5-swarm-pattern)

</div>

<div class="pattern-card implemented">

#### Hierarchical Pattern
**Multi-level delegation**

Managers delegate to sub-managers who delegate to workers. Perfect for complex decomposition.

**Use cases**: Enterprise workflows, project management, organizational hierarchies

```yaml
orchestration:
  pattern: hierarchical
  manager: project-manager
  teams:
    frontend: [ui-engineer, ux-engineer]
    backend: [api-engineer, db-engineer]
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#6-hierarchical-pattern)

</div>

<div class="pattern-card implemented">

#### RAG Pattern
**Retrieval-Augmented Generation**

Retrieve relevant docs from vector store, then generate grounded answers. **Most common enterprise pattern**.

**Use cases**: Enterprise chatbots, documentation Q&A, knowledge retrieval

```yaml
orchestration:
  pattern: rag
  retriever: vector-store
  generator: answer-agent
  top_k: 5
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#7-rag-pattern)

</div>

<div class="pattern-card implemented">

#### Reflection Pattern
**Iterative refinement with self-critique**

Generator creates output, critic reviews it, generator refines. **20-50% quality improvement**.

**Use cases**: Code generation, content creation, complex reasoning

```yaml
orchestration:
  pattern: reflection
  generator: code-generator
  critic: code-reviewer
  max_iterations: 3
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#8-reflection-pattern)

</div>

<div class="pattern-card implemented">

#### Ensemble Pattern
**Multi-model voting for accuracy**

Multiple models vote on outputs to reduce errors. **25-50% error reduction** in high-stakes decisions.

**Use cases**: Medical diagnosis, financial forecasting, content moderation

```yaml
orchestration:
  pattern: ensemble
  models: [gpt-4, claude-3.5, gemini-1.5]
  voting: majority
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#9-ensemble-pattern)

</div>

<div class="pattern-card implemented">

#### Classifier Pattern
**Intent-based routing**

Classify user intent and route to specialized agents. Perfect for support ticket routing and content categorization.

**Use cases**: Ticket classification, intent-based routing, content categorization

```yaml
orchestration:
  pattern: classifier
  classifier: intent-classifier
  routes:
    technical: tech-agent
    billing: billing-agent
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#10-classifier-pattern)

</div>

<div class="pattern-card implemented">

#### Aggregation Pattern
**Multi-agent synthesis**

Combine outputs from multiple agents using consensus, weighted, or semantic strategies.

**Use cases**: Expert synthesis, multi-source analysis, decision fusion

```yaml
orchestration:
  pattern: aggregation
  agents: [expert-1, expert-2, expert-3]
  strategy: consensus
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#11-aggregation-pattern)

</div>

<div class="pattern-card implemented">

#### Planning Pattern
**Dynamic task decomposition**

Break complex tasks into subtasks with dynamic replanning based on execution results.

**Use cases**: Multi-step research, data pipelines, software development

```yaml
orchestration:
  pattern: planning
  planner: task-planner
  executors: [executor-1, executor-2]
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#12-planning-pattern)

</div>

<div class="pattern-card implemented">

#### MapReduce Pattern
**Distributed batch processing**

Process large datasets in parallel chunks and aggregate results.

**Use cases**: Batch processing, data analysis, document processing

```yaml
orchestration:
  pattern: mapreduce
  mapper: chunk-processor
  reducer: result-aggregator
```

[View Pattern Docs →](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md#13-mapreduce-pattern)

</div>

### 🔮 Future Patterns (Roadmap)

<div class="pattern-card future">

#### Debate Pattern
**Adversarial collaboration for accuracy**

Agents with different perspectives debate to reach consensus. **20-40% improvement in factual accuracy**.

**Use cases**: Research synthesis, legal analysis, complex decision-making

**Phase 5** · v2.1+ (2025 H2)

</div>

<div class="pattern-card future">

#### Plan-and-Execute Pattern
**Strategic planning before execution**

Planner decomposes task, executors handle sub-tasks, planner adjusts based on results.

**Use cases**: Multi-step research, data pipelines, software development

**Phase 2 or v2.1** · TBD

</div>

<div class="pattern-card future">

#### Nested/Composite Pattern
**Encapsulate complex workflows as single agents**

Complex multi-agent workflows packaged as reusable components.

**Use cases**: Modular agent development, workflow reuse, testing

**Phase 6** · v2.2+ (2025 H2)

</div>

</div>

## Pattern Comparison

| Pattern | Complexity | Cost | Latency | Accuracy | Production Maturity |
|---------|-----------|------|---------|----------|---------------------|
| **Supervisor** | Low | 1× | Low | Medium | ⭐⭐⭐⭐⭐ Very High |
| **Sequential** | Low | N× | High | Medium | ⭐⭐⭐⭐⭐ Very High |
| **Parallel** | Medium | N× | Low | Medium | ⭐⭐⭐⭐⭐ Very High |
| **Router** | Low | 0.25-0.5× | Low | High | ⭐⭐⭐⭐⭐ Very High |
| **Swarm** | Medium | Variable | Medium | High | ⭐⭐⭐⭐ High |
| **Hierarchical** | Medium | N× | Medium | High | ⭐⭐⭐⭐ High |
| **RAG** | Medium | 0.3× | Medium | High | ⭐⭐⭐⭐⭐ Very High |
| **Reflection** | Medium | 2-4× | High | Very High | ⭐⭐⭐⭐ High |
| **Ensemble** | Medium | 3-5× | Low | Very High | ⭐⭐⭐⭐ High |
| **Classifier** | Low | 1× | Low | High | ⭐⭐⭐⭐⭐ Very High |
| **Aggregation** | Medium | N× | Medium | High | ⭐⭐⭐⭐ High |
| **Planning** | Medium | Optimized | Medium | High | ⭐⭐⭐⭐ High |
| **MapReduce** | Medium | N× | Low | High | ⭐⭐⭐⭐⭐ Very High |
| **Debate** (Roadmap) | High | 9× | Very High | Very High | 🔮 Future |
| **Nested** (Roadmap) | High | Variable | Variable | Variable | 🔮 Future |

### Cost Legend
- `1×` = Single agent execution
- `N×` = N agents (sequential or parallel)
- `0.25-0.5×` = Router savings (cheap models for most queries)
- `0.3×` = RAG token reduction vs full KB
- `2-4×`, `3-5×`, `9×` = Multiple iterations/agents

### Latency Legend
- **Low**: < 1s overhead
- **Medium**: 1-5s overhead
- **High**: 5-15s overhead
- **Very High**: > 15s overhead

## Pattern Selection Guide

### Choose by Goal

**💰 Reduce Costs**
→ **Router** (25-50% savings) or **RAG** (70% token reduction)

**⚡ Improve Speed**
→ **Parallel** (3-4× speedup)

**🎯 Improve Accuracy**
→ **Ensemble** (25-50% error reduction) or **Reflection** (20-50% improvement)

**🔄 Adaptive Routing**
→ **Swarm** (dynamic handoffs) or **Supervisor** (centralized control)

**📊 Complex Workflows**
→ **Hierarchical** (multi-level) or **Sequential** (ordered steps)

**📚 Knowledge-Intensive**
→ **RAG** (retrieval-augmented)

### Decision Tree

```text
Need to reduce costs? → Router or RAG
Need high accuracy? → Ensemble or Reflection
Have independent sub-tasks? → Parallel
Need ordered steps? → Sequential
Need dynamic routing? → Swarm
Need multi-level management? → Hierarchical
Need knowledge base access? → RAG
General orchestration? → Supervisor (default)
```

## Real-World Examples

### Cost Optimization with Router

```go
// Before: Always using GPT-4
// Cost: $0.03 per request
// After: Router to GPT-3.5 for 80% of queries
// Cost: $0.006 per request (80% savings)

router := orchestration.NewRouter(
    "cost-optimizer",
    runtime,
    "complexity-classifier",
    map[string]string{
        "simple":  "gpt-3.5-turbo-agent",
        "complex": "gpt-4-turbo-agent",
    },
)
```

**Result**: **25-50% cost reduction** in production deployments.

### Speed Improvement with Parallel

```go
// Before: Sequential execution
// Time: 10s (sum of all agents)
// After: Parallel execution
// Time: 3s (max of all agents)

parallel := orchestration.NewParallel(
    "market-research",
    runtime,
    []string{"competitors", "market-size", "trends", "regulations"},
)
```

**Result**: **3-4× speedup** for independent research tasks.

### Accuracy Improvement with Ensemble

```go
// Before: Single model (GPT-4)
// Error rate: 15%
// After: 3-model ensemble
// Error rate: 6% (60% reduction)

ensemble := orchestration.NewEnsemble(
    "medical-diagnosis",
    runtime,
    []string{"gpt4-diagnostic", "claude-diagnostic", "gemini-diagnostic"},
    orchestration.WithVotingStrategy(orchestration.VotingMajority),
)
```

**Result**: **25-50% error reduction** for high-stakes decisions.

## Roadmap

All 13 core patterns are **implemented and production-ready** in v0.2.0+.

### Future Patterns (v2.1+, 2025 H2)

- 🔮 **Debate Pattern** - Adversarial collaboration for accuracy
- 🔮 **Nested/Composite Pattern** - Encapsulate complex workflows as single agents

## Getting Started

### Installation

```bash
go get github.com/aixgo-dev/aixgo
```

### Quick Example

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  - name: data-producer
    role: producer
    interval: 1s
    outputs:
      - target: analyzer

  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a data analyst. Analyze incoming data and provide insights.
    inputs:
      - source: data-producer
    outputs:
      - target: logger

  - name: logger
    role: logger
    inputs:
      - source: analyzer
```

```go
// main.go
package main

import (
    "github.com/aixgo-dev/aixgo"
    _ "github.com/aixgo-dev/aixgo/agents"
)

func main() {
    if err := aixgo.Run("config/agents.yaml"); err != nil {
        panic(err)
    }
}
```

### Learn More

- [Documentation](https://aixgo.dev)
- [Pattern Catalog](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md)
- [Examples](https://github.com/aixgo-dev/aixgo/tree/main/examples)
- [GitHub Repository](https://github.com/aixgo-dev/aixgo)

## Support

- **GitHub Issues**: [github.com/aixgo-dev/aixgo/issues](https://github.com/aixgo-dev/aixgo/issues)
- **Documentation**: [aixgo.dev](https://aixgo.dev)
- **Discord**: Join our community (coming soon)

---

<style>
.pattern-grid {
    display: grid;
    gap: 2rem;
    margin: 2rem 0;
}

.pattern-card {
    border: 2px solid #e0e0e0;
    border-radius: 8px;
    padding: 1.5rem;
    background: #f9f9f9;
}

.pattern-card.implemented {
    border-color: #22c55e;
    background: #f0fdf4;
}

.pattern-card.planned {
    border-color: #3b82f6;
    background: #eff6ff;
}

.pattern-card.future {
    border-color: #a855f7;
    background: #faf5ff;
}

.pattern-card h4 {
    margin-top: 0;
    color: #1f2937;
}

.pattern-card pre {
    background: #fff;
    border: 1px solid #e0e0e0;
    border-radius: 4px;
    padding: 1rem;
    font-size: 0.875rem;
}

table {
    width: 100%;
    border-collapse: collapse;
    margin: 2rem 0;
}

table th,
table td {
    border: 1px solid #e0e0e0;
    padding: 0.75rem;
    text-align: left;
}

table th {
    background: #f3f4f6;
    font-weight: 600;
}

table tr:nth-child(even) {
    background: #f9fafb;
}
</style>
