---
title: 'Aixgo - AI Agents in Pure Go'
description: 'Production-ready AI agent framework for Go developers. Build multi-agent systems with 6 LLM providers, enterprise security, and full observability.'
layout: 'page'
keywords: ['go ai agent framework', 'golang ai framework', 'ai agents golang', 'multi-agent systems', 'langchain alternative go']
---

<link rel="stylesheet" href="/css/alpha-notices.css">

<div class="hero-section-wrapper">

# The Future of AI Agents is Go

<p class="hero-subheadline">Build production AI systems as single binaries. 6 LLM providers, enterprise security, full observability.</p>

<div class="hero-ctas">

{{< button href="https://github.com/aixgo-dev/aixgo#quick-start" target="_blank" class="cta-primary cta-massive" >}} Get Started → {{< /button >}}

</div>

</div>

---

<h2 class="section-title-reduced">From Prototype to Planet-Scale in 60 Seconds</h2>

<div class="code-section">
<div class="code-label">Install</div>

```bash
go get github.com/aixgo-dev/aixgo
```

</div>

<div class="code-section">
<div class="code-label">Create config/agents.yaml</div>

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

</div>

<div class="code-section">
<div class="code-label">Create main.go</div>

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

</div>

<div class="code-section">
<div class="code-label">Deploy anywhere</div>

```bash
# Local development
go run main.go

# Production - single <20MB binary
go build -o agent
./agent

# Edge, Lambda, Cloud Run, Kubernetes - one binary, zero configuration
```

</div>

<div style="text-align: center; margin: 3rem 0;">

{{< button href="https://github.com/aixgo-dev/aixgo" target="_blank" >}} View Full Documentation → {{< /button >}}

</div>

---

<div class="comparison-table-section">

<h2 class="section-title-reduced">Why the Industry is Moving to Go</h2>

<p class="comparison-intro">Python dominated AI because it was easy to prototype. Go will dominate production because it's built to ship. Read our [Philosophy](/why-aixgo) to understand why production AI deserves production tooling.</p>

<div class="comparison-cards">

<div class="comparison-card">
<h3>Container Size</h3>
<div class="comparison-item">
<span class="comparison-label">Python frameworks</span>
<span class="comparison-value">1.2GB</span>
<span class="comparison-impact">with dependencies</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Aixgo</span>
<span class="comparison-value"><20MB</span>
<span class="comparison-impact">single binary</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Impact</span>
<span class="comparison-impact">Deploy to edge devices, serverless, anywhere</span>
</div>
</div>

<div class="comparison-card">
<h3>Startup Performance</h3>
<div class="comparison-item">
<span class="comparison-label">Python frameworks</span>
<span class="comparison-value">30-45s</span>
<span class="comparison-impact">cold start</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Aixgo</span>
<span class="comparison-value">&lt;100ms</span>
<span class="comparison-impact">instant startup</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Impact</span>
<span class="comparison-impact">True serverless viability, real-time response</span>
</div>
</div>

<div class="comparison-card">
<h3>Runtime Safety</h3>
<div class="comparison-item">
<span class="comparison-label">Python frameworks</span>
<span class="comparison-value">Runtime</span>
<span class="comparison-impact">Discover errors in production</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Aixgo</span>
<span class="comparison-value">Compile-time</span>
<span class="comparison-impact">Compiler catches errors before deploy</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Impact</span>
<span class="comparison-impact">Ship with confidence, sleep at night</span>
</div>
</div>

<div class="comparison-card">
<h3>LLM Data Validation</h3>
<div class="comparison-item">
<span class="comparison-label">Python + Pydantic</span>
<span class="comparison-value">Runtime only</span>
<span class="comparison-impact">Type changes found in production</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Aixgo</span>
<span class="comparison-value">Compile-time</span>
<span class="comparison-impact">Type changes caught before deploy, auto-retry on LLM errors</span>
</div>
<div class="comparison-item">
<span class="comparison-label">Impact</span>
<span class="comparison-impact">Refactor with confidence, LLM errors auto-recover</span>
</div>
</div>

</div>

</div>

---

<div id="feature-status" class="alpha-status-section">

## Production-Ready Features

Aixgo provides a comprehensive feature set for building production AI agent systems.

### LLM Providers (6 Cloud + Local Available)

- **OpenAI** - Chat, streaming SSE, function calling, JSON mode
- **Anthropic (Claude)** - Messages API, streaming SSE, tool use
- **Google Gemini** - GenerateContent API, streaming SSE, function calling
- **X.AI (Grok)** - Chat, streaming SSE, function calling (OpenAI-compatible)
- **Vertex AI** - Google Cloud AI Platform, streaming SSE, function calling
- **HuggingFace** - Free Inference API, cloud backends
- **Ollama** - Local models (phi, llama, mistral, gemma), zero API costs, enterprise security

### Security (Enterprise-Grade)

- **Authentication Framework** - 4 modes: disabled, delegated, builtin, hybrid
- **RBAC Authorization** - Role-based access control
- **Rate Limiting** - Token bucket, per-tool/per-user limits
- **Prompt Injection Protection** - 5 categories, 25+ detection patterns
- **TLS/mTLS Support** - Secure communications
- **Audit Logging** - Elasticsearch, Splunk HEC, Webhook backends
- **Safe YAML Parser** - Secure configuration handling

### Agent System & Orchestration

