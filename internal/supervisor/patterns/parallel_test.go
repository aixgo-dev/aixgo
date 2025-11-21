package patterns

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelPattern_ExecuteAll(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "result-" + name, nil
	}

	p := NewParallelPattern(executor, ParallelConfig{
		Aggregation: AggregateAll,
	})

	results, err := p.Execute(context.Background(), []string{"a1", "a2", "a3"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestParallelPattern_ExecuteAny(t *testing.T) {
	callCount := int32(0)
	executor := func(ctx context.Context, name, input string) (string, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(10 * time.Millisecond)
		return "result-" + name, nil
	}

	p := NewParallelPattern(executor, ParallelConfig{
		Aggregation: AggregateAny,
	})

	results, err := p.Execute(context.Background(), []string{"a1", "a2", "a3"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}
}

func TestParallelPattern_ConcurrencyLimit(t *testing.T) {
	maxConcurrent := int32(0)
	current := int32(0)

	executor := func(ctx context.Context, name, input string) (string, error) {
		c := atomic.AddInt32(&current, 1)
		if c > atomic.LoadInt32(&maxConcurrent) {
			atomic.StoreInt32(&maxConcurrent, c)
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		return "done", nil
	}

	p := NewParallelPattern(executor, ParallelConfig{
		ConcurrencyLimit: 2,
		Aggregation:      AggregateAll,
	})

	_, err := p.Execute(context.Background(), []string{"a1", "a2", "a3", "a4"}, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxConcurrent > 2 {
		t.Errorf("concurrency limit violated: max was %d, expected <= 2", maxConcurrent)
	}
}

func TestParallelPattern_Timeout(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		select {
		case <-time.After(1 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	p := NewParallelPattern(executor, ParallelConfig{
		Timeout:     50 * time.Millisecond,
		Aggregation: AggregateAll,
	})

	results, _ := p.Execute(context.Background(), []string{"a1"}, "input")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected timeout error")
	}
}

func TestParallelPattern_ContextCancel(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		select {
		case <-time.After(1 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	p := NewParallelPattern(executor, ParallelConfig{
		Aggregation: AggregateAll,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := p.Execute(ctx, []string{"a1", "a2"}, "input")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestAggregateOutputs(t *testing.T) {
	results := []ExecutionResult{
		{AgentName: "a1", Output: "out1", Error: nil},
		{AgentName: "a2", Output: "out2", Error: nil},
		{AgentName: "a3", Output: "", Error: errors.New("failed")},
	}

	output := AggregateOutputs(results)
	if output == "" {
		t.Error("expected non-empty output")
	}
}
