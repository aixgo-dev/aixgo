# Agent Types Documentation

This document provides comprehensive documentation for the Classifier and Aggregator agent types in aixgo.
These production-ready agents leverage LLM capabilities for intelligent content classification and
multi-agent output aggregation.

## Table of Contents

- [Overview](#overview)
- [Classifier Agent](#classifier-agent)
  - [What It Does](#what-it-does)
  - [Key Features](#key-features)
  - [Configuration](#configuration)
  - [Usage Patterns](#usage-patterns)
  - [Best Practices](#best-practices)
- [Aggregator Agent](#aggregator-agent)
  - [Purpose and Use Cases](#purpose-and-use-cases)
  - [Core Features](#core-features)
  - [Aggregation Strategies](#aggregation-strategies)
  - [Aggregator Configuration](#aggregator-configuration)
  - [Common Usage Patterns](#common-usage-patterns)
  - [Aggregator Best Practices](#aggregator-best-practices)
- [Examples](#examples)
- [Integration Guide](#integration-guide)
- [Performance Considerations](#performance-considerations)

## Overview

The aixgo framework provides specialized agent types for common AI-powered tasks:

- **Classifier Agent**: LLM-based content classification with structured outputs, confidence scoring,
  and semantic understanding
- **Aggregator Agent**: Multi-agent output synthesis using consensus, weighted, semantic, hierarchical,
  and RAG-based strategies

Both agents are built on the aixgo agent runtime and support multiple LLM providers
(OpenAI, Anthropic, xAI, Vertex AI, HuggingFace).

## Classifier Agent

### What It Does

The Classifier agent analyzes input text and assigns it to predefined categories using LLM-powered
semantic understanding. Unlike traditional rule-based classifiers, it understands context, nuance,
and handles edge cases intelligently.

**Use Cases:**

- Content moderation and categorization
- Customer support ticket routing
- Document classification
- Intent detection in conversational AI
- Sentiment analysis with custom categories
- Multi-label tagging systems

### Key Features

#### 1. Structured JSON Outputs

The classifier uses JSON schema validation to ensure consistent, parseable results:

```json
{
  "category": "technical_support",
  "confidence": 0.92,
  "reasoning": "User describes a specific product issue requiring technical assistance",
  "alternatives": [
    {"category": "billing", "confidence": 0.15}
  ],
  "tokens_used": 234,
  "prompt_strategy": "few-shot"
}
```

#### 2. Few-Shot Learning

Provide examples to improve accuracy without fine-tuning:

```yaml
few_shot_examples:
  - input: "My password isn't working"
    category: "account_access"
    reason: "User experiencing login credential issues"
  - input: "What are your business hours?"
    category: "general_inquiry"
    reason: "Request for general company information"
```

#### 3. Confidence Scoring

Automatically rejects low-confidence classifications:

```yaml
confidence_threshold: 0.7  # Only accept classifications above 70% confidence
```

#### 4. Multi-Label Support

Enable classification into multiple categories simultaneously:

```yaml
multi_label: true  # Allow content to belong to multiple categories
```

#### 5. Semantic Similarity (Optional)

Use embeddings for faster category matching when `use_embeddings: true`.

### Configuration

Full classifier configuration structure:

```yaml
agents:
  - name: content_classifier
    type: classifier
    model: gpt-4
    inputs:
      - source: input_queue
    outputs:
      - target: classified_output

    classifier_config:
      # Define categories with rich metadata
      categories:
        - name: technical_support
          description: "Issues requiring technical troubleshooting or product support"
          keywords: ["error", "bug", "not working", "crash", "issue"]
          examples:
            - "The app crashes when I click submit"
            - "Error code 500 appears on checkout"

        - name: billing_inquiry
          description: "Questions about payments, invoices, or pricing"
          keywords: ["payment", "invoice", "charge", "refund", "price"]
          examples:
            - "I was charged twice this month"
            - "Can I get a refund for my subscription?"

      # Enable embeddings for semantic matching
      use_embeddings: false

      # Minimum confidence to accept classification
      confidence_threshold: 0.7

      # Allow multiple categories per input
      multi_label: false

      # Few-shot examples for better accuracy
      few_shot_examples:
        - input: "My account won't let me log in"
          category: technical_support
          reason: "Authentication system issue"

      # LLM parameters
      temperature: 0.3      # Low temperature for consistent classification
      max_tokens: 500       # Sufficient for reasoning
```

#### Category Definition Best Practices

Each category should include:

- **name**: Unique identifier (snake_case recommended)
- **description**: Clear, specific explanation of what belongs in this category
- **keywords**: Terms strongly associated with this category
- **examples**: 2-3 representative examples

### Usage Patterns

#### Basic Classification Pipeline

```yaml
agents:
  # Producer generates content to classify
  - name: content_producer
    type: producer
    outputs:
      - target: unclassified_content

  # Classifier categorizes content
  - name: classifier
    type: classifier
    model: gpt-4
    inputs:
      - source: unclassified_content
    outputs:
      - target: classified_content
    classifier_config:
      categories:
        - name: urgent
          description: "Requires immediate attention"
        - name: normal
          description: "Standard priority"
      confidence_threshold: 0.75

  # Logger outputs results
  - name: logger
    type: logger
    inputs:
      - source: classified_content
```

#### Multi-Label Classification

```yaml
classifier_config:
  multi_label: true
  categories:
    - name: technical
      description: "Contains technical content"
    - name: urgent
      description: "Requires urgent action"
    - name: customer_facing
      description: "Should be visible to customers"
  confidence_threshold: 0.6  # Lower threshold for multi-label
```

#### High-Accuracy Classification with Few-Shot

```yaml
classifier_config:
  temperature: 0.2  # Very low for maximum consistency
  few_shot_examples:
    # Provide 3-5 high-quality examples per category
    - input: "Example input text"
      category: category_name
      reason: "Why this belongs in this category"
  confidence_threshold: 0.85  # Higher threshold when using few-shot
```

### Best Practices

#### 1. Category Design

**Do:**

- Create clear, mutually exclusive categories (unless using multi-label)
- Provide detailed descriptions explaining boundaries between similar categories
- Include 3-5 diverse keywords per category
- Add 2-3 representative examples

**Don't:**

- Create overlapping categories without enabling multi-label
- Use vague category names or descriptions
- Create more than 10-15 categories (split into multiple classifiers if needed)

#### 2. Prompt Engineering

The classifier uses a Chain-of-Thought prompting strategy:

1. Presents categories with descriptions and examples
2. Shows few-shot examples if provided
3. Asks LLM to think step-by-step
4. Requires structured JSON output

**Optimize by:**

- Writing clear, specific category descriptions
- Providing diverse few-shot examples
- Using keywords that capture semantic meaning, not just literal matches

#### 3. Confidence Tuning

- **0.5-0.6**: Exploratory use, expect some incorrect classifications
- **0.7-0.8**: Production baseline, good balance of coverage and accuracy
- **0.85+**: High-stakes scenarios, may reject valid but ambiguous inputs

Monitor the `performanceData` metrics to tune your threshold:

```go
// Logged automatically every 100 classifications
"Classifier Performance Insights: Avg Tokens: 245, Avg Latency: 850ms,
 Success Rate: 87.50%, Avg Confidence: 0.82"
```

#### 4. Token Optimization

- Use concise category descriptions (1-2 sentences)
- Limit few-shot examples to 3 per category
- Set `max_tokens: 500` for classification tasks
- Enable `use_embeddings: true` for 100+ categories (when implemented)

#### 5. Input Validation

The classifier automatically validates inputs:

- Maximum length: 100,000 characters
- Rejects null bytes and control characters
- Logs validation errors without crashing

Pre-filter inputs before classification for best results.

## Aggregator Agent

### Purpose and Use Cases

The Aggregator agent synthesizes outputs from multiple agents into a coherent, unified result.
It handles conflicts, deduplication, and consensus-building using LLM-powered analysis.

**Use Cases:**

- Multi-agent research synthesis
- Combining outputs from specialized agents
- Consensus building in distributed AI systems
- Ensemble learning for improved accuracy
- Cross-validation of agent outputs
- RAG systems with multiple retrievers

### Core Features

#### 1. Multiple Aggregation Strategies

Five built-in strategies for different use cases:

- **Consensus**: Find common ground and resolve conflicts
- **Weighted**: Prioritize certain agents over others
- **Semantic**: Group similar outputs using clustering
- **Hierarchical**: Multi-level summarization and synthesis
- **RAG-based**: Retrieval-augmented generation from agent outputs

#### 2. Conflict Resolution

Automatically detects and resolves contradictions:

```json
{
  "conflicts_resolved": [
    {
      "topic": "pricing_model",
      "conflicting_sources": ["agent_a", "agent_b"],
      "resolution": "subscription-based",
      "reasoning": "Agent A provided specific pricing data; Agent B was speculative"
    }
  ]
}
```

#### 3. Semantic Clustering

Groups similar outputs automatically:

```json
{
  "semantic_clusters": [
    {
      "cluster_id": "cluster_0",
      "members": ["research_agent", "analysis_agent"],
      "core_concept": "market_trends",
      "avg_similarity": 0.89
    }
  ]
}
```

#### 4. Consensus Scoring

Quantifies agreement among agents:

```json
{
  "consensus_level": 0.87,  // 87% agreement among inputs
  "aggregated_content": "Synthesized output..."
}
```

#### 5. Performance Tracking

Built-in observability:

- Token usage per aggregation
- Processing time metrics
- Average consensus levels
- Conflict resolution counts

### Aggregation Strategies

#### Consensus Strategy (Default)

Finds common themes and resolves disagreements through LLM analysis.

**Best for:**

- Combining multiple research agents
- Fact-checking across sources
- Building unified recommendations

**Configuration:**

```yaml
aggregator_config:
  aggregation_strategy: consensus
  consensus_threshold: 0.7  # Minimum agreement level
  conflict_resolution: llm_mediated
```

**How it works:**

1. Collects inputs from all sources
2. Identifies common themes and disagreements
3. Uses LLM to analyze and resolve conflicts
4. Synthesizes unified output with reasoning
5. Calculates consensus score based on input similarity

#### Weighted Strategy

Applies importance weights to different agent outputs.

**Best for:**

- Prioritizing expert agents over general agents
- Incorporating human feedback weights
- Confidence-based aggregation

**Configuration:**

```yaml
aggregator_config:
  aggregation_strategy: weighted
  source_weights:
    expert_agent: 1.0
    general_agent_1: 0.6
    general_agent_2: 0.4
```

**How it works:**

1. Applies configured weights to each input
2. Sorts inputs by weight (highest first)
3. LLM synthesizes with explicit weight awareness
4. Higher-weighted sources have more influence

#### Semantic Strategy

Groups inputs by semantic similarity before aggregation.

**Best for:**

- Large numbers of agents (5+)
- Diverse output types
- Identifying distinct perspectives

**Configuration:**

```yaml
aggregator_config:
  aggregation_strategy: semantic
  semantic_similarity_threshold: 0.85  # Clustering threshold
  deduplication_method: semantic
```

**How it works:**

1. Calculates similarity between all input pairs
2. Clusters inputs above similarity threshold
3. Identifies core concept for each cluster
4. LLM synthesizes each cluster separately
5. Combines cluster insights into final output

#### Hierarchical Strategy

Multi-level aggregation for scalability.

**Best for:**

- 10+ agents
- Very long outputs
- Structured summarization

**Configuration:**

```yaml
aggregator_config:
  aggregation_strategy: hierarchical
  max_input_sources: 20
  summarization_enabled: true
```

**How it works:**

1. Groups inputs into batches of 3
2. Summarizes each group independently
3. Aggregates group summaries into final output
4. Reduces token usage for large-scale aggregation

#### RAG-Based Strategy

Treats agent outputs as retrieved context for generation.

**Best for:**

- Question-answering systems
- Multi-source research
- Citation-based synthesis

**Configuration:**

```yaml
aggregator_config:
  aggregation_strategy: rag_based
  max_input_sources: 10
```

**How it works:**

1. Formats agent outputs as retrieval context
2. Each input tagged with source agent
3. LLM generates response using all context
4. Maintains source attribution

### Aggregator Configuration

Full aggregator configuration structure:

```yaml
agents:
  - name: output_aggregator
    type: aggregator
    model: gpt-4
    inputs:
      - source: agent_1_output
      - source: agent_2_output
      - source: agent_3_output
    outputs:
      - target: final_output

    aggregator_config:
      # Strategy selection
      aggregation_strategy: consensus  # consensus, weighted, semantic, hierarchical, rag_based

      # Conflict handling
      conflict_resolution: llm_mediated

      # Deduplication
      deduplication_method: semantic

      # Enable summarization in output
      summarization_enabled: true

      # Maximum agents to aggregate
      max_input_sources: 10

      # Timeout for collecting inputs (ms)
      timeout_ms: 5000

      # Semantic similarity threshold for clustering
      semantic_similarity_threshold: 0.85

      # Source weights (for weighted strategy)
      source_weights:
        agent_1_output: 1.0
        agent_2_output: 0.7
        agent_3_output: 0.5

      # Consensus threshold
      consensus_threshold: 0.7

      # LLM parameters
      temperature: 0.5      # Balanced for synthesis
      max_tokens: 1500      # More tokens for comprehensive aggregation
```

### Common Usage Patterns

#### Multi-Agent Research Synthesis

```yaml
agents:
  # Multiple research agents
  - name: web_researcher
    type: researcher
    outputs:
      - target: research_results

  - name: paper_analyzer
    type: analyzer
    outputs:
      - target: research_results

  - name: expert_agent
    type: expert
    outputs:
      - target: research_results

  # Aggregator synthesizes all research
  - name: synthesis_agent
    type: aggregator
    model: gpt-4
    inputs:
      - source: research_results
    outputs:
      - target: final_report
    aggregator_config:
      aggregation_strategy: consensus
      consensus_threshold: 0.75
      summarization_enabled: true
```

#### Weighted Expert Consensus

```yaml
aggregator_config:
  aggregation_strategy: weighted
  source_weights:
    senior_expert: 1.0
    domain_specialist: 0.9
    general_ai: 0.5
    fallback_agent: 0.3
  conflict_resolution: highest_weight_wins
```

#### Large-Scale Hierarchical Aggregation

```yaml
aggregator_config:
  aggregation_strategy: hierarchical
  max_input_sources: 50
  timeout_ms: 10000  # Longer timeout for many agents
  summarization_enabled: true
  temperature: 0.4   # Lower temperature for factual synthesis
```

#### RAG Pipeline with Multiple Retrievers

```yaml
agents:
  # Multiple retrieval agents
  - name: vector_retriever
    type: retriever
    outputs:
      - target: retrieval_results

  - name: keyword_retriever
    type: retriever
    outputs:
      - target: retrieval_results

  - name: graph_retriever
    type: retriever
    outputs:
      - target: retrieval_results

  # RAG aggregator
  - name: rag_synthesizer
    type: aggregator
    model: gpt-4
    inputs:
      - source: retrieval_results
    outputs:
      - target: answer
    aggregator_config:
      aggregation_strategy: rag_based
      max_tokens: 2000
```

### Aggregator Best Practices

#### 1. Strategy Selection

**Consensus** - When you need:

- Fact verification across sources
- Balanced synthesis
- Conflict transparency

**Weighted** - When you need:

- Expert prioritization
- Confidence-based mixing
- Known source reliability differences

**Semantic** - When you need:

- Deduplication of similar ideas
- Perspective identification
- Many agents (5+)

**Hierarchical** - When you need:

- Scalability to 10+ agents
- Token efficiency
- Structured summarization

**RAG-based** - When you need:

- Citation preservation
- Question answering
- Retrieved context synthesis

#### 2. Timeout Configuration

Set timeout based on expected input arrival:

- **Fast agents (1-2s response)**: `timeout_ms: 3000`
- **Standard agents (3-5s)**: `timeout_ms: 5000`
- **Complex agents (5-10s)**: `timeout_ms: 10000`

The aggregator buffers inputs and processes them when the timeout expires.

#### 3. Input Buffer Management

The aggregator automatically buffers inputs from multiple sources before processing them together. This buffering is handled internally and requires no configuration from users.

Best practices for working with the aggregator:

- Set appropriate timeout values to control when buffered inputs are processed
- Latest message from each source is used when timeout expires
- All buffering is thread-safe and managed automatically by the framework

#### 4. Consensus Threshold Tuning

- **0.5-0.6**: High disagreement acceptable, exploratory
- **0.7-0.8**: Production baseline, good agreement
- **0.85+**: Require strong consensus, may reject valid but diverse inputs

#### 5. Token Management

Aggregation typically uses more tokens than classification:

- **2-3 agents**: 500-1000 tokens
- **4-6 agents**: 1000-1500 tokens
- **7-10 agents**: 1500-2500 tokens
- **10+ agents**: Use hierarchical strategy

Monitor `tokens_used` in results to optimize.

#### 6. Observability

The aggregator logs statistics every 10 aggregations:

```text
Aggregator Stats: Total: 100, Avg Consensus: 0.82, Conflicts: 23,
Avg Time: 1.2s, Tokens: 125000
```

Use these metrics to:

- Detect degrading consensus (agents diverging)
- Identify token usage trends
- Monitor processing times
- Track conflict frequency

## Examples

Complete example workflows are available in the `examples/` directory:

### Classifier Workflow

```bash
examples/classifier-workflow/
├── config.yaml           # Full classifier pipeline configuration
├── main.go              # Executable example
└── README.md            # Classifier example documentation
```

**Demonstrates:**

- Multi-category classification
- Few-shot learning configuration
- Confidence threshold tuning
- Performance metrics collection

### Aggregator Workflow

```bash
examples/aggregator-workflow/
├── config.yaml           # Multi-agent aggregation setup
├── main.go              # Executable example
└── README.md            # Aggregator example documentation
```

**Demonstrates:**

- All five aggregation strategies
- Multi-agent coordination
- Conflict resolution
- Consensus measurement

## Integration Guide

### Basic Integration Steps

**Step 1: Define Agent in Configuration**

```yaml
agents:
  - name: my_classifier
    type: classifier
    model: gpt-4
    inputs:
      - source: input_queue
    outputs:
      - target: classified_output
    classifier_config:
      # ... configuration
```

**Step 2: Set Required Environment Variables**

```bash
# For OpenAI
export OPENAI_API_KEY=your_key_here

# For Anthropic
export ANTHROPIC_API_KEY=your_key_here

# For xAI
export XAI_API_KEY=your_key_here

# For Vertex AI
export VERTEX_PROJECT_ID=your_project
export VERTEX_LOCATION=us-central1
```

**Step 3: Run the Agent Pipeline**

```go
import "github.com/aixgo-dev/aixgo/pkg/runtime"

func main() {
    rt, err := runtime.NewRuntime("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    if err := rt.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Multi-Agent Patterns

#### Parallel Classification + Aggregation

```yaml
agents:
  # Producer
  - name: producer
    type: producer
    outputs:
      - target: content

  # Multiple specialized classifiers
  - name: sentiment_classifier
    type: classifier
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

  - name: topic_classifier
    type: classifier
    inputs:
      - source: content
    outputs:
      - target: classifications
    classifier_config:
      categories:
        - name: tech
          description: "Technology topic"
        - name: business
          description: "Business topic"

  # Aggregator combines classifications
  - name: aggregator
    type: aggregator
    inputs:
      - source: classifications
    outputs:
      - target: final_result
    aggregator_config:
      aggregation_strategy: consensus
```

#### Hierarchical Processing Pipeline

```yaml
agents:
  # Stage 1: Initial classification
  - name: primary_classifier
    type: classifier
    outputs:
      - target: primary_category

  # Stage 2: Sub-classification based on primary
  - name: tech_subclassifier
    type: classifier
    inputs:
      - source: primary_category
    outputs:
      - target: tech_details

  - name: business_subclassifier
    type: classifier
    inputs:
      - source: primary_category
    outputs:
      - target: business_details

  # Stage 3: Aggregate detailed classifications
  - name: final_aggregator
    type: aggregator
    inputs:
      - source: tech_details
      - source: business_details
    outputs:
      - target: comprehensive_result
```

### Error Handling

Both agents include robust error handling:

**Validation Errors:**

- Logged but don't crash the agent
- Invalid inputs are skipped
- Continues processing next input

**LLM Errors:**

- Returned as errors from processing methods
- Logged with context
- Can be monitored via observability spans

**Configuration Errors:**

- Fail at agent initialization (fail-fast)
- Provide clear error messages
- Validate all required fields

### Monitoring Integration

Both agents support the observability framework:

```go
import "github.com/aixgo-dev/aixgo/internal/observability"

// Automatically created spans
span := observability.StartSpan("classifier.classify", map[string]any{
    "input_length": len(input),
    "categories": len(config.Categories),
})
defer span.End()
```

Integrate with your observability backend:

- OpenTelemetry traces
- Custom metrics collection
- Performance monitoring

## Performance Considerations

### Token Usage

**Classifier:**

- Base usage: 200-500 tokens per classification
- With few-shot (3 examples): +150-300 tokens
- With many categories (10+): +100-200 tokens
- **Optimization**: Limit category descriptions, use concise examples

**Aggregator:**

- 2-3 agents: 500-1000 tokens
- 4-6 agents: 1000-1500 tokens
- 7-10 agents: 1500-2500 tokens
- Hierarchical (10+ agents): 1000-2000 tokens (efficient)
- **Optimization**: Use hierarchical strategy for many agents

### Latency

**Classifier:**

- Typical: 500ms - 2s depending on model
- Factors: Model speed, input length, few-shot examples
- **Optimization**: Use faster models (GPT-3.5 vs GPT-4), cache prompts

**Aggregator:**

- Typical: 1s - 5s depending on strategy and agent count
- Factors: Number of inputs, strategy complexity, timeout setting
- **Optimization**: Tune timeouts, use hierarchical for many agents

### Caching

**Prompt Caching (Classifier):**

The classifier maintains an internal prompt cache:

```go
promptCache: map[string]string  // Caches built prompts
```

Benefits:

- Reduces prompt construction overhead
- Speeds up repeated similar inputs
- Automatic cache management (no configuration needed)

**Limitations:**

- In-memory only (cleared on restart)
- Cache size not limited (uses available memory)

**Provider-Level Caching:**

Some providers offer prompt caching:

- Anthropic: Automatic prompt caching for repeated prefixes
- OpenAI: No built-in prompt caching
- Consider using provider caching for high-volume scenarios

### Concurrency

**Classifier:**

- Single goroutine per agent instance
- Processes inputs sequentially from queue
- **Scaling**: Run multiple classifier instances for parallel processing

**Aggregator:**

- Collects from multiple input channels concurrently
- Thread-safe input buffering with mutex
- Processes aggregations sequentially
- **Scaling**: Separate aggregators for independent aggregation tasks

### Memory Usage

**Classifier:**

- Prompt cache: ~1-5 KB per cached prompt
- Performance data: Last 1000 classifications (~100 KB)
- Minimal memory footprint

**Aggregator:**

- Input buffer: Size of buffered messages (cleared each timeout)
- Statistics: Last N processing times (~10 KB)
- Semantic clusters: Temporary during processing

**Best Practices:**

- Monitor memory in high-throughput scenarios
- Consider buffer clear frequency for aggregators
- Performance data auto-managed (keeps last 1000 records)

### Cost Optimization

**Choose Appropriate Models**

```yaml
# Production traffic - balance cost and quality
model: gpt-4o-mini

# Critical decisions - maximum accuracy
model: gpt-4

# High volume, simple classification - lowest cost
model: gpt-3.5-turbo
```

**Optimize Token Usage**

- Concise category descriptions
- Limit few-shot examples to 3
- Use hierarchical aggregation for 10+ agents
- Set appropriate `max_tokens` limits

**Batch Processing**

The aggregator naturally batches inputs within the timeout window - optimize timeout for your throughput.

**Monitor Costs**

Track `tokens_used` in results and use provider dashboards to monitor spending.

---

## Additional Resources

- [Main aixgo Documentation](../README.md)
- [Agent Framework Guide](../docs/agent-framework.md)
- [LLM Provider Configuration](../docs/providers.md)
- [Example Workflows](../examples/)

## Support

For issues, questions, or contributions:

- GitHub Issues: [https://github.com/aixgo-dev/aixgo/issues](https://github.com/aixgo-dev/aixgo/issues)
- Documentation: [https://github.com/aixgo-dev/aixgo](https://github.com/aixgo-dev/aixgo)
