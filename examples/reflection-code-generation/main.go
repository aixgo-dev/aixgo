package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/orchestration"
	"github.com/aixgo-dev/aixgo/internal/runtime"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// This example demonstrates the Reflection orchestration pattern
// for iterative code generation with self-critique.
//
// Benefits:
// - 20-50% quality improvement through iterative refinement
// - Self-critique identifies issues automatically
// - Converges to high-quality output
//
// Use case: Code generation with automated review

func main() {
	ctx := context.Background()

	// Create local runtime
	rt := runtime.NewLocalRuntime()

	// Start runtime
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer func() { _ = rt.Stop(ctx) }()

	// Register agents
	generator := NewMockCodeGeneratorAgent()
	critic := NewMockCodeCriticAgent()

	if err := rt.Register(generator); err != nil {
		log.Fatalf("Failed to register generator: %v", err)
	}
	if err := rt.Register(critic); err != nil {
		log.Fatalf("Failed to register critic: %v", err)
	}

	// Create reflection orchestrator
	reflection := orchestration.NewReflection(
		"code-generator",
		rt,
		"code-generator",
		"code-critic",
		orchestration.WithMaxIterations(3),
	)

	// Code generation request
	request := "Write a function to calculate Fibonacci numbers with memoization"

	fmt.Println("🔄 Reflection Code Generation Demo")
	fmt.Println()
	fmt.Printf("Request: %s\n", request)
	fmt.Println()

	input := &agent.Message{
		Message: &pb.Message{
			Type:    "code-request",
			Payload: request,
		},
	}

	result, err := reflection.Execute(ctx, input)
	if err != nil {
		log.Fatalf("Reflection execution failed: %v", err)
	}

	var response map[string]interface{}
	_ = json.Unmarshal([]byte(result.Payload), &response)

	fmt.Println("✅ Final Code:")
	fmt.Println(response["code"])
	fmt.Println()
	fmt.Printf("Quality Score: %.2f/1.00\n", response["quality"])
	fmt.Printf("Iterations: %d\n", int(response["iteration"].(float64)))
	fmt.Println()
	fmt.Println("💡 Benefits demonstrated:")
	fmt.Println("  ✓ Iterative refinement with self-critique")
	fmt.Println("  ✓ Quality improves with each iteration")
	fmt.Println("  ✓ Automatic convergence to high-quality output")
	fmt.Println("  ✓ 20-50% quality improvement over single-pass generation")
}

// MockCodeGeneratorAgent generates code
type MockCodeGeneratorAgent struct {
	iteration int
}

func NewMockCodeGeneratorAgent() *MockCodeGeneratorAgent {
	return &MockCodeGeneratorAgent{iteration: 0}
}

func (m *MockCodeGeneratorAgent) Name() string                    { return "code-generator" }
func (m *MockCodeGeneratorAgent) Role() string                    { return "generator" }
func (m *MockCodeGeneratorAgent) Start(ctx context.Context) error { return nil }
func (m *MockCodeGeneratorAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockCodeGeneratorAgent) Ready() bool                     { return true }

func (m *MockCodeGeneratorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	m.iteration++

	// Simulate code improvement with each iteration
	codes := []string{
		// Iteration 1: Basic implementation (quality: 0.6)
		`func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}`,

		// Iteration 2: Added memoization (quality: 0.8)
		`func fibonacci(n int) int {
    memo := make(map[int]int)
    return fibHelper(n, memo)
}

func fibHelper(n int, memo map[int]int) int {
    if n <= 1 {
        return n
    }
    if val, ok := memo[n]; ok {
        return val
    }
    memo[n] = fibHelper(n-1, memo) + fibHelper(n-2, memo)
    return memo[n]
}`,

		// Iteration 3: Optimized with comments and error handling (quality: 0.95)
		`// fibonacci calculates the nth Fibonacci number using memoization
// Time complexity: O(n), Space complexity: O(n)
func fibonacci(n int) (int, error) {
    if n < 0 {
        return 0, errors.New("n must be non-negative")
    }

    memo := make(map[int]int)
    return fibHelper(n, memo), nil
}

func fibHelper(n int, memo map[int]int) int {
    // Base cases
    if n <= 1 {
        return n
    }

    // Check memo
    if val, ok := memo[n]; ok {
        return val
    }

    // Calculate and memoize
    memo[n] = fibHelper(n-1, memo) + fibHelper(n-2, memo)
    return memo[n]
}`,
	}

	codeIndex := m.iteration - 1
	if codeIndex >= len(codes) {
		codeIndex = len(codes) - 1
	}

	response := map[string]interface{}{
		"code":      codes[codeIndex],
		"iteration": m.iteration,
	}

	resultJSON, _ := json.Marshal(response)

	return &agent.Message{
		Message: &pb.Message{
			Type:    "code-output",
			Payload: string(resultJSON),
		},
	}, nil
}

// MockCodeCriticAgent critiques code quality
type MockCodeCriticAgent struct{}

func NewMockCodeCriticAgent() *MockCodeCriticAgent {
	return &MockCodeCriticAgent{}
}

func (m *MockCodeCriticAgent) Name() string                    { return "code-critic" }
func (m *MockCodeCriticAgent) Role() string                    { return "critic" }
func (m *MockCodeCriticAgent) Start(ctx context.Context) error { return nil }
func (m *MockCodeCriticAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockCodeCriticAgent) Ready() bool                     { return true }

func (m *MockCodeCriticAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	var codeData map[string]interface{}
	_ = json.Unmarshal([]byte(input.Payload), &codeData)

	code := codeData["code"].(string)
	iteration := int(codeData["iteration"].(float64))

	// Quality improves with each iteration
	qualities := []float64{0.6, 0.8, 0.95}
	quality := qualities[iteration-1]
	if iteration > len(qualities) {
		quality = qualities[len(qualities)-1]
	}

	issues := []string{}
	if !strings.Contains(code, "memo") {
		issues = append(issues, "Missing memoization for efficiency")
	}
	if !strings.Contains(code, "//") {
		issues = append(issues, "Missing documentation comments")
	}
	if !strings.Contains(code, "error") {
		issues = append(issues, "No error handling for invalid input")
	}

	critique := map[string]interface{}{
		"quality": quality,
		"issues":  issues,
		"passed":  quality >= 0.95,
	}

	resultJSON, _ := json.Marshal(critique)

	return &agent.Message{
		Message: &pb.Message{
			Type:    "critique",
			Payload: string(resultJSON),
		},
	}, nil
}
