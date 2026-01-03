---
title: 'Quick Start Guide'
description: 'Get started with Aixgo in under 5 minutes. Build your first multi-agent system.'
breadcrumb: 'Getting Started'
category: 'Getting Started'
weight: 1
---

Get running in under 5 minutes. Create a simple data analysis pipeline with three agents: a producer that generates data, an analyzer that processes it with an LLM, and a logger
that persists the results.

## 1. Install Aixgo

```bash
go get github.com/aixgo-dev/aixgo
```

## 2. Set Up Your API Key

Before running agents, configure your LLM provider API key:

```bash
# For OpenAI (used in this example)
export OPENAI_API_KEY=your-openai-key-here

# OR for xAI/Grok
export XAI_API_KEY=your-xai-key-here

# OR for Anthropic
export ANTHROPIC_API_KEY=your-anthropic-key-here
```

Get your key from:

- **OpenAI Platform**: https://platform.openai.com/
- **xAI Console**: https://console.x.ai/
- **Anthropic Console**: https://console.anthropic.com/

## 3. Create `config/agents.yaml`

```yaml
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

## 4. Create `main.go`

```go
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

## 5. Run it

```bash
go run main.go
```

That's it! You now have a running multi-agent system with producer, analyzer, and logger agents orchestrated by a supervisor. The entire deployment is a single binary.

## What Just Happened?

This example demonstrates Aixgo's core concepts:

- **Producer Agent** (`data-producer`) - Generates periodic messages every second
- **ReAct Agent** (`analyzer`) - Uses an LLM (GPT-4 Turbo) to analyze incoming data
- **Logger Agent** (`logger`) - Persists the analysis results
- **Supervisor** (`coordinator`) - Orchestrates the agents and manages message routing

The supervisor automatically:

- Starts agents in dependency order
- Routes messages from data-producer → analyzer → logger
- Enforces the max_rounds limit (10 iterations)
- Handles graceful shutdown

## Next Steps

Now that you have your first agent running, explore Aixgo's powerful features:

### Build Production Systems

- **[Vector Databases & RAG](/guides/vector-databases)** - Add semantic search and retrieval-augmented generation to eliminate hallucinations
- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Build complex workflows with multiple specialized agents
- **[Production Deployment](/guides/production-deployment)** - Deploy your agents to production with monitoring and scaling

### Advanced Features

- **[Provider Integration](/guides/provider-integration)** - Connect to OpenAI, Anthropic, Google, and more
- **[Observability](/guides/observability)** - Monitor your agents with OpenTelemetry and distributed tracing
- **[Type Safety](/guides/type-safety)** - Leverage Go's type system for compile-time error detection

### Core Concepts

- **[Core Concepts](/guides/core-concepts)** - Learn about agent types and supervisor patterns
- **[Extending Aixgo](/guides/extending-aixgo)** - Add custom LLM providers, vector databases, and embeddings

### Examples

Browse our example configurations for common use cases like chatbots, data processing, and RAG systems
