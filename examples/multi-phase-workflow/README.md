# Multi-Phase Workflow Example

Demonstrates complex multi-phase workflows by composing Aixgo's orchestration patterns: Parallel, Ensemble, Sequential, and Router.

## Overview

Combine orchestration patterns to create sophisticated workflows:

1. **Parallel**: Execute multiple agents concurrently
2. **Ensemble**: Multi-model voting with validation thresholds
3. **Sequential**: Step-by-step processing with dependencies
4. **Router**: Dynamic routing based on classification

## Quick Start

```bash
go run main.go
```

## Architecture Patterns

### Pattern 1: Parallel → Ensemble → Sequential

```text
Phase 1: Parallel Data Extraction (3-4× faster)
   Agent 1, Agent 2, Agent 3 (concurrent)
         ↓
Phase 2: Ensemble Voting (Validation Gate, 60% agreement required)
   Aggregator 1, Aggregator 2 (voting)
         ↓
Phase 3: Sequential Risk Assessment (ordered processing)
   Risk Analysis → Recommendations
```

### Pattern 2: Parallel → Merge → Sequential

```text
Input → Parallel Feature Extraction
         ├─► Spec Extractor
         ├─► Review Analyzer
         └─► Competitor Analyzer
      → Merge & Validate
      → Sequential Description Generation
         ├─► Short Description
         └─► Long Description (uses short as context)
```

## Code Examples

### Example 1: Policy Analysis (3-Phase Workflow)

```go
// PHASE 1: Parallel Extraction (3-4× faster than sequential)
phase1 := orchestration.NewParallel(
    "policy-extraction",
    rt,
    []string{"data-protection-agent", "access-control-agent", "incident-agent"},
    orchestration.WithFailFast(false),
)
phase1Results, _ := phase1.Execute(ctx, input)

// PHASE 2: Aggregation with Validation (Ensemble voting)
phase2 := orchestration.NewEnsemble(
    "policy-aggregator",
    rt,
    []string{"aggregator-1", "aggregator-2", "aggregator-3"},
    orchestration.WithVotingStrategy(orchestration.VotingMajority),
    orchestration.WithAgreementThreshold(0.6), // VALIDATION GATE: 60% agreement
)
phase2Results, _ := phase2.Execute(ctx, phase1Results)

// PHASE 3: Sequential Risk Assessment (ordered execution)
riskAgent, _ := rt.Get("risk-analyzer")
riskAnalysis, _ := riskAgent.Execute(ctx, phase2Results)

recAgent, _ := rt.Get("recommendation-agent")
finalResult, _ := recAgent.Execute(ctx, riskAnalysis)
```

### Example 2: E-commerce Product Enrichment

```go
// PHASE 1: Parallel Feature Extraction
phase1 := orchestration.NewParallel(
    "feature-extraction",
    rt,
    []string{"spec-extractor", "review-analyzer", "competitor-analyzer"},
)
features, _ := phase1.Execute(ctx, productInput)

// PHASE 2: Merge and Validate
mergerAgent, _ := rt.Get("feature-merger")
mergedFeatures, _ := mergerAgent.Execute(ctx, features)

// PHASE 3: Sequential Description Generation
shortDescAgent, _ := rt.Get("short-desc-generator")
shortDesc, _ := shortDescAgent.Execute(ctx, mergedFeatures)

longDescAgent, _ := rt.Get("long-desc-generator")
longDesc, _ := longDescAgent.Execute(ctx, shortDesc) // Uses short as context
```

## When to Use Each Pattern

| Pattern | Use When | Example |
|---------|----------|---------|
| **Parallel** | Independent tasks, need speed | Multi-source data gathering |
| **Ensemble** | Validation/voting needed | Quality gates, critical decisions |
| **Sequential** | Task dependencies, order matters | Multi-step refinement |
| **Router** | Different inputs need different paths | Complexity-based model selection |

## Error Handling Strategies

### Fail-Fast (Strict)

```go
parallel := orchestration.NewParallel("strict", rt, agents,
    orchestration.WithFailFast(true)) // Stop on first error
```

### Partial Results (Resilient)

```go
parallel := orchestration.NewParallel("resilient", rt, agents,
    orchestration.WithFailFast(false)) // Continue with partial results
```

### Validation Gates

```go
ensemble := orchestration.NewEnsemble("validator", rt, agents,
    orchestration.WithAgreementThreshold(0.75)) // 75% agreement required
```

## Validation Between Phases

### 1. Ensemble Voting Thresholds

```go
ensemble := orchestration.NewEnsemble("validator", rt, agents,
    orchestration.WithVotingStrategy(orchestration.VotingMajority),
    orchestration.WithAgreementThreshold(0.6), // Must have 60% agreement
)
// If agreement < threshold, phase fails and workflow stops
```

### 2. Custom Validation Functions

```go
func validatePhase2(result *agent.Message) error {
    var data map[string]interface{}
    if err := json.Unmarshal([]byte(result.Payload), &data); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    if _, ok := data["required_field"]; !ok {
        return fmt.Errorf("missing required field")
    }
    return nil
}

phase2Result, _ := phase2.Execute(ctx, phase1Result)
if err := validatePhase2(phase2Result); err != nil {
    return fmt.Errorf("phase 2 validation failed: %w", err)
}
```

## Performance Characteristics

| Phase Type | Latency | Throughput | Cost | Quality Benefit |
|-----------|---------|------------|------|----------------|
| Parallel | Same as slowest agent | 3-4× improvement | All agents run | N/A |
| Ensemble | Same as slowest + voting | Similar to parallel | Multiple models | 25-50% error reduction |
| Sequential | Sum of all steps | Slowest step bottleneck | Cumulative | Progressive context building |

## Common Workflow Patterns

1. **Research Pipeline**: Parallel(gather) → Ensemble(validate) → Sequential(synthesize)
2. **Content Moderation**: Router(classify) → Parallel(multi-check) → Ensemble(vote) → Sequential(action)
3. **Financial Analysis**: Parallel(data) → Ensemble(consensus) → Sequential(report) → Router(action)
4. **Medical Diagnosis**: Parallel(symptoms) → Ensemble(diagnosis voting) → Sequential(treatment planning)

## Learn More

- [Parallel Research Example](../parallel-research/) - Deep dive on parallel pattern
- [Deterministic Aggregation Example](../deterministic-aggregation/) - Voting strategies
- [Router Cost Optimization Example](../router-cost-optimization/) - Dynamic routing
- [Orchestration Patterns Documentation](/docs/PATTERNS.md) - Full API reference
