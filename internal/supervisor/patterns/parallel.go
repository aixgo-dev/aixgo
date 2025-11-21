package patterns

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ParallelConfig configures parallel execution
type ParallelConfig struct {
	ConcurrencyLimit int                 // Max concurrent executions (0 = unlimited)
	Timeout          time.Duration       // Timeout per agent
	Aggregation      AggregationStrategy // How to aggregate results
}

// ParallelPattern executes multiple agents concurrently
type ParallelPattern struct {
	config   ParallelConfig
	executor AgentExecutor
}

// NewParallelPattern creates a new parallel execution pattern
func NewParallelPattern(executor AgentExecutor, config ParallelConfig) *ParallelPattern {
	if config.Aggregation == "" {
		config.Aggregation = AggregateAll
	}
	return &ParallelPattern{
		config:   config,
		executor: executor,
	}
}

// Execute runs all agents in parallel and aggregates results
func (p *ParallelPattern) Execute(ctx context.Context, agents []string, input string) ([]ExecutionResult, error) {
	if len(agents) == 0 {
		return nil, nil
	}

	resultsCh := make(chan ExecutionResult, len(agents))
	var wg sync.WaitGroup

	// Create semaphore for concurrency limit
	var sem chan struct{}
	if p.config.ConcurrencyLimit > 0 {
		sem = make(chan struct{}, p.config.ConcurrencyLimit)
	}

	// Launch goroutines for each agent
	for _, agentName := range agents {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			// Acquire semaphore if limited
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					resultsCh <- ExecutionResult{
						AgentName: name,
						Error:     ctx.Err(),
					}
					return
				}
			}

			// Create timeout context for this agent
			execCtx := ctx
			if p.config.Timeout > 0 {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(ctx, p.config.Timeout)
				defer cancel()
			}

			start := time.Now()
			output, err := p.executor(execCtx, name, input)
			resultsCh <- ExecutionResult{
				AgentName: name,
				Output:    output,
				Error:     err,
				Duration:  time.Since(start).Milliseconds(),
			}
		}(agentName)
	}

	// Close results channel when all done
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	return p.aggregate(ctx, resultsCh, len(agents))
}

func (p *ParallelPattern) aggregate(ctx context.Context, resultsCh <-chan ExecutionResult, total int) ([]ExecutionResult, error) {
	var results []ExecutionResult
	successCount := 0
	majorityThreshold := (total / 2) + 1

	for {
		select {
		case result, ok := <-resultsCh:
			if !ok {
				// Channel closed, all results received
				return results, nil
			}
			results = append(results, result)
			if result.Error == nil {
				successCount++
			}

			switch p.config.Aggregation {
			case AggregateAny:
				if result.Error == nil {
					return results, nil
				}
			case AggregateMajority:
				if successCount >= majorityThreshold {
					return results, nil
				}
			}

		case <-ctx.Done():
			return results, ctx.Err()
		}
	}
}

// AggregateOutputs combines outputs from all results
func AggregateOutputs(results []ExecutionResult) string {
	var outputs []string
	for _, r := range results {
		if r.Error == nil && r.Output != "" {
			outputs = append(outputs, fmt.Sprintf("[%s]: %s", r.AgentName, r.Output))
		}
	}
	return strings.Join(outputs, "\n")
}
