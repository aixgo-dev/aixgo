package agent

import (
	"context"
	"testing"
	"time"
)

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantDur time.Duration
		wantErr bool
	}{
		{
			name:    "valid duration seconds",
			text:    "5s",
			wantDur: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "valid duration minutes",
			text:    "10m",
			wantDur: 10 * time.Minute,
			wantErr: false,
		},
		{
			name:    "valid duration hours",
			text:    "2h",
			wantDur: 2 * time.Hour,
			wantErr: false,
		},
		{
			name:    "valid duration milliseconds",
			text:    "500ms",
			wantDur: 500 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "valid duration microseconds",
			text:    "100us",
			wantDur: 100 * time.Microsecond,
			wantErr: false,
		},
		{
			name:    "valid duration nanoseconds",
			text:    "1000ns",
			wantDur: 1000 * time.Nanosecond,
			wantErr: false,
		},
		{
			name:    "valid combined duration",
			text:    "1h30m",
			wantDur: 1*time.Hour + 30*time.Minute,
			wantErr: false,
		},
		{
			name:    "invalid duration",
			text:    "invalid",
			wantErr: true,
		},
		{
			name:    "empty duration",
			text:    "",
			wantErr: true,
		},
		{
			name:    "negative duration",
			text:    "-5s",
			wantDur: -5 * time.Second,
			wantErr: false,
		},
		{
			name:    "zero duration",
			text:    "0s",
			wantDur: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.text))

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if d.Duration != tt.wantDur {
					t.Errorf("Duration = %v, want %v", d.Duration, tt.wantDur)
				}
			}
		})
	}
}

func TestAgentDef_GetString(t *testing.T) {
	tests := []struct {
		name   string
		def    AgentDef
		key    string
		defVal string
		want   string
	}{
		{
			name: "key exists with string value",
			def: AgentDef{
				Extra: map[string]any{
					"custom_key": "custom_value",
				},
			},
			key:    "custom_key",
			defVal: "default",
			want:   "custom_value",
		},
		{
			name: "key does not exist",
			def: AgentDef{
				Extra: map[string]any{},
			},
			key:    "missing_key",
			defVal: "default",
			want:   "default",
		},
		{
			name: "key exists with non-string value",
			def: AgentDef{
				Extra: map[string]any{
					"number_key": 123,
				},
			},
			key:    "number_key",
			defVal: "default",
			want:   "default",
		},
		{
			name: "nil Extra map",
			def: AgentDef{
				Extra: nil,
			},
			key:    "any_key",
			defVal: "default",
			want:   "default",
		},
		{
			name: "empty string value",
			def: AgentDef{
				Extra: map[string]any{
					"empty": "",
				},
			},
			key:    "empty",
			defVal: "default",
			want:   "",
		},
		{
			name: "complex string value",
			def: AgentDef{
				Extra: map[string]any{
					"path": "/path/to/file.txt",
				},
			},
			key:    "path",
			defVal: "/default/path",
			want:   "/path/to/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.GetString(tt.key, tt.defVal)
			if got != tt.want {
				t.Errorf("GetString(%q, %q) = %q, want %q", tt.key, tt.defVal, got, tt.want)
			}
		})
	}
}

func TestAgentDef_UnmarshalKey(t *testing.T) {
	def := AgentDef{}

	// Test that UnmarshalKey returns nil (TODO implementation)
	err := def.UnmarshalKey("test_key", map[string]any{})
	if err != nil {
		t.Errorf("UnmarshalKey returned error: %v, want nil (TODO implementation)", err)
	}

	// Test with various input types
	tests := []struct {
		name string
		key  string
		val  any
	}{
		{"string value", "key1", "value"},
		{"int value", "key2", 123},
		{"map value", "key3", map[string]any{"nested": "value"}},
		{"nil value", "key4", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := def.UnmarshalKey(tt.key, tt.val)
			if err != nil {
				t.Errorf("UnmarshalKey(%q, %v) returned error: %v", tt.key, tt.val, err)
			}
		})
	}
}

func TestAgentDef_Fields(t *testing.T) {
	def := AgentDef{
		Name:     "test-agent",
		Role:     "test-role",
		Interval: Duration{Duration: 5 * time.Second},
		Listen:   "test-channel",
		Inputs: []Input{
			{Source: "input1"},
			{Source: "input2"},
		},
		Outputs: []Output{
			{Target: "output1", Addr: "addr1"},
			{Target: "output2"},
		},
		Model:  "test-model",
		Prompt: "test-prompt",
		Tools: []Tool{
			{
				Name:        "tool1",
				Description: "Test tool",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		Extra: map[string]any{
			"custom": "value",
		},
	}

	if def.Name != "test-agent" {
		t.Errorf("Name = %v, want test-agent", def.Name)
	}
	if def.Role != "test-role" {
		t.Errorf("Role = %v, want test-role", def.Role)
	}
	if def.Interval.Duration != 5*time.Second {
		t.Errorf("Interval = %v, want 5s", def.Interval.Duration)
	}
	if def.Listen != "test-channel" {
		t.Errorf("Listen = %v, want test-channel", def.Listen)
	}
	if len(def.Inputs) != 2 {
		t.Errorf("len(Inputs) = %v, want 2", len(def.Inputs))
	}
	if len(def.Outputs) != 2 {
		t.Errorf("len(Outputs) = %v, want 2", len(def.Outputs))
	}
	if def.Model != "test-model" {
		t.Errorf("Model = %v, want test-model", def.Model)
	}
	if def.Prompt != "test-prompt" {
		t.Errorf("Prompt = %v, want test-prompt", def.Prompt)
	}
	if len(def.Tools) != 1 {
		t.Errorf("len(Tools) = %v, want 1", len(def.Tools))
	}
	if def.Extra["custom"] != "value" {
		t.Errorf("Extra[custom] = %v, want value", def.Extra["custom"])
	}
}

func TestInput_Fields(t *testing.T) {
	input := Input{Source: "test-source"}
	if input.Source != "test-source" {
		t.Errorf("Source = %v, want test-source", input.Source)
	}
}

func TestOutput_Fields(t *testing.T) {
	output := Output{
		Target: "test-target",
		Addr:   "test-addr",
	}
	if output.Target != "test-target" {
		t.Errorf("Target = %v, want test-target", output.Target)
	}
	if output.Addr != "test-addr" {
		t.Errorf("Addr = %v, want test-addr", output.Addr)
	}
}

func TestTool_Fields(t *testing.T) {
	tool := Tool{
		Name:        "test-tool",
		Description: "Test description",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param": map[string]any{"type": "string"},
			},
		},
	}

	if tool.Name != "test-tool" {
		t.Errorf("Name = %v, want test-tool", tool.Name)
	}
	if tool.Description != "Test description" {
		t.Errorf("Description = %v, want Test description", tool.Description)
	}
	if tool.InputSchema["type"] != "object" {
		t.Errorf("InputSchema[type] = %v, want object", tool.InputSchema["type"])
	}
}

