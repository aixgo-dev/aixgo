package patterns

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifierPattern_Execute(t *testing.T) {
	classifier := func(ctx context.Context, input string) (string, float64, error) {
		if input == "math" {
			return "math-agent", 0.9, nil
		}
		return "general-agent", 0.6, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result-" + name, nil
	}

	p := NewClassifierPattern(executor, ClassifierConfig{
		Classifier: classifier,
	})

	result, err := p.Execute(context.Background(), "math")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClassifiedAgent != "math-agent" {
		t.Errorf("expected math-agent, got %s", result.ClassifiedAgent)
	}
	if result.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", result.Confidence)
	}
}

func TestClassifierPattern_DefaultAgent(t *testing.T) {
	classifier := func(ctx context.Context, input string) (string, float64, error) {
		return "low-conf", 0.3, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result-" + name, nil
	}

	p := NewClassifierPattern(executor, ClassifierConfig{
		Classifier:          classifier,
		ConfidenceThreshold: 0.5,
		DefaultAgent:        "fallback",
	})

	result, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClassifiedAgent != "fallback" {
		t.Errorf("expected fallback agent, got %s", result.ClassifiedAgent)
	}
}

func TestClassifierPattern_ClassificationError(t *testing.T) {
	classifier := func(ctx context.Context, input string) (string, float64, error) {
		return "", 0, errors.New("classification failed")
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result", nil
	}

	p := NewClassifierPattern(executor, ClassifierConfig{
		Classifier:   classifier,
		DefaultAgent: "fallback",
	})

	result, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClassifiedAgent != "fallback" {
		t.Errorf("expected fallback agent on error, got %s", result.ClassifiedAgent)
	}
}

func TestClassifierPattern_NoClassifier(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result", nil
	}

	p := NewClassifierPattern(executor, ClassifierConfig{})

	_, err := p.Execute(context.Background(), "input")
	if err == nil {
		t.Error("expected error for no classifier")
	}
}

func TestMultiClassifierPattern_Execute(t *testing.T) {
	classifier := func(ctx context.Context, input string) ([]AgentClassification, error) {
		return []AgentClassification{
			{AgentName: "agent1", Confidence: 0.9, Priority: 1},
			{AgentName: "agent2", Confidence: 0.8, Priority: 2},
		}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result-" + name, nil
	}

	p := NewMultiClassifierPattern(executor, MultiClassifierConfig{
		Classifier: classifier,
		MaxAgents:  3,
	})

	results, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestMultiClassifierPattern_Parallel(t *testing.T) {
	classifier := func(ctx context.Context, input string) ([]AgentClassification, error) {
		return []AgentClassification{
			{AgentName: "agent1", Confidence: 0.9},
			{AgentName: "agent2", Confidence: 0.8},
		}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "result-" + name, nil
	}

	p := NewMultiClassifierPattern(executor, MultiClassifierConfig{
		Classifier: classifier,
		Parallel:   true,
	})

	start := time.Now()
	results, err := p.Execute(context.Background(), "input")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	// Parallel should be faster than sequential (2 x 10ms)
	if elapsed > 50*time.Millisecond {
		t.Errorf("parallel execution too slow: %v", elapsed)
	}
}

func TestMultiClassifierPattern_ConfidenceFilter(t *testing.T) {
	classifier := func(ctx context.Context, input string) ([]AgentClassification, error) {
		return []AgentClassification{
			{AgentName: "agent1", Confidence: 0.9},
			{AgentName: "agent2", Confidence: 0.3}, // Below threshold
		}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result-" + name, nil
	}

	p := NewMultiClassifierPattern(executor, MultiClassifierConfig{
		Classifier:          classifier,
		ConfidenceThreshold: 0.5,
	})

	results, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (filtered), got %d", len(results))
	}
}
