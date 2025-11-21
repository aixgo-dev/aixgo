package main

import (
	"log"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/examples/huggingface-mcp/tools"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
)

func main() {
	log.Println("Starting HuggingFace + MCP Example")
	log.Println("Make sure Ollama is running: ollama serve")
	log.Println("And model is pulled: ollama pull phi3.5:3.8b-mini-instruct-q4_K_M")

	// Create MCP server
	mcpServer := mcp.NewServer("weather-tools")

	// Register tools
	if err := tools.RegisterWeatherTools(mcpServer); err != nil {
		log.Fatalf("Failed to register weather tools: %v", err)
	}

	log.Println("Registered weather tools with MCP server")

	// Run aixgo with MCP server
	if err := aixgo.RunWithMCP("config.yaml", mcpServer); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
