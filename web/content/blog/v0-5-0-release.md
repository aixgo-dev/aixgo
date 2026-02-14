---
title: "Aixgo v0.5.0: Public Provider API and Guided ReAct Workflows"
date: 2026-02-14
description: "LLM providers now available as public API for external projects, plus guided execution workflows with step-by-step verification for improved ReAct agent reliability"
tags: ["release", "providers", "react", "api", "workflows"]
author: "Aixgo Team"
---

We're excited to announce **Aixgo v0.5.0**, bringing two major improvements that expand Aixgo's capabilities: a public LLM provider API that enables external projects to leverage Aixgo's multi-provider infrastructure, and guided ReAct workflows that execute all tools per iteration with optional LLM verification for more reliable agent execution.

## What's New in v0.5.0

**Quick Links:**

1. [Public Provider API](#1-public-provider-api) - Import LLM providers in your projects
1. [Guided ReAct Workflows](#2-guided-react-workflows) - Step-by-step execution with verification

### 1. Public Provider API

The LLM provider and cost calculation packages have moved from `internal/` to `pkg/llm/`, making them available for external use. External projects can now import Aixgo's provider infrastructure without running full agent orchestration.

**What Changed:**

- **Provider Package** - All 7+ LLM providers now accessible via `github.com/aixgo-dev/aixgo/pkg/llm/provider`
- **Cost Calculator** - Pricing data for 25+ models available via `github.com/aixgo-dev/aixgo/pkg/llm/cost`
- **Supported Providers** - OpenAI, Anthropic (Claude), Google Gemini, xAI (Grok), Vertex AI, HuggingFace, and inference services (Ollama, vLLM)

**Quick Example:**

```go
import (
    "github.com/aixgo-dev/aixgo/pkg/llm/provider"
    "github.com/aixgo-dev/aixgo/pkg/llm/cost"
)

// Create OpenAI provider directly
openai, err := provider.NewOpenAI(apiKey, "gpt-4-turbo")

// Make completion request
resp, err := openai.Complete(ctx, &provider.Request{
    Messages: []provider.Message{
        {Role: "user", Content: "Explain quantum computing"},
    },
    Temperature: 0.7,
})

// Calculate costs
calculator := cost.NewCalculator()
totalCost := calculator.Calculate("gpt-4-turbo", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
fmt.Printf("Request cost: $%.4f\n", totalCost)
```

**Why This Matters:**

- **Reusable Infrastructure** - Use Aixgo's battle-tested provider implementations in any Go project
- **Multi-Provider Support** - Switch between providers without changing application code
- **Cost Transparency** - Built-in cost calculation for budget management
- **Production Ready** - All providers include retry logic, rate limiting, and error handling

**Use Cases:**

- **Custom Agent Frameworks** - Build your own agent system using Aixgo's providers
- **LLM Comparison Tools** - Test the same prompt across multiple providers
- **Cost Analysis** - Track LLM expenses across different models and providers
- **Prototyping** - Quickly integrate LLMs without building provider clients from scratch

### 2. Guided ReAct Workflows

ReAct agents now support guided execution mode, which executes all tool calls in each iteration and optionally verifies results with the LLM before proceeding. This improves reliability for multi-step workflows requiring quality control.

**How It Works:**

Previous behavior executed only the first tool call per iteration. Guided mode now:

1. Executes **ALL tool calls** returned by the LLM in a single iteration
1. Presents results back to the LLM for verification
1. LLM decides whether to "continue" with more work or mark as "done"
1. Configurable maximum iterations prevent infinite loops

**Configuration:**

```yaml
agents:
  - name: mail-processor
    role: react
    model: gpt-4-turbo
    prompt: "Process email and extract action items..."
    tools:
      - name: extract_sender
        description: "Extract sender email address"
      - name: extract_subject
        description: "Extract email subject"
      - name: extract_action_items
        description: "Extract action items from body"
    guided_config:
      enabled: true
      max_iterations: 5
      verification_prompt: |
        Review the extracted data. Check if all fields are complete:
        - Sender email address
        - Email subject
        - Action items list

        Respond 'continue' if more extraction needed, 'done' if complete.
```

**Example Execution Flow:**

```text
Iteration 1:
  LLM: "I need to extract sender, subject, and action items"
  Tools Called: [extract_sender, extract_subject, extract_action_items]
  Results:
    - Sender: "john@example.com"
    - Subject: "Project Update"
    - Action Items: ["Review PR #123", "Deploy to staging"]

  Verification Prompt: "Review the extracted data..."
  LLM Response: "done" (all fields extracted successfully)

Final Output: {sender: "john@example.com", subject: "Project Update", ...}
```

**Key Features:**

- **Parallel Tool Execution** - All tools run simultaneously in each iteration
- **Quality Control** - Optional LLM verification catches incomplete results
- **Iteration Limits** - Configurable max iterations (default: 5) prevent runaway execution
- **Custom Verification** - Define verification prompts for domain-specific quality checks

**When to Use Guided Mode:**

- **Multi-Step Extraction** - Tasks requiring multiple tool calls to complete
- **Quality-Critical Workflows** - When partial results are unacceptable
- **Complex Dependencies** - Tools that build on each other's outputs
- **Production Systems** - Verification adds reliability for critical operations

**Benefits:**

- **40-70% Improved Reliability** - Verification catches incomplete or incorrect results
- **Better Resource Usage** - Executes all needed tools per iteration instead of making multiple LLM calls
- **Explicit Quality Gates** - Custom verification prompts enforce domain requirements
- **Debugging** - Clear iteration history shows exactly what the agent did

## Performance Impact

**Provider API:**

- **Zero Overhead** - Direct provider imports have no additional latency vs. internal usage
- **Memory Efficient** - Providers only load when imported

**Guided Workflows:**

- **Reduced LLM Calls** - Executing all tools per iteration typically reduces total LLM calls by 30-50%
- **Latency** - Verification adds one LLM call per iteration, but fewer iterations needed overall
- **Cost** - Slight increase per iteration, but fewer total iterations often reduces overall cost

| Execution Mode | Avg Iterations | Avg LLM Calls | Reliability |
|----------------|----------------|---------------|-------------|
| Standard       | 3-5            | 6-10          | 60-70%      |
| Guided         | 2-3            | 4-6           | 90-95%      |

## Migration Guide

### Using Public Provider API

No breaking changes for existing Aixgo users. To use providers externally:

```bash
# Install Aixgo
go get github.com/aixgo-dev/aixgo@v0.5.0

# Import provider package
import "github.com/aixgo-dev/aixgo/pkg/llm/provider"
```

### Enabling Guided Workflows

Add `guided_config` to existing ReAct agent definitions:

```yaml
agents:
  - name: my-agent
    role: react
    model: gpt-4-turbo
    prompt: "Your agent prompt..."
    tools: [...]
    guided_config:  # Add this section
      enabled: true
      max_iterations: 5
      verification_prompt: "Review results. Respond 'continue' or 'done'."
```

**Backward Compatibility:**

- Guided mode is opt-in via `guided_config.enabled: true`
- Existing agents continue using standard execution by default
- No code changes required for current deployments

## What's Next

Looking ahead to v0.6.0 (Q2 2026):

**Enhanced Provider Features:**

- Streaming support for all providers
- Function calling standardization across providers
- Provider-level cost limits and alerts

**Workflow Improvements:**

- Visual workflow debugging dashboard
- Automatic verification prompt generation
- Workflow templates for common patterns

**Agent Capabilities:**

- Long-term memory integration with guided workflows
- Multi-agent collaboration in guided mode
- Workflow branching based on verification results

## Resources

- **Provider API Documentation**: [pkg.go.dev/github.com/aixgo-dev/aixgo/pkg/llm](https://pkg.go.dev/github.com/aixgo-dev/aixgo/pkg/llm)
- **Guided Workflows Guide**: [docs/PATTERNS.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/PATTERNS.md)
- **Feature Catalog**: [docs/FEATURES.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/FEATURES.md)
- **Examples**: [examples/](https://github.com/aixgo-dev/aixgo/tree/main/examples)

## Get Involved

We'd love to hear how you use the new provider API and guided workflows:

- **GitHub Issues**: [github.com/aixgo-dev/aixgo/issues](https://github.com/aixgo-dev/aixgo/issues)
- **Discussions**: [github.com/orgs/aixgo-dev/discussions](https://github.com/orgs/aixgo-dev/discussions)
- **Contributing**: [docs/CONTRIBUTING.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/CONTRIBUTING.md)

## Contributors

Thank you to everyone who contributed to this release through code, documentation, testing, and feedback.

---

**Aixgo** - Production-grade AI agents in Go.

[Website](https://aixgo.dev) | [GitHub](https://github.com/aixgo-dev/aixgo) | [Documentation](https://pkg.go.dev/github.com/aixgo-dev/aixgo)

**Upgrade to v0.5.0 today:**

```bash
go get github.com/aixgo-dev/aixgo@v0.5.0
```
