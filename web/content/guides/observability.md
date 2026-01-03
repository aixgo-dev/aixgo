---
title: 'Observability & Monitoring'
description: 'Instrument Aixgo agents with OpenTelemetry, distributed tracing, and structured logging for production visibility.'
breadcrumb: 'Deployment'
category: 'Deployment'
weight: 9
---

Production AI systems need comprehensive observability. This guide shows how to instrument Aixgo agents with OpenTelemetry, distributed tracing, metrics, and structured logging.

## Built-in Observability

Aixgo includes comprehensive observability features out of the box:

- **OpenTelemetry** - OTLP export for distributed tracing
- **Langfuse Integration** - LLM-specific analytics via OTLP
- **Prometheus Metrics** - Production monitoring and alerting
- **Health Checks** - Liveness and readiness probes

### Enable with Configuration

```yaml
# config/agents.yaml
supervisor:
  name: coordinator
  max_rounds: 10

observability:
  tracing: true
  service_name: 'aixgo-production'
  exporter: 'otlp'
  endpoint: 'localhost:4317'
  sampling_rate: 1.0 # 100% of traces

agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
```

That's it. Distributed tracing is now enabled for your entire agent system.

## Distributed Tracing

### How Tracing Works

Every message flow is automatically traced:

```text
Request → Producer → Analyzer → Logger → Response
   |          |          |         |          |
  span1     span2      span3     span4      span5
```

Each span includes:

- Agent name
- Processing duration
- Input/output messages
- LLM API calls
- Tool executions
- Errors

### Viewing Traces

Aixgo exports traces to any OpenTelemetry-compatible backend:

**Jaeger (Open Source):**

```yaml
# docker-compose.yml
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - '4317:4317' # OTLP gRPC
      - '16686:16686' # Jaeger UI
```

```bash
# Start Jaeger
docker-compose up -d

# View traces at http://localhost:16686
```

**Grafana Cloud:**

```yaml
observability:
  tracing: true
  service_name: 'aixgo-prod'
  exporter: 'otlp'
  endpoint: 'https://otlp-gateway-prod-us-central-0.grafana.net/otlp'
  headers:
    authorization: 'Bearer ${GRAFANA_API_KEY}'
```

**Datadog:**

```yaml
observability:
  tracing: true
  service_name: 'aixgo-prod'
  exporter: 'datadog'
  endpoint: 'http://datadog-agent:8126'
```

### Trace Attributes

Aixgo automatically adds contextual attributes:

```json
{
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "service.name": "aixgo-production",
  "agent.name": "analyzer",
  "agent.role": "react",
  "agent.model": "gpt-4-turbo",
  "message.id": "msg-123",
  "llm.provider": "xai",
  "llm.model": "gpt-4-turbo",
  "llm.tokens.input": 150,
  "llm.tokens.output": 75,
  "llm.latency_ms": 850,
  "tool.name": "query_database",
  "tool.duration_ms": 120
}
```

## Structured Logging

### Default JSON Logging

Aixgo uses structured logging with automatic trace correlation:

```go
import "log/slog"

func main() {
    // Configure JSON logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    slog.Info("Starting Aixgo agent",
        "version", "1.0.0",
        "env", os.Getenv("ENV"),
    )

    if err := aixgo.Run("config/agents.yaml"); err != nil {
        slog.Error("Agent failed", "error", err)
        os.Exit(1)
    }
}
```

**Output:**

```json
{
  "time": "2025-11-16T10:30:00Z",
  "level": "INFO",
  "msg": "Starting Aixgo agent",
  "version": "1.0.0",
  "env": "production",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

### Contextual Logging

Log with trace context automatically:

```go
func processMessage(ctx context.Context, msg string) {
    slog.InfoContext(ctx, "Processing message",
        "message_length", len(msg),
        "agent", "analyzer",
    )
    // trace_id and span_id added automatically
}
```

## Metrics

### Built-in Metrics

Aixgo exports these metrics automatically:

**Agent Metrics:**

- `aixgo_agent_messages_total` - Total messages processed per agent
- `aixgo_agent_errors_total` - Total errors per agent
- `aixgo_agent_duration_seconds` - Processing duration histogram

**LLM Metrics:**

- `aixgo_llm_requests_total` - Total LLM API calls
- `aixgo_llm_tokens_input` - Input tokens consumed
- `aixgo_llm_tokens_output` - Output tokens generated
- `aixgo_llm_latency_seconds` - LLM API latency histogram

**Supervisor Metrics:**

- `aixgo_supervisor_rounds_total` - Total workflow rounds
- `aixgo_supervisor_active_agents` - Number of active agents
- `aixgo_supervisor_message_queue_size` - Pending messages

### Prometheus Integration

```yaml
# config/agents.yaml
observability:
  tracing: true
  metrics: true
  metrics_port: 9090 # Prometheus scrape endpoint
