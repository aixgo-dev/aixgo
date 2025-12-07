package agent

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"seconds", "30s", 30 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"complex", "1h30m", 90 * time.Minute, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && d.Duration != tt.expected {
				t.Errorf("UnmarshalText() = %v, want %v", d.Duration, tt.expected)
			}
		})
	}
}

func TestAgentDef_GetString(t *testing.T) {
	def := AgentDef{
		Extra: map[string]any{
			"existing": "value",
			"number":   42,
		},
	}

	if got := def.GetString("existing", "default"); got != "value" {
		t.Errorf("GetString() = %v, want %v", got, "value")
	}

	if got := def.GetString("missing", "default"); got != "default" {
		t.Errorf("GetString() = %v, want %v", got, "default")
	}

	if got := def.GetString("number", "default"); got != "default" {
		t.Errorf("GetString() = %v, want %v", got, "default")
	}
}

func TestAgentDef_UnmarshalKey(t *testing.T) {
	tests := []struct {
		name    string
		extra   map[string]any
		key     string
		target  any
		wantErr bool
	}{
		{
			name:   "unmarshal string",
			extra:  map[string]any{"key": "value"},
			key:    "key",
			target: new(string),
		},
		{
			name:   "unmarshal int",
			extra:  map[string]any{"count": float64(42)},
			key:    "count",
			target: new(int),
		},
		{
			name:  "unmarshal struct",
			extra: map[string]any{"config": map[string]any{"host": "localhost", "port": float64(8080)}},
			key:   "config",
			target: new(struct {
				Host string
				Port int
			}),
		},
		{
			name:   "missing key",
			extra:  map[string]any{},
			key:    "missing",
			target: new(string),
		},
		{
			name:   "unmarshal slice",
			extra:  map[string]any{"items": []any{"a", "b", "c"}},
			key:    "items",
			target: new([]string),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := AgentDef{Extra: tt.extra}
			err := def.UnmarshalKey(tt.key, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentDef_YAMLUnmarshal(t *testing.T) {
	yamlData := `
name: test-agent
role: worker
interval: 30s
model: gpt-4
prompt: "Do something"
custom_field: custom_value
nested:
  key: value
  count: 42
`
	var def AgentDef
	if err := yaml.Unmarshal([]byte(yamlData), &def); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if def.Name != "test-agent" {
		t.Errorf("Name = %v, want test-agent", def.Name)
	}
	if def.Role != "worker" {
		t.Errorf("Role = %v, want worker", def.Role)
	}
	if def.Interval.Duration != 30*time.Second {
		t.Errorf("Interval = %v, want 30s", def.Interval.Duration)
	}
	if def.Model != "gpt-4" {
		t.Errorf("Model = %v, want gpt-4", def.Model)
	}

	// Check custom fields captured in Extra
	if def.Extra["custom_field"] != "custom_value" {
		t.Errorf("Extra[custom_field] = %v, want custom_value", def.Extra["custom_field"])
	}

	// Unmarshal nested structure
	var nested struct {
		Key   string `json:"key"`
		Count int    `json:"count"`
	}
	if err := def.UnmarshalKey("nested", &nested); err != nil {
		t.Errorf("UnmarshalKey(nested) error = %v", err)
	}
	if nested.Key != "value" || nested.Count != 42 {
		t.Errorf("nested = %+v, want {Key:value Count:42}", nested)
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	called := false
	factory := func(def AgentDef, rt Runtime) (Agent, error) {
		called = true
		return nil, nil
	}

	reg.Register("test-role", factory)

	f, ok := reg.GetFactory("test-role")
	if !ok {
		t.Error("GetFactory() should find registered factory")
	}

	_, _ = f(AgentDef{}, nil)
	if !called {
		t.Error("Factory should have been called")
	}

	_, ok = reg.GetFactory("unknown")
	if ok {
		t.Error("GetFactory() should not find unregistered factory")
	}
}
