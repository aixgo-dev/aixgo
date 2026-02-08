package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/orchestration"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// This example demonstrates the Parallel orchestration pattern
// for conducting multi-source market research in parallel.
//
// Benefits:
// - 3-4× speedup vs sequential execution
// - Independent research tasks run concurrently
// - Automatic result aggregation
//
// Use case: Market research requiring data from multiple sources

func main() {
	ctx := context.Background()

	// Create runtime
	rt := aixgo.NewRuntime()

	// Start runtime
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer func() { _ = rt.Stop(ctx) }()

	// Register research agents (in production, these would be real agents)
	// For this example, we'll use mock agents
	competitorAgent := NewMockResearchAgent("competitor-analysis", "Analyzing top 3 competitors in AI agent space...")
	marketSizeAgent := NewMockResearchAgent("market-sizing", "Estimating TAM/SAM/SOM for AI agent market...")
	trendsAgent := NewMockResearchAgent("tech-trends", "Identifying emerging trends in agent orchestration...")
	regulationsAgent := NewMockResearchAgent("regulations", "Reviewing AI regulations and compliance requirements...")

	if err := rt.Register(competitorAgent); err != nil {
		log.Fatalf("Failed to register competitor agent: %v", err)
	}
	if err := rt.Register(marketSizeAgent); err != nil {
		log.Fatalf("Failed to register market size agent: %v", err)
	}
	if err := rt.Register(trendsAgent); err != nil {
		log.Fatalf("Failed to register trends agent: %v", err)
	}
	if err := rt.Register(regulationsAgent); err != nil {
		log.Fatalf("Failed to register regulations agent: %v", err)
	}

	// Create parallel orchestrator
	parallel := orchestration.NewParallel(
		"market-research",
		rt,
		[]string{
			"competitor-analysis",
			"market-sizing",
			"tech-trends",
			"regulations",
		},
	)

	// Create research request
	input := &agent.Message{
		Message: &pb.Message{
			Type:    "research-request",
			Payload: "Conduct comprehensive market research for AI agent orchestration platforms",
		},
	}

	// Execute parallel research
	fmt.Println("🚀 Starting parallel market research...")
	fmt.Println("  → Analyzing competitors")
	fmt.Println("  → Sizing market")
	fmt.Println("  → Identifying trends")
	fmt.Println("  → Reviewing regulations")
	fmt.Println()

	result, err := parallel.Execute(ctx, input)
	if err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	if result == nil {
		log.Fatal("Parallel execution returned nil result")
	}

	// Display results
	fmt.Println("✅ Research complete!")
	fmt.Println()
	fmt.Println("📊 Aggregated Results:")
	if result.Payload != "" {
		fmt.Println(result.Payload)
	} else {
		fmt.Println("(no payload returned)")
	}
	fmt.Println()
	fmt.Println("💡 Benefits demonstrated:")
	fmt.Println("  ✓ 4 research tasks completed concurrently")
	fmt.Println("  ✓ 3-4× faster than sequential execution")
	fmt.Println("  ✓ Automatic result aggregation")
	fmt.Println("  ✓ Continues even if some agents fail")
}

// MockResearchAgent simulates a research agent for the example
type MockResearchAgent struct {
	name   string
	result string
}

func NewMockResearchAgent(name, result string) *MockResearchAgent {
	return &MockResearchAgent{
		name:   name,
		result: result,
	}
}

func (m *MockResearchAgent) Name() string {
	return m.name
}

func (m *MockResearchAgent) Role() string {
	return "researcher"
}

func (m *MockResearchAgent) Start(ctx context.Context) error {
	return nil
}

func (m *MockResearchAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	// Simulate research work
	data := map[string]string{
		"analysis": m.result,
		"source":   m.name,
		"status":   "completed",
	}

	resultJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &agent.Message{
		Message: &pb.Message{
			Type:    "research-result",
			Payload: string(resultJSON),
		},
	}, nil
}

func (m *MockResearchAgent) Stop(ctx context.Context) error {
	return nil
}

func (m *MockResearchAgent) Ready() bool {
	return true
}
