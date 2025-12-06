package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	pb "github.com/aixgo-dev/aixgo/proto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Parallel executes multiple agents concurrently and aggregates results.
// Provides 3-4× speedup for independent tasks compared to sequential execution.
//
// Use cases:
// - Multi-source research
// - Batch processing
// - A/B testing
// - Independent data gathering
type Parallel struct {
	*BaseOrchestrator
	agents        []string
	aggregateFunc func(results map[string]*agent.Message) (*agent.Message, error)
	failFast      bool // If true, return error on first failure; otherwise collect all results
}

// ParallelOption configures a Parallel orchestrator
type ParallelOption func(*Parallel)

// WithAggregateFunc sets a custom aggregation function
func WithAggregateFunc(fn func(results map[string]*agent.Message) (*agent.Message, error)) ParallelOption {
	return func(p *Parallel) {
		p.aggregateFunc = fn
	}
}

// WithFailFast enables fail-fast mode (stop on first error)
func WithFailFast(enabled bool) ParallelOption {
	return func(p *Parallel) {
		p.failFast = enabled
	}
}

// NewParallel creates a new Parallel orchestrator
func NewParallel(name string, runtime agent.Runtime, agents []string, opts ...ParallelOption) *Parallel {
	p := &Parallel{
		BaseOrchestrator: NewBaseOrchestrator(name, "parallel", runtime),
		agents:           agents,
		aggregateFunc:    defaultAggregateFunc,
		failFast:         false,
	}

	for _, opt := range opts {
		opt(p)
	}

	p.SetReady(true)
	return p
}

// Execute runs all agents in parallel and aggregates results
func (p *Parallel) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("orchestration.parallel.%s", p.name),
		trace.WithAttributes(
			attribute.String("orchestration.pattern", "parallel"),
			attribute.StringSlice("orchestration.agents", p.agents),
			attribute.Int("orchestration.agent_count", len(p.agents)),
			attribute.Bool("orchestration.fail_fast", p.failFast),
		),
	)
	defer span.End()

	startTime := time.Now()

	// Execute all agents in parallel
	results, errors := p.runtime.CallParallel(ctx, p.agents, input)

	duration := time.Since(startTime)

	// Record metrics
	span.SetAttributes(
		attribute.Int64("orchestration.duration_ms", duration.Milliseconds()),
		attribute.Int("orchestration.success_count", len(results)),
		attribute.Int("orchestration.error_count", len(errors)),
	)

	// Handle errors based on fail-fast mode
	if len(errors) > 0 {
		if p.failFast {
			// Return first error
			for agent, err := range errors {
				span.RecordError(err)
				return nil, fmt.Errorf("agent %s failed: %w", agent, err)
			}
		}

		// Log errors but continue with partial results
		for agentName, err := range errors {
			span.SetAttributes(attribute.String(fmt.Sprintf("error.%s", agentName), err.Error()))
		}
	}

	// If all agents failed, return error
	if len(results) == 0 {
		err := fmt.Errorf("all %d agents failed", len(errors))
		span.RecordError(err)
		return nil, err
	}

	// Aggregate results
	aggregated, err := p.aggregateFunc(results)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("aggregation failed: %w", err)
	}

	span.SetAttributes(attribute.Bool("orchestration.success", true))
	return aggregated, nil
}

// defaultAggregateFunc combines all results into a JSON array
func defaultAggregateFunc(results map[string]*agent.Message) (*agent.Message, error) {
	// Collect all results
	aggregated := make(map[string]interface{})
	for name, msg := range results {
		aggregated[name] = msg.Message
	}

	// Marshal to JSON
	resultJSON, err := json.Marshal(aggregated)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal aggregated results: %w", err)
	}

	return &agent.Message{
		Message: &pb.Message{
			Type:    "aggregated",
			Payload: string(resultJSON),
		},
	}, nil
}

// Common aggregation strategies

// ConcatAggregator concatenates all text results
func ConcatAggregator(separator string) func(results map[string]*agent.Message) (*agent.Message, error) {
	return func(results map[string]*agent.Message) (*agent.Message, error) {
		var combined string
		for _, msg := range results {
			if msg.Message != nil {
				// Extract text content from message
				combined += fmt.Sprintf("%v%s", msg.Payload, separator)
			}
		}

		return &agent.Message{
			Message: &pb.Message{
				Type:    "concatenated",
				Payload: combined,
			},
		}, nil
	}
}

// FirstSuccessAggregator returns the first successful result
func FirstSuccessAggregator() func(results map[string]*agent.Message) (*agent.Message, error) {
	return func(results map[string]*agent.Message) (*agent.Message, error) {
		for _, msg := range results {
			if msg != nil {
				return msg, nil
			}
		}
		return nil, fmt.Errorf("no successful results")
	}
}

// MajorityVoteAggregator returns the most common result
func MajorityVoteAggregator() func(results map[string]*agent.Message) (*agent.Message, error) {
	return func(results map[string]*agent.Message) (*agent.Message, error) {
		// Count occurrences of each result
		counts := make(map[string]int)
		messages := make(map[string]*agent.Message)

		for _, msg := range results {
			key := fmt.Sprintf("%v", msg.Message)
			counts[key]++
			messages[key] = msg
		}

		// Find most common
		var maxCount int
		var maxKey string
		for key, count := range counts {
			if count > maxCount {
				maxCount = count
				maxKey = key
			}
		}

		if maxKey == "" {
			return nil, fmt.Errorf("no results to vote on")
		}

		return messages[maxKey], nil
	}
}
