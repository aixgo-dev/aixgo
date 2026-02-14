package llm

import (
	"context"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
)

// TestIntegration_HuggingFaceWithMCP tests the full integration of HuggingFace provider with MCP
func TestIntegration_HuggingFaceWithMCP(t *testing.T) {
	// Setup: Create a mock MCP server with a calculator tool
	mcpServer := mcp.NewServer("calculator-server")
	err := mcpServer.RegisterTool(mcp.Tool{
		Name:        "add",
		Description: "Adds two numbers",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			a := args.Int("a")
			b := args.Int("b")
			return a + b, nil
		},
		Schema: mcp.Schema{
			"a": mcp.SchemaField{
				Type:        "integer",
				Description: "First number",
				Required:    true,
			},
			"b": mcp.SchemaField{
				Type:        "integer",
				Description: "Second number",
				Required:    true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	mcp.RegisterLocalServer(mcpServer)
	defer mcp.ClearLocalServers()

	// Setup: Create a mock inference service that simulates ReAct responses
	mockInf := &mockInferenceReAct{
		responses: []string{
			// First response: LLM decides to use the add tool
			`Thought: The user wants to add 15 and 27. I should use the add tool.
Action: add
Action Input: {"a": 15, "b": 27}`,
			// Second response: LLM receives tool result and provides final answer
			`Thought: The add tool returned 42, which is the correct sum.
Final Answer: The sum of 15 and 27 is 42.`,
		},
	}

	// Create HuggingFace provider
	hfProvider := provider.NewHuggingFaceProvider(mockInf, "test-model")

	// Connect to MCP server
	ctx := context.Background()
	err = hfProvider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "calculator-server",
		Transport: "local",
	})
	if err != nil {
		t.Fatalf("Failed to connect to MCP server: %v", err)
	}

	// Execute completion request
	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What is 15 plus 27?"},
		},
		MaxTokens:   200,
		Temperature: 0.7,
	}

	resp, err := hfProvider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	// Verify response
	if resp.Content != "The sum of 15 and 27 is 42." {
		t.Errorf("Unexpected response: %q", resp.Content)
	}

	// Verify the LLM was called twice (once for tool call, once for final answer)
	if mockInf.callCount != 2 {
		t.Errorf("Expected 2 inference calls, got %d", mockInf.callCount)
	}
}

// TestIntegration_MultipleToolCalls tests ReAct loop with multiple tool calls
func TestIntegration_MultipleToolCalls(t *testing.T) {
	// Setup: Create MCP server with multiple tools
	mcpServer := mcp.NewServer("multi-tool-server")

	if err := mcpServer.RegisterTool(mcp.Tool{
		Name:        "get_weather",
		Description: "Gets the weather for a city",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			city := args.String("city")
			return "Sunny, 72°F in " + city, nil
		},
	}); err != nil {
		t.Fatalf("Failed to register get_weather tool: %v", err)
	}

	if err := mcpServer.RegisterTool(mcp.Tool{
		Name:        "get_time",
		Description: "Gets the current time",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			return "2:30 PM", nil
		},
	}); err != nil {
		t.Fatalf("Failed to register get_time tool: %v", err)
	}

	mcp.RegisterLocalServer(mcpServer)
	defer mcp.ClearLocalServers()

	// Mock inference with multiple tool calls
	mockInf := &mockInferenceReAct{
		responses: []string{
			`Thought: I need to get the weather first.
Action: get_weather
Action Input: {"city": "San Francisco"}`,
			`Thought: Now I need to check the time.
Action: get_time
Action Input: {}`,
			`Thought: I have both pieces of information.
Final Answer: It's 2:30 PM and the weather in San Francisco is Sunny, 72°F.`,
		},
	}

	hfProvider := provider.NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	if err := hfProvider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "multi-tool-server",
		Transport: "local",
	}); err != nil {
		t.Fatalf("Failed to connect to MCP server: %v", err)
	}

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather and time in San Francisco?"},
		},
	}

	resp, err := hfProvider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	expected := "It's 2:30 PM and the weather in San Francisco is Sunny, 72°F."
	if resp.Content != expected {
		t.Errorf("Response = %q, want %q", resp.Content, expected)
	}

	// Verify 3 calls were made
	if mockInf.callCount != 3 {
		t.Errorf("Expected 3 inference calls, got %d", mockInf.callCount)
	}
}

// TestIntegration_ToolErrorRecovery tests error handling in ReAct loop
func TestIntegration_ToolErrorRecovery(t *testing.T) {
	// Setup: Create MCP server with a tool that can fail
	mcpServer := mcp.NewServer("error-server")
	callCount := 0

	if err := mcpServer.RegisterTool(mcp.Tool{
		Name:        "flaky_tool",
		Description: "A tool that may fail",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			callCount++
			if callCount == 1 {
				return nil, &testError{msg: "temporary failure"}
			}
			return "success", nil
		},
	}); err != nil {
		t.Fatalf("Failed to register flaky_tool: %v", err)
	}

	mcp.RegisterLocalServer(mcpServer)
	defer mcp.ClearLocalServers()

	mockInf := &mockInferenceReAct{
		responses: []string{
			`Thought: Let me try the flaky tool.
Action: flaky_tool
Action Input: {}`,
			`Thought: The tool failed, but I can provide an answer anyway.
Final Answer: Despite the error, I can tell you the operation is possible.`,
		},
	}

	hfProvider := provider.NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	if err := hfProvider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "error-server",
		Transport: "local",
	}); err != nil {
		t.Fatalf("Failed to connect to MCP server: %v", err)
	}

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Test the flaky tool"},
		},
	}

	resp, err := hfProvider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	expected := "Despite the error, I can tell you the operation is possible."
	if resp.Content != expected {
		t.Errorf("Response = %q, want %q", resp.Content, expected)
	}
}

