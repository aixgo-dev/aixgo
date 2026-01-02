package agent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// MockAgent is a test agent implementation
type MockAgent struct {
	name   string
	role   string
	ready  bool
	mu     sync.RWMutex
	execFn func(ctx context.Context, input *Message) (*Message, error)
}

func NewMockAgent(name, role string) *MockAgent {
	return &MockAgent{
		name:  name,
		role:  role,
		ready: false,
		execFn: func(ctx context.Context, input *Message) (*Message, error) {
			return NewMessage("response", map[string]string{"status": "ok"}), nil
		},
	}
}

func (a *MockAgent) Name() string { return a.name }
func (a *MockAgent) Role() string { return a.role }

func (a *MockAgent) Ready() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ready
}

func (a *MockAgent) Start(ctx context.Context) error {
	a.mu.Lock()
	a.ready = true
	a.mu.Unlock()
	return nil
}

func (a *MockAgent) Execute(ctx context.Context, input *Message) (*Message, error) {
	return a.execFn(ctx, input)
}

func (a *MockAgent) Stop(ctx context.Context) error {
	a.mu.Lock()
	a.ready = false
	a.mu.Unlock()
	return nil
}

// Test Message creation and manipulation
func TestMessage(t *testing.T) {
	t.Run("NewMessage creates valid message", func(t *testing.T) {
		payload := map[string]string{"key": "value"}
		msg := NewMessage("test_type", payload)

		if msg.ID == "" {
			t.Error("Expected non-empty ID")
		}
		if msg.Type != "test_type" {
			t.Errorf("Expected type 'test_type', got '%s'", msg.Type)
		}
		if msg.Timestamp == "" {
			t.Error("Expected non-empty timestamp")
		}
		if msg.Metadata == nil {
			t.Error("Expected initialized metadata map")
		}

		// Verify payload can be unmarshaled
		var result map[string]string
		if err := msg.UnmarshalPayload(&result); err != nil {
			t.Errorf("Failed to unmarshal payload: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("Expected key=value, got key=%s", result["key"])
		}
	})

	t.Run("WithMetadata adds metadata", func(t *testing.T) {
		msg := NewMessage("test", nil).
			WithMetadata("priority", "high").
			WithMetadata("source", "api")

		if msg.GetMetadataString("priority", "") != "high" {
			t.Error("Expected priority=high")
		}
		if msg.GetMetadataString("source", "") != "api" {
			t.Error("Expected source=api")
		}
	})

	t.Run("Clone creates independent copy", func(t *testing.T) {
		original := NewMessage("test", map[string]string{"key": "value"}).
			WithMetadata("meta", "data")

		clone := original.Clone()

		// Verify clone has same values
		if clone.ID != original.ID {
			t.Error("Clone should have same ID")
		}
		if clone.Type != original.Type {
			t.Error("Clone should have same Type")
		}

		// Modify clone metadata
		clone.WithMetadata("meta", "modified")

		// Original should be unchanged
		if original.GetMetadataString("meta", "") == "modified" {
			t.Error("Modifying clone should not affect original")
		}
	})

	t.Run("UnmarshalPayload handles various types", func(t *testing.T) {
		type TestPayload struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}

		payload := TestPayload{Name: "test", Count: 42}
		msg := NewMessage("test", payload)

		var result TestPayload
		if err := msg.UnmarshalPayload(&result); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if result.Name != "test" || result.Count != 42 {
			t.Errorf("Unexpected payload: %+v", result)
		}
	})

	t.Run("UnmarshalPayload returns error for empty payload", func(t *testing.T) {
		msg := &Message{Type: "test", Payload: ""}
		var result interface{}
		if err := msg.UnmarshalPayload(&result); err == nil {
			t.Error("Expected error for empty payload")
		}
	})
}

