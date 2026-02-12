package orchestration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// ManagerMockAgent simulates a manager that assigns tasks to teams
type ManagerMockAgent struct {
	*MockAgent
	teamAssignments map[string]any
}

func NewManagerMockAgent(name string, assignments map[string]string) *ManagerMockAgent {
	// Convert map[string]string to map[string]any for metadata
	interfaceAssignments := make(map[string]any)
	for k, v := range assignments {
		interfaceAssignments[k] = v
	}

	return &ManagerMockAgent{
		MockAgent:       NewMockAgent(name, "manager", 10*time.Millisecond, "manager response"),
		teamAssignments: interfaceAssignments,
	}
}

func (m *ManagerMockAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	m.mu.Lock()
	m.callCount++
	checkSynthesis := input != nil && input.Metadata != nil && input.Metadata["synthesis_stage"] == true
	m.mu.Unlock()

	time.Sleep(m.delay)

	// If this is the synthesis stage, return final result
	if checkSynthesis {
		return &agent.Message{
			Message: &pb.Message{
				Payload: "Final synthesized result",
				Type:    "synthesis_result",
			},
		}, nil
	}

	// Otherwise, return task assignments
	return &agent.Message{
		Message: &pb.Message{
			Payload: "Task decomposition",
			Type:    "task_assignment",
			Metadata: map[string]any{
				"team_assignments": m.teamAssignments,
			},
		},
	}, nil
}

func TestHierarchicalExecute(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	// Manager decomposes tasks and assigns to teams
	manager := NewManagerMockAgent("manager", map[string]string{
		"frontend": "Build user interface",
		"backend":  "Create API endpoints",
	})

	// Team workers
	uiWorker := NewMockAgent("ui-worker", "worker", 20*time.Millisecond, "UI completed")
	apiWorker := NewMockAgent("api-worker", "worker", 20*time.Millisecond, "API completed")

	_ = rt.Register(manager)
	_ = rt.Register(uiWorker)
	_ = rt.Register(apiWorker)

	// Create hierarchical orchestrator
	teams := map[string][]string{
		"frontend": {"ui-worker"},
		"backend":  {"api-worker"},
	}

	hierarchical := NewHierarchical("test-hierarchical", rt, "manager", teams)

	// Execute
	input := &agent.Message{
		Message: &pb.Message{
			Id:      "task-1",
			Payload: "Build a web application",
		},
	}

	result, err := hierarchical.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Hierarchical execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.Payload != "Final synthesized result" {
		t.Errorf("Result payload = %s, want final synthesis", result.Payload)
	}

	// Verify manager was called twice (decomposition + synthesis)
	if manager.CallCount() != 2 {
		t.Errorf("Manager call count = %d, want 2", manager.CallCount())
	}

	// Verify workers were called
	if uiWorker.CallCount() != 1 {
		t.Errorf("UI worker call count = %d, want 1", uiWorker.CallCount())
	}
	if apiWorker.CallCount() != 1 {
		t.Errorf("API worker call count = %d, want 1", apiWorker.CallCount())
	}
}

func TestHierarchicalMultipleWorkers(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	manager := NewManagerMockAgent("manager", map[string]string{
		"engineering": "Build the system",
	})

	// Multiple workers in one team
	worker1 := NewMockAgent("worker1", "worker", 10*time.Millisecond, "Part 1 done")
	worker2 := NewMockAgent("worker2", "worker", 10*time.Millisecond, "Part 2 done")

	_ = rt.Register(manager)
	_ = rt.Register(worker1)
	_ = rt.Register(worker2)

	teams := map[string][]string{
		"engineering": {"worker1", "worker2"},
	}

	hierarchical := NewHierarchical("test-hierarchical", rt, "manager", teams)

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "task",
		},
	}

	result, err := hierarchical.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Hierarchical execution failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Both workers should be called in parallel
	if worker1.CallCount() != 1 {
		t.Errorf("Worker1 call count = %d, want 1", worker1.CallCount())
	}
	if worker2.CallCount() != 1 {
		t.Errorf("Worker2 call count = %d, want 1", worker2.CallCount())
	}
}

func TestHierarchicalTeamNotFound(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	// Manager assigns to non-existent team
	manager := NewManagerMockAgent("manager", map[string]string{
		"nonexistent-team": "Some task",
	})

	_ = rt.Register(manager)

	teams := map[string][]string{
		"real-team": {"worker"},
	}

	hierarchical := NewHierarchical("test-hierarchical", rt, "manager", teams)

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "task",
		},
	}

	// Should still complete (skips non-existent team)
	result, err := hierarchical.Execute(ctx, input)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	// Manager should still be called for synthesis
	if manager.CallCount() != 2 {
		t.Errorf("Manager call count = %d, want 2", manager.CallCount())
	}
}

