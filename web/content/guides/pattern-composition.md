---
title: 'Pattern Composition for Multi-Phase Workflows'
description: 'Compose existing orchestration patterns to build complex multi-phase workflows without custom orchestrators'
breadcrumb: 'Pattern Composition'
category: 'Orchestration'
weight: 20
---

Aixgo provides 13 orchestration patterns that can be **composed** to build complex multi-phase workflows without writing custom orchestrators. This guide shows you how to combine existing patterns for sophisticated multi-stage processing.

## Overview

Instead of creating custom orchestration logic, compose existing patterns into multi-phase workflows:

```text
Phase 1 (Parallel) → Validate → Phase 2 (Aggregate) → Validate → Phase 3 (Sequential)
```

Each phase uses an existing pattern. Validation gates ensure quality between phases.

### The Composition Principle

**Don't create new patterns when you can compose existing ones.**

Aixgo's 13 patterns are building blocks. Most complex workflows are compositions of these patterns, not new patterns entirely.

### Benefits of Composition

- **Faster development**: Reuse proven patterns instead of building from scratch
- **Better reliability**: Each pattern is battle-tested in production
- **Easier maintenance**: Standard patterns are easier to understand and debug
- **Natural validation points**: Phase boundaries provide natural validation gates
- **Flexibility**: Reconfigure by swapping phases, not rewriting logic

## Composing Patterns Programmatically

### Example: Policy Analysis Workflow

This three-phase workflow demonstrates pattern composition:

1. **Phase 1 (Parallel)**: Extract data from multiple sources concurrently
2. **Validation Gate**: Ensure all required data extracted
3. **Phase 2 (Aggregation)**: Combine extracted data into coherent summary
4. **Validation Gate**: Verify aggregated output meets requirements
5. **Phase 3 (Sequential)**: Perform risk assessment on aggregated data

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/aixgo-dev/aixgo/pkg/agent"
    "github.com/aixgo-dev/aixgo/pkg/patterns"
    "github.com/aixgo-dev/aixgo/pkg/supervisor"
)

func runPolicyAnalysisWorkflow(ctx context.Context, policyDocument string) error {
    // Initialize executor
    executor := supervisor.NewExecutor()

    // Phase 1: Parallel data extraction
    phase1 := patterns.NewParallelPattern(executor, patterns.ParallelConfig{
        Timeout:  5 * time.Second,
        FailFast: false, // Collect all results even if some fail
    })

    phase1Result, err := phase1.Execute(ctx,
        []string{"data-agent", "summary-agent", "rights-agent"},
        agent.NewMessage(policyDocument))
    if err != nil {
        return fmt.Errorf("phase 1 failed: %w", err)
    }

    // Validation Gate 1: Ensure all data extracted
    if err := validatePhase1(phase1Result); err != nil {
        return fmt.Errorf("phase 1 validation failed: %w", err)
    }

    // Phase 2: Aggregation of extracted data
    aggregator := patterns.NewAggregationPattern(executor, patterns.AggregationConfig{
        Method:           patterns.AggregationConsensus,
        MinimumResponses: 2,
        Timeout:          10 * time.Second,
    })

    phase2Result, err := aggregator.Execute(ctx,
        []string{"policy-merger"},
        phase1Result.AggregatedOutput)
    if err != nil {
        return fmt.Errorf("phase 2 failed: %w", err)
    }

    // Validation Gate 2: Verify aggregated summary
    if err := validatePhase2(phase2Result); err != nil {
        return fmt.Errorf("phase 2 validation failed: %w", err)
    }

    // Phase 3: Sequential risk assessment
    phase3 := patterns.NewSequentialPattern(executor, patterns.SequentialConfig{})

    finalResult, err := phase3.Execute(ctx,
        []string{"risk-assessor", "compliance-checker"},
        phase2Result.AggregatedOutput)
    if err != nil {
        return fmt.Errorf("phase 3 failed: %w", err)
    }

    // Final validation
    if err := validateFinalResult(finalResult); err != nil {
        return fmt.Errorf("final validation failed: %w", err)
    }

    fmt.Printf("Workflow completed successfully: %v\n", finalResult)
    return nil
}

// Validation functions use the Validatable interface
type Phase1Output struct {
    DataPractices []string `json:"data_practices"`
    Summary       string   `json:"summary"`
    UserRights    []string `json:"user_rights"`
}

