# Cost Optimization Example

Demonstrates cost optimization through three complementary strategies: application-level caching, budget monitoring with OpenTelemetry, and the Router pattern for model selection.

## Overview

Cost optimization is an **infrastructure concern**, not a framework feature. Aixgo provides the primitives:

1. **Router pattern** for intelligent model selection (built-in)
2. **Observable metrics** via OpenTelemetry (built-in)
3. **Extensible agent interface** for caching wrappers (application-level)

**Result**: 60-80% cost reduction without framework changes.

## Quick Start

```bash
go run main.go
```

## Three-Pronged Approach

### 1. Application-Level Caching (Redis/In-Memory)

Wrap agents with caching layer:

```go
type CachedAgent struct {
    agent.Agent
    cache   *Cache
    ttl     time.Duration
}

func (c *CachedAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
    cacheKey := hashInput(input.Payload)

    // Check cache first
    if cached, found := c.cache.Get(cacheKey); found {
        return cached, nil  // Cache hit - no LLM cost!
    }

    // Cache miss - call underlying agent
    result, _ := c.Agent.Execute(ctx, input)
    c.cache.Set(cacheKey, result, c.ttl)
    return result, nil
}
```

**Results**:

- Cache hit rate: 40-60% for FAQ/support
- Cost savings: $0.03 per cached query
- Latency: ~100× faster (no LLM roundtrip)

### 2. Budget Monitoring with OpenTelemetry

Track costs and enforce limits:

```go
type BudgetMonitor struct {
    limit   float64
    metrics *CostMetrics
}

func (b *BudgetMonitor) CheckBudget() error {
    if b.metrics.TotalCost > b.limit {
        return fmt.Errorf("budget exceeded: $%.2f > $%.2f",
            b.metrics.TotalCost, b.limit)
    }
    return nil
}
```

**Integration with OpenTelemetry**:

```go
func RecordCost(ctx context.Context, agentName string, cost float64) {
    meter := otel.Meter("aixgo")
    counter, _ := meter.Float64Counter("llm.cost.total")
    counter.Add(ctx, cost,
        attribute.String("agent", agentName),
    )
}
```

### 3. Router Pattern for Model Selection (Built-in)

Use Aixgo's Router orchestration pattern:

```go
router := orchestration.NewRouter(
    "cost-optimizer",
    runtime,
    "complexity-classifier",
    map[string]string{
        "simple":  "cheap-model",   // gpt-3.5-turbo ($0.002/query)
        "complex": "expensive-model", // gpt-4 ($0.030/query)
    },
    orchestration.WithDefaultRoute("cheap-model"),
)
result, _ := router.Execute(ctx, input)
```

**Results**:

| Query | Complexity | Model | Cost | vs. Always GPT-4 |
|-------|-----------|-------|------|------------------|
| "What are your hours?" | simple | gpt-3.5-turbo | $0.002 | **93% savings** |
| "How to reset password?" | simple | gpt-3.5-turbo | $0.002 | **93% savings** |
| "Explain consensus algorithms" | complex | gpt-4 | $0.030 | 0% (needed) |

**Overall**: 25-50% cost reduction in production workloads.

## Combining All Three Strategies

```go
// 1. Router for model selection
router := orchestration.NewRouter("optimizer", runtime, "classifier", routes)

// 2. Wrap with caching
cache := NewRedisCache("localhost:6379")
cachedRouter := NewCachedAgent(router, cache, 5*time.Minute)

// 3. Add budget monitoring
metrics := NewCostMetrics()
monitor := NewBudgetMonitor(100.0, metrics)

// 4. Execute with all optimizations
if err := monitor.CheckBudget(); err != nil {
    return err
}

result, err := cachedRouter.Execute(ctx, input)
if err == nil {
    metrics.RecordCall(agentName, model, cost)
}
```

**Combined Results**:

- Cache hits: 40-60% (no cost)
- Router savings: 25-50% on cache misses
- Budget monitoring: Prevents overruns
- **Total savings: 60-80%**

## Real-World Production Setup

### 1. Redis Caching

```yaml
cache:
  type: redis
  addr: redis:6379
  ttl: 5m
  max_entries: 100000
```

### 2. OpenTelemetry Export

```yaml
observability:
  metrics:
    exporter: prometheus
    endpoint: :9090
  tracing:
    exporter: jaeger
    endpoint: jaeger:14268
```

### 3. Budget Alerts

```yaml
# prometheus-alerts.yaml
groups:
  - name: llm_costs
    rules:
      - alert: DailyCostHigh
        expr: increase(llm_cost_total[24h]) > 50
      - alert: CacheHitRateLow
        expr: llm_cache_hit_rate < 0.3
```

## Cost Optimization Checklist

- [ ] Implement caching for repeated queries (Redis)
- [ ] Set up OpenTelemetry metrics export
- [ ] Configure Router pattern for model selection
- [ ] Define budget limits per application/team
- [ ] Set up Prometheus alerts for cost thresholds
- [ ] Monitor cache hit rates and optimize TTLs
- [ ] Review cost breakdowns by agent/model weekly

## Why Framework Doesn't Include These

| Concern | Owner | Why |
|---------|-------|-----|
| **Caching** | Application | Strategy varies: Redis vs. in-memory, TTL policies |
| **Budget limits** | Business | Limits differ by org, team, use case |
| **Metrics backend** | Infrastructure | Prometheus vs. DataDog vs. New Relic |
| **Model selection** | Framework | ✅ **Router pattern provided** |

**Framework provides primitives, applications provide policies.**

## Common Patterns

1. **FAQ Bot** (High Cache Hit): 70-80% cache hit rate, 24hr TTL
2. **Dynamic Content** (Router + Short Cache): 30-40% cache hit, 40-50% router savings
3. **Cost-Capped API**: Budget monitor middleware

## Learn More

- [Router Cost Optimization Example](../router-cost-optimization/) - Router pattern deep dive
- [OpenTelemetry Integration Guide](/docs/observability.md) - Metrics setup
- [Production Deployment Guide](/docs/production.md) - Complete setup
