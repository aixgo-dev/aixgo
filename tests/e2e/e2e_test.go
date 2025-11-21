package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	"github.com/aixgo-dev/aixgo/pkg/security"
)

// TestE2E_SimpleRequestResponse tests a basic request-response cycle
func TestE2E_SimpleRequestResponse(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup mock provider response
	env.Provider().AddTextResponse("Hello! I received your message.")

	// Verify provider works
	resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
	})

	AssertNoError(t, err, "completion should succeed")
	AssertEqual(t, "Hello! I received your message.", resp.Content, "response content")
}

// TestE2E_ToolExecution tests tool execution flow
func TestE2E_ToolExecution(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Register a test tool
	env.MCPServer().RegisterTool("get_time", "Gets the current time", func(ctx context.Context, args mcp.Args) (any, error) {
		return time.Now().Format(time.RFC3339), nil
	})

	// Call the tool
	result, err := env.MCPServer().CallTool(env.Context(), "get_time", nil)
	AssertNoError(t, err, "tool call should succeed")

	// Verify result is a timestamp
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	_, err = time.Parse(time.RFC3339, resultStr)
	AssertNoError(t, err, "result should be valid RFC3339 timestamp")

	// Verify call was recorded
	calls := env.MCPServer().GetCalls()
	AssertEqual(t, 1, len(calls), "should have one tool call")
	AssertEqual(t, "get_time", calls[0].Name, "tool name")
}

// TestE2E_MessageRouting tests message routing through the runtime
func TestE2E_MessageRouting(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup channel
	ch, err := env.Runtime().Recv("test-channel")
	AssertNoError(t, err, "recv should succeed")

	// Send message in goroutine
	go func() {
		msg := CreateTestMessage("msg-1", "test", "Hello World")
		env.Runtime().SendMessage("test-channel", msg)
	}()

	// Receive message
	select {
	case msg := <-ch:
		AssertEqual(t, "msg-1", msg.Id, "message ID")
		AssertEqual(t, "Hello World", msg.Payload, "message payload")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestE2E_SecurityValidation tests security validation in E2E flow
func TestE2E_SecurityValidation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	validator := &security.StringValidator{
		MaxLength:         1000,
		DisallowNullBytes: true,
		CheckSQLInjection: true,
	}

	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid input", "Hello, world!", false},
		{"sql injection", "'; DROP TABLE users;--", true},
		{"null bytes", "test\x00data", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.Validate(tc.input)
			if tc.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestE2E_MultiStepConversation tests multi-turn conversation
func TestE2E_MultiStepConversation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup responses for multi-turn conversation
	env.Provider().AddTextResponse("I understand you want to know about the weather.")
	env.Provider().AddTextResponse("The weather in Tokyo is sunny and 25 degrees.")
	env.Provider().AddTextResponse("Is there anything else you'd like to know?")

	messages := []string{
		"What's the weather like?",
		"Tell me about Tokyo specifically.",
		"Thanks!",
	}

	for i, msg := range messages {
		resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: msg},
			},
		})

		AssertNoError(t, err, "completion should succeed for message "+string(rune('0'+i)))
		if resp.Content == "" {
			t.Errorf("response %d should not be empty", i)
		}
	}

	// Verify all calls were made
	calls := env.Provider().GetCalls()
	AssertEqual(t, 3, len(calls), "should have 3 calls")
}

// TestE2E_ToolCallWithProvider tests provider tool call simulation
func TestE2E_ToolCallWithProvider(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Setup tool call response followed by final response
	env.Provider().AddToolCallResponse("weather", map[string]any{
		"location": "Tokyo",
	})
	env.Provider().AddTextResponse("The weather in Tokyo is sunny.")

	// First call should return tool call
	resp1, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather in Tokyo?"},
		},
	})

	AssertNoError(t, err, "first completion should succeed")
	AssertEqual(t, 1, len(resp1.ToolCalls), "should have one tool call")
	AssertEqual(t, "weather", resp1.ToolCalls[0].Function.Name, "tool name")

	// Second call should return final response
	resp2, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "What's the weather in Tokyo?"},
			{Role: "assistant", Content: ""},
			{Role: "tool", Content: `{"temperature": 25, "condition": "sunny"}`},
		},
	})

	AssertNoError(t, err, "second completion should succeed")
	AssertContains(t, resp2.Content, "sunny", "response should mention weather")
}

