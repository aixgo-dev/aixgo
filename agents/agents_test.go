package agents

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// Test Producer Agent

func TestProducer_Registration(t *testing.T) {
	// Verify producer is registered
	factory, ok := agent.GetFactory("producer")
	if !ok {
		t.Fatal("producer factory not registered")
	}

	def := agent.AgentDef{
		Name:     "test-producer",
		Role:     "producer",
		Interval: agent.Duration{Duration: 1 * time.Second},
		Outputs: []agent.Output{
			{Target: "test-output"},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	ag, err := factory(def, rt)

	if err != nil {
		t.Fatalf("producer factory returned error: %v", err)
	}

	if ag == nil {
		t.Fatal("producer factory returned nil agent")
	}

	producer, ok := ag.(*Producer)
	if !ok {
		t.Fatal("factory did not return *Producer")
	}

	if producer.def.Name != def.Name {
		t.Errorf("producer.def.Name = %v, want %v", producer.def.Name, def.Name)
	}
}

func TestProducer_Start(t *testing.T) {
	def := agent.AgentDef{
		Name:     "test-producer",
		Role:     "producer",
		Interval: agent.Duration{Duration: 100 * time.Millisecond},
		Outputs: []agent.Output{
			{Target: "test-output"},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	rt.channels["test-output"] = make(chan *agent.Message, 10)

	producer := &Producer{def: def}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	// Start producer in goroutine
	done := make(chan error, 1)
	go func() {
		done <- producer.Start(ctx)
	}()

	// Wait for a few messages
	receivedCount := 0
	timeout := time.After(500 * time.Millisecond)

checkMessages:
	for receivedCount < 3 {
		select {
		case msg := <-rt.channels["test-output"]:
			if msg == nil {
				t.Error("received nil message")
			}
			if msg.Type != "ray_burst" {
				t.Errorf("message Type = %v, want ray_burst", msg.Type)
			}
			if msg.Payload == "" {
				t.Error("message Payload is empty")
			}
			receivedCount++
		case <-timeout:
			break checkMessages
		}
	}

	if receivedCount < 2 {
		t.Errorf("received %d messages, want at least 2", receivedCount)
	}

	// Cancel and verify shutdown
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Producer.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for Producer.Start to complete")
	}
}

func TestProducer_MultipleOutputs(t *testing.T) {
	def := agent.AgentDef{
		Name:     "multi-output-producer",
		Role:     "producer",
		Interval: agent.Duration{Duration: 50 * time.Millisecond},
		Outputs: []agent.Output{
			{Target: "output1"},
			{Target: "output2"},
			{Target: "output3"},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	rt.channels["output1"] = make(chan *agent.Message, 10)
	rt.channels["output2"] = make(chan *agent.Message, 10)
	rt.channels["output3"] = make(chan *agent.Message, 10)

	producer := &Producer{def: def}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	// Start producer
	go producer.Start(ctx)

	// Wait for context timeout
	<-ctx.Done()

	// Verify each output received messages
	for i := 1; i <= 3; i++ {
		target := "output" + string(rune('0'+i))
		if len(rt.channels[target]) == 0 {
			t.Errorf("output %s received no messages", target)
		}
	}
}

func TestProducer_ContextCancellation(t *testing.T) {
	def := agent.AgentDef{
		Name:     "cancel-producer",
		Role:     "producer",
		Interval: agent.Duration{Duration: 10 * time.Millisecond},
		Outputs:  []agent.Output{{Target: "output"}},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	rt.channels["output"] = make(chan *agent.Message, 10)

	producer := &Producer{def: def}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	done := make(chan error, 1)
	go func() {
		done <- producer.Start(ctx)
	}()

	// Cancel immediately
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Producer.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Producer did not respond to context cancellation")
	}
}

// Test Logger Agent

func TestLogger_Registration(t *testing.T) {
	factory, ok := agent.GetFactory("logger")
	if !ok {
		t.Fatal("logger factory not registered")
	}

	def := agent.AgentDef{
		Name: "test-logger",
		Role: "logger",
		Inputs: []agent.Input{
			{Source: "test-input"},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	ag, err := factory(def, rt)

	if err != nil {
		t.Fatalf("logger factory returned error: %v", err)
	}

	if ag == nil {
		t.Fatal("logger factory returned nil agent")
	}

	logger, ok := ag.(*Logger)
	if !ok {
		t.Fatal("factory did not return *Logger")
	}

	if logger.def.Name != def.Name {
		t.Errorf("logger.def.Name = %v, want %v", logger.def.Name, def.Name)
	}
}

func TestLogger_Start(t *testing.T) {
	def := agent.AgentDef{
		Name: "test-logger",
		Role: "logger",
		Inputs: []agent.Input{
			{Source: "test-input"},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	inputChan := make(chan *agent.Message, 10)
	rt.channels["test-input"] = inputChan

	logger := &Logger{def: def}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	done := make(chan error, 1)
	go func() {
		done <- logger.Start(ctx)
	}()

	// Give logger time to set up
	time.Sleep(50 * time.Millisecond)

	// Send messages
	for i := 0; i < 3; i++ {
		msg := &agent.Message{Message: &pb.Message{
			Id:      string(rune('A' + i)),
			Type:    "test-type",
			Payload: "test payload",
		}}
		inputChan <- msg
	}

	// Give logger time to process
	time.Sleep(50 * time.Millisecond)

	// Cancel
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Logger.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for Logger.Start to complete")
	}
}

func TestLogger_MultipleInputs(t *testing.T) {
	def := agent.AgentDef{
		Name: "multi-input-logger",
		Role: "logger",
		Inputs: []agent.Input{
			{Source: "input1"},
			{Source: "input2"},
			{Source: "input3"},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	rt.channels["input1"] = make(chan *agent.Message, 10)
	rt.channels["input2"] = make(chan *agent.Message, 10)
	rt.channels["input3"] = make(chan *agent.Message, 10)

	logger := &Logger{def: def}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	done := make(chan error, 1)
	go func() {
		done <- logger.Start(ctx)
	}()

	// Give logger time to set up
	time.Sleep(50 * time.Millisecond)

	// Send to each input
	for i := 1; i <= 3; i++ {
		source := "input" + string(rune('0'+i))
		msg := &agent.Message{Message: &pb.Message{
			Id:      source,
			Type:    "test",
			Payload: "payload from " + source,
		}}
		rt.channels[source] <- msg
	}

	// Give time to process
	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for Logger to complete")
	}
}

func TestLogger_ContextCancellation(t *testing.T) {
	def := agent.AgentDef{
		Name:   "cancel-logger",
		Role:   "logger",
		Inputs: []agent.Input{{Source: "input"}},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	rt.channels["input"] = make(chan *agent.Message, 10)

	logger := &Logger{def: def}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	done := make(chan error, 1)
	go func() {
		done <- logger.Start(ctx)
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Logger.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Logger did not respond to context cancellation")
	}
}

// Test ReActAgent

func TestReActAgent_Registration(t *testing.T) {
	factory, ok := agent.GetFactory("react")
	if !ok {
		t.Fatal("react factory not registered")
	}

	def := agent.AgentDef{
		Name:    "test-react",
		Role:    "react",
		Model:   "test-model",
		Prompt:  "test prompt",
		Inputs:  []agent.Input{{Source: "input"}},
		Outputs: []agent.Output{{Target: "output"}},
		Tools: []agent.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"param": map[string]any{"type": "string"},
					},
					"required": []any{"param"},
				},
			},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	ag, err := factory(def, rt)

	if err != nil {
		t.Fatalf("react factory returned error: %v", err)
	}

	if ag == nil {
		t.Fatal("react factory returned nil agent")
	}

	reactAgent, ok := ag.(*ReActAgent)
	if !ok {
		t.Fatal("factory did not return *ReActAgent")
	}

	if reactAgent.def.Name != def.Name {
		t.Errorf("reactAgent.def.Name = %v, want %v", reactAgent.def.Name, def.Name)
	}

	if reactAgent.model != def.Model {
		t.Errorf("reactAgent.model = %v, want %v", reactAgent.model, def.Model)
	}

	if reactAgent.client == nil {
		t.Error("reactAgent.client is nil")
	}

	if len(reactAgent.tools) != 1 {
		t.Errorf("len(reactAgent.tools) = %v, want 1", len(reactAgent.tools))
	}

	if reactAgent.rt != rt {
		t.Error("reactAgent.rt does not match provided runtime")
	}
}

func TestReActAgent_ToolValidation(t *testing.T) {
	def := agent.AgentDef{
		Name:  "tool-validation-react",
		Role:  "react",
		Model: "test-model",
		Tools: []agent.Tool{
			{
				Name:        "validate_tool",
				Description: "Tool with validation",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"age":  map[string]any{"type": "number"},
					},
					"required": []any{"name", "age"},
				},
			},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	factory, _ := agent.GetFactory("react")
	ag, err := factory(def, rt)

	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}

	reactAgent := ag.(*ReActAgent)

	// Test valid input
	validInput := map[string]any{
		"name": "John",
		"age":  float64(30),
	}

	ctx := context.Background()
	result, err := reactAgent.tools["validate_tool"](ctx, validInput)

	if err != nil {
		t.Errorf("tool with valid input returned error: %v", err)
	}

	if result == nil {
		t.Error("tool returned nil result")
	}

	// Test invalid input (missing required field)
	invalidInput := map[string]any{
		"name": "John",
	}

	_, err = reactAgent.tools["validate_tool"](ctx, invalidInput)

	if err == nil {
		t.Error("expected error for invalid input, got nil")
	}
}

func TestReActAgent_Start(t *testing.T) {
	def := agent.AgentDef{
		Name:    "start-react",
		Role:    "react",
		Model:   "test-model",
		Prompt:  "test prompt",
		Inputs:  []agent.Input{{Source: "react-input"}},
		Outputs: []agent.Output{{Target: "react-output"}},
		Tools:   []agent.Tool{},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	inputChan := make(chan *agent.Message, 10)
	rt.channels["react-input"] = inputChan
	rt.channels["react-output"] = make(chan *agent.Message, 10)

	factory, _ := agent.GetFactory("react")
	ag, _ := factory(def, rt)
	reactAgent := ag.(*ReActAgent)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- reactAgent.Start(ctx)
	}()

	// Give agent time to start
	time.Sleep(50 * time.Millisecond)

	// Close input channel to trigger completion
	close(inputChan)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ReActAgent.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for ReActAgent.Start to complete")
	}
}

func TestReActAgent_MultipleTools(t *testing.T) {
	def := agent.AgentDef{
		Name:  "multi-tool-react",
		Role:  "react",
		Model: "test-model",
		Tools: []agent.Tool{
			{
				Name:        "tool1",
				Description: "First tool",
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"param1": map[string]any{"type": "string"}},
				},
			},
			{
				Name:        "tool2",
				Description: "Second tool",
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"param2": map[string]any{"type": "number"}},
				},
			},
			{
				Name:        "tool3",
				Description: "Third tool",
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"param3": map[string]any{"type": "boolean"}},
				},
			},
		},
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	factory, _ := agent.GetFactory("react")
	ag, err := factory(def, rt)

	if err != nil {
		t.Fatalf("factory returned error: %v", err)
	}

	reactAgent := ag.(*ReActAgent)

	if len(reactAgent.tools) != 3 {
		t.Errorf("len(tools) = %v, want 3", len(reactAgent.tools))
	}

	// Verify all tools exist
	for _, toolDef := range def.Tools {
		if _, ok := reactAgent.tools[toolDef.Name]; !ok {
			t.Errorf("tool %s not found", toolDef.Name)
		}
	}
}

func TestMustMarshal(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"nil", nil},
		{"string", "test"},
		{"number", 42},
		{"map", map[string]any{"key": "value"}},
		{"slice", []string{"a", "b", "c"}},
		{"complex", map[string]any{
			"nested": map[string]any{
				"value": 123,
			},
			"array": []int{1, 2, 3},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustMarshal(tt.input)
			if result == nil {
				t.Error("mustMarshal returned nil")
			}
		})
	}
}

