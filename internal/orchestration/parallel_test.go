package orchestration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// MockAgent for testing
type MockAgent struct {
	name      string
	role      string
	delay     time.Duration
	response  string
	callCount int
	mu        sync.Mutex
}

func NewMockAgent(name, role string, delay time.Duration, response string) *MockAgent {
	return &MockAgent{
		name:     name,
		role:     role,
		delay:    delay,
		response: response,
	}
}

func (m *MockAgent) Name() string {
	return m.name
}

func (m *MockAgent) Role() string {
	return m.role
}

func (m *MockAgent) Start(ctx context.Context) error {
	return nil
}

func (m *MockAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	time.Sleep(m.delay)
	return &agent.Message{
		Message: &pb.Message{
			Payload: m.response,
		},
	}, nil
}

// CallCount returns the call count thread-safely
func (m *MockAgent) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *MockAgent) Stop(ctx context.Context) error {
	return nil
}

func (m *MockAgent) Ready() bool {
	return true
}

// MockRuntime for testing
type MockRuntime struct {
	agents map[string]agent.Agent
	mu     sync.RWMutex
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		agents: make(map[string]agent.Agent),
	}
}

func (r *MockRuntime) Register(a agent.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.Name()] = a
	return nil
}

func (r *MockRuntime) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, name)
	return nil
}

func (r *MockRuntime) Get(name string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[name]
	if !ok {
		return nil, agent.ErrAgentNotFound
	}
	return a, nil
}

func (r *MockRuntime) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

func (r *MockRuntime) Send(target string, msg *agent.Message) error {
	return nil
}

func (r *MockRuntime) Recv(source string) (<-chan *agent.Message, error) {
	return nil, nil
}

func (r *MockRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
	a, err := r.Get(target)
	if err != nil {
		return nil, err
	}
	return a.Execute(ctx, input)
}

func (r *MockRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, map[string]error) {
	results := make(map[string]*agent.Message)
	errors := make(map[string]error)

	// Execute in parallel
	type result struct {
		name string
		msg  *agent.Message
		err  error
	}

	ch := make(chan result, len(targets))

	for _, target := range targets {
		go func(t string) {
			msg, err := r.Call(ctx, t, input)
			ch <- result{name: t, msg: msg, err: err}
		}(target)
	}

	for i := 0; i < len(targets); i++ {
		res := <-ch
		if res.err != nil {
			errors[res.name] = res.err
		} else {
			results[res.name] = res.msg
		}
	}

	return results, errors
}

func (r *MockRuntime) Broadcast(msg *agent.Message) error {
	return nil
}

func (r *MockRuntime) Start(ctx context.Context) error {
	return nil
}

func (r *MockRuntime) Stop(ctx context.Context) error {
	return nil
}

// Tests

func TestParallelExecute(t *testing.T) {
	ctx := context.Background()

	// Create mock runtime and agents
	rt := NewMockRuntime()

	agent1 := NewMockAgent("agent1", "test", 100*time.Millisecond, "result1")
	agent2 := NewMockAgent("agent2", "test", 100*time.Millisecond, "result2")
	agent3 := NewMockAgent("agent3", "test", 100*time.Millisecond, "result3")

	_ = rt.Register(agent1)
	_ = rt.Register(agent2)
	_ = rt.Register(agent3)

	// Create parallel orchestrator
	parallel := NewParallel(
		"test-parallel",
		rt,
		[]string{"agent1", "agent2", "agent3"},
	)

	// Execute
	input := &agent.Message{
		Message: &pb.Message{
			Payload: "test input",
		},
	}

	start := time.Now()
	result, err := parallel.Execute(ctx, input)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Parallel execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Check that execution was actually parallel (should be ~100ms, not ~300ms)
	if duration > 200*time.Millisecond {
		t.Errorf("Execution took too long (%v), likely not parallel", duration)
	}

	// Verify all agents were called
	if agent1.CallCount() != 1 {
		t.Errorf("Agent1 call count = %d, want 1", agent1.CallCount())
	}
	if agent2.CallCount() != 1 {
		t.Errorf("Agent2 call count = %d, want 1", agent2.CallCount())
	}
	if agent3.CallCount() != 1 {
		t.Errorf("Agent3 call count = %d, want 1", agent3.CallCount())
	}
}

func TestParallelWithPartialFailure(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	agent1 := NewMockAgent("agent1", "test", 10*time.Millisecond, "result1")
	agent2 := NewMockAgent("agent2", "test", 10*time.Millisecond, "result2")

	_ = rt.Register(agent1)
	_ = rt.Register(agent2)

	// Create parallel orchestrator with non-existent agent
	parallel := NewParallel(
		"test-parallel",
		rt,
		[]string{"agent1", "agent2", "nonexistent"},
	)

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "test input",
		},
	}

	// Should still succeed with partial results (not fail-fast by default)
	result, err := parallel.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Parallel execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}
}

func TestParallelFailFast(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	agent1 := NewMockAgent("agent1", "test", 10*time.Millisecond, "result1")

	_ = rt.Register(agent1)

	// Create parallel orchestrator with fail-fast enabled
	parallel := NewParallel(
		"test-parallel",
		rt,
		[]string{"agent1", "nonexistent"},
		WithFailFast(true),
	)

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "test input",
		},
	}

	// Should fail fast
	_, err := parallel.Execute(ctx, input)

	if err == nil {
		t.Fatal("Expected error with fail-fast, got nil")
	}
}

func TestParallelName(t *testing.T) {
	rt := NewMockRuntime()

	parallel := NewParallel(
		"my-parallel",
		rt,
		[]string{"agent1"},
	)

	if parallel.Name() != "my-parallel" {
		t.Errorf("Name() = %s, want my-parallel", parallel.Name())
	}
}

func TestParallelPattern(t *testing.T) {
	rt := NewMockRuntime()

	parallel := NewParallel(
		"test",
		rt,
		[]string{"agent1"},
	)

	if parallel.Pattern() != "parallel" {
		t.Errorf("Pattern() = %s, want parallel", parallel.Pattern())
	}
}

func TestParallelReady(t *testing.T) {
	rt := NewMockRuntime()

	parallel := NewParallel(
		"test",
		rt,
		[]string{"agent1"},
	)

	if !parallel.Ready() {
		t.Error("Ready() = false, want true")
	}
}
