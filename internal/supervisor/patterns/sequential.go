package patterns

import (
	"context"
	"time"
)

// SequentialConfig configures sequential execution
type SequentialConfig struct {
	ErrorStrategy ErrorStrategy       // How to handle errors
	Timeout       time.Duration       // Timeout per agent
	BranchFn      func(string) string // Optional: determine next agent based on output
}

// SequentialPattern executes agents in sequence, passing output as input
type SequentialPattern struct {
	config   SequentialConfig
	executor AgentExecutor
}

// NewSequentialPattern creates a new sequential execution pattern
func NewSequentialPattern(executor AgentExecutor, config SequentialConfig) *SequentialPattern {
	if config.ErrorStrategy == "" {
		config.ErrorStrategy = StopOnError
	}
	return &SequentialPattern{
		config:   config,
		executor: executor,
	}
}

// Execute runs agents in sequence
func (s *SequentialPattern) Execute(ctx context.Context, agents []string, input string) ([]ExecutionResult, error) {
	var results []ExecutionResult
	currentInput := input

	for _, agentName := range agents {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// Create timeout context for this agent
		execCtx := ctx
		if s.config.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, s.config.Timeout)
			defer cancel()
		}

		start := time.Now()
		output, err := s.executor(execCtx, agentName, currentInput)
		result := ExecutionResult{
			AgentName: agentName,
			Output:    output,
			Error:     err,
			Duration:  time.Since(start).Milliseconds(),
		}
		results = append(results, result)

		if err != nil {
			if s.config.ErrorStrategy == StopOnError {
				return results, err
			}
			continue
		}

		// Use output as next input
		currentInput = output

		// Handle conditional branching
		if s.config.BranchFn != nil {
			nextAgent := s.config.BranchFn(output)
			_ = nextAgent // Next agent determined; requires dynamic list modification
		}
	}

	return results, nil
}

// GetFinalOutput returns the output of the last successful execution
func GetFinalOutput(results []ExecutionResult) string {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Error == nil {
			return results[i].Output
		}
	}
	return ""
}
