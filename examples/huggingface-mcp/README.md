# HuggingFace + MCP Example

This example demonstrates using HuggingFace models (Phi-3.5 Mini) with MCP-based tool calling in aixgo.

## Prerequisites

1. **Ollama** installed and running:
   ```bash
   # Install Ollama (macOS)
   brew install ollama

   # Or download from: https://ollama.com

   # Start Ollama server
   ollama serve
   ```

2. **Pull the Phi-3.5 Mini model**:
   ```bash
   ollama pull phi3.5:3.8b-mini-instruct-q4_K_M
   ```

## Architecture

This example showcases:

- **MCP Tools**: Weather tools defined via MCP (Model Context Protocol)
- **Local Inference**: Phi-3.5 Mini running via Ollama
- **ReAct Pattern**: LLM uses reasoning + action loop to call tools
- **Multi-Agent System**: Producer → Assistant → Logger

```
┌─────────────────────────────────────────┐
│  Producer Agent                         │
│  (generates queries every 5s)           │
└────────────┬────────────────────────────┘
             │
             ↓
┌─────────────────────────────────────────┐
│  Assistant Agent (ReAct)                │
│  - Phi-3.5 Mini via Ollama             │
│  - MCP tool calling                     │
│  - ReAct prompting                      │
└────────────┬────────────────────────────┘
             │
             ↓ (calls MCP tools)
┌─────────────────────────────────────────┐
│  MCP Server (embedded)                  │
│  - get_weather tool                     │
└─────────────────────────────────────────┘
             ↓
┌─────────────────────────────────────────┐
│  Logger Agent                           │
│  (logs all responses)                   │
└─────────────────────────────────────────┘
```

## Running the Example

```bash
# From the example directory
cd examples/huggingface-mcp

# Run the example
go run main.go
```

## Expected Output

```
2025/01/20 15:30:00 Starting HuggingFace + MCP Example
2025/01/20 15:30:00 Make sure Ollama is running: ollama serve
2025/01/20 15:30:00 Registered weather tools with MCP server
2025/01/20 15:30:00 Registered local MCP server: weather-tools
2025/01/20 15:30:00 Created agent: producer (role: producer)
2025/01/20 15:30:00 Created agent: assistant (role: react)
2025/01/20 15:30:00 Created agent: logger (role: logger)
2025/01/20 15:30:00 All agents started. Press Ctrl+C to stop.
2025/01/20 15:30:05 Producer sent: "What's the weather in Tokyo?"
2025/01/20 15:30:07 Assistant: Thought: I need to check the weather in Tokyo
2025/01/20 15:30:07 Assistant: Action: get_weather
2025/01/20 15:30:07 Assistant: Action Input: {"city": "Tokyo"}
2025/01/20 15:30:07 Tool result: {"city":"Tokyo","temperature":72,"condition":"sunny"}
2025/01/20 15:30:08 Assistant: Final Answer: The weather in Tokyo is sunny with a temperature of 72°F
2025/01/20 15:30:08 Logger: Logged message from assistant
```

## Configuration

See `config.yaml` for the complete configuration including:

- MCP server definition (local transport)
- Model service (Phi-3.5 Mini via Ollama)
- Agent definitions with MCP tool access

## Customizing

### Add More Tools

Edit `tools/weather.go` to add more tools:

```go
func RegisterDatabaseTools(server *mcp.Server) error {
    return server.RegisterTool(mcp.Tool{
        Name:        "query_database",
        Description: "Query the database",
        Handler:     queryDatabase,
        Schema: mcp.Schema{
            "query": mcp.SchemaField{
                Type:     "string",
                Required: true,
            },
        },
    })
}
```

### Use Different Models

Change the model in `config.yaml`:

```yaml
model_services:
  - name: gemma-local
    provider: huggingface
    model: gemma2:2b-instruct-q4_0
    runtime: ollama
```

Then pull the model:
```bash
ollama pull gemma2:2b-instruct-q4_0
```

## Troubleshooting

### Ollama not running
```
Error: connect to ollama failed
Solution: Run `ollama serve` in another terminal
```

### Model not found
```
Error: model not found: phi3.5
Solution: Run `ollama pull phi3.5:3.8b-mini-instruct-q4_K_M`
```

### Tool calling not working
- Check that agent has `mcp_servers` configured
- Verify tools are registered with MCP server
- Look for "Tool result" in logs to confirm execution

## Next Steps

1. **Distributed Mode**: Deploy MCP tools as separate services
2. **Cloud Fallback**: Add cloud API fallback when Ollama unavailable
3. **More Tools**: Add database, web search, filesystem tools
4. **Production**: Deploy to Cloud Run with Docker

See main documentation at `/docs` for more details.
