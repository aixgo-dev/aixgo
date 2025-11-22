package patterns

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAggregationPattern_Merge(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output-" + name, nil
	}

	p := NewAggregationPattern(executor, AggregationConfig{
		Method:    AggregationMerge,
		Delimiter: "\n",
	})

	result, err := p.Execute(context.Background(), []string{"a1", "a2"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.AggregatedOutput, "output-a1") {
		t.Error("missing a1 output")
	}
	if !strings.Contains(result.AggregatedOutput, "output-a2") {
		t.Error("missing a2 output")
	}
}

func TestAggregationPattern_Vote(t *testing.T) {
	var callCount int32
	executor := func(ctx context.Context, name, input string) (string, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count <= 2 {
			return "yes", nil
		}
		return "no", nil
	}

	p := NewAggregationPattern(executor, AggregationConfig{
		Method: AggregationVote,
	})

	result, err := p.Execute(context.Background(), []string{"a1", "a2", "a3"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.ToLower(result.AggregatedOutput) != "yes" {
		t.Errorf("expected 'yes' (majority), got %s", result.AggregatedOutput)
	}
}

func TestAggregationPattern_Best(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output-" + name, nil
	}

	scorer := func(ctx context.Context, name, output string) float64 {
		if name == "a2" {
			return 0.9
		}
		return 0.5
	}

	p := NewAggregationPattern(executor, AggregationConfig{
		Method: AggregationBest,
		Scorer: scorer,
	})

	result, err := p.Execute(context.Background(), []string{"a1", "a2", "a3"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AggregatedOutput != "output-a2" {
		t.Errorf("expected output-a2 (best score), got %s", result.AggregatedOutput)
	}
	if result.Scores["a2"] != 0.9 {
		t.Errorf("expected score 0.9 for a2, got %f", result.Scores["a2"])
	}
}

func TestAggregationPattern_Summarize(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output-" + name, nil
	}

	summarizer := func(ctx context.Context, outputs []string) (string, error) {
		return "summary: " + strings.Join(outputs, ", "), nil
	}

	p := NewAggregationPattern(executor, AggregationConfig{
		Method:     AggregationSummarize,
		Summarizer: summarizer,
	})

	result, err := p.Execute(context.Background(), []string{"a1", "a2"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result.AggregatedOutput, "summary:") {
		t.Errorf("expected summary, got %s", result.AggregatedOutput)
	}
}

func TestAggregationPattern_MinimumResponses(t *testing.T) {
	var callCount int32
	executor := func(ctx context.Context, name, input string) (string, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			return "", errors.New("agent failed")
		}
		return "output", nil
	}

	p := NewAggregationPattern(executor, AggregationConfig{
		Method:           AggregationMerge,
		MinimumResponses: 3,
	})

	_, err := p.Execute(context.Background(), []string{"a1", "a2", "a3"}, "input")
	if err == nil {
		t.Error("expected error for insufficient responses")
	}
}

func TestAggregationPattern_NoAgents(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	p := NewAggregationPattern(executor, AggregationConfig{})

	_, err := p.Execute(context.Background(), []string{}, "input")
	if err == nil {
		t.Error("expected error for no agents")
	}
}

func TestAggregationPattern_ConcurrencyLimit(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output-" + name, nil
	}

	p := NewAggregationPattern(executor, AggregationConfig{
		ConcurrencyLimit: 2,
	})

	result, err := p.Execute(context.Background(), []string{"a1", "a2", "a3", "a4"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.IndividualResults) != 4 {
		t.Errorf("expected 4 results, got %d", len(result.IndividualResults))
	}
}
