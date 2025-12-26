# CLAUDE.md - Project Guide for AI Assistants

**Version**: 0.2.0
**Last Updated**: 2025-12-14
**Target**: AI assistants (Claude Code, GitHub Copilot, Cursor, etc.)

This document helps AI assistants understand and work effectively with the Aixgo project.

## Table of Contents

- [Project Overview](#project-overview)
- [Core Features Reference](#core-features-reference)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Code Conventions](#code-conventions)
- [Key Concepts](#key-concepts)
- [Configuration](#configuration)
- [Common Tasks](#common-tasks)
- [Security](#security)
- [Testing](#testing)
- [Deployment](#deployment)

---

## Project Overview

### What is Aixgo?

Aixgo is a **production-grade AI agent framework for Go** that enables building secure,
scalable multi-agent systems without Python dependencies. **Integrate via `go get` and
compile to <10MB binaries** - 100x smaller than Python framework containers.

Key capabilities:

- **Framework + Lightweight Binaries**: Import into your Go app, deploy as <10MB binaries
  with zero runtime dependencies (vs 1GB+ Python containers)
- **Type-Safe Architecture**: Compile-time error detection with Go's type system
- **Seamless Scaling**: Start local with Go channels, scale to distributed with gRPC
- **Multi-Pattern Orchestration**: 13 production-proven patterns (Supervisor, Sequential,
  Parallel, Router, Swarm, Hierarchical, RAG, Reflection, Ensemble, Classifier,
  Aggregation, Planning, MapReduce)

### Installation

```bash
go get github.com/aixgo-dev/aixgo
```

### Core Value Proposition

- **Production-First**: Built for systems that ship, scale, and stay running
- **Performance**: <100ms cold start vs 10-45s for Python frameworks
- **Deployment**: <10MB binaries vs 1GB+ Python containers (zero runtime dependencies)
- **Concurrency**: True parallelism (no GIL) with native Go channels and goroutines
- **Type Safety**: Compile-time error detection vs runtime failures

### Target Users

- **Backend Engineers**: Building AI-powered services in Go stacks
- **DevOps Teams**: Deploying production AI systems
- **Data Engineers**: Adding AI enrichment to ETL pipelines
- **Enterprises**: Running AI agents on-premises or edge devices

### Current Status

- **Version**: 0.2.0
- **Maturity**: Production-ready for core features
- **Go Version**: 1.24.0+
- **License**: MIT

---

## Core Features Reference

> **MANDATORY**: The comprehensive features reference is located at
> **[docs/FEATURES.md](docs/FEATURES.md)**. This file contains 200+ searchable
> features organized by category.

### Update Requirement

**IMPORTANT**: Whenever changes are made to the project, you **MUST** review and update
`docs/FEATURES.md` to reflect any:

- New features added
- Features modified or enhanced
- Features deprecated or removed
- Status changes (e.g., from roadmap to implemented)
- New configuration options
- New examples or use cases

### What FEATURES.md Contains

- **Agent Types**: All 6 agent implementations with capabilities
- **LLM Providers**: 7 providers + inference services
- **Orchestration Patterns**: All 13 patterns with status and metrics
- **Security Features**: 40+ security capabilities
- **Observability**: Tracing, metrics, health checks
- **Integration**: MCP, vector stores, embeddings
- **Deployment**: Docker, Kubernetes, Cloud Run options
- **Roadmap**: Planned features with status indicators

### Feature Status Indicators

When updating FEATURES.md, use these status indicators:

- ✅ **Implemented** - Feature is complete and production-ready
- 🚧 **In Progress** - Feature is being actively developed
- 🔮 **Roadmap** - Feature is planned for future implementation
- ❌ **Not Available** - Feature is not supported

---

## Architecture

### High-Level Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                     Application Layer                       │
│  (YAML Config, CLI Tools, Example Applications)             │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                   Orchestration Layer                       │
│  - Supervisor (internal/supervisor/)                        │
│  - Patterns (internal/supervisor/patterns/)                 │
│  - Workflow Engine (internal/workflow/)                     │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                      Agent Layer                            │
│  - Agent Types (agents/): ReAct, Classifier, Aggregator,    │
│    Planner, Producer, Logger                                │
│  - Agent Core (internal/agent/): Factory, Types, Registry   │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Runtime Layer                            │
│  - Local Runtime: Go channels (aixgo/runtime.go)            │
│  - Distributed Runtime: gRPC (internal/runtime/)            │
│  - Message Protocol (proto/)                                │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                   Integration Layer                         │
│  - LLM Providers (internal/llm/provider/): OpenAI,          │
│    Anthropic, Gemini, xAI, VertexAI, HuggingFace            │
│  - MCP (pkg/mcp/): Local, gRPC, multi-server                │
│  - Vector Stores (pkg/vectorstore/): Firestore, Memory      │
│  - Embeddings (pkg/embeddings/)                             │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                 Observability & Security                    │
│  - OpenTelemetry (internal/observability/)                  │
│  - Metrics/Health (pkg/observability/)                      │
│  - Security (pkg/security/): Auth, SSRF, Rate Limiting      │
└─────────────────────────────────────────────────────────────┘
```

### Key Packages and Responsibilities

#### Core Packages (Top-Level)

- **`aixgo.go`**: Main entry point, config loading, agent system initialization
- **`runtime.go`**: SimpleRuntime implementation (local, in-process communication)

#### Agent Layer

- **`agents/`**: Agent implementations
  - `react.go`: ReAct agents with LLM integration and tool calling
  - `classifier.go`: AI-powered content classification
  - `aggregator.go`: Multi-agent synthesis (consensus, weighted, semantic, hierarchical)
  - `planner.go`: Task decomposition and planning
  - `producer.go`: Message generation at intervals
  - `logger.go`: Message logging/consumption
  - `base.go`: Shared base agent functionality

- **`internal/agent/`**: Agent core infrastructure
  - `types.go`: Agent, Runtime, Message interfaces
  - `factory.go`: Agent factory pattern and registry
  - `testutil.go`: Testing utilities

#### LLM Integration

- **`internal/llm/`**: LLM integration and validation
  - `provider/`: Provider implementations (OpenAI, Anthropic, Gemini, xAI, Vertex, HuggingFace)
  - `validator/`: Pydantic AI-style validation retry for structured outputs
  - `schema/`: JSON schema types for structured outputs
  - `parser/`: Tool call parsing
  - `prompt/`: ReAct prompt templates
  - `cost/`: Token and cost tracking
  - `context/`: Context window management
  - `evaluation/`: Benchmarking and evaluation
  - `inference/`: Inference services (Ollama, vLLM, HuggingFace)

- **`pkg/llm/`**: Public LLM interfaces

#### Orchestration

- **`internal/supervisor/`**: Supervisor implementation
  - `supervisor.go`: Core supervisor logic
  - `patterns/`: Orchestration patterns
    - `parallel.go`: Parallel execution
    - `mapreduce.go`: MapReduce pattern
    - `classifier.go`: Classification orchestration
    - `planning.go`: Planning orchestration

- **`internal/workflow/`**: Workflow engine
  - `executor.go`: Workflow execution
  - `persistence.go`: Workflow state persistence

#### Integration & Tools

- **`pkg/mcp/`**: Model Context Protocol
  - `client.go`: MCP client implementation
  - `server.go`: MCP server implementation
  - `transport_local.go`: Local transport
  - `cluster.go`: Multi-server support
  - `discovery.go`: Service discovery
  - `registry.go`: Tool registry
  - `typed_tools.go`: Type-safe tool definitions

- **`pkg/vectorstore/`**: Vector database support
  - `firestore/`: Google Firestore implementation
  - `memory/`: In-memory implementation

- **`pkg/embeddings/`**: Embedding models (OpenAI, HuggingFace)

- **`pkg/memory/`**: Semantic memory and long-term storage

#### Observability & Security

- **`internal/observability/`**: OpenTelemetry integration
  - Distributed tracing
  - Structured logging
  - Metrics export

- **`pkg/observability/`**: Public observability APIs
  - `health.go`: Health checks
  - `metrics.go`: Prometheus metrics
  - `server.go`: Observability HTTP server

- **`pkg/security/`**: Security features
  - `auth.go`: Authentication (disabled, delegated, builtin, hybrid)
  - `ratelimit.go`: Rate limiting
  - `sanitize.go`: Input sanitization
  - `yaml.go`: Safe YAML parsing with limits
  - `audit.go`: Audit logging
  - `iap.go`: Google Cloud IAP integration

#### Runtime & Messaging

- **`internal/runtime/`**: Distributed runtime (gRPC)
- **`proto/`**: Protocol buffer definitions
  - `message.proto`: Message protocol
  - `mcp/`: MCP protocol definitions

#### Configuration & Deployment

- **`config/`**: Example YAML configurations
- **`cmd/`**: CLI tools
  - `orchestrator/`: Orchestrator binary
  - `benchmark/`: Benchmarking tool
  - `deploy/`: Deployment tools (Cloud Run, Kubernetes)

- **`deploy/`**: Deployment manifests
- **`docker/`**: Docker configurations

#### Examples & Documentation

- **`examples/`**: 15+ production-ready examples
- **`docs/`**: Comprehensive documentation
  - `PATTERNS.md`: 13 orchestration patterns
  - `SECURITY_BEST_PRACTICES.md`: Security guidelines
  - `DEPLOYMENT.md`: Deployment guide
  - `OBSERVABILITY.md`: Observability guide
  - `TESTING_GUIDE.md`: Testing strategies

---

## Project Structure

```text
aixgo/
├── aixgo.go                    # Main entry point, config loading
├── runtime.go                  # SimpleRuntime (local communication)
├── go.mod                      # Go module dependencies
├── Makefile                    # Build automation
├── Dockerfile                  # Production Docker image
├── docker-compose.yml          # Local development setup
│
├── agents/                     # Agent implementations
│   ├── react.go                # ReAct agents (LLM + tools)
│   ├── classifier.go           # Content classification
│   ├── aggregator.go           # Multi-agent synthesis
│   ├── planner.go              # Task planning
│   ├── producer.go             # Message generation
│   ├── logger.go               # Message logging
│   └── base.go                 # Shared functionality
│
├── internal/                   # Internal packages
│   ├── agent/                  # Agent core
│   │   ├── types.go            # Interfaces and types
│   │   └── factory.go          # Factory pattern
│   │
│   ├── llm/                    # LLM integration
│   │   ├── provider/           # LLM providers
│   │   │   ├── openai.go
│   │   │   ├── anthropic.go
│   │   │   ├── gemini.go
│   │   │   ├── xai.go
│   │   │   ├── vertexai.go
│   │   │   └── huggingface_*.go
│   │   ├── validator/          # Validation retry
│   │   ├── schema/             # JSON schemas
│   │   ├── parser/             # Tool parsing
│   │   ├── prompt/             # Prompt templates
│   │   ├── cost/               # Cost tracking
│   │   └── inference/          # Inference services
│   │
│   ├── supervisor/             # Supervisor orchestration
│   │   ├── supervisor.go
│   │   └── patterns/           # Orchestration patterns
│   │
│   ├── workflow/               # Workflow engine
│   ├── observability/          # OpenTelemetry
│   └── runtime/                # Distributed runtime (gRPC)
│
├── pkg/                        # Public packages
│   ├── mcp/                    # Model Context Protocol
│   ├── vectorstore/            # Vector databases
│   ├── embeddings/             # Embedding models
│   ├── memory/                 # Semantic memory
│   ├── security/               # Security features
│   ├── observability/          # Observability APIs
│   └── llm/                    # LLM interfaces
│
├── proto/                      # Protocol buffers
│   ├── message.proto
│   └── mcp/
│
├── cmd/                        # CLI tools
│   ├── orchestrator/
│   ├── benchmark/
│   └── deploy/
│
├── config/                     # Example configs
├── examples/                   # Example applications
├── docs/                       # Documentation
├── tests/                      # E2E tests
└── deploy/                     # Deployment manifests
```

---

## Development Workflow

### Building the Project

```bash
# Build all packages
make build
# or
go build -v ./...

# Build specific binary
go build -o bin/orchestrator cmd/orchestrator/main.go
```

### Running Tests (Development)

```bash
# Run all tests with race detection and coverage
make test

# Run short tests (no race detector)
make test-short

# Run specific package tests
go test -v ./agents/
go test -v ./internal/llm/provider/

# Generate coverage report
make coverage
# Opens coverage.html in browser

# Run benchmarks
make bench
```

### Linting

```bash
# Run golangci-lint (requires installation)
make lint

# Format code
make fmt

# Run go vet
make vet

# Run all checks (fmt + vet + lint + test)
make check
```

### Running Examples

```bash
# Run the main example
make run

# Run specific example
go run examples/classifier-workflow/main.go

# With environment variables
export OPENAI_API_KEY=sk-...
go run examples/main.go
```

### Code Generation

```bash
# Generate protocol buffers (if modified)
cd proto/mcp
go generate

# Generate mocks (if needed)
go generate ./...
```

### Dependency Management

```bash
# Download dependencies
make deps

# Update dependencies
go get -u ./...
go mod tidy

# Vendor dependencies (optional)
go mod vendor
```

---

## Code Conventions

### Go Style Guidelines

Aixgo follows **standard Go conventions** from [Effective Go](https://golang.org/doc/effective_go.html)
and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

#### Package Organization

- **`internal/`**: Private packages, not importable by external projects
- **`pkg/`**: Public packages, stable APIs for external use
- **Top-level packages**: Core framework logic (aixgo.go, runtime.go)

#### Naming Conventions

```go
// Interfaces: Noun or adjective
type Agent interface { ... }
type Runtime interface { ... }
type Provider interface { ... }

// Constructors: New* prefix
func NewSimpleRuntime() *SimpleRuntime
func NewReActAgent(...) *ReActAgent

// Factory functions: Create* prefix
func CreateAgent(def AgentDef, rt Runtime) (Agent, error)

// Boolean methods: Is*/Has*/Can* prefix
func (a *Agent) Ready() bool

// Error variables: Err* prefix
var ErrRuntimeNotFound = errors.New("runtime not found")
```

#### Error Handling Patterns

```go
// Always check errors
result, err := agent.Execute(ctx, msg)
if err != nil {
    return nil, fmt.Errorf("execute agent: %w", err)  // Use %w for wrapping
}

// Sentinel errors for API boundaries
var (
    ErrAgentNotFound = errors.New("agent not found")
    ErrRuntimeNotFound = errors.New("runtime not found")
)

// Custom error types for complex errors
type ValidationError struct {
    Field string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Reason)
}
```

#### Context Usage

```go
// Always accept context as first parameter
func (a *Agent) Execute(ctx context.Context, input *Message) (*Message, error)

// Use context for cancellation, timeouts, and values
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// Retrieve runtime from context
rt, err := agent.RuntimeFromContext(ctx)
if err != nil {
    return nil, err
}
```

#### Concurrency Patterns

```go
// Use goroutines for parallel execution
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

// Use sync.RWMutex for read-heavy workloads
type Registry struct {
    agents map[string]Agent
    mu     sync.RWMutex
}

func (r *Registry) Get(name string) (Agent, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    agent, ok := r.agents[name]
    if !ok {
        return nil, ErrAgentNotFound
    }
    return agent, nil
}
```

### Testing Patterns

#### Table-Driven Tests

```go
func TestAgentExecution(t *testing.T) {
    tests := []struct {
        name    string
        input   *Message
        want    *Message
        wantErr bool
    }{
        {
            name:    "successful execution",
            input:   &Message{Content: "test"},
            want:    &Message{Content: "result"},
            wantErr: false,
        },
        {
            name:    "error case",
            input:   &Message{Content: "invalid"},
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := agent.Execute(ctx, tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Execute() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

#### Mocking and Test Utilities

```go
// Use interfaces for dependency injection
type Provider interface {
    Complete(ctx context.Context, req *Request) (*Response, error)
}

// Create mock implementations
type MockProvider struct {
    CompleteFunc func(ctx context.Context, req *Request) (*Response, error)
}

func (m *MockProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
    if m.CompleteFunc != nil {
        return m.CompleteFunc(ctx, req)
    }
    return nil, errors.New("not implemented")
}

// Use testutil packages
// See agents/testutil.go, internal/agent/testutil.go
```

#### Agent Integration Tests

```go
// Use build tags for integration tests
//go:build integration

package test

import "testing"

func TestEndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    // ... test code
}
```

### Security Considerations

#### Input Validation

```go
// Always validate user inputs
func sanitizeInput(input string) (string, error) {
    // Remove control characters
    clean := strings.Map(func(r rune) rune {
        if r < 32 && r != '\n' && r != '\t' {
            return -1
        }
        return r
    }, input)

    // Check length limits
    if len(clean) > maxInputLength {
        return "", fmt.Errorf("input exceeds max length")
    }

    return clean, nil
}
```

#### SSRF Protection

```go
// Use allowlists for URLs
func isAllowedURL(urlStr string) error {
    u, err := url.Parse(urlStr)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    // Block private IPs
    if isPrivateIP(u.Hostname()) {
        return errors.New("private IPs not allowed")
    }

    // Check against allowlist
    if !isInAllowlist(u.Hostname()) {
        return errors.New("domain not in allowlist")
    }

    return nil
}
```

#### Safe YAML Parsing

```go
// Use security.SafeYAMLParser with limits
parser := security.NewSafeYAMLParser(security.YAMLLimits{
    MaxSize:       1 * 1024 * 1024, // 1MB
    MaxDepth:      10,
    MaxKeys:       1000,
    MaxAliases:    100,
})

var config Config
if err := parser.UnmarshalYAML(data, &config); err != nil {
    return fmt.Errorf("parse config: %w", err)
}
```

#### Rate Limiting

```go
// Use rate limiters for external APIs
limiter := security.NewRateLimiter(security.RateLimitConfig{
    RequestsPerSecond: 10,
    Burst:             20,
})

if !limiter.Allow(userID) {
    return errors.New("rate limit exceeded")
}
```

---

## Key Concepts

### Agent Types

#### 1. ReAct Agent

**Purpose**: Reasoning and Acting with LLM integration and tool calling.

**Configuration**:

```yaml
agents:
  - name: analyst
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a data analyst. Analyze data and provide insights.
    tools:
      - name: query_db
        description: Query the database
        input_schema:
          type: object
          properties:
            query: { type: string }
          required: [query]
```

**Code Location**: `agents/react.go`

#### 2. Classifier Agent

**Purpose**: AI-powered content classification with confidence scoring.

**Configuration**:

```yaml
agents:
  - name: ticket-classifier
    role: classifier
    model: gpt-4-turbo
    classifier_config:
      categories:
        - name: technical_issue
          description: "Technical problems"
          keywords: ["error", "bug", "crash"]
        - name: billing_inquiry
          description: "Payment questions"
          keywords: ["payment", "charge", "refund"]
      confidence_threshold: 0.7
      temperature: 0.3
```

**Code Location**: `agents/classifier.go`

#### 3. Aggregator Agent

**Purpose**: Synthesize outputs from multiple agents.

**Strategies**:

- **Consensus**: Majority voting
- **Weighted**: Confidence-weighted aggregation
- **Semantic**: Embedding-based similarity
- **Hierarchical**: Multi-level synthesis
- **RAG-based**: Retrieval-augmented aggregation

**Configuration**:

```yaml
agents:
  - name: synthesizer
    role: aggregator
    model: gpt-4-turbo
    inputs:
      - source: expert-1
      - source: expert-2
      - source: expert-3
    aggregator_config:
      aggregation_strategy: consensus
      consensus_threshold: 0.75
      conflict_resolution: llm_mediated
      timeout_ms: 5000
```

**Code Location**: `agents/aggregator.go`

#### 4. Planner Agent

**Purpose**: Task decomposition and planning.

**Configuration**:

```yaml
agents:
  - name: project-planner
    role: planner
    model: gpt-4-turbo
    planner_config:
      max_tasks: 20
      enable_replanning: true
      task_timeout_ms: 30000
```

**Code Location**: `agents/planner.go`

#### 5. Producer Agent

**Purpose**: Generate messages at intervals.

**Configuration**:

```yaml
agents:
  - name: event-generator
    role: producer
    interval: 500ms
    outputs:
      - target: processor
```

**Code Location**: `agents/producer.go`

#### 6. Logger Agent

**Purpose**: Consume and log messages.

**Configuration**:

```yaml
agents:
  - name: audit-log
    role: logger
    inputs:
      - source: processor
```

**Code Location**: `agents/logger.go`

### LLM Providers

Aixgo supports multiple LLM providers through a unified `Provider` interface:

#### OpenAI

- Models: GPT-4, GPT-3.5-turbo, etc.
- Features: Chat completion, streaming, function calling
- Code: `internal/llm/provider/openai.go`

#### Anthropic (Claude)

- Models: Claude 3.5 Sonnet, Claude 3 Opus, etc.
- Features: Chat completion, streaming, tool use
- Code: `internal/llm/provider/anthropic.go`

#### Google Gemini

- Models: Gemini 1.5 Pro, Gemini 1.5 Flash
- Features: Chat completion, multimodal
- Code: `internal/llm/provider/gemini.go`

#### xAI (Grok)

- Models: Grok-beta
- Features: Chat completion
- Code: `internal/llm/provider/xai.go`

#### Google Vertex AI

- Models: Gemini on Vertex AI
- Features: Enterprise deployment
- Code: `internal/llm/provider/vertexai.go`

#### HuggingFace

- Models: Meta-Llama, Mistral, etc.
- Features: Chat completion, embeddings
- Variants: Basic (cloud API), Optimized (production)
- Code: `internal/llm/provider/huggingface_*.go`

**Provider Selection** (automatic based on model name):

- `gpt-*` → OpenAI
- `claude-*` → Anthropic
- `gemini-*` → Google Gemini
- `grok-*`, `xai-*` → xAI
- `meta-llama/*`, `mistralai/*` → HuggingFace

### Orchestration Patterns

See [docs/PATTERNS.md](~/go/src/github.com/aixgo-dev/aixgo/docs/PATTERNS.md)
for comprehensive pattern documentation.

**Available Patterns**:

1. **Supervisor**: Centralized orchestration with routing
2. **Sequential**: Ordered pipeline execution
3. **Parallel**: Concurrent execution (3-4× speedup)
4. **Router**: Intelligent routing for cost optimization (25-50% savings)
5. **Swarm**: Decentralized agent handoffs
6. **Hierarchical**: Multi-level delegation
7. **RAG**: Retrieval-Augmented Generation
8. **Reflection**: Iterative refinement for quality
9. **Ensemble**: Multi-model voting for accuracy
10. **Classifier**: Content classification
11. **Aggregation**: Multi-agent synthesis
12. **Planning**: Task decomposition
13. **MapReduce**: Parallel batch processing

### Model Context Protocol (MCP)

MCP enables tool calling and integration with external services.

**Transport Modes**:

- **Local**: In-process communication
- **gRPC**: Remote service communication
- **Multi-Server**: Multiple MCP servers

**Configuration**:

```yaml
mcp_servers:
  - name: weather-service
    transport: local  # or grpc
    address: localhost:50051  # for gRPC
    tls: true
    auth:
      type: bearer
      token_env: WEATHER_API_TOKEN

agents:
  - name: assistant
    role: react
    model: gpt-4-turbo
    mcp_servers:
      - weather-service
```

**Code Location**: `pkg/mcp/`

### Runtime Systems

#### Local Runtime (SimpleRuntime)

- **Purpose**: In-process communication using Go channels
- **Use Case**: Single binary deployment
- **Code**: `runtime.go`

```go
runtime := NewSimpleRuntime()
```

#### Distributed Runtime

- **Purpose**: Multi-node orchestration using gRPC
- **Use Case**: Distributed deployment
- **Code**: `internal/runtime/`

### Validation Retry (Pydantic AI-Style)

Automatic retry with validation errors for structured outputs (40-70% improved reliability).

```go
// Define schema
type Response struct {
    Category   string  `json:"category" jsonschema:"required,enum=technical|billing|sales"`
    Confidence float64 `json:"confidence" jsonschema:"required,minimum=0,maximum=1"`
}

// Validation retry automatically retries with error feedback
result, err := provider.CompleteStructured(ctx, req, &Response{})
```

**Code Location**: `internal/llm/validator/`

---

## Configuration

### YAML Configuration Format

```yaml
# Supervisor configuration (optional)
supervisor:
  name: coordinator
  model: gpt-4-turbo
  max_rounds: 10

# MCP servers (optional)
mcp_servers:
  - name: weather-service
    transport: local
    # or grpc with address
    # address: localhost:50051
    # tls: true

# Model services (optional, for HuggingFace)
model_services:
  - name: llama-service
    provider: huggingface
    model: meta-llama/Llama-2-7b
    runtime: ollama  # or vllm, cloud
    config:
      variant: optimized
      address: http://localhost:11434

# Agents (required)
agents:
  - name: producer
    role: producer
    interval: 1s
    outputs:
      - target: analyzer

  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: |
      You are a data analyst.
    inputs:
      - source: producer
    outputs:
      - target: logger

  - name: logger
    role: logger
    inputs:
      - source: analyzer
```

### Environment Variables

**LLM Provider API Keys**:

```bash
# Required: At least one provider key
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export XAI_API_KEY=xai-...
export HUGGINGFACE_API_KEY=hf_...

# Optional
export GOOGLE_API_KEY=...  # For Gemini
export GOOGLE_CLOUD_PROJECT=...  # For Vertex AI
```

**Observability**:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=aixgo
export LANGFUSE_PUBLIC_KEY=...
export LANGFUSE_SECRET_KEY=...
```

**Security**:

```bash
export AIXGO_API_KEY_<NAME>=...  # For builtin auth
export ENVIRONMENT=production  # development, staging, production
```

### Runtime Options

```go
// Create runtime
runtime := NewSimpleRuntime()

// Load config
loader := NewConfigLoader(&OSFileReader{})
config, err := loader.LoadConfig("config/agents.yaml")

// Start with config
err = RunWithConfig(config)
```

---

## Common Tasks

### Adding a New LLM Provider

1. **Create provider implementation** in `internal/llm/provider/`:

```go
package provider

import "context"

type MyProvider struct {
    apiKey string
    model  string
}

func NewMyProvider(apiKey, model string) *MyProvider {
    return &MyProvider{apiKey: apiKey, model: model}
}

func (p *MyProvider) Complete(ctx context.Context, req *Request) (*Response, error) {
    // Implement provider logic
    return &Response{Content: "..."}, nil
}

// Implement Provider interface methods
```

1. **Register in provider registry** (`internal/llm/provider/registry.go`):

```go
func init() {
    RegisterProvider("myprovider", func(model string) (Provider, error) {
        apiKey := os.Getenv("MYPROVIDER_API_KEY")
        return NewMyProvider(apiKey, model), nil
    })
}
```

1. **Add tests** in `internal/llm/provider/myprovider_test.go`

2. **Update documentation**

### Creating a New Agent Type

1. **Implement Agent interface** in `agents/`:

```go
package agents

import (
    "context"
    "github.com/aixgo-dev/aixgo/internal/agent"
)

type MyAgent struct {
    name    string
    runtime agent.Runtime
}

func NewMyAgent(def agent.AgentDef, rt agent.Runtime) (*MyAgent, error) {
    return &MyAgent{
        name:    def.Name,
        runtime: rt,
    }, nil
}

func (a *MyAgent) Name() string { return a.name }
func (a *MyAgent) Role() string { return "myagent" }

func (a *MyAgent) Start(ctx context.Context) error {
    // Async execution logic
    return nil
}

func (a *MyAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    // Sync execution logic
    return &agent.Message{Content: "result"}, nil
}

func (a *MyAgent) Stop(ctx context.Context) error { return nil }
func (a *MyAgent) Ready() bool { return true }
```

1. **Register agent type** in `agents/` init function:

```go
func init() {
    agent.Register("myagent", func(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
        return NewMyAgent(def, rt)
    })
}
```

1. **Add tests** in `agents/myagent_test.go`

2. **Update documentation**

### Adding New Tools

Tools are defined via MCP. Create a new MCP server:

```go
package main

import (
    "context"
    "github.com/aixgo-dev/aixgo/pkg/mcp"
)

func main() {
    server := mcp.NewServer("my-tools")

    // Register tool
    server.RegisterTool(mcp.Tool{
        Name:        "my_tool",
        Description: "Does something useful",
        InputSchema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "param": map[string]any{"type": "string"},
            },
            "required": []string{"param"},
        },
        Handler: func(ctx context.Context, args mcp.Args) (any, error) {
            param := args.String("param")
            // Tool logic
            return map[string]any{"result": param}, nil
        },
    })

    // Start server
    server.Serve(":50051")
}
```

### Writing Tests

#### Unit Tests

```go
func TestAgentExecute(t *testing.T) {
    // Create mock runtime
    runtime := agent.NewMockRuntime()

    // Create agent
    def := agent.AgentDef{
        Name: "test-agent",
        Role: "react",
    }
    agent, err := CreateAgent(def, runtime)
    require.NoError(t, err)

    // Test execution
    ctx := context.Background()
    input := &agent.Message{Content: "test"}
    result, err := agent.Execute(ctx, input)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

#### E2E Integration Tests

```go
//go:build integration

func TestEndToEndWorkflow(t *testing.T) {
    // Load config
    config, err := LoadConfig("testdata/config.yaml")
    require.NoError(t, err)

    // Start system
    runtime := NewSimpleRuntime()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Test workflow
    // ...
}
```

---

## Security

### Security Features

1. **SSRF Protection**: URL validation, private IP blocking
2. **Input Validation**: Schema validation, sanitization
3. **Rate Limiting**: Token bucket algorithm
4. **Authentication**: Disabled, delegated (IAP), builtin, hybrid modes
5. **Audit Logging**: Security event tracking
6. **Safe YAML Parsing**: Size/depth/complexity limits
7. **Prompt Injection Defense**: Structured outputs, input sanitization

### Security Best Practices

See [docs/SECURITY_BEST_PRACTICES.md](~/go/src/github.com/aixgo-dev/aixgo/docs/SECURITY_BEST_PRACTICES.md) for comprehensive guidelines.

**Key Principles**:

- **Validate all inputs** using JSON schemas
- **Sanitize user content** before LLM processing
- **Use allowlists** for URLs, file paths, commands
- **Prefer structured outputs** over text parsing
- **Enable rate limiting** for production
- **Use authentication** (not disabled mode) in production
- **Audit security events** for compliance

### Security Testing

```go
// Test input validation
func TestInputValidation(t *testing.T) {
    tests := []struct {
        input   string
        wantErr bool
    }{
        {"valid input", false},
        {"<script>alert('xss')</script>", true},
        {strings.Repeat("a", maxLength+1), true},
    }
    // ...
}

// Test SSRF protection
func TestSSRFProtection(t *testing.T) {
    privateIPs := []string{
        "http://127.0.0.1",
        "http://192.168.1.1",
        "http://10.0.0.1",
    }
    for _, url := range privateIPs {
        err := isAllowedURL(url)
        assert.Error(t, err)
    }
}
```

---

## Testing

### Testing Strategy

See [docs/TESTING_GUIDE.md](~/go/src/github.com/aixgo-dev/aixgo/docs/TESTING_GUIDE.md) for comprehensive testing documentation.

**Test Levels**:

1. **Unit Tests**: Package-level tests (80%+ coverage target)
2. **Integration Tests**: Cross-package tests with `//go:build integration`
3. **E2E Tests**: Full workflow tests in `tests/e2e/`
4. **Security Tests**: Dedicated security test suites

### Test Execution Commands

```bash
# All tests
make test

# Unit tests only
go test -v -short ./...

# Integration tests
go test -v -tags=integration ./...

# E2E tests
go test -v ./tests/e2e/

# With coverage
go test -cover ./...
make coverage  # Generates HTML report

# Benchmarks
make bench
```

### Test Utilities

- **`testutil.go`**: Top-level test utilities
- **`agents/testutil.go`**: Agent testing utilities
- **`internal/agent/testutil.go`**: Core agent testing utilities

---

## Deployment

### Deployment Options

See [docs/DEPLOYMENT.md](~/go/src/github.com/aixgo-dev/aixgo/docs/DEPLOYMENT.md) for comprehensive deployment guide.

#### 1. Docker

```bash
# Build image
docker build -t aixgo:latest .

# Run container
docker run -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  aixgo:latest
```

#### 2. Docker Compose

```bash
docker-compose up
```

#### 3. Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

#### 4. Google Cloud Run

```bash
# Using deployment tool
make deploy-cloudrun GCP_PROJECT_ID=my-project

# Manual
gcloud run deploy aixgo \
  --image gcr.io/my-project/aixgo \
  --platform managed \
  --region us-central1 \
  --set-env-vars OPENAI_API_KEY=sk-...
```

### Deployment Best Practices

1. **Use environment variables** for secrets (never commit API keys)
2. **Enable authentication** (delegated or builtin, not disabled)
3. **Configure rate limiting** for production workloads
4. **Set up observability** (OpenTelemetry, Prometheus)
5. **Use health checks** for liveness/readiness probes
6. **Monitor costs** via built-in cost tracking
7. **Enable audit logging** for compliance

### Observability

See [docs/OBSERVABILITY.md](~/go/src/github.com/aixgo-dev/aixgo/docs/OBSERVABILITY.md) for comprehensive observability guide.

**Features**:

- **Distributed Tracing**: OpenTelemetry integration
- **Metrics**: Prometheus metrics (HTTP, gRPC, agent, system)
- **Health Checks**: `/health`, `/health/live`, `/health/ready`
- **Cost Tracking**: Automatic token and cost metrics per LLM call
- **Langfuse Support**: LLM-specific observability

**Endpoints**:

- `GET /health` - Overall health status
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe
- `GET /metrics` - Prometheus metrics

---

## Additional Resources

### Documentation

- **[README.md](~/go/src/github.com/aixgo-dev/aixgo/README.md)**: Project overview and quick start
- **[docs/FEATURES.md](~/go/src/github.com/aixgo-dev/aixgo/docs/FEATURES.md)**: **Complete authoritative feature catalog** (single source of truth for ALL features)
- **[docs/PATTERNS.md](~/go/src/github.com/aixgo-dev/aixgo/docs/PATTERNS.md)**: 13 orchestration patterns with deep-dive guides
- **[docs/SECURITY_BEST_PRACTICES.md](~/go/src/github.com/aixgo-dev/aixgo/docs/SECURITY_BEST_PRACTICES.md)**: Security guidelines and best practices
- **[docs/DEPLOYMENT.md](~/go/src/github.com/aixgo-dev/aixgo/docs/DEPLOYMENT.md)**: Deployment guide (Cloud Run, Kubernetes, Docker)
- **[docs/OBSERVABILITY.md](~/go/src/github.com/aixgo-dev/aixgo/docs/OBSERVABILITY.md)**: Observability setup and configuration
- **[docs/TESTING_GUIDE.md](~/go/src/github.com/aixgo-dev/aixgo/docs/TESTING_GUIDE.md)**: Testing strategies and utilities
- **[docs/CONTRIBUTING.md](~/go/src/github.com/aixgo-dev/aixgo/docs/CONTRIBUTING.md)**: How to contribute

### Examples

Browse **15+ production-ready examples** in `~/go/src/github.com/aixgo-dev/aixgo/examples/`:

- Agent types (ReAct, Classifier, Aggregator, Planner)
- LLM providers (OpenAI, Anthropic, Gemini, xAI, HuggingFace)
- Orchestration patterns (Supervisor, Parallel, Sequential, Router, Swarm)
- MCP integration (Local, gRPC, multi-server)
- Security (Authentication, rate limiting, SSRF protection)

### External Links

- **Website**: <https://aixgo.dev>
- **GitHub**: <https://github.com/aixgo-dev/aixgo>
- **Go Package**: <https://pkg.go.dev/github.com/aixgo-dev/aixgo>
- **Discussions**: <https://github.com/orgs/aixgo-dev/discussions>

---

## Quick Reference

### Common Commands

```bash
# Build
make build

# Test
make test

# Lint
make lint

# Run example
make run

# Deploy to Cloud Run
make deploy-cloudrun GCP_PROJECT_ID=my-project

# Generate coverage
make coverage

# Format code
make fmt

# Clean build artifacts
make clean
```

### Import Paths

```go
// Core
import "github.com/aixgo-dev/aixgo"
import "github.com/aixgo-dev/aixgo/internal/agent"

// LLM
import "github.com/aixgo-dev/aixgo/internal/llm/provider"
import "github.com/aixgo-dev/aixgo/pkg/llm"

// MCP
import "github.com/aixgo-dev/aixgo/pkg/mcp"

// Security
import "github.com/aixgo-dev/aixgo/pkg/security"

// Observability
import "github.com/aixgo-dev/aixgo/pkg/observability"
```

### Key Files to Review

When working on specific features, these are the key files to review:

- **Agent Creation**: `internal/agent/factory.go`, `agents/*.go`
- **LLM Integration**: `internal/llm/provider/*.go`
- **Orchestration**: `internal/supervisor/supervisor.go`, `internal/supervisor/patterns/*.go`
- **MCP**: `pkg/mcp/client.go`, `pkg/mcp/server.go`
- **Security**: `pkg/security/*.go`
- **Configuration**: `aixgo.go` (config loading)
- **Runtime**: `runtime.go` (local), `internal/runtime/*.go` (distributed)

---

**End of CLAUDE.md**

For questions or contributions, see [docs/CONTRIBUTING.md](~/go/src/github.com/aixgo-dev/aixgo/docs/CONTRIBUTING.md) or open an issue at <https://github.com/aixgo-dev/aixgo/issues>.
