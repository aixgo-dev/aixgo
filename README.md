# aixgo

[![Go Version](https://img.shields.io/github/go-mod/go-version/aixgo-dev/aixgo)](https://go.dev/) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/aixgo-dev/aixgo)](https://goreportcard.com/report/github.com/aixgo-dev/aixgo)

Production-grade AI agent framework for Go. Build secure, scalable multi-agent systems without Python dependencies.

**[Documentation](https://aixgo.dev)** | **[Quick Start](#quick-start)** | **[Examples](examples/)** | **[Contributing](docs/CONTRIBUTING.md)**

## Features

- **Single Binary Deployment**: Ship AI agents in <10MB binaries with zero runtime dependencies
- **Type-Safe Agent Architecture**: Compile-time error detection with Go's type system
- **Seamless Scaling**: Start local with Go channels, scale to distributed with gRPC—no code changes
- **Multi-Agent Orchestration**: Built-in supervisor pattern for coordinating agent workflows
- **Observable by Default**: OpenTelemetry integration for distributed tracing and monitoring

## Quick Start

### Installation

```bash
go get github.com/aixgo-dev/aixgo
```

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
  model: grok-beta
  max_rounds: 10

agents:
  - name: data-producer
    role: producer
    interval: 1s
    outputs:
      - target: analyzer

  - name: analyzer
    role: react
    model: grok-beta
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

## Documentation & Resources

Comprehensive documentation, guides, and examples are available at **[aixgo.dev](https://aixgo.dev)**:

### 📚 Guides

- **[Core Concepts](https://aixgo.dev/guides/core-concepts)** - Agent types, orchestration patterns, and architecture
- **[Multi-Agent Orchestration](https://aixgo.dev/guides/multi-agent-orchestration)** - Supervisor patterns and workflows
- **[Security Best Practices](https://aixgo.dev/guides/security)** - Production security configurations
- **[Production Deployment](https://aixgo.dev/guides/deployment)** - Docker, Kubernetes, and Cloud Run

### 🎯 Examples

Browse **29+ complete, production-ready configuration examples**:

- **[Agent Types](https://aixgo.dev/examples/agents)** - Producer, ReAct, Logger, Classifier, Aggregator, Planner
- **[LLM Providers](https://aixgo.dev/examples/llm-providers)** - OpenAI, Anthropic, Gemini, Vertex AI, xAI, HuggingFace
- **[MCP Integration](https://aixgo.dev/examples/mcp)** - Local transport, gRPC, multi-server setups
- **[Security](https://aixgo.dev/examples/security)** - Authentication, authorization, TLS configurations
- **[Orchestration](https://aixgo.dev/examples/orchestration)** - MapReduce, parallel, sequential, reflection patterns
- **[Use Cases](https://aixgo.dev/examples/use-cases)** - Complete end-to-end applications

### 🚀 Quick Links

- **[API Reference](https://pkg.go.dev/github.com/aixgo-dev/aixgo)** - Full Go package documentation
- **[Release Notes](https://aixgo.dev/releases)** - Track feature development and roadmap
- **[GitHub Discussions](https://github.com/orgs/aixgo-dev/discussions)** - Ask questions and share ideas

## Why Aixgo?

### Production-First Design

Python AI frameworks excel at prototyping but struggle in production. Aixgo is built for systems that ship, scale, and stay running.

| Dimension       | Python Frameworks      | Aixgo                    |
| --------------- | ---------------------- | ------------------------ |
| **Deployment**  | 1GB+ containers        | <10MB binary             |
| **Cold Start**  | 10-45 seconds          | <100ms                   |
| **Type Safety** | Runtime errors         | Compile-time checks      |
| **Concurrency** | GIL limitations        | True parallelism         |
| **Scaling**     | Manual queues/services | Built-in channels → gRPC |

### Use Cases

- **Data Pipelines**: Add AI enrichment to high-throughput ETL workflows
- **API Services**: Production AI endpoints with Go's performance characteristics
- **Edge Deployment**: Run AI agents on resource-constrained devices
- **Multi-Agent Systems**: Coordinate complex workflows with supervisor patterns
- **Distributed Systems**: Scale from single instance to multi-region with zero refactoring

## Architecture

Aixgo implements a message-based multi-agent architecture with three core patterns:

### Agent Types

- **Producer**: Generates messages at configured intervals
- **ReAct**: Reasoning + Acting agents powered by LLMs with tool calling
- **Logger**: Consumes and logs messages from other agents

### Communication Model

Agents communicate through a runtime abstraction layer:

- **Local Mode**: Go channels for in-process communication
- **Distributed Mode**: gRPC for multi-node orchestration
- **Same Code**: Automatic transport selection without code changes

### Supervisor Pattern

The supervisor orchestrates agent execution:

- Manages agent lifecycle (start, run, shutdown)
- Routes messages between agents based on configuration
- Enforces execution constraints (max rounds, timeouts)
- Provides observability hooks for monitoring

## Agent Roles

### Producer Agent

Generates periodic messages for downstream agents:

```yaml
agents:
  - name: event-generator
    role: producer
    interval: 500ms
    outputs:
      - target: processor
```

### ReAct Agent

LLM-powered agent with reasoning and tool calling:

```yaml
agents:
  - name: analyst
    role: react
    model: grok-beta
    prompt: 'You are an expert data analyst.'
    tools:
      - name: query_database
        description: 'Query the database'
        input_schema:
          type: object
          properties:
            query: { type: string }
          required: [query]
    inputs:
      - source: event-generator
    outputs:
      - target: logger
```

### Logger Agent

Consumes and logs messages:

```yaml
agents:
  - name: audit-log
    role: logger
    inputs:
      - source: analyst
```

## Configuration

Aixgo uses YAML-based declarative configuration:

```yaml
supervisor:
  name: string # Supervisor identifier
  model: string # LLM model to use
  max_rounds: int # Maximum execution rounds

agents:
  - name: string # Unique agent name
    role: string # producer | react | logger
    interval: duration # For producer agents
    model: string # For react agents
    prompt: string # System prompt for react agents
    tools: [] # Tool definitions for react agents
    inputs: [] # Input sources
    outputs: [] # Output targets
```

## Observability

Aixgo includes built-in OpenTelemetry support for production observability:

- **Distributed Tracing**: Track messages across multi-agent workflows
- **Structured Logging**: Context-aware logs with trace correlation
- **Metrics Export**: Agent performance and health metrics
- **Integration Ready**: Works with Grafana, Datadog, Langfuse, and more

## Development

### Building from Source

```bash
git clone https://github.com/aixgo-dev/aixgo.git
cd aixgo
go build ./...
```

### Running Tests

```bash
# Run all tests
go test ./...

# With race detection
go test -race ./...

# With coverage
go test -cover ./...
```

### Project Structure

```text
aixgo/
├── agents/           # Agent implementations (Producer, ReAct, Logger)
├── config/           # Example configurations
├── docs/             # Documentation
├── examples/         # Example applications
├── internal/
│   ├── agent/        # Agent core types and factory
│   ├── llm/          # LLM integration and validation
│   ├── observability/# OpenTelemetry integration
│   └── supervisor/   # Supervisor implementation
├── proto/            # Message protocol definitions
├── aixgo.go          # Main entry point and config loader
└── runtime.go        # Message runtime and communication layer
```

## Roadmap

See our [GitHub Project Board](https://github.com/orgs/aixgo-dev/projects/1) for the latest roadmap, feature development, and priorities.

## Documentation

For comprehensive documentation, visit **[https://aixgo.dev](https://aixgo.dev)**.

**Repository Documentation:**

### Getting Started

- [Quick Start Guide](docs/QUICKSTART.md) - Get started in 5 minutes
- [Deployment Guide](docs/DEPLOYMENT.md) - Deploy to Cloud Run, Kubernetes, or local
- [API Reference](https://pkg.go.dev/github.com/aixgo-dev/aixgo) - GoDoc documentation

### Development

- [Contributing Guide](docs/CONTRIBUTING.md) - How to contribute
- [Testing Guide](docs/TESTING_GUIDE.md) - Testing strategies
- [Observability Guide](docs/OBSERVABILITY.md) - OpenTelemetry integration
- [Tools Reference](TOOLS.md) - Development tools and commands

### Security

- [Authentication Guide](docs/AUTHENTICATION.md) - Configure auth modes for different deployments
- [Security Status](SECURITY_STATUS.md) - Current security implementation status
- [Security Best Practices](SECURITY_BEST_PRACTICES.md) - Secure development guidelines
- [Production Security Checklist](PRODUCTION_SECURITY_CHECKLIST.md) - Pre-deployment requirements
- [Docker Security](DOCKER_SECURITY.md) - Container security hardening

## Authentication Configuration

Aixgo supports flexible authentication modes for different deployment scenarios:

| Mode        | Use Case          | Description                            |
| ----------- | ----------------- | -------------------------------------- |
| `disabled`  | Local development | No authentication (NOT for production) |
| `delegated` | Cloud Run + IAP   | Infrastructure handles auth            |
| `builtin`   | Self-hosted       | Application validates API keys         |
| `hybrid`    | Mixed auth        | Both infrastructure and API keys       |

### Quick Examples

**Local Development** (no auth):

```yaml
environment: development
auth_mode: disabled
```

**Cloud Run with IAP**:

```yaml
environment: production
auth_mode: delegated
delegated_auth:
  identity_header: X-Goog-Authenticated-User-Email
  iap:
    enabled: true
```

**Self-hosted with API Keys**:

```yaml
environment: production
auth_mode: builtin
builtin_auth:
  method: api_key
  api_keys:
    source: environment
    env_prefix: AIXGO_API_KEY_
```

See [Authentication Guide](docs/AUTHENTICATION.md) for complete documentation and [examples/](examples/) for full configuration files.

## Contributing

We welcome contributions! Please see our [Contributing Guide](docs/CONTRIBUTING.md) for details.

Key areas we're looking for help:

- Additional agent implementations
- LLM provider integrations
- Documentation improvements
- Example applications
- Performance optimizations

## License

MIT License - see [LICENSE](LICENSE) for details.

## Community

- [GitHub Discussions](https://github.com/aixgo-dev/aixgo/discussions) - Ask questions, share ideas
- [Issues](https://github.com/aixgo-dev/aixgo/issues) - Report bugs, request features

## Why Go for AI Agents?

**"Where Python prototypes go to die in production, Go agents ship and scale."**

Python excels at AI research and prototyping. Go excels at production systems. Aixgo bridges the gap:

- **Performance**: Compiled binaries with true concurrency, no GIL
- **Simplicity**: Single binary deployment, no dependency management
- **Security**: Minimal attack surface, static linking, memory safety
- **Scalability**: Native support for distributed systems
- **Maintainability**: Type safety catches errors before production

Build AI agents that ship with the same performance, security, and operational simplicity as the rest of your Go stack.

---

**Production-grade AI agents in pure Go.**
