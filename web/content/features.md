---
title: 'Aixgo Features'
description: "Explore Aixgo's complete feature set across AI agents, LLM providers, security, observability, and infrastructure. Production-ready features for building scalable AI agent systems."
---

**Version**: 0.2.4 | **Status**: Production-Ready Core Features

> **Complete Technical Documentation:**
> - **[FEATURES.md on GitHub](https://github.com/aixgo-dev/aixgo/blob/main/docs/FEATURES.md)** - Authoritative feature catalog with code references
> - **[PATTERNS.md on GitHub](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md)** - Deep-dive guides for all 13 orchestration patterns
> - **[Roadmap](https://github.com/orgs/aixgo-dev/projects/1)** - Development roadmap and planned features

---

## At a Glance

| Category | Count | Status |
|----------|-------|--------|
| Agent Types | 6 specialized types | ✅ All implemented |
| LLM Providers | 6+ cloud + local | ✅ All implemented |
| Orchestration Patterns | 13 patterns | ✅ All implemented |
| Security Modes | 4 auth modes | ✅ All implemented |
| Observability Backends | 6+ backends | ✅ All implemented |

### Performance

- **Binary Size**: <20MB
- **Cold Start**: <100ms
- **Infrastructure Savings**: 60-70% vs Python frameworks

---

## Agent Types

Build specialized agents for different tasks:

| Agent | Purpose | Key Features |
|-------|---------|--------------|
| **ReAct** | Reasoning + Acting | Tool calling, streaming, structured outputs |
| **Classifier** | Content routing | Confidence scores, multi-label, custom taxonomies |
| **Aggregator** | Multi-agent synthesis | 5 LLM strategies + 4 voting modes |
| **Planner** | Task decomposition | Dependency analysis, progress tracking |
| **Producer** | Message generation | Interval-based, event-driven |
| **Logger** | Audit trails | Structured JSON, multiple targets |

---

## LLM Providers

Connect to any provider with a unified interface:

| Provider | Models | Status |
|----------|--------|--------|
| **OpenAI** | GPT-4, GPT-3.5 Turbo | ✅ |
| **Anthropic** | Claude 3.5 Sonnet, Opus, Haiku | ✅ |
| **Google Gemini** | Gemini 1.5 Pro, Flash | ✅ |
| **xAI** | Grok-beta | ✅ |
| **Vertex AI** | Gemini on GCP | ✅ |
| **HuggingFace** | Meta-Llama, Mistral, 100+ models | ✅ |
| **Ollama** | phi, llama, mistral, gemma (local) | ✅ |
| **vLLM** | Self-hosted inference | ✅ |

---

## Orchestration Patterns

All 13 patterns are production-ready:

| Pattern | Benefit |
|---------|---------|
| **Supervisor** | Simple multi-agent coordination |
| **Sequential** | ETL and content pipelines |
| **Parallel** | 3-4× speedup |
| **Router** | 25-50% cost savings |
| **Swarm** | Adaptive agent handoffs |
| **Hierarchical** | Complex workflows |
| **RAG** | 70% token reduction |
| **Reflection** | 20-50% quality improvement |
| **Ensemble** | 25-50% error reduction |
| **Classifier** | Intent-based routing |
| **Aggregation** | Expert consensus |
| **Planning** | Multi-step workflows |
| **MapReduce** | Large dataset processing |

[View Pattern Details →](/features-patterns)

---

## Security

Enterprise-grade security built-in:

- **4 Auth Modes**: Disabled, Delegated (IAP), Builtin (API keys), Hybrid
- **Input Protection**: SSRF protection, sanitization, prompt injection defense
- **Rate Limiting**: Token bucket, per-user quotas
- **Audit**: SIEM integration (Elasticsearch, Splunk, Datadog)

---

## Observability

Complete production monitoring:

- **Tracing**: OpenTelemetry with Langfuse, Jaeger, Honeycomb, Grafana
- **Metrics**: Prometheus export, system and agent metrics
- **Cost Tracking**: Automatic token counting, per-request costs
- **Health Checks**: Kubernetes-ready liveness and readiness probes

---

## Deployment

Deploy anywhere with Go's simplicity:

| Target | Details |
|--------|---------|
| **Single Binary** | <20MB, zero dependencies |
| **Docker** | Multi-stage builds, ~50MB standard |
| **Kubernetes** | Full manifests, HPA ready |
| **Cloud Run** | Auto-scaling, IAP integration |

---

## Getting Started

```bash
go get github.com/aixgo-dev/aixgo
```

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo

agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: "You are a data analyst."
```

```go
package main

import "github.com/aixgo-dev/aixgo"

func main() {
    aixgo.Run("config/agents.yaml")
}
```

---

## Learn More

- **[Quick Start Guide](/guides/quick-start)** - Get running in 5 minutes
- **[Pattern Catalog](/features-patterns)** - All 13 orchestration patterns
- **[GitHub Repository](https://github.com/aixgo-dev/aixgo)** - Source code and examples
- **[Full Feature Reference](https://github.com/aixgo-dev/aixgo/blob/main/docs/FEATURES.md)** - Complete technical documentation