```

**Prometheus configuration:**

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'aixgo'
    static_configs:
      - targets: ['localhost:9090']
```

### Custom Metrics

Add application-specific metrics:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dataProcessed = promauto.NewCounter(prometheus.CounterOpts{
        Name: "myapp_data_processed_total",
        Help: "Total data records processed",
    })

    processingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
        Name: "myapp_processing_duration_seconds",
        Help: "Time spent processing data",
        Buckets: prometheus.DefBuckets,
    })
)

func processData(data string) {
    timer := prometheus.NewTimer(processingDuration)
    defer timer.ObserveDuration()

    // Process data
    // ...

    dataProcessed.Inc()
}
```

## Health Checks

Aixgo exposes liveness and readiness endpoints for container orchestration:

```yaml
# config/agents.yaml
observability:
  health_port: 8080 # Health check endpoint
```

**Endpoints:**

- `/healthz` - Liveness probe (is the process running?)
- `/readyz` - Readiness probe (is the service ready to accept traffic?)

**Kubernetes example:**

```yaml
# deployment.yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Integration with Observability Platforms

### Grafana Stack (Loki + Tempo + Mimir)

Complete observability setup:

```yaml
# docker-compose.yml
services:
  # Logs
  loki:
    image: grafana/loki:latest
    ports:
      - '3100:3100'

  # Traces
  tempo:
    image: grafana/tempo:latest
    ports:
      - '4317:4317' # OTLP gRPC
      - '3200:3200' # Tempo UI

  # Metrics
  prometheus:
    image: prom/prometheus:latest
    ports:
      - '9090:9090'
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  # Dashboards
  grafana:
    image: grafana/grafana:latest
    ports:
      - '3000:3000'
    environment:
      - GF_AUTH_ANONYMOUS_ENABLED=true
```

**Aixgo configuration:**

```yaml
observability:
  tracing: true
  metrics: true
  service_name: 'aixgo-prod'
  exporter: 'otlp'
  endpoint: 'tempo:4317'
  metrics_port: 9090
```

### Datadog

```yaml
# docker-compose.yml
services:
  datadog-agent:
    image: gcr.io/datadoghq/agent:latest
    environment:
      - DD_API_KEY=${DATADOG_API_KEY}
      - DD_SITE=datadoghq.com
      - DD_LOGS_ENABLED=true
      - DD_APM_ENABLED=true
    ports:
      - '8126:8126' # APM
```

**Aixgo configuration:**

```yaml
observability:
  tracing: true
  service_name: 'aixgo-prod'
  exporter: 'datadog'
  endpoint: 'datadog-agent:8126'
```

### Langfuse (LLM-Specific)

Track LLM calls, costs, and performance:

```yaml
observability:
  tracing: true
  llm_observability:
    enabled: true
    provider: 'langfuse'
    endpoint: 'https://cloud.langfuse.com'
    public_key: ${LANGFUSE_PUBLIC_KEY}
    secret_key: ${LANGFUSE_SECRET_KEY}
```

Langfuse dashboard shows:

- LLM call traces
- Token usage and costs
- Model performance comparisons
- Prompt engineering analytics

## Debugging Multi-Agent Workflows

### Trace Visualization

View message flow across agents:

```text
Request [trace_id: abc123]
  ├─ producer (50ms)
  │   └─ generate_data
  ├─ analyzer (1.2s)
  │   ├─ llm_call [gpt-4-turbo] (850ms)
  │   └─ tool_call [query_db] (120ms)
  └─ logger (30ms)
      └─ persist_result

Total: 1.28s
```

### Identifying Bottlenecks

Sort spans by duration to find slow operations:

| Agent    | Operation     | Duration | % of Total |
| -------- | ------------- | -------- | ---------- |
| analyzer | llm_call      | 850ms    | 66%        |
| analyzer | tool_call     | 120ms    | 9%         |
| producer | generate_data | 50ms     | 4%         |

**Insight:** LLM call is the bottleneck (66% of time)

**Actions:**

- Use faster model (gpt-3.5-turbo instead of gpt-4)
- Optimize prompt length
- Add caching for repeated queries

## Alerting

### Prometheus Alerts

```yaml
# alerts.yml
groups:
  - name: aixgo
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: rate(aixgo_agent_errors_total[5m]) > 0.1
        for: 5m
        annotations:
          summary: 'High error rate in {{ $labels.agent }}'

      # Slow LLM calls
      - alert: SlowLLMCalls
        expr: histogram_quantile(0.95, aixgo_llm_latency_seconds) > 5
        for: 10m
        annotations:
          summary: '95th percentile LLM latency > 5s'

      # High token usage
      - alert: HighTokenUsage
        expr: rate(aixgo_llm_tokens_input[1h]) > 100000
        for: 1h
        annotations:
          summary: 'High token consumption (cost concern)'
```

