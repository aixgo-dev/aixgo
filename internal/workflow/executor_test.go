package workflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecutor_RegisterWorkflow(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	workflow := &Workflow{
		ID:        "test-workflow",
		Name:      "Test Workflow",
		StartStep: "step-1",
		Steps: map[string]*Step{
			"step-1": {
				ID:   "step-1",
				Name: "First Step",
				Handler: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					return map[string]any{"result": "done"}, nil
				},
			},
		},
	}

	if err := executor.RegisterWorkflow(workflow); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	// Test missing ID
	err := executor.RegisterWorkflow(&Workflow{
		StartStep: "step-1",
		Steps:     map[string]*Step{"step-1": {}},
	})
	if err == nil {
		t.Error("expected error for missing ID")
	}

	// Test missing start step
	err = executor.RegisterWorkflow(&Workflow{
		ID:    "no-start",
		Steps: map[string]*Step{"step-1": {}},
	})
	if err == nil {
		t.Error("expected error for missing start step")
	}

	// Test start step not found
	err = executor.RegisterWorkflow(&Workflow{
		ID:        "bad-start",
		StartStep: "nonexistent",
		Steps:     map[string]*Step{"step-1": {}},
	})
	if err == nil {
		t.Error("expected error for start step not found")
	}
}

func TestExecutor_Execute(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	step1Called := false
	step2Called := false

	workflow := &Workflow{
		ID:        "test-workflow",
		Name:      "Test Workflow",
		StartStep: "step-1",
		Steps: map[string]*Step{
			"step-1": {
				ID:        "step-1",
				Name:      "First Step",
				NextSteps: []string{"step-2"},
				Handler: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					step1Called = true
					return map[string]any{"step1": "done"}, nil
				},
			},
			"step-2": {
				ID:   "step-2",
				Name: "Second Step",
				Handler: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					step2Called = true
					if input["step1"] != "done" {
						return nil, errors.New("step1 output not available")
					}
					return map[string]any{"step2": "complete"}, nil
				},
			},
		},
	}

	if err := executor.RegisterWorkflow(workflow); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	ctx := context.Background()
	state, err := executor.Execute(ctx, "test-workflow", &ExecuteOptions{
		Context: map[string]any{"initial": "value"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !step1Called {
		t.Error("step-1 was not called")
	}
	if !step2Called {
		t.Error("step-2 was not called")
	}

	if state.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", state.Status)
	}

	if state.Context["step2"] != "complete" {
		t.Error("step2 output not in context")
	}
}

func TestExecutor_ExecuteWithRetry(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	attempts := 0
	workflow := &Workflow{
		ID:        "retry-workflow",
		Name:      "Retry Workflow",
		StartStep: "flaky-step",
		Steps: map[string]*Step{
			"flaky-step": {
				ID:      "flaky-step",
				Name:    "Flaky Step",
				Retries: 3,
				Handler: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					attempts++
					if attempts < 3 {
						return nil, errors.New("temporary failure")
					}
					return map[string]any{"success": true}, nil
				},
			},
		},
	}

	if err := executor.RegisterWorkflow(workflow); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	ctx := context.Background()
	state, err := executor.Execute(ctx, "retry-workflow", nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	if state.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", state.Status)
	}
}

func TestExecutor_ExecuteTimeout(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	workflow := &Workflow{
		ID:        "timeout-workflow",
		Name:      "Timeout Workflow",
		StartStep: "slow-step",
		Steps: map[string]*Step{
			"slow-step": {
				ID:      "slow-step",
				Name:    "Slow Step",
				Timeout: 50 * time.Millisecond,
				Handler: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					select {
					case <-time.After(200 * time.Millisecond):
						return map[string]any{"success": true}, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			},
		},
	}

	if err := executor.RegisterWorkflow(workflow); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	ctx := context.Background()
	state, err := executor.Execute(ctx, "timeout-workflow", nil)
	if err == nil {
		t.Error("expected timeout error")
	}

	if state.Status != StatusFailed {
		t.Errorf("expected status failed, got %s", state.Status)
	}
}

func TestExecutor_PauseAndResume(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	workflow := &Workflow{
		ID:        "pausable-workflow",
		Name:      "Pausable Workflow",
		StartStep: "step-1",
		Steps: map[string]*Step{
			"step-1": {
				ID:   "step-1",
				Name: "Only Step",
				Handler: func(ctx context.Context, input map[string]any) (map[string]any, error) {
					return map[string]any{"done": true}, nil
				},
			},
		},
	}

	if err := executor.RegisterWorkflow(workflow); err != nil {
		t.Fatalf("register workflow: %v", err)
	}

	// Create a state manually in paused status
	state := &State{
		ID:          "paused-exec",
		WorkflowID:  "pausable-workflow",
		Status:      StatusPaused,
		CurrentStep: "step-1",
		StepStates:  make(map[string]any),
		Context:     make(map[string]any),
		StartedAt:   time.Now(),
	}
	_ = store.Save(state)

	// Resume
	ctx := context.Background()
	resumedState, err := executor.Resume(ctx, "paused-exec")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	if resumedState.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", resumedState.Status)
	}
}

func TestExecutor_GetState(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	state := &State{
		ID:         "test-exec",
		WorkflowID: "test-workflow",
		Status:     StatusRunning,
		StartedAt:  time.Now(),
	}
	_ = store.Save(state)

	loaded, err := executor.GetState("test-exec")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}

	if loaded.ID != "test-exec" {
		t.Errorf("expected ID test-exec, got %s", loaded.ID)
	}
}

func TestExecutor_ListExecutions(t *testing.T) {
	store := NewMemoryStore()
	executor := NewExecutor(store)

	_ = store.Save(&State{ID: "exec-1", WorkflowID: "workflow-1", StartedAt: time.Now()})
	_ = store.Save(&State{ID: "exec-2", WorkflowID: "workflow-1", StartedAt: time.Now()})
	_ = store.Save(&State{ID: "exec-3", WorkflowID: "workflow-2", StartedAt: time.Now()})

	executions, err := executor.ListExecutions("workflow-1")
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}

	if len(executions) != 2 {
		t.Errorf("expected 2 executions, got %d", len(executions))
	}
}