func TestHierarchicalName(t *testing.T) {
	rt := NewMockRuntime()

	hierarchical := NewHierarchical("my-hierarchy", rt, "manager", nil)

	if hierarchical.Name() != "my-hierarchy" {
		t.Errorf("Name() = %s, want my-hierarchy", hierarchical.Name())
	}
}

func TestHierarchicalPattern(t *testing.T) {
	rt := NewMockRuntime()

	hierarchical := NewHierarchical("test", rt, "manager", nil)

	if hierarchical.Pattern() != "hierarchical" {
		t.Errorf("Pattern() = %s, want hierarchical", hierarchical.Pattern())
	}
}

func TestHierarchicalMaxDepth(t *testing.T) {
	rt := NewMockRuntime()

	hierarchical := NewHierarchical("test", rt, "manager", nil, WithMaxDepth(5))

	if hierarchical.maxDepth != 5 {
		t.Errorf("maxDepth = %d, want 5", hierarchical.maxDepth)
	}
}

func TestExtractTeamAssignments(t *testing.T) {
	tests := []struct {
		name     string
		msg      *agent.Message
		wantLen  int
		wantTeam string
	}{
		{
			name: "extract from metadata",
			msg: &agent.Message{
				Message: &pb.Message{
					Metadata: map[string]any{
						"team_assignments": map[string]any{
							"team1": "task1",
							"team2": "task2",
						},
					},
				},
			},
			wantLen:  2,
			wantTeam: "team1",
		},
		{
			name: "structured assignment",
			msg: &agent.Message{
				Message: &pb.Message{
					Metadata: map[string]any{
						"team_assignments": map[string]any{
							"team1": map[string]any{
								"payload": "complex task",
							},
						},
					},
				},
			},
			wantLen:  1,
			wantTeam: "team1",
		},
		{
			name: "nil message",
			msg:  nil,
			wantLen: 0,
		},
		{
			name: "no metadata",
			msg: &agent.Message{
				Message: &pb.Message{
					Payload: "task",
				},
			},
			wantLen: 0,
		},
		{
			name: "no team_assignments key",
			msg: &agent.Message{
				Message: &pb.Message{
					Metadata: map[string]any{
						"other_key": "value",
					},
				},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTeamAssignments(tt.msg)
			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
			}
			if tt.wantTeam != "" {
				if _, ok := result[tt.wantTeam]; !ok {
					t.Errorf("Expected team %s not found in result", tt.wantTeam)
				}
				if result[tt.wantTeam].Type != "team_task" {
					t.Errorf("Task type = %s, want team_task", result[tt.wantTeam].Type)
				}
			}
		})
	}
}

