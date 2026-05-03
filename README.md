# aixgo

Production-grade AI agent framework for Go. Structured-output validation with automatic retry, type-safe orchestration, eight LLM providers, sub-20MB binary. No Python. No GIL. No 1GB containers.

[![Go Reference](https://pkg.go.dev/badge/github.com/aixgo-dev/aixgo.svg)](https://pkg.go.dev/github.com/aixgo-dev/aixgo)
[![Go Report Card](https://goreportcard.com/badge/github.com/aixgo-dev/aixgo)](https://goreportcard.com/report/github.com/aixgo-dev/aixgo)
[![CI](https://github.com/aixgo-dev/aixgo/actions/workflows/ci.yml/badge.svg)](https://github.com/aixgo-dev/aixgo/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aixgo-dev/aixgo)](https://go.dev/)
[![Latest Release](https://img.shields.io/github/v/release/aixgo-dev/aixgo?sort=semver)](https://github.com/aixgo-dev/aixgo/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/aixgo-dev/aixgo?style=social)](https://github.com/aixgo-dev/aixgo/stargazers)

> ★ **Star this repo** to follow releases.

---

## Why aixgo

Aixgo is a production-grade agent framework written in pure Go. It gives you structured output validation with automatic retry, six agent types, thirteen orchestration patterns, and eight LLM providers — all in a single sub-20MB binary that starts in under 100ms.

If you ship Go services and you're tired of dragging Python and a 1GB container along for an agent, this is for you.

| Metric | aixgo | Python frameworks |
|---|---|---|
| **Binary size** | <20MB | 1GB+ containers |
| **Cold start** | <100ms | 10–45s |
| **Concurrency** | True parallelism (no GIL) | GIL-limited |
| **Type safety** | Compile-time | Runtime errors |
| **Validation retry** | Built-in (structured-output) | Library-dependent |
| **Distribution** | Single binary | Container + interpreter |

---

## Quick start

```bash
go get github.com/aixgo-dev/aixgo
```

Define your agents in YAML:

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo

agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: "You are a data analyst. Analyze incoming data and return insights."
    inputs: [producer]
    outputs: [logger]

  - name: producer
    role: producer
    interval: 1s
    outputs: [analyzer]

  - name: logger
    role: logger
    inputs: [analyzer]
```

Run it from Go:

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

```bash
export OPENAI_API_KEY=sk-...
go run main.go
```

That's a multi-agent system with producer → analyzer → logger orchestrated by a supervisor, in fewer than 30 lines.

---

## Features at a glance

- **6 agent types** — ReAct, Classifier, Aggregator, Planner, Producer, Logger
- **13 orchestration patterns** — Supervisor, Sequential, Parallel, Router, Swarm, Hierarchical, RAG, Reflection, Ensemble, Classifier, Aggregation, Planning, MapReduce
- **8+ LLM providers** — OpenAI, Anthropic, Gemini, xAI, Vertex AI, Amazon Bedrock, HuggingFace, plus inference services (Ollama, vLLM)
- **Validation retry** — Structured output validation with automatic retry (40–70% improved reliability)
- **MCP support** — Model Context Protocol for tool calling (local, gRPC, multi-server)
- **Session persistence** — Built-in conversation memory, JSONL or Redis backends
- **Enterprise security** — 4 auth modes, RBAC, rate limiting, SSRF protection, hardening
- **Full observability** — OpenTelemetry, Prometheus, Langfuse, cost tracking

See [docs/FEATURES.md](docs/FEATURES.md) for the complete catalog with code references.

---

## Install

| User type | Command | What's included | Size |
|---|---|---|---|
| **Library user** | `go get github.com/aixgo-dev/aixgo` | Go source code only | ~2MB |
| **CLI user** | `go install github.com/aixgo-dev/aixgo/cmd/aixgo@latest` | Single executable binary | <20MB |
| **Pre-built binary** | [GitHub Releases](https://github.com/aixgo-dev/aixgo/releases/latest) | tar.gz / zip per platform | <20MB |
| **Contributor** | `git clone https://github.com/aixgo-dev/aixgo.git` | Full repo with examples and docs | ~10MB |

The CLI is available for Linux, macOS, and Windows (amd64 and arm64). After install, generate shell completion:

```bash
aixgo completion bash | sudo tee /etc/bash_completion.d/aixgo   # Linux
aixgo completion zsh > "${fpath[1]}/_aixgo"                     # Zsh
aixgo completion fish > ~/.config/fish/completions/aixgo.fish   # Fish
```

### Configure API keys

```bash
cp .env.example .env
# Required: at least one of these
export OPENAI_API_KEY=sk-...        # GPT models
export ANTHROPIC_API_KEY=sk-ant-... # Claude models
export XAI_API_KEY=xai-...          # Grok models
export HUGGINGFACE_API_KEY=hf_...   # HuggingFace models
```

The framework auto-detects the right key from the `model` name (`gpt-*` → OpenAI, `claude-*` → Anthropic, `grok-*` → xAI, etc.).

---

## Featured examples

Five examples worth reading first. All runnable; full set is in [examples/](examples/).

| Example | What it demonstrates |
|---|---|
| [validation-with-retry](examples/pydantic-style-validation/) | Structured output with automatic retry — the headline feature |
| [parallel-research](examples/parallel-research/) | Fan-out/fan-in over multiple LLMs with cost tracking |
| [rag-documentation](examples/rag-documentation/) | RAG over Markdown docs, end to end |
| [router-cost-optimization](examples/router-cost-optimization/) | Provider routing for 25–50% cost savings |
| [session-react](examples/session-react/) | ReAct agent with persistent multi-turn sessions |

---

## Architecture

Five layers, all pluggable:

- **Agent layer** — six specialised agent types, register your own via `agent.Register`.
- **Orchestration layer** — thirteen patterns implemented in `internal/supervisor/patterns/`.
- **Runtime layer** — local (Go channels) or distributed (gRPC). Single binary in either case.
- **Integration layer** — eight LLM providers, MCP tool calling, vector stores, embeddings.
- **Observability layer** — OpenTelemetry, Prometheus, Langfuse, cost tracking.

For deep architecture and pattern docs see [docs/PATTERNS.md](docs/PATTERNS.md).

---

## Documentation

- [aixgo.dev](https://aixgo.dev) — Comprehensive guides, blog, and reference.
- [pkg.go.dev](https://pkg.go.dev/github.com/aixgo-dev/aixgo) — Generated API reference.
- [docs/FEATURES.md](docs/FEATURES.md) — Authoritative feature catalog.
- [docs/PATTERNS.md](docs/PATTERNS.md) — Thirteen orchestration patterns with examples.
- [docs/SECURITY_BEST_PRACTICES.md](docs/SECURITY_BEST_PRACTICES.md) — Security guide.
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) — Cloud Run, Kubernetes, Docker.
- [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) — OpenTelemetry and cost tracking.
- [docs/SESSIONS.md](docs/SESSIONS.md) — Conversation memory and session persistence.

---

## Development

```bash
git clone https://github.com/aixgo-dev/aixgo.git
cd aixgo
make test     # tests with race detector
make lint     # golangci-lint
make build    # build the aixgo CLI binary
```

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for the full contributor guide. Good first issues are tagged on [GitHub Issues](https://github.com/aixgo-dev/aixgo/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22).

---

## Community

- [GitHub Discussions](https://github.com/aixgo-dev/aixgo/discussions) — Ask questions, share ideas.
- [GitHub Issues](https://github.com/aixgo-dev/aixgo/issues) — Report bugs, request features.
- [Roadmap](https://github.com/orgs/aixgo-dev/projects/1) — Track feature development.

---

## See also

- [aixgate](https://github.com/aixgo-dev/aixgate) — Deny-by-default sandbox for AI coding agents.
- [aixgo.dev](https://aixgo.dev) — Documentation, guides, and examples.

---

## License

[MIT](LICENSE).

> aixgo.dev builds agents. Aixgate keeps them in their lane.
