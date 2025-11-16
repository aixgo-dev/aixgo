package agent

import (
	"context"
	"errors"
	"testing"
)

func TestRegister(t *testing.T) {
	// Note: We cannot directly access the registry, so we test via Register/GetFactory
	// We'll use unique role names to avoid conflicts

	tests := []struct {
		name    string
		role    string
		factory FactoryFunc
	}{
		{
			name: "register simple factory",
			role: "test-role-unique-1",
			factory: func(def AgentDef, rt Runtime) (Agent, error) {
				return &testAgent{}, nil
			},
		},
		{
			name: "register another role",
			role: "another-role-unique-2",
			factory: func(def AgentDef, rt Runtime) (Agent, error) {
				return &testAgent{}, nil
			},
		},
		{
			name: "register factory with error",
			role: "error-role-unique-3",
			factory: func(def AgentDef, rt Runtime) (Agent, error) {
				return nil, errors.New("factory error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Register(tt.role, tt.factory)

			// Verify factory was registered
			factory, ok := GetFactory(tt.role)
			if !ok {
				t.Errorf("Factory for role %q was not registered", tt.role)
			}
			if factory == nil {
				t.Error("Registered factory is nil")
			}
		})
	}
}

func TestRegister_Overwrite(t *testing.T) {
	role := "test-role-overwrite-unique"

	// Register first factory
	factory1 := func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{name: "agent1"}, nil
	}
	Register(role, factory1)

	// Register second factory with same role (should overwrite)
	factory2 := func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{name: "agent2"}, nil
	}
	Register(role, factory2)

	// Get factory and create agent
	factory, ok := GetFactory(role)
	if !ok {
		t.Fatal("Factory not found after overwrite")
	}

	agent, err := factory(AgentDef{}, nil)
	if err != nil {
		t.Fatalf("Factory returned error: %v", err)
	}

	testAg := agent.(*testAgent)
	if testAg.name != "agent2" {
		t.Errorf("Factory was not overwritten, got agent %q, want agent2", testAg.name)
	}
}

func TestGetFactory(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		register   bool
		wantExists bool
	}{
		{
			name:       "get existing factory",
			role:       "existing-role-unique-get",
			register:   true,
			wantExists: true,
		},
		{
			name:       "get non-existing factory",
			role:       "non-existing-role-xyz-123-never-registered",
			register:   false,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.register {
				Register(tt.role, func(def AgentDef, rt Runtime) (Agent, error) {
					return &testAgent{}, nil
				})
			}

			factory, ok := GetFactory(tt.role)

			if ok != tt.wantExists {
				t.Errorf("GetFactory(%q) ok = %v, want %v", tt.role, ok, tt.wantExists)
			}

			if tt.wantExists && factory == nil {
				t.Error("Factory exists but is nil")
			}

			if !tt.wantExists && factory != nil {
				t.Error("Factory should not exist but is not nil")
			}
		})
	}
}

func TestCreateAgent(t *testing.T) {
	// Register test factories with unique names
	Register("success-role-create-unique", func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{name: def.Name}, nil
	})

	Register("error-role-create-unique", func(def AgentDef, rt Runtime) (Agent, error) {
		return nil, errors.New("factory error")
	})

	tests := []struct {
		name    string
		def     AgentDef
		wantErr bool
		errMsg  string
	}{
		{
			name: "create agent with registered role",
			def: AgentDef{
				Name: "test-agent",
				Role: "success-role-create-unique",
			},
			wantErr: false,
		},
		{
			name: "create agent with unregistered role",
			def: AgentDef{
				Name: "unknown-agent",
				Role: "unknown-role-never-exists-xyz",
			},
			wantErr: true,
			errMsg:  "unknown role: unknown-role-never-exists-xyz",
		},
		{
			name: "create agent with factory error",
			def: AgentDef{
				Name: "error-agent",
				Role: "error-role-create-unique",
			},
			wantErr: true,
			errMsg:  "factory error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := CreateAgent(tt.def, &mockRuntime{})

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if agent == nil {
					t.Error("agent is nil")
				}
			}
		})
	}
}

func TestCreateAgent_WithRuntime(t *testing.T) {
	// Register factory that uses runtime
	Register("runtime-role-unique-rt", func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{
			name:    def.Name,
			runtime: rt,
		}, nil
	})

	rt := &mockRuntime{}
	def := AgentDef{
		Name: "runtime-agent",
		Role: "runtime-role-unique-rt",
	}

	agent, err := CreateAgent(def, rt)
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	testAg := agent.(*testAgent)
	if testAg.runtime != rt {
		t.Error("Agent runtime was not set correctly")
	}
}

func TestCreateAgent_MultipleAgents(t *testing.T) {
	// Register multiple agent types
	Register("type-a-multi-unique", func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{name: "agent-a"}, nil
	})

	Register("type-b-multi-unique", func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{name: "agent-b"}, nil
	})

	Register("type-c-multi-unique", func(def AgentDef, rt Runtime) (Agent, error) {
		return &testAgent{name: "agent-c"}, nil
	})

	agents := []AgentDef{
		{Name: "a1", Role: "type-a-multi-unique"},
		{Name: "b1", Role: "type-b-multi-unique"},
		{Name: "c1", Role: "type-c-multi-unique"},
	}

	rt := &mockRuntime{}

	for _, def := range agents {
		agent, err := CreateAgent(def, rt)
		if err != nil {
			t.Errorf("CreateAgent(%v) returned error: %v", def.Name, err)
		}
		if agent == nil {
			t.Errorf("CreateAgent(%v) returned nil agent", def.Name)
		}
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	// Test concurrent registration and retrieval with unique roles
	// to avoid race conditions on the same key
	done := make(chan bool, 10)

	// Register different roles concurrently
	for i := 0; i < 5; i++ {
		go func(id int) {
			role := "concurrent-role-register-" + intToStr(id)
			Register(role, func(def AgentDef, rt Runtime) (Agent, error) {
				return &testAgent{}, nil
			})
			done <- true
		}(i)
	}

	// Get different roles concurrently
	for i := 0; i < 5; i++ {
		go func(id int) {
			role := "concurrent-role-get-" + intToStr(id)
			GetFactory(role) // May or may not exist
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all registered factories exist
	for i := 0; i < 5; i++ {
		role := "concurrent-role-register-" + intToStr(i)
		_, ok := GetFactory(role)
		if !ok {
			t.Errorf("Factory for role %s should exist after concurrent registration", role)
		}
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := "0123456789"
	var result []byte
	for n > 0 {
		result = append([]byte{digits[n%10]}, result...)
		n /= 10
	}
	return string(result)
}

// Test helper types
type testAgent struct {
	name    string
	runtime Runtime
}

func (a *testAgent) Start(ctx context.Context) error {
	return nil
}
