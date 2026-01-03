---
title: 'Agent Types Guide'
description: "Comprehensive guide to all Aixgo agent types including Classifier and Aggregator agents with examples and best practices."
breadcrumb: 'Guides'
category: 'Agents'
weight: 3
---

Aixgo provides specialized agent types for building production-grade multi-agent systems. This guide covers all available agent types, when to use each, and how to configure them for optimal performance.

## Overview

Aixgo offers six core agent types, each designed for specific roles in your multi-agent architecture:

- **Producer**: Generate periodic messages for downstream processing
- **ReAct**: LLM-powered reasoning and tool execution
- **Logger**: Message consumption and persistence
- **Classifier**: Intelligent content classification with confidence scoring
- **Aggregator**: Multi-agent output synthesis and consensus building
- **Planner**: Task decomposition and workflow orchestration

## Producer Agent

Producer agents generate messages at configured intervals, providing the data input for your agent workflows.

### When to Use

- Polling external APIs or data sources
- Generating synthetic test data
- Periodic health checks or monitoring
- Time-based event triggers
- ETL pipeline data ingestion

### Configuration

```yaml
agents:
  - name: event-generator
    role: producer
    interval: 500ms
    outputs:
      - target: processor
```

### Best Practices

- Set appropriate intervals based on your data source refresh rate
- Use exponential backoff for failed polling attempts
- Consider rate limits when polling external APIs
- Implement circuit breakers for unreliable sources

