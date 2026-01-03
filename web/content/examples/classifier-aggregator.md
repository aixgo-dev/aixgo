---
title: 'Classifier and Aggregator Examples'
description: "Production-ready examples showcasing AI-powered content classification and multi-agent aggregation workflows."
breadcrumb: 'Examples'
category: 'Examples'
weight: 4
---

This guide showcases two powerful agent types for intelligent content processing and multi-agent coordination: the Classifier agent and the Aggregator agent. These examples demonstrate real-world use cases with complete configurations and explanations.

**Working Examples**:

- [classifier-workflow](https://github.com/aixgo-dev/aixgo/tree/main/examples/classifier-workflow) - Customer support ticket classification with intelligent routing
- [aggregator-workflow](https://github.com/aixgo-dev/aixgo/tree/main/examples/aggregator-workflow) - Multi-expert research synthesis with three aggregation strategies

## Overview

### Classifier Agent

The Classifier agent uses LLM-powered semantic understanding to categorize content with confidence scoring, structured outputs, and few-shot learning capabilities. It goes beyond simple keyword matching to understand context and nuance.

**Key Capabilities:**

- Semantic content categorization
- Confidence scoring (0-1 scale)
- Few-shot learning without fine-tuning
- Multi-label classification support
- Structured JSON outputs with schema validation
- Alternative category suggestions

### Aggregator Agent

The Aggregator agent synthesizes outputs from multiple agents using intelligent strategies including consensus building, weighted synthesis, semantic clustering, hierarchical summarization, and RAG-based aggregation.

**Key Capabilities:**

- Five aggregation strategies for different use cases
- Automatic conflict detection and resolution
- Semantic clustering of similar outputs
- Consensus scoring and metrics
- Source attribution and weighting
- Performance tracking and observability

## Classifier Agent Examples

### Example 1: Customer Support Ticket Classification

This example demonstrates an AI-powered support ticket routing system that automatically categorizes and prioritizes incoming customer requests.

#### Use Case

A SaaS company receives hundreds of support tickets daily across various categories:

- Technical issues requiring engineering support
- Billing inquiries for the finance team
- Account access problems for the security team
- Feature requests for product management
- Bug reports for quality assurance
- General inquiries for customer success

The Classifier agent analyzes each ticket's content and automatically routes it to the appropriate team with priority assignment.

#### Configuration

```yaml
supervisor:
  name: support-coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  # Producer simulates incoming tickets
  - name: ticket-source
    role: producer
    interval: 2s
    outputs:
      - target: incoming-tickets

  # Classifier categorizes tickets
  - name: ticket-classifier
    role: classifier
    model: gpt-4-turbo
    inputs:
      - source: incoming-tickets
    outputs:
      - target: classified-tickets

    classifier_config:
      # Category definitions with rich metadata
      categories:
        - name: technical_issue
          description: "Issues requiring technical troubleshooting or product support including API errors, performance problems, system failures, and integration issues"
          keywords: ["error", "bug", "crash", "not working", "500", "timeout", "api", "integration"]
          examples:
            - "The API returns 500 errors when I try to create a new user"
            - "Our application crashes when processing large files"
            - "Dashboard performance is very slow with large datasets"

        - name: billing_inquiry
          description: "Questions about payments, invoices, pricing, subscriptions, or refunds"
          keywords: ["payment", "invoice", "charge", "refund", "subscription", "billing", "price"]
          examples:
            - "I was charged twice for this month's subscription"
            - "Can I get a refund for the unused portion?"
            - "What's the difference between Pro and Enterprise pricing?"

        - name: account_access
          description: "Login problems, password resets, authentication issues, or security concerns"
          keywords: ["login", "password", "access", "authentication", "locked", "2fa", "security"]
          examples:
            - "I can't log into my account after the password reset"
            - "My account is locked and I need access urgently"
            - "2FA codes aren't being sent to my phone"

        - name: feature_request
          description: "Suggestions for new features, enhancements, or product improvements"
          keywords: ["feature", "enhancement", "suggestion", "would be nice", "could you add"]
          examples:
            - "Would be great to have dark mode support"
            - "Can you add export to Excel functionality?"
            - "Integration with Slack would be very helpful"

        - name: bug_report
          description: "Detailed reports of system defects, unexpected behavior, or errors with reproduction steps"
          keywords: ["bug", "defect", "incorrect", "broken", "wrong", "unexpected"]
          examples:
            - "The date picker shows wrong dates in Safari browser"
            - "Export function produces corrupted CSV files"
            - "Notifications are sent multiple times for the same event"

        - name: general_inquiry
          description: "Other questions about products, services, documentation, or company information"
          keywords: ["question", "how to", "information", "hours", "documentation"]
          examples:
            - "What are your business hours for phone support?"
            - "Where can I find the API documentation?"
            - "Do you offer enterprise-level SLAs?"

      # Minimum confidence threshold
      confidence_threshold: 0.7

      # Disable multi-label for clear routing
      multi_label: false

      # Few-shot examples improve accuracy
      few_shot_examples:
        - input: "My account credentials aren't working after I reset my password"
          category: account_access
          reason: "User experiencing authentication issues following password reset"

        - input: "Can I downgrade my subscription and get a partial refund?"
          category: billing_inquiry
          reason: "Question about subscription changes and refund policies"

        - input: "The search function returns no results even though the data exists"
          category: bug_report
          reason: "Specific system defect with reproduction scenario"

      # LLM parameters
      temperature: 0.3      # Low temperature for consistent categorization
      max_tokens: 500       # Sufficient for reasoning

  # Logger outputs results
  - name: classification-logger
    role: logger
    inputs:
      - source: classified-tickets
```

#### Expected Output

```json
{
  "category": "technical_issue",
  "confidence": 0.92,
  "reasoning": "User describes a specific API error (500) when performing a standard operation (user creation). This requires technical investigation and troubleshooting by the engineering team.",
  "alternatives": [
    {
      "category": "bug_report",
      "confidence": 0.45
    }
  ],
  "tokens_used": 234,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Key Features Demonstrated

- **Semantic Understanding**: Goes beyond keywords to understand context
- **Confidence Scoring**: Each classification includes a confidence metric
- **Alternative Suggestions**: Provides secondary options for ambiguous cases
- **Few-Shot Learning**: Examples improve accuracy without model fine-tuning
- **Structured Outputs**: JSON schema validation ensures reliable parsing

**View Complete Example**: [examples/classifier-workflow](https://github.com/aixgo-dev/aixgo/tree/main/examples/classifier-workflow)

### Example 2: Multi-Label Content Tagging

This example shows how to use the Classifier agent for assigning multiple tags to content items.

#### Configuration Snippet

```yaml
classifier_config:
  # Enable multi-label classification
  multi_label: true

  categories:
    - name: technical
      description: "Contains technical or engineering content"

    - name: urgent
      description: "Requires immediate attention or action"

    - name: customer_facing
      description: "Should be visible to end customers"

    - name: executive_summary
      description: "Suitable for executive-level summaries"

  # Lower threshold for multi-label scenarios
  confidence_threshold: 0.6

  temperature: 0.4
```

#### Expected Output

```json
{
  "categories": ["technical", "urgent", "customer_facing"],
  "confidence_scores": {
    "technical": 0.89,
    "urgent": 0.76,
    "customer_facing": 0.71,
    "executive_summary": 0.42
  },
  "reasoning": "Content contains technical details about a critical production issue that affects customers and requires immediate engineering attention.",
  "tokens_used": 312
}
```

## Aggregator Agent Examples

### Example 1: Multi-Expert Research Synthesis

This example demonstrates how multiple specialized AI agents can analyze a complex topic from different perspectives, with the Aggregator agent synthesizing their insights into a comprehensive report.

#### Use Case

A research team is analyzing "The Impact of Large Language Models on Software Development." They deploy six specialized expert agents:

- **Technical Expert**: Deep technical implementation analysis
- **Data Scientist**: Empirical metrics and statistical analysis
- **Business Analyst**: ROI and economic impact assessment
- **Security Expert**: Security risks and vulnerability analysis
- **Ethics Expert**: Ethical implications and bias considerations
- **Domain Expert**: Practical implementation challenges

The Aggregator agent combines their perspectives using three different strategies: consensus, semantic clustering, and weighted synthesis.

#### Configuration

```yaml
supervisor:
  name: research-coordinator
  model: gpt-4-turbo
  max_rounds: 15

agents:
  # Research topic producer
  - name: research-prompt
    role: producer
    interval: 10s
    outputs:
      - target: research-topic

  # Expert agents analyze from different perspectives
  - name: technical-expert
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a senior technical architect with 15 years of experience.
      Analyze topics from a deep technical implementation perspective.
      Focus on architecture, scalability, performance, and technical feasibility.
    inputs:
      - source: research-topic
    outputs:
      - target: expert-analyses

  - name: data-scientist
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a data scientist specializing in empirical analysis.
      Provide statistical insights, metrics, and data-driven analysis.
      Focus on measurable impacts and quantitative assessment.
    inputs:
      - source: research-topic
    outputs:
      - target: expert-analyses

  - name: business-analyst
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a business analyst focused on ROI and economic impact.
      Analyze business implications, cost-benefit, and market dynamics.
      Focus on organizational impact and financial considerations.
    inputs:
      - source: research-topic
    outputs:
      - target: expert-analyses

  - name: security-expert
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a cybersecurity expert specializing in AI systems.
      Analyze security risks, vulnerabilities, and compliance considerations.
      Focus on threat modeling and security best practices.
    inputs:
      - source: research-topic
    outputs:
      - target: expert-analyses

  - name: ethics-expert
    role: react
    model: gpt-4-turbo
    prompt: |
      You are an AI ethics expert.
      Analyze ethical implications, bias, fairness, and social impact.
      Focus on responsible AI practices and ethical considerations.
    inputs:
      - source: research-topic
    outputs:
      - target: expert-analyses

  - name: domain-expert
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a domain expert with practical implementation experience.
      Analyze real-world challenges, adoption barriers, and practical considerations.
      Focus on implementation feasibility and lessons learned.
    inputs:
      - source: research-topic
    outputs:
      - target: expert-analyses

  # Consensus aggregation - find common ground
  - name: consensus-aggregator
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: expert-analyses
    outputs:
      - target: consensus-synthesis
    aggregator_config:
      aggregation_strategy: consensus
      consensus_threshold: 0.75
      conflict_resolution: llm_mediated
      timeout_ms: 5000
      temperature: 0.5
      max_tokens: 2000

  # Semantic aggregation - cluster by themes
  - name: semantic-aggregator
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: expert-analyses
    outputs:
      - target: semantic-synthesis
    aggregator_config:
      aggregation_strategy: semantic
      semantic_similarity_threshold: 0.85
      deduplication_method: semantic
      timeout_ms: 5000
      temperature: 0.4
      max_tokens: 2000

  # Weighted aggregation - prioritize expertise
  - name: weighted-aggregator
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: expert-analyses
    outputs:
      - target: weighted-synthesis
    aggregator_config:
      aggregation_strategy: weighted
      source_weights:
        technical-expert: 0.9
        data-scientist: 0.85
        business-analyst: 0.75
        security-expert: 0.95
        ethics-expert: 0.80
        domain-expert: 0.85
      conflict_resolution: highest_weight_wins
      timeout_ms: 5000
      temperature: 0.5
      max_tokens: 2000

  # Final logger
  - name: synthesis-logger
    role: logger
    inputs:
      - source: consensus-synthesis
      - source: semantic-synthesis
      - source: weighted-synthesis
```

#### Expected Output - Consensus Strategy

```json
{
  "strategy": "consensus",
  "consensus_level": 0.87,
  "aggregated_content": "After analyzing expert inputs, there is strong consensus (87%) on the following key findings:\n\n1. LLMs significantly accelerate routine development tasks (40-60% productivity gain)\n2. Security considerations require new scanning and validation approaches\n3. Code quality shows mixed results, requiring human oversight\n4. Business ROI is positive for organizations above certain scale\n5. Ethical considerations around training data and bias remain critical\n\nKey areas of agreement:\n- Transformative impact on software development workflows\n- Need for new tooling and processes\n- Importance of developer training and adaptation\n\nResolved conflicts:\n- Testing approaches: Hybrid strategy combining automated and manual review\n- Adoption timeline: Phased implementation recommended over wholesale replacement",

  "conflicts_resolved": [
    {
      "topic": "testing_methodology",
      "conflicting_sources": ["technical-expert", "domain-expert"],
      "resolution": "Hybrid approach combining both perspectives",
      "reasoning": "Technical expert emphasized automated testing capabilities while domain expert highlighted practical limitations. Resolution integrates both automated AI-assisted testing with mandatory human review for critical paths."
    }
  ],

  "tokens_used": 1850,
  "processing_time_ms": 2340
}
```

#### Expected Output - Semantic Strategy

```json
{
  "strategy": "semantic",
  "semantic_clusters": [
    {
      "cluster_id": "cluster_0",
      "members": ["technical-expert", "domain-expert"],
      "core_concept": "Implementation Challenges",
      "avg_similarity": 0.89,
      "summary": "Both experts emphasize practical implementation barriers including integration complexity, tooling maturity, and organizational readiness."
    },
    {
      "cluster_id": "cluster_1",
      "members": ["security-expert", "ethics-expert"],
      "core_concept": "Risk and Governance",
      "avg_similarity": 0.82,
      "summary": "Shared focus on risk management, compliance frameworks, and responsible AI practices."
    },
    {
      "cluster_id": "cluster_2",
      "members": ["business-analyst", "data-scientist"],
      "core_concept": "Measurable Impact",
      "avg_similarity": 0.85,
      "summary": "Quantitative analysis of productivity gains, cost savings, and empirical performance metrics."
    }
  ],

  "aggregated_content": "Thematic analysis reveals three primary concern areas:\n\n**Implementation Challenges (Technical + Domain)**\n- Integration with existing development workflows\n- Tooling ecosystem maturity gaps\n- Developer training and skill adaptation\n\n**Risk and Governance (Security + Ethics)**\n- New vulnerability classes from AI-generated code\n- Bias in training data and outputs\n- Compliance and regulatory considerations\n\n**Measurable Impact (Business + Data)**\n- 40-60% productivity gains for routine tasks\n- ROI positive at scale (100+ developers)\n- Variable quality requiring oversight investment",

  "tokens_used": 1650,
  "processing_time_ms": 2120
}
```

#### Expected Output - Weighted Strategy

```json
{
  "strategy": "weighted",
  "applied_weights": {
    "security-expert": 0.95,
    "technical-expert": 0.9,
    "domain-expert": 0.85,
    "data-scientist": 0.85,
    "ethics-expert": 0.80,
    "business-analyst": 0.75
  },

  "aggregated_content": "Weighted analysis prioritizing security and technical expertise yields:\n\n**Critical Priority (Security Expert - Weight 0.95)**\n- New attack vectors from AI-generated code require specialized scanning\n- Supply chain risks from training data dependencies\n- Compliance frameworks lagging behind technology adoption\n\n**High Priority (Technical Expert - Weight 0.9)**\n- Architecture patterns evolving toward AI-first designs\n- Performance characteristics differ from traditional code\n- Integration complexity higher than anticipated\n\n**Important Considerations (Other Experts)**\n- Business ROI positive with appropriate scale and oversight\n- Ethical implications require ongoing monitoring\n- Practical adoption challenges in legacy systems\n\nRecommendations (weighted by expertise):\n1. Security-first adoption approach (Security Expert)\n2. Phased rollout with monitoring (Technical + Domain Experts)\n3. Investment in training and tooling (All Experts)",

  "tokens_used": 1720,
  "processing_time_ms": 2250
}
```

#### Key Features Demonstrated

- **Multiple Aggregation Strategies**: Consensus, semantic, and weighted approaches
- **Conflict Resolution**: Automatic detection and LLM-mediated resolution
- **Semantic Clustering**: Grouping similar expert perspectives
- **Weighted Synthesis**: Prioritizing high-authority sources
- **Comprehensive Metrics**: Consensus levels, token usage, processing time

**View Complete Example**: [examples/aggregator-workflow](https://github.com/aixgo-dev/aixgo/tree/main/examples/aggregator-workflow)

### Example 2: RAG Pipeline with Multiple Retrievers

This example shows how to use the Aggregator agent in a retrieval-augmented generation (RAG) system with multiple specialized retrievers.

#### Configuration Snippet

```yaml
agents:
  # Multiple retrieval agents
  - name: vector-retriever
    role: react
    model: gpt-4-turbo
    prompt: "You are a vector similarity retriever"
    outputs:
      - target: retrieval-results

  - name: keyword-retriever
    role: react
    model: gpt-4-turbo
    prompt: "You are a keyword-based retriever"
    outputs:
      - target: retrieval-results

  - name: graph-retriever
    role: react
    model: gpt-4-turbo
    prompt: "You are a graph traversal retriever"
    outputs:
      - target: retrieval-results

  # RAG aggregator synthesizes retrieved content
  - name: rag-synthesizer
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: retrieval-results
    outputs:
      - target: final-answer
    aggregator_config:
      aggregation_strategy: rag_based
      max_input_sources: 10
      timeout_ms: 3000
      temperature: 0.7
      max_tokens: 2000
```

## When to Use Each Strategy

### Classifier Agent

**Use Classifier When:**

- You need to categorize or route content automatically
- Semantic understanding is important (beyond keyword matching)
- You want confidence scores for quality assessment
- Multi-label tagging is required
- Few-shot learning can improve accuracy without fine-tuning

**Example Scenarios:**

- Customer support ticket routing
- Content moderation and filtering
- Intent detection in chatbots
- Document classification and organization
- Sentiment analysis with custom categories

### Aggregator Agent Strategies

#### Consensus Strategy

**Use When:**

- You need to find common ground among diverse opinions
- Conflict resolution and transparency are important
- Building agreement for decision-making
- Identifying universally accepted insights

**Example Scenarios:**

- Multi-expert research synthesis
- Fact verification across sources
- Team decision-making processes
- Policy development with stakeholder input

#### Weighted Strategy

**Use When:**

- Some agents have more expertise or authority
- Certain perspectives are more critical
- You need to balance expertise with inclusion
- High-stakes decisions requiring domain expertise

**Example Scenarios:**

- Expert panel analysis with varying credentials
- Prioritizing technical over non-technical input
- Confidence-based output mixing
- Quality-weighted content aggregation

#### Semantic Strategy

**Use When:**

- Understanding thematic relationships is crucial
- You want to preserve conceptual groupings
- Dealing with complex, multi-faceted topics
- Many agents (5+) producing varied outputs
- Deduplication of similar ideas is needed

**Example Scenarios:**

- Large-scale research synthesis
- Theme extraction from diverse sources
- Perspective identification and clustering
- Knowledge map creation

#### Hierarchical Strategy

**Use When:**

- Dealing with very large numbers of agents (10+)
- Multi-level summarization is needed
- Token efficiency is critical
- Recursive aggregation provides better results

**Example Scenarios:**

- Enterprise-scale multi-agent systems
- Cascading summarization pipelines
- Cost-optimized large-scale aggregation

#### RAG-Based Strategy

**Use When:**

- You have a knowledge base to reference
- Source attribution is important
- Fact-checking against documentation is needed
- Question-answering with citations

**Example Scenarios:**

- Multi-retriever RAG systems
- Document-based Q&A
- Research with source tracking
- Compliance-oriented systems requiring citations

## Performance Considerations

### Classifier Agent

- **Token Usage**: 200-500 tokens per classification
  - Add 150-300 tokens for few-shot examples
  - Add 100-200 tokens for 10+ categories
- **Latency**: 500ms-2s depending on model
- **Cost Optimization**: Use GPT-4o-mini for high-volume classification

### Aggregator Agent

- **Token Usage** (varies by strategy and agent count):
  - 2-3 agents: 500-1000 tokens
  - 4-6 agents: 1000-1500 tokens
  - 7-10 agents: 1500-2500 tokens
  - 10+ agents: Use hierarchical strategy (1000-2000 tokens)
- **Latency**: 1s-5s depending on strategy and agent count
- **Timeout Configuration**:
  - Fast agents (1-2s): `timeout_ms: 3000`
  - Standard agents (3-5s): `timeout_ms: 5000`
  - Complex agents (5-10s): `timeout_ms: 10000`

## Best Practices

### Classifier Best Practices

1. **Category Design**
   - Create clear, mutually exclusive categories (unless multi-label)
   - Provide detailed descriptions with boundary explanations
   - Include 3-5 diverse keywords per category
   - Add 2-3 representative examples

2. **Confidence Tuning**
   - 0.5-0.6: Exploratory use
   - 0.7-0.8: Production baseline
   - 0.85+: High-stakes scenarios

3. **Token Optimization**
   - Use concise category descriptions
   - Limit few-shot examples to 3 per category
   - Set appropriate max_tokens (500 for classification)

### Aggregator Best Practices

1. **Strategy Selection**
   - Consensus: Balanced synthesis with conflict transparency
   - Weighted: Expert prioritization
   - Semantic: Deduplication and theme extraction
   - Hierarchical: Scalability (10+ agents)
   - RAG-based: Citation preservation

2. **Timeout Configuration**
   - Set based on expected agent response times
   - Add buffer for network latency
   - Monitor timeout expiry rates

3. **Input Management**
   - Latest message from each source is used
   - Buffering is automatic and thread-safe
   - No manual buffer management needed

## Next Steps

- **[Agent Types Guide](/guides/agent-types)** - Comprehensive agent documentation
- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Coordination patterns
- **[Classifier Example Source](https://github.com/aixgo-dev/aixgo/tree/main/examples/classifier-workflow)** - Complete implementation
- **[Aggregator Example Source](https://github.com/aixgo-dev/aixgo/tree/main/examples/aggregator-workflow)** - Complete implementation
- **[Agent Framework Code](https://github.com/aixgo-dev/aixgo/tree/main/agents)** - Source code reference

## Additional Resources

- [Classifier Agent Documentation](https://github.com/aixgo-dev/aixgo/blob/main/agents/README.md#classifier-agent)
- [Aggregator Agent Documentation](https://github.com/aixgo-dev/aixgo/blob/main/agents/README.md#aggregator-agent)
- [Complete Examples Directory](https://github.com/aixgo-dev/aixgo/tree/main/examples)
- [API Reference](https://pkg.go.dev/github.com/aixgo-dev/aixgo)
