# Aixgo — Product Requirements Document

**A production-grade AI agent framework for Go**

| | |
|---|---|
| **Project** | Aixgo |
| **Repository** | <https://github.com/aixgo-dev/aixgo> |
| **Website** | <https://aixgo.dev> |
| **Author** | Charles Green |
| **Version** | 0.1 |
| **Date** | May 2026 |
| **License** | MIT |
| **Sibling project** | [Aixgate](https://github.com/aixgo-dev/aixgate) — runtime sandbox for AI coding agents |

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [Goals and Non-Goals](#3-goals-and-non-goals)
4. [Users and Use Cases](#4-users-and-use-cases)
5. [Product Overview](#5-product-overview)
6. [Architecture](#6-architecture)
7. [Orchestration Patterns](#7-orchestration-patterns)
8. [LLM Provider Strategy](#8-llm-provider-strategy)
9. [Security Model](#9-security-model)
10. [Observability](#10-observability)
11. [Distribution and Packaging](#11-distribution-and-packaging)
12. [Relationship to Aixgate and the Wider Ecosystem](#12-relationship-to-aixgate-and-the-wider-ecosystem)
13. [API Stability and the v1.0 Commitment](#13-api-stability-and-the-v10-commitment)
14. [Roadmap](#14-roadmap)
15. [Success Metrics](#15-success-metrics)
16. [Risks and Open Questions](#16-risks-and-open-questions)
17. [Appendix](#17-appendix)

---

## 1. Executive Summary

Aixgo is an open-source framework for building production AI agent systems in Go. It provides the primitives — agents, orchestration patterns, LLM provider integrations, tool calling via the Model Context Protocol (MCP), observability, and security controls — needed to take a multi-agent system from a single-binary local prototype to a horizontally scaled distributed deployment without rewriting the application or changing the wire format.

Aixgo is designed for teams who run AI in production and care about the same properties they care about for the rest of their stack: small deployments, predictable startup, compile-time safety, native concurrency, and observability that does not require a research grant. It is written in Go 1.26+, distributed as a single binary under 20 MB, MIT-licensed, and developed in the open at <https://github.com/aixgo-dev/aixgo>.

### What ships in the box

- **6 agent types** — ReAct, Classifier, Aggregator, Planner, Producer, Logger
- **13 orchestration patterns** — Supervisor, Sequential, Parallel, Router, Swarm, Hierarchical, RAG, Reflection, Ensemble, Classifier, Aggregation, Planning, MapReduce
- **8+ LLM providers** — OpenAI, Anthropic, Google Gemini, xAI, Vertex AI, Amazon Bedrock, HuggingFace, plus local inference via Ollama and vLLM
- **MCP tool calling** over local, gRPC, and multi-server transports
- **Two runtimes** — local (Go channels) and distributed (gRPC), addressed identically by application code
- **Validation retry** for structured LLM outputs, improving reliability of structured generation by 40–70%
- **OpenTelemetry-native observability** with first-class integrations for Prometheus, Grafana, Datadog, Langfuse, and New Relic
- **Production security primitives** — authentication, rate limiting, SSRF protection, input sanitization, audit logging, prompt-injection defenses

### Why this exists

Python AI frameworks earned their position by being the fastest path from idea to demo. That position is well-deserved. The trade-off becomes visible at the operational boundary: container size, cold start, GIL-limited concurrency, and runtime type errors all show up in the production budget. Many teams build the prototype in Python and then rewrite for production in Go, Rust, or Java. Aixgo aims to make that second step unnecessary for teams whose production stack is already Go.

For the longer-form positioning argument, see <https://aixgo.dev/why-aixgo> and the philosophy notes at <https://aixgo.dev/philosophy>.

---

## 2. Problem Statement

### 2.1 The cost of Python at the production boundary

Python is the right tool for AI research, prototyping, and notebook work. Most modern LLM tooling is published for Python first, often Python only. Teams adopt the dominant ecosystem because that is rational. The problem is not the language; the problem is the gap between what a research framework optimizes for and what a production framework needs to optimize for.

When a Python AI service ships to production, several costs become structural rather than incidental:

- **Deployment footprint.** A `python:3.11`-based image with the typical AI dependency tree (LangChain or CrewAI, vector store client, embedding model client, observability stack) runs 1–1.5 GB. The application code is a small fraction of that.
- **Cold start.** Common Python AI services take 10–45 seconds from container start to ready. This is acceptable for long-lived services and a poor fit for serverless economics.
- **Concurrency.** The Global Interpreter Lock means that genuinely parallel multi-agent execution requires multiple processes, each carrying its own dependency tree.
- **Type discovery.** Python's dynamic typing surfaces interface mismatches between agents, tools, and providers at runtime — often at the request boundary, in production.
- **Dependency surface.** A dependency tree of 200+ transitive packages is a large attack surface for CVE management and supply-chain auditing.

These properties do not make Python wrong for AI work. They make it expensive when the deployment target is a serverless platform, an edge device, an air-gapped environment, a regulated workload, or a high-availability service that needs to scale to thousands of concurrent agents.

### 2.2 The status quo for Go developers

Go developers building AI features today have three practical options, none of them great:

1. **Add a Python service.** Run a sidecar Python service for the AI parts and call it over HTTP or gRPC from the Go application. This works but doubles the operational surface — two languages, two dependency trees, two deployment artifacts, two teams of on-call rotation.
2. **Hand-roll the integration.** Call provider APIs directly from Go and build orchestration, retry, observability, and tool-calling primitives in-house. This works for simple use cases and stops scaling around the time the first non-trivial pattern (Router, Reflection, RAG) is needed.
3. **Use a small experimental Go library.** A handful of Go AI libraries exist, but most are early, narrow in scope, lacking production primitives (auth, rate limiting, observability), or not actively maintained.

Aixgo is an attempt to fill the gap directly: a Go-native framework with the breadth of feature coverage of the major Python frameworks, designed for the operational profile Go developers expect.

### 2.3 Why now

The frontier of LLM capability is no longer the differentiator it was 18 months ago. Frontier models from multiple vendors are available behind comparable APIs, and most production agent value now comes from orchestration, retrieval, tool integration, and operational discipline — exactly the parts where production tooling matters more than the prototype experience. The market for "production-grade Go AI" is the natural complement to "research-grade Python AI," and it is currently underserved.

---

## 3. Goals and Non-Goals

### 3.1 Goals

**Primary goals for v1.0:**

1. **Be the obvious choice for Go teams shipping AI agents to production.** A Go backend team should be able to add a multi-agent feature to their existing service without introducing a Python runtime, a new container, or a new on-call rotation.
2. **Maintain feature parity with the major Python frameworks for production-relevant patterns.** That means: orchestration patterns, multi-provider LLM access, vector retrieval, tool calling via MCP, structured output validation, and conversation persistence. Research-only patterns are out of scope.
3. **Hold a credible v1 compatibility commitment.** YAML workflow definitions, public Go SDK APIs, and gRPC/MCP wire formats should be source-stable across all v1.x releases. See [§13](#13-api-stability-and-the-v10-commitment) and <https://aixgo.dev/v1-compatibility>.
4. **Ship operational primitives first-class, not as add-ons.** OpenTelemetry tracing, structured logging, metrics, health probes, rate limits, and authentication are part of the framework, not a project the user has to do themselves.
5. **Make the deployment story honest.** A `<20 MB` single binary, sub-100 ms cold start, and an explicit story for serverless, edge, and Kubernetes targets. No "works on my machine" caveats.

**Secondary goals:**

6. **Make distributed mode boring.** The same code that runs locally over Go channels should run across nodes over gRPC, with the only difference being the runtime selection at startup.
7. **Be observable by default.** Every agent call, every tool invocation, and every LLM round-trip should produce a span and structured log entry with no instrumentation code in user space.
8. **Be hostable.** Cloud Run and Kubernetes are first-tier deployment targets with documented templates. Edge and AWS Lambda are second-tier and on the v1.x roadmap.

### 3.2 Non-Goals

These are out of scope, and the project will say no to them:

- **Replacing Python for AI research.** Researchers should keep using Python. Aixgo will not chase the latest experimental orchestration paper unless it produces a production-relevant pattern.
- **Hosting LLMs.** Aixgo integrates with hosted providers (OpenAI, Anthropic, etc.) and local inference services (Ollama, vLLM). It does not ship its own inference engine.
- **Becoming a vector database.** Aixgo provides a vector store interface and reference implementations (Firestore, in-memory) and integrations with external stores. It does not aim to be the vector store.
- **Becoming a UI framework.** Aixgo has no opinion on what the front-end looks like. It exposes Go APIs and gRPC; the UI is the user's choice.
- **Becoming a notebook environment.** Aixgo is a server-side framework. There is no notebook integration on the roadmap.
- **Becoming a one-language-fits-all framework.** The target audience is teams whose production stack is Go. Aixgo is not trying to be the best agent framework for teams whose production stack is something else.

### 3.3 Tonal commitment

Aixgo's positioning is comparative — it exists because there is a real engineering trade-off between Python frameworks and a Go-native one. The project commits to making that comparison honestly and respectfully. The Python AI ecosystem is excellent at what it is for. Aixgo is excellent at something different. Marketing language that disparages alternatives or their communities is out of bounds.

---

## 4. Users and Use Cases

### 4.1 Primary users

**Backend engineers at Go-stack companies.** Teams whose production services are Go-based and who need to add AI capabilities — agents that summarize tickets, route customer requests, run RAG over an internal knowledge base, classify content, or orchestrate multi-step workflows. The prevailing alternative is a Python sidecar; Aixgo collapses that to a single language and a single binary.

**DevOps and platform teams.** Operators who need to deploy AI workloads to existing infrastructure (Kubernetes, Cloud Run, internal PaaS) and who care about container size, cold start, dependency auditing, and observability conventions. Aixgo's deployment profile (single binary, OTel-native, structured config) matches what they already operate.

**Data engineers building enrichment pipelines.** Teams building ETL/ELT flows that need an LLM step — extraction, classification, enrichment, summarization — and who want to embed the AI logic in the same Go pipeline that runs the data work, without spawning a Python subprocess per task.

**Enterprises with regulated or air-gapped requirements.** Organizations where the deployment environment forbids large dependency trees, requires reproducible builds, requires audit logging of every model call, or requires on-prem or sovereign-cloud deployment. Aixgo's MIT licensing, single-binary distribution, and built-in audit logging are designed for this profile.

### 4.2 Secondary users

- **AI engineers exploring Go.** Practitioners curious whether a Go-native framework can match their Python workflow's velocity. Aixgo's YAML workflow definitions and reference examples are designed to make this exploration short.
- **Embedded and edge developers.** Teams putting agents on resource-constrained devices where a 1 GB Python container is structurally impossible.
- **Open-source contributors.** Aixgo is an open project. Contribution paths are documented in [docs/CONTRIBUTING.md](CONTRIBUTING.md).

### 4.3 Representative use cases

| Use case | Pattern(s) | Why Aixgo |
|---|---|---|
| Customer support triage and routing | Classifier → Router → ReAct | Sub-100 ms cold start matters for first-touch latency; type-safe agent contracts matter for routing correctness |
| Documentation Q&A | RAG + Reranker | First-class RAG pattern with conversational, multi-query, and hybrid variants; multiple vector store backends |
| Multi-source research synthesis | Parallel + Aggregator | True parallel execution (no GIL); aggregator supports consensus, weighted, semantic, hierarchical, and RAG-based strategies |
| Long-running task automation | Planning + Sequential | Planner agent with multiple strategies (Tree-of-Thought, MCTS, backward chaining); workflow persistence and recovery |
| Cost optimization on hybrid model fleets | Router | Documented 25–50% cost reduction by routing simple queries to cheaper models without code changes |
| Compliance-grade content generation | Reflection + Audit logging | Self-critique loop with quality scoring; full audit trail for every model call |
| Code analysis and refactoring | ReAct + MCP tools | MCP integration for deterministic tool calling; type-safe tool registration |

---

## 5. Product Overview

Aixgo is structured as six layers, each with a stable contract and a clear scope. The complete surface is documented in [docs/FEATURES.md](FEATURES.md); this section is a strategic overview, not a feature list.

```
┌──────────────────────────────────────────────────────────────┐
│  Application                  YAML config, CLI, examples     │
├──────────────────────────────────────────────────────────────┤
│  Orchestration                Supervisor, 13 patterns        │
├──────────────────────────────────────────────────────────────┤
│  Agents                       6 agent types                  │
├──────────────────────────────────────────────────────────────┤
│  Runtime                      Local (channels) / gRPC        │
├──────────────────────────────────────────────────────────────┤
│  Integration                  LLM providers, MCP, vectors    │
├──────────────────────────────────────────────────────────────┤
│  Observability & Security     OTel, auth, rate limit, audit  │
└──────────────────────────────────────────────────────────────┘
```

### 5.1 Layer summary

**Application.** The entry point for an Aixgo deployment is a YAML workflow file plus a small `main.go` (or a `go run` against a CLI binary). Workflow YAML is the primary surface and is covered by the v1 compatibility promise. Hand-written Go code that uses the SDK directly is supported but optional.

**Orchestration.** The supervisor coordinates agents and applies orchestration patterns. The 13 patterns shipped today are documented in [docs/PATTERNS.md](PATTERNS.md). Each pattern is implemented as a real, tested orchestrator — not a string template — and is selectable via configuration.

**Agents.** Six agent types cover the production cases Aixgo cares about: ReAct (LLM + tool calling), Classifier (intent routing), Aggregator (multi-source synthesis), Planner (task decomposition), Producer (timed message generation), and Logger (sink). New agent types are added in `agents/` and registered through a factory; the registry is open for community contributions.

**Runtime.** The local runtime uses Go channels for in-process message passing and is the default for single-binary deployments. The distributed runtime uses gRPC for cross-node communication and is the default for horizontal scaling. Application code is identical across the two; the difference is one configuration choice.

**Integration.** Three integration surfaces — LLM providers (auto-dispatched by model prefix), MCP for tool calling (local, gRPC, multi-server), and vector stores (Firestore, in-memory, with more on the roadmap). Each surface has a stable interface and a registry pattern for extension.

**Observability and security.** OpenTelemetry is the observability spine. All agent calls produce spans; structured logs follow conventional field names; metrics are exported via Prometheus format. Security primitives — authentication, rate limiting, SSRF protection, input sanitization, audit logging, prompt-injection defenses — are part of the framework and active by default in production mode.

For the complete authoritative feature catalog (200+ features across these layers), see [docs/FEATURES.md](FEATURES.md).

---

## 6. Architecture

This section covers the architectural choices that distinguish Aixgo from alternative designs. For the implementation map (which file lives where, which package does what), see the [Architecture section in CLAUDE.md](../CLAUDE.md#architecture).

### 6.1 Single binary, two runtimes

A common pitfall in distributed agent frameworks is that "local mode" and "distributed mode" have different application code. Aixgo deliberately treats both as transports of the same runtime contract: a runtime knows how to `Call(ctx, agentName, msg)` and `CallParallel(ctx, agentNames, msg)`, regardless of whether the call resolves to a Go channel send or an outbound gRPC RPC.

This means a system can be developed and tested locally as a single binary, then deployed as a multi-node cluster by changing the runtime selection in configuration. The application code, the YAML workflow, and the agent implementations are unchanged.

### 6.2 YAML as the primary surface

The decision to make YAML — not a Go DSL — the primary user-facing surface is intentional. YAML is reviewable in pull requests, deployable without rebuilding the binary, shareable across teams, and accessible to non-Go users. Aixgo's YAML schema is covered by the v1 compatibility promise; the Go SDK underneath is also stable, but the framework treats SDK use as an advanced path, not the default.

### 6.3 Provider auto-dispatch

LLM providers are selected by model prefix at config load time:

| Prefix | Provider |
|---|---|
| `gpt-*` | OpenAI |
| `claude-*` | Anthropic (direct API) |
| `gemini-*` | Google Gemini |
| `grok-*`, `xai-*` | xAI |
| `bedrock/`, `anthropic.`, `amazon.`, `meta.`, `mistral.`, `cohere.`, `ai21.` | Amazon Bedrock |
| `meta-llama/*`, `mistralai/*` | HuggingFace |
| (configured base URL) | Ollama, vLLM, generic OpenAI-compatible inference |

The dispatch rule is a stable interface; new providers are added without changing user-visible configuration.

### 6.4 MCP for tool calling

Aixgo speaks the Model Context Protocol natively. Tools can be registered locally, exposed over gRPC for remote invocation, or aggregated from multiple MCP servers. This means a tool implemented for Aixgo also works with any other MCP-compatible client (Claude Desktop, Cursor, Aider, etc.), and conversely Aixgo can call any MCP-compatible server.

### 6.5 Validation retry

Structured LLM outputs (JSON, tool calls) are passed through a validator before being returned to the caller. If the model produces invalid output, Aixgo retries with the validation error appended to the prompt, up to a configurable bound. In benchmarks, this raises the success rate of structured generation by 40–70% versus single-shot calls.

### 6.6 Conversation persistence

Sessions are stored via a pluggable backend (JSONL file, Redis). The session abstraction handles conversation history, working memory, and checkpoint resumption. Backends are plug-replaceable; PostgreSQL is on the v0.4.x roadmap.

For deeper architectural detail on a per-component basis, see [docs/FEATURES.md](FEATURES.md), [docs/PATTERNS.md](PATTERNS.md), [docs/SESSIONS.md](SESSIONS.md), and [docs/OBSERVABILITY.md](OBSERVABILITY.md).

---

## 7. Orchestration Patterns

Aixgo ships 13 orchestration patterns, all production-implemented and tested. The patterns are listed below with their primary use case; full documentation including code examples, configuration, performance characteristics, and decision-tree guidance lives in [docs/PATTERNS.md](PATTERNS.md).

| Pattern | Primary use |
|---|---|
| Supervisor | Centralized hub-and-spoke coordination |
| Sequential | Ordered pipeline execution |
| Parallel | Concurrent multi-agent processing (3–4× speedup) |
| Router | Intelligent model routing (25–50% cost savings) |
| Swarm | Decentralized agent handoffs |
| Hierarchical | Multi-level delegation |
| RAG | Retrieval-augmented generation (~70% token reduction) |
| Reflection | Self-critique and refinement (20–50% quality improvement) |
| Ensemble | Multi-model voting (25–50% error reduction) |
| Classifier | Intent-based routing |
| Aggregation | Multi-agent synthesis |
| Planning | Dynamic task decomposition |
| MapReduce | Distributed batch processing |

Two further patterns are on the v2.x research roadmap: Debate and Nested/Composite. Neither is required for v1.0.

---

## 8. LLM Provider Strategy

### 8.1 Multi-provider as a first-class commitment

Vendor lock-in to a single LLM provider is a structural risk for any production AI system. Aixgo's multi-provider design treats provider choice as a deployment-time concern, not a code-time concern. Switching from `gpt-4-turbo` to `claude-sonnet-4-6` is a YAML edit, not a code change.

### 8.2 Provider tiers

**Tier 1 — direct provider APIs.** OpenAI, Anthropic (direct), Google Gemini, xAI, Vertex AI, Amazon Bedrock, HuggingFace. These are first-class, with full feature coverage including streaming, tool calling, and structured output where the provider supports them.

**Tier 2 — local inference services.** Ollama and vLLM via OpenAI-compatible endpoints. These enable on-prem, air-gapped, and zero-cost-per-token deployments. SSRF protection is built into the inference client.

**Tier 3 — community-contributed.** New providers can be added by implementing the `Provider` interface and registering at `init()` time. The registry is open and documented in CLAUDE.md's "Add New LLM Provider" section.

### 8.3 What the provider abstraction guarantees

- Auto-dispatch by model prefix (no code changes to switch providers)
- Standardized error wrapping (provider-specific errors are normalized)
- Retry with exponential backoff on transient failures
- Cost tracking per call, per agent, per session
- Tracing spans for every LLM call
- SSRF protection for any provider with a configurable base URL

### 8.4 What the provider abstraction does not guarantee

- Feature parity across providers (e.g. not every provider supports tool calling; not every provider streams)
- Identical pricing models or rate-limit behavior
- Identical latency characteristics

These are properties of the providers themselves; Aixgo surfaces them honestly through configuration and documentation rather than papering over them.

---

## 9. Security Model

Security is a layered concern in Aixgo. The full developer guidance is in [docs/SECURITY_BEST_PRACTICES.md](SECURITY_BEST_PRACTICES.md); this section summarizes the threat model and the framework's response.

### 9.1 Threat model

Aixgo is built to operate safely in environments where the following are true:

- The agent receives input from untrusted sources (end-user prompts, external API responses, scraped content).
- The agent invokes external tools and services that can have side effects.
- The agent may be deployed multi-tenant, with isolation expected between tenants.
- The deployment environment is subject to compliance regimes (SOC 2, ISO 27001, financial regulation) that require auditable access controls.

The model does **not** assume the LLM itself is trusted. Every output from a model is treated as untrusted input to downstream code.

### 9.2 Built-in defenses

- **Authentication.** Multiple modes (API key, JWT with JWKS verification, IAP). Production mode disables the no-auth bypass.
- **Authorization.** Role-based access control on agent invocation and tool calls.
- **Rate limiting.** Per-API-key, per-agent, per-tool limits. Backed by in-memory or Redis stores.
- **Input sanitization.** Length limits, character filtering, structured-output validation, prompt-injection detection.
- **SSRF protection.** Outbound HTTP from any user-configurable URL is checked against host allowlists, denies private/metadata IPs, denies redirects to untrusted hosts. Used by inference clients, JWK fetching, and any user-defined HTTP tool.
- **Audit logging.** Every agent call, tool invocation, and security-relevant event is logged in structured form. Compatible with SIEM integration.
- **YAML safety.** YAML configuration is parsed through a hardened parser with size, depth, key, and alias limits to prevent billion-laughs and resource-exhaustion attacks.
- **Container hardening.** Distributed Docker images run as non-root (UID 65534), use SHA-pinned base images, drop NET_RAW, and pass kube-linter against rendered Kubernetes overlays.

### 9.3 What Aixgo does not protect against

- Vulnerabilities in user-supplied agent prompts (the framework cannot enforce that a developer's prompts are safe; it provides defenses, not magic).
- Vulnerabilities in third-party MCP servers (the framework verifies the connection but cannot audit the server's logic).
- Compromise of an LLM provider's API.
- Operator-level threats — root on the host machine remains root on the host machine.

For the on-machine sandbox layer, see [Aixgate](https://github.com/aixgo-dev/aixgate), which is the sibling project responsible for filesystem-level deny-by-default policy enforcement at the OS boundary.

---

## 10. Observability

Aixgo is observable by default. The full integration guide is in [docs/OBSERVABILITY.md](OBSERVABILITY.md); this section states the policy.

- **OpenTelemetry is the spine.** Every agent invocation, tool call, and LLM round-trip emits a span. Traces propagate across the local and distributed runtimes uniformly.
- **Structured logging is the default.** No `printf` debugging in framework code; every event has a structured shape with conventional field names.
- **Metrics are exported in Prometheus format.** Agent counts, request rates, error rates, latency histograms, and cost-per-call are first-class metrics.
- **Health probes are HTTP endpoints.** `/health`, `/health/live`, `/health/ready` follow Kubernetes conventions out of the box.
- **First-class integrations.** Prometheus, Grafana, Datadog, Langfuse, New Relic. Configuration over code.
- **Cost tracking.** Per-call, per-agent, per-session cost accumulation with provider-specific pricing tables.

Instrumentation is configuration, not code. A user does not write `tracer.Start()` calls — they set `OTEL_EXPORTER_OTLP_ENDPOINT` and the framework does the rest.

---

## 11. Distribution and Packaging

### 11.1 Distribution channels

- **Source.** `go get github.com/aixgo-dev/aixgo` (the canonical install for SDK users).
- **Binary releases.** GitHub Releases with GoReleaser-built binaries for linux-amd64, darwin-amd64, darwin-arm64, windows-amd64. Signed checksums.
- **Container images.** OCI images at `ghcr.io/aixgo-dev/aixgo` with SHA-pinned base images, non-root runtime, and SBOM attestation.
- **Kubernetes templates.** Kustomize overlays for staging and production at `deploy/k8s/overlays/`. Lint-clean against kube-linter.
- **Cloud Run templates.** A `make deploy-cloudrun GCP_PROJECT_ID=…` target plus IAP integration documented in `examples/cloudrun-iap`.

### 11.2 Versioning

The single source of truth for the current version is [GitHub Releases](https://github.com/aixgo-dev/aixgo/releases/latest). The git tag is canonical; binary releases, container tags, and Go module versions follow.

### 11.3 Release cadence

- **Patch releases (`v0.x.y`)** ship as needed for security fixes, dependency bumps, and bug fixes. Automated via Dependabot for routine bumps.
- **Minor releases (`v0.x.0`)** carry features and breaking changes during the v0.x phase. Each minor release has a release blog post on aixgo.dev.
- **v1.0** will lock the API surface per [§13](#13-api-stability-and-the-v10-commitment).

### 11.4 Supply-chain posture

- Dependabot-managed dependency updates, grouped by ecosystem.
- govulncheck on every PR against the main module and every example module.
- gosec static analysis on every PR.
- Trivy filesystem scan on every PR.
- Image base layers SHA-pinned.
- GitHub Actions SHA-pinned.

---

## 12. Relationship to Aixgate and the Wider Ecosystem

The aixgo-dev organization publishes two complementary products. Each has its own repository, release cadence, and PRD; this section is the canonical place to read about how they fit together.

> **aixgo.dev builds agents. Aixgate keeps them in their lane.**

### 12.1 Aixgo

The framework documented here. Builds agents. Repository: <https://github.com/aixgo-dev/aixgo>. Website: <https://aixgo.dev>. PRD: this document.

### 12.2 Aixgate

A runtime sandbox for AI coding agents — Claude Code, Cursor, Aider, OpenAI Codex agents, and any other process that reads or writes files on behalf of an LLM. Aixgate enforces deny-by-default filesystem access policies at the OS boundary, so sensitive files (`.env`, cloud credentials, SSH keys) are never exposed to an agent unless an explicit policy rule permits it.

- Repository: <https://github.com/aixgo-dev/aixgate>
- PRD: <https://github.com/aixgo-dev/aixgate/blob/main/docs/PRD.md>

Aixgate is **not** a dependency of Aixgo, and Aixgo is **not** a dependency of Aixgate. They share an organization, an open-source license, a tonal commitment, and the goal of making production AI safer and more boring. They are otherwise independent projects with independent versioning.

### 12.3 When to use which

| You need to … | Use |
|---|---|
| Build a multi-agent system that runs in production | Aixgo |
| Run a coding agent on your laptop without it reading your `.env` | Aixgate |
| Run agents in a Go service with full observability | Aixgo |
| Apply a portable policy to whatever AI coding tool your team uses | Aixgate |
| Integrate LLM provider access into a Go data pipeline | Aixgo |
| Audit every filesystem access made by an agent | Aixgate |

### 12.4 History and decision record

Aixgate began life as `Warden`, a sub-component of the aixgo monorepo. It was renamed in [#197](https://github.com/aixgo-dev/aixgo/pull/197) and extracted to its own repository in [#206](https://github.com/aixgo-dev/aixgo/pull/206). The reasoning is recorded in [ADR 0002](adr/0002-aixgate-separate-repo.md) (which supersedes the earlier monorepo proposal in [ADR 0001](adr/0001-aixgate-monorepo.md)). The web site source is similarly extracted to <https://github.com/aixgo-dev/web>.

The aixgo monorepo is therefore deliberately scoped to the framework and its examples. Sister repositories handle adjacent concerns.

---

## 13. API Stability and the v1.0 Commitment

The full compatibility promise lives at <https://aixgo.dev/v1-compatibility> and is the canonical document. This section restates the headline commitment and the scope.

### 13.1 What v1 will guarantee

When Aixgo reaches v1.0:

- **YAML workflow configurations** written for v1.0 will execute correctly on all v1.x releases without modification.
- **Public Go SDK APIs** in the `github.com/aixgo-dev/aixgo` module will maintain source-level compatibility.
- **gRPC and MCP protocol wire formats** will remain backward-compatible.

### 13.2 What is in scope

- All documented YAML schema elements
- All exported symbols in the root `aixgo` package and the public `pkg/...` packages
- All gRPC service definitions and message types
- The MCP transport contracts

### 13.3 What is out of scope

- `internal/...` packages (these are intentionally not part of the SDK)
- Behavior governed by experimental feature flags
- Upstream provider API changes (Aixgo will adapt, but the framework cannot guarantee what OpenAI, Anthropic, etc., do with their APIs)
- Performance characteristics (the framework reserves the right to make any v1.x version faster)

### 13.4 Pre-v1 (where we are today)

The project is currently in the v0.x phase, where breaking changes are possible and are documented in release notes. The v1.0 compatibility promise activates at v1.0, not before. Production users running on v0.x should pin to specific minor versions and read release notes carefully.

---

## 14. Roadmap

The single source of truth for current and historical releases is [GitHub Releases](https://github.com/aixgo-dev/aixgo/releases). The forward-looking roadmap below is indicative and is governed by what ships, not what is planned.

### 14.1 Near-term (v0.4.x)

Production hardening of the existing surface:

- PostgreSQL session backend (#98)
- Conversation branching and tree navigation (#99)
- Long-term memory and semantic retrieval (#31)
- Additional vector store backends (Qdrant)
- Continued security and observability polish

### 14.2 Mid-term (v0.5.x – v0.9.x)

The path to v1.0:

- API stability work and the formal v1 compatibility audit (#100)
- Multi-modal support (vision, audio, documents) (#32)
- Kubernetes operator for agent orchestration
- Crash recovery and multi-region support (#34)
- Infrastructure-as-code modules (Terraform/OpenTofu)

### 14.3 Long-term (v1.0 and beyond)

- v1.0: lock the API surface and activate the compatibility promise
- v1.x: AWS Lambda packaging strategy
- v1.x: agent template library for common patterns (#33)
- v2.x: research patterns (Debate, Nested/Composite)

The current release status is always available at <https://github.com/aixgo-dev/aixgo/releases/latest>.

---

## 15. Success Metrics

The project's success is measured against signals that reflect actual production adoption, not vanity numbers.

### 15.1 Engineering signals

- **Time to first production deployment.** A new user should reach a working binary in production in under 30 minutes from `go get`. Tracked via developer feedback, blog tutorials, and example completion.
- **Cold start.** Sub-100 ms p50 cold start on standard cloud-runner hardware, holding across releases.
- **Binary size.** Below 20 MB for the main `aixgo` binary, holding across releases.
- **Test coverage.** 80%+ across core packages. Currently at this threshold; not allowed to regress.
- **Security CI cleanliness.** Zero open critical or high findings on the main branch from gosec, Trivy, or govulncheck. Currently green; not allowed to regress.

### 15.2 Adoption signals

- **GitHub stars and forks.** Trailing indicator, weakly correlated with serious adoption.
- **Production deployments.** Tracked via opt-in user surveys and case studies. The bar is "name a company running aixgo in production" rather than "stars."
- **Community contributions.** External contributors landing PRs against the main repo. Stub-issue triage, dependency bumps, and docs fixes count; design contributions count more.
- **Provider and pattern coverage.** Number of LLM providers and orchestration patterns added by the community vs. the maintainer.

### 15.3 Operational signals

- **Release cadence.** Steady, predictable patch releases. Dependabot keeping the dependency tree current.
- **Issue lifecycle.** Open issues triaged within a week. Stale-stub triage performed quarterly so the project board reflects reality.
- **CI green-rate on `main`.** Above 95%. Failures are addressed immediately, not allowed to accumulate.

### 15.4 What success is not

The project deliberately does not target raw GitHub star count, Hacker News front-page placements, or "killing" any other framework. Aixgo's success is measured by whether Go teams ship production AI on it.

---

## 16. Risks and Open Questions

This section is honest about what could go wrong and what isn't yet decided. Public PRDs that omit this section tend to age badly.

### 16.1 Strategic risks

**Python ecosystem velocity.** New orchestration patterns and provider integrations land in Python first. Aixgo will always be following on capability discovery. Mitigation: focus on production-relevant patterns, not research patterns; let Python lead on research.

**LLM provider consolidation.** If the LLM market consolidates around 1–2 providers, the multi-provider story becomes less differentiating. Mitigation: multi-provider remains a hedge against vendor risk regardless of market share, and the consolidation seems unlikely on a 2–3-year horizon.

**Vendor framework competition.** OpenAI, Anthropic, and others publish their own agent frameworks. Aixgo's value is in being vendor-neutral and Go-native; if a vendor publishes a Go-native framework, the Aixgo position narrows. Mitigation: stay vendor-neutral, ship better operational primitives, prioritize the v1 compatibility commitment.

### 16.2 Technical risks

**Dependency footprint creep.** The Go ecosystem is generally light, but LLM provider clients (especially the AWS SDK and Google API client) are large. Mitigation: keep heavy clients optional via build tags where practical; track binary size as a regression metric.

**Distributed-mode complexity.** gRPC distributed mode has more moving parts than local mode. Mitigation: distributed mode is opt-in; local mode remains the default; integration tests cover the distributed path.

**Compatibility commitment cost.** v1 compatibility is a real engineering constraint and will slow some refactors. Mitigation: ship v1 only when the surface is genuinely stable; use the v0.x phase to make all the breaking changes that need to be made.

### 16.3 Operational risks

**Sole-maintainer fragility.** The project currently has a small maintainer footprint. Mitigation: code is open, documented, and licensed permissively; contribution paths are documented; design decisions are recorded in ADRs so a future maintainer can reconstruct the rationale.

**Doc drift.** A framework with this surface area can develop a gap between code and documentation. Mitigation: `docs/FEATURES.md` is mandatory-update on feature changes per CLAUDE.md; PR template requires docs review.

### 16.4 Open questions

- **Should v1.0 lock the YAML schema or the SDK first?** Both are stable today, but if a divergence becomes necessary, which is the harder commitment to break?
- **What is the right boundary between Aixgo and Aixgate?** Currently they share nothing in code. Should they share an audit-log format? A policy primitives package? Or stay independent?
- **How aggressively should community-contributed providers and patterns be promoted to first-class?** A mature framework has a clear bar; that bar is not yet documented.
- **What's the right approach for benchmarks and comparative numbers?** Microbenchmarks are easy to game; production benchmarks are hard to publish. The current approach (publish ranges grounded in real deployments, refuse to publish numbers we can't defend) is honest but slow.

These are tracked as design discussions on GitHub Discussions: <https://github.com/orgs/aixgo-dev/discussions>.

---

## 17. Appendix

### 17.1 Terminology

- **Agent.** A unit of LLM-powered behavior with a defined input and output contract.
- **Orchestration pattern.** A reusable composition of agents that solves a generic class of problems (e.g. RAG, Reflection, Router).
- **Supervisor.** The orchestrator that coordinates agents within a workflow.
- **MCP (Model Context Protocol).** Anthropic's open protocol for tool calling between LLM applications and tool servers.
- **Validation retry.** The mechanism for re-prompting an LLM with the validation error of its prior attempt, used to improve structured-output reliability.
- **Local runtime.** In-process message passing via Go channels.
- **Distributed runtime.** Cross-node message passing via gRPC.

### 17.2 Canonical references

- **Repository:** <https://github.com/aixgo-dev/aixgo>
- **Website:** <https://aixgo.dev>
- **Go package documentation:** <https://pkg.go.dev/github.com/aixgo-dev/aixgo>
- **Releases:** <https://github.com/aixgo-dev/aixgo/releases/latest>
- **Discussions:** <https://github.com/orgs/aixgo-dev/discussions>
- **Sibling project (Aixgate):** <https://github.com/aixgo-dev/aixgate>
- **Web site source:** <https://github.com/aixgo-dev/web>

### 17.3 Internal documentation

- [README.md](../README.md) — quick start
- [CLAUDE.md](../CLAUDE.md) — implementation map
- [docs/FEATURES.md](FEATURES.md) — authoritative feature catalog (200+ features)
- [docs/PATTERNS.md](PATTERNS.md) — orchestration pattern reference
- [docs/SECURITY_BEST_PRACTICES.md](SECURITY_BEST_PRACTICES.md) — security guidance
- [docs/OBSERVABILITY.md](OBSERVABILITY.md) — observability setup
- [docs/SESSIONS.md](SESSIONS.md) — session persistence
- [docs/TESTING_GUIDE.md](TESTING_GUIDE.md) — testing strategy
- [docs/DEPLOYMENT.md](DEPLOYMENT.md) — deployment guide
- [docs/CONTRIBUTING.md](CONTRIBUTING.md) — contribution guide

### 17.4 Architectural decision records

- [ADR 0001 — Aixgate in the monorepo (superseded)](adr/0001-aixgate-monorepo.md)
- [ADR 0002 — Aixgate in a separate repository](adr/0002-aixgate-separate-repo.md)

### 17.5 Document maintenance

This PRD is a living document. Material changes (new sections, scope changes, milestone shifts) land via pull request and are reviewed alongside the code that motivates them. Minor edits (typos, link updates, version bumps) can land directly. The document's authority is "what's on `main`"; older versions are recoverable from git history.

When editing this document, the rule is the same as for the rest of the project: **detailed, honest, and publicly respectful**. The goal is to give someone considering Aixgo enough to make an informed decision, not to talk them into anything.
