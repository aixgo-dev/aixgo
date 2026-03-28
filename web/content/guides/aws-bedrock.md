---
title: 'AWS Bedrock Integration Guide'
description: 'Integrate Aixgo with Amazon Bedrock for enterprise-grade AI models with single API access, regional deployment, and AWS security.'
breadcrumb: 'Reference'
category: 'Reference'
weight: 12
---

## Introduction

Amazon Bedrock is AWS's fully managed service providing API access to foundation models from leading AI companies through a unified interface. Bedrock enables you to build and scale generative AI applications with enterprise security, compliance, and operational simplicity.

### What is Amazon Bedrock?

Bedrock offers access to foundation models from Anthropic (Claude), Amazon (Nova, Titan), Meta (Llama), Mistral AI, and more through a single API. Unlike direct provider integrations, Bedrock provides:

- **Unified API** - Single integration for multiple model providers
- **AWS Security** - IAM-based access control, VPC endpoints, encryption at rest/in transit
- **Compliance** - HIPAA, GDPR, SOC 2, ISO 27001 certified
- **Regional Deployment** - Deploy in AWS regions globally with data residency compliance
- **Cost Management** - Consolidated billing, cost allocation tags, AWS Cost Explorer integration
- **Guardrails** - Built-in content filtering, PII detection, topic blocking

### Benefits for Go Applications

Integrating Bedrock with Aixgo unlocks powerful advantages:

**No Python Dependencies** - Pure Go implementation eliminates the 1GB+ Python containers and 10-45 second cold starts typical of Python frameworks. Aixgo with Bedrock delivers <100ms cold starts in single binaries under 20MB.

**Enterprise Security** - Leverage AWS IAM roles, VPC endpoints, and AWS PrivateLink to keep model requests within your VPC without internet traversal. Bedrock never uses your data for model training.

**Multi-Region Resilience** - Deploy agents across multiple AWS regions with automatic failover. Bedrock is available in 10+ regions globally.

**Cost Optimization** - Use AWS Cost Explorer to track model usage by project, environment, or agent. Set up billing alarms to prevent runaway costs.

### When to Use Bedrock vs Direct Provider APIs

**Choose Bedrock when you:**

- Already operate AWS infrastructure
- Need compliance certifications (HIPAA, GDPR, SOC 2)
- Require data residency in specific AWS regions
- Want consolidated billing across multiple model providers
- Need VPC isolation for sensitive workloads

**Choose Direct Provider APIs when you:**

- Operate multi-cloud or cloud-agnostic infrastructure
- Need the absolute latest model versions (Bedrock has ~1-4 week lag)
- Require provider-specific features not yet in Bedrock
- Prefer direct provider support relationships

## Prerequisites

### AWS Account Setup

1. **Create AWS Account** (if not already created):
   - Visit https://aws.amazon.com
   - Complete account registration
   - Set up billing payment method

1. **Enable Bedrock in your region**:
   - Navigate to AWS Bedrock console: https://console.aws.amazon.com/bedrock
   - Select your preferred region (e.g., us-east-1, us-west-2, eu-west-1)
   - Bedrock automatically provisions in enabled regions

### IAM Permissions

