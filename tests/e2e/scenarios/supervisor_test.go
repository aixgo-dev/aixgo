package scenarios

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/aixgo-dev/aixgo/tests/e2e"
)

// SupervisorState tracks supervisor orchestration state
type SupervisorState struct {
	mu           sync.Mutex
	tasksIssued  []string
	tasksResults map[string]string
	phase        string
}

// NewSupervisorState creates a new supervisor state
func NewSupervisorState() *SupervisorState {
	return &SupervisorState{
		tasksIssued:  make([]string, 0),
		tasksResults: make(map[string]string),
		phase:        "init",
	}
}

// IssueTask records a task being issued
func (s *SupervisorState) IssueTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasksIssued = append(s.tasksIssued, taskID)
}

// RecordResult records a task result
func (s *SupervisorState) RecordResult(taskID, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasksResults[taskID] = result
}

// SetPhase sets the current phase
func (s *SupervisorState) SetPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

// GetPhase gets the current phase
func (s *SupervisorState) GetPhase() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

// TestSupervisor_BasicOrchestration tests basic supervisor orchestration
func TestSupervisor_BasicOrchestration(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	state := NewSupervisorState()

	// Setup supervisor responses
	env.Provider().AddTextResponse("Planning phase: I will assign tasks to workers.")
	env.Provider().AddTextResponse("Execution phase: All tasks completed successfully.")
	env.Provider().AddTextResponse("Summary: Project completed with 3 tasks.")

	// Phase 1: Planning
	state.SetPhase("planning")
	resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "You are a supervisor agent."},
			{Role: "user", Content: "Plan the project"},
		},
	})
	e2e.AssertNoError(t, err, "planning phase")
	e2e.AssertContains(t, resp.Content, "Planning", "should indicate planning")

	// Phase 2: Task execution
	state.SetPhase("execution")
	for i := 0; i < 3; i++ {
		taskID := "task-" + string(rune('A'+i))
		state.IssueTask(taskID)
		state.RecordResult(taskID, "completed")
	}

	// Phase 3: Summarize
	state.SetPhase("summary")
	summaryResp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "You are a supervisor agent."},
			{Role: "user", Content: "Summarize the project"},
		},
	})
	e2e.AssertNoError(t, err, "summary phase")
	e2e.AssertContains(t, summaryResp.Content, "completed", "should indicate completion")

	e2e.AssertEqual(t, "summary", state.GetPhase(), "should be in summary phase")
}

// TestSupervisor_TaskDelegation tests supervisor task delegation
func TestSupervisor_TaskDelegation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	// Register delegation tool
	delegatedTasks := make([]string, 0)
	var delegateMu sync.Mutex

	env.MCPServer().RegisterTool("delegate_task", "Delegate a task to a worker", func(ctx context.Context, args mcp.Args) (any, error) {
		taskName := args.String("task")
		workerID := args.String("worker")

		delegateMu.Lock()
		delegatedTasks = append(delegatedTasks, taskName+"->"+workerID)
		delegateMu.Unlock()

		return map[string]any{
			"status":  "delegated",
			"task":    taskName,
			"worker":  workerID,
			"task_id": "task-123",
		}, nil
	})

	// Setup supervisor to delegate tasks
	env.Provider().AddToolCallResponse("delegate_task", map[string]any{
		"task":   "analyze_data",
		"worker": "analyst-1",
	})
	env.Provider().AddTextResponse("Task delegated to analyst-1")

	// Supervisor delegates task
	resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "You are a supervisor. Delegate tasks to workers."},
			{Role: "user", Content: "Analyze the sales data"},
		},
	})
	e2e.AssertNoError(t, err, "delegation request")
	e2e.AssertEqual(t, 1, len(resp.ToolCalls), "should delegate via tool call")

	// Execute delegation
	_, err = env.MCPServer().CallTool(env.Context(), "delegate_task", map[string]any{
		"task":   "analyze_data",
		"worker": "analyst-1",
	})
	e2e.AssertNoError(t, err, "delegation execution")

	delegateMu.Lock()
	numDelegated := len(delegatedTasks)
	delegateMu.Unlock()

	e2e.AssertEqual(t, 1, numDelegated, "one task should be delegated")
}

// TestSupervisor_WorkerManagement tests supervisor worker management
func TestSupervisor_WorkerManagement(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	workers := map[string]agent.AgentDef{
		"worker-1": e2e.CreateTestAgentDef("worker-1", "react", "gpt-4"),
		"worker-2": e2e.CreateTestAgentDef("worker-2", "react", "gpt-4"),
		"worker-3": e2e.CreateTestAgentDef("worker-3", "react", "gpt-4"),
	}

	// Setup responses
	for range workers {
		env.Provider().AddTextResponse("Worker task completed")
	}

	// Supervisor manages workers
	results := make(map[string]string)
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	for workerID := range workers {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Execute task for " + id},
				},
			})
			if err != nil {
				return
			}

			resultsMu.Lock()
			results[id] = resp.Content
			resultsMu.Unlock()
		}(workerID)
	}

	wg.Wait()

	resultsMu.Lock()
	numResults := len(results)
	resultsMu.Unlock()

	e2e.AssertEqual(t, len(workers), numResults, "all workers should complete")
}

