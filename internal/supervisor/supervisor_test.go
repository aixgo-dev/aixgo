package supervisor

import (
	"context"
	"os"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

func init() {
	_ = os.Setenv("OPENAI_API_KEY", "test-key-for-testing")
}

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
			name: "create supervisor with zero max rounds defaults to 10",
			def: SupervisorDef{
				Name:      "zero-rounds",
				Model:     "model",
				MaxRounds: 0,
			},
			agents: make(map[string]agent.Agent),
			rt:     &mockRuntime{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.def, tt.agents, tt.rt)
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			if s.def.Name != tt.def.Name {
				t.Errorf("supervisor.def.Name = %v, want %v", s.def.Name, tt.def.Name)
			}

			if s.client == nil {
				t.Error("supervisor.client is nil")
			}
		})
	}
}

func TestSupervisor_Start(t *testing.T) {
	def := SupervisorDef{
		Name:      "test-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	s, err := New(def, make(map[string]agent.Agent), &mockRuntime{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	err = s.Start(context.Background())
	if err != nil {
		t.Errorf("Start returned error: %v", err)
	}
}

func TestSupervisor_Run_NoAgents(t *testing.T) {
	def := SupervisorDef{
		Name:      "test-supervisor",
		Model:     "test-model",
		MaxRounds: 5,
	}

	s, err := New(def, make(map[string]agent.Agent), nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := s.Run(context.Background(), "test input")
	if err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	// With no agents, should return empty summary
	if result != "" {
		t.Errorf("expected empty result with no agents, got %q", result)
	}
}

func TestSupervisor_Run_WithAgent(t *testing.T) {
	def := SupervisorDef{
		Name:            "test-supervisor",
		Model:           "test-model",
		MaxRounds:       5,
		RoutingStrategy: StrategyRoundRobin,
	}

	agents := map[string]agent.Agent{
		"test-agent": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := s.Run(context.Background(), "test input")
	if err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestSupervisor_TaskManagement(t *testing.T) {
	def := SupervisorDef{
		Name:      "task-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	// Test AssignTask
	err = s.AssignTask("task1", "Test task", "agent1")
	if err != nil {
		t.Errorf("AssignTask returned error: %v", err)
	}

	// Test GetTask
	task, exists := s.GetTask("task1")
	if !exists {
		t.Error("task1 should exist")
	}
	if task.Status != TaskPending {
		t.Errorf("task status = %v, want %v", task.Status, TaskPending)
	}

	// Test CompleteTask
	err = s.CompleteTask("task1", "completed successfully")
	if err != nil {
		t.Errorf("CompleteTask returned error: %v", err)
	}

	task, _ = s.GetTask("task1")
	if task.Status != TaskCompleted {
		t.Errorf("task status = %v, want %v", task.Status, TaskCompleted)
	}
}

func TestSupervisor_TaskManagement_Errors(t *testing.T) {
	def := SupervisorDef{
		Name:      "task-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	// Test AssignTask to non-existent agent
	err = s.AssignTask("task1", "Test task", "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent agent")
	}

	// Test CompleteTask for non-existent task
	err = s.CompleteTask("nonexistent", "result")
	if err == nil {
		t.Error("expected error for non-existent task")
	}

	// Test FailTask for non-existent task
	err = s.FailTask("nonexistent", "reason")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestSupervisor_FailTask(t *testing.T) {
	def := SupervisorDef{
		Name:      "task-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_ = s.AssignTask("task1", "Test task", "agent1")

	err = s.FailTask("task1", "failed reason")
	if err != nil {
		t.Errorf("FailTask returned error: %v", err)
	}

	task, _ := s.GetTask("task1")
	if task.Status != TaskFailed {
		t.Errorf("task status = %v, want %v", task.Status, TaskFailed)
	}
	if task.Result != "failed reason" {
		t.Errorf("task result = %v, want %v", task.Result, "failed reason")
	}
}

func TestSupervisor_GetPendingTasks(t *testing.T) {
	def := SupervisorDef{
		Name:      "task-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_ = s.AssignTask("task1", "Task 1", "agent1")
	_ = s.AssignTask("task2", "Task 2", "agent1")
	_ = s.AssignTask("task3", "Task 3", "agent1")

	_ = s.CompleteTask("task2", "done")

	pending := s.GetPendingTasks()
	if len(pending) != 2 {
		t.Errorf("pending tasks count = %v, want 2", len(pending))
	}
}

func TestSupervisor_Handoff(t *testing.T) {
	def := SupervisorDef{
		Name:      "handoff-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
		"agent2": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	resp, err := s.Handoff(context.Background(), "agent1", "agent2", "handoff message")
	if err != nil {
		t.Errorf("Handoff returned error: %v", err)
	}

	if resp.AgentName != "agent2" {
		t.Errorf("response agent = %v, want agent2", resp.AgentName)
	}
}

func TestSupervisor_Handoff_NonexistentAgent(t *testing.T) {
	def := SupervisorDef{
		Name:      "handoff-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, err = s.Handoff(context.Background(), "agent1", "nonexistent", "message")
	if err == nil {
		t.Error("expected error for non-existent target agent")
	}
}

func TestSupervisor_GetCurrentRound(t *testing.T) {
	def := SupervisorDef{
		Name:      "round-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	s, err := New(def, make(map[string]agent.Agent), nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	round := s.GetCurrentRound()
	if round != 0 {
		t.Errorf("initial round = %v, want 0", round)
	}
}

func TestSupervisor_GetMessages(t *testing.T) {
	def := SupervisorDef{
		Name:            "msg-supervisor",
		Model:           "test-model",
		MaxRounds:       5,
		RoutingStrategy: StrategyRoundRobin,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	_, _ = s.Run(context.Background(), "test input")

	msgs := s.GetMessages()
	if len(msgs) < 1 {
		t.Error("expected at least one message")
	}

	// First message should be user input
	if msgs[0].Role != "user" {
		t.Errorf("first message role = %v, want user", msgs[0].Role)
	}
}

func TestSupervisor_RoutingStrategies(t *testing.T) {
	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
		"agent2": &mockAgent{},
	}

	tests := []struct {
		name     string
		strategy RoutingStrategy
	}{
		{"round_robin", StrategyRoundRobin},
		{"best_match", StrategyBestMatch},
		{"manual", StrategyManual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := SupervisorDef{
				Name:            "strategy-supervisor",
				Model:           "test-model",
				MaxRounds:       5,
				RoutingStrategy: tt.strategy,
			}

			s, err := New(def, agents, nil)
			if err != nil {
				t.Fatalf("New returned error: %v", err)
			}

			if s.def.RoutingStrategy != tt.strategy {
				t.Errorf("strategy = %v, want %v", s.def.RoutingStrategy, tt.strategy)
			}
		})
	}
}

func TestSupervisor_SelectManual(t *testing.T) {
	def := SupervisorDef{
		Name:            "manual-supervisor",
		Model:           "test-model",
		MaxRounds:       5,
		RoutingStrategy: StrategyManual,
	}

	agents := map[string]agent.Agent{
		"coder":    &mockAgent{},
		"reviewer": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	// Test manual selection with @agent syntax
	selected := s.selectManual("@coder write some code")
	if selected != "coder" {
		t.Errorf("selected = %v, want coder", selected)
	}

	// Test fallback when agent not found
	selected = s.selectManual("@nonexistent do something")
	if selected == "" {
		t.Error("expected fallback to first agent")
	}

	// Test fallback when no @ prefix
	selected = s.selectManual("just a message")
	if selected == "" {
		t.Error("expected fallback to first agent")
	}
}

func TestSupervisor_GetAgents(t *testing.T) {
	def := SupervisorDef{
		Name:      "get-agents-supervisor",
		Model:     "test-model",
		MaxRounds: 10,
	}

	agents := map[string]agent.Agent{
		"agent1": &mockAgent{},
		"agent2": &mockAgent{},
	}

	s, err := New(def, agents, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	gotAgents := s.GetAgents()
	if len(gotAgents) != 2 {
		t.Errorf("agents count = %v, want 2", len(gotAgents))
	}
}

func TestGetAPIKeyFromEnv(t *testing.T) {
	key := getAPIKeyFromEnv("gpt-4-turbo")
	if key == "" {
		t.Error("getAPIKeyFromEnv returned empty string for gpt model")
	}

	key = getAPIKeyFromEnv("gpt-4")
	if key == "" {
		t.Error("getAPIKeyFromEnv returned empty string for gpt model")
	}
}

func TestSupervisorDef_Fields(t *testing.T) {
	def := SupervisorDef{
		Name:            "field-test",
		Model:           "test-model",
		MaxRounds:       15,
		RoutingStrategy: StrategyBestMatch,
		SystemPrompt:    "You are a supervisor",
	}

	if def.Name != "field-test" {
		t.Errorf("Name = %v, want field-test", def.Name)
	}
	if def.MaxRounds != 15 {
		t.Errorf("MaxRounds = %v, want 15", def.MaxRounds)
	}
	if def.RoutingStrategy != StrategyBestMatch {
		t.Errorf("RoutingStrategy = %v, want %v", def.RoutingStrategy, StrategyBestMatch)
	}
}

func TestTask_Fields(t *testing.T) {
	task := Task{
		ID:          "task-123",
		Description: "Test task",
		AssignedTo:  "agent1",
		Status:      TaskPending,
		Result:      "",
		Round:       0,
	}

	if task.ID != "task-123" {
		t.Errorf("ID = %v, want task-123", task.ID)
	}
	if task.Status != TaskPending {
		t.Errorf("Status = %v, want %v", task.Status, TaskPending)
	}
}

func TestAgentResponse_Fields(t *testing.T) {
	resp := AgentResponse{
		AgentName: "agent1",
		Content:   "response content",
		NextAgent: "agent2",
		Complete:  true,
	}

	if resp.AgentName != "agent1" {
		t.Errorf("AgentName = %v, want agent1", resp.AgentName)
	}
	if !resp.Complete {
		t.Error("Complete should be true")
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "Hello",
		Agent:   "agent1",
	}

	if msg.Role != "assistant" {
		t.Errorf("Role = %v, want assistant", msg.Role)
	}
	if msg.Agent != "agent1" {
		t.Errorf("Agent = %v, want agent1", msg.Agent)
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