// Test Agent interface implementation
func TestMockAgent(t *testing.T) {
	agent := NewMockAgent("test-agent", "test")

	if agent.Name() != "test-agent" {
		t.Errorf("Expected name 'test-agent', got '%s'", agent.Name())
	}
	if agent.Role() != "test" {
		t.Errorf("Expected role 'test', got '%s'", agent.Role())
	}
	if agent.Ready() {
		t.Error("Agent should not be ready before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start agent
	go func() {
		if err := agent.Start(ctx); err != nil {
			t.Errorf("Start failed: %v", err)
		}
	}()

	// Wait for agent to be ready
	time.Sleep(10 * time.Millisecond)

	if !agent.Ready() {
		t.Error("Agent should be ready after Start")
	}

	// Test Execute
	input := NewMessage("request", map[string]string{"action": "test"})
	response, err := agent.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var result map[string]string
	if err := response.UnmarshalPayload(&result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("Expected status=ok, got status=%s", result["status"])
	}

	// Stop agent
	if err := agent.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if agent.Ready() {
		t.Error("Agent should not be ready after Stop")
	}
}

// Test LocalRuntime
func TestLocalRuntime(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		rt := NewLocalRuntime()
		agent := NewMockAgent("agent1", "test")

		if err := rt.Register(agent); err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		retrieved, err := rt.Get("agent1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if retrieved.Name() != "agent1" {
			t.Errorf("Expected agent1, got %s", retrieved.Name())
		}

		// Try to register duplicate
		if err := rt.Register(agent); err == nil {
			t.Error("Expected error when registering duplicate agent")
		}
	})

	t.Run("List returns all agents", func(t *testing.T) {
		rt := NewLocalRuntime()
		_ = rt.Register(NewMockAgent("agent1", "test"))
		_ = rt.Register(NewMockAgent("agent2", "test"))
		_ = rt.Register(NewMockAgent("agent3", "test"))

		names := rt.List()
		if len(names) != 3 {
			t.Errorf("Expected 3 agents, got %d", len(names))
		}
	})

	t.Run("Unregister removes agent", func(t *testing.T) {
		rt := NewLocalRuntime()
		agent := NewMockAgent("agent1", "test")
		_ = rt.Register(agent)

		if err := rt.Unregister("agent1"); err != nil {
			t.Fatalf("Unregister failed: %v", err)
		}

		if _, err := rt.Get("agent1"); err == nil {
			t.Error("Expected error when getting unregistered agent")
		}

		// Try to unregister non-existent agent
		if err := rt.Unregister("nonexistent"); err == nil {
			t.Error("Expected error when unregistering non-existent agent")
		}
	})

	t.Run("Call executes agent", func(t *testing.T) {
		rt := NewLocalRuntime()
		agent := NewMockAgent("agent1", "test")
		agent.ready = true // Mark as ready
		_ = rt.Register(agent)

		ctx := context.Background()
		input := NewMessage("request", map[string]string{"data": "test"})

		response, err := rt.Call(ctx, "agent1", input)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var result map[string]string
		if err := response.UnmarshalPayload(&result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("Unexpected response: %+v", result)
		}
	})

	t.Run("Call fails for non-existent agent", func(t *testing.T) {
		rt := NewLocalRuntime()
		ctx := context.Background()
		input := NewMessage("request", nil)

		if _, err := rt.Call(ctx, "nonexistent", input); err == nil {
			t.Error("Expected error when calling non-existent agent")
		}
	})

	t.Run("Call fails for non-ready agent", func(t *testing.T) {
		rt := NewLocalRuntime()
		agent := NewMockAgent("agent1", "test")
		agent.ready = false // Not ready
		_ = rt.Register(agent)

		ctx := context.Background()
		input := NewMessage("request", nil)

		if _, err := rt.Call(ctx, "agent1", input); err == nil {
			t.Error("Expected error when calling non-ready agent")
		}
	})

	t.Run("CallParallel executes multiple agents", func(t *testing.T) {
		rt := NewLocalRuntime()

		agent1 := NewMockAgent("agent1", "test")
		agent1.ready = true
		agent1.execFn = func(ctx context.Context, input *Message) (*Message, error) {
			return NewMessage("response", map[string]string{"agent": "1"}), nil
		}

		agent2 := NewMockAgent("agent2", "test")
		agent2.ready = true
		agent2.execFn = func(ctx context.Context, input *Message) (*Message, error) {
			return NewMessage("response", map[string]string{"agent": "2"}), nil
		}

		_ = rt.Register(agent1)
		_ = rt.Register(agent2)

		ctx := context.Background()
		input := NewMessage("request", nil)

		results, errs := rt.CallParallel(ctx, []string{"agent1", "agent2"}, input)

		if len(errs) > 0 {
			t.Errorf("Expected no errors, got %d: %v", len(errs), errs)
		}
		if len(results) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(results))
		}

		// Verify each result
		var result1, result2 map[string]string
		if err := results["agent1"].UnmarshalPayload(&result1); err != nil {
			t.Fatalf("Failed to unmarshal agent1 result: %v", err)
		}
		if err := results["agent2"].UnmarshalPayload(&result2); err != nil {
			t.Fatalf("Failed to unmarshal agent2 result: %v", err)
		}

		if result1["agent"] != "1" {
			t.Errorf("Expected agent=1, got agent=%s", result1["agent"])
		}
		if result2["agent"] != "2" {
			t.Errorf("Expected agent=2, got agent=%s", result2["agent"])
		}
	})

	t.Run("Send and Recv for async communication", func(t *testing.T) {
		rt := NewLocalRuntime()
		agent := NewMockAgent("agent1", "test")
		_ = rt.Register(agent)

		// Get receive channel
		recvCh, err := rt.Recv("agent1")
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}

		// Send message
		msg := NewMessage("test", map[string]string{"async": "true"})
		if err := rt.Send("agent1", msg); err != nil {
			t.Fatalf("Send failed: %v", err)
		}

		// Receive message
		select {
		case received := <-recvCh:
			if received.Type != "test" {
				t.Errorf("Expected type 'test', got '%s'", received.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for message")
		}
	})

	t.Run("Broadcast sends to all agents", func(t *testing.T) {
		rt := NewLocalRuntime()
		_ = rt.Register(NewMockAgent("agent1", "test"))
		_ = rt.Register(NewMockAgent("agent2", "test"))

		ch1, _ := rt.Recv("agent1")
		ch2, _ := rt.Recv("agent2")

		msg := NewMessage("broadcast", map[string]string{"to": "all"})
		if err := rt.Broadcast(msg); err != nil {
			t.Fatalf("Broadcast failed: %v", err)
		}

		// Both agents should receive the message
		timeout := time.After(100 * time.Millisecond)
		received := 0

		for received < 2 {
			select {
			case <-ch1:
				received++
			case <-ch2:
				received++
			case <-timeout:
				t.Fatalf("Timeout waiting for broadcast (received %d/2)", received)
			}
		}
	})

	t.Run("Start and Stop runtime", func(t *testing.T) {
		rt := NewLocalRuntime()
		agent := NewMockAgent("agent1", "test")
		_ = rt.Register(agent)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start runtime
		if err := rt.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		// Wait for agent to be ready
		time.Sleep(50 * time.Millisecond)

		if !agent.Ready() {
			t.Error("Agent should be ready after runtime start")
		}

		// Try to start again
		if err := rt.Start(ctx); err == nil {
			t.Error("Expected error when starting already started runtime")
		}

		// Stop runtime
		if err := rt.Stop(ctx); err != nil {
			t.Fatalf("Stop failed: %v", err)
		}

		// Try to stop again
		if err := rt.Stop(ctx); err == nil {
			t.Error("Expected error when stopping non-started runtime")
		}
	})
}

