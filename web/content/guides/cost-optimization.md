---
title: 'Cost Optimization Strategies'
description: 'Optimize LLM costs using caching, monitoring, intelligent routing, and deterministic aggregation'
breadcrumb: 'Cost Optimization'
category: 'Production'
weight: 25
---

Aixgo provides built-in cost tracking and optimization features. This guide shows how to combine them with standard infrastructure tools (Redis, OpenTelemetry) for 25-80% cost reduction in production systems.

**Working Example**: See [cost-optimization](https://github.com/aixgo-dev/aixgo/tree/main/examples/cost-optimization) for a complete implementation demonstrating all three optimization strategies.

## Overview

LLM API costs can quickly escalate in production. Aixgo helps you optimize costs through:

- **Built-in cost tracking**: Automatic token usage and cost monitoring per agent
- **Application-level caching**: Cache responses using Redis or similar
- **Intelligent routing**: Use Router pattern to send queries to cost-appropriate models
- **Deterministic aggregation**: $0 voting strategies vs expensive LLM aggregation
- **Budget monitoring**: Alert on cost thresholds before overspend
- **Batch processing**: Process multiple items in single API calls

## Built-In Cost Tracking

Every LLM call is automatically tracked with detailed metrics.

### What's Tracked

- **Token usage**: Input and output tokens per call
- **Cost calculation**: Based on provider pricing tables
- **Per-agent metrics**: Know which agents cost most
- **OpenTelemetry integration**: Export to Prometheus, Datadog, Langfuse
- **Time-series data**: Track costs over time

### Querying Cost Metrics

```go
import (
    "github.com/aixgo-dev/aixgo/internal/observability"
)

func getCostMetrics() {
    // Total cost (all agents)
    totalCost := observability.GetMetric("llm.cost.total")
    fmt.Printf("Total LLM cost: $%.2f\n", totalCost)

    // Total tokens used
    tokensUsed := observability.GetMetric("llm.tokens.total")
    fmt.Printf("Total tokens: %d\n", tokensUsed)

    // Cost per agent
    agentCost := observability.GetMetricByLabel("llm.cost.total",
        map[string]string{"agent": "policy-analyzer"})
    fmt.Printf("Policy analyzer cost: $%.2f\n", agentCost)

    // Daily cost
    dailyCost := observability.GetMetric("llm.cost.daily")
    fmt.Printf("Today's cost: $%.2f\n", dailyCost)
}
```

### Export to Monitoring Systems

```yaml
# OpenTelemetry configuration
observability:
  enabled: true
  exporter: prometheus
  endpoint: http://prometheus:9090

  # Or use Langfuse for LLM-specific tracking
  # exporter: langfuse
  # langfuse_public_key: pk-lf-...
  # langfuse_secret_key: sk-lf-...
```

## Cost Optimization Strategies

### 1. Application-Level Caching (60-80% cost reduction)

**Why application-level caching?**

Caching is an infrastructure concern, not a framework concern. Use Redis, Memcached, or HTTP cache headers at the application layer.

**Benefits:**

- 60-80% cost reduction for repeated queries
- Framework stays focused on agent orchestration
- You control TTL, invalidation, cache keys
- Standard tools (Redis) with proven reliability

#### Redis Caching Wrapper

```go
package cache

import (
    "context"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/aixgo-dev/aixgo/pkg/agent"
)

type CachedAgent struct {
    cache *redis.Client
    agent agent.Agent
    ttl   time.Duration
}

func NewCachedAgent(rdb *redis.Client, ag agent.Agent, ttl time.Duration) *CachedAgent {
    return &CachedAgent{
        cache: rdb,
        agent: ag,
        ttl:   ttl,
    }
}

func (c *CachedAgent) Execute(ctx context.Context, msg *agent.Message) (*agent.Message, error) {
    // Generate cache key from message payload
    cacheKey := c.generateCacheKey(msg.Payload)

    // Check cache first
    if cached, err := c.cache.Get(ctx, cacheKey).Result(); err == nil {
        return &agent.Message{Payload: cached}, nil
    }

    // Cache miss - execute agent
    result, err := c.agent.Execute(ctx, msg)
    if err != nil {
        return nil, err
    }

    // Cache the result
    if err := c.cache.Set(ctx, cacheKey, result.Payload, c.ttl).Err(); err != nil {
        // Log cache write failure but don't fail the request
        fmt.Printf("Cache write failed: %v\n", err)
    }

    return result, nil
}

func (c *CachedAgent) generateCacheKey(payload string) string {
    hash := sha256.Sum256([]byte(payload))
    return fmt.Sprintf("agent:%s:%x", c.agent.Name(), hash)
}
```

#### Usage Example

```go
func main() {
    // Setup Redis
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // Create base agent
    baseAgent := agent.NewReact("policy-analyzer", model, "Analyze policy")

    // Wrap with caching
    cachedAgent := cache.NewCachedAgent(rdb, baseAgent, 1*time.Hour)

    // Use cached agent in workflow
    // Identical queries return cached results instantly
    result, err := cachedAgent.Execute(ctx, msg)
}
```

#### Cache Strategies

**Aggressive caching (24h+ TTL):**

- Static content analysis
- Historical data processing
- Infrequently changing queries
- **Cost reduction: 70-80%**

```go
cachedAgent := cache.NewCachedAgent(rdb, agent, 24*time.Hour)
```

**Moderate caching (1-6h TTL):**

- Semi-static queries
- Daily reports
- Moderate freshness requirements
- **Cost reduction: 40-60%**

```go
cachedAgent := cache.NewCachedAgent(rdb, agent, 3*time.Hour)
```

**Conservative caching (<1h TTL):**

- Real-time data with some tolerance
- Frequently changing content
- Short-term deduplication
- **Cost reduction: 20-40%**

```go
cachedAgent := cache.NewCachedAgent(rdb, agent, 30*time.Minute)
```

### 2. Intelligent Model Selection (25-50% cost reduction)

Use the **Router pattern** to route queries to cost-appropriate models.

#### Cost-Based Routing

```yaml
supervisor:
  name: cost-optimizer

agents:
  # Classify query complexity
  - name: complexity-classifier
    role: classifier
    model: gpt-4o-mini  # Use cheap model for classification
    inputs:
      - source: user-query
    outputs:
      - target: cheap-handler
        condition: 'category == simple'
      - target: mid-handler
        condition: 'category == moderate'
      - target: expensive-handler
        condition: 'category == complex'
    classifier_config:
      categories:
        - name: simple
          description: "Basic queries answerable with simple reasoning"
          examples:
            - "What is the capital of France?"
            - "Define machine learning"

        - name: moderate
          description: "Queries requiring moderate analysis"
          examples:
            - "Compare supervised vs unsupervised learning"
            - "Explain benefits of microservices"

        - name: complex
          description: "Queries requiring deep reasoning and analysis"
          examples:
            - "Design a distributed consensus algorithm"
            - "Analyze trade-offs in system architecture"

  # Cost-optimized handlers
  - name: cheap-handler
    role: react
    model: gpt-4o-mini  # $0.15/1M input tokens
    prompt: 'Handle simple query efficiently'
    inputs:
      - source: complexity-classifier

  - name: mid-handler
    role: react
    model: gpt-4-turbo  # $10/1M input tokens
    prompt: 'Handle moderate complexity query'
    inputs:
      - source: complexity-classifier

  - name: expensive-handler
    role: react
    model: gpt-4  # $30/1M input tokens
    prompt: 'Handle complex query with deep analysis'
    inputs:
      - source: complexity-classifier
```

#### Cost Comparison

**Without routing (all queries → GPT-4):**

- 1000 queries/day
- Average 500 tokens/query
- Cost: 1000 × 500 × $30/1M = $15/day
- **Monthly cost: $450**

**With routing (70% simple, 20% moderate, 10% complex):**

- Simple (700): 700 × 500 × $0.15/1M = $0.05/day
- Moderate (200): 200 × 500 × $10/1M = $1.00/day
- Complex (100): 100 × 500 × $30/1M = $1.50/day
- **Daily cost: $2.55**
- **Monthly cost: $76.50**

**Savings: 83% ($373.50/month)**

### 3. Deterministic Aggregation ($0 vs $$)

Use deterministic voting strategies instead of LLM aggregation when possible.

#### Cost Comparison: LLM vs Voting

**LLM-powered consensus:**

```yaml
aggregator_config:
  aggregation_strategy: consensus  # Uses LLM
```

- Cost: $0.02-0.05 per aggregation (depending on agent count)
- Speed: 2-5 seconds
- Quality: High (semantic synthesis)

**Deterministic voting:**

```yaml
aggregator_config:
  aggregation_strategy: voting_majority  # No LLM
```

- Cost: $0.00 per aggregation
- Speed: <1ms (instant)
- Quality: Good (for classification/voting tasks)

#### When to Use Each

**Use voting_majority when:**

- Classifying content (sentiment, category, priority)
- Binary decisions (approve/reject)
- Equal-weight agent opinions
- Reproducibility required

**Use LLM consensus when:**

- Narrative synthesis needed
- Semantic understanding required
- Conflict resolution complex
- Quality justifies cost

#### Cost Savings Example

1000 aggregations/day:

- **LLM consensus**: 1000 × $0.03 = $30/day ($900/month)
- **Deterministic voting**: 1000 × $0 = $0/day ($0/month)
- **Savings: $900/month**

### 4. Batch Processing (90%+ cost reduction)

Process multiple items in one API call instead of many individual calls.

#### Anti-Pattern (100 separate calls)

```go
// DON'T DO THIS - expensive!
for _, item := range items {
    result, err := llm.CreateStructured[Result](ctx, client, item, nil)
    results = append(results, result)
}
```

Cost: 100 calls × overhead = expensive

#### Optimized (1 batched call)

```go
// DO THIS - efficient!
type BatchResults struct {
    Results []Result `json:"results" validate:"required,dive"`
}

prompt := fmt.Sprintf("Analyze these items: %v", items)
batchResult, err := llm.CreateStructured[BatchResults](ctx, client, prompt, nil)
```

Cost: 1 call = 90%+ savings

#### Batch Size Optimization

```go
const (
    MinBatchSize = 10   // Below this, batching adds overhead
    MaxBatchSize = 100  // Above this, context window limits hit
)

func processBatch(ctx context.Context, items []Item) ([]Result, error) {
    if len(items) < MinBatchSize {
        // Process individually for small batches
        return processIndividually(ctx, items)
    }

    // Split into optimal batch sizes
    batches := splitIntoBatches(items, MaxBatchSize)

    var results []Result
    for _, batch := range batches {
        batchResults, err := processSingleBatch(ctx, batch)
        if err != nil {
            return nil, err
        }
        results = append(results, batchResults...)
    }

    return results, nil
}
```

### 5. Budget Monitoring and Alerts

Track spending and alert on thresholds before overspend.

#### Budget Monitor Implementation

```go
package budget

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/aixgo-dev/aixgo/internal/observability"
)

type BudgetMonitor struct {
    dailyLimit   float64
    warningLevel float64  // Percentage of limit for warnings (e.g., 0.8 = 80%)
    alertFunc    func(cost float64, limit float64, level string)
}

func NewBudgetMonitor(dailyLimit float64, warningLevel float64, alertFunc func(float64, float64, string)) *BudgetMonitor {
    return &BudgetMonitor{
        dailyLimit:   dailyLimit,
        warningLevel: warningLevel,
        alertFunc:    alertFunc,
    }
}

func (b *BudgetMonitor) CheckBudget(ctx context.Context) error {
    cost := observability.GetMetric("llm.cost.daily")

    // Critical: Budget exceeded
    if cost > b.dailyLimit {
        b.alertFunc(cost, b.dailyLimit, "critical")
        return fmt.Errorf("daily budget exceeded: $%.2f > $%.2f",
            cost, b.dailyLimit)
    }

    // Warning: Approaching limit
    if cost > b.dailyLimit*b.warningLevel {
        b.alertFunc(cost, b.dailyLimit, "warning")
        log.Printf("Warning: %.0f%% of daily budget used ($%.2f of $%.2f)",
            (cost/b.dailyLimit)*100, cost, b.dailyLimit)
    }

    return nil
}

func (b *BudgetMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := b.CheckBudget(ctx); err != nil {
                log.Printf("Budget check failed: %v", err)
            }
        case <-ctx.Done():
            return
        }
    }
}
```

#### Usage

```go
func main() {
    // Setup budget monitoring
    monitor := budget.NewBudgetMonitor(
        100.0,  // $100 daily limit
        0.8,    // Alert at 80% usage
        func(cost, limit float64, level string) {
            // Send alert (Slack, PagerDuty, etc.)
            if level == "critical" {
                sendAlert(fmt.Sprintf("CRITICAL: Budget exceeded! $%.2f > $%.2f", cost, limit))
            } else {
                sendAlert(fmt.Sprintf("WARNING: 80%% budget used ($%.2f)", cost))
            }
        },
    )

    // Start background monitoring
    ctx := context.Background()
    go monitor.StartMonitoring(ctx, 5*time.Minute)

    // Run your application
    // Monitor will alert on threshold violations
}
```

### 6. Model Selection by Task

Different models have different cost/quality tradeoffs. Choose appropriately.

#### Model Cost Comparison (as of 2024)

| Model | Input Cost | Output Cost | Use Case |
|-------|-----------|-------------|----------|
| gpt-4o-mini | $0.15/1M | $0.60/1M | Simple queries, classification |
| gpt-4-turbo | $10/1M | $30/1M | Moderate complexity |
| gpt-4 | $30/1M | $60/1M | Complex reasoning |
| claude-3-haiku | $0.25/1M | $1.25/1M | Fast, simple tasks |
| claude-3-sonnet | $3/1M | $15/1M | Balanced quality/cost |
| claude-3-opus | $15/1M | $75/1M | Highest quality |

#### Task-Optimized Model Selection

```yaml
agents:
  # Simple classification - cheapest model
  - name: content-classifier
    role: classifier
    model: gpt-4o-mini
    prompt: 'Classify content category'

  # Data extraction - mid-tier model
  - name: data-extractor
    role: react
    model: gpt-4-turbo
    prompt: 'Extract structured data from text'

  # Complex analysis - expensive model
  - name: deep-analyzer
    role: react
    model: gpt-4
    prompt: 'Perform deep analytical reasoning'

  # High-volume logging - no LLM needed
  - name: logger
    role: logger
    # No model cost - just writes to storage
```

## Cost Optimization Checklist

Use this checklist for production systems:

- [ ] **Enable cost tracking**: OpenTelemetry integration configured
- [ ] **Implement caching**: Redis wrapper for frequently accessed data
- [ ] **Use Router pattern**: Route to cheap models when appropriate
- [ ] **Choose voting over LLM**: Use deterministic aggregation when possible
- [ ] **Batch when possible**: Combine multiple items in single calls
- [ ] **Monitor budgets**: Alert at 80% threshold
- [ ] **Review expensive agents**: Weekly analysis of high-cost agents
- [ ] **Optimize prompts**: Shorter prompts reduce token costs
- [ ] **Set max_tokens limits**: Prevent unexpectedly long responses
- [ ] **Use appropriate models**: Match model tier to task complexity

## Real-World Cost Optimization Example

### Before Optimization

**System:** Customer support ticket analysis

**Configuration:**

- 10,000 tickets/day
- All tickets processed by GPT-4
- No caching
- LLM aggregation for all summaries
- Average 800 tokens per ticket

**Daily Cost:**

- Tickets: 10,000 × 800 tokens × $30/1M = $240/day
- Aggregation: 1,000 summaries × $0.03 = $30/day
- **Total: $270/day ($8,100/month)**

### After Optimization

**Optimizations applied:**

1. **60% cache hit rate** (Redis, 6h TTL)
2. **Router pattern** (70% simple → gpt-4o-mini, 30% complex → gpt-4)
3. **Deterministic voting** for priority classification
4. **Batch processing** for summaries (10 tickets/batch)

**Daily Cost:**

- Cached (6,000): $0
- Simple (2,800): 2,800 × 800 × $0.15/1M = $0.34/day
- Complex (1,200): 1,200 × 800 × $30/1M = $28.80/day
- Priority voting: $0 (deterministic)
- Batch summaries: 100 batches × $0.30 = $30/day
- **Total: $59.14/day ($1,774/month)**

**Savings: 78% ($6,326/month)**

### Optimization ROI

| Optimization | Cost Reduction | Effort | ROI |
|-------------|----------------|--------|-----|
| Redis caching | $162/day | 2 days | High |
| Router pattern | $40/day | 1 day | Very High |
| Voting aggregation | $30/day | 4 hours | Very High |
| Batch processing | $10/day | 4 hours | High |

## Monitoring Cost Trends

### Export to Prometheus

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'aixgo'
    scrape_interval: 30s
    static_configs:
      - targets: ['localhost:9090']
```

### Grafana Dashboard

Create dashboard with:

- Daily cost trend
- Cost per agent
- Token usage by model
- Cache hit rate
- Cost per query (moving average)

### Alerts

```yaml
# alerts.yml
groups:
  - name: cost_alerts
    rules:
      - alert: HighDailyCost
        expr: llm_cost_daily > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Daily LLM cost exceeds $100"

      - alert: CriticalDailyCost
        expr: llm_cost_daily > 200
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "CRITICAL: Daily cost exceeds $200"

      - alert: LowCacheHitRate
        expr: cache_hit_rate < 0.4
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Cache hit rate below 40%"
```

## Best Practices

1. **Start with monitoring**: Can't optimize what you don't measure
2. **Cache aggressively**: Most queries are repeated
3. **Route intelligently**: 70%+ of queries are simple
4. **Batch when possible**: Dramatic cost savings for multi-item processing
5. **Use deterministic aggregation**: Free vs expensive for voting tasks
6. **Set budget alerts**: Prevent overspend surprises
7. **Review monthly**: Identify high-cost agents and optimize
8. **Test caching behavior**: Ensure cache invalidation works correctly
9. **Monitor cache hit rates**: >50% hit rate indicates good caching strategy
10. **Use appropriate models**: Don't use GPT-4 for classification

## See Also

- [Router Pattern](./multi-agent-orchestration/#router-pattern) - Intelligent routing
- [Aggregator Strategies](./agent-types/#aggregator-agent) - Deterministic vs LLM aggregation
- [Observability Guide](./observability/) - Metrics and monitoring
- [Production Deployment](./production-deployment/) - Production best practices
