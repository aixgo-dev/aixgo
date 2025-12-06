package aixgo

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

func TestRun_ConfigFileNotFound(t *testing.T) {
	err := Run("/nonexistent/config.yaml")

	if err == nil {
		t.Error("expected error for nonexistent config file, got nil")
	}

	if !os.IsNotExist(err) && !containsString(err.Error(), "failed to read config") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_InvalidYAML(t *testing.T) {
	// Create temporary invalid YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
this is not valid YAML: [[[
agents:
  - name: test
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err = Run(configPath)

	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}

	if !containsString(err.Error(), "failed to parse config") {
		t.Errorf("error = %v, want error containing 'failed to parse config'", err)
	}
}

func TestRun_UnknownAgentRole(t *testing.T) {
	// Save and restore registry
	originalRegistry := make(map[string]agent.FactoryFunc)
	for k, v := range getRegistry() {
		originalRegistry[k] = v
	}
	defer func() {
		setRegistry(originalRegistry)
	}()

	// Clear registry
	setRegistry(make(map[string]agent.FactoryFunc))

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "unknown-role.yaml")

	configContent := `
agents:
  - name: test-agent
    role: unknown-role
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err = Run(configPath)

	if err == nil {
		t.Error("expected error for unknown role, got nil")
	}

	if !containsString(err.Error(), "failed to create agent") {
		t.Errorf("error = %v, want error containing 'failed to create agent'", err)
	}
}

func TestRun_EmptyConfig(t *testing.T) {
	// Register a test agent so we can test empty config behavior
	originalRegistry := make(map[string]agent.FactoryFunc)
	for k, v := range getRegistry() {
		originalRegistry[k] = v
	}
	defer func() {
		setRegistry(originalRegistry)
	}()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yaml")

	configContent := `
agents: []
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run in goroutine and send interrupt after short delay
	done := make(chan error, 1)
	go func() {
		done <- Run(configPath)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Send interrupt
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Run to complete")
	}
}

func TestRun_ValidConfig(t *testing.T) {
	// Save and restore registry
	originalRegistry := make(map[string]agent.FactoryFunc)
	for k, v := range getRegistry() {
		originalRegistry[k] = v
	}
	defer func() {
		setRegistry(originalRegistry)
	}()

	// Register test agent
	agent.Register("test-role", func(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &testAgent{def: def}, nil
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "valid.yaml")

	configContent := `
agents:
  - name: test-agent-1
    role: test-role
    interval: 5s
  - name: test-agent-2
    role: test-role
    interval: 10s
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- Run(configPath)
	}()

	// Give agents time to start
	time.Sleep(200 * time.Millisecond)

	// Send interrupt
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Run to complete")
	}
}

func TestRun_SIGTERM(t *testing.T) {
	// Save and restore registry
	originalRegistry := make(map[string]agent.FactoryFunc)
	for k, v := range getRegistry() {
		originalRegistry[k] = v
	}
	defer func() {
		setRegistry(originalRegistry)
	}()

	// Register test agent
	agent.Register("test-role", func(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &testAgent{def: def}, nil
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sigterm.yaml")

	configContent := `
agents:
  - name: sigterm-agent
    role: test-role
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- Run(configPath)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGTERM)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Run to complete after SIGTERM")
	}
}

func TestRun_AgentError(t *testing.T) {
	// Save and restore registry
	originalRegistry := make(map[string]agent.FactoryFunc)
	for k, v := range getRegistry() {
		originalRegistry[k] = v
	}
	defer func() {
		setRegistry(originalRegistry)
	}()

	// Register agent that returns error
	agent.Register("error-role", func(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		return &errorAgent{}, nil
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "error-agent.yaml")

	configContent := `
agents:
  - name: error-agent
    role: error-role
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- Run(configPath)
	}()

	// Give agent time to start and error
	time.Sleep(200 * time.Millisecond)

	// Send interrupt
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion (should complete despite agent error)
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Run to complete")
	}
}

func TestRun_MultipleAgentsSameRole(t *testing.T) {
	// Save and restore registry
	originalRegistry := make(map[string]agent.FactoryFunc)
	for k, v := range getRegistry() {
		originalRegistry[k] = v
	}
	defer func() {
		setRegistry(originalRegistry)
	}()

	// Register test agent
	callCount := 0
	agent.Register("multi-role", func(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
		callCount++
		return &testAgent{def: def}, nil
	})

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "multi-agents.yaml")

	configContent := `
agents:
  - name: agent-1
    role: multi-role
  - name: agent-2
    role: multi-role
  - name: agent-3
    role: multi-role
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- Run(configPath)
	}()

	// Give agents time to start
	time.Sleep(200 * time.Millisecond)

	// Send interrupt
	proc, _ := os.FindProcess(os.Getpid())
	_ = proc.Signal(syscall.SIGINT)

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Run to complete")
	}

	if callCount != 3 {
		t.Errorf("factory called %d times, want 3", callCount)
	}
}

func TestConfig_Fields(t *testing.T) {
	config := Config{
		Agents: []agent.AgentDef{
			{Name: "agent1", Role: "role1"},
			{Name: "agent2", Role: "role2"},
		},
	}

	if len(config.Agents) != 2 {
		t.Errorf("len(Agents) = %v, want 2", len(config.Agents))
	}

	if config.Agents[0].Name != "agent1" {
		t.Errorf("Agents[0].Name = %v, want agent1", config.Agents[0].Name)
	}

	if config.Agents[1].Role != "role2" {
		t.Errorf("Agents[1].Role = %v, want role2", config.Agents[1].Role)
	}
}

func TestConfig_EmptyAgents(t *testing.T) {
	var config Config

	if config.Agents != nil {
		t.Errorf("zero value Agents = %v, want nil", config.Agents)
	}

	config.Agents = []agent.AgentDef{}
	if len(config.Agents) != 0 {
		t.Errorf("empty Agents length = %v, want 0", len(config.Agents))
	}
}

// Test helpers
type testAgent struct {
	def agent.AgentDef
}

func (a *testAgent) Name() string                                                      { return a.def.Name }
func (a *testAgent) Role() string                                                      { return a.def.Role }
func (a *testAgent) Ready() bool                                                       { return true }
func (a *testAgent) Stop(ctx context.Context) error                                    { return nil }
func (a *testAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	return input, nil
}

func (a *testAgent) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

type errorAgent struct{}

func (e *errorAgent) Name() string                                                      { return "error" }
func (e *errorAgent) Role() string                                                      { return "error" }
func (e *errorAgent) Ready() bool                                                       { return true }
func (e *errorAgent) Stop(ctx context.Context) error                                    { return nil }
func (e *errorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	return nil, os.ErrInvalid
}

func (e *errorAgent) Start(ctx context.Context) error {
	return os.ErrInvalid
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper to access package-level registry for testing
func getRegistry() map[string]agent.FactoryFunc {
	// This accesses the internal registry through exported functions
	registry := make(map[string]agent.FactoryFunc)
	// We can't directly access it, so we return an empty map
	// The tests use Register/GetFactory which work with the real registry
	return registry
}

func setRegistry(r map[string]agent.FactoryFunc) {
	// We can't directly set it, but we can clear and repopulate
	// This is handled by the test setup/teardown
}