Create an IAM policy with the minimum required permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "BedrockModelAccess",
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream",
        "bedrock:ListFoundationModels",
        "bedrock:GetFoundationModel"
      ],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/*"
      ]
    }
  ]
}
```

Attach this policy to:

- An IAM user (for development)
- An IAM role (for production EC2/ECS/EKS deployments)

### Model Access Requests

Before using models, request access in the Bedrock console:

1. Navigate to **Model access** in the Bedrock console
1. Select models you want to use:
   - Anthropic Claude models (claude-3-5-sonnet, claude-3-haiku, etc.)
   - Amazon Nova models (nova-pro, nova-lite, nova-micro)
   - Meta Llama models (llama3-70b, llama3-8b)
   - Mistral AI models (mistral-large, mistral-7b)
   - Amazon Titan models (titan-text-express, titan-text-lite)
1. Click **Request model access**
1. Wait for approval (usually instant for most models)

### Go and Aixgo Installation

**Install Go 1.26+**:

```bash
# Verify Go version
go version  # Should show 1.26 or higher

# If Go is not installed, download from https://go.dev/dl/
```

**Install Aixgo**:

```bash
go get github.com/aixgo-dev/aixgo
```

## Quick Start

### Environment Setup

Configure AWS credentials using one of these methods:

**Method 1: Environment Variables**

```bash
export AWS_ACCESS_KEY_ID=<your-access-key-id>
export AWS_SECRET_ACCESS_KEY=<your-secret-access-key>
export AWS_REGION=us-east-1
```

**Method 2: AWS CLI Configuration**

```bash
aws configure
# Enter access key, secret key, and region when prompted
```

**Method 3: IAM Roles (Recommended for Production)**

When running on EC2, ECS, or EKS, attach an IAM role with Bedrock permissions. No credentials needed in code.

### First API Call Example

Create a simple agent using Claude 3.5 Sonnet via Bedrock:

**config/bedrock-agent.yaml**:

```yaml
supervisor:
  name: bedrock-coordinator
  model: anthropic.claude-3-5-sonnet-20240620-v1:0
  provider: bedrock
  region: us-east-1

agents:
  - name: bedrock-analyst
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1
    prompt: |
      You are a data analyst using AWS Bedrock.
      Analyze incoming data and provide insights.
    temperature: 0.7
    max_tokens: 1000
```

**main.go**:

```go
package main

import (
    "github.com/aixgo-dev/aixgo"
    _ "github.com/aixgo-dev/aixgo/agents"
)

func main() {
    if err := aixgo.Run("config/bedrock-agent.yaml"); err != nil {
        panic(err)
    }
}
```

**Run it**:

```bash
export AWS_REGION=us-east-1
go run main.go
```

### Model Selection Guidance

Choose models based on your use case:

**For complex reasoning and analysis**: `anthropic.claude-3-5-sonnet-20240620-v1:0`

**For fast, cost-effective tasks**: `anthropic.claude-3-haiku-20240307-v1:0` or `amazon.nova-micro-v1:0`

**For multimodal (text + images)**: `anthropic.claude-3-5-sonnet-20240620-v1:0` or `amazon.nova-pro-v1:0`

**For long-context processing**: `anthropic.claude-3-5-sonnet-20240620-v1:0` (200K tokens)

**For code generation**: `meta.llama3-70b-instruct-v1:0` or `anthropic.claude-3-5-sonnet-20240620-v1:0`

## Configuration Options

### YAML Configuration

**Basic agent configuration**:

```yaml
agents:
  - name: bedrock-researcher
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1
    temperature: 0.5
    max_tokens: 2000
    top_p: 0.9
```

**Multi-model configuration with fallback**:

```yaml
agents:
  - name: resilient-agent
    role: react
    providers:
      # Primary: Claude 3.5 Sonnet (most capable)
      - model: anthropic.claude-3-5-sonnet-20240620-v1:0
        provider: bedrock
        region: us-east-1

      # Fallback 1: Amazon Nova Pro (fast, cost-effective)
      - model: amazon.nova-pro-v1:0
        provider: bedrock
        region: us-east-1

      # Fallback 2: Claude 3 Haiku (cheapest)
      - model: anthropic.claude-3-haiku-20240307-v1:0
        provider: bedrock
        region: us-west-2

    fallback_strategy: cascade
```

### Go SDK Programmatic Usage

**Direct agent creation**:

```go
import (
    "github.com/aixgo-dev/aixgo"
    "github.com/aixgo-dev/aixgo/providers/bedrock"
)

func main() {
    agent := aixgo.NewAgent(
        aixgo.WithName("bedrock-agent"),
        aixgo.WithModel("anthropic.claude-3-5-sonnet-20240620-v1:0"),
        aixgo.WithProvider(bedrock.Provider{
            Region: "us-east-1",
        }),
        aixgo.WithTemperature(0.7),
        aixgo.WithMaxTokens(1000),
    )

    // Use agent...
}
```

**With custom AWS credentials**:

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
)

cfg, err := config.LoadDefaultConfig(ctx,
    config.WithRegion("us-east-1"),
    config.WithCredentialsProvider(
        credentials.NewStaticCredentialsProvider(
            "access-key-id",
            "secret-access-key",
            "",
        ),
    ),
)

agent := aixgo.NewAgent(
    aixgo.WithProvider(bedrock.Provider{
        AWSConfig: cfg,
    }),
)
```

### Multi-Region Deployment

Deploy agents across multiple regions for resilience:

```yaml
agents:
  # Primary region: us-east-1
  - name: us-east-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1

  # Secondary region: us-west-2
  - name: us-west-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-west-2

  # European region: eu-west-1
  - name: eu-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: eu-west-1
```

## Authentication

### Environment Variables

**Standard AWS credentials**:

```bash
export AWS_ACCESS_KEY_ID=<your-access-key-id>
export AWS_SECRET_ACCESS_KEY=<your-secret-access-key>
export AWS_REGION=us-east-1
```

**With session token (for temporary credentials)**:

```bash
export AWS_ACCESS_KEY_ID=<your-access-key-id>
export AWS_SECRET_ACCESS_KEY=<your-secret-access-key>
export AWS_SESSION_TOKEN=<your-session-token>
export AWS_REGION=us-east-1
```

### IAM Roles for EC2/ECS/EKS

**Recommended for production deployments**. Attach an IAM role to your compute instance:

**EC2 Instance**:

1. Create IAM role with Bedrock permissions policy
1. Attach role to EC2 instance
1. No credentials needed in code - SDK automatically uses instance metadata

**ECS Task**:

```json
{
  "taskRoleArn": "arn:aws:iam::123456789012:role/BedrockTaskRole",
  "containerDefinitions": [
    {
      "name": "aixgo-agent",
      "image": "myapp:latest",
      "environment": [
        {"name": "AWS_REGION", "value": "us-east-1"}
      ]
    }
  ]
}
```

**EKS Pod (IRSA - IAM Roles for Service Accounts)**:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: aixgo-agent
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/BedrockPodRole
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aixgo-agent
spec:
  template:
    spec:
      serviceAccountName: aixgo-agent
      containers:
      - name: agent
        image: myapp:latest
        env:
        - name: AWS_REGION
          value: us-east-1
```

### AWS Profiles

Use named profiles for development:

```bash
# ~/.aws/credentials
[default]
aws_access_key_id = <default-access-key>
aws_secret_access_key = <default-secret-key>

[production]
aws_access_key_id = <prod-access-key>
aws_secret_access_key = <prod-secret-key>
```

```bash
# Use specific profile
export AWS_PROFILE=production
go run main.go
```

### Cross-Account Access

Access Bedrock in another AWS account using role assumption:

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials/stscreds"
    "github.com/aws/aws-sdk-go-v2/service/sts"
)

cfg, err := config.LoadDefaultConfig(ctx)
stsClient := sts.NewFromConfig(cfg)

creds := stscreds.NewAssumeRoleProvider(stsClient,
    "arn:aws:iam::123456789012:role/CrossAccountBedrockRole")

cfg, err = config.LoadDefaultConfig(ctx,
    config.WithCredentialsProvider(creds),
)
```

## Available Models

### Model ID Reference Table

| Provider | Model Name | Model ID | Context Length | Features |
|----------|-----------|----------|----------------|----------|
| **Anthropic** | Claude 3.5 Sonnet | `anthropic.claude-3-5-sonnet-20240620-v1:0` | 200K tokens | Tool use, vision, long context |
| **Anthropic** | Claude 3 Opus | `anthropic.claude-3-opus-20240229-v1:0` | 200K tokens | Most capable, highest quality |
| **Anthropic** | Claude 3 Sonnet | `anthropic.claude-3-sonnet-20240229-v1:0` | 200K tokens | Balanced performance/cost |
| **Anthropic** | Claude 3 Haiku | `anthropic.claude-3-haiku-20240307-v1:0` | 200K tokens | Fastest, lowest cost |
| **Amazon** | Nova Pro | `amazon.nova-pro-v1:0` | 300K tokens | Multimodal, video understanding |
| **Amazon** | Nova Lite | `amazon.nova-lite-v1:0` | 300K tokens | Fast, cost-effective |
| **Amazon** | Nova Micro | `amazon.nova-micro-v1:0` | 128K tokens | Ultra-low latency |
| **Meta** | Llama 3.1 405B | `meta.llama3-1-405b-instruct-v1:0` | 128K tokens | Largest, most capable |
| **Meta** | Llama 3.1 70B | `meta.llama3-1-70b-instruct-v1:0` | 128K tokens | High quality, open source |
| **Meta** | Llama 3.1 8B | `meta.llama3-1-8b-instruct-v1:0` | 128K tokens | Fast, cost-effective |
| **Meta** | Llama 3 70B | `meta.llama3-70b-instruct-v1:0` | 8K tokens | Strong performance |
| **Meta** | Llama 3 8B | `meta.llama3-8b-instruct-v1:0` | 8K tokens | Lightweight |
| **Mistral AI** | Mistral Large 2 | `mistral.mistral-large-2407-v1:0` | 128K tokens | Complex reasoning |
| **Mistral AI** | Mistral Small | `mistral.mistral-small-2402-v1:0` | 32K tokens | Cost-efficient |
| **Mistral AI** | Mixtral 8x7B | `mistral.mixtral-8x7b-instruct-v0:1` | 32K tokens | Mixture of experts |
| **Amazon** | Titan Text Express | `amazon.titan-text-express-v1` | 8K tokens | AWS-native, summarization |
| **Amazon** | Titan Text Lite | `amazon.titan-text-lite-v1` | 4K tokens | Ultra-low cost |

### Regional Availability

Model availability varies by region. Check current availability:

```bash
aws bedrock list-foundation-models --region us-east-1
```

**Generally available in**: us-east-1, us-west-2, eu-west-1, eu-central-1, ap-southeast-1, ap-northeast-1

## Advanced Features

### Tool Calling

Enable agents to call functions using Bedrock's tool use API:

```yaml
agents:
  - name: tool-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1
    prompt: |
      You are an assistant with access to tools.
      Use the get_weather tool to answer weather questions.
    tools:
      - name: get_weather
        description: Get current weather for a location
        parameters:
          type: object
          properties:
            location:
              type: string
              description: City name
            unit:
              type: string
              enum: [celsius, fahrenheit]
          required: [location]
```

**Go implementation**:

```go
type WeatherTool struct{}

func (t *WeatherTool) Execute(ctx context.Context, args map[string]any) (any, error) {
    location := args["location"].(string)
    unit := args["unit"].(string)

    // Call weather API
    weather, err := getWeather(location)
    if err != nil {
        return nil, err
    }

    return map[string]any{
        "temperature": convertTemp(weather.Temp, unit),
        "conditions":  weather.Conditions,
        "location":    location,
    }, nil
}
```

### Structured Output

Use JSON schema to enforce response structure:

```yaml
agents:
  - name: structured-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1
    prompt: |
      Extract customer information from the text.
      Return as JSON with name, email, phone fields.
    output_schema:
      type: object
      properties:
        name:
          type: string
        email:
          type: string
          format: email
        phone:
          type: string
      required: [name, email]
```

Aixgo validates responses against schema with automatic retry on validation failures (40-70% improved reliability).

### Streaming Responses

Stream model outputs for real-time user experiences:

```go
agent := aixgo.NewAgent(
    aixgo.WithModel("anthropic.claude-3-5-sonnet-20240620-v1:0"),
    aixgo.WithProvider(bedrock.Provider{Region: "us-east-1"}),
    aixgo.WithStreaming(true),
)

stream, err := agent.StreamExecute(ctx, input)
for chunk := range stream {
    fmt.Print(chunk.Content)
}
```

### Guardrails Integration

Apply AWS Bedrock Guardrails for content filtering:

```yaml
agents:
  - name: safe-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1
    bedrock_config:
      guardrail_id: <your-guardrail-id>
      guardrail_version: "1"
```

Guardrails provide:

- **Content filtering** - Block harmful, toxic, or inappropriate content
- **PII redaction** - Automatically detect and mask sensitive information
- **Topic blocking** - Prevent discussion of specific topics
- **Word filtering** - Block profanity and custom word lists

## Production Deployment

### VPC Endpoints

Use VPC endpoints to keep Bedrock traffic within your VPC:

1. **Create VPC endpoint**:

```bash
aws ec2 create-vpc-endpoint \
  --vpc-id vpc-12345678 \
  --service-name com.amazonaws.us-east-1.bedrock-runtime \
  --route-table-ids rtb-12345678 \
  --subnet-ids subnet-12345678
```

1. **Configure security group**:

Allow HTTPS (port 443) from your application subnets.

1. **Update endpoint policy** (optional):

```json
{
  "Statement": [
    {
      "Principal": "*",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Effect": "Allow",
      "Resource": "arn:aws:bedrock:*::foundation-model/*"
    }
  ]
}
```

**Benefit**: Traffic never leaves AWS network, improving security and reducing latency.

### CloudTrail Logging

Enable CloudTrail to audit all Bedrock API calls:

```bash
aws cloudtrail create-trail \
  --name bedrock-audit-trail \
  --s3-bucket-name my-cloudtrail-bucket

aws cloudtrail start-logging --name bedrock-audit-trail
```

CloudTrail logs capture:

- Model invocations (InvokeModel, InvokeModelWithResponseStream)
- Request metadata (timestamp, IAM principal, source IP)
- Model IDs and regions used
- Error responses

Use for compliance auditing, security analysis, and debugging.

### Cost Management

**Set up billing alarms**:

```bash
aws cloudwatch put-metric-alarm \
  --alarm-name bedrock-cost-alert \
  --alarm-description "Alert when Bedrock costs exceed $100/day" \
  --metric-name EstimatedCharges \
  --namespace AWS/Billing \
  --statistic Maximum \
  --period 86400 \
  --threshold 100 \
  --comparison-operator GreaterThanThreshold
```

**Track costs by tag**:

Tag Bedrock invocations using IAM role tags or application tags:

```go
// Tag IAM role with project/environment
// Costs automatically allocated in Cost Explorer
```

**Use AWS Cost Explorer** to analyze:

- Cost by model (Claude vs Nova vs Llama)
- Cost by region
- Cost by project or environment
- Trend analysis and forecasting

### Multi-Region Failover

Implement automatic failover across regions:

```yaml
agents:
  - name: resilient-agent
    role: react
    providers:
      # Primary: us-east-1
      - model: anthropic.claude-3-5-sonnet-20240620-v1:0
        provider: bedrock
        region: us-east-1
        timeout: 30s

      # Failover 1: us-west-2
      - model: anthropic.claude-3-5-sonnet-20240620-v1:0
        provider: bedrock
        region: us-west-2
        timeout: 30s

      # Failover 2: eu-west-1
      - model: anthropic.claude-3-5-sonnet-20240620-v1:0
        provider: bedrock
        region: eu-west-1
        timeout: 30s

    fallback_strategy: cascade
    retry:
      max_attempts: 2
      initial_backoff: 1s
```

Aixgo automatically tries each region in order if the primary fails.

## Cost Optimization

### Model Selection by Use Case

**Choose the right model for each task to optimize costs**:

| Use Case | Recommended Model | Why |
|----------|------------------|-----|
| Simple classification | `amazon.nova-micro-v1:0` | Lowest cost, sub-second latency |
| Chatbots | `anthropic.claude-3-haiku-20240307-v1:0` | Fast, cost-effective, natural conversation |
| Data analysis | `anthropic.claude-3-5-sonnet-20240620-v1:0` | Strong reasoning, tool use |
| Content generation | `meta.llama3-1-70b-instruct-v1:0` | High quality, lower cost than Claude |
| Code generation | `anthropic.claude-3-5-sonnet-20240620-v1:0` | Best for complex code |
| Summarization | `amazon.titan-text-express-v1` | Purpose-built, ultra-low cost |
| Complex reasoning | `anthropic.claude-3-opus-20240229-v1:0` | Most capable (use sparingly) |

**Cost comparison (approximate per 1M tokens)**:

- Claude 3 Haiku: $0.25 input / $1.25 output
- Amazon Nova Micro: $0.035 input / $0.14 output
- Amazon Titan Express: $0.20 input / $0.60 output
- Llama 3.1 70B: $0.99 input / $0.99 output
- Claude 3.5 Sonnet: $3.00 input / $15.00 output

### Token Usage Monitoring

**Enable token tracking in Aixgo**:

```yaml
observability:
  llm_observability:
    enabled: true
    track_tokens: true
    daily_token_limit: 1000000
    cost_alert_threshold: 50  # Alert if daily cost > $50
```

**Track via CloudWatch metrics**:

Aixgo publishes custom metrics to CloudWatch:

- `LLM/TokensUsed` - Total tokens per request
- `LLM/InputTokens` - Prompt tokens
- `LLM/OutputTokens` - Completion tokens
- `LLM/Cost` - Estimated cost per request

**Set up cost dashboards**:

Create CloudWatch dashboard to monitor:

- Daily token usage trend
- Cost by agent
- Cost by model
- Anomaly detection

## Troubleshooting

### Common Errors

#### AccessDenied

**Error Message**:

```text
An error occurred (AccessDeniedException) when calling the InvokeModel operation:
User: arn:aws:iam::123456789012:user/myuser is not authorized to perform:
bedrock:InvokeModel on resource: arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-3-5-sonnet-20240620-v1:0
```

**Solutions**:

1. **Verify IAM permissions** - Ensure user/role has `bedrock:InvokeModel` permission
1. **Check resource ARN** - Confirm policy allows access to specific model
1. **Verify model access** - Request model access in Bedrock console if not already granted
1. **Check region** - Ensure IAM policy allows access in the target region

```bash
# Test IAM permissions
aws bedrock invoke-model \
  --model-id anthropic.claude-3-5-sonnet-20240620-v1:0 \
  --region us-east-1 \
  --body '{"anthropic_version": "bedrock-2023-05-31", "messages": [{"role": "user", "content": "Hi"}], "max_tokens": 100}' \
  --content-type application/json \
  output.txt
```

#### ThrottlingException

**Error Message**:

```text
An error occurred (ThrottlingException) when calling the InvokeModel operation:
Rate exceeded
```

**Solutions**:

1. **Implement exponential backoff** - Aixgo does this automatically with retry configuration
1. **Request quota increase** - Submit request in AWS Service Quotas console
1. **Distribute load** - Use multiple regions or models
1. **Reduce request rate** - Add rate limiting in your application

```yaml
agents:
  - name: throttle-resilient-agent
    role: react
    model: anthropic.claude-3-5-sonnet-20240620-v1:0
    provider: bedrock
    region: us-east-1
    retry:
      max_attempts: 5
      initial_backoff: 2s
      max_backoff: 30s
      multiplier: 2
```

#### ModelNotFound

**Error Message**:

```text
An error occurred (ResourceNotFoundException) when calling the InvokeModel operation:
Could not resolve the foundation model from the model identifier: anthropic.claude-3-5-sonnet-20240620-v1:0
```

**Solutions**:

1. **Verify model ID** - Check for typos in model identifier
1. **Check regional availability** - Model may not be available in your region
1. **Request model access** - Enable model in Bedrock console Model Access page
1. **Use correct model ID format** - Bedrock model IDs differ from provider APIs

```bash
# List available models in your region
aws bedrock list-foundation-models --region us-east-1 | grep modelId
```

### Debug Logging

Enable debug logging for detailed request/response information:

```bash
export AIXGO_DEBUG=true
export AWS_SDK_LOG_LEVEL=debug
go run main.go
```

Debug logs include:

- Full request payloads sent to Bedrock
- Response bodies and headers
- Token counts and costs
- Retry attempts and backoff timing
- AWS SDK debug output

### AWS Support Resources

**AWS Support Plans**:

- **Developer** - Business hours email support ($29/month)
- **Business** - 24/7 support, <1 hour response for urgent issues ($100/month)
- **Enterprise** - Dedicated TAM, <15 minute response for critical issues ($15,000/month)

**Bedrock Documentation**: https://docs.aws.amazon.com/bedrock/

**AWS re:Post Community**: https://repost.aws/tags/TAL_SxuHzRSGusOWwH3XbQw/amazon-bedrock

**Bedrock Quotas**: https://console.aws.amazon.com/servicequotas/ (search for "Bedrock")

**Open Support Case**: https://console.aws.amazon.com/support/

## Next Steps

Now that you have Bedrock integration configured, explore advanced Aixgo capabilities:

- **[Multi-Agent Orchestration](/guides/multi-agent-orchestration)** - Build complex workflows with multiple Bedrock agents
- **[Observability](/guides/observability)** - Monitor Bedrock costs and performance with OpenTelemetry
- **[Production Deployment](/guides/production-deployment)** - Deploy Bedrock agents on ECS/EKS
- **[Vector Databases & RAG](/guides/vector-databases)** - Combine Bedrock with retrieval-augmented generation
- **[Cost Optimization](/guides/cost-optimization)** - Advanced techniques for reducing Bedrock costs
- **[Provider Integration](/guides/provider-integration)** - Compare Bedrock with direct provider APIs
