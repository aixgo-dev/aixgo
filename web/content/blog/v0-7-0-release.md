---
title: "Aixgo v0.7.0: Amazon Bedrock Integration"
date: 2026-03-28
description: "Access Claude, Llama, Nova, and more via AWS with enterprise-grade security"
tags: ["release", "bedrock", "aws", "enterprise", "multi-model"]
author: "Aixgo Team"
---

We're excited to announce **Aixgo v0.7.0**, bringing Amazon Bedrock support to the framework.
This release enables Go developers to access foundation models from Anthropic, Meta, Amazon,
and Mistral through AWS's managed AI service with enterprise-grade security.

## What's New in v0.7.0

**Quick Links:**

1. [Amazon Bedrock Provider](#1-amazon-bedrock-provider) - Access 20+ foundation models
1. [Multi-Model Access](#2-multi-model-access) - Claude, Llama, Nova, Titan, and more
1. [Enterprise Security](#3-enterprise-security) - AWS IAM, VPC endpoints, CloudTrail
1. [Getting Started](#4-getting-started) - Quick setup guide

### 1. Amazon Bedrock Provider

The centerpiece of v0.7.0 is the new Bedrock provider, offering access to foundation models
from multiple providers through a single, unified AWS API.

**Key Benefits:**

- **Single API** - Access Claude, Llama, Nova, Mistral, and Titan without managing multiple provider accounts
- **Converse API** - Unified messaging format that works across all models
- **AWS Security** - IAM authentication, VPC endpoints, CloudTrail logging
- **Cost Control** - Consolidated billing through AWS

**Simple Configuration:**

```yaml
agents:
  - name: analyst
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    prompt: "You are a data analyst..."
```

**Go SDK Usage:**

```go
import "github.com/aixgo-dev/aixgo/pkg/llm/provider"

p, err := provider.CreateProvider("bedrock", map[string]any{
    "region": "us-east-1",
})

resp, err := p.CreateCompletion(ctx, provider.CompletionRequest{
    Model: "anthropic.claude-3-5-sonnet-20240620-v1:0",
    Messages: []provider.Message{
        {Role: "user", Content: "Analyze this data..."},
    },
})
```

### 2. Multi-Model Access

Amazon Bedrock provides access to foundation models from multiple AI providers:

| Provider  | Models                                           |
| --------- | ------------------------------------------------ |
| Anthropic | Claude 3.5 Sonnet, Claude 3 Haiku, Claude 3 Opus, Claude Opus 4 |
| Amazon    | Nova Pro, Nova Lite, Nova Micro                  |
| Meta      | Llama 3 70B, Llama 3 8B, Llama 4 series         |
| Mistral   | Mistral Large                                    |
| Amazon    | Titan Text Express, Titan Text Lite              |
| Cohere    | Command R, Command R+                            |
| AI21      | Jamba 1.5 Large, Jamba 1.5 Mini                  |

**Automatic Model Detection:**

Aixgo automatically routes to Bedrock based on model ID patterns:

```go
// All of these use the Bedrock provider automatically
provider.DetectProvider("anthropic.claude-3-5-sonnet-20240620-v1:0") // -> "bedrock"
provider.DetectProvider("amazon.nova-pro-v1:0")                       // -> "bedrock"
provider.DetectProvider("meta.llama3-70b-instruct-v1:0")             // -> "bedrock"
provider.DetectProvider("bedrock/anthropic.claude-3-haiku")          // -> "bedrock"
```

### 3. Enterprise Security

Amazon Bedrock integrates with AWS's enterprise security features:

**AWS IAM Authentication:**

```bash
# Option 1: Environment variables
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...

# Option 2: IAM roles (EC2/ECS/EKS)
# No configuration needed - automatically uses instance role

# Option 3: Named profiles
export AWS_PROFILE=production
```

**VPC Endpoints:**

Keep traffic within your VPC for enhanced security:

```text
Your VPC
├── Subnet (private)
│   ├── Aixgo Application
│   └── VPC Endpoint (bedrock-runtime)
└── No internet gateway needed for Bedrock calls
```

**CloudTrail Logging:**

Every API call is logged for audit and compliance:

- Model invocations
- Access patterns
- Cost attribution

### 4. Getting Started

**Prerequisites:**

1. AWS account with Bedrock access
1. Model access granted in the Bedrock console
1. AWS credentials configured

**Installation:**

```bash
go get github.com/aixgo-dev/aixgo@v0.7.0
```

**Quick Start:**

```yaml
# agents.yaml
supervisor:
  name: bedrock-demo
  model: anthropic.claude-3-5-sonnet-20240620-v1:0
  provider: bedrock

agents:
  - name: analyst
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    prompt: "You are a helpful assistant."
```

```bash
# Set AWS credentials
export AWS_REGION=us-east-1

# Run
aixgo run -config agents.yaml
```

## Cost Comparison

Amazon Bedrock pricing is competitive with direct provider access:

| Model           | Bedrock (per 1M) | Direct API (per 1M) |
| --------------- | ---------------- | ------------------- |
| Claude 3.5 Sonnet | $3 / $15       | $3 / $15            |
| Claude 3 Haiku  | $0.25 / $1.25    | $0.25 / $1.25       |
| Llama 3 70B     | $2.65 / $3.50    | N/A (open source)   |
| Nova Pro        | $0.80 / $3.20    | N/A (Bedrock only)  |

Plus benefits of consolidated AWS billing, volume discounts, and enterprise contracts.

## Migration Guide

### From Anthropic Direct API

```yaml
# Before (v0.6.0)
agents:
  - name: analyst
    model: claude-3-5-sonnet-20240620
    provider: anthropic
    api_key: ${ANTHROPIC_API_KEY}

# After (v0.7.0 with Bedrock)
agents:
  - name: analyst
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
```

### Environment Variables

```bash
# Remove (optional - can keep for fallback)
# export ANTHROPIC_API_KEY=...

# Add
export AWS_REGION=us-east-1
# Credentials via IAM role, profile, or access keys
```

## What's Next

Our roadmap for v0.8.0 includes:

- **Bedrock Guardrails** - Content filtering and safety controls
- **Bedrock Agents** - Integration with AWS Bedrock Agents
- **Knowledge Bases** - RAG with Bedrock Knowledge Bases
- **Custom Models** - Support for fine-tuned Bedrock models

## Resources

- **AWS Bedrock Guide**: [aixgo.dev/guides/aws-bedrock](/guides/aws-bedrock/)
- **Provider Integration**: [aixgo.dev/guides/provider-integration](/guides/provider-integration/)
- **Feature Catalog**: [docs/FEATURES.md](https://github.com/aixgo-dev/aixgo/blob/main/docs/FEATURES.md)

---

**Upgrade to v0.7.0:**

```bash
go get github.com/aixgo-dev/aixgo@v0.7.0
```

We're excited to see what you build with Amazon Bedrock and Aixgo. Share your projects and feedback in
[GitHub Discussions](https://github.com/orgs/aixgo-dev/discussions)!
