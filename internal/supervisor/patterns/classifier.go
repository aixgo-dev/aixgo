package patterns

import (
	"context"
	"fmt"
	"time"
)

// Classifier determines which agent should handle a task
type Classifier func(ctx context.Context, input string) (agentName string, confidence float64, err error)

// ClassifierConfig configures the classifier pattern
type ClassifierConfig struct {
	Classifier          Classifier    // Function to classify input
	ConfidenceThreshold float64       // Minimum confidence to proceed
	DefaultAgent        string        // Agent to use if classification fails
	Timeout             time.Duration // Timeout per execution
}

// ClassifierPattern routes tasks to agents based on classification
type ClassifierPattern struct {
	config   ClassifierConfig
	executor AgentExecutor
}

// NewClassifierPattern creates a new classifier pattern
func NewClassifierPattern(executor AgentExecutor, config ClassifierConfig) *ClassifierPattern {
	if config.ConfidenceThreshold <= 0 {
		config.ConfidenceThreshold = 0.5
	}
	return &ClassifierPattern{
		config:   config,
		executor: executor,
	}
}

// ClassificationResult contains the classification and execution result
type ClassificationResult struct {
	ClassifiedAgent string
	Confidence      float64
	ExecutionResult
}

// Execute classifies the input and routes to the appropriate agent
func (c *ClassifierPattern) Execute(ctx context.Context, input string) (*ClassificationResult, error) {
	if c.config.Classifier == nil {
		return nil, fmt.Errorf("no classifier configured")
	}

	// Classify the input
	agentName, confidence, err := c.config.Classifier(ctx, input)
	if err != nil {
		if c.config.DefaultAgent == "" {
			return nil, fmt.Errorf("classification failed: %w", err)
		}
		agentName = c.config.DefaultAgent
		confidence = 0.0
	}

	// Check confidence threshold
	if confidence < c.config.ConfidenceThreshold && c.config.DefaultAgent != "" {
		agentName = c.config.DefaultAgent
	}

	// Execute with the selected agent
	execCtx := ctx
	if c.config.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	start := time.Now()
	output, execErr := c.executor(execCtx, agentName, input)

	return &ClassificationResult{
		ClassifiedAgent: agentName,
		Confidence:      confidence,
		ExecutionResult: ExecutionResult{
			AgentName: agentName,
			Output:    output,
			Error:     execErr,
			Duration:  time.Since(start).Milliseconds(),
		},
	}, nil
}

// MultiClassifier determines multiple agents that should handle a task
type MultiClassifier func(ctx context.Context, input string) (agents []AgentClassification, err error)

// AgentClassification represents a classified agent with confidence
type AgentClassification struct {
	AgentName  string
	Confidence float64
	Priority   int
}

// MultiClassifierConfig configures multi-agent classification
type MultiClassifierConfig struct {
	Classifier          MultiClassifier
	ConfidenceThreshold float64
	MaxAgents           int           // Maximum agents to execute
	Timeout             time.Duration // Timeout per agent
	Parallel            bool          // Execute agents in parallel
}

// MultiClassifierPattern routes to multiple agents based on classification
type MultiClassifierPattern struct {
	config   MultiClassifierConfig
	executor AgentExecutor
}

// NewMultiClassifierPattern creates a new multi-classifier pattern
func NewMultiClassifierPattern(executor AgentExecutor, config MultiClassifierConfig) *MultiClassifierPattern {
	if config.ConfidenceThreshold <= 0 {
		config.ConfidenceThreshold = 0.5
	}
	if config.MaxAgents <= 0 {
		config.MaxAgents = 3
	}
	return &MultiClassifierPattern{
		config:   config,
		executor: executor,
	}
}

// Execute classifies and routes to multiple agents
func (m *MultiClassifierPattern) Execute(ctx context.Context, input string) ([]ClassificationResult, error) {
	if m.config.Classifier == nil {
		return nil, fmt.Errorf("no classifier configured")
	}

	// Get classifications
	classifications, err := m.config.Classifier(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("classification failed: %w", err)
	}

	// Filter by confidence and limit
	var selectedAgents []AgentClassification
	for _, c := range classifications {
		if c.Confidence >= m.config.ConfidenceThreshold {
			selectedAgents = append(selectedAgents, c)
			if len(selectedAgents) >= m.config.MaxAgents {
				break
			}
		}
	}

	if len(selectedAgents) == 0 {
		return nil, nil
	}

	if m.config.Parallel {
		return m.executeParallel(ctx, input, selectedAgents)
	}
	return m.executeSequential(ctx, input, selectedAgents)
}

func (m *MultiClassifierPattern) executeSequential(ctx context.Context, input string, agents []AgentClassification) ([]ClassificationResult, error) {
	var results []ClassificationResult
	for _, agent := range agents {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		execCtx := ctx
		if m.config.Timeout > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, m.config.Timeout)
			defer cancel()
		}

		start := time.Now()
		output, err := m.executor(execCtx, agent.AgentName, input)
		results = append(results, ClassificationResult{
			ClassifiedAgent: agent.AgentName,
			Confidence:      agent.Confidence,
			ExecutionResult: ExecutionResult{
				AgentName: agent.AgentName,
				Output:    output,
				Error:     err,
				Duration:  time.Since(start).Milliseconds(),
			},
		})
	}
	return results, nil
}

func (m *MultiClassifierPattern) executeParallel(ctx context.Context, input string, agents []AgentClassification) ([]ClassificationResult, error) {
	resultsCh := make(chan ClassificationResult, len(agents))

	for _, agent := range agents {
		go func(a AgentClassification) {
			execCtx := ctx
			if m.config.Timeout > 0 {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithTimeout(ctx, m.config.Timeout)
				defer cancel()
			}

			start := time.Now()
			output, err := m.executor(execCtx, a.AgentName, input)
			resultsCh <- ClassificationResult{
				ClassifiedAgent: a.AgentName,
				Confidence:      a.Confidence,
				ExecutionResult: ExecutionResult{
					AgentName: a.AgentName,
					Output:    output,
					Error:     err,
					Duration:  time.Since(start).Milliseconds(),
				},
			}
		}(agent)
	}

	var results []ClassificationResult
	for i := 0; i < len(agents); i++ {
		select {
		case r := <-resultsCh:
			results = append(results, r)
		case <-ctx.Done():
			return results, ctx.Err()
		}
	}
	return results, nil
}
