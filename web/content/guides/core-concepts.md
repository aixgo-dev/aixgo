---
title: 'Core Concepts'
description: "Understand Aixgo's fundamental building blocks: agent types, supervisor patterns, and message-based communication."
breadcrumb: 'Getting Started'
category: 'Getting Started'
weight: 2
---

Aixgo implements a message-based multi-agent architecture inspired by successful production patterns. This guide explains the core concepts you need to understand before building complex systems. These patterns reflect our [philosophy](/why-aixgo) of production-first design with Go-native patterns.

## Agent Types

Aixgo provides six foundational agent types, each designed for specific roles in your system:

### Producer Agents

Producer agents generate periodic messages for downstream processing. They're ideal for data ingestion, event generation, or any scenario where you need to create messages on a
schedule.

```yaml
agents:
  - name: event-generator
    role: producer
    interval: 500ms
    outputs:
      - target: processor
```

**Use cases:**

- Polling external APIs
- Generating synthetic data for testing
- Periodic health checks
- Time-based event triggers

### ReAct Agents

ReAct (Reasoning + Acting) agents combine LLM-powered reasoning with tool calling capabilities. They receive messages, reason about them using an LLM, and can execute tools to
perform actions.

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
```

**Use cases:**

- Data analysis and enrichment
- Decision-making workflows
- Natural language processing
- Complex business logic with LLM reasoning

### Logger Agents

Logger agents consume and persist messages for observability, debugging, and audit trails. They're the endpoints of your data flows.

```yaml
agents:
  - name: audit-log
    role: logger
    inputs:
      - source: analyst
```

**Use cases:**

- Audit logging
- Debugging workflows
- Data persistence
- Monitoring and alerting

### Classifier Agents

Classifier agents categorize content or documents using LLM-powered classification with multiple strategies. They excel at organizing, tagging, and routing data based on
intelligent analysis.

```yaml
agents:
  - name: content-classifier
    role: classifier
    model: gpt-4-turbo
    prompt: 'Classify incoming content into categories'
    strategy: multi-label # Options: zero-shot, few-shot, multi-label, single-label
    categories:
      - technology
      - business
      - science
      - entertainment
```

**Classification Strategies:**

- **ZeroShot** - Classify without training examples
- **FewShot** - Use provided examples for better accuracy
- **MultiLabel** - Assign multiple categories to a single item
- **SingleLabel** - Assign exactly one category per item

**Use cases:**

- Content categorization and tagging
- Email routing and triage
- Document organization
- Sentiment analysis and topic detection

### Aggregator Agents

Aggregator agents combine outputs from multiple agents using sophisticated strategies. They synthesize information, build consensus, and create unified results from diverse inputs.

```yaml
agents:
  - name: result-aggregator
    role: aggregator
    model: gpt-4-turbo
    prompt: 'Combine analysis from multiple sources'
    strategy: semantic # Options: consensus, weighted, semantic, hierarchical, rag
    inputs:
      - source: analyzer-1
      - source: analyzer-2
      - source: analyzer-3
```

**Aggregation Strategies:**

- **Consensus** - Find agreement across inputs
- **Weighted** - Prioritize certain sources over others
- **Semantic** - Use embeddings to merge similar information
- **Hierarchical** - Organize information by importance
- **RAG** - Retrieval-augmented generation for context-aware synthesis

**Use cases:**

- Multi-agent consensus building
- Research synthesis from multiple sources
- Decision-making with diverse inputs
- Knowledge base construction

### Planner Agents

Planner agents create sophisticated reasoning chains and execution strategies. They break down complex problems into steps and coordinate multi-stage workflows.

```yaml
agents:
  - name: task-planner
    role: planner
    model: gpt-4-turbo
    prompt: 'Plan the execution strategy'
    strategy: chain-of-thought # Options: chain-of-thought, tree-of-thought, react, monte-carlo, backward-chaining, hierarchical
