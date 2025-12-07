package patterns

import (
	"context"
	"time"
)

// QualityAssessor evaluates output quality and determines if improvement is needed
// Returns a score (0.0-1.0) and whether to continue iterating
type QualityAssessor func(ctx context.Context, iteration int, output string) (score float64, continueIterating bool)

// ReflectionConfig configures the reflection pattern
type ReflectionConfig struct {
	MaxIterations      int             // Maximum iterations
	Timeout            time.Duration   // Timeout per iteration
	QualityThreshold   float64         // Stop when quality reaches this level
	ConvergenceWindow  int             // Number of iterations to check for convergence
	ConvergenceEpsilon float64         // Minimum improvement to not be considered converged
	QualityAssessor    QualityAssessor // Function to assess quality
}

// ReflectionResult represents the result of a reflection iteration
type ReflectionResult struct {
	Iteration int
	Output    string
	Score     float64
	Error     error
	Duration  int64
}

// ReflectionPattern implements self-improvement through iterative feedback
type ReflectionPattern struct {
	config   ReflectionConfig
	executor AgentExecutor
}

// NewReflectionPattern creates a new reflection pattern
func NewReflectionPattern(executor AgentExecutor, config ReflectionConfig) *ReflectionPattern {
	if config.MaxIterations <= 0 {
		config.MaxIterations = 5
	}
	if config.QualityThreshold <= 0 {
		config.QualityThreshold = 0.9
	}
	if config.ConvergenceWindow <= 0 {
		config.ConvergenceWindow = 3
	}
	if config.ConvergenceEpsilon <= 0 {
		config.ConvergenceEpsilon = 0.01
	}
	return &ReflectionPattern{
		config:   config,
		executor: executor,
	}
}

// Execute runs the reflection loop
func (r *ReflectionPattern) Execute(ctx context.Context, agentName, input string) ([]ReflectionResult, error) {
	var results []ReflectionResult
	currentInput := input
	var scores []float64

	for i := 0; i < r.config.MaxIterations; i++ {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// Create timeout context
		execCtx := ctx
		if r.config.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, r.config.Timeout)
			defer cancel()
		}

		start := time.Now()
		output, err := r.executor(execCtx, agentName, currentInput)

		result := ReflectionResult{
			Iteration: i + 1,
			Output:    output,
			Error:     err,
			Duration:  time.Since(start).Milliseconds(),
		}

		if err != nil {
			results = append(results, result)
			return results, err
		}

		// Assess quality
		score := 0.0
		continueIterating := true
		if r.config.QualityAssessor != nil {
			score, continueIterating = r.config.QualityAssessor(ctx, i+1, output)
		}
		result.Score = score
		results = append(results, result)
		scores = append(scores, score)

		// Check if quality threshold met
		if score >= r.config.QualityThreshold {
			return results, nil
		}

		// Check if assessor says to stop
		if !continueIterating {
			return results, nil
		}

		// Check for convergence
		if r.hasConverged(scores) {
			return results, nil
		}

		// Feed output back as input for next iteration
		currentInput = output
	}

	return results, nil
}

// hasConverged checks if the scores have converged
func (r *ReflectionPattern) hasConverged(scores []float64) bool {
	if len(scores) < r.config.ConvergenceWindow {
		return false
	}

	// Check if recent scores are within epsilon of each other
	recent := scores[len(scores)-r.config.ConvergenceWindow:]
	min, max := recent[0], recent[0]
	for _, s := range recent {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}

	return (max - min) < r.config.ConvergenceEpsilon
}

// GetBestResult returns the result with the highest score
func GetBestResult(results []ReflectionResult) *ReflectionResult {
	if len(results) == 0 {
		return nil
	}

	best := &results[0]
	for i := range results {
		if results[i].Score > best.Score && results[i].Error == nil {
			best = &results[i]
		}
	}
	return best
}
