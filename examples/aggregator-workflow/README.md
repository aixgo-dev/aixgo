# Multi-Agent Aggregator Workflow

Demonstrates multi-agent collaboration using the Aggregator agent to synthesize insights from specialized AI agents.

## Quick Start

```bash
cd examples/aggregator-workflow
export OPENAI_API_KEY="your-key"
go run main.go
```

## Overview

This example showcases a **Research Synthesis System** with:

- **6 specialized expert agents** - Technical, Data Science, Business, Security, Ethics, Domain experts
- **3 aggregation strategies** - Consensus, Semantic, and Weighted synthesis
- **Conflict resolution** - LLM-powered reasoning to resolve expert disagreements
- **Semantic clustering** - Group related insights thematically
- **Final synthesis** - Comprehensive analysis with recommendations

**Comprehensive Guide**: See [Classifier & Aggregator Examples](https://aixgo.dev/examples/classifier-aggregator/) for strategy selection, configuration options, and advanced patterns.

## Aggregation Strategies

| Strategy | When to Use | Key Feature |
|----------|-------------|-------------|
| **Consensus** | Building agreement | Identifies common ground, resolves conflicts |
| **Semantic** | Understanding themes | Groups by similarity, preserves relationships |
| **Weighted** | Expert prioritization | Applies expertise-based weights |

## Files

- `main.go` - Complete multi-agent workflow
- `config.yaml` - Expert definitions, aggregation settings
- `research_synthesis_output.json` - Detailed results (generated)

## Example Output

```text
=== CONSENSUS AGGREGATION ===
Consensus Level: 0.83
Content: Based on expert consensus, LLMs are transforming software
development with 83% agreement on key impacts...
Conflicts Resolved: 2

=== FINAL SYNTHESIS ===
Key Insights:
1. Technical Impact (Confidence: 0.92) - 40-60% acceleration
2. Security Considerations (Priority: 0.95) - New vulnerability classes
3. Business Implications (Confidence: 0.75) - Positive ROI at scale
```

## Related

- [Classifier Workflow](../classifier-workflow/) - Intelligent classification
- [Patterns Guide](../../docs/PATTERNS.md) - All orchestration patterns