func (p Phase1Output) Validate() error {
    if len(p.DataPractices) == 0 {
        return fmt.Errorf("data practices required")
    }
    if p.Summary == "" {
        return fmt.Errorf("summary required")
    }
    if len(p.UserRights) == 0 {
        return fmt.Errorf("user rights required")
    }
    return nil
}

func validatePhase1(result *patterns.PatternResult) error {
    var output Phase1Output
    if err := result.Unmarshal(&output); err != nil {
        return err
    }
    return output.Validate()
}
```

## Common Composition Patterns

### 1. Extract-Transform-Load (ETL)

Classic data pipeline pattern using three sequential phases.

**Structure:**

```text
Parallel Extraction → Sequential Transform → Sequential Load
```

**Use when:**

- Processing large datasets with multiple sources
- Data transformations are ordered and dependent
- Loading requires specific sequencing

**YAML Configuration:**

```yaml
supervisor:
  name: etl-pipeline

agents:
  # Extraction Phase (Parallel)
  - name: source-1-extractor
    role: producer
    outputs:
      - target: transformer

  - name: source-2-extractor
    role: producer
    outputs:
      - target: transformer

  - name: source-3-extractor
    role: producer
    outputs:
      - target: transformer

  # Transform Phase (Sequential)
  - name: transformer
    role: react
    model: gpt-4-turbo
    prompt: 'Transform and normalize data'
    inputs:
      - source: source-1-extractor
      - source: source-2-extractor
      - source: source-3-extractor
    outputs:
      - target: validator

  - name: validator
    role: react
    model: gpt-4-turbo
    prompt: 'Validate transformed data'
    inputs:
      - source: transformer
    outputs:
      - target: loader

  # Load Phase (Sequential)
  - name: loader
    role: logger
    inputs:
      - source: validator
```

### 2. Multi-Expert Analysis

Combine parallel expert agents with consensus aggregation and sequential reporting.

**Structure:**

```text
Parallel Experts → Aggregation (Consensus) → Sequential Report
```

**Use when:**

- Need diverse expert perspectives
- Consensus building improves quality
- Final report requires structured formatting

**YAML Configuration:**

```yaml
supervisor:
  name: expert-analysis

agents:
  # Parallel Expert Phase
  - name: technical-expert
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze from technical perspective'
    outputs:
      - target: consensus-builder

  - name: business-expert
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze from business perspective'
    outputs:
      - target: consensus-builder

  - name: legal-expert
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze from legal perspective'
    outputs:
      - target: consensus-builder

  # Aggregation Phase
  - name: consensus-builder
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: technical-expert
      - source: business-expert
      - source: legal-expert
    outputs:
      - target: report-generator
    aggregator_config:
      aggregation_strategy: consensus
      consensus_threshold: 0.7

  # Sequential Reporting Phase
  - name: report-generator
    role: react
    model: gpt-4-turbo
    prompt: 'Format final report with executive summary'
    inputs:
      - source: consensus-builder
    outputs:
      - target: report-validator

  - name: report-validator
    role: react
    model: gpt-4-turbo
    prompt: 'Validate report completeness and accuracy'
    inputs:
      - source: report-generator
    outputs:
      - target: final-output
```

### 3. Iterative Refinement

Use reflection pattern within each phase for quality improvement.

**Structure:**

```text
Sequential Draft → Reflection (Critic + Generator) → Sequential Publish
```

**Use when:**

- Quality is paramount
- Iterative improvement adds significant value
- Time permits multiple refinement cycles

**YAML Configuration:**

```yaml
supervisor:
  name: iterative-refinement
  max_rounds: 10

agents:
  # Draft Phase
  - name: initial-drafter
    role: react
    model: gpt-4-turbo
    prompt: 'Create initial draft'
    outputs:
      - target: critic

  # Reflection Phase
  - name: critic
    role: react
    model: gpt-4-turbo
    prompt: 'Critique and identify improvements'
    inputs:
      - source: initial-drafter
      - source: refiner  # Feedback loop
    outputs:
      - target: refiner

  - name: refiner
    role: react
    model: gpt-4-turbo
    prompt: 'Improve based on critique'
    inputs:
      - source: critic
    outputs:
      - target: critic      # Continue reflection loop
      - target: publisher   # Exit when satisfied

  # Publish Phase
  - name: publisher
    role: logger
    inputs:
      - source: refiner
