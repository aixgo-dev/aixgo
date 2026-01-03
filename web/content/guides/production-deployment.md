---
title: 'Production Deployment'
description: 'Deploy Aixgo agents to production with best practices for scaling and monitoring.'
breadcrumb: 'Deployment'
category: 'Deployment'
weight: 8
---

Aixgo is built for production deployment. This guide covers deployment patterns, best practices, and strategies for running AI agents at scale with enterprise security and full
observability.

## Deployment Patterns

### Pattern 1: Single Binary on Cloud Run / Lambda

The simplest production deployment: compile to a single binary and deploy to serverless platforms.

**Pros:**

- Zero infrastructure management
- Auto-scaling included
- Pay-per-request pricing
- <100ms cold start

**Best for:**

- API endpoints
- Event-driven workflows
- Low to medium throughput (<1000 req/s)

#### Example: Cloud Run

```dockerfile
# Build stage
FROM golang:1.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agent main.go

# Runtime stage
FROM scratch
COPY --from=builder /app/agent /agent
COPY --from=builder /app/config/ /config/
CMD ["/agent"]
```

```bash
# Build and deploy
docker build -t gcr.io/my-project/aixgo-agent .
docker push gcr.io/my-project/aixgo-agent
gcloud run deploy aixgo-agent \
  --image gcr.io/my-project/aixgo-agent \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated
```

**Configuration:**

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  mode: local # Single process
  max_rounds: 10
```

### Pattern 2: Container on Kubernetes

For higher control and customization, deploy as containers on Kubernetes.

**Pros:**

- Full control over scaling
- Persistent connections
- Custom networking
- Multi-region support

**Best for:**

- High throughput (>1000 req/s)
- Stateful workflows
- Complex networking requirements

#### Example: Kubernetes Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aixgo-agent
spec:
  replicas: 3
  selector:
    matchLabels:
      app: aixgo-agent
  template:
    metadata:
      labels:
        app: aixgo-agent
    spec:
      containers:
        - name: agent
          image: my-registry/aixgo-agent:latest
          ports:
            - containerPort: 8080
          env:
            - name: CONFIG_PATH
              value: '/config/agents.yaml'
            - name: LOG_LEVEL
              value: 'info'
          resources:
            requests:
              memory: '128Mi'
              cpu: '100m'
            limits:
              memory: '512Mi'
              cpu: '500m'
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: aixgo-agent
spec:
  selector:
    app: aixgo-agent
  ports:
    - port: 80
      targetPort: 8080
  type: LoadBalancer
```

### Pattern 3: Edge Deployment

Deploy to edge devices, IoT gateways, or resource-constrained environments.

**Pros:**

- Minimal footprint (8MB binary)
- No external dependencies
- Works offline
- Low latency (local processing)

**Best for:**

- IoT devices
- Edge computing
- Offline-first applications
- Latency-sensitive workloads

#### Example: Raspberry Pi Deployment

```bash
# Cross-compile for ARM
GOOS=linux GOARCH=arm64 go build -o agent-arm64 main.go

# Deploy to device
scp agent-arm64 pi@192.168.1.100:/home/pi/aixgo/
scp config/agents.yaml pi@192.168.1.100:/home/pi/aixgo/config/

# Run as systemd service
ssh pi@192.168.1.100 'sudo systemctl start aixgo-agent'
```

**Systemd service file:**

```ini
# /etc/systemd/system/aixgo-agent.service
[Unit]
Description=Aixgo Agent
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/aixgo
ExecStart=/home/pi/aixgo/agent-arm64
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

## Configuration Management

### Environment-Based Configs

Use different configs for different environments:

```go
// main.go
package main

import (
    "fmt"
    "os"
    "github.com/aixgo-dev/aixgo"
    _ "github.com/aixgo-dev/aixgo/agents"
)

func main() {
    env := os.Getenv("ENV")
    if env == "" {
        env = "local"
    }

    configPath := fmt.Sprintf("config/agents-%s.yaml", env)
    if err := aixgo.Run(configPath); err != nil {
        panic(err)
    }
}
```

**Config files:**

- `config/agents-local.yaml` - Development
- `config/agents-staging.yaml` - Staging
- `config/agents-prod.yaml` - Production

### Secrets Management

Never hardcode API keys or secrets. Use environment variables or secret managers.

```yaml
# config/agents-prod.yaml
supervisor:
  name: coordinator

agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
    api_key: ${GROK_API_KEY} # From environment
    prompt: 'Analyze the data'
```

```bash
# Cloud Run
gcloud run deploy aixgo-agent \
  --set-env-vars GROK_API_KEY=your-secret-key

# Kubernetes Secret
kubectl create secret generic aixgo-secrets \
  --from-literal=GROK_API_KEY=your-secret-key

# Reference in deployment
env:
- name: GROK_API_KEY
  valueFrom:
    secretKeyRef:
      name: aixgo-secrets
      key: GROK_API_KEY