// Benchmark tests
func BenchmarkMessageCreation(b *testing.B) {
	payload := map[string]string{"key": "value", "data": "test"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewMessage("test", payload)
	}
}

func BenchmarkMessageUnmarshal(b *testing.B) {
	type Payload struct {
		Key  string `json:"key"`
		Data string `json:"data"`
	}
	msg := NewMessage("test", Payload{Key: "value", Data: "test"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var p Payload
		_ = msg.UnmarshalPayload(&p)
	}
}

func BenchmarkRuntimeCall(b *testing.B) {
	rt := NewLocalRuntime()
	agent := NewMockAgent("agent1", "test")
	agent.ready = true
	_ = rt.Register(agent)

	ctx := context.Background()
	input := NewMessage("request", map[string]string{"data": "test"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rt.Call(ctx, "agent1", input)
	}
}

func BenchmarkRuntimeCallParallel(b *testing.B) {
	rt := NewLocalRuntime()
	for i := 0; i < 5; i++ {
		agent := NewMockAgent(string(rune('a'+i)), "test")
		agent.ready = true
		_ = rt.Register(agent)
	}

	ctx := context.Background()
	input := NewMessage("request", map[string]string{"data": "test"})
	targets := []string{"a", "b", "c", "d", "e"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.CallParallel(ctx, targets, input)
	}
}

// Example usage
func ExampleNewMessage() {
	// Create a message with a structured payload
	type AnalysisRequest struct {
		DocumentID string `json:"document_id"`
		Priority   string `json:"priority"`
	}

	payload := AnalysisRequest{
		DocumentID: "doc-123",
		Priority:   "high",
	}

	msg := NewMessage("analysis_request", payload).
		WithMetadata("source", "api").
		WithMetadata("user_id", "user-456")

	// Marshal to JSON for inspection
	data, _ := json.Marshal(msg)
	_ = data // In real code, you'd send this somewhere

	// Output demonstrates the message was created
	// (actual output would vary due to dynamic ID and timestamp)
}

func ExampleLocalRuntime() {
	// Create a runtime and register agents
	rt := NewLocalRuntime()

	agent := NewMockAgent("analyzer", "analysis")
	_ = rt.Register(agent)
	agent.ready = true

	// Call an agent synchronously
	ctx := context.Background()
	input := NewMessage("analyze", map[string]string{"text": "sample"})
	response, _ := rt.Call(ctx, "analyzer", input)

	_ = response // Process the response

	// Output demonstrates the runtime was used
}