```

### 4. Hierarchical Processing

Multi-level delegation with aggregation at each level.

**Structure:**

```text
Parallel Teams → Team Aggregation → Hierarchical Summary
```

**Use when:**

- Large number of agents (10+)
- Natural organizational hierarchy
- Multi-level decision making needed

**YAML Configuration:**

```yaml
supervisor:
  name: hierarchical-processing

agents:
  # Team A (Parallel)
  - name: team-a-worker-1
    role: react
    model: gpt-4-turbo
    outputs:
      - target: team-a-lead

  - name: team-a-worker-2
    role: react
    model: gpt-4-turbo
    outputs:
      - target: team-a-lead

  # Team A Aggregation
  - name: team-a-lead
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: team-a-worker-1
      - source: team-a-worker-2
    outputs:
      - target: executive
    aggregator_config:
      aggregation_strategy: voting_majority

  # Team B (Parallel)
  - name: team-b-worker-1
    role: react
    model: gpt-4-turbo
    outputs:
      - target: team-b-lead

  - name: team-b-worker-2
    role: react
    model: gpt-4-turbo
    outputs:
      - target: team-b-lead

  # Team B Aggregation
  - name: team-b-lead
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: team-b-worker-1
      - source: team-b-worker-2
    outputs:
      - target: executive
    aggregator_config:
      aggregation_strategy: voting_majority

  # Executive Hierarchical Summary
  - name: executive
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: team-a-lead
      - source: team-b-lead
    outputs:
      - target: final-report
    aggregator_config:
      aggregation_strategy: hierarchical
```

## Validation Between Phases

Use the `Validatable` interface for type-safe phase validation.

### Phase Output Structs

Define clear contracts between phases:

```go
// Phase 1 Output: Parallel extraction
type ExtractionOutput struct {
    Sources []SourceData `json:"sources" validate:"required,dive"`
    Metadata ExtractionMetadata `json:"metadata" validate:"required"`
}

func (e ExtractionOutput) Validate() error {
    if len(e.Sources) == 0 {
        return fmt.Errorf("at least one source required")
    }
    if e.Metadata.Timestamp.IsZero() {
        return fmt.Errorf("metadata timestamp required")
    }
    return nil
}

// Phase 2 Output: Aggregation
type AggregationOutput struct {
    Summary string `json:"summary" validate:"required,min=100"`
    Confidence float64 `json:"confidence" validate:"required,gte=0,lte=1"`
}

func (a AggregationOutput) Validate() error {
    if a.Confidence < 0.7 {
        return fmt.Errorf("confidence too low: %.2f < 0.70", a.Confidence)
    }
    return nil
}

// Phase 3 Output: Risk assessment
type RiskAssessmentOutput struct {
    RiskLevel string `json:"risk_level" validate:"required,oneof=low medium high"`
    Factors []string `json:"factors" validate:"required"`
    Recommendations []string `json:"recommendations" validate:"required"`
}

func (r RiskAssessmentOutput) Validate() error {
    if len(r.Factors) == 0 {
        return fmt.Errorf("risk factors required")
    }
    if r.RiskLevel == "high" && len(r.Recommendations) == 0 {
        return fmt.Errorf("recommendations required for high risk")
    }
    return nil
}
```

### Validation Gates

Implement validation between each phase:

```go
func validateExtractionPhase(result *patterns.PatternResult) error {
    var output ExtractionOutput
    if err := result.Unmarshal(&output); err != nil {
        return fmt.Errorf("unmarshal failed: %w", err)
    }
    return output.Validate()
}

func validateAggregationPhase(result *patterns.PatternResult) error {
    var output AggregationOutput
    if err := result.Unmarshal(&output); err != nil {
        return fmt.Errorf("unmarshal failed: %w", err)
    }
    return output.Validate()
}