```

**Planning Strategies:**

- **Chain-of-Thought** - Linear step-by-step reasoning
- **Tree-of-Thought** - Explore multiple reasoning paths
- **ReAct** - Reason and act iteratively
- **MonteCarlo** - Simulate multiple scenarios
- **Backward Chaining** - Work backwards from goal to steps
- **Hierarchical** - Break down into sub-problems

**Use cases:**

- Complex problem decomposition
- Multi-step workflow planning
- Strategic decision-making
- Research and analysis pipelines

## Vector Stores & RAG

Aixgo includes first-class support for vector databases and Retrieval-Augmented Generation (RAG), enabling your agents to access and search large knowledge bases with semantic understanding.

### What are Vector Stores?

Vector stores are databases optimized for similarity search using high-dimensional embeddings. They enable:

- **Semantic Search**: Find information by meaning, not just keywords
- **Long-Term Memory**: Give agents persistent memory across sessions
- **Knowledge Grounding**: Reduce hallucinations by grounding responses in real data
- **Context Retrieval**: Provide relevant context for more accurate responses

### Collection-Based Architecture

Aixgo's vectorstore uses a Collection-based architecture for logical isolation:

```go
import "github.com/aixgo-dev/aixgo/pkg/vectorstore/memory"

store, _ := memory.New()

// Create isolated collections for different purposes
cache := store.Collection("cache",
    vectorstore.WithTTL(5*time.Minute),
    vectorstore.WithDeduplication(true),
)

memory := store.Collection("agent-memory",
    vectorstore.WithScope("user", "session"),
)

docs := store.Collection("knowledge-base",
    vectorstore.WithDimensions(1536),
)
```

### 10 Powerful Use Cases

1. **Semantic Caching**: Cache LLM responses by meaning, not exact text
2. **Agent Memory**: Give agents persistent, scoped memory
3. **Conversation History**: Track and search dialogue history
4. **Content Deduplication**: Automatically detect duplicate content
5. **Multi-Modal Search**: Search across text, images, and documents
6. **Temporal Data**: Automatic expiration and time-based queries
7. **Multi-Tenancy**: Isolate data by tenant/user/session
8. **Batch Operations**: Efficient bulk indexing with progress tracking
9. **Streaming Queries**: Handle large result sets efficiently
10. **Advanced Filtering**: Complex boolean queries with metadata filters

### Supported Providers

- **Memory**: In-memory store for development and testing
- **Firestore**: Production-ready, serverless vector database
- **Qdrant** (coming soon): High-performance vector search
- **pgvector** (coming soon): PostgreSQL extension for vector search

### Quick Example

```go
// Index documents
doc := &vectorstore.Document{
    ID: "doc1",
    Content: vectorstore.NewTextContent("Aixgo is a Go framework for AI agents"),
    Embedding: vectorstore.NewEmbedding(embedding, "text-embedding-3-small"),
    Tags: []string{"documentation", "framework"},
}
result, _ := docs.Upsert(ctx, doc)

// Semantic search
query := &vectorstore.Query{
    Embedding: vectorstore.NewEmbedding(queryEmb, "text-embedding-3-small"),
    Limit:     5,
    MinScore:  0.7,
    Filters: vectorstore.TagFilter("documentation"),
}
results, _ := docs.Query(ctx, query)
```

### Learn More

- **[Vector Databases Guide](/guides/vector-databases)**: Complete guide to building RAG systems
- **[Embeddings Guide](/guides/embeddings)**: Choosing and using embedding models
- **[RAG Example](/examples/rag-agent)**: Production-ready RAG implementation

## The Supervisor Pattern

The supervisor is the orchestration layer that manages agent lifecycle and message routing. It provides the following capabilities:

### Lifecycle Management

- **Dependency-aware startup** - Starts agents in the correct order based on their input/output relationships
- **Graceful shutdown** - Ensures clean termination of all agents
- **Health monitoring** - Tracks agent status and handles failures

### Message Routing

The supervisor routes messages between agents based on configured inputs and outputs:

```yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

