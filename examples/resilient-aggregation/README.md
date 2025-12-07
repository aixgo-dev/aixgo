# Resilient Aggregation Example

Demonstrates the Aggregator agent's comprehensive resilience features: 9 aggregation strategies (5 LLM + 4 deterministic), buffer timeout for missing inputs, partial result aggregation, and confidence-based voting.

## Overview

Aixgo's `AggregatorAgent` is production-ready with built-in resilience:

### 9 Aggregation Strategies

**LLM-Powered** (intelligent synthesis):

1. `consensus` - Find consensus among inputs
2. `weighted` - Weighted aggregation by source importance
3. `semantic` - Semantic clustering and synthesis
4. `hierarchical` - Multi-level summarization
5. `rag_based` - Retrieval-augmented generation

**Deterministic** (zero LLM cost):

6. `voting_majority` - Simple majority vote
7. `voting_unanimous` - Require all to agree
8. `voting_weighted` - Confidence-weighted voting
9. `voting_confidence` - Highest confidence wins

## Quick Start

```bash
go run main.go
```

## Code Examples

### Creating a Resilient Aggregator

```go
agentDef := &agent.AgentDef{
    Name:  "resilient-aggregator",
    Role:  "aggregator",
    Model: "gpt-4",
    Extra: map[string]any{
        "aggregator_config": map[string]any{
            "aggregation_strategy": agents.StrategyVotingMajority,
            "timeout_ms":          5000,  // Wait 5s for inputs
            "max_input_sources":    5,     // Expect up to 5 agents
            // Even if only 3 respond, aggregator will process them
        },
    },
}

aggregator, _ := agents.NewAggregatorAgent(*agentDef, runtime)
```

### Feature 1: Handling Missing Inputs

Expect 5 agents, only 3 respond:

```go
config := agents.AggregatorConfig{
    AggregationStrategy: agents.StrategyVotingMajority,
    TimeoutMs:          5000,
    MaxInputSources:    5,
}

// Only 3 agents respond within timeout
inputs := []*agents.AgentInput{
    {AgentName: "agent-1", Content: "Option A", Confidence: 0.8},
    {AgentName: "agent-2", Content: "Option A", Confidence: 0.7},
    {AgentName: "agent-3", Content: "Option B", Confidence: 0.6},
}

// Aggregator proceeds with available inputs
result, _ := aggregator.Execute(ctx, msg)
// Result: "Option A" (2/3 majority), Agreement: 66%
```

### Feature 2: Partial Result Aggregation

```go
// Use weighted voting to prioritize high-confidence inputs
config := agents.AggregatorConfig{
    AggregationStrategy: agents.StrategyVotingWeighted,
}

inputs := []*agents.AgentInput{
    {AgentName: "agent-1", Content: "Complete analysis", Confidence: 0.9},
    {AgentName: "agent-2", Content: "Complete analysis", Confidence: 0.85},
    {AgentName: "agent-3", Content: "Partial data", Confidence: 0.3},  // Low quality
    {AgentName: "agent-4", Content: "Complete analysis", Confidence: 0.8},
}

// High-confidence inputs dominate
result, _ := aggregator.Execute(ctx, msg)
```

### Feature 3: Confidence-Based Selection

```go
config := agents.AggregatorConfig{
    AggregationStrategy: agents.StrategyVotingConfidence,
}

inputs := []*agents.AgentInput{
    {AgentName: "novice", Content: "Probably A", Confidence: 0.5},
    {AgentName: "expert", Content: "Definitely C", Confidence: 0.95},
    {AgentName: "senior", Content: "Likely A", Confidence: 0.8},
}

// Selects "Definitely C" (highest confidence: 0.95)
result, _ := aggregator.Execute(ctx, msg)
```

## Strategy Comparison

### LLM-Powered Strategies

| Strategy | Use Case | Pros | Cons |
|----------|----------|------|------|
| `consensus` | General-purpose | Intelligent synthesis | Higher cost |
| `weighted` | Expert panels | Respects source authority | Requires weight config |
| `semantic` | Opinion clustering | Groups similar views | Computationally intensive |

### Deterministic Strategies (Zero LLM Cost)

| Strategy | Use Case | Pros | Cons |
|----------|----------|------|------|
| `voting_majority` | Democratic decisions | Simple, fast, fair | Ignores confidence |
| `voting_unanimous` | Safety-critical | Ensures agreement | Fails on disagreement |
| `voting_weighted` | Confidence voting | Respects uncertainty | Requires confidence scores |
| `voting_confidence` | Expert selection | Defers to most confident | Single point of failure |

## Resilience Patterns

### Pattern 1: Best-Effort Aggregation

```go
config := agents.AggregatorConfig{
    AggregationStrategy: agents.StrategyVotingMajority,
    TimeoutMs:          3000,
    MaxInputSources:    10,
}
// Aggregates 3-10 inputs, no error if < 10 respond
```

### Pattern 2: High-Confidence Requirement

```go
config := agents.AggregatorConfig{
    AggregationStrategy: agents.StrategyVotingWeighted,
    ConsensusThreshold:  0.8,  // 80% agreement required
}
// Returns error if agreement < 80%
```

### Pattern 3: Fallback Chain

```go
strategies := []string{
    agents.StrategyConsensus,        // Try LLM first
    agents.StrategyVotingWeighted,   // Fallback to weighted
    agents.StrategyVotingMajority,   // Final fallback: majority
}

for _, strategy := range strategies {
    result, err = aggregator.Execute(ctx, msg)
    if err == nil {
        break  // Success
    }
}
```

## Configuration Examples

### Conservative (High Quality)

```yaml
aggregator_config:
  aggregation_strategy: consensus
  timeout_ms: 10000
  consensus_threshold: 0.8
  temperature: 0.3
```

### Balanced (Production Default)

```yaml
aggregator_config:
  aggregation_strategy: voting_weighted
  timeout_ms: 5000
  consensus_threshold: 0.6
```

### Fast (High Throughput)

```yaml
aggregator_config:
  aggregation_strategy: voting_majority
  timeout_ms: 2000
```

### Cost-Optimized

```yaml
aggregator_config:
  aggregation_strategy: voting_confidence  # Deterministic, zero LLM cost
  timeout_ms: 3000
```

## Learn More

- [Deterministic Aggregation Example](../deterministic-aggregation/) - Voting strategies deep dive
- [Multi-Phase Workflow Example](../multi-phase-workflow/) - Using aggregators in workflows
- [Aggregator Agent Documentation](/docs/agents/aggregator.md) - Full API reference
