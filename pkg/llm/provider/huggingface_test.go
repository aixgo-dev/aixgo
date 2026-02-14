package provider

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
)

func TestNewHuggingFaceProvider(t *testing.T) {
	mockInf := &mockInference{}
	provider := NewHuggingFaceProvider(mockInf, "test-model")

	if provider == nil {
		t.Fatal("NewHuggingFaceProvider() returned nil")
		return
	}

	if provider.inference != mockInf {
		t.Error("provider.inference not set correctly")
	}

	if provider.model != "test-model" {
		t.Errorf("provider.model = %q, want 'test-model'", provider.model)
	}

	if provider.mcpClient == nil {
		t.Error("provider.mcpClient is nil")
	}

	if provider.toolRegistry == nil {
		t.Error("provider.toolRegistry is nil")
	}

	if provider.mcpSessions == nil {
		t.Error("provider.mcpSessions is nil")
	}
}

func TestHuggingFaceProvider_Name(t *testing.T) {
	provider := NewHuggingFaceProvider(&mockInference{}, "test-model")
	if provider.Name() != "huggingface" {
		t.Errorf("Name() = %q, want 'huggingface'", provider.Name())
	}
}

func TestHuggingFaceProvider_ConnectMCPServer(t *testing.T) {
	// Setup: create and register a local MCP server
	mcpServer := mcp.NewServer("test-server")
	_ = mcpServer.RegisterTool(mcp.Tool{
		Name:        "calculator",
		Description: "Performs calculations",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			return 42, nil
		},
	})
	mcp.RegisterLocalServer(mcpServer)
	defer func() {
		mcp.ClearLocalServers() // Clean up
	}()

	provider := NewHuggingFaceProvider(&mockInference{}, "test-model")
	ctx := context.Background()

	config := mcp.ServerConfig{
		Name:      "test-server",
		Transport: "local",
	}

	err := provider.ConnectMCPServer(ctx, config)
	if err != nil {
		t.Fatalf("ConnectMCPServer() error = %v, want nil", err)
	}

	// Verify session was created
	if len(provider.mcpSessions) != 1 {
		t.Errorf("ConnectMCPServer() created %d sessions, want 1", len(provider.mcpSessions))
	}

	// Verify tools were registered
	if !provider.toolRegistry.HasTool("calculator") {
		t.Error("ConnectMCPServer() did not register tools")
	}
}

func TestHuggingFaceProvider_CreateCompletion_FinalAnswer(t *testing.T) {
	mockInf := &mockInference{
		responses: []*inference.GenerateResponse{
			{
				Text:         "Thought: I can answer this directly\nFinal Answer: 42",
				FinishReason: "stop",
				Usage: inference.Usage{
					PromptTokens:     10,
					CompletionTokens: 15,
					TotalTokens:      25,
				},
			},
		},
	}

	provider := NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "What is the answer?"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	resp, err := provider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v, want nil", err)
	}

	if resp.Content != "42" {
		t.Errorf("CreateCompletion() content = %q, want '42'", resp.Content)
	}

	if resp.FinishReason != "stop" {
		t.Errorf("CreateCompletion() finish_reason = %q, want 'stop'", resp.FinishReason)
	}

	if resp.Usage.TotalTokens != 25 {
		t.Errorf("CreateCompletion() total_tokens = %d, want 25", resp.Usage.TotalTokens)
	}
}

func TestHuggingFaceProvider_CreateCompletion_WithToolCall(t *testing.T) {
	// Setup: create and register a local MCP server with a tool
	mcpServer := mcp.NewServer("calc-server")
	_ = mcpServer.RegisterTool(mcp.Tool{
		Name:        "multiply",
		Description: "Multiplies two numbers",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			a := args.Int("a")
			b := args.Int("b")
			return a * b, nil
		},
	})
	mcp.RegisterLocalServer(mcpServer)
	defer func() {
		mcp.ClearLocalServers()
	}()

	mockInf := &mockInference{
		responses: []*inference.GenerateResponse{
			{
				Text: `Thought: I need to multiply 6 and 7
Action: multiply
Action Input: {"a": 6, "b": 7}`,
				FinishReason: "stop",
			},
			{
				Text:         "Thought: I have the result\nFinal Answer: The answer is 42",
				FinishReason: "stop",
			},
		},
	}

	provider := NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	// Connect to MCP server
	_ = provider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "calc-server",
		Transport: "local",
	})

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "What is 6 times 7?"},
		},
	}

	resp, err := provider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v, want nil", err)
	}

	if resp.Content != "The answer is 42" {
		t.Errorf("CreateCompletion() content = %q, want 'The answer is 42'", resp.Content)
	}

	// Verify tool was called
	if mockInf.callCount < 2 {
		t.Errorf("CreateCompletion() called inference %d times, want at least 2", mockInf.callCount)
	}
}

