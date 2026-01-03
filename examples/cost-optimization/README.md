# Cost Optimization Example

Demonstrates cost reduction through caching, budget monitoring, and the Router pattern for intelligent model selection.

## Quick Start

```bash
cd examples/cost-optimization
export OPENAI_API_KEY="your-key"
go run main.go
```

## Overview

This example shows three complementary cost optimization strategies:

- **Application-level caching** - 40-60% cache hit rate, ~100× faster responses
- **Budget monitoring** - Track costs via OpenTelemetry, enforce limits
- **Router pattern** - 25-50% savings by routing simple queries to cheaper models

**Combined savings: 60-80% cost reduction**

**Comprehensive Guide**: See [Cost Optimization](https://aixgo.dev/guides/cost-optimization/) for detailed configuration, production setup, and best practices.

## Files

- `main.go` - Implementation demonstrating all three strategies
- Cache wrapper, budget monitor, and Router pattern examples

## Key Results

| Strategy | Savings | Use Case |
|----------|---------|----------|
| Caching | 40-60% | FAQ, repeated queries |
| Router | 25-50% | Mixed complexity workload |
| Combined | 60-80% | Production systems |

## Related

- [Router Cost Optimization](../router-cost-optimization/) - Router pattern deep dive
- [OpenTelemetry Guide](../../docs/OBSERVABILITY.md) - Metrics and monitoring