// TestE2E_AuditLogging tests audit logging integration
func TestE2E_AuditLogging(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Log some events
	env.AuditLogger().LogAuthAttempt(env.Context(), true, nil)
	env.AuditLogger().LogToolExecution(env.Context(), "test_tool", nil, "success", nil)

	// Verify events
	events := env.AuditLogger().GetEvents()
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
}

// TestE2E_AgentDefinitionCreation tests creating agent definitions
func TestE2E_AgentDefinitionCreation(t *testing.T) {
	def := CreateTestAgentDef("test-agent", "react", "gpt-4")

	AssertEqual(t, "test-agent", def.Name, "agent name")
	AssertEqual(t, "react", def.Role, "agent role")
	AssertEqual(t, "gpt-4", def.Model, "agent model")
	AssertEqual(t, "input-test-agent", def.Inputs[0].Source, "input source")
	AssertEqual(t, "output-test-agent", def.Outputs[0].Target, "output target")
}

// TestE2E_RateLimiting tests rate limiting behavior
func TestE2E_RateLimiting(t *testing.T) {
	rateLimiter := security.NewRateLimiter(5.0, 5)
	clientID := "test-client"

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !rateLimiter.Allow(clientID) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be rate limited
	if rateLimiter.Allow(clientID) {
		t.Error("6th request should be rate limited")
	}
}

// TestE2E_ContextPropagation tests context propagation
func TestE2E_ContextPropagation(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Create auth context
	principal := &security.Principal{
		ID:    "user-123",
		Name:  "Test User",
		Roles: []string{"user"},
	}

	authCtx := &security.AuthContext{
		Principal:   principal,
		SessionID:   "session-123",
		IPAddress:   "127.0.0.1",
		RequestTime: time.Now(),
	}

	ctx := security.WithAuthContext(env.Context(), authCtx)

	// Retrieve auth context
	retrieved, err := security.GetAuthContext(ctx)
	AssertNoError(t, err, "should get auth context")
	AssertEqual(t, "user-123", retrieved.Principal.ID, "principal ID")
	AssertEqual(t, "session-123", retrieved.SessionID, "session ID")
}

// TestE2E_ConcurrentOperations tests concurrent operations
func TestE2E_ConcurrentOperations(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Add enough responses for concurrent calls
	for i := 0; i < 10; i++ {
		env.Provider().AddTextResponse("Response")
	}

	done := make(chan bool, 10)

	// Launch concurrent requests
	for i := 0; i < 10; i++ {
		go func(id int) {
			_, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Hello"},
				},
			})
			if err != nil {
				t.Errorf("concurrent request %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent operations")
		}
	}
}

// TestE2E_ErrorRecovery tests error recovery scenarios
func TestE2E_ErrorRecovery(t *testing.T) {
	env := NewTestEnvironment(t)
	defer env.Cleanup()

	// Test with invalid tool call
	result, err := env.MCPServer().CallTool(env.Context(), "nonexistent_tool", nil)

	// Should not error, just return nil
	if err != nil {
		t.Errorf("calling nonexistent tool should not error: %v", err)
	}
	if result != nil {
		t.Errorf("result should be nil for nonexistent tool")
	}
}

// TestE2E_AgentDefWithTools tests agent definition with tools
func TestE2E_AgentDefWithTools(t *testing.T) {
	def := agent.AgentDef{
		Name:  "tool-agent",
		Role:  "react",
		Model: "gpt-4",
		Tools: []agent.Tool{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
		},
		Inputs:  []agent.Input{{Source: "input"}},
		Outputs: []agent.Output{{Target: "output"}},
	}

	AssertEqual(t, 1, len(def.Tools), "should have one tool")
	AssertEqual(t, "search", def.Tools[0].Name, "tool name")
}
