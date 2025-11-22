package patterns

import (
	"context"
	"fmt"
	"time"
)

// PlanStep represents a single step in an execution plan
type PlanStep struct {
	ID           string            // Unique step identifier
	AgentName    string            // Agent to execute
	Input        string            // Input for this step
	Dependencies []string          // IDs of steps that must complete first
	Metadata     map[string]string // Optional metadata
}

// Plan represents an execution plan
type Plan struct {
	Steps       []PlanStep
	Description string
	Metadata    map[string]string
}

// Planner creates an execution plan from input
type Planner func(ctx context.Context, input string) (*Plan, error)

// PlanValidator validates a plan before execution
type PlanValidator func(plan *Plan) error

// PlanningConfig configures the planning pattern
type PlanningConfig struct {
	Planner           Planner
	Validator         PlanValidator
	Timeout           time.Duration // Timeout per step
	MaxSteps          int           // Maximum steps allowed
	ContinueOnError   bool          // Continue executing if a step fails
	ReplanOnFailure   bool          // Attempt to create new plan on failure
	MaxReplanAttempts int
}

// PlanningPattern creates and executes plans
type PlanningPattern struct {
	config   PlanningConfig
	executor AgentExecutor
}

// NewPlanningPattern creates a new planning pattern
func NewPlanningPattern(executor AgentExecutor, config PlanningConfig) *PlanningPattern {
	if config.MaxSteps <= 0 {
		config.MaxSteps = 10
	}
	if config.MaxReplanAttempts <= 0 {
		config.MaxReplanAttempts = 2
	}
	return &PlanningPattern{
		config:   config,
		executor: executor,
	}
}

// StepResult represents the result of a plan step
type StepResult struct {
	StepID    string
	AgentName string
	Output    string
	Error     error
	Duration  int64
	Skipped   bool
}

// PlanningResult contains the full planning and execution result
type PlanningResult struct {
	Plan          *Plan
	StepResults   []StepResult
	FinalOutput   string
	ReplanCount   int
	TotalDuration int64
}

// Execute creates a plan and executes it
func (p *PlanningPattern) Execute(ctx context.Context, input string) (*PlanningResult, error) {
	if p.config.Planner == nil {
		return nil, fmt.Errorf("no planner configured")
	}

	start := time.Now()
	result := &PlanningResult{}

	for attempt := 0; attempt <= p.config.MaxReplanAttempts; attempt++ {
		if attempt > 0 {
			result.ReplanCount++
		}

		// Create plan
		plan, err := p.config.Planner(ctx, input)
		if err != nil {
			if !p.config.ReplanOnFailure || attempt >= p.config.MaxReplanAttempts {
				return nil, fmt.Errorf("planning failed: %w", err)
			}
			continue
		}

		// Validate plan
		if p.config.Validator != nil {
			if err := p.config.Validator(plan); err != nil {
				if !p.config.ReplanOnFailure || attempt >= p.config.MaxReplanAttempts {
					return nil, fmt.Errorf("plan validation failed: %w", err)
				}
				continue
			}
		}

		// Check max steps
		if len(plan.Steps) > p.config.MaxSteps {
			return nil, fmt.Errorf("plan exceeds maximum steps: %d > %d", len(plan.Steps), p.config.MaxSteps)
		}

		result.Plan = plan

		// Execute plan
		stepResults, execErr := p.executePlan(ctx, plan)
		result.StepResults = stepResults

		if execErr != nil && p.config.ReplanOnFailure && attempt < p.config.MaxReplanAttempts {
			// Update input with failure context for replanning
			input = fmt.Sprintf("%s\n\nPrevious execution failed: %v", input, execErr)
			continue
		}

		// Get final output from last successful step
		for i := len(stepResults) - 1; i >= 0; i-- {
			if stepResults[i].Error == nil && !stepResults[i].Skipped {
				result.FinalOutput = stepResults[i].Output
				break
			}
		}

		result.TotalDuration = time.Since(start).Milliseconds()
		return result, execErr
	}

	result.TotalDuration = time.Since(start).Milliseconds()
	return result, fmt.Errorf("exceeded maximum replan attempts")
}