### Grafana Alerts

Create dashboards with alerts:

```json
{
  "dashboard": {
    "title": "Aixgo Agent Monitoring",
    "panels": [
      {
        "title": "Agent Throughput",
        "targets": [
          {
            "expr": "rate(aixgo_agent_messages_total[5m])"
          }
        ],
        "alert": {
          "conditions": [
            {
              "evaluator": { "type": "lt", "params": [10] },
              "query": { "params": ["A", "5m", "now"] }
            }
          ]
        }
      }
    ]
  }
}
```

## Best Practices

### 1. Always Enable Tracing in Production

```yaml
# config/agents-prod.yaml
observability:
  tracing: true
  sampling_rate: 0.1 # 10% sampling to reduce overhead
```

### 2. Use Structured Logging

```go
// ❌ Bad: unstructured
log.Println("Error processing message:", err)

// ✅ Good: structured
slog.Error("Message processing failed",
    "error", err,
    "agent", "analyzer",
    "message_id", msgID,
)
```

### 3. Set SLOs and Monitor Them

Define service level objectives:

- **Latency:** P95 < 2 seconds
- **Error rate:** < 1%
- **Throughput:** > 100 msg/s

Monitor with alerts.

### 4. Correlate Logs with Traces

Always log with context:

```go
slog.InfoContext(ctx, "Processing started")
// trace_id and span_id automatically included
```

### 5. Track LLM Costs

Monitor token usage to control costs:

```go
// Aixgo tracks this automatically
// aixgo_llm_tokens_input
// aixgo_llm_tokens_output

// Calculate cost
costPerToken := 0.00001  // $0.01 per 1K tokens
totalCost := totalTokens * costPerToken
```

## Troubleshooting Common Issues

### Missing Traces

**Symptom:** Traces not appearing in backend

**Check:**

1. Endpoint configured correctly
2. Network connectivity to OTLP endpoint
3. Firewall rules allow port 4317 (OTLP)
4. API keys/credentials set correctly

**Debug:**

```yaml
observability:
  tracing: true
  debug: true # Enable verbose OTLP logging
```

### High Overhead

**Symptom:** Tracing impacting performance

**Solution:**

```yaml
observability:
  tracing: true
  sampling_rate: 0.1 # Sample 10% instead of 100%
```

### Incomplete Spans

**Symptom:** Some spans missing from trace

**Cause:** Long-running operations timing out

**Solution:**

```yaml
observability:
  tracing: true
  span_timeout: 60s # Increase timeout
```

## Example: Complete Observability Setup

```yaml
# config/agents-prod.yaml
supervisor:
  name: coordinator
  max_rounds: 10

observability:
  # Tracing
  tracing: true
  service_name: 'aixgo-production'
  exporter: 'otlp'
  endpoint: 'tempo:4317'
  sampling_rate: 0.2

  # Metrics
  metrics: true
  metrics_port: 9090

  # LLM-specific
  llm_observability:
    enabled: true
    provider: 'langfuse'
    endpoint: 'https://cloud.langfuse.com'
    public_key: ${LANGFUSE_PUBLIC_KEY}
    secret_key: ${LANGFUSE_SECRET_KEY}

agents:
  - name: analyzer
    role: react
    model: gpt-4-turbo
    prompt: 'Analyze the data'
```

**Application code:**

```go
package main

import (
    "log/slog"
    "os"
    "github.com/aixgo-dev/aixgo"
    _ "github.com/aixgo-dev/aixgo/agents"
)

func main() {
    // Structured JSON logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)

    slog.Info("Starting Aixgo agent",
        "version", "1.0.0",
        "env", "production",
    )

    // Run with observability enabled
    if err := aixgo.Run("config/agents-prod.yaml"); err != nil {
        slog.Error("Agent failed", "error", err)
        os.Exit(1)
    }
}
```

**Result:**

- ✅ Distributed traces in Tempo/Grafana
- ✅ Metrics in Prometheus
- ✅ LLM analytics in Langfuse
- ✅ Structured logs with trace correlation

## Key Takeaways

1. **Built-in observability** - OpenTelemetry included, no manual instrumentation
2. **Distributed tracing** - Track messages across multi-agent workflows
3. **Structured logging** - JSON logs with automatic trace correlation
4. **Metrics export** - Agent, LLM, and supervisor metrics
5. **Platform integration** - Works with Grafana, Datadog, Langfuse, etc.

Production AI systems are complex. Comprehensive observability is not optional—it's essential.

## Next Steps

- **[Production Deployment](/guides/production-deployment)** - Deploy with monitoring
- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Debug complex workflows
- **[Building Docker Images](/guides/docker-from-scratch)** - Container-based deployments