agents:
  - name: producer
    role: producer
    outputs:
      - target: analyzer # Messages flow to analyzer

  - name: analyzer
    role: react
    inputs:
      - source: producer # Receives from producer
    outputs:
      - target: logger # Sends to logger

  - name: logger
    role: logger
    inputs:
      - source: analyzer # Receives from analyzer
```

### Execution Constraints

- **Max rounds** - Limits total iterations to prevent runaway workflows
- **Timeouts** - Prevents agents from running indefinitely
- **Error handling** - Manages agent failures gracefully

### Observability Hooks

The supervisor provides distributed tracing integration, allowing you to:

- Track message flow across agents
- Measure agent performance
- Debug complex workflows
- Monitor system health

## Communication Abstraction

One of Aixgo's most powerful features is its runtime abstraction layer that handles message transport automatically.

### Local Mode (Development)

Uses Go channels for in-process communication:

```go
supervisor := aixgo.NewSupervisor("coordinator")
supervisor.AddAgent(producer)
supervisor.AddAgent(analyzer)
supervisor.Run()  // Uses Go channels internally
```

**Benefits:**

- Fast development iteration
- Easy debugging
- No infrastructure required
- Perfect for prototyping

### Distributed Mode (Production)

Uses gRPC with protobuf for multi-node orchestration:

```go
// Same code as local mode!
supervisor := aixgo.NewSupervisor("coordinator")
supervisor.AddAgent(producer)
supervisor.AddAgent(analyzer)
supervisor.Run()  // Uses gRPC when configured
```

**Benefits:**

- Multi-region deployment
- Horizontal scaling
- Fault isolation
- Production-grade reliability

### Automatic Selection

The runtime automatically selects the appropriate transport based on configuration. This means:

1. **Prototype locally** - Develop with Go channels for speed
2. **Deploy to single instance** - Run on Cloud Run/Lambda with same code
3. **Scale to distributed** - Add configuration, no code changes needed

## Message Flow Example

Here's how messages flow through a typical Aixgo system:

```yaml
# Data flows: producer → analyzer → logger

supervisor:
  name: coordinator
  max_rounds: 5

agents:
  - name: data-source
    role: producer
    interval: 1s
    outputs:
      - target: processor

  - name: processor
    role: react
    model: gpt-4-turbo
    prompt: 'Process the incoming data'
    inputs:
      - source: data-source
    outputs:
      - target: storage

  - name: storage
    role: logger
    inputs:
      - source: processor
```

**Execution flow:**

1. Supervisor starts agents in order: data-source → processor → storage
2. `data-source` generates a message every 1 second
3. Message is routed to `processor` via supervisor
4. `processor` analyzes with LLM and outputs result
5. Message is routed to `storage`
6. `storage` persists the result
7. Process repeats for max_rounds (5 iterations)
8. Supervisor initiates graceful shutdown

## Key Principles

### 1. Declarative Configuration

Agents and workflows are defined in YAML, making them:

- Version-controlled
- Easy to review
- Platform-independent
- Testable

### 2. Type Safety

Go's type system ensures:

- Configuration errors caught at compile time
- Tool interfaces are type-checked
- Refactoring is safe across large systems

### 3. Observable by Default

Every agent interaction is:

- Traceable via OpenTelemetry
- Logged with structured context
- Measurable with metrics

### 4. Production-Ready from Day One

The same code runs in:

- Local development
- Single-instance deployments
- Distributed production systems

## Next Steps

Now that you understand the core concepts, you can:

- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Build complex multi-agent workflows
- **[Single Binary vs Distributed Mode](/guides/single-vs-distributed)** - Understand scaling patterns
- **[Type Safety & LLM Integration](/guides/type-safety)** - Leverage compile-time guarantees
