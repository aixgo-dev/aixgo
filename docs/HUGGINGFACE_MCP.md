# HuggingFace + MCP Integration

Complete guide for using HuggingFace models with MCP-based tool calling in aixgo.

## Overview

This integration enables:

- **Local Inference**: Run HuggingFace models locally via Ollama
- **MCP Tools**: Define tools using Model Context Protocol
- **ReAct Pattern**: Automatic reasoning + action loops
- **Hybrid Deployment**: Single binary or distributed services
- **Pure Go**: No Python dependencies for orchestration

## Quick Start

### 1. Install Dependencies

```bash
# Install Ollama
brew install ollama  # macOS
# or download from https://ollama.com

# Start Ollama
ollama serve
```

### 2. Pull a Model

```bash
# Phi-3.5 Mini (recommended)
ollama pull phi3.5:3.8b-mini-instruct-q4_K_M

# Or Gemma 2B
ollama pull gemma2:2b-instruct-q4_0
```

### 3. Run the Example

```bash
cd examples/huggingface-mcp
go run main.go
```

## Architecture

### MCP-First Tool Calling

All tools are defined via MCP (no embedded tools):

```go
// Define tools
mcpServer := mcp.NewServer("my-tools")
mcpServer.RegisterTool(mcp.Tool{
    Name:        "get_weather",
    Description: "Get weather for a city",
    Handler: func(ctx context.Context, args mcp.Args) (any, error) {
        city := args.String("city")
        return fetchWeather(city), nil
    },
    Schema: mcp.Schema{
        "city": {Type: "string", Required: true},
    },
})

// Run aixgo with MCP server
aixgo.RunWithMCP("config.yaml", mcpServer)
```

### ReAct Prompting

HuggingFace models use ReAct (Reasoning + Acting) pattern:

```
User: What's the weather in Tokyo?

Thought: I need to check the weather in Tokyo
Action: get_weather
Action Input: {"city": "Tokyo"}
Observation: {"temperature": 72, "condition": "sunny"}
Thought: I have the weather information
Final Answer: The weather in Tokyo is sunny with 72°F
```

### Hybrid Inference

Automatic fallback from local to cloud:

```go
hybrid := inference.NewHybridInference(
    ollama.NewClient("http://localhost:11434"),  // Try local first
    cloud.NewClient("xai"),                      // Fallback to cloud
)
```

## Configuration

### Complete Example

```yaml
supervisor:
  name: coordinator
  model: grok-beta
  max_rounds: 10

# MCP server definitions
mcp_servers:
  - name: my-tools
    transport: local # Embedded in binary

  - name: database-tools
    transport: grpc # Remote service
    address: 'db-tools.example.com:443'
    tls: true

# Model services
model_services:
  - name: phi-local
    provider: huggingface
    model: phi3.5:3.8b-mini-instruct-q4_K_M
    runtime: ollama
    transport: local
    config:
      quantization: int4

  - name: gemma-cloud
    provider: huggingface
    model: google/gemma-2b-it
    runtime: cloud
    transport: grpc
    address: 'inference.example.com:443'

# Agents
agents:
  - name: assistant
    role: react
    model: phi-local
    prompt: 'You are a helpful assistant.'
    mcp_servers: [my-tools, database-tools]
    inputs:
      - source: producer
    outputs:
      - target: logger
```

## Deployment Modes

### Mode 1: Single Binary (Development)

```bash
# Build
go build -o aixgo cmd/aixgo/main.go

# Run
./aixgo config/agents.yaml
```

**Characteristics**:

- ✅ Single 8MB binary
- ✅ Embedded MCP server
- ✅ Uses local Ollama or cloud APIs
- ✅ Perfect for development

### Mode 2: Docker Compose (Local Testing)

```bash
# Start everything
docker-compose up

# Includes:
# - aixgo orchestrator
# - Ollama with models
# - MCP servers (if configured)
```

### Mode 3: Distributed (Production)

Deploy each component separately:

```bash
# Deploy Ollama service
docker run -d -p 11434:11434 ollama/ollama

# Deploy MCP tool servers
docker run -d -p 50051:50051 mcp-database-tools

# Deploy aixgo orchestrator
docker run -d aixgo:latest
```

## Model Selection Guide

| Model            | Size | Context | Quality | Speed  | Recommended For  |
| ---------------- | ---- | ------- | ------- | ------ | ---------------- |
| **Phi-3.5 Mini** | 3.8B | 128K    | ★★★★☆   | Fast   | **Best overall** |
| Gemma 2B         | 2.5B | 8K      | ★★★☆☆   | Faster | Simple tasks     |
| Qwen2.5 3B       | 3B   | 32K     | ★★★★☆   | Fast   | Reasoning        |
| Llama 3.2 3B     | 3B   | 128K    | ★★★★☆   | Medium | General purpose  |