func TestHuggingFaceProvider_CreateCompletion_MaxIterations(t *testing.T) {
	mockInf := &mockInference{
		responses: []*inference.GenerateResponse{
			{Text: "Thought: Thinking\nAction: tool1\nAction Input: {}"},
			{Text: "Thought: Still thinking\nAction: tool2\nAction Input: {}"},
			{Text: "Thought: More thinking\nAction: tool3\nAction Input: {}"},
			{Text: "Thought: Keep going\nAction: tool4\nAction Input: {}"},
			{Text: "Thought: One more\nAction: tool5\nAction Input: {}"},
		},
	}

	provider := NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Test question"},
		},
	}

	_, err := provider.CreateCompletion(ctx, req)
	if err == nil {
		t.Error("CreateCompletion() error = nil, want error for max iterations")
	}

	if err.Error() != "max iterations reached without final answer" {
		t.Errorf("CreateCompletion() error = %q, want 'max iterations reached without final answer'", err.Error())
	}
}

func TestHuggingFaceProvider_CreateCompletion_ToolError(t *testing.T) {
	// Setup: create MCP server with failing tool
	mcpServer := mcp.NewServer("fail-server")
	_ = mcpServer.RegisterTool(mcp.Tool{
		Name:        "failing_tool",
		Description: "A tool that fails",
		Handler: func(ctx context.Context, args mcp.Args) (any, error) {
			return nil, errors.New("tool execution failed")
		},
	})
	mcp.RegisterLocalServer(mcpServer)
	defer func() {
		mcp.ClearLocalServers()
	}()

	mockInf := &mockInference{
		responses: []*inference.GenerateResponse{
			{
				Text: `Thought: Try the failing tool
Action: failing_tool
Action Input: {}`,
			},
			{
				Text: "Thought: Tool failed but I can still answer\nFinal Answer: Handled the error",
			},
		},
	}

	provider := NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	_ = provider.ConnectMCPServer(ctx, mcp.ServerConfig{
		Name:      "fail-server",
		Transport: "local",
	})

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	resp, err := provider.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v, want nil", err)
	}

	if resp.Content != "Handled the error" {
		t.Errorf("CreateCompletion() content = %q, want 'Handled the error'", resp.Content)
	}
}

func TestHuggingFaceProvider_CreateCompletion_InferenceError(t *testing.T) {
	mockInf := &mockInference{
		err: errors.New("inference service error"),
	}

	provider := NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	_, err := provider.CreateCompletion(ctx, req)
	if err == nil {
		t.Error("CreateCompletion() error = nil, want error")
	}
}

func TestHuggingFaceProvider_CreateStructured(t *testing.T) {
	provider := NewHuggingFaceProvider(&mockInference{}, "test-model")
	ctx := context.Background()

	req := StructuredRequest{
		CompletionRequest: CompletionRequest{
			Messages: []Message{{Role: "user", Content: "Test"}},
		},
	}

	_, err := provider.CreateStructured(ctx, req)
	if err == nil {
		t.Error("CreateStructured() error = nil, want error")
	}
}

func TestHuggingFaceProvider_CreateStreaming(t *testing.T) {
	// Create mock inference that returns a final answer
	mockInf := &mockInference{
		responses: []*inference.GenerateResponse{
			{Text: "Thought: I'll respond directly\nFinal Answer: Test streaming response"},
		},
	}
	provider := NewHuggingFaceProvider(mockInf, "test-model")
	ctx := context.Background()

	req := CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Test"}},
	}

	stream, err := provider.CreateStreaming(ctx, req)
	if err != nil {
		t.Fatalf("CreateStreaming() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Collect all chunks
	var result strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		result.WriteString(chunk.Delta)
	}

	if result.String() != "Test streaming response" {
		t.Errorf("got content %q, want 'Test streaming response'", result.String())
	}
}

func TestParseToolCall(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantName string
		wantArgs map[string]any
		wantNil  bool
	}{
		{
			name: "valid tool call",
			text: `Thought: I need to calculate
Action: multiply
Action Input: {"a": 5, "b": 3}`,
			wantName: "multiply",
			wantArgs: map[string]any{"a": float64(5), "b": float64(3)},
			wantNil:  false,
		},
		{
			name: "tool call with spaces",
			text: `Action:  calculator
Action Input:  {"value": 42}  `,
			wantName: "calculator",
			wantArgs: map[string]any{"value": float64(42)},
			wantNil:  false,
		},
		{
			name:    "missing action",
			text:    `Thought: Thinking\nAction Input: {"a": 1}`,
			wantNil: true,
		},
		{
			name:    "missing action input",
			text:    `Action: tool1\nThought: Thinking`,
			wantNil: true,
		},
		{
			name:    "invalid JSON",
			text:    `Action: tool1\nAction Input: {invalid json}`,
			wantNil: true,
		},
		{
			name: "fixable JSON with single quotes",
			text: `Action: tool1
Action Input: {'key': 'value'}`,
			wantName: "tool1",
			wantArgs: map[string]any{"key": "value"},
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseToolCall(tt.text)

			if tt.wantNil {
				if result != nil {
					t.Errorf("parseToolCall() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Fatal("parseToolCall() = nil, want non-nil")
				return
			}

			if result.Name != tt.wantName {
				t.Errorf("parseToolCall() name = %q, want %q", result.Name, tt.wantName)
			}

			if len(result.Arguments) != len(tt.wantArgs) {
				t.Errorf("parseToolCall() args length = %d, want %d", len(result.Arguments), len(tt.wantArgs))
			}

			for key, wantVal := range tt.wantArgs {
				gotVal, ok := result.Arguments[key]
				if !ok {
					t.Errorf("parseToolCall() missing arg %q", key)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("parseToolCall() args[%q] = %v, want %v", key, gotVal, wantVal)
				}
			}
		})
	}
}

