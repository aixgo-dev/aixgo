package patterns

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// AggregationMethod defines how results are aggregated
type AggregationMethod string

const (
	AggregationMerge     AggregationMethod = "merge"     // Combine all outputs
	AggregationVote      AggregationMethod = "vote"      // Majority voting
	AggregationSummarize AggregationMethod = "summarize" // Summarize via agent
	AggregationBest      AggregationMethod = "best"      // Select best based on scorer
)

// Scorer evaluates the quality of an output
type Scorer func(ctx context.Context, agentName, output string) float64

// Summarizer creates a summary from multiple outputs
type Summarizer func(ctx context.Context, outputs []string) (string, error)

// AggregationConfig configures the aggregation pattern
type AggregationConfig struct {
	Method           AggregationMethod
	Timeout          time.Duration
	ConcurrencyLimit int
	Scorer           Scorer      // For AggregationBest
	Summarizer       Summarizer  // For AggregationSummarize
	Delimiter        string      // For AggregationMerge
	MinimumResponses int         // Minimum responses required
}

// AggregationPattern runs multiple agents and aggregates results
type AggregationPattern struct {
	config   AggregationConfig
	executor AgentExecutor
}

// NewAggregationPattern creates a new aggregation pattern
func NewAggregationPattern(executor AgentExecutor, config AggregationConfig) *AggregationPattern {
	if config.Method == "" {
		config.Method = AggregationMerge
	}
	if config.Delimiter == "" {
		config.Delimiter = "\n---\n"
	}
	if config.MinimumResponses <= 0 {
		config.MinimumResponses = 1
	}
	return &AggregationPattern{
		config:   config,
		executor: executor,
	}
}

// AggregationResult contains execution and aggregation results
type AggregationResult struct {
	IndividualResults []ExecutionResult
	AggregatedOutput  string
	Method            AggregationMethod
	Scores            map[string]float64 // Agent scores if using scorer
}

// Execute runs all agents and aggregates their results
func (a *AggregationPattern) Execute(ctx context.Context, agents []string, input string) (*AggregationResult, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents provided")
	}

	// Execute all agents
	results := a.executeAll(ctx, agents, input)

	// Filter successful results
	var successfulResults []ExecutionResult
	for _, r := range results {
		if r.Error == nil {
			successfulResults = append(successfulResults, r)
		}
	}

	if len(successfulResults) < a.config.MinimumResponses {
		return &AggregationResult{
			IndividualResults: results,
			Method:            a.config.Method,
		}, fmt.Errorf("insufficient responses: got %d, need %d", len(successfulResults), a.config.MinimumResponses)
	}

	// Aggregate based on method
	aggregatedOutput, scores, err := a.aggregate(ctx, successfulResults)
	if err != nil {
		return &AggregationResult{
			IndividualResults: results,
			Method:            a.config.Method,
		}, err
	}

	return &AggregationResult{
		IndividualResults: results,
		AggregatedOutput:  aggregatedOutput,
		Method:            a.config.Method,
		Scores:            scores,
	}, nil
}

func (a *AggregationPattern) executeAll(ctx context.Context, agents []string, input string) []ExecutionResult {
	resultsCh := make(chan ExecutionResult, len(agents))
	var wg sync.WaitGroup

	var sem chan struct{}
	if a.config.ConcurrencyLimit > 0 {
		sem = make(chan struct{}, a.config.ConcurrencyLimit)
	}

	for _, agent := range agents {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					resultsCh <- ExecutionResult{AgentName: name, Error: ctx.Err()}
					return
				}
			}

			execCtx := ctx
			if a.config.Timeout > 0 {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(ctx, a.config.Timeout)
				defer cancel()
			}

			start := time.Now()
			output, err := a.executor(execCtx, name, input)
			resultsCh <- ExecutionResult{
				AgentName: name,
				Output:    output,
				Error:     err,
				Duration:  time.Since(start).Milliseconds(),
			}
		}(agent)
	}

	wg.Wait()
	close(resultsCh)

	var results []ExecutionResult
	for r := range resultsCh {
		results = append(results, r)
	}
	return results
}

func (a *AggregationPattern) aggregate(ctx context.Context, results []ExecutionResult) (string, map[string]float64, error) {
	switch a.config.Method {
	case AggregationMerge:
		return a.aggregateMerge(results), nil, nil
	case AggregationVote:
		return a.aggregateVote(results), nil, nil
	case AggregationSummarize:
		return a.aggregateSummarize(ctx, results)
	case AggregationBest:
		return a.aggregateBest(ctx, results)
	default:
		return a.aggregateMerge(results), nil, nil
	}
}

func (a *AggregationPattern) aggregateMerge(results []ExecutionResult) string {
	var outputs []string
	for _, r := range results {
		if r.Output != "" {
			outputs = append(outputs, fmt.Sprintf("[%s]: %s", r.AgentName, r.Output))
		}
	}
	return strings.Join(outputs, a.config.Delimiter)
}

func (a *AggregationPattern) aggregateVote(results []ExecutionResult) string {
	votes := make(map[string]int)
	for _, r := range results {
		normalized := strings.TrimSpace(strings.ToLower(r.Output))
		votes[normalized]++
	}

	var maxVotes int
	var winner string
	for output, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winner = output
		}
	}

	// Return original output that matches winner
	for _, r := range results {
		if strings.TrimSpace(strings.ToLower(r.Output)) == winner {
			return r.Output
		}
	}
	return winner
}

func (a *AggregationPattern) aggregateSummarize(ctx context.Context, results []ExecutionResult) (string, map[string]float64, error) {
	if a.config.Summarizer == nil {
		return "", nil, fmt.Errorf("summarizer not configured")
	}

	var outputs []string
	for _, r := range results {
		outputs = append(outputs, r.Output)
	}

	summary, err := a.config.Summarizer(ctx, outputs)
	return summary, nil, err
}

func (a *AggregationPattern) aggregateBest(ctx context.Context, results []ExecutionResult) (string, map[string]float64, error) {
	if a.config.Scorer == nil {
		return "", nil, fmt.Errorf("scorer not configured")
	}

	scores := make(map[string]float64)
	type scored struct {
		result ExecutionResult
		score  float64
	}
	var scoredResults []scored

	for _, r := range results {
		score := a.config.Scorer(ctx, r.AgentName, r.Output)
		scores[r.AgentName] = score
		scoredResults = append(scoredResults, scored{result: r, score: score})
	}

	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})

	if len(scoredResults) > 0 {
		return scoredResults[0].result.Output, scores, nil
	}
	return "", scores, nil
}
