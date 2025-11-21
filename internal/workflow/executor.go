package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Step represents a single step in a workflow
type Step struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Handler     StepHandler    `json:"-"`
	NextSteps   []string       `json:"next_steps,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Retries     int            `json:"retries,omitempty"`
	Timeout     time.Duration  `json:"timeout,omitempty"`
}

// StepHandler is the function that executes a step
type StepHandler func(ctx context.Context, input map[string]any) (map[string]any, error)

// Workflow defines a workflow with steps
type Workflow struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Steps       map[string]*Step `json:"steps"`
	StartStep   string           `json:"start_step"`
}

// Executor executes workflows with persistence
type Executor struct {
	store     Store
	workflows map[string]*Workflow
	mu        sync.RWMutex
}

// NewExecutor creates a new workflow executor
func NewExecutor(store Store) *Executor {
	return &Executor{
		store:     store,
		workflows: make(map[string]*Workflow),
	}
}

// RegisterWorkflow registers a workflow with the executor
func (e *Executor) RegisterWorkflow(workflow *Workflow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if workflow.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}
	if workflow.StartStep == "" {
		return fmt.Errorf("workflow start step is required")
	}
	if _, exists := workflow.Steps[workflow.StartStep]; !exists {
		return fmt.Errorf("start step %s not found in workflow", workflow.StartStep)
	}

	e.workflows[workflow.ID] = workflow
	return nil
}

// ExecuteOptions configures workflow execution
type ExecuteOptions struct {
	// ResumeFromCheckpoint enables resumption from last checkpoint
	ResumeFromCheckpoint bool
	// Context provides initial workflow context
	Context map[string]any
	// ExecutionID allows specifying a custom execution ID
	ExecutionID string
}

// Execute starts or resumes workflow execution
func (e *Executor) Execute(ctx context.Context, workflowID string, opts *ExecuteOptions) (*State, error) {
	e.mu.RLock()
	workflow, exists := e.workflows[workflowID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	if opts == nil {
		opts = &ExecuteOptions{}
	}

	// Create or load execution state
	var state *State
	var err error

	if opts.ExecutionID != "" && opts.ResumeFromCheckpoint {
		state, err = e.store.Load(opts.ExecutionID)
		if err != nil {
			return nil, fmt.Errorf("load state: %w", err)
		}

		// Load checkpoint if available
		checkpoint, _ := e.store.LoadLatestCheckpoint(opts.ExecutionID)
		if checkpoint != nil {
			state.CurrentStep = checkpoint.StepID
			state.StepStates = checkpoint.State
		}
	} else {
		executionID := opts.ExecutionID
		if executionID == "" {
			executionID = uuid.New().String()
		}

		state = &State{
			ID:          executionID,
			WorkflowID:  workflowID,
			Status:      StatusRunning,
			CurrentStep: workflow.StartStep,
			StepStates:  make(map[string]any),
			Context:     opts.Context,
			StartedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if state.Context == nil {
			state.Context = make(map[string]any)
		}
	}

	// Execute workflow
	state.Status = StatusRunning
	if err := e.store.Save(state); err != nil {
		return nil, fmt.Errorf("save initial state: %w", err)
	}

	// Run execution loop
	if err := e.runLoop(ctx, workflow, state); err != nil {
		state.Status = StatusFailed
		state.Error = err.Error()
		_ = e.store.Save(state)
		return state, err
	}

	return state, nil
}

// runLoop executes the workflow step by step
func (e *Executor) runLoop(ctx context.Context, workflow *Workflow, state *State) error {
	for state.CurrentStep != "" {
		select {
		case <-ctx.Done():
			state.Status = StatusCancelled
			return ctx.Err()
		default:
		}

		step, exists := workflow.Steps[state.CurrentStep]
		if !exists {
			return fmt.Errorf("step not found: %s", state.CurrentStep)
		}

		// Create checkpoint before executing step
		checkpoint := &Checkpoint{
			ID:     uuid.New().String(),
			StepID: state.CurrentStep,
			State:  copyMap(state.StepStates),
		}
		if err := e.store.SaveCheckpoint(state.ID, checkpoint); err != nil {
			return fmt.Errorf("save checkpoint: %w", err)
		}

		// Execute step with optional timeout
		stepCtx := ctx
		var cancel context.CancelFunc
		if step.Timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		}

		output, err := e.executeStep(stepCtx, step, state)
		if cancel != nil {
			cancel()
		}

		if err != nil {
			return fmt.Errorf("step %s failed: %w", step.ID, err)
		}

		// Update state with step output
		state.StepStates[step.ID] = output

		// Merge output into context for next steps
		for k, v := range output {
			state.Context[k] = v
		}

		// Determine next step
		if len(step.NextSteps) > 0 {
			state.CurrentStep = step.NextSteps[0]
		} else {
			state.CurrentStep = ""
		}

		state.UpdatedAt = time.Now()
		if err := e.store.Save(state); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}

	// Workflow completed
	now := time.Now()
	state.Status = StatusCompleted
	state.CompletedAt = &now
	return e.store.Save(state)
}

// executeStep executes a single step with retry logic
func (e *Executor) executeStep(ctx context.Context, step *Step, state *State) (map[string]any, error) {
	if step.Handler == nil {
		return nil, fmt.Errorf("step %s has no handler", step.ID)
	}

	maxRetries := step.Retries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		output, err := step.Handler(ctx, state.Context)
		if err == nil {
			return output, nil
		}
		lastErr = err

		// Wait before retry
		if attempt < maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second * time.Duration(attempt+1)):
			}
		}
	}

	return nil, lastErr
}

// Pause pauses a running workflow
func (e *Executor) Pause(executionID string) error {
	state, err := e.store.Load(executionID)
	if err != nil {
		return err
	}

	if state.Status != StatusRunning {
		return fmt.Errorf("cannot pause workflow in status: %s", state.Status)
	}

	state.Status = StatusPaused
	return e.store.Save(state)
}

// Resume resumes a paused workflow
func (e *Executor) Resume(ctx context.Context, executionID string) (*State, error) {
	state, err := e.store.Load(executionID)
	if err != nil {
		return nil, err
	}

	if state.Status != StatusPaused {
		return nil, fmt.Errorf("cannot resume workflow in status: %s", state.Status)
	}

	return e.Execute(ctx, state.WorkflowID, &ExecuteOptions{
		ExecutionID:          executionID,
		ResumeFromCheckpoint: true,
	})
}

// GetState returns the current state of a workflow execution
func (e *Executor) GetState(executionID string) (*State, error) {
	return e.store.Load(executionID)
}

// ListExecutions returns all executions for a workflow
func (e *Executor) ListExecutions(workflowID string) ([]*State, error) {
	return e.store.List(workflowID)
}

// copyMap creates a deep copy of a map
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}
