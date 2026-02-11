# aixgo

[![Go Version](https://img.shields.io/github/go-mod/go-version/aixgo-dev/aixgo)](https://go.dev/) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/aixgo-dev/aixgo)](https://goreportcard.com/report/github.com/aixgo-dev/aixgo)

Production-grade AI agent framework for Go. Build secure, scalable multi-agent systems without Python dependencies.

**[Documentation](https://aixgo.dev)** | **[Quick Start](#quick-start)** | **[Features](docs/FEATURES.md)** | **[Examples](examples/)** | **[Contributing](docs/CONTRIBUTING.md)**

## Why Aixgo?

Python AI frameworks excel at prototyping but struggle in production. Aixgo is built for systems that ship, scale, and stay running.

| Dimension       | Python Frameworks      | Aixgo                    |
| --------------- | ---------------------- | ------------------------ |
| **Deployment**  | 1GB+ containers        | <20MB binary             |
| **Cold Start**  | 10-45 seconds          | <100ms                   |
| **Type Safety** | Runtime errors         | Compile-time checks      |
| **Concurrency** | GIL limitations        | True parallelism         |
| **Scaling**     | Manual queues/services | Built-in channels → gRPC |

### Key Features

- **6 Agent Types** - ReAct, Classifier, Aggregator, Planner, Producer, Logger
- **13 Orchestration Patterns** - All production-proven patterns implemented
- **6+ LLM Providers** - OpenAI, Anthropic, Gemini, xAI, Vertex AI, HuggingFace, plus local inference
- **Session Persistence** - Built-in conversation memory with JSONL and Redis storage (v0.3.0+)
- **Enterprise Security** - 4 auth modes, RBAC, rate limiting, SSRF protection, comprehensive hardening
- **Full Observability** - OpenTelemetry, Prometheus, Langfuse, cost tracking
- **Cost Optimization** - 25-50% savings with Router pattern, 70% token reduction with RAG

> 📖 **Complete Feature Catalog**: See [docs/FEATURES.md](docs/FEATURES.md) for all features with code references and technical details.

### What's New in v0.3.0

**Session Persistence** - AI agents now remember conversations with built-in session management:

```go
// Sessions are automatic - agents remember context
sess, _ := mgr.GetOrCreate(ctx, "assistant", "user-123")
result, _ := rt.CallWithSession(ctx, "assistant", msg, sess.ID())
```

**Runtime Consolidation** - Unified API with functional options:

```go
rt := aixgo.NewRuntime(
    aixgo.WithSessionManager(sessionMgr),
    aixgo.WithMetrics(metricsCollector),
)
```

**Distributed Runtime Parity** - TLS/mTLS, streaming, and Redis sessions for multi-node deployments.

**Security Hardening** - 29 code scanning alerts fixed including path traversal, subprocess injection, and safe integer conversions.

Read the full release notes: [v0.3.0 Release Blog Post](https://aixgo.dev/blog/v0.3.0-session-persistence/)

## Quick Start

### Installation

Choose the installation method based on your use case:

#### As a Library

For adding Aixgo to your Go project:

```bash
go get github.com/aixgo-dev/aixgo
```

This downloads only the Go framework source code (~2MB), not the website or documentation.

#### CLI Binary

The `aixgo` CLI runs agents from YAML configuration files.

**Option 1: Install via `go install`** (requires Go 1.24+):

```bash
go install github.com/aixgo-dev/aixgo/cmd/aixgo@latest
```

**Option 2: Download pre-built binaries**:

Download platform-specific binaries from [GitHub Releases](https://github.com/aixgo-dev/aixgo/releases):

```bash
# Linux/macOS
curl -L https://github.com/aixgo-dev/aixgo/releases/latest/download/aixgo_Linux_x86_64.tar.gz | tar xz
sudo mv aixgo /usr/local/bin/
```

Available for Linux, macOS, and Windows (amd64, arm64).

#### Full Repository (Contributors)

For contributing or exploring examples:

```bash
git clone https://github.com/aixgo-dev/aixgo.git
cd aixgo
go build ./...
```

This includes the full repository with website source (`web/`), examples, and documentation.

#### What You Get

| User Type | Command | What's Included | Size |
|-----------|---------|----------------|------|
| **Library user** | `go get github.com/aixgo-dev/aixgo` | Go source code only | ~2MB |
| **CLI user** | `go install` or binary download | Single executable binary | <20MB |
| **Contributor** | `git clone` | Full repo including web/, examples/, docs/ | ~20MB |

### Setup

Before running your agents, you need to configure API keys for LLM providers. Create a `.env` file in your project root (or set environment variables):

```bash
# Copy the example environment file
cp .env.example .env

# Edit .env and add your API keys
# Required: At least one of these API keys
export OPENAI_API_KEY=sk-...        # For GPT models
export XAI_API_KEY=xai-...          # For Grok models
export ANTHROPIC_API_KEY=sk-ant-... # For Claude models (optional)
export HUGGINGFACE_API_KEY=hf_...  # For HuggingFace models (optional)
```

The framework will automatically detect the appropriate API key based on your model name:

- `grok-*` or `xai-*` models use `XAI_API_KEY`
- `gpt-*` models use `OPENAI_API_KEY`
- `claude-*` models use `ANTHROPIC_API_KEY`
- HuggingFace models (e.g., `meta-llama/*`) use `HUGGINGFACE_API_KEY`

### Your First Agent

Create a simple multi-agent system in under 5 minutes:

**1. Create a configuration file** (`config/agents.yaml`):

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

**2. Create your main.go**:

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

**3. Run your agent system**:

```bash
go run main.go
```

That's it! You now have a running multi-agent system with producer, analyzer, and logger agents orchestrated by a supervisor.

## Use Cases

- **Data Pipelines** - Add AI enrichment to high-throughput ETL workflows
- **API Services** - Production AI endpoints with Go's performance
- **Edge Deployment** - Run AI agents on resource-constrained devices
- **Multi-Agent Systems** - Coordinate complex workflows with supervisor patterns

## Architecture

Aixgo provides a flexible, layered architecture:

- **Agent Layer** - 6 specialized agent types
- **Orchestration Layer** - 13 production-proven patterns
- **Runtime Layer** - Local (Go channels) or Distributed (gRPC)
- **Integration Layer** - 6+ LLM providers, MCP tool calling, vector stores
- **Observability Layer** - OpenTelemetry, Prometheus, cost tracking

> 🔗 **Deep Dive**: For detailed architecture and pattern documentation, see [docs/PATTERNS.md](docs/PATTERNS.md).

## Documentation

**Comprehensive guides and examples available at [aixgo.dev](https://aixgo.dev)**

### Resources

- **[Website](https://aixgo.dev)** - Comprehensive guides and documentation
- **[docs/](docs/)** - Technical reference documentation
- **[examples/](examples/)** - Production-ready code examples
- **[web/](web/)** - Website source code

### Core Documentation

- **[FEATURES.md](docs/FEATURES.md)** - Complete feature catalog with code references
- **[PATTERNS.md](docs/PATTERNS.md)** - 13 orchestration patterns with examples
- **[SECURITY_BEST_PRACTICES.md](docs/SECURITY_BEST_PRACTICES.md)** - Security guidelines
- **[DEPLOYMENT.md](docs/DEPLOYMENT.md)** - Cloud Run, Kubernetes, Docker
- **[OBSERVABILITY.md](docs/OBSERVABILITY.md)** - OpenTelemetry and cost tracking
- **[API Reference](https://pkg.go.dev/github.com/aixgo-dev/aixgo)** - GoDoc documentation

### Examples

Browse **15+ production-ready examples** in [examples/](examples/):

- Agent types: ReAct, Classifier, Aggregator, Planner
- LLM providers: OpenAI, Anthropic, Gemini, xAI, HuggingFace
- Orchestration: MapReduce, parallel, sequential, reflection
- Security: Authentication, authorization, TLS
- Complete use cases: End-to-end applications

## Development

```bash
# Build
git clone https://github.com/aixgo-dev/aixgo.git
cd aixgo
go build ./...

# Test
go test ./...
go test -race ./...

# Coverage
go test -cover ./...
```

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for contribution guidelines.

## Contributing

We welcome contributions! See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for guidelines.

## Community

- **[GitHub Discussions](https://github.com/aixgo-dev/aixgo/discussions)** - Ask questions, share ideas
- **[Issues](https://github.com/aixgo-dev/aixgo/issues)** - Report bugs, request features
- **[Roadmap](https://github.com/orgs/aixgo-dev/projects/1)** - Track feature development

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Production-grade AI agents in pure Go.**

Build AI agents that ship with the same performance, security, and operational simplicity as the rest of your Go stack.