func TestExtractFinalAnswer(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "simple final answer",
			text: "Final Answer: The result is 42",
			want: "The result is 42",
		},
		{
			name: "final answer with newline",
			text: "Thought: Done\nFinal Answer: Success\nSome extra text",
			want: "Success",
		},
		{
			name: "final answer with whitespace",
			text: "Final Answer:   Trimmed answer   ",
			want: "Trimmed answer",
		},
		{
			name: "no final answer",
			text: "Thought: Thinking\nAction: tool",
			want: "",
		},
		{
			name: "case sensitive",
			text: "final answer: lowercase",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFinalAnswer(tt.text)
			if got != tt.want {
				t.Errorf("extractFinalAnswer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFixCommonJSONErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single quotes to double quotes",
			input: "{'key': 'value'}",
			want:  `{"key": "value"}`,
		},
		{
			name:  "trailing comma in object",
			input: `{"a": 1, "b": 2,}`,
			want:  `{"a": 1, "b": 2}`,
		},
		{
			name:  "trailing comma in array",
			input: `["a", "b", "c",]`,
			want:  `["a", "b", "c"]`,
		},
		{
			name:  "comments removed",
			input: `{"key": "value" /* comment */}`,
			want:  `{"key": "value" }`,
		},
		{
			name:  "line comments removed",
			input: "{\n  \"key\": \"value\" // comment\n}",
			want:  "{\n  \"key\": \"value\" \n}",
		},
		{
			name:  "multiple fixes",
			input: "{'a': 1, 'b': 2,} // comment",
			want:  `{"a": 1, "b": 2} `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixCommonJSONErrors(tt.input)
			if got != tt.want {
				t.Errorf("fixCommonJSONErrors() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHuggingFaceProvider_BuildReActPrompt(t *testing.T) {
	provider := NewHuggingFaceProvider(&mockInference{}, "test-model")

	// Manually add a tool to the registry to test prompt building
	_ = provider.toolRegistry.Register("test-server", []mcp.Tool{
		{
			Name:        "calculator",
			Description: "Performs calculations",
			Schema: mcp.Schema{
				"expression": mcp.SchemaField{
					Type:        "string",
					Description: "The expression to evaluate",
				},
			},
		},
	})

	messages := []Message{
		{Role: "user", Content: "What is 2+2?"},
	}

	prompt := provider.buildReActPrompt(messages, nil)

	// Verify prompt contains expected elements
	expectedElements := []string{
		"You are an AI assistant with access to the following tools",
		"TOOLS:",
		"calculator",
		"Performs calculations",
		"User: What is 2+2?",
		"Thought:",
		"Action:",
		"Action Input:",
		"Final Answer:",
	}

	for _, elem := range expectedElements {
		if !contains(prompt, elem) {
			t.Errorf("buildReActPrompt() missing expected element: %q\nPrompt:\n%s", elem, prompt)
		}
	}
}

// Helper functions and mocks

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockInference struct {
	responses []*inference.GenerateResponse
	err       error
	callCount int
	lastReq   inference.GenerateRequest
}

func (m *mockInference) Generate(ctx context.Context, req inference.GenerateRequest) (*inference.GenerateResponse, error) {
	m.lastReq = req
	m.callCount++

	if m.err != nil {
		return nil, m.err
	}

	if len(m.responses) > 0 {
		// Return responses in sequence
		idx := m.callCount - 1
		if idx >= len(m.responses) {
			idx = len(m.responses) - 1
		}
		return m.responses[idx], nil
	}

	return &inference.GenerateResponse{
		Text:         "Default response",
		FinishReason: "stop",
	}, nil
}

func (m *mockInference) Available() bool {
	return true
}

// Helper to clear local servers (add to mcp package for testing)
func init() {
	// This is a helper for tests - in production code, you might want to add
	// a ClearLocalServers function to the mcp package
}