### Why Phi-3.5 Mini?

- ✅ 128K context window (vs 8K for Gemma)
- ✅ Better reasoning capabilities
- ✅ Similar speed and size
- ✅ Handles complex tool calling better

## Tool Calling Reliability

### Expected Success Rates

| Model        | Native Function Calling | ReAct Success Rate | Notes           |
| ------------ | ----------------------- | ------------------ | --------------- |
| GPT-4        | ✅ Yes                  | 99%                | Native support  |
| Phi-3.5 Mini | ❌ No                   | 80-85%             | Good with ReAct |
| Gemma 2B     | ❌ No                   | 70-80%             | More errors     |

### Improving Reliability

**1. Structured Output (Best)**:

```go
// Use constrained generation if supported
response := client.Generate(prompt,
    WithSchema(ToolCallSchema),
    WithGrammar(jsonGrammar),
)
```

**2. Few-Shot Examples**:

```go
prompt := `Examples:
User: Check database
Assistant: {"tool": "query_db", "args": {"query": "SELECT ..."}}

Now you try:
User: ` + userMessage
```

**3. Retry with Correction**:

```go
for attempt := 0; attempt < 3; attempt++ {
    response := llm.Generate(prompt)
    if valid := parseToolCall(response); valid {
        return executeTool(valid)
    }
    prompt += "\nPrevious output was invalid. Try again with correct JSON."
}
```

## Performance

### Latency Targets

| Operation              | Target | Typical |
| ---------------------- | ------ | ------- |
| Local tool call        | <0.1ms | 0.05ms  |
| MCP local transport    | <2ms   | 1ms     |
| MCP gRPC (localhost)   | <5ms   | 3ms     |
| Ollama inference (CPU) | <2s    | 1.5s    |
| Ollama inference (GPU) | <200ms | 150ms   |

### Throughput

**Phi-3.5 Mini (INT4)**:

- CPU (M2 Max): 25-30 tokens/sec
- GPU (NVIDIA L4): 120-150 tokens/sec
- GPU (A100): 200-250 tokens/sec

## Troubleshooting

### Common Issues

**1. Ollama Connection Failed**

```
Error: connect to ollama: connection refused
```

Solution: Run `ollama serve` first

**2. Model Not Found**

```
Error: model not found: phi3.5
```

Solution: Run `ollama pull phi3.5:3.8b-mini-instruct-q4_K_M`

**3. Tool Calling Not Working**

```
Agent generates text but doesn't call tools
```

Solutions:

- Check `mcp_servers` is configured in agent YAML
- Verify tools are registered with MCP server
- Check logs for "Tool result" messages
- Try adding few-shot examples to prompt

**4. Out of Memory**

```
Error: cannot allocate memory
```

Solutions:

- Use smaller model (Gemma 2B instead of Phi-3.5)
- Reduce context window in config
- Use INT4 quantization instead of INT8

## API Reference

### MCP Server

```go
// Create server
server := mcp.NewServer("my-tools")

// Register tool
server.RegisterTool(mcp.Tool{
    Name:        "tool_name",
    Description: "What the tool does",
    Handler:     toolFunction,
    Schema:      mcp.Schema{ /* ... */ },
})

// Type-safe registration
mcp.RegisterTypedTool(server, "tool_name", "description",
    func(ctx context.Context, input MyInput) (MyOutput, error) {
        // Implementation
    })
```

### Hybrid Inference

```go
import "github.com/aixgo-dev/aixgo/internal/llm/inference"
import "github.com/aixgo-dev/aixgo/internal/llm/runtime/ollama"
import "github.com/aixgo-dev/aixgo/internal/llm/runtime/cloud"

// Create services
local := ollama.NewClient("http://localhost:11434")
cloud := cloud.NewClient("xai")

// Create hybrid
hybrid := inference.NewHybridInference(local, cloud)
hybrid.SetPreferLocal(true)

// Generate
response, err := hybrid.Generate(ctx, inference.GenerateRequest{
    Model:       "phi3.5",
    Prompt:      "What is 2+2?",
    MaxTokens:   100,
    Temperature: 0.7,
})
```

## Next Steps

1. **Try the Example**: `cd examples/huggingface-mcp && go run main.go`
2. **Add Custom Tools**: Create your own tools in `tools/`
3. **Deploy Distributed**: Set up remote MCP servers
4. **Production Deploy**: Use Cloud Run with Docker images

See also:

- [MCP Tools Guide](MCP_TOOLS.md)
- [Deployment Guide](DEPLOYMENT.md)
- [API Reference](https://pkg.go.dev/github.com/aixgo-dev/aixgo)
