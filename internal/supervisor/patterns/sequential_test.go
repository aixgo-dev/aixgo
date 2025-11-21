package patterns

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSequentialPattern_Execute(t *testing.T) {
	calls := []string{}
	executor := func(ctx context.Context, name, input string) (string, error) {
		calls = append(calls, name)
		return input + "-" + name, nil
	}

	s := NewSequentialPattern(executor, SequentialConfig{})

	results, err := s.Execute(context.Background(), []string{"a1", "a2", "a3"}, "start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Verify order
	if calls[0] != "a1" || calls[1] != "a2" || calls[2] != "a3" {
		t.Errorf("agents not called in order: %v", calls)
	}

	// Verify chaining
	if results[2].Output != "start-a1-a2-a3" {
		t.Errorf("expected chained output, got %s", results[2].Output)
	}
}

func TestSequentialPattern_StopOnError(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		if name == "a2" {
			return "", errors.New("agent failed")
		}
		return "ok", nil
	}

	s := NewSequentialPattern(executor, SequentialConfig{
		ErrorStrategy: StopOnError,
	})

	results, err := s.Execute(context.Background(), []string{"a1", "a2", "a3"}, "input")
	if err == nil {
		t.Error("expected error")
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (stopped at a2), got %d", len(results))
	}
}

func TestSequentialPattern_ContinueOnError(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		if name == "a2" {
			return "", errors.New("agent failed")
		}
		return input + "-" + name, nil
	}

	s := NewSequentialPattern(executor, SequentialConfig{
		ErrorStrategy: ContinueOnError,
	})

	results, err := s.Execute(context.Background(), []string{"a1", "a2", "a3"}, "start")
	if err != nil {
		t.Fatalf("should not return error with ContinueOnError: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestSequentialPattern_Timeout(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		select {
		case <-time.After(1 * time.Second):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	s := NewSequentialPattern(executor, SequentialConfig{
		Timeout:       50 * time.Millisecond,
		ErrorStrategy: StopOnError,
	})

	results, err := s.Execute(context.Background(), []string{"a1"}, "input")
	if err == nil {
		t.Error("expected timeout error")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSequentialPattern_ContextCancel(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "done", nil
	}

	s := NewSequentialPattern(executor, SequentialConfig{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := s.Execute(ctx, []string{"a1", "a2"}, "input")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestGetFinalOutput(t *testing.T) {
	results := []ExecutionResult{
		{AgentName: "a1", Output: "out1", Error: nil},
		{AgentName: "a2", Output: "out2", Error: nil},
		{AgentName: "a3", Output: "", Error: errors.New("failed")},
	}

	output := GetFinalOutput(results)
	if output != "out2" {
		t.Errorf("expected out2, got %s", output)
	}
}
