package patterns

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMapReducePattern_Execute(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return strings.ToUpper(input), nil
	}

	splitter := func(input string) []string {
		return strings.Split(input, ",")
	}

	reducer := func(results []ExecutionResult) (string, error) {
		var outputs []string
		for _, r := range results {
			if r.Error == nil {
				outputs = append(outputs, r.Output)
			}
		}
		return strings.Join(outputs, "|"), nil
	}

	m := NewMapReducePattern(executor, MapReduceConfig{
		Splitter: splitter,
		Reducer:  reducer,
	})

	output, results, err := m.Execute(context.Background(), "agent1", "a,b,c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Output should contain all uppercased values
	if !strings.Contains(output, "A") || !strings.Contains(output, "B") || !strings.Contains(output, "C") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestMapReducePattern_ConcurrencyLimit(t *testing.T) {
	maxConcurrent := int32(0)
	current := int32(0)

	executor := func(ctx context.Context, name, input string) (string, error) {
		c := atomic.AddInt32(&current, 1)
		if c > atomic.LoadInt32(&maxConcurrent) {
			atomic.StoreInt32(&maxConcurrent, c)
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		return input, nil
	}

	splitter := func(input string) []string {
		return []string{"1", "2", "3", "4", "5"}
	}

	m := NewMapReducePattern(executor, MapReduceConfig{
		Splitter:         splitter,
		ConcurrencyLimit: 2,
	})

	_, _, err := m.Execute(context.Background(), "agent1", "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if maxConcurrent > 2 {
		t.Errorf("concurrency limit violated: max was %d, expected <= 2", maxConcurrent)
	}
}

func TestMapReducePattern_Timeout(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		select {
		case <-time.After(1 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	splitter := func(input string) []string {
		return []string{"1", "2"}
	}

	m := NewMapReducePattern(executor, MapReduceConfig{
		Splitter: splitter,
		Timeout:  50 * time.Millisecond,
	})

	_, results, _ := m.Execute(context.Background(), "agent1", "input")

	hasTimeout := false
	for _, r := range results {
		if r.Error != nil {
			hasTimeout = true
			break
		}
	}
	if !hasTimeout {
		t.Error("expected at least one timeout error")
	}
}

func TestMapReducePattern_NoSplitter(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return strings.ToUpper(input), nil
	}

	m := NewMapReducePattern(executor, MapReduceConfig{})

	output, results, err := m.Execute(context.Background(), "agent1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if output != "HELLO" {
		t.Errorf("expected HELLO, got %s", output)
	}
}

func TestMapReducePattern_ReducerError(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return input, nil
	}

	splitter := func(input string) []string {
		return []string{"a", "b"}
	}

	reducer := func(results []ExecutionResult) (string, error) {
		return "", errors.New("reduce error")
	}

	m := NewMapReducePattern(executor, MapReduceConfig{
		Splitter: splitter,
		Reducer:  reducer,
	})

	_, _, err := m.Execute(context.Background(), "agent1", "input")
	if err == nil {
		t.Error("expected reducer error")
	}
}

func TestMapReducePattern_EmptyInput(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return input, nil
	}

	splitter := func(input string) []string {
		return []string{}
	}

	m := NewMapReducePattern(executor, MapReduceConfig{
		Splitter: splitter,
	})

	output, results, err := m.Execute(context.Background(), "agent1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	if output != "" {
		t.Errorf("expected empty output, got %s", output)
	}
}
