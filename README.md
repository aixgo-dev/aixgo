# aixgo

Vendor-neutral, config-driven multi-agent orchestration for Go. Define agent topologies in YAML, run them as a single sub-20MB binary, and swap between eight LLM providers without touching code.

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

Agents in Go stopped being a novelty. Google's ADK treats Go as first-class, Microsoft has a Go agent framework in public preview, and there is an official Go MCP SDK. The case for Go itself is settled: small static binaries, cold starts under 100ms, real parallelism. Every serious Go framework gives you that now.

aixgo occupies the ground those frameworks leave open.

**Vendor neutrality.** ADK is built around Vertex and Gemini. Microsoft's framework is built around Azure and Foundry. OpenAI and Anthropic ship their agent SDKs for Python and TypeScript only. aixgo puts eight LLM providers behind one interface and picks the provider from the model name, so moving an agent from `gpt-4-turbo` to a Claude or Grok model is a one-line YAML change.

**Declarative orchestration.** Other Go frameworks are SDKs: the topology lives in code. In aixgo, agent topology is configuration. You define agents, their wiring, and the orchestration pattern in YAML, review it in a pull request, and ship it the way you ship any other config. Teams that live in Kubernetes manifests and Terraform plans tend to feel at home.

**Built for the people on call.** Auth, rate limiting, SSRF protection, input sanitization, health checks, OpenTelemetry, and cost tracking ship in the box. The design assumption is that someone has to run this in production, not just demo it.

| | aixgo | Cloud-vendor Go SDKs | Python frameworks |
|---|---|---|---|
| **Providers** | 8+, one interface | Their cloud first | Varies by library |
| **Topology** | Declarative YAML | Code | Code |
| **Distribution** | Single <20MB binary | Binary + cloud services | 1GB+ container |
| **Security & observability** | Built in | Bring your own | Bring your own |
| **Cold start** | <100ms | Varies | 10–45s |

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