**Learn more**: [Producer examples](https://github.com/aixgo-dev/aixgo/tree/main/examples/producer-workflow)

## ReAct Agent

ReAct (Reasoning + Acting) agents combine LLM reasoning with tool execution capabilities for complex decision-making workflows.

### When to Use

- Data analysis requiring intelligent reasoning
- Decision-making workflows with business logic
- Natural language processing tasks
- Complex multi-step operations
- Tool-assisted problem solving

### Configuration

```yaml
agents:
  - name: analyst
    role: react
    model: gpt-4-turbo
    prompt: 'You are an expert data analyst.'
    tools:
      - name: query_database
        description: 'Query the database'
        input_schema:
          type: object
          properties:
            query: { type: string }
          required: [query]
    inputs:
      - source: event-generator
    outputs:
      - target: logger
```

### Best Practices

- Provide clear, specific system prompts
- Define precise tool schemas with validation
- Use appropriate temperature settings (0.2-0.4 for deterministic, 0.7-1.0 for creative)
- Implement timeout handling for long-running operations
- Monitor token usage and optimize prompts

**Learn more**: [ReAct examples](https://github.com/aixgo-dev/aixgo/tree/main/examples/react-workflow)

## Logger Agent

Logger agents consume and persist messages, providing observability and audit capabilities for your workflows.

### When to Use

- Audit logging and compliance
- Debugging multi-agent workflows
- Data persistence and archival
- Monitoring and alerting
- Performance metric collection

### Configuration

```yaml
agents:
  - name: audit-log
    role: logger
    inputs:
      - source: analyst
```

### Best Practices

- Use structured logging formats (JSON)
- Implement log rotation and retention policies
- Set up log aggregation for distributed systems
- Create alerts for error patterns

**Learn more**: [Logger examples](https://github.com/aixgo-dev/aixgo/tree/main/examples/logger-workflow)

## Classifier Agent

Classifier agents use LLM-powered semantic understanding to categorize content with confidence scoring, few-shot learning, and structured outputs.

### When to Use

- Customer support ticket routing and prioritization
- Content moderation and categorization
- Document classification and tagging
- Intent detection in conversational AI
- Sentiment analysis with custom categories
- Multi-label content tagging

### Key Features

- **Structured JSON Outputs**: Schema-validated responses for reliable parsing
- **Confidence Scoring**: Automatic quality assessment (0-1 scale)
- **Few-Shot Learning**: Improve accuracy with example-based training
- **Multi-Label Support**: Assign multiple categories simultaneously
- **Alternative Classifications**: Secondary suggestions for low-confidence results
- **Semantic Understanding**: Context-aware classification beyond keywords

### Configuration

```yaml
agents:
  - name: ticket-classifier
    role: classifier
    model: gpt-4-turbo
    inputs:
      - source: support-tickets
    outputs:
      - target: classified-tickets
    classifier_config:
      categories:
        - name: technical_issue
          description: "Issues requiring technical troubleshooting or product support"
          keywords: ["error", "bug", "not working", "crash"]
          examples:
            - "The app crashes when I click submit"
            - "Error code 500 appears on checkout"

        - name: billing_inquiry
          description: "Questions about payments, invoices, or pricing"
          keywords: ["payment", "invoice", "charge", "refund"]
          examples:
            - "I was charged twice this month"
            - "Can I get a refund?"

      # Minimum confidence for automatic classification
      confidence_threshold: 0.7

      # Allow multiple categories per input
      multi_label: false

      # Few-shot examples for improved accuracy
      few_shot_examples:
        - input: "My account won't let me log in"
          category: technical_issue
          reason: "Authentication system issue"

      # LLM parameters
      temperature: 0.3      # Low for consistent classification
      max_tokens: 500       # Sufficient for reasoning
```

### Category Definition Best Practices

Each category should include:

- **name**: Unique identifier (use snake_case)
- **description**: Clear explanation of category boundaries
- **keywords**: Terms strongly associated with this category
- **examples**: 2-3 representative samples

### Confidence Threshold Guidelines

- **0.5-0.6**: Exploratory use, may have incorrect classifications
- **0.7-0.8**: Production baseline, good accuracy/coverage balance
- **0.85+**: High-stakes scenarios, may reject ambiguous inputs

### Example Output

```json
{
  "category": "technical_issue",
  "confidence": 0.92,
  "reasoning": "User describes specific product issue requiring technical assistance",
  "alternatives": [
    {"category": "billing_inquiry", "confidence": 0.15}
  ],
  "tokens_used": 234
}
```

**Learn more**:
- [Classifier agent documentation](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/agents/README.md)
- [Classifier workflow example](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/examples/classifier-workflow/README.md)

## Aggregator Agent

Aggregator agents synthesize outputs from multiple agents using 9 intelligent strategies, from zero-cost deterministic voting to sophisticated LLM-powered consensus building.

### When to Use

- Multi-agent research synthesis
- Combining outputs from specialized expert agents
- Consensus building in distributed AI systems
- Ensemble learning for improved accuracy
- Cross-validation of agent outputs
- RAG systems with multiple retrievers
- Conflict resolution between diverse perspectives
- Production systems requiring deterministic, reproducible results

### Key Features

- **9 Aggregation Strategies**: 5 LLM-powered + 4 deterministic voting methods
- **Resilience by Default**: Handles partial failures, missing inputs, timeouts
- **Zero-Cost Options**: Deterministic voting strategies with no LLM calls
- **Conflict Resolution**: Automatic detection and LLM-mediated resolution
- **Semantic Clustering**: Group similar outputs using text similarity
- **Consensus Scoring**: Quantify agreement levels (0-1 scale)
- **Performance Tracking**: Built-in observability and metrics

### All 9 Aggregation Strategies

#### LLM-Powered Strategies

These strategies use LLM reasoning for sophisticated synthesis. They provide high-quality results but incur API costs and latency.

**1. consensus** - Find common ground among diverse opinions

- **Use when**: Need balanced synthesis with conflict transparency
- **Cost**: $$ (LLM calls for analysis)
- **Speed**: Slow (2-5s depending on agent count)
- **Reproducibility**: Low (LLM output varies)
- **Best for**: Fact verification, balanced synthesis, transparent disagreement handling

```yaml
aggregator_config:
  aggregation_strategy: consensus
  consensus_threshold: 0.7
  conflict_resolution: llm_mediated
```

**2. weighted** - Prioritize high-authority sources

- **Use when**: Some agents have more expertise or reliability
- **Cost**: $$ (LLM calls for synthesis)
- **Speed**: Slow (2-5s)
- **Reproducibility**: Low (LLM output varies)
- **Best for**: Expert prioritization, confidence-based mixing, known reliability differences

```yaml
aggregator_config:
  aggregation_strategy: weighted
  source_weights:
    expert_agent: 1.0
    general_agent_1: 0.6
    general_agent_2: 0.4
```

**3. semantic** - Group similar outputs by theme

- **Use when**: Many agents with overlapping insights
- **Cost**: $$$ (LLM + embedding calls)
- **Speed**: Slow (3-7s with embedding overhead)
- **Reproducibility**: Medium (embeddings are deterministic, synthesis is not)
- **Best for**: Large agent counts (5+), deduplication, perspective identification

```yaml
aggregator_config:
  aggregation_strategy: semantic
  semantic_similarity_threshold: 0.85
  deduplication_method: semantic
```

**4. hierarchical** - Multi-level summarization

- **Use when**: 10+ agents, need efficient aggregation
- **Cost**: $$$$ (multiple LLM calls for hierarchical processing)
- **Speed**: Slowest (5-15s for multi-level aggregation)
- **Reproducibility**: Low (multiple LLM calls compound variance)
- **Best for**: Large agent counts (10+), token efficiency, structured summarization

```yaml
aggregator_config:
  aggregation_strategy: hierarchical
  max_input_sources: 20
  summarization_enabled: true
```

**5. rag_based** - Citation-based aggregation

- **Use when**: Need source attribution and traceability
- **Cost**: $$ (LLM calls for generation)
- **Speed**: Slow (2-5s)
- **Reproducibility**: Low (LLM output varies)
- **Best for**: Question answering, multi-source research, citation preservation

```yaml
aggregator_config:
  aggregation_strategy: rag_based
  max_input_sources: 10
```

#### Deterministic Strategies (v0.1.3+)

These strategies provide instant, reproducible results with zero LLM costs. Perfect for production systems requiring deterministic behavior.

**6. voting_majority** - Simple majority vote

- **Use when**: Democratic decision, equal weight agents
- **Cost**: $0 (no LLM calls)
- **Speed**: Instant (<1ms)
- **Reproducibility**: 100% (same inputs always produce same output)
- **Best for**: Classification tasks, binary decisions, equal-expertise agents

```yaml
aggregator_config:
  aggregation_strategy: voting_majority
  # No LLM configuration needed
```

**7. voting_unanimous** - Requires all to agree

- **Use when**: Safety-critical decisions, consensus required
- **Cost**: $0 (no LLM calls)
- **Speed**: Instant (<1ms)
- **Reproducibility**: 100%
- **Best for**: High-stakes decisions, regulatory compliance, safety validation

```yaml
aggregator_config:
  aggregation_strategy: voting_unanimous
  # Fails unless all agents agree
```

**8. voting_weighted** - Confidence-weighted voting

- **Use when**: Expert panels with varying confidence levels
- **Cost**: $0 (no LLM calls)
- **Speed**: Instant (<1ms)
- **Reproducibility**: 100%
- **Best for**: Expert systems with confidence scoring, hierarchical decision making

```yaml
aggregator_config:
  aggregation_strategy: voting_weighted
  source_weights:
    expert_1: 0.9
    expert_2: 0.7
    expert_3: 0.5
```

**9. voting_confidence** - Highest confidence wins

- **Use when**: Defer to most confident expert
- **Cost**: $0 (no LLM calls)
- **Speed**: Instant (<1ms)
- **Reproducibility**: 100%
- **Best for**: Expert selection, competitive agent systems, confidence-based routing

```yaml
aggregator_config:
  aggregation_strategy: voting_confidence
  # Selects result from agent with highest confidence score
```

### Resilience Features

The aggregator is **resilient by default**, designed to handle real-world failures gracefully:

- **Handles missing inputs**: Processes available results even if some agents fail or timeout
- **Timeout-based collection**: Waits specified time for inputs, then proceeds with what's available
- **Minimum response requirements**: Can specify minimum agents needed before aggregating
- **Confidence scoring**: Weights results by confidence levels when available
- **Partial result support**: Aggregates whatever is available, doesn't require all sources

### When to Use Each Strategy

**Use deterministic (voting_*)** when:

- Reproducibility required (testing, debugging, compliance)
- Cost optimization critical ($0 vs $$ per aggregation)
- Speed essential (instant vs seconds)
- Simple aggregation sufficient (voting, selection)
- Auditing requires deterministic behavior

**Use LLM-powered** when:

- Semantic synthesis needed (narrative combining diverse viewpoints)
- Conflict resolution requires reasoning
- Narrative output preferred over structured votes
- Complex cross-agent analysis required
- Quality justifies cost and latency

### Strategy Selection Decision Tree

```text
Need reproducibility (same input → same output)?
├─ YES → Use deterministic strategies (voting_*)
│   ├─ All must agree? → voting_unanimous
│   ├─ Different expertise levels? → voting_weighted
│   ├─ Defer to most confident? → voting_confidence
│   └─ Equal weight voting? → voting_majority
│
└─ NO → Use LLM-powered strategies
    ├─ 10+ agents? → hierarchical
    ├─ Need citations? → rag_based
    ├─ Many similar outputs? → semantic
    ├─ Different expertise? → weighted
    └─ Balanced synthesis? → consensus
```

### Example: Resilient Aggregation

```yaml
agents:
  - name: policy-aggregator
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: legal-expert
      - source: technical-expert
      - source: business-expert
      - source: compliance-expert
      - source: security-expert
    outputs:
      - target: final-report
    aggregator_config:
      # Deterministic voting for reproducibility
      aggregation_strategy: voting_majority

      # Resilience settings
      timeout_ms: 5000          # Wait 5s for inputs
      max_input_sources: 5      # Expect up to 5 agents
      min_input_sources: 3      # Require at least 3 responses

      # Even if only 3 of 5 respond in time, aggregation proceeds
      # If fewer than 3 respond, aggregation fails gracefully
```

**Behavior:**

- If all 5 experts respond within 5s: Aggregate all inputs
- If 3-4 respond: Proceed with available inputs (meets minimum)
- If <3 respond: Fail with clear error message
- If some agents fail: Continue with successful responses

See [resilient-aggregation example](../../examples/resilient-aggregation/) for complete implementation.

### Full Configuration Example

```yaml
agents:
  - name: research-synthesizer
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: expert-1
      - source: expert-2
      - source: expert-3
    outputs:
      - target: final-report
    aggregator_config:
      # Strategy selection
      aggregation_strategy: consensus

      # Conflict handling
      conflict_resolution: llm_mediated

      # Deduplication
      deduplication_method: semantic

      # Enable summarization
      summarization_enabled: true

      # Maximum agents to aggregate
      max_input_sources: 10

      # Timeout for collecting inputs (ms)
      timeout_ms: 5000

      # Semantic clustering threshold
      semantic_similarity_threshold: 0.85

      # Source weights (for weighted strategy)
      source_weights:
        expert-1: 1.0
        expert-2: 0.7
        expert-3: 0.5

      # Consensus threshold
      consensus_threshold: 0.7

      # LLM parameters
      temperature: 0.5
      max_tokens: 1500
```

### Example Output

```json
{
  "strategy": "consensus",
  "consensus_level": 0.87,
  "aggregated_content": "After analyzing all expert inputs, the following synthesis emerges...",
  "conflicts_resolved": [
    {
      "topic": "implementation_approach",
      "conflicting_sources": ["expert-1", "expert-2"],
      "resolution": "Hybrid approach combining both perspectives",
      "reasoning": "Expert 1's architectural concerns addressed by Expert 2's practical constraints"
    }
  ],
  "semantic_clusters": [
    {
      "cluster_id": "cluster_0",
      "members": ["expert-1", "expert-3"],
      "core_concept": "technical_implementation",
      "avg_similarity": 0.89
    }
  ],
  "tokens_used": 1250
}
```

### Best Practices

#### Strategy Selection

- **Consensus**: Use when you need balanced synthesis with conflict transparency
- **Weighted**: Use when certain agents have more expertise or authority
- **Semantic**: Use for deduplication and thematic organization (5+ agents)
- **Hierarchical**: Use for scalability with many agents (10+)
- **RAG-based**: Use for question answering with source attribution

#### Timeout Configuration

Set based on expected agent response times:

- Fast agents (1-2s): `timeout_ms: 3000`
- Standard agents (3-5s): `timeout_ms: 5000`
- Complex agents (5-10s): `timeout_ms: 10000`

#### Token Management

Typical token usage:

- 2-3 agents: 500-1000 tokens
- 4-6 agents: 1000-1500 tokens
- 7-10 agents: 1500-2500 tokens
- 10+ agents: Use hierarchical strategy

**Learn more**:
- [Aggregator agent documentation](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/agents/README.md)
- [Aggregator workflow example](/Users/charlesgreen/go/src/github.com/aixgo-dev/aixgo/examples/aggregator-workflow/README.md)

## Planner Agent

Planner agents decompose complex tasks into executable steps and orchestrate their execution across multiple agents.

### When to Use

- Complex multi-step workflows requiring coordination
- Dynamic task decomposition based on context
- Adaptive workflow execution
- Resource allocation and scheduling
- Goal-oriented planning with dependencies

### Configuration

```yaml
agents:
  - name: task-planner
    role: planner
    model: gpt-4-turbo
    prompt: 'You are a task planning expert'
    inputs:
      - source: user-requests
    outputs:
      - target: execution-queue
```

**Learn more**: [Planner examples](https://github.com/aixgo-dev/aixgo/tree/main/examples/planner-workflow)

## Integration Patterns

### Parallel Classification + Aggregation

Combine multiple classifiers with an aggregator for comprehensive analysis:

```yaml
agents:
  # Input producer
  - name: content-source
    role: producer
    outputs:
      - target: content

  # Parallel classifiers
  - name: sentiment-classifier
    role: classifier
    inputs:
      - source: content
    outputs:
      - target: classifications
    classifier_config:
      categories:
        - name: positive
          description: "Positive sentiment"
        - name: negative
          description: "Negative sentiment"

  - name: topic-classifier
    role: classifier
    inputs:
      - source: content
    outputs:
      - target: classifications
    classifier_config:
      categories:
        - name: technology
          description: "Technology-related content"
        - name: business
          description: "Business-related content"

  # Aggregator combines classifications
  - name: final-classifier
    role: aggregator
    inputs:
      - source: classifications
    outputs:
      - target: final-output
    aggregator_config:
      aggregation_strategy: consensus
```

### Multi-Expert Research Pipeline

Deploy specialized experts with weighted aggregation:

```yaml
agents:
  # Expert agents
  - name: technical-expert
    role: react
    model: gpt-4-turbo
    prompt: "You are a technical architecture expert"
    outputs:
      - target: expert-analyses

  - name: security-expert
    role: react
    model: gpt-4-turbo
    prompt: "You are a security expert"
    outputs:
      - target: expert-analyses

  - name: business-expert
    role: react
    model: gpt-4-turbo
    prompt: "You are a business analyst"
    outputs:
      - target: expert-analyses

  # Weighted aggregator
  - name: research-synthesis
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: expert-analyses
    outputs:
      - target: final-report
    aggregator_config:
      aggregation_strategy: weighted
      source_weights:
        technical-expert: 0.9
        security-expert: 0.95
        business-expert: 0.7
```

## Performance Considerations

### Token Usage Optimization

- **Producer**: No LLM calls, zero token usage
- **ReAct**: 200-2000 tokens per message (depends on complexity)
- **Logger**: No LLM calls, zero token usage
- **Classifier**: 200-500 tokens per classification (add 150-300 for few-shot)
- **Aggregator**: 500-2500 tokens (scales with agent count)
- **Planner**: 300-1000 tokens per planning operation

### Latency Guidelines

- **Producer**: <10ms (local generation)
- **ReAct**: 500ms-5s (LLM-dependent)
- **Logger**: <50ms (I/O-dependent)
- **Classifier**: 500ms-2s (LLM-dependent)
- **Aggregator**: 1s-5s (scales with agent count)
- **Planner**: 1s-3s (LLM-dependent)

### Cost Management

Choose appropriate models for your use case:

```yaml
# Production traffic - balance cost and quality
model: gpt-4o-mini

# Critical decisions - maximum accuracy
model: gpt-4-turbo

# High volume, simple tasks - lowest cost
model: gpt-3.5-turbo
```

## Next Steps

- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Learn orchestration patterns
- **[Classifier Example](/examples/classifier-workflow)** - Hands-on classifier implementation
- **[Aggregator Example](/examples/aggregator-workflow)** - Multi-agent synthesis walkthrough
- **[Provider Integration](/guides/provider-integration)** - Configure LLM providers
- **[Production Deployment](/guides/production-deployment)** - Deploy agent systems

## Additional Resources

- [Agent Framework Source Code](https://github.com/aixgo-dev/aixgo/tree/main/agents)
- [Complete Examples Directory](https://github.com/aixgo-dev/aixgo/tree/main/examples)
- [API Documentation](https://pkg.go.dev/github.com/aixgo-dev/aixgo)
