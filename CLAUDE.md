# CLAUDE.md - AI Assistant Project Guide

**Version**: 0.2.3
**Last Updated**: 2026-01-02
**Target**: AI assistants (Claude Code, GitHub Copilot, Cursor, etc.)

Quick reference for AI assistants working with Aixgo - a production-grade AI agent framework for Go.

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [Code Conventions](#code-conventions)
- [Key Concepts](#key-concepts)
- [Website](#website-web)
- [Common Tasks](#common-tasks)
- [Quick Reference](#quick-reference)

---

## Project Overview

Aixgo is a **production-grade AI agent framework for Go** enabling secure, scalable multi-agent systems without Python dependencies.

**Installation**: `go get github.com/aixgo-dev/aixgo`

### Why Aixgo?

| Metric | Aixgo | Python Frameworks |
|--------|-------|-------------------|
| Binary Size | <10MB | 1GB+ containers |
| Cold Start | <100ms | 10-45s |
| Concurrency | True parallelism (no GIL) | GIL-limited |
| Type Safety | Compile-time | Runtime errors |

### Key Capabilities

- **13 orchestration patterns** - Supervisor, Sequential, Parallel, Router, Swarm, Hierarchical, RAG, Reflection, Ensemble, Classifier, Aggregation, Planning, MapReduce
- **6 agent types** - ReAct, Classifier, Aggregator, Planner, Producer, Logger
- **7+ LLM providers** - OpenAI, Anthropic, Gemini, xAI, Vertex AI, HuggingFace, + inference services (Ollama, vLLM)
- **Validation retry** - Pydantic AI-style structured output validation with automatic retry (40-70% improved reliability)
- **MCP support** - Model Context Protocol for tool calling (local, gRPC, multi-server)

### Target Users

- **Backend Engineers** - Building AI-powered services in Go stacks
- **DevOps Teams** - Deploying production AI systems with minimal footprint
- **Data Engineers** - Adding AI enrichment to ETL pipelines
- **Enterprises** - Running AI agents on-premises or edge devices

### Current Status

- **Version**: 0.2.2
- **Maturity**: Production-ready for core features
- **Go Version**: 1.24.0+
- **License**: MIT

### Development Focus

Current priorities:
1. **Stability** - Production hardening and edge case handling
2. **Observability** - Enhanced tracing, metrics, and cost tracking
3. **Patterns** - Expanding orchestration pattern capabilities
4. **MCP** - Multi-server support and tool discovery

### Core Features Reference

**MANDATORY**: See **[docs/FEATURES.md](docs/FEATURES.md)** for the complete authoritative feature catalog (200+ features).

**Update Requirement**: When making changes, **MUST** update `docs/FEATURES.md` to reflect:
- New/modified/deprecated features
- Status changes (🚧 In Progress → ✅ Implemented)
- New configuration options or examples

**Status Indicators**:
- ✅ Implemented
- 🚧 In Progress
- 🔮 Roadmap
- ❌ Not Available

---

## Architecture

### Layer Overview

```text
Application (YAML Config, CLI, Examples)
    ↓
Orchestration (Supervisor, Patterns, Workflow)
    ↓
Agents (ReAct, Classifier, Aggregator, Planner, Producer, Logger)
    ↓
Runtime (Local: Go channels, Distributed: gRPC)
    ↓
Integration (LLM Providers, MCP, Vector Stores, Embeddings)
    ↓
Observability & Security (OpenTelemetry, Auth, Rate Limiting)
```

### Runtime Systems

**Local Runtime** (`runtime.go`):
- In-process communication via Go channels
- Single binary deployment
- Use: `NewSimpleRuntime()`

**Distributed Runtime** (`internal/runtime/`):
- Multi-node orchestration via gRPC
- Horizontal scaling across machines
- Protocol buffers in `proto/`

### Key Package Map

**Core**:
- `aixgo.go` - Entry point, config loading
- `runtime.go` - Local runtime (SimpleRuntime)

**Agents** (`agents/`):
- `react.go` - LLM + tool calling
- `classifier.go` - Content classification
- `aggregator.go` - Multi-agent synthesis
- `planner.go` - Task planning
- `producer.go` / `logger.go` - Message generation/logging

**Internal**:
- `internal/agent/` - Factory, types, interfaces
- `internal/llm/provider/` - OpenAI, Anthropic, Gemini, xAI, Vertex, HuggingFace
- `internal/llm/validator/` - Pydantic-style validation retry
- `internal/supervisor/` - Orchestration core
- `internal/supervisor/patterns/` - Parallel, MapReduce, etc.
- `internal/observability/` - OpenTelemetry
- `internal/runtime/` - Distributed runtime (gRPC)

**Public** (`pkg/`):
- `pkg/mcp/` - Model Context Protocol
- `pkg/vectorstore/` - Firestore, Memory
- `pkg/embeddings/` - OpenAI, HuggingFace
- `pkg/security/` - Auth, rate limiting, SSRF, sanitization
- `pkg/observability/` - Health checks, metrics

**Other**:
- `proto/` - Protocol buffers
- `cmd/` - CLI tools (orchestrator, benchmark, deploy)
- `config/` - Example YAML configs
- `examples/` - 15+ production examples
- `docs/` - Comprehensive documentation

---

## Code Conventions

### Go Standards

Follow [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

### Naming

```go
// Interfaces: Noun/adjective
type Agent interface { ... }

// Constructors: New* prefix
func NewSimpleRuntime() *SimpleRuntime

// Factory: Create* prefix
func CreateAgent(def AgentDef, rt Runtime) (Agent, error)

// Booleans: Is*/Has*/Can* prefix
func (a *Agent) Ready() bool

// Errors: Err* prefix
var ErrAgentNotFound = errors.New("agent not found")
```

### Error Handling

```go
// Wrap errors with %w
result, err := agent.Execute(ctx, msg)
if err != nil {
    return nil, fmt.Errorf("execute agent: %w", err)
}

// Sentinel errors
var ErrAgentNotFound = errors.New("agent not found")
```

### Context

```go
// Always first parameter
func (a *Agent) Execute(ctx context.Context, input *Message) (*Message, error)

// Use for timeouts/cancellation
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### Concurrency

```go
// Parallel execution with WaitGroup
var wg sync.WaitGroup
results := make(chan *Message, len(agents))

for _, target := range targets {
    wg.Add(1)
    go func(t string) {
        defer wg.Done()
        result, _ := runtime.Call(ctx, t, input)
        results <- result
    }(target)
}
wg.Wait()
close(results)

// RWMutex for read-heavy workloads
type Registry struct {
    agents map[string]Agent
    mu     sync.RWMutex
}
```

### Testing

```go
// Table-driven tests
func TestAgentExecution(t *testing.T) {
    tests := []struct {
        name    string
        input   *Message
        want    *Message
        wantErr bool
    }{
        {"success", &Message{Content: "test"}, &Message{Content: "result"}, false},
        {"error", &Message{Content: "invalid"}, nil, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := agent.Execute(ctx, tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
            // assertions...
        })
    }
}

// Integration tests with build tags
//go:build integration
func TestEndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // test code...
}
```

### Security Patterns

```go
// Input validation
func sanitizeInput(input string) (string, error) {
    clean := strings.Map(func(r rune) rune {
        if r < 32 && r != '\n' && r != '\t' {
            return -1
        }
        return r
    }, input)

    if len(clean) > maxInputLength {
        return "", fmt.Errorf("input exceeds max length")
    }
    return clean, nil
}

// SSRF protection with allowlists
func isAllowedURL(urlStr string) error {
    u, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }
    if isPrivateIP(u.Hostname()) {
        return errors.New("private IPs not allowed")
    }
    if !isInAllowlist(u.Hostname()) {
        return errors.New("domain not in allowlist")
    }
    return nil
}

// Safe YAML parsing
parser := security.NewSafeYAMLParser(security.YAMLLimits{
    MaxSize: 1*1024*1024, MaxDepth: 10, MaxKeys: 1000, MaxAliases: 100,
})
```

---

## Key Concepts

### Agent Types (6 total)

**ReAct** (`agents/react.go`): LLM + tool calling
```yaml
agents:
  - name: analyst
    role: react
    model: gpt-4-turbo
    prompt: "You are a data analyst..."
    tools: [...]
```

**Classifier** (`agents/classifier.go`): Content classification with confidence
```yaml
agents:
  - name: ticket-classifier
    role: classifier
    classifier_config:
      categories: [...]
      confidence_threshold: 0.7
```

**Aggregator** (`agents/aggregator.go`): Multi-agent synthesis (consensus, weighted, semantic, hierarchical, RAG)
```yaml
agents:
  - name: synthesizer
    role: aggregator
    inputs: [expert-1, expert-2, expert-3]
    aggregator_config:
      aggregation_strategy: consensus
```

**Planner** (`agents/planner.go`): Task decomposition
**Producer** (`agents/producer.go`): Message generation at intervals
**Logger** (`agents/logger.go`): Message logging

### LLM Providers (7+)

**Supported**: OpenAI, Anthropic (Claude), Google Gemini, xAI (Grok), Vertex AI, HuggingFace

**Provider Selection** (automatic by model prefix):
- `gpt-*` → OpenAI
- `claude-*` → Anthropic
- `gemini-*` → Google Gemini
- `grok-*`, `xai-*` → xAI
- `meta-llama/*`, `mistralai/*` → HuggingFace

**Code**: `internal/llm/provider/<provider>.go`

### Orchestration Patterns

See **[docs/PATTERNS.md](docs/PATTERNS.md)** for comprehensive guides.

**13 Patterns**: Supervisor, Sequential, Parallel (3-4× speedup), Router (25-50% cost savings), Swarm, Hierarchical, RAG, Reflection, Ensemble, Classifier, Aggregation, Planning, MapReduce

### Model Context Protocol (MCP)

Tool calling via local, gRPC, or multi-server transport.

```yaml
mcp_servers:
  - name: weather-service
    transport: local  # or grpc
    address: localhost:50051  # for gRPC
    tls: true
```

**Code**: `pkg/mcp/`

### Configuration

**YAML Example**:
```yaml
supervisor:
  name: coordinator
  model: gpt-4-turbo

agents:
  - name: producer
    role: producer
    interval: 1s
    outputs: [analyzer]

  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: "Analyze data..."
    inputs: [producer]
    outputs: [logger]

  - name: logger
    role: logger
    inputs: [analyzer]
```

**Environment Variables**:
```bash
# LLM Providers (at least one required)
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export XAI_API_KEY=xai-...

# Observability (optional)
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export LANGFUSE_PUBLIC_KEY=...

# Security (optional)
export AIXGO_API_KEY_<NAME>=...
export ENVIRONMENT=production
```

---

## Website (`web/`)

Hugo-based static website for [aixgo.dev](https://aixgo.dev).

### Development

```bash
cd web
make dev    # Start dev server at localhost:1313
make build  # Build for production
make lint   # Lint markdown content
```

### Data-Driven Content

Feature matrices and roadmap are driven by YAML data files:
- `data/features.yaml` - Feature matrix with status indicators (complete/in_progress/roadmap)
- `data/milestones.yaml` - Development milestones for homepage

### Content Structure

- `content/guides/` - 18+ technical guides (quick-start, agent-types, cost-optimization, etc.)
- `content/blog/` - Release announcements and blog posts
- `content/examples/` - YAML configuration examples

### Key Templates

- `layouts/index.html` - Homepage template
- `layouts/shortcodes/` - Reusable components:
  - `feature-releases.html` - Feature table renderer
  - `status-badge.html` - Status indicators (checkmark/construction/roadmap)
  - `alpha-notice.html` - Alpha warning banner

### Configuration

- `config/_default/hugo.toml` - Main Hugo config (baseURL, language, SEO)
- `firebase.json` - Firebase Hosting config
- `.firebaserc` - Firebase project ID

### Deployment

Automatically deployed via Google Cloud Build on push to main:
1. Builds Hugo site with `--minify`
2. Deploys to Firebase Hosting

Manual: `cd web && make build && firebase deploy --only hosting`

### Key Conventions

- **Data files first**: Always update `features.yaml` and `milestones.yaml` rather than hardcoding content
- **Ordered lists**: Use `1.` numbering throughout (markdownlint rule)
- **File naming**: kebab-case (e.g., `provider-integration.md`)
- **Code blocks**: Always specify language for syntax highlighting

---

## Common Tasks

### Add New LLM Provider

1. Create `internal/llm/provider/myprovider.go`:
```go
type MyProvider struct { apiKey, model string }
func (p *MyProvider) Complete(ctx context.Context, req *Request) (*Response, error) { ... }
```

2. Register in `internal/llm/provider/registry.go`:
```go
func init() {
    RegisterProvider("myprovider", func(model string) (Provider, error) {
        return NewMyProvider(os.Getenv("MYPROVIDER_API_KEY"), model), nil
    })
}
```

3. Add tests, update docs

### Create New Agent Type

1. Implement in `agents/myagent.go`:
```go
type MyAgent struct { name string; runtime agent.Runtime }
func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) { ... }
```

2. Register:
```go
func init() {
    agent.Register("myagent", func(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
        return NewMyAgent(def, rt)
    })
}
```

3. Add tests, update docs

### Add MCP Tools

Create MCP server:
```go
server := mcp.NewServer("my-tools")
server.RegisterTool(mcp.Tool{
    Name: "my_tool",
    Description: "Does something useful",
    InputSchema: map[string]any{ /* JSON schema */ },
    Handler: func(ctx context.Context, args mcp.Args) (any, error) { ... },
})
server.Serve(":50051")
```

---

## Quick Reference

### Commands

```bash
make build          # Build all packages
make test           # Run tests with race detection
make lint           # Run golangci-lint
make coverage       # Generate HTML coverage report
make run            # Run main example
make deploy-cloudrun GCP_PROJECT_ID=my-project
```

### Import Paths

```go
import "github.com/aixgo-dev/aixgo"
import "github.com/aixgo-dev/aixgo/internal/agent"
import "github.com/aixgo-dev/aixgo/internal/llm/provider"
import "github.com/aixgo-dev/aixgo/pkg/mcp"
import "github.com/aixgo-dev/aixgo/pkg/security"
import "github.com/aixgo-dev/aixgo/pkg/observability"
```

### Key Files by Feature

- **Agents**: `internal/agent/factory.go`, `agents/*.go`
- **LLM**: `internal/llm/provider/*.go`
- **Orchestration**: `internal/supervisor/supervisor.go`, `internal/supervisor/patterns/*.go`
- **MCP**: `pkg/mcp/client.go`, `pkg/mcp/server.go`
- **Security**: `pkg/security/*.go`
- **Config**: `aixgo.go`
- **Runtime**: `runtime.go`, `internal/runtime/*.go`

### Documentation

- **[docs/FEATURES.md](docs/FEATURES.md)** - **Authoritative feature catalog** (update on ALL changes)
- **[docs/PATTERNS.md](docs/PATTERNS.md)** - 13 orchestration patterns
- **[docs/SECURITY_BEST_PRACTICES.md](docs/SECURITY_BEST_PRACTICES.md)** - Security guidelines
- **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)** - Deployment guide
- **[docs/OBSERVABILITY.md](docs/OBSERVABILITY.md)** - Observability setup
- **[docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md)** - Testing strategies
- **[README.md](README.md)** - Quick start

### Security Best Practices

See [docs/SECURITY_BEST_PRACTICES.md](docs/SECURITY_BEST_PRACTICES.md) for details.

**Key Principles**:
- Validate inputs with JSON schemas
- Sanitize user content before LLM processing
- Use allowlists for URLs/paths/commands
- Prefer structured outputs over text parsing
- Enable rate limiting and authentication in production
- Audit security events

### Testing

See [docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md) for comprehensive guide.

```bash
make test                           # All tests
go test -v -short ./...            # Unit tests
go test -v -tags=integration ./... # Integration tests
go test -v ./tests/e2e/            # E2E tests
```

Test utilities: `testutil.go`, `agents/testutil.go`, `internal/agent/testutil.go`

### Deployment

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for comprehensive guide.

**Options**: Docker, Docker Compose, Kubernetes, Google Cloud Run

**Best Practices**:
1. Use environment variables for secrets
2. Enable authentication (not disabled mode)
3. Configure rate limiting
4. Set up observability (OpenTelemetry, Prometheus)
5. Use health checks (`/health`, `/health/live`, `/health/ready`)
6. Monitor costs via built-in tracking

### Examples

Browse 15+ production examples in `examples/`:
- Agent types, LLM providers, orchestration patterns
- MCP integration, security features

---

**Resources**:
- Website: <https://aixgo.dev>
- GitHub: <https://github.com/aixgo-dev/aixgo>
- Go Package: <https://pkg.go.dev/github.com/aixgo-dev/aixgo>
- Discussions: <https://github.com/orgs/aixgo-dev/discussions>

For contributions, see [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) or open an issue.
