# Customer Support Ticket Classification Workflow

Production-ready AI-powered ticket classification using the Classifier agent with confidence scoring and intelligent routing.

## Quick Start

```bash
cd examples/classifier-workflow
export OPENAI_API_KEY="your-key"
go run main.go
```

## Overview

This example demonstrates:

- **Semantic classification** into predefined categories (technical, billing, account, etc.)
- **Confidence scoring** for quality assessment (0-1 scale)
- **AI reasoning** explaining classification decisions
- **Intelligent routing** to appropriate teams with priority assignment
- **Few-shot learning** for improved accuracy

**Comprehensive Guide**: See [Classifier & Aggregator Examples](https://aixgo.dev/examples/classifier-aggregator/) for detailed configuration, integration patterns, and production deployment.

## Key Features

- Chain-of-thought reasoning for transparent decisions
- Structured JSON outputs with schema validation
- Alternative classifications when confidence is low
- Multi-label support for complex tickets
- Customizable categories and routing rules

## Files

- `main.go` - Complete workflow implementation
- `config.yaml` - Category definitions, few-shot examples, routing rules

## Example Output

```text
Processing ticket: Cannot access my account after password reset
  ✓ Classified as: account_access (Confidence: 0.92)
  → Routed to: Security Team (Priority: high)

AI Reasoning: Authentication issues after password reset indicate
account access problems requiring immediate security attention.
```

## Related

- [Aggregator Workflow](../aggregator-workflow/) - Multi-agent synthesis
- [Security Best Practices](../../docs/SECURITY_BEST_PRACTICES.md) - Production security