func validateRiskAssessmentPhase(result *patterns.PatternResult) error {
    var output RiskAssessmentOutput
    if err := result.Unmarshal(&output); err != nil {
        return fmt.Errorf("unmarshal failed: %w", err)
    }
    return output.Validate()
}
```

### Error Handling Between Phases

Handle phase failures gracefully:

```go
func executeWorkflow(ctx context.Context, input string) (*WorkflowResult, error) {
    // Phase 1
    phase1Result, err := executePhase1(ctx, input)
    if err != nil {
        return nil, &PhaseError{
            Phase: 1,
            Error: err,
            RecoveryHint: "Check data source availability",
        }
    }

    if err := validateExtractionPhase(phase1Result); err != nil {
        return nil, &ValidationError{
            Phase: 1,
            Error: err,
            Data: phase1Result,
        }
    }

    // Phase 2
    phase2Result, err := executePhase2(ctx, phase1Result)
    if err != nil {
        return nil, &PhaseError{
            Phase: 2,
            Error: err,
            RecoveryHint: "Consider relaxing consensus threshold",
        }
    }

    // Continue for remaining phases...
}

type PhaseError struct {
    Phase int
    Error error
    RecoveryHint string
}

func (e *PhaseError) Error() string {
    return fmt.Sprintf("phase %d failed: %v (hint: %s)",
        e.Phase, e.Error, e.RecoveryHint)
}

type ValidationError struct {
    Phase int
    Error error
    Data any
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("phase %d validation failed: %v",
        e.Phase, e.Error)
}
```

## When to Compose vs. Create New Pattern

### Compose Existing Patterns When

- Workflow is sequential phases
- Each phase uses existing pattern logic
- Validation/gates are simple checks
- Patterns can be independently configured
- Standard orchestration patterns apply

**Example:** ETL pipeline, multi-expert analysis, hierarchical review

### Create New Pattern When

- Fundamentally new execution model needed
- Custom state management across agents required
- Novel coordination logic not captured by existing patterns
- Pattern will be reused across many projects
- Existing composition would be overly complex

**Example:** Custom negotiation protocol, domain-specific coordination

### Decision Matrix

| Scenario | Compose | New Pattern |
|----------|---------|-------------|
| 3-phase workflow with standard patterns | ✅ | ❌ |
| Multi-level aggregation | ✅ | ❌ |
| Custom agent negotiation protocol | ❌ | ✅ |
| Sequential + Parallel + Aggregation | ✅ | ❌ |
| Novel state machine coordination | ❌ | ✅ |
| ETL with validation gates | ✅ | ❌ |
| Custom distributed consensus algorithm | ❌ | ✅ |

## Performance Optimization

### Minimize Phase Transitions

Each phase transition adds overhead. Combine phases when possible:

**Before (3 phases):**

```text
Extract → Validate → Transform → Validate → Load
```

**After (2 phases):**

```text
Extract + Validate → Transform + Validate + Load
```

### Parallel Phase Execution

When phases are independent, run them in parallel:

```go
// Run independent phases concurrently
var wg sync.WaitGroup
var phase1Result, phase2Result *patterns.PatternResult
var phase1Err, phase2Err error

wg.Add(2)

go func() {
    defer wg.Done()
    phase1Result, phase1Err = executePhase1(ctx, input)
}()

go func() {
    defer wg.Done()
    phase2Result, phase2Err = executePhase2(ctx, input)
}()

wg.Wait()

if phase1Err != nil || phase2Err != nil {
    return handleErrors(phase1Err, phase2Err)
}
```

### Cache Phase Results

For repeated workflows, cache phase results:

```go
type PhaseCache struct {
    cache map[string]*patterns.PatternResult
    mu    sync.RWMutex
}

func (c *PhaseCache) Get(key string) (*patterns.PatternResult, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    result, ok := c.cache[key]
    return result, ok
}

func (c *PhaseCache) Set(key string, result *patterns.PatternResult) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[key] = result
}

// Usage
cacheKey := hashInput(input)
if cached, ok := phaseCache.Get(cacheKey); ok {
    return cached, nil
}