```

## Health Checks & Monitoring

### Health Endpoints

Implement health check endpoints for orchestration platforms:

```go
// health.go
package main

import (
    "net/http"
    "time"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
    // Check if agents are initialized
    if !agentsReady() {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("READY"))
}

func main() {
    http.HandleFunc("/health", healthHandler)
    http.HandleFunc("/ready", readyHandler)

    go func() {
        http.ListenAndServe(":8080", nil)
    }()

    // Start agents
    aixgo.Run("config/agents.yaml")
}
```

### Metrics & Logging

Use structured logging and metrics for observability:

```go
import (
    "log/slog"
    "os"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    slog.Info("Starting Aixgo agent", "env", os.Getenv("ENV"))

    if err := aixgo.Run("config/agents.yaml"); err != nil {
        slog.Error("Agent failed", "error", err)
        os.Exit(1)
    }
}
```

## Scaling Strategies

### Vertical Scaling

Increase resources for a single instance:

```yaml
# Kubernetes resources
resources:
  requests:
    memory: '256Mi' # Increase from 128Mi
    cpu: '200m' # Increase from 100m
  limits:
    memory: '1Gi'
    cpu: '1000m'
```

**When to use:**

- Single-instance bottleneck
- Memory-intensive LLM operations
- Before horizontal scaling

### Horizontal Scaling

Add more instances:

```yaml
# Kubernetes HPA
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: aixgo-agent-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: aixgo-agent
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

**When to use:**

- High request volume
- Redundancy requirements
- Multi-region deployment

## Best Practices

### 1. Use Minimal Base Images

Prefer `FROM scratch` or `alpine` for smallest attack surface:

```dockerfile
FROM scratch  # 0MB base
# or
FROM alpine:latest  # ~5MB base
```

### 2. Enable Observability from Day One

Configure OpenTelemetry before deploying:

```yaml
# config/agents-prod.yaml
observability:
  tracing: true
  service_name: 'aixgo-prod'
  exporter: 'otlp'
  endpoint: 'otel-collector:4317'
```

### 3. Set Resource Limits

Always define resource requests and limits:

```yaml
resources:
  requests: # Minimum guaranteed
    memory: '128Mi'
    cpu: '100m'
  limits: # Maximum allowed
    memory: '512Mi'
    cpu: '500m'
```

### 4. Use Rolling Updates

Deploy with zero downtime:

```yaml
# Kubernetes deployment strategy
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
```

### 5. Implement Graceful Shutdown

Handle termination signals properly:

```go
import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        slog.Info("Shutdown signal received")
        cancel()
    }()

    // Run agents with context
    if err := aixgo.RunWithContext(ctx, "config/agents.yaml"); err != nil {
        slog.Error("Agent failed", "error", err)
        os.Exit(1)
    }
}
```

### 6. Monitor Cold Start Times

Track startup performance:

```go
import "time"

func main() {
    startTime := time.Now()

    if err := aixgo.Run("config/agents.yaml"); err != nil {
        panic(err)
    }

    slog.Info("Agent started", "duration_ms", time.Since(startTime).Milliseconds())
}
```

Target: <100ms for serverless, <1s for containers

### 7. Use Configuration Validation

Validate configs before deployment:

```bash
# Pre-deployment validation
go run main.go --validate-config config/agents-prod.yaml
```

## Performance Benchmarks

Real-world production metrics:

| Metric          | Python (LangChain) | Aixgo        | Improvement        |
| --------------- | ------------------ | ------------ | ------------------ |
| Container Size  | 1.2GB              | 8MB          | 150x smaller       |
| Cold Start      | 45 seconds         | <100ms       | 450x faster        |
| Memory Baseline | 512MB              | 50MB         | 10x more efficient |
| Throughput      | 500-1,000 req/s    | 10,000 req/s | 10-20x higher      |

## Troubleshooting

### High Memory Usage

**Symptom:** OOM kills, high memory consumption

**Solutions:**

- Reduce max_rounds to limit workflow iterations
- Implement message batching
- Increase memory limits
- Use pagination for large datasets

### Slow Response Times

**Symptom:** High P99 latency

**Solutions:**

- Enable distributed tracing to identify bottlenecks
- Optimize LLM prompts (shorter = faster)
- Use faster models (gpt-3.5-turbo vs gpt-4)
- Add caching layer for repeated queries

### Failed Health Checks

**Symptom:** Pods restarting, traffic not routing

**Solutions:**

- Increase initialDelaySeconds for slow startup
- Check agent initialization logic
- Verify configuration is valid
- Review logs for startup errors

## Next Steps

- **[Observability & Monitoring](/guides/observability)** - Set up comprehensive monitoring
- **[Building Docker Images](/guides/docker-from-scratch)** - Optimize container builds
- **[Single Binary vs Distributed](/guides/single-vs-distributed)** - Understand scaling patterns