func TestRuntimeKey(t *testing.T) {
	// Test that RuntimeKey can be used as a context key
	key1 := RuntimeKey{}
	key2 := RuntimeKey{}

	// Two different instances should be considered equal for context use
	ctx := context.WithValue(context.Background(), key1, "test-value")
	val := ctx.Value(key2)

	if val != "test-value" {
		t.Errorf("context value = %v, want test-value", val)
	}
}

func TestRuntimeFromContext(t *testing.T) {
	// Create a mock runtime
	mockRuntime := &mockRuntime{}

	// Test with runtime in context
	ctx := context.WithValue(context.Background(), RuntimeKey{}, mockRuntime)
	rt, ok := RuntimeFromContext(ctx)

	if !ok {
		t.Error("RuntimeFromContext returned false for context with runtime")
	}

	if rt != mockRuntime {
		t.Error("RuntimeFromContext did not return the expected runtime")
	}
}

func TestRuntimeFromContext_Missing(t *testing.T) {
	// Test that RuntimeFromContext returns false when runtime is not in context
	ctx := context.Background()
	rt, ok := RuntimeFromContext(ctx)

	if ok {
		t.Error("RuntimeFromContext returned true when runtime missing from context")
	}

	if rt != nil {
		t.Error("RuntimeFromContext returned non-nil runtime when missing from context")
	}
}

func TestMustRuntimeFromContext(t *testing.T) {
	// Create a mock runtime
	mockRuntime := &mockRuntime{}

	// Test with runtime in context
	ctx := context.WithValue(context.Background(), RuntimeKey{}, mockRuntime)
	rt := MustRuntimeFromContext(ctx)

	if rt != mockRuntime {
		t.Error("MustRuntimeFromContext did not return the expected runtime")
	}
}

func TestMustRuntimeFromContext_Panic(t *testing.T) {
	// Test that MustRuntimeFromContext panics when runtime is not in context
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustRuntimeFromContext did not panic when runtime missing from context")
		}
	}()

	ctx := context.Background()
	_ = MustRuntimeFromContext(ctx)
}

func TestMessage_Wrapper(t *testing.T) {
	// Test Message wrapper around pb.Message
	// The Message type is just a wrapper, so we just verify it exists
	// The actual proto.Message type is tested in proto package
	_ = Message{}
}

// Mock runtime for testing
type mockRuntime struct{}

func (m *mockRuntime) Send(target string, msg *Message) error {
	return nil
}

func (m *mockRuntime) Recv(source string) (<-chan *Message, error) {
	ch := make(chan *Message)
	return ch, nil
}

func TestAgentDef_EmptyExtra(t *testing.T) {
	def := AgentDef{
		Name:  "test",
		Role:  "test-role",
		Extra: map[string]any{},
	}

	// Test GetString with empty Extra
	val := def.GetString("nonexistent", "default")
	if val != "default" {
		t.Errorf("GetString on empty Extra = %v, want default", val)
	}
}

func TestAgentDef_MultipleToolsAndIOStreams(t *testing.T) {
	def := AgentDef{
		Name: "complex-agent",
		Role: "complex",
		Inputs: []Input{
			{Source: "input1"},
			{Source: "input2"},
			{Source: "input3"},
		},
		Outputs: []Output{
			{Target: "output1", Addr: "addr1"},
			{Target: "output2", Addr: "addr2"},
			{Target: "output3", Addr: "addr3"},
		},
		Tools: []Tool{
			{Name: "tool1", Description: "desc1", InputSchema: map[string]any{}},
			{Name: "tool2", Description: "desc2", InputSchema: map[string]any{}},
			{Name: "tool3", Description: "desc3", InputSchema: map[string]any{}},
		},
	}

	if len(def.Inputs) != 3 {
		t.Errorf("len(Inputs) = %v, want 3", len(def.Inputs))
	}
	if len(def.Outputs) != 3 {
		t.Errorf("len(Outputs) = %v, want 3", len(def.Outputs))
	}
	if len(def.Tools) != 3 {
		t.Errorf("len(Tools) = %v, want 3", len(def.Tools))
	}

	// Verify order is preserved
	if def.Inputs[0].Source != "input1" || def.Inputs[2].Source != "input3" {
		t.Error("Input order not preserved")
	}
	if def.Outputs[0].Target != "output1" || def.Outputs[2].Target != "output3" {
		t.Error("Output order not preserved")
	}
	if def.Tools[0].Name != "tool1" || def.Tools[2].Name != "tool3" {
		t.Error("Tool order not preserved")
	}
}