func TestAggregateTeamResults(t *testing.T) {
	tests := []struct {
		name      string
		results   map[string]*agent.Message
		checkFunc func(t *testing.T, result *agent.Message)
	}{
		{
			name:    "empty results",
			results: map[string]*agent.Message{},
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result == nil {
					t.Fatal("Result is nil")
				}
				if result.Type != "team_result" {
					t.Errorf("Type = %s, want team_result", result.Type)
				}
			},
		},
		{
			name: "single result",
			results: map[string]*agent.Message{
				"worker1": {
					Message: &pb.Message{
						Payload: "single result",
					},
				},
			},
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result.Payload != "single result" {
					t.Errorf("Payload = %s, want single result", result.Payload)
				}
			},
		},
		{
			name: "multiple results",
			results: map[string]*agent.Message{
				"worker1": {
					Message: &pb.Message{
						Payload: "result1",
					},
				},
				"worker2": {
					Message: &pb.Message{
						Payload: "result2",
					},
				},
			},
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result == nil {
					t.Fatal("Result is nil")
				}
				if result.Type != "team_aggregated" {
					t.Errorf("Type = %s, want team_aggregated", result.Type)
				}
				if !strings.Contains(result.Payload, "result1") {
					t.Error("Payload missing result1")
				}
				if !strings.Contains(result.Payload, "result2") {
					t.Error("Payload missing result2")
				}
				if result.Metadata == nil {
					t.Fatal("Metadata is nil")
				}
				if result.Metadata["worker_count"] != 2 {
					t.Errorf("worker_count = %v, want 2", result.Metadata["worker_count"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateTeamResults(tt.results)
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestCombineTeamResults(t *testing.T) {
	tests := []struct {
		name        string
		original    *agent.Message
		teamResults map[string]*agent.Message
		checkFunc   func(t *testing.T, result *agent.Message)
	}{
		{
			name:        "nil original",
			original:    nil,
			teamResults: nil,
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result != nil {
					t.Error("Expected nil result for nil original")
				}
			},
		},
		{
			name: "empty team results",
			original: &agent.Message{
				Message: &pb.Message{
					Payload: "original task",
				},
			},
			teamResults: map[string]*agent.Message{},
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result.Payload != "original task" {
					t.Errorf("Payload = %s, want original task", result.Payload)
				}
			},
		},
		{
			name: "combine with team results",
			original: &agent.Message{
				Message: &pb.Message{
					Id:      "task-1",
					Payload: "Build application",
				},
			},
			teamResults: map[string]*agent.Message{
				"frontend": {
					Message: &pb.Message{
						Payload: "UI built",
					},
				},
				"backend": {
					Message: &pb.Message{
						Payload: "API built",
					},
				},
			},
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result == nil {
					t.Fatal("Result is nil")
				}
				if result.Type != "synthesis_input" {
					t.Errorf("Type = %s, want synthesis_input", result.Type)
				}
				if !strings.Contains(result.Payload, "Original Task:") {
					t.Error("Missing original task header")
				}
				if !strings.Contains(result.Payload, "Build application") {
					t.Error("Missing original task content")
				}
				if !strings.Contains(result.Payload, "Team Results:") {
					t.Error("Missing team results header")
				}
				if !strings.Contains(result.Payload, "UI built") {
					t.Error("Missing frontend result")
				}
				if !strings.Contains(result.Payload, "API built") {
					t.Error("Missing backend result")
				}
				if result.Metadata == nil {
					t.Fatal("Metadata is nil")
				}
				if result.Metadata["synthesis_stage"] != true {
					t.Error("synthesis_stage not set")
				}
				if result.Metadata["team_count"] != 2 {
					t.Errorf("team_count = %v, want 2", result.Metadata["team_count"])
				}
			},
		},
		{
			name: "preserve original metadata",
			original: &agent.Message{
				Message: &pb.Message{
					Payload: "task",
					Metadata: map[string]any{
						"priority": "high",
					},
				},
			},
			teamResults: map[string]*agent.Message{
				"team1": {
					Message: &pb.Message{
						Payload: "done",
					},
				},
			},
			checkFunc: func(t *testing.T, result *agent.Message) {
				if result.Metadata["priority"] != "high" {
					t.Error("Original metadata not preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineTeamResults(tt.original, tt.teamResults)
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestHierarchicalWorkerFailure(t *testing.T) {
	ctx := context.Background()

	rt := NewMockRuntime()

	manager := NewManagerMockAgent("manager", map[string]string{
		"team1": "task",
	})

	_ = rt.Register(manager)
	// Don't register worker - it will fail

	teams := map[string][]string{
		"team1": {"nonexistent-worker"},
	}

	hierarchical := NewHierarchical("test-hierarchical", rt, "manager", teams)

	input := &agent.Message{
		Message: &pb.Message{
			Payload: "task",
		},
	}

	_, err := hierarchical.Execute(ctx, input)

	// Should get error from worker failure when agent not found
	if err == nil {
		t.Fatal("Expected error from worker failure, got nil")
	}

	if !strings.Contains(err.Error(), "worker") && !strings.Contains(err.Error(), "failed") {
		t.Errorf("Error message should mention worker failure: %v", err)
	}
}

func TestNewProjectManager(t *testing.T) {
	rt := NewMockRuntime()

	pm := NewProjectManager("project-mgr", rt)

	if pm.Name() != "project-mgr" {
		t.Errorf("Name = %s, want project-mgr", pm.Name())
	}

	if pm.manager != "project-manager" {
		t.Errorf("manager = %s, want project-manager", pm.manager)
	}

	if len(pm.teams) != 3 {
		t.Errorf("team count = %d, want 3", len(pm.teams))
	}

	if _, ok := pm.teams["frontend"]; !ok {
		t.Error("frontend team not found")
	}
	if _, ok := pm.teams["backend"]; !ok {
		t.Error("backend team not found")
	}
	if _, ok := pm.teams["qa"]; !ok {
		t.Error("qa team not found")
	}
}

func TestNewEnterpriseWorkflow(t *testing.T) {
	rt := NewMockRuntime()

	departments := map[string][]string{
		"engineering": {"eng1", "eng2"},
		"sales":       {"sales1"},
	}

	ew := NewEnterpriseWorkflow("enterprise", rt, departments)

	if ew.Name() != "enterprise" {
		t.Errorf("Name = %s, want enterprise", ew.Name())
	}

	if ew.manager != "ceo-agent" {
		t.Errorf("manager = %s, want ceo-agent", ew.manager)
	}

	if ew.maxDepth != 5 {
		t.Errorf("maxDepth = %d, want 5", ew.maxDepth)
	}

	if len(ew.teams) != 2 {
		t.Errorf("team count = %d, want 2", len(ew.teams))
	}
}
