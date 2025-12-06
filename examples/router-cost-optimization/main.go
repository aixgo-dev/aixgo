package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/orchestration"
	"github.com/aixgo-dev/aixgo/internal/runtime"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// This example demonstrates the Router orchestration pattern
// for cost optimization by routing queries to appropriate models.
//
// Benefits:
// - 25-50% cost reduction in production
// - Simple queries → cheap models (gpt-3.5, claude-haiku)
// - Complex queries → expensive models (gpt-4, claude-opus)
//
// Use case: Customer support chatbot with varying complexity

func main() {
	ctx := context.Background()

	// Create local runtime
	rt := runtime.NewLocalRuntime()

	// Start runtime
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer rt.Stop(ctx)

	// Register agents
	classifier := NewMockClassifierAgent()
	cheapAgent := NewMockLLMAgent("cheap-model", "gpt-3.5-turbo", 0.002) // $0.002 per query
	expensiveAgent := NewMockLLMAgent("expensive-model", "gpt-4", 0.03)  // $0.03 per query

	if err := rt.Register(classifier); err != nil {
		log.Fatalf("Failed to register classifier: %v", err)
	}
	if err := rt.Register(cheapAgent); err != nil {
		log.Fatalf("Failed to register cheap agent: %v", err)
	}
	if err := rt.Register(expensiveAgent); err != nil {
		log.Fatalf("Failed to register expensive agent: %v", err)
	}

	// Create router orchestrator
	router := orchestration.NewRouter(
		"cost-optimizer",
		rt,
		"complexity-classifier",
		map[string]string{
			"simple":  "cheap-model",
			"complex": "expensive-model",
		},
		orchestration.WithDefaultRoute("cheap-model"),
	)

	// Test queries
	queries := []struct {
		text       string
		complexity string
	}{
		{"What are your business hours?", "simple"},
		{"How do I reset my password?", "simple"},
		{"Explain the technical architecture of your distributed agent orchestration system", "complex"},
		{"What is your refund policy?", "simple"},
	}

	totalCost := 0.0
	totalCostWithoutRouter := 0.0

	fmt.Println("🎯 Router Cost Optimization Demo")
	fmt.Println()

	for i, query := range queries {
		fmt.Printf("Query %d: \"%s\"\n", i+1, query.text)

		input := &agent.Message{
			Message: &pb.Message{
				Type:    "query",
				Payload: query.text,
			},
		}

		result, err := router.Execute(ctx, input)
		if err != nil {
			log.Fatalf("Router execution failed: %v", err)
		}

		var response map[string]interface{}
		json.Unmarshal([]byte(result.Message.Payload), &response)

		modelUsed := response["model"].(string)
		cost := response["cost"].(float64)

		totalCost += cost
		totalCostWithoutRouter += 0.03 // Always using expensive model

		fmt.Printf("  → Routed to: %s\n", modelUsed)
		fmt.Printf("  → Cost: $%.4f\n", cost)
		fmt.Println()
	}

	savings := ((totalCostWithoutRouter - totalCost) / totalCostWithoutRouter) * 100

	fmt.Println("💰 Cost Summary:")
	fmt.Printf("  Total cost with router: $%.4f\n", totalCost)
	fmt.Printf("  Total cost without router: $%.4f\n", totalCostWithoutRouter)
	fmt.Printf("  Savings: %.1f%%\n", savings)
	fmt.Println()
	fmt.Println("💡 Benefits demonstrated:")
	fmt.Println("  ✓ Automatic complexity classification")
	fmt.Println("  ✓ Intelligent routing to cost-appropriate models")
	fmt.Println("  ✓ 25-50% cost reduction in production")
	fmt.Println("  ✓ Maintains quality for complex queries")
}

// MockClassifierAgent classifies query complexity
type MockClassifierAgent struct{}

func NewMockClassifierAgent() *MockClassifierAgent {
	return &MockClassifierAgent{}
}

func (m *MockClassifierAgent) Name() string                                             { return "complexity-classifier" }
func (m *MockClassifierAgent) Role() string                                             { return "classifier" }
func (m *MockClassifierAgent) Start(ctx context.Context) error                          { return nil }
func (m *MockClassifierAgent) Stop(ctx context.Context) error                           { return nil }
func (m *MockClassifierAgent) Ready() bool                                              { return true }

func (m *MockClassifierAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	query := input.Message.Payload

	// Simple heuristic: long queries or technical terms = complex
	complexity := "simple"
	if len(query) > 80 || containsTechnicalTerms(query) {
		complexity = "complex"
	}

	return &agent.Message{
		Message: &pb.Message{
			Type:    "classification",
			Payload: complexity,
		},
	}, nil
}

func containsTechnicalTerms(text string) bool {
	terms := []string{"architecture", "distributed", "technical", "system", "implementation"}
	for _, term := range terms {
		if contains(text, term) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MockLLMAgent simulates an LLM agent with cost tracking
type MockLLMAgent struct {
	name  string
	model string
	cost  float64
}

func NewMockLLMAgent(name, model string, cost float64) *MockLLMAgent {
	return &MockLLMAgent{
		name:  name,
		model: model,
		cost:  cost,
	}
}

func (m *MockLLMAgent) Name() string                                             { return m.name }
func (m *MockLLMAgent) Role() string                                             { return "llm" }
func (m *MockLLMAgent) Start(ctx context.Context) error                          { return nil }
func (m *MockLLMAgent) Stop(ctx context.Context) error                           { return nil }
func (m *MockLLMAgent) Ready() bool                                              { return true }

func (m *MockLLMAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	response := map[string]interface{}{
		"answer": "Mock response from " + m.model,
		"model":  m.model,
		"cost":   m.cost,
	}

	resultJSON, _ := json.Marshal(response)

	return &agent.Message{
		Message: &pb.Message{
			Type:    "response",
			Payload: string(resultJSON),
		},
	}, nil
}