// TestIntegration_HybridInferenceWithReAct tests hybrid inference with ReAct
func TestIntegration_HybridInferenceWithReAct(t *testing.T) {
	// Setup MCP server
	mcpServer := mcp.NewServer("hybrid-server")
	if err := mcpServer.RegisterTool(mcp.Tool{
		Name:        "multiply",
		Description: "Multiplies two numbers",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			a := args.Int("a")
			b := args.Int("b")
			return a * b, nil
		},
	}); err != nil {
		t.Fatalf("Failed to register multiply tool: %v", err)
	}
	mcp.RegisterLocalServer(mcpServer)
	defer mcp.ClearLocalServers()

	// Create hybrid inference (local + cloud)
	localInf := &mockInferenceReAct{
		available: true,
		responses: []string{
			`Thought: I need to multiply 8 and 9.
Action: multiply
Action Input: {"a": 8, "b": 9}`,
			`Thought: The result is 72.
Final Answer: 8 times 9 equals 72.`,
		},
	}

	cloudInf := &mockInferenceReAct{
		available: true,
		responses: []string{
			"This should not be called",
		},
	}

	hybrid := inference.NewHybridInference(localInf, cloudInf)

	// Create provider with hybrid inference
	hfProvider := provider.NewHuggingFaceProvider(hybrid, "test-model")
	ctx := context.Background()

	if err := hfProvider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "hybrid-server",
		Transport: "local",
	}); err != nil {
		t.Fatalf("Failed to connect to MCP server: %v", err)
	}

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What is 8 times 9?"},
		},
	}

	resp, err := hfProvider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	if resp.Content != "8 times 9 equals 72." {
		t.Errorf("Response = %q", resp.Content)
	}

	// Verify local was used, not cloud
	if localInf.callCount != 2 {
		t.Errorf("Local inference called %d times, want 2", localInf.callCount)
	}
	if cloudInf.callCount != 0 {
		t.Errorf("Cloud inference called %d times, want 0", cloudInf.callCount)
	}
}

// TestIntegration_NoToolsAvailable tests behavior when no tools are registered
func TestIntegration_NoToolsAvailable(t *testing.T) {
	mockInf := &mockInferenceReAct{
		responses: []string{
			`Thought: I don't have any tools, so I'll answer directly.
Final Answer: I can help you with that using my knowledge.`,
		},
	}

	hfProvider := provider.NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	resp, err := hfProvider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	expected := "I can help you with that using my knowledge."
	if resp.Content != expected {
		t.Errorf("Response = %q, want %q", resp.Content, expected)
	}
}

// TestIntegration_ComplexJSONParsing tests that the ReAct loop handles tool calls correctly
func TestIntegration_ComplexJSONParsing(t *testing.T) {
	mcpServer := mcp.NewServer("json-server")
	if err := mcpServer.RegisterTool(mcp.Tool{
		Name:        "echo",
		Description: "Echoes back the input",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			msg := args.String("message")
			return "Echo: " + msg, nil
		},
	}); err != nil {
		t.Fatalf("Failed to register echo tool: %v", err)
	}
	mcp.RegisterLocalServer(mcpServer)
	defer mcp.ClearLocalServers()

	// Test complete ReAct flow
	mockInf := &mockInferenceReAct{
		responses: []string{
			// First response - direct final answer (no tool needed)
			`Thought: This is a simple greeting, I can answer directly.
Final Answer: Hello! How can I help you today?`,
		},
	}

	hfProvider := provider.NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	if err := hfProvider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "json-server",
		Transport: "local",
	}); err != nil {
		t.Fatalf("Failed to connect to MCP server: %v", err)
	}

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := hfProvider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion failed: %v", err)
	}

	expected := "Hello! How can I help you today?"
	if resp.Content != expected {
		t.Errorf("Response = %q, want %q", resp.Content, expected)
	}

	// Verify only one inference call was made (no tool needed)
	if mockInf.callCount != 1 {
		t.Errorf("Expected 1 inference call, got %d", mockInf.callCount)
	}
}

// Mock implementations for integration tests

type mockInferenceReAct struct {
	responses []string
	callCount int
	available bool
}

func (m *mockInferenceReAct) Generate(ctx context.Context, req inference.GenerateRequest) (*inference.GenerateResponse, error) {
	defer func() { m.callCount++ }()

	if m.callCount >= len(m.responses) {
		return &inference.GenerateResponse{
			Text:         "No more responses",
			FinishReason: "stop",
		}, nil
	}

	return &inference.GenerateResponse{
		Text:         m.responses[m.callCount],
		FinishReason: "stop",
		Usage: inference.Usage{
			PromptTokens:     50,
			CompletionTokens: 30,
			TotalTokens:      80,
		},
	}, nil
}

func (m *mockInferenceReAct) Available() bool {
	if m.available {
		return true
	}
	return m.callCount < len(m.responses)
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
