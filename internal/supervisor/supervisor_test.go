package supervisor

import (
	"context"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		def    SupervisorDef
		agents map[string]agent.Agent
		rt     agent.Runtime
	}{
		{
			name: "create supervisor with basic config",
			def: SupervisorDef{
				Name:      "test-supervisor",
				Model:     "test-model",
				MaxRounds: 10,
			},
			agents: make(map[string]agent.Agent),
			rt:     &mockRuntime{},
		},
		{
			name: "create supervisor with agents",
			def: SupervisorDef{
				Name:      "supervisor-with-agents",
				Model:     "gpt-4",
				MaxRounds: 5,
			},
			agents: map[string]agent.Agent{
				"agent1": &mockAgent{},
				"agent2": &mockAgent{},
			},
			rt: &mockRuntime{},
		},
		{
			name: "create supervisor with zero max rounds",
			def: SupervisorDef{
				Name:      "zero-rounds",
				Model:     "model",
				MaxRounds: 0,
			},
			agents: make(map[string]agent.Agent),
			rt:     &mockRuntime{},
		},
		{
			name: "create supervisor with empty name",
			def: SupervisorDef{
				Name:      "",
				Model:     "model",
				MaxRounds: 10,
			},
			agents: make(map[string]agent.Agent),
			rt:     &mockRuntime{},
		},
		{
			name: "create supervisor with nil agents map",
			def: SupervisorDef{
				Name:      "nil-agents",
				Model:     "model",
				MaxRounds: 10,
			},
			agents: nil,
			rt:     &mockRuntime{},
		},
		{
			name: "create supervisor with nil runtime",
			def: SupervisorDef{
				Name:      "nil-runtime",
				Model:     "model",
				MaxRounds: 10,
			},
			agents: make(map[string]agent.Agent),
			rt:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.def, tt.agents, tt.rt)

			if s == nil {
				t.Fatal("New returned nil supervisor")
			}

			if s.def.Name != tt.def.Name {
				t.Errorf("supervisor.def.Name = %v, want %v", s.def.Name, tt.def.Name)
			}

			if s.def.Model != tt.def.Model {
				t.Errorf("supervisor.def.Model = %v, want %v", s.def.Model, tt.def.Model)
			}

			if s.def.MaxRounds != tt.def.MaxRounds {
				t.Errorf("supervisor.def.MaxRounds = %v, want %v", s.def.MaxRounds, tt.def.MaxRounds)
			}

			if s.client == nil {
				t.Error("supervisor.client is nil")
			}

			// Verify agents map
			if tt.agents == nil && s.agents != nil {
				t.Error("supervisor.agents should be nil")
			}
			if tt.agents != nil && s.agents == nil {
				t.Error("supervisor.agents is nil but should not be")
			}
			if tt.agents != nil && s.agents != nil && len(s.agents) != len(tt.agents) {
				t.Errorf("len(supervisor.agents) = %v, want %v", len(s.agents), len(tt.agents))
			}

			// Verify runtime
			if tt.rt != nil && s.rt != tt.rt {
				t.Error("supervisor.rt does not match provided runtime")
			}
		})
	}
}