func (p *PlanningPattern) executePlan(ctx context.Context, plan *Plan) ([]StepResult, error) {
	var results []StepResult
	completedSteps := make(map[string]string) // stepID -> output

	for _, step := range plan.Steps {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// Check dependencies
		dependenciesMet := true
		for _, depID := range step.Dependencies {
			if _, ok := completedSteps[depID]; !ok {
				dependenciesMet = false
				break
			}
		}

		if !dependenciesMet {
			results = append(results, StepResult{
				StepID:    step.ID,
				AgentName: step.AgentName,
				Skipped:   true,
				Error:     fmt.Errorf("dependencies not met"),
			})
			if !p.config.ContinueOnError {
				return results, fmt.Errorf("step %s skipped: dependencies not met", step.ID)
			}
			continue
		}

		// Prepare input - substitute dependency outputs
		stepInput := step.Input
		for depID, output := range completedSteps {
			stepInput = substituteOutput(stepInput, depID, output)
		}

		// Execute step
		execCtx := ctx
		if p.config.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, p.config.Timeout)
			defer cancel()
		}

		start := time.Now()
		output, err := p.executor(execCtx, step.AgentName, stepInput)

		result := StepResult{
			StepID:    step.ID,
			AgentName: step.AgentName,
			Output:    output,
			Error:     err,
			Duration:  time.Since(start).Milliseconds(),
		}
		results = append(results, result)

		if err != nil {
			if !p.config.ContinueOnError {
				return results, err
			}
		} else {
			completedSteps[step.ID] = output
		}
	}

	return results, nil
}

// substituteOutput replaces {stepID} placeholders with actual outputs
func substituteOutput(input, stepID, output string) string {
	placeholder := fmt.Sprintf("{%s}", stepID)
	for i := 0; i < 10; i++ { // Limit iterations to prevent infinite loops
		newInput := replaceFirst(input, placeholder, output)
		if newInput == input {
			break
		}
		input = newInput
	}
	return input
}

func replaceFirst(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

// ChainOfThoughtConfig configures chain-of-thought reasoning
type ChainOfThoughtConfig struct {
	ReasoningAgent string        // Agent for reasoning steps
	ExecutionAgent string        // Agent for final execution
	MaxSteps       int           // Maximum reasoning steps
	Timeout        time.Duration // Timeout per step
}

// ChainOfThoughtPattern implements chain-of-thought reasoning
type ChainOfThoughtPattern struct {
	config   ChainOfThoughtConfig
	executor AgentExecutor
}

// NewChainOfThoughtPattern creates a new chain-of-thought pattern
func NewChainOfThoughtPattern(executor AgentExecutor, config ChainOfThoughtConfig) *ChainOfThoughtPattern {
	if config.MaxSteps <= 0 {
		config.MaxSteps = 5
	}
	return &ChainOfThoughtPattern{
		config:   config,
		executor: executor,
	}
}

// ThoughtStep represents a step in chain-of-thought reasoning
type ThoughtStep struct {
	Step     int
	Thought  string
	Action   string
	Result   string
	Error    error
	Duration int64
}

// ChainOfThoughtResult contains the full reasoning chain
type ChainOfThoughtResult struct {
	Steps       []ThoughtStep
	FinalAnswer string
	Error       error
}

// Execute runs chain-of-thought reasoning
func (c *ChainOfThoughtPattern) Execute(ctx context.Context, input string) (*ChainOfThoughtResult, error) {
	result := &ChainOfThoughtResult{}
	currentContext := input

	for i := 0; i < c.config.MaxSteps; i++ {
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}

		execCtx := ctx
		if c.config.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
			defer cancel()
		}

		// Get reasoning step
		prompt := fmt.Sprintf("Step %d reasoning for: %s\n\nProvide your thought process and next action.", i+1, currentContext)
		start := time.Now()
		thought, err := c.executor(execCtx, c.config.ReasoningAgent, prompt)

		step := ThoughtStep{
			Step:     i + 1,
			Thought:  thought,
			Duration: time.Since(start).Milliseconds(),
			Error:    err,
		}

		if err != nil {
			step.Error = err
			result.Steps = append(result.Steps, step)
			result.Error = err
			return result, err
		}

		// Check if reasoning indicates completion
		if containsFinalAnswer(thought) {
			step.Action = "conclude"
			result.Steps = append(result.Steps, step)
			result.FinalAnswer = extractFinalAnswer(thought)
			return result, nil
		}

		step.Action = "continue"
		result.Steps = append(result.Steps, step)
		currentContext = thought
	}

	// Execute final step if we have an execution agent
	if c.config.ExecutionAgent != "" {
		execCtx := ctx
		if c.config.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
			defer cancel()
		}

		finalAnswer, err := c.executor(execCtx, c.config.ExecutionAgent, currentContext)
		result.FinalAnswer = finalAnswer
		result.Error = err
		return result, err
	}

	result.FinalAnswer = currentContext
	return result, nil
}

func containsFinalAnswer(thought string) bool {
	markers := []string{"FINAL ANSWER:", "Final Answer:", "CONCLUSION:", "Conclusion:"}
	for _, marker := range markers {
		if containsSubstring(thought, marker) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func extractFinalAnswer(thought string) string {
	markers := []string{"FINAL ANSWER:", "Final Answer:", "CONCLUSION:", "Conclusion:"}
	for _, marker := range markers {
		idx := findSubstring(thought, marker)
		if idx >= 0 {
			return thought[idx+len(marker):]
		}
	}
	return thought
}