- **Six Agent Types** - Producer, ReAct, Logger, Classifier, Aggregator, Planner
- **Classifier Strategies** - ZeroShot, FewShot, MultiLabel, SingleLabel
- **Aggregator Strategies** - Consensus, Weighted, Semantic, Hierarchical, RAG
- **Planner Strategies** - Chain-of-Thought, Tree-of-Thought, ReAct, MonteCarlo, Backward Chaining, Hierarchical
- **Supervisor Orchestration** - Multi-agent coordination
- **Patterns** - Parallel, Sequential, Reflection, MapReduce
- **Workflow Engine** - Complex workflow orchestration
- **Persistence** - FileStore, MemoryStore, checkpoints

### MCP (Model Context Protocol)

- **Transports** - Local and gRPC
- **Service Discovery** - Static, DNS, Kubernetes, Consul
- **Cluster Coordination** - Load balancing, health checks, failover
- **Dynamic Tool Registration** - Runtime tool management

### Observability

- **OpenTelemetry** - OTLP export, distributed tracing
- **Langfuse Integration** - LLM-specific analytics via OTLP
- **Prometheus Metrics** - Production monitoring
- **Health Checks** - Liveness and readiness probes

### Deployment

- **Containers** - Dockerfile, Docker Compose
- **Cloud** - Cloud Run, Kubernetes (Kustomize)
- **CI/CD** - GitHub Actions workflows
- **Testing** - Unit tests, E2E tests, benchmarking, linting

### Planned Features

- **Vision Support** - Anthropic multimodal (planned)
- **Kubernetes Operator** - Native K8s orchestration (planned)
- **Terraform IaC** - Infrastructure as code (planned)

{{< button href="https://github.com/aixgo-dev/aixgo/blob/main/ROADMAP.md" target="_blank" >}} View Full Roadmap → {{< /button >}}

</div>

---

## The Go Advantage

| What Matters in Production | Python Frameworks             | Aixgo                          |
| -------------------------- | ----------------------------- | ------------------------------ |
| **Deploy Anywhere**        | 1GB+ containers, complex deps | <20MB binary, zero deps          |
| **Cold Start Speed**       | 10-45 seconds                 | <100ms                         |
| **Type Safety**            | Runtime discovery             | Compile-time guarantees        |
| **Concurrency**            | GIL bottleneck                | Native parallelism             |
| **Scaling Pattern**        | Rewrite for distribution      | Same code, local → distributed |
| **Security Surface**       | 200+ dependencies             | ~10 vetted packages            |
| **Memory Efficiency**      | GC pressure, leaks common     | Predictable, efficient GC      |
| **Operational Cost**       | High compute overhead         | 60-70% infrastructure savings  |

---

<div class="features-section">

<h2 class="section-title-reduced">Built for What's Next</h2>

<p class="features-intro">Aixgo isn't just another framework. It's the foundation for the next generation of AI systems:</p>

<div class="feature-item">
<h3>Multi-Agent Orchestration {{< status-badge status="available" >}}</h3>
<p>Coordinate specialized agents with built-in supervisor patterns. Six agent types (Producer, ReAct, Logger, Classifier, Aggregator, Planner) with multiple strategies. Support for parallel/sequential/reflection/MapReduce workflows.</p>
</div>

<div class="feature-item">
<h3>Edge to Cloud, Seamlessly {{< status-badge status="available" >}}</h3>
<p>Deploy the same <20MB binary everywhere - edge devices, serverless, Kubernetes. Local transport and gRPC for distributed systems.</p>
</div>

<div class="feature-item">
<h3>Observable by Default {{< status-badge status="available" >}}</h3>
<p>OpenTelemetry with OTLP export, Langfuse integration, Prometheus metrics, and health checks built-in.</p>
</div>

<div class="feature-item">
<h3>Vector Databases & RAG {{< status-badge status="available" >}}</h3>
<p>Build production-ready RAG systems with semantic search. Collection-based architecture with Firestore, memory stores, and extensible provider support. <a href="/guides/vector-databases">Learn more →</a></p>
</div>

<div class="feature-item">
<h3>LLM Provider Freedom {{< status-badge status="available" >}}</h3>
<p>6 providers with streaming: OpenAI, Anthropic, Google Gemini, X.AI Grok, Vertex AI, HuggingFace. Switch providers with config changes.</p>
</div>

<div class="feature-item">
<h3>Enterprise Security {{< status-badge status="available" >}}</h3>
<p>Authentication (4 modes), RBAC, rate limiting, prompt injection protection, TLS/mTLS, and audit logging.</p>
</div>

</div>

---

<div class="timeline-section">

<h2 class="section-title-reduced">Roadmap</h2>

<div class="timeline">

<div class="timeline-item">
<div class="timeline-period">Planned</div>
<div class="timeline-content">
<ul>
<li>Vision support for Anthropic multimodal</li>
<li>Kubernetes operator for agent orchestration</li>
<li>Terraform IaC modules</li>
<li>Vector database integrations (pgvector, Qdrant)</li>
</ul>
</div>
</div>

<div class="timeline-item">
<div class="timeline-period">Future</div>
<div class="timeline-content">
<ul>
<li>Agent federation across organizations</li>
<li>Formal verification tooling</li>
<li>WASM target for browser agents</li>
<li>GPU acceleration support</li>
</ul>
</div>
</div>

</div>

</div>

---

## Get Started

The AI infrastructure built on Python is reaching its limits. Aixgo is what comes next.

{{< button href="https://github.com/aixgo-dev/aixgo#quick-start" target="_blank" class="cta-primary cta-massive" >}} Get Started → {{< /button >}}

**MIT Licensed** · Open Source · Community Driven

[GitHub](https://github.com/aixgo-dev/aixgo) · [Documentation](https://github.com/aixgo-dev/aixgo#readme) · [Discussions](https://github.com/aixgo-dev/aixgo/discussions) ·
[Roadmap](https://github.com/aixgo-dev/aixgo/issues)
