package patterns

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPlanningPattern_Execute(t *testing.T) {
	planner := func(ctx context.Context, input string) (*Plan, error) {
		return &Plan{
			Steps: []PlanStep{
				{ID: "s1", AgentName: "agent1", Input: input},
				{ID: "s2", AgentName: "agent2", Input: "{s1}", Dependencies: []string{"s1"}},
			},
		}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output-" + name, nil
	}

	p := NewPlanningPattern(executor, PlanningConfig{
		Planner: planner,
	})

	result, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.StepResults) != 2 {
		t.Errorf("expected 2 step results, got %d", len(result.StepResults))
	}
	if result.FinalOutput != "output-agent2" {
		t.Errorf("expected output-agent2, got %s", result.FinalOutput)
	}
}

func TestPlanningPattern_DependencySubstitution(t *testing.T) {
	planner := func(ctx context.Context, input string) (*Plan, error) {
		return &Plan{
			Steps: []PlanStep{
				{ID: "step1", AgentName: "agent1", Input: input},
				{ID: "step2", AgentName: "agent2", Input: "Previous: {step1}", Dependencies: []string{"step1"}},
			},
		}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		if name == "agent2" {
			if !strings.Contains(input, "output-agent1") {
				t.Errorf("expected substituted output in input, got %s", input)
			}
		}
		return "output-" + name, nil
	}

	p := NewPlanningPattern(executor, PlanningConfig{
		Planner: planner,
	})

	_, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanningPattern_MaxSteps(t *testing.T) {
	planner := func(ctx context.Context, input string) (*Plan, error) {
		steps := make([]PlanStep, 15)
		for i := range steps {
			steps[i] = PlanStep{ID: string(rune('a' + i)), AgentName: "agent"}
		}
		return &Plan{Steps: steps}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	p := NewPlanningPattern(executor, PlanningConfig{
		Planner:  planner,
		MaxSteps: 10,
	})

	_, err := p.Execute(context.Background(), "input")
	if err == nil {
		t.Error("expected error for exceeding max steps")
	}
}

func TestPlanningPattern_ContinueOnError(t *testing.T) {
	planner := func(ctx context.Context, input string) (*Plan, error) {
		return &Plan{
			Steps: []PlanStep{
				{ID: "s1", AgentName: "fail-agent", Input: input},
				{ID: "s2", AgentName: "success-agent", Input: input},
			},
		}, nil
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		if name == "fail-agent" {
			return "", errors.New("agent failed")
		}
		return "success", nil
	}

	p := NewPlanningPattern(executor, PlanningConfig{
		Planner:         planner,
		ContinueOnError: true,
	})

	result, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error with ContinueOnError: %v", err)
	}
	if len(result.StepResults) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.StepResults))
	}
	if result.FinalOutput != "success" {
		t.Errorf("expected success output, got %s", result.FinalOutput)
	}
}

func TestPlanningPattern_Validation(t *testing.T) {
	planner := func(ctx context.Context, input string) (*Plan, error) {
		return &Plan{
			Steps: []PlanStep{{ID: "s1", AgentName: "agent"}},
		}, nil
	}

	validator := func(plan *Plan) error {
		return errors.New("invalid plan")
	}

	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	p := NewPlanningPattern(executor, PlanningConfig{
		Planner:   planner,
		Validator: validator,
	})

	_, err := p.Execute(context.Background(), "input")
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestPlanningPattern_NoPlanner(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "output", nil
	}

	p := NewPlanningPattern(executor, PlanningConfig{})

	_, err := p.Execute(context.Background(), "input")
	if err == nil {
		t.Error("expected error for no planner")
	}
}

func TestChainOfThoughtPattern_Execute(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		if strings.Contains(input, "Step 3") {
			return "FINAL ANSWER: The answer is 42", nil
		}
		return "Thinking about the problem...", nil
	}

	p := NewChainOfThoughtPattern(executor, ChainOfThoughtConfig{
		ReasoningAgent: "reasoner",
		MaxSteps:       5,
	})

	result, err := p.Execute(context.Background(), "What is the meaning of life?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Steps) < 1 {
		t.Error("expected at least 1 step")
	}
	if !strings.Contains(result.FinalAnswer, "42") {
		t.Errorf("expected final answer with 42, got %s", result.FinalAnswer)
	}
}

func TestChainOfThoughtPattern_MaxSteps(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		return "Still thinking...", nil // Never concludes
	}

	p := NewChainOfThoughtPattern(executor, ChainOfThoughtConfig{
		ReasoningAgent: "reasoner",
		MaxSteps:       3,
	})

	result, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Steps) != 3 {
		t.Errorf("expected 3 steps (max), got %d", len(result.Steps))
	}
}

func TestChainOfThoughtPattern_ExecutionAgent(t *testing.T) {
	executor := func(ctx context.Context, name, input string) (string, error) {
		if name == "executor" {
			return "Executed: " + input, nil
		}
		return "Reasoning complete", nil
	}

	p := NewChainOfThoughtPattern(executor, ChainOfThoughtConfig{
		ReasoningAgent: "reasoner",
		ExecutionAgent: "executor",
		MaxSteps:       2,
	})

	result, err := p.Execute(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result.FinalAnswer, "Executed:") {
		t.Errorf("expected execution agent output, got %s", result.FinalAnswer)
	}
}

func TestSubstituteOutput(t *testing.T) {
	input := "Use {step1} and {step2} results"
	result := substituteOutput(input, "step1", "OUTPUT1")
	if result != "Use OUTPUT1 and {step2} results" {
		t.Errorf("unexpected substitution: %s", result)
	}
}
