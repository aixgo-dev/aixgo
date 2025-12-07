package patterns

import (
	"context"
)

// ExecutionResult represents the result of executing an agent
type ExecutionResult struct {
	AgentName string
	Output    string
	Error     error
	Duration  int64 // milliseconds
}

// AgentExecutor is the function signature for executing an agent
type AgentExecutor func(ctx context.Context, agentName, input string) (string, error)

// AggregationStrategy defines how to aggregate results from parallel execution
type AggregationStrategy string

const (
	AggregateAll      AggregationStrategy = "all"      // Wait for all results
	AggregateAny      AggregationStrategy = "any"      // Return first successful result
	AggregateMajority AggregationStrategy = "majority" // Return when majority complete
)

// ErrorStrategy defines how to handle errors in sequential execution
type ErrorStrategy string

const (
	StopOnError     ErrorStrategy = "stop"     // Stop execution on first error
	ContinueOnError ErrorStrategy = "continue" // Continue despite errors
)