// TestSupervisor_ErrorHandling tests supervisor error handling
func TestSupervisor_ErrorHandling(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	state := NewSupervisorState()

	// Setup: task fails, supervisor retries
	env.Provider().AddTextResponse("Task failed: worker-1 encountered error")
	env.Provider().AddTextResponse("Retrying task with worker-2")
	env.Provider().AddTextResponse("Task completed successfully by worker-2")

	// Initial failure
	state.SetPhase("execution")
	resp1, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "user", Content: "Execute task"},
		},
	})
	e2e.AssertNoError(t, err, "first attempt")

	// Detect failure and retry
	if hasSubstring(resp1.Content, "failed") {
		state.SetPhase("retry")
		resp2, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "Retry with different worker"},
			},
		})
		e2e.AssertNoError(t, err, "retry attempt")
		e2e.AssertContains(t, resp2.Content, "Retry", "should indicate retry")
	}
}

// TestSupervisor_PriorityScheduling tests priority-based task scheduling
func TestSupervisor_PriorityScheduling(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	type Task struct {
		ID       string
		Priority int
	}

	tasks := []Task{
		{ID: "low-priority", Priority: 1},
		{ID: "medium-priority", Priority: 2},
		{ID: "high-priority", Priority: 3},
	}

	// Sort by priority (high to low)
	for i := 0; i < len(tasks)-1; i++ {
		for j := i + 1; j < len(tasks); j++ {
			if tasks[j].Priority > tasks[i].Priority {
				tasks[i], tasks[j] = tasks[j], tasks[i]
			}
		}
	}

	// Add responses for each task
	for _, task := range tasks {
		env.Provider().AddTextResponse("Executed: " + task.ID)
	}

	// Execute in priority order
	executionOrder := make([]string, 0)

	for _, task := range tasks {
		resp, err := env.Provider().CreateCompletion(env.Context(), provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "Execute " + task.ID},
			},
		})
		e2e.AssertNoError(t, err, "task "+task.ID)
		executionOrder = append(executionOrder, task.ID)
		_ = resp
	}

	// Verify high priority executed first
	e2e.AssertEqual(t, "high-priority", executionOrder[0], "high priority should execute first")
}

// TestSupervisor_ResourceAllocation tests resource allocation
func TestSupervisor_ResourceAllocation(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	type Resource struct {
		ID        string
		Capacity  int
		Allocated int
	}

	resources := []*Resource{
		{ID: "gpu-1", Capacity: 100, Allocated: 0},
		{ID: "gpu-2", Capacity: 100, Allocated: 0},
	}

	allocate := func(resourceID string, amount int) bool {
		for _, r := range resources {
			if r.ID == resourceID {
				if r.Allocated+amount <= r.Capacity {
					r.Allocated += amount
					return true
				}
				return false
			}
		}
		return false
	}

	// Allocate resources
	e2e.AssertEqual(t, true, allocate("gpu-1", 50), "should allocate 50 to gpu-1")
	e2e.AssertEqual(t, true, allocate("gpu-1", 50), "should allocate another 50 to gpu-1")
	e2e.AssertEqual(t, false, allocate("gpu-1", 10), "should fail to over-allocate gpu-1")
	e2e.AssertEqual(t, true, allocate("gpu-2", 100), "should allocate 100 to gpu-2")
}

// TestSupervisor_WorkflowExecution tests complete workflow execution
func TestSupervisor_WorkflowExecution(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	workflow := []struct {
		step    string
		timeout time.Duration
	}{
		{step: "init", timeout: 100 * time.Millisecond},
		{step: "plan", timeout: 100 * time.Millisecond},
		{step: "execute", timeout: 100 * time.Millisecond},
		{step: "validate", timeout: 100 * time.Millisecond},
		{step: "complete", timeout: 100 * time.Millisecond},
	}

	for _, step := range workflow {
		env.Provider().AddTextResponse("Step " + step.step + " completed")
	}

	completedSteps := make([]string, 0)

	for _, step := range workflow {
		ctx, cancel := context.WithTimeout(env.Context(), step.timeout)

		done := make(chan bool, 1)
		go func() {
			_, err := env.Provider().CreateCompletion(ctx, provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Execute " + step.step},
				},
			})
			if err == nil {
				done <- true
			}
		}()

		select {
		case <-done:
			completedSteps = append(completedSteps, step.step)
		case <-ctx.Done():
			t.Errorf("step %s timed out", step.step)
		}

		cancel()
	}

	e2e.AssertEqual(t, len(workflow), len(completedSteps), "all steps should complete")
}

// TestSupervisor_MessageBroadcast tests supervisor broadcasting to workers
func TestSupervisor_MessageBroadcast(t *testing.T) {
	env := e2e.NewTestEnvironment(t)
	defer env.Cleanup()

	workerChannels := []string{"worker-1-input", "worker-2-input", "worker-3-input"}

	// Broadcast message to all workers
	broadcastMsg := &agent.Message{
		Message: &pb.Message{
			Id:        "broadcast-1",
			Type:      "broadcast",
			Payload:   "Update your configuration",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}

	for _, ch := range workerChannels {
		err := env.Runtime().Send(ch, broadcastMsg)
		e2e.AssertNoError(t, err, "broadcast to "+ch)
	}

	calls := env.Runtime().GetSendCalls()
	e2e.AssertEqual(t, len(workerChannels), len(calls), "should broadcast to all workers")
}

// hasSubstring checks if s contains substr
func hasSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
