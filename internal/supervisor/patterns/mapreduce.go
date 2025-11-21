package patterns

import (
	"context"
	"sync"
	"time"
)

// Splitter divides input into chunks for parallel processing
type Splitter func(input string) []string

// Reducer combines results from map phase
type Reducer func(results []ExecutionResult) (string, error)

// MapReduceConfig configures the map-reduce pattern
type MapReduceConfig struct {
	Splitter         Splitter      // Function to split input
	Reducer          Reducer       // Function to reduce results
	ConcurrencyLimit int           // Max concurrent map operations
	Timeout          time.Duration // Timeout per map operation
}

// MapReducePattern implements map-reduce execution
type MapReducePattern struct {
	config   MapReduceConfig
	executor AgentExecutor
}

// NewMapReducePattern creates a new map-reduce pattern
func NewMapReducePattern(executor AgentExecutor, config MapReduceConfig) *MapReducePattern {
	return &MapReducePattern{
		config:   config,
		executor: executor,
	}
}

// Execute runs the map-reduce pattern
func (m *MapReducePattern) Execute(ctx context.Context, agentName, input string) (string, []ExecutionResult, error) {
	// Split phase
	var chunks []string
	if m.config.Splitter != nil {
		chunks = m.config.Splitter(input)
	} else {
		chunks = []string{input}
	}

	if len(chunks) == 0 {
		return "", nil, nil
	}

	// Map phase - parallel execution
	results := m.mapPhase(ctx, agentName, chunks)

	// Reduce phase
	var output string
	var err error
	if m.config.Reducer != nil {
		output, err = m.config.Reducer(results)
	} else {
		output = defaultReduce(results)
	}

	return output, results, err
}

func (m *MapReducePattern) mapPhase(ctx context.Context, agentName string, chunks []string) []ExecutionResult {
	resultsCh := make(chan ExecutionResult, len(chunks))
	var wg sync.WaitGroup

	// Create semaphore for concurrency limit
	var sem chan struct{}
	if m.config.ConcurrencyLimit > 0 {
		sem = make(chan struct{}, m.config.ConcurrencyLimit)
	}

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, data string) {
			defer wg.Done()

			// Acquire semaphore
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					resultsCh <- ExecutionResult{
						AgentName: agentName,
						Error:     ctx.Err(),
					}
					return
				}
			}

			// Create timeout context
			execCtx := ctx
			if m.config.Timeout > 0 {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(ctx, m.config.Timeout)
				defer cancel()
			}

			start := time.Now()
			output, err := m.executor(execCtx, agentName, data)
			resultsCh <- ExecutionResult{
				AgentName: agentName,
				Output:    output,
				Error:     err,
				Duration:  time.Since(start).Milliseconds(),
			}
		}(i, chunk)
	}

	wg.Wait()
	close(resultsCh)

	var results []ExecutionResult
	for r := range resultsCh {
		results = append(results, r)
	}
	return results
}

func defaultReduce(results []ExecutionResult) string {
	var combined string
	for _, r := range results {
		if r.Error == nil && r.Output != "" {
			if combined != "" {
				combined += "\n"
			}
			combined += r.Output
		}
	}
	return combined
}
