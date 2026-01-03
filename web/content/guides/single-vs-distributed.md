---
title: 'Single Binary vs Distributed Mode'
description: 'Understand how Aixgo scales from local development to distributed production with zero code changes.'
breadcrumb: 'Core Concepts'
category: 'Core Concepts'
weight: 5
---

One of Aixgo's planned features is seamless scaling: write your code once, and run it anywhere. Currently in alpha, local mode is available. Distributed mode coming in v0.2.

## The Problem with Traditional Scaling

Most frameworks require you to choose your architecture upfront:

- **Local development** - Simple, fast, but doesn't match production
- **Distributed production** - Complex, requires queues, service orchestration, infrastructure
- **The gap** - Refactoring, rewriting, architectural changes to move from local to distributed

This creates a painful transition: prototype locally with one architecture, then rewrite for production with another.

## Aixgo's Solution: Transport Abstraction

Aixgo abstracts message transport into a runtime layer. Your agent code stays the same; the runtime selects the appropriate transport based on configuration.

```go
// This code works locally AND distributed
supervisor := aixgo.NewSupervisor("coordinator")
supervisor.AddAgent(producer)
supervisor.AddAgent(analyzer)
supervisor.Run()  // Runtime picks: channels or gRPC
```

## Local Mode: Go Channels

### How It Works

In local mode, Aixgo uses Go channels for inter-agent communication. All agents run in the same process, communicating through in-memory channels.

```go
// Internal implementation (simplified)
type LocalRuntime struct {
    channels map[string]chan Message
}

func (r *LocalRuntime) Send(target string, msg Message) {
    r.channels[target] <- msg
}
```

### Benefits

- **Fast iteration** - No infrastructure setup required
- **Easy debugging** - Single process, standard Go debugging tools
- **Low latency** - In-memory communication, microsecond message passing
- **Perfect for development** - Rapid prototyping and testing

### When to Use Local Mode

✅ **Local development** - Prototyping and testing on your machine

✅ **Single-instance deployments** - Cloud Run, Lambda, simple containers

✅ **Small workloads** - Low throughput, single-region requirements

✅ **Edge devices** - Resource-constrained environments where one process is sufficient

## Distributed Mode: gRPC

### How It Works

In distributed mode, Aixgo uses gRPC with Protocol Buffers for inter-agent communication. Agents can run on different nodes, regions, or even cloud providers.

```go
// Same agent code! Runtime handles gRPC automatically
supervisor := aixgo.NewSupervisor("coordinator")
supervisor.AddAgent(producer)  // May run on node A
supervisor.AddAgent(analyzer)  // May run on node B
supervisor.Run()  // Runtime uses gRPC
```

### Benefits

- **Horizontal scaling** - Run agents on multiple nodes
- **Fault isolation** - Agent failures don't crash the entire system
- **Multi-region support** - Deploy across geographic regions
- **Resource optimization** - Run compute-heavy agents on dedicated hardware

### When to Use Distributed Mode

✅ **High throughput** - Processing thousands of messages per second

✅ **Multi-region** - Geographic distribution for latency or compliance

✅ **Resource isolation** - Separate agents with different compute requirements

✅ **Fault tolerance** - Critical systems requiring redundancy

## The Same Code, Different Configuration

Here's the key insight: **your agent code never changes**. Only configuration differs.

### Local Configuration

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  mode: local # Uses Go channels

agents:
  - name: producer
    role: producer
    outputs:
      - target: analyzer

  - name: analyzer
    role: react
    inputs:
      - source: producer
```

### Distributed Configuration

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  mode: distributed # Uses gRPC

agents:
  - name: producer
    role: producer
    endpoint: 'producer-service:50051' # gRPC endpoint
    outputs:
      - target: analyzer

  - name: analyzer
    role: react
    endpoint: 'analyzer-service:50051' # gRPC endpoint
    inputs:
      - source: producer
```

## Scaling Path: Local → Single Instance → Distributed

### Step 1: Develop Locally

```bash
# Development on your laptop
go run main.go
```

Configuration: `mode: local`

### Step 2: Deploy Single Instance

```dockerfile
# Dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o agent main.go

FROM scratch
COPY --from=builder /app/agent /agent
COPY config/ /config/
CMD ["/agent"]
```

Deploy to Cloud Run, Lambda, or any container platform. Still using `mode: local` - all agents in one process.

### Step 3: Scale to Distributed

When you need more capacity:

1. **Split agents into services** - Deploy each agent as a separate service
2. **Update configuration** - Change `mode: distributed`, add endpoints
3. **Deploy** - Same binary, different config

```yaml
# Kubernetes deployment example
apiVersion: apps/v1
kind: Deployment
metadata:
  name: producer
spec:
  template:
    spec:
      containers:
        - name: producer
          image: my-agent:latest
          args: ['--agent=producer']
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: analyzer
spec:
  template:
    spec:
      containers:
        - name: analyzer
          image: my-agent:latest # Same image!
          args: ['--agent=analyzer']
```

## Performance Comparison

Choose the right deployment mode based on your performance and scale requirements.

|                | Local Mode                    | Distributed Mode            |
| -------------- | ----------------------------- | --------------------------- |
| **Latency**    | Microseconds (in-memory)      | Milliseconds (network)      |
| **Throughput** | 10,000+ msg/s (single core)   | 100,000+ msg/s (multi-node) |
| **Deployment** | Single binary                 | Multiple services           |
| **Scaling**    | Vertical (bigger instance)    | Horizontal (more nodes)     |
| **Cost**       | $10-50/month (small instance) | $100-500/month (cluster)    |
| **Complexity** | Simple                        | Requires orchestration      |

## Best Practices

### Start Local

Always begin with local mode:

- Fast development iteration
- Easy debugging
- Validate logic before adding infrastructure

### Measure Before Distributing

Only move to distributed mode when you have evidence you need it:

- Throughput exceeding single-instance capacity
- Geographic distribution requirements
- Specific resource isolation needs

Don't prematurely optimize—many production workloads run fine in local mode on a single Cloud Run instance.

### Use Environment-Based Configuration

```go
// main.go
func main() {
    env := os.Getenv("ENV")
    configPath := fmt.Sprintf("config/agents-%s.yaml", env)

    if err := aixgo.Run(configPath); err != nil {
        panic(err)
    }
}
```

```bash
# Development
ENV=local go run main.go

# Production (single instance)
ENV=prod go run main.go

# Production (distributed)
ENV=prod-distributed go run main.go
```

### Monitor Transport Performance

Use OpenTelemetry to track:

- Message latency (local vs network)
- Throughput per agent
- Resource utilization

This data informs when to scale.

## Current Status & Roadmap

**Local Mode: ✅ Available (v0.1)** Ready for production use with Go channels.

**Distributed Mode: 🚧 Coming Soon (v0.2)** gRPC transport and distributed orchestration scheduled for Q1 2026.

While distributed mode is in development, you can build production systems today using local mode. Most use cases—especially single-instance Cloud Run/Lambda deployments—work
perfectly with in-process channels.

## Next Steps

- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Build complex workflows
- **[Production Deployment](/guides/production-deployment)** - Deploy to production environments
- **[Observability & Monitoring](/guides/observability)** - Track performance and debug issues