result, err := executePhase1(ctx, input)
if err == nil {
    phaseCache.Set(cacheKey, result)
}
```

## Real-World Example: Document Processing Pipeline

Complete example combining multiple patterns for document processing.

### Requirements

1. Extract text from multiple document formats (PDF, Word, HTML)
2. Classify document type and sensitivity
3. Parallel analysis: summarization, entity extraction, sentiment
4. Aggregate analysis results
5. Risk assessment and compliance check
6. Generate final report

### Implementation

```go
func processDocument(ctx context.Context, doc Document) (*Report, error) {
    executor := supervisor.NewExecutor()

    // Phase 1: Parallel text extraction
    extractors := []string{
        "pdf-extractor",
        "word-extractor",
        "html-extractor",
    }

    phase1 := patterns.NewParallelPattern(executor, patterns.ParallelConfig{
        Timeout: 10 * time.Second,
        FailFast: false,
    })

    extractionResult, err := phase1.Execute(ctx, extractors,
        agent.NewMessage(doc))
    if err != nil {
        return nil, fmt.Errorf("extraction failed: %w", err)
    }

    // Phase 2: Classification (Router pattern)
    classifier := patterns.NewRouterPattern(executor, patterns.RouterConfig{
        RoutingStrategy: patterns.ClassificationBased,
    })

    classificationResult, err := classifier.Execute(ctx,
        []string{"document-classifier"},
        extractionResult.AggregatedOutput)
    if err != nil {
        return nil, fmt.Errorf("classification failed: %w", err)
    }

    // Phase 3: Parallel analysis
    analyzers := []string{
        "summarizer",
        "entity-extractor",
        "sentiment-analyzer",
    }

    phase3 := patterns.NewParallelPattern(executor, patterns.ParallelConfig{
        Timeout: 15 * time.Second,
    })

    analysisResult, err := phase3.Execute(ctx, analyzers,
        classificationResult.AggregatedOutput)
    if err != nil {
        return nil, fmt.Errorf("analysis failed: %w", err)
    }

    // Phase 4: Aggregation
    aggregator := patterns.NewAggregationPattern(executor, patterns.AggregationConfig{
        Method: patterns.AggregationConsensus,
        MinimumResponses: 2,
    })

    aggregatedResult, err := aggregator.Execute(ctx,
        []string{"analysis-aggregator"},
        analysisResult.AggregatedOutput)
    if err != nil {
        return nil, fmt.Errorf("aggregation failed: %w", err)
    }

    // Phase 5: Sequential risk and compliance
    riskCheckers := []string{
        "risk-assessor",
        "compliance-checker",
    }

    phase5 := patterns.NewSequentialPattern(executor, patterns.SequentialConfig{})

    riskResult, err := phase5.Execute(ctx, riskCheckers,
        aggregatedResult.AggregatedOutput)
    if err != nil {
        return nil, fmt.Errorf("risk assessment failed: %w", err)
    }

    // Phase 6: Report generation
    reportGenerator := patterns.NewSequentialPattern(executor, patterns.SequentialConfig{})

    finalResult, err := reportGenerator.Execute(ctx,
        []string{"report-generator"},
        riskResult.AggregatedOutput)
    if err != nil {
        return nil, fmt.Errorf("report generation failed: %w", err)
    }

    var report Report
    if err := finalResult.Unmarshal(&report); err != nil {
        return nil, fmt.Errorf("unmarshal report failed: %w", err)
    }

    return &report, nil
}
```

## Best Practices

1. **Define clear phase boundaries**: Each phase should have well-defined inputs and outputs
2. **Validate between phases**: Use `Validatable` interface for type-safe validation
3. **Handle failures gracefully**: Provide recovery hints and context in errors
4. **Cache when appropriate**: Reuse expensive phase results for identical inputs
5. **Monitor phase performance**: Track latency and success rates per phase
6. **Document phase contracts**: Clear documentation of expected inputs/outputs
7. **Use appropriate patterns**: Match pattern to phase requirements (parallel vs sequential)
8. **Consider cost**: LLM-powered patterns vs deterministic patterns based on needs

## See Also

- [Multi-Agent Orchestration Guide](./multi-agent-orchestration/) - All 13 orchestration patterns
- [Sequential Pattern](./multi-agent-orchestration/#sequential-pattern) - Ordered execution
- [Parallel Pattern](./multi-agent-orchestration/#parallel-pattern) - Concurrent execution
- [Aggregation Pattern](./multi-agent-orchestration/#aggregation-pattern) - Multi-source synthesis
- [Validation with Retry](./validation-with-retry/) - Automatic validation and retry
- [Multi-Phase Workflow Example](../../examples/multi-phase-workflow/) - Complete working example