// Mock Runtime for testing
type mockRuntime struct {
	channels  map[string]chan *agent.Message
	sendError error
	recvError error
}

func (m *mockRuntime) Send(target string, msg *agent.Message) error {
	if m.sendError != nil {
		return m.sendError
	}

	ch, ok := m.channels[target]
	if !ok {
		return errors.New("channel not found")
	}

	select {
	case ch <- msg:
		return nil
	default:
		return errors.New("channel full")
	}
}

func (m *mockRuntime) Recv(source string) (<-chan *agent.Message, error) {
	if m.recvError != nil {
		return nil, m.recvError
	}

	ch, ok := m.channels[source]
	if !ok {
		// Create channel if it doesn't exist
		ch = make(chan *agent.Message, 10)
		m.channels[source] = ch
	}

	return ch, nil
}

func TestProducer_NoOutputs(t *testing.T) {
	def := agent.AgentDef{
		Name:     "no-outputs",
		Role:     "producer",
		Interval: agent.Duration{Duration: 50 * time.Millisecond},
		Outputs:  []agent.Output{}, // No outputs
	}

	rt := &mockRuntime{channels: make(map[string]chan *agent.Message)}
	producer := &Producer{def: def}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	// Should not panic
	done := make(chan error, 1)
	go func() {
		done <- producer.Start(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Producer.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for Producer to complete")
	}
}

func TestLogger_RecvError(t *testing.T) {
	def := agent.AgentDef{
		Name:   "recv-error-logger",
		Role:   "logger",
		Inputs: []agent.Input{{Source: "error-input"}},
	}

	rt := &mockRuntime{
		channels:  make(map[string]chan *agent.Message),
		recvError: errors.New("recv error"),
	}

	logger := &Logger{def: def}

	ctx := context.WithValue(context.Background(), agent.RuntimeKey{}, rt)

	err := logger.Start(ctx)

	if err == nil {
		t.Error("expected error from Recv, got nil")
	}
}

func TestProducer_SendError(t *testing.T) {
	def := agent.AgentDef{
		Name:     "send-error-producer",
		Role:     "producer",
		Interval: agent.Duration{Duration: 10 * time.Millisecond},
		Outputs:  []agent.Output{{Target: "error-output"}},
	}

	rt := &mockRuntime{
		channels:  make(map[string]chan *agent.Message),
		sendError: errors.New("send error"),
	}

	producer := &Producer{def: def}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	// Should handle send errors gracefully (logs but doesn't return error)
	done := make(chan error, 1)
	go func() {
		done <- producer.Start(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Producer.Start returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout")
	}
}
