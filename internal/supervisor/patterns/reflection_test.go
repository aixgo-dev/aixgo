package patterns

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReflectionPattern_Execute(t *testing.T) {
	iteration := 0
	executor := func(ctx context.Context, name, input string) (string, error) {
		iteration++
		return input + "-improved", nil
	}

	assessor := func(ctx context.Context, iter int, output string) (float64, bool) {
		// Improve score each iteration
		return float64(iter) * 0.3, iter < 3
	}

	r := NewReflectionPattern(executor, ReflectionConfig{
		MaxIterations:    5,
		QualityThreshold: 0.9,
		QualityAssessor:  assessor,
	})

	results, err := r.Execute(context.Background(), "agent1", "start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 iterations (assessor stops at 3), got %d", len(results))
	}
}

func TestReflectionPattern_QualityThreshold(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	iteration := 0
	assessor := func(ctx context.Context, iter int, output string) (float64, bool) {
		iteration++
		if iteration >= 2 {
			return 0.95, true // Exceeds threshold
		}
		return 0.5, true
	}

	r := NewReflectionPattern(executor, ReflectionConfig{
		MaxIterations:    10,
		QualityThreshold: 0.9,
		QualityAssessor:  assessor,
	})

	results, err := r.Execute(context.Background(), "agent1", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 iterations (threshold met), got %d", len(results))
	}
}

func TestReflectionPattern_MaxIterations(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	assessor := func(ctx context.Context, iter int, output string) (float64, bool) {
		return 0.1, true // Low score, keep iterating
	}

	r := NewReflectionPattern(executor, ReflectionConfig{
		MaxIterations:    3,
		QualityThreshold: 0.9,
		QualityAssessor:  assessor,
	})

	results, err := r.Execute(context.Background(), "agent1", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 iterations (max), got %d", len(results))
	}
}

func TestReflectionPattern_Convergence(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	assessor := func(ctx context.Context, iter int, output string) (float64, bool) {
		return 0.5, true // Same score each time
	}

	r := NewReflectionPattern(executor, ReflectionConfig{
		MaxIterations:      10,
		QualityThreshold:   0.9,
		ConvergenceWindow:  3,
		ConvergenceEpsilon: 0.01,
		QualityAssessor:    assessor,
	})

	results, err := r.Execute(context.Background(), "agent1", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 iterations (convergence), got %d", len(results))
	}
}

func TestReflectionPattern_Error(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "", errors.New("agent error")
	}

	r := NewReflectionPattern(executor, ReflectionConfig{
		MaxIterations: 5,
	})

	results, err := r.Execute(context.Background(), "agent1", "input")
	if err == nil {
		t.Error("expected error")
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestReflectionPattern_Timeout(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		select {
		case <-time.After(1 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	r := NewReflectionPattern(executor, ReflectionConfig{
		MaxIterations: 5,
		Timeout:       50 * time.Millisecond,
	})

	results, err := r.Execute(context.Background(), "agent1", "input")
	if err == nil {
		t.Error("expected timeout error")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestGetBestResult(t *testing.T) {
	results := []ReflectionResult{
		{Iteration: 1, Output: "out1", Score: 0.3, Error: nil},
		{Iteration: 2, Output: "out2", Score: 0.8, Error: nil},
		{Iteration: 3, Output: "out3", Score: 0.5, Error: nil},
	}

	best := GetBestResult(results)
	if best.Iteration != 2 {
		t.Errorf("expected iteration 2, got %d", best.Iteration)
	}
}
