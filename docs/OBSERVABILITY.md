# Observability Guide

Aixgo provides built-in OpenTelemetry support for tracing agent execution and workflows. By default, traces are sent to **Langfuse**, but you can easily switch to any
OpenTelemetry-compatible backend.

**For comprehensive documentation, visit [https://aixgo.dev](https://aixgo.dev)**

## Table of Contents

- [Quick Start with Langfuse](#quick-start-with-langfuse)
- [Configuration Options](#configuration-options)
- [Alternative Backends](#alternative-backends)
- [Local Development](#local-development)
- [Custom Tracing](#custom-tracing)
- [Best Practices](#best-practices)

## Quick Start with Langfuse

Langfuse is a powerful LLM observability platform that helps you trace, analyze, and debug AI agent workflows.

### 1. Get Your Langfuse API Keys

1. Sign up at [https://cloud.langfuse.com](https://cloud.langfuse.com)
2. Create a new project
3. Copy your **Public Key** and **Secret Key** from the project settings

### 2. Configure Environment Variables

Create a `.env` file in your project root:

```bash
# Copy from .env.example
cp .env.example .env
```

Edit `.env` and add your Langfuse credentials:

```bash
LANGFUSE_PUBLIC_KEY=pk-lf-...
LANGFUSE_SECRET_KEY=sk-lf-...
```

### 3. Run Your Application

```bash
# Load environment variables and run
export $(cat .env | xargs) && go run examples/main.go
```

That's it! Your traces are now being sent to Langfuse.

### 4. View Traces in Langfuse

1. Go to your Langfuse project dashboard
2. Navigate to "Traces"
3. See real-time agent execution traces with:
   - Span timings
   - Input/output payloads
   - Error tracking
   - Agent-to-agent communication flow

## Configuration Options

Aixgo uses standard OpenTelemetry environment variables for configuration:

### Service Name

```bash
# Set a custom service name (default: "aixgo")
OTEL_SERVICE_NAME=my-ai-service
```

### Enable/Disable Tracing

```bash
# Disable tracing (default: true)
OTEL_TRACES_ENABLED=false
```

### Exporter Type

```bash
# Choose exporter type (default: "otlp")
# Options: "otlp", "stdout", "none"
OTEL_TRACES_EXPORTER=stdout
```

### OTLP Endpoint

```bash
# Override default Langfuse endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=https://your-endpoint.com/v1/traces
```

### Custom Headers

```bash
# Add custom headers (format: key1=value1,key2=value2)
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer token,Custom-Header=value
```

## Alternative Backends

Aixgo works with any OpenTelemetry-compatible backend. Here are popular options:

### Jaeger (Open Source)

```bash
# Run Jaeger locally
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Configure Aixgo
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318/v1/traces
```

View traces at: <http://localhost:16686>

### Honeycomb

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=https://api.honeycomb.io:443
OTEL_EXPORTER_OTLP_HEADERS=x-honeycomb-team=YOUR_API_KEY,x-honeycomb-dataset=YOUR_DATASET
```

### Grafana Cloud

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-prod-us-central-0.grafana.net/otlp
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic YOUR_BASE64_CREDENTIALS
```

### New Relic

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
OTEL_EXPORTER_OTLP_HEADERS=api-key=YOUR_LICENSE_KEY
```

### Datadog

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
# Make sure Datadog Agent is running with OTLP receiver enabled
```

### Self-Hosted Langfuse

```bash
# Run Langfuse locally
docker-compose -f docker-compose.langfuse.yml up -d

# Configure Aixgo
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:3000/api/public/otel
LANGFUSE_PUBLIC_KEY=your-local-public-key
LANGFUSE_SECRET_KEY=your-local-secret-key
```

## Local Development

### Stdout Exporter (Debugging)

For local development, use the stdout exporter to see traces in your console:

```bash
OTEL_TRACES_EXPORTER=stdout
```

Example output:

```json
{
  "Name": "react.think",
  "SpanContext": {
    "TraceID": "4b6f7e2a8c9d3f1e...",
    "SpanID": "8c9d3f1e2a4b6f7e"
  },
  "Attributes": [
    {
      "Key": "input",
      "Value": "Cosmic ray: 682.5 TeV"
    }
  ],
  "StartTime": "2025-01-15T10:30:45.123Z",
  "EndTime": "2025-01-15T10:30:45.456Z"
}
```

### Disable Tracing

For performance testing or when tracing is not needed:

```bash
OTEL_TRACES_ENABLED=false
# or
OTEL_TRACES_EXPORTER=none
```

## Custom Tracing

### Adding Custom Spans

You can add custom tracing to your code:

```go
import "github.com/aixgo-dev/aixgo/internal/observability"

func myFunction(ctx context.Context) {
    // Create a span
    span := observability.StartSpan("my-operation", map[string]any{
        "user_id": 123,
        "action": "process",
    })
    defer span.End()

    // Your code here

    // Add more attributes
    span.SetAttribute("result_count", 42)

    // Record errors
    if err != nil {
        span.SetError(err)
        return err
    }
}
```

### Using Parent Context

For proper trace hierarchy:

```go
func parentFunction(ctx context.Context) {
    // Create span with parent context
    ctx, span := observability.StartSpanWithContext(
        ctx,
        "parent-operation",
        map[string]any{"batch_size": 100},
    )
    defer span.End()

    // Pass context to child functions
    childFunction(ctx)
}

func childFunction(ctx context.Context) {
    // This span will be a child of the parent
    ctx, span := observability.StartSpanWithContext(
        ctx,
        "child-operation",
        nil,
    )
    defer span.End()

    // Your code here
}
```

## Best Practices

### 1. Always Use Defer

Always defer `span.End()` to ensure spans are closed even if errors occur:

```go
span := observability.StartSpan("operation", nil)
defer span.End() // ✅ Good

// vs

span := observability.StartSpan("operation", nil)
// ... code ...
span.End() // ❌ Bad - might be skipped on error
```

### 2. Add Meaningful Attributes

Include relevant context in span attributes:

```go
span := observability.StartSpan("process-message", map[string]any{
    "message_id": msg.ID,
    "message_type": msg.Type,
    "agent_name": "spectrum-analyzer",
    "payload_size": len(msg.Payload),
})
```

### 3. Record Errors

Always record errors in spans for better debugging:

```go
result, err := doSomething()
if err != nil {
    span.SetError(err)
    return err
}
```

### 4. Use Hierarchical Spans

Create child spans for sub-operations to visualize the execution flow:

```text
root-span
├── database-query
├── llm-call
│   ├── prepare-prompt
│   ├── api-request
│   └── parse-response
└── send-result
```

### 5. Sensitive Data

Avoid including sensitive data in span attributes:

```go
// ❌ Bad
span.SetAttribute("api_key", apiKey)
span.SetAttribute("password", password)

// ✅ Good
span.SetAttribute("api_key_present", apiKey != "")
span.SetAttribute("user_authenticated", true)
```

### 6. Performance Impact

Tracing adds minimal overhead, but for high-throughput scenarios:

```bash
# Sample traces instead of capturing everything
# (Most OTLP backends support sampling)
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1  # Sample 10% of traces
```

## Troubleshooting

### Traces Not Appearing

1. **Check credentials:**

   ```bash
   # Verify env vars are set
   echo $LANGFUSE_PUBLIC_KEY
   echo $LANGFUSE_SECRET_KEY
   ```

2. **Verify endpoint:**

   ```bash
   # Test connectivity
   curl https://cloud.langfuse.com/api/public/otel
   ```

3. **Enable stdout for debugging:**

   ```bash
   OTEL_TRACES_EXPORTER=stdout
   ```

4. **Check application logs:**

   ```bash
   # Look for observability initialization messages
   go run examples/main.go 2>&1 | grep -i observability
   ```

### High Latency

If observability is causing latency:

1. **Use batching** (enabled by default in Aixgo)
2. **Reduce attribute size** - avoid large payloads in span attributes
3. **Sample traces** for high-volume scenarios
4. **Use local OTLP collector** to batch and forward traces

### Missing Parent-Child Relationships

Always pass context through function calls:

```go
// ❌ Bad - creates orphaned spans
func processMessage(msg Message) {
    ctx := context.Background() // New context!
    span := observability.StartSpan("process", nil)
    defer span.End()
}

// ✅ Good - preserves trace hierarchy
func processMessage(ctx context.Context, msg Message) {
    ctx, span := observability.StartSpanWithContext(ctx, "process", nil)
    defer span.End()
}
```

## Advanced Configuration

### Programmatic Configuration

Instead of environment variables, you can configure observability programmatically:

```go
import "github.com/aixgo-dev/aixgo/internal/observability"

func main() {
    config := observability.Config{
        ServiceName:  "my-service",
        Enabled:      true,
        ExporterType: "otlp",
        OTLPEndpoint: "https://cloud.langfuse.com/api/public/otel",
        OTLPHeaders: map[string]string{
            "Authorization": "Basic pk-lf-...:sk-lf-...",
        },
    }

    if err := observability.Init(config); err != nil {
        log.Fatal(err)
    }
    defer observability.Shutdown(context.Background())

    // Run application
}
```

### Multiple Exporters

To send traces to multiple backends simultaneously, use an OpenTelemetry Collector:

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      http:

exporters:
  otlp/langfuse:
    endpoint: https://cloud.langfuse.com/api/public/otel
    headers:
      authorization: Basic ${LANGFUSE_CREDENTIALS}

  otlp/jaeger:
    endpoint: http://localhost:4318

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/langfuse, otlp/jaeger]
```

Then configure Aixgo to send to the collector:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

## Support

- **Langfuse Documentation:** <https://langfuse.com/docs>
- **OpenTelemetry Go:** <https://opentelemetry.io/docs/languages/go/>
- **Aixgo Issues:** <https://github.com/aixgo-dev/aixgo/issues>

---

**Next:** [Testing Guide](./TESTING_GUIDE.md) | [Pattern Catalog](./PATTERNS.md)
