# Quickstart Guide

Get started with aixgo in under 5 minutes. This guide will walk you through creating your first multi-agent system.

**For comprehensive documentation, visit [https://aixgo.dev](https://aixgo.dev)**

## Prerequisites

- Go 1.21 or later
- Basic understanding of Go programming
- Text editor or IDE

## Installation

### Using go get

```bash
go get github.com/aixgo-dev/aixgo
```

### From Source

```bash
git clone https://github.com/aixgo-dev/aixgo.git
cd aixgo
go build ./...
```

## Your First Multi-Agent System

Let's build a simple data processing pipeline with three agents:

1. **Producer** - Generates data every second
2. **Analyzer** - Processes data using an LLM
3. **Logger** - Logs the results

### Step 1: Create Project Structure

```bash
mkdir my-aixgo-project
cd my-aixgo-project
go mod init my-aixgo-project
```

### Step 2: Install Aixgo

```bash
go get github.com/aixgo-dev/aixgo
```

### Step 3: Create Configuration File

Create `config/agents.yaml`:

```yaml
supervisor:
  name: data-pipeline-supervisor
  model: grok-beta
  max_rounds: 10

agents:
  # Producer agent generates data every second
  - name: data-producer
    role: producer
    interval: 1s
    outputs:
      - target: data-analyzer

  # Analyzer agent processes data using LLM
  - name: data-analyzer
    role: react
    model: grok-beta
    prompt: |
      You are a data analyst. Analyze the incoming data and provide
      a brief summary with key insights.
    inputs:
      - source: data-producer
    outputs:
      - target: result-logger

  # Logger agent logs the analysis results
  - name: result-logger
    role: logger
    inputs:
      - source: data-analyzer
```

### Step 4: Create Main Application

Create `main.go`:

```go
package main

import (
    "log"

    "github.com/aixgo-dev/aixgo"
    _ "github.com/aixgo-dev/aixgo/agents"
)

func main() {
    // Run the agent system with the configuration file
    if err := aixgo.Run("config/agents.yaml"); err != nil {
        log.Fatalf("Failed to run aixgo: %v", err)
    }
}
```

### Step 5: Set Up Environment

You'll need an API key for the LLM provider. For this example, we're using xAI's Grok:

```bash
export XAI_API_KEY="your-api-key-here"
```

For other providers:

```bash
# OpenAI
export OPENAI_API_KEY="your-api-key-here"

# Anthropic
export ANTHROPIC_API_KEY="your-api-key-here"
```

### Step 6: Run Your Agent System

```bash
go run main.go
```

You should see output like:

```text
2025/11/16 10:00:00 Starting agent: data-producer
2025/11/16 10:00:00 Starting agent: data-analyzer
2025/11/16 10:00:00 Starting agent: result-logger
2025/11/16 10:00:01 [data-producer] Generated message
2025/11/16 10:00:01 [data-analyzer] Processing message
2025/11/16 10:00:02 [result-logger] Analysis complete: ...
```

Congratulations! You've created your first aixgo multi-agent system.

## Understanding the Configuration

Let's break down the configuration file:

### Supervisor Configuration

```yaml
supervisor:
  name: data-pipeline-supervisor # Unique identifier
  model: grok-beta # LLM model for orchestration
  max_rounds: 10 # Maximum execution rounds
```

The supervisor orchestrates agent execution and enforces execution limits.

### Agent Roles

#### Producer Agent

```yaml
- name: data-producer
  role: producer
  interval: 1s # Generate message every second
  outputs:
    - target: data-analyzer # Send to analyzer
```

Generates periodic messages for downstream processing.

#### ReAct Agent

```yaml
- name: data-analyzer
  role: react
  model: grok-beta # LLM to use
  prompt: | # System prompt
    You are a data analyst...
  inputs:
    - source: data-producer # Receive from producer
  outputs:
    - target: result-logger # Send to logger
```

Uses LLM reasoning and tool calling to process messages.

#### Logger Agent

```yaml
- name: result-logger
  role: logger
  inputs:
    - source: data-analyzer # Receive from analyzer
```

Consumes and logs messages from other agents.

## Next Steps

### Add Tool Calling

Enhance your ReAct agent with tools:

```yaml
- name: data-analyzer
  role: react
  model: grok-beta
  prompt: 'You are a data analyst with access to a database.'
  tools:
    - name: query_database
      description: 'Query the database for historical data'
      input_schema:
        type: object
        properties:
          query:
            type: string
            description: 'SQL query to execute'
        required: [query]
  inputs:
    - source: data-producer
  outputs:
    - target: result-logger
```

When the agent needs data, it will automatically call your tool. You'll need to implement the tool handler in your Go code.

### Multiple Input/Output Streams

Agents can have multiple inputs and outputs:

```yaml
- name: aggregator
  role: react
  model: grok-beta
  prompt: 'Aggregate data from multiple sources.'
  inputs:
    - source: producer-1
    - source: producer-2
    - source: producer-3
  outputs:
    - target: logger-1
    - target: logger-2
```

### Custom Intervals

Adjust producer timing based on your needs:

```yaml
- name: fast-producer
  role: producer
  interval: 100ms # Every 100 milliseconds

- name: slow-producer
  role: producer
  interval: 5m # Every 5 minutes
```

Supported units: `ms` (milliseconds), `s` (seconds), `m` (minutes), `h` (hours).

## Common Patterns

### Sequential Processing Pipeline

```yaml
agents:
  - name: ingester
    role: producer
    interval: 1s
    outputs:
      - target: validator

  - name: validator
    role: react
    prompt: 'Validate incoming data'
    inputs:
      - source: ingester
    outputs:
      - target: enricher

  - name: enricher
    role: react
    prompt: 'Enrich validated data'
    inputs:
      - source: validator
    outputs:
      - target: storage

  - name: storage
    role: logger
    inputs:
      - source: enricher
```

### Parallel Processing

```yaml
agents:
  - name: data-source
    role: producer
    interval: 1s
    outputs:
      - target: analyzer-1
      - target: analyzer-2
      - target: analyzer-3

  - name: analyzer-1
    role: react
    prompt: 'Sentiment analysis'
    inputs:
      - source: data-source
    outputs:
      - target: aggregator

  - name: analyzer-2
    role: react
    prompt: 'Entity extraction'
    inputs:
      - source: data-source
    outputs:
      - target: aggregator

  - name: analyzer-3
    role: react
    prompt: 'Topic classification'
    inputs:
      - source: data-source
    outputs:
      - target: aggregator

  - name: aggregator
    role: react
    prompt: 'Combine all analysis results'
    inputs:
      - source: analyzer-1
      - source: analyzer-2
      - source: analyzer-3
    outputs:
      - target: logger

  - name: logger
    role: logger
    inputs:
      - source: aggregator
```

### Fan-Out/Fan-In

```yaml
agents:
  # Single source
  - name: event-stream
    role: producer
    interval: 500ms
    outputs:
      - target: processor-1
      - target: processor-2

  # Parallel processors
  - name: processor-1
    role: react
    prompt: 'Process type A events'
    inputs:
      - source: event-stream
    outputs:
      - target: combiner

  - name: processor-2
    role: react
    prompt: 'Process type B events'
    inputs:
      - source: event-stream
    outputs:
      - target: combiner

  # Combine results
  - name: combiner
    role: react
    prompt: 'Combine processed events'
    inputs:
      - source: processor-1
      - source: processor-2
    outputs:
      - target: final-logger

  - name: final-logger
    role: logger
    inputs:
      - source: combiner
```

## Troubleshooting

### Agent Not Starting

Check that:

- Agent names are unique
- All referenced inputs/outputs exist
- Configuration YAML is valid
- Required environment variables are set

### No Output from Logger

Verify:

- Logger has valid input sources
- Upstream agents are outputting to the logger
- Messages are flowing through the pipeline

### LLM API Errors

Common issues:

- Missing or invalid API key
- Rate limiting (add delays or reduce frequency)
- Model name typos (check provider documentation)
- Network connectivity issues

### Performance Issues

Optimize by:

- Adjusting producer intervals
- Reducing max_rounds if appropriate
- Using smaller/faster LLM models
- Implementing caching in custom tools

## Example Projects

Check the `examples/` directory for complete working examples:

- **Simple Pipeline**: Basic producer → processor → logger
- **Multi-Agent Analysis**: Parallel analysis with aggregation
- **Tool Calling**: Agent with database access
- **Event Processing**: Real-time event stream processing

## Further Reading

**Main Documentation:**

- [Aixgo Documentation](https://aixgo.dev) - Comprehensive guides and tutorials

**Repository Documentation:**

- [README.md](../README.md) - Project overview
- [TESTING_GUIDE.md](TESTING_GUIDE.md) - Testing your agents
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contributing to aixgo
- [API Reference](https://pkg.go.dev/github.com/aixgo-dev/aixgo) - GoDoc documentation

## Getting Help

- **Documentation**: [https://aixgo.dev](https://aixgo.dev)
- **GitHub Discussions**: [Ask questions](https://github.com/aixgo-dev/aixgo/discussions)
- **GitHub Issues**: [Report bugs](https://github.com/aixgo-dev/aixgo/issues)
- **API Reference**: [GoDoc](https://pkg.go.dev/github.com/aixgo-dev/aixgo)

## What's Next?

Now that you have a working agent system, you can:

1. **Add Custom Tools**: Implement tool handlers for database access, API calls, etc.
2. **Implement Custom Agents**: Create new agent types beyond producer/react/logger
3. **Add Observability**: Integrate OpenTelemetry for distributed tracing
4. **Scale to Production**: Deploy your agents with proper monitoring and error handling
5. **Explore Distributed Mode**: Scale beyond a single instance (coming in v0.2)

Happy building with aixgo!
