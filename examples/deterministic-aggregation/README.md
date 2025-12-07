# Deterministic Aggregation Example

Demonstrates Aixgo's deterministic (non-LLM) aggregation strategies for combining outputs from multiple agents. These strategies are cost-free, deterministic, fast (millisecond latency), and production-ready.

## Overview

Unlike LLM-based aggregation, deterministic strategies offer:

- **Cost-free**: Zero LLM API calls
- **Deterministic**: Same inputs always produce same output
- **Fast**: Millisecond latency
- **Transparent**: Clear, auditable decision logic
- **Production-ready**: Suitable for systems requiring reproducibility

## Use Case: Policy Analysis

This example simulates a climate policy expert panel where five specialists provide recommendations. We demonstrate four voting strategies.

## Voting Strategies

### 1. Majority Vote (`voting_majority`)

**How it works**: The most common recommendation wins.

**When to use**: Democratic decision-making, crowd wisdom, all opinions equal weight.

**Example output**:

```text
Selected: Implement carbon tax and renewable energy incentives
Agreement: 0.80 (80%)
Explanation: Majority vote: 4/5 agents agreed
```

### 2. Unanimous Vote (`voting_unanimous`)

**How it works**: All agents must agree. Fails if there's any disagreement.

**When to use**: Critical safety decisions, regulatory compliance, consensus requirements.

**Example output**:

```text
Error: unanimous vote failed: industry-representative disagrees with environmental-scientist
```

### 3. Weighted Vote (`voting_weighted`)

**How it works**: Votes weighted by confidence scores. Higher confidence = more influence.

**When to use**: Expert panels with varying expertise, meaningful confidence levels.

**Example output**:

```text
Selected: Implement carbon tax
Agreement: 0.85 (85%)
Explanation: Weighted vote: 4.50/5.25 (86% of total weight)
```

### 4. Confidence Vote (`voting_confidence`)

**How it works**: Agent with highest confidence wins, regardless of majority.

**When to use**: Defer to expert judgment, trust-based systems, one agent clearly more qualified.

**Example output**:

```text
Selected: Implement carbon tax
Agreement: 0.95
Explanation: Confidence vote: selected from energy-analyst with highest confidence 0.95
```

## Running the Example

```bash
go run main.go
```

## Configuration

Edit `config.yaml` to customize:

```yaml
scenario: "Your policy question"

experts:
  - name: "expert-1"
    role: "Expert Role"
    recommendation: "Their recommendation"
    confidence: 0.9  # 0-1 confidence score
    rationale: "Why they recommend this"

voting_strategies:
  - "voting_majority"
  - "voting_unanimous"
  - "voting_weighted"
  - "voting_confidence"
```

## Comparison: Deterministic vs LLM Aggregation

| Aspect | Deterministic | LLM-Based |
|--------|--------------|-----------|
| **Cost** | Free | API costs per aggregation |
| **Speed** | <1ms | 500-2000ms |
| **Reproducibility** | 100% deterministic | May vary between runs |
| **Flexibility** | Fixed voting rules | Handles nuance, synthesis |
| **Auditability** | Fully transparent | Black box decisions |

## When to Use Deterministic Aggregation

Choose deterministic strategies when:

1. **Reproducibility is critical**: Audit logs, compliance, testing
2. **Cost matters**: High-volume operations, budget constraints
3. **Simple aggregation suffices**: Voting, consensus, selection
4. **Speed is essential**: Real-time systems, low-latency requirements
5. **Transparency needed**: Explainable decisions, regulated industries

## Advanced Usage

### Custom Confidence Scoring

```go
inputs := []*agents.AgentInput{
    {AgentName: "expert-1", Content: "recommendation", Confidence: 0.95},
    {AgentName: "expert-2", Content: "recommendation", Confidence: 0.60},
}
```

### Programmatic Strategy Selection

```go
func selectStrategy(requiresConsensus bool, hasConfidenceScores bool) string {
    if requiresConsensus {
        return agents.StrategyVotingUnanimous
    }
    if hasConfidenceScores {
        return agents.StrategyVotingWeighted
    }
    return agents.StrategyVotingMajority
}
```

### Error Handling

```go
result, err := aggregator.Execute(ctx, msg)
if err != nil {
    if strings.Contains(err.Error(), "unanimous vote failed") {
        log.Printf("Consensus not reached, escalating decision...")
    }
}
```

## Performance Characteristics

```text
Benchmark Results (M1 Mac):
- Majority Vote:   ~0.1ms per aggregation
- Unanimous Vote:  ~0.1ms per aggregation
- Weighted Vote:   ~0.2ms per aggregation
- Confidence Vote: ~0.1ms per aggregation

vs LLM Aggregation: ~800ms average (8000× slower)
```

## Learn More

- [Resilient Aggregation Example](../resilient-aggregation/) - LLM-based aggregation strategies
- [Aggregator Agent Documentation](/docs/agents/aggregator.md) - Full API reference
- [Multi-Phase Workflow Example](../multi-phase-workflow/) - Aggregators in workflows