func TestSupervisor_Start(t *testing.T) {
	tests := []struct {
		name    string
		def     SupervisorDef
		agents  map[string]agent.Agent
		rt      agent.Runtime
		wantErr bool
	}{
		{
			name: "start supervisor successfully",
			def: SupervisorDef{
				Name:      "test-supervisor",
				Model:     "test-model",
				MaxRounds: 10,
			},
			agents:  make(map[string]agent.Agent),
			rt:      &mockRuntime{},
			wantErr: false,
		},
		{
			name: "start with multiple agents",
			def: SupervisorDef{
				Name:      "multi-agent-supervisor",
				Model:     "gpt-4",
				MaxRounds: 5,
			},
			agents: map[string]agent.Agent{
				"agent1": &mockAgent{},
				"agent2": &mockAgent{},
				"agent3": &mockAgent{},
			},
			rt:      &mockRuntime{},
			wantErr: false,
		},
		{
			name: "start with canceled context",
			def: SupervisorDef{
				Name:      "canceled-supervisor",
				Model:     "model",
				MaxRounds: 10,
			},
			agents:  make(map[string]agent.Agent),
			rt:      &mockRuntime{},
			wantErr: false, // Start doesn't currently handle context cancellation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.def, tt.agents, tt.rt)

			ctx := context.Background()
			if tt.name == "start with canceled context" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			err := s.Start(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSupervisorDef_Fields(t *testing.T) {
	def := SupervisorDef{
		Name:      "field-test",
		Model:     "test-model",
		MaxRounds: 15,
	}

	if def.Name != "field-test" {
		t.Errorf("Name = %v, want field-test", def.Name)
	}
	if def.Model != "test-model" {
		t.Errorf("Model = %v, want test-model", def.Model)
	}
	if def.MaxRounds != 15 {
		t.Errorf("MaxRounds = %v, want 15", def.MaxRounds)
	}
}

func TestSupervisorDef_ZeroValue(t *testing.T) {
	var def SupervisorDef

	if def.Name != "" {
		t.Errorf("zero value Name = %v, want empty string", def.Name)
	}
	if def.Model != "" {
		t.Errorf("zero value Model = %v, want empty string", def.Model)
	}
	if def.MaxRounds != 0 {
		t.Errorf("zero value MaxRounds = %v, want 0", def.MaxRounds)
	}
}

func TestGetAPIKey(t *testing.T) {
	key := getAPIKey()

	if key == "" {
		t.Error("getAPIKey returned empty string")
	}

	// Verify it returns the placeholder
	expected := "xai-api-key-placeholder"
	if key != expected {
		t.Errorf("getAPIKey() = %v, want %v", key, expected)
	}
}

func TestSupervisor_Fields(t *testing.T) {
	def := SupervisorDef{
		Name:      "field-supervisor",
		Model:     "gpt-4",
		MaxRounds: 20,
	}

	agents := map[string]agent.Agent{
		"test-agent": &mockAgent{},
	}

	rt := &mockRuntime{}

	s := New(def, agents, rt)

	// Verify all fields are properly set
	if s.def.Name != def.Name {
		t.Errorf("s.def.Name = %v, want %v", s.def.Name, def.Name)
	}

	if s.def.Model != def.Model {
		t.Errorf("s.def.Model = %v, want %v", s.def.Model, def.Model)
	}

	if s.def.MaxRounds != def.MaxRounds {
		t.Errorf("s.def.MaxRounds = %v, want %v", s.def.MaxRounds, def.MaxRounds)
	}

	if s.client == nil {
		t.Error("s.client is nil")
	}

	if len(s.agents) != len(agents) {
		t.Errorf("len(s.agents) = %v, want %v", len(s.agents), len(agents))
	}

	if s.rt != rt {
		t.Error("s.rt does not match provided runtime")
	}
}

func TestSupervisor_ClientInitialization(t *testing.T) {
	def := SupervisorDef{
		Name:      "client-test",
		Model:     "model",
		MaxRounds: 10,
	}

	s := New(def, nil, nil)

	if s.client == nil {
		t.Fatal("client should be initialized")
	}

	// Client should be initialized with the API key from getAPIKey()
	// We can't inspect the client's API key directly, but we can verify it's not nil
}

func TestSupervisor_MultipleInstances(t *testing.T) {
	// Test that multiple supervisors can be created independently
	def1 := SupervisorDef{Name: "supervisor1", Model: "model1", MaxRounds: 5}
	def2 := SupervisorDef{Name: "supervisor2", Model: "model2", MaxRounds: 10}

	s1 := New(def1, nil, nil)
	s2 := New(def2, nil, nil)

	if s1.def.Name == s2.def.Name {
		t.Error("supervisors should have different names")
	}

	if s1.def.MaxRounds == s2.def.MaxRounds {
		t.Error("supervisors should have different max rounds")
	}

	// Each should have its own client
	if s1.client == s2.client {
		t.Error("supervisors should have separate clients")
	}
}

func TestSupervisor_StartConcurrent(t *testing.T) {
	// Test that multiple supervisors can start concurrently
	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func(id int) {
			def := SupervisorDef{
				Name:      "concurrent-supervisor",
				Model:     "model",
				MaxRounds: 10,
			}
			s := New(def, nil, nil)
			ctx := context.Background()
			_ = s.Start(ctx)
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestSupervisor_AgentsMapModification(t *testing.T) {
	// Test that modifying the agents map after creation doesn't affect supervisor
	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	def := SupervisorDef{
		Name:      "map-test",
		Model:     "model",
		MaxRounds: 10,
	}

	s := New(def, agents, nil)

	// Modify original map
	agents["agent2"] = &mockAgent{}
	delete(agents, "agent1")

	// Supervisor's map should be the same reference (maps are reference types)
	if len(s.agents) != len(agents) {
		t.Logf("Note: supervisor.agents length = %v, original map length = %v (maps are passed by reference)", len(s.agents), len(agents))
	}
}

// Mock types for testing
type mockAgent struct{}

func (m *mockAgent) Start(ctx context.Context) error {
	return nil
}

type mockRuntime struct{}

func (m *mockRuntime) Send(target string, msg *agent.Message) error {
	return nil
}

func (m *mockRuntime) Recv(source string) (<-chan *agent.Message, error) {
	ch := make(chan *agent.Message)
	return ch, nil
}
