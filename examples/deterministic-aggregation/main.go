package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/agents"
	"github.com/aixgo-dev/aixgo/internal/agent"
	pb "github.com/aixgo-dev/aixgo/proto"
	"gopkg.in/yaml.v3"
)

// PolicyRecommendation represents a policy recommendation from an expert
type PolicyRecommendation struct {
	Expert       string  `json:"expert"`
	Recommendation string `json:"recommendation"`
	Confidence   float64 `json:"confidence"`
	Rationale    string  `json:"rationale"`
}

// Config holds the example configuration
type Config struct {
	Scenario      string                 `yaml:"scenario"`
	Experts       []ExpertConfig         `yaml:"experts"`
	Strategies    []string               `yaml:"voting_strategies"`
	OutputFormat  string                 `yaml:"output_format"`
}

// ExpertConfig defines an expert agent
type ExpertConfig struct {
	Name         string  `yaml:"name"`
	Role         string  `yaml:"role"`
	Recommendation string `yaml:"recommendation"`
	Confidence   float64 `yaml:"confidence"`
	Rationale    string  `yaml:"rationale"`
}

func main() {
	fmt.Println("=================================================")
	fmt.Println("Deterministic Aggregation Example")
	fmt.Println("Policy Analysis with Voting Strategies")
	fmt.Println("=================================================\n")

	// Load configuration
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Scenario: %s\n\n", config.Scenario)

	// Display expert recommendations
	fmt.Println("Expert Recommendations:")
	fmt.Println("-------------------------")
	for _, expert := range config.Experts {
		fmt.Printf("%s (%s):\n", expert.Name, expert.Role)
		fmt.Printf("  Recommendation: %s\n", expert.Recommendation)
		fmt.Printf("  Confidence: %.2f\n", expert.Confidence)
		fmt.Printf("  Rationale: %s\n\n", expert.Rationale)
	}

	// Create agent inputs from expert recommendations
	inputs := createAgentInputs(config.Experts)

	// Initialize runtime
	rt := aixgo.NewRuntime()

	// Test each voting strategy
	fmt.Println("\n=================================================")
	fmt.Println("Testing Deterministic Voting Strategies")
	fmt.Println("=================================================\n")

	ctx := context.Background()

	for _, strategy := range config.Strategies {
		testVotingStrategy(ctx, rt, strategy, inputs)
	}

	// Summary comparison
	fmt.Println("\n=================================================")
	fmt.Println("Strategy Comparison Summary")
	fmt.Println("=================================================\n")
	printStrategySummary()
}

func testVotingStrategy(ctx context.Context, rt agent.Runtime, strategy string, inputs []*agents.AgentInput) {
	fmt.Printf("--- %s ---\n", strings.ToUpper(strategy))
	fmt.Printf("Strategy: %s\n", strategy)

	// Create aggregator configuration
	agentDef := createAggregatorDef(strategy)

	// Create aggregator agent
	aggregator, err := agents.NewAggregatorAgent(*agentDef, rt)
	if err != nil {
		log.Printf("Failed to create aggregator: %v\n\n", err)
		return
	}

	// Perform aggregation using internal method via Execute
	// We'll create a message with JSON payload containing all inputs
	inputsJSON, _ := json.Marshal(inputs)
	msg := &agent.Message{
		Message: &pb.Message{
			Type:    "aggregation_request",
			Payload: string(inputsJSON),
		},
	}

	result, err := aggregator.Execute(ctx, msg)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("This is expected for strategies that require specific conditions.\n\n")
		return
	}

	// Parse result
	var aggResult agents.AggregationResult
	if err := json.Unmarshal([]byte(result.Payload), &aggResult); err != nil {
		log.Printf("Failed to parse result: %v\n\n", err)
		return
	}

	// Display results
	fmt.Printf("Selected: %s\n", aggResult.AggregatedContent)
	fmt.Printf("Agreement: %.2f (%.0f%%)\n", aggResult.ConsensusLevel, aggResult.ConsensusLevel*100)
	fmt.Printf("Tokens Used: %d (deterministic - no LLM calls)\n", aggResult.TokensUsed)
	fmt.Printf("Explanation: %s\n", aggResult.SummaryInsights)
	fmt.Printf("Sources: %v\n\n", aggResult.Sources)
}

func createAgentInputs(experts []ExpertConfig) []*agents.AgentInput {
	inputs := make([]*agents.AgentInput, len(experts))
	for i, expert := range experts {
		inputs[i] = &agents.AgentInput{
			AgentName:  expert.Name,
			Content:    expert.Recommendation,
			Confidence: expert.Confidence,
			Metadata: map[string]any{
				"role":      expert.Role,
				"rationale": expert.Rationale,
			},
		}
	}
	return inputs
}

func createAggregatorDef(strategy string) *agent.AgentDef {
	// Create agent definition struct
	return &agent.AgentDef{
		Name:  "policy-aggregator",
		Role:  "aggregator",
		Model: "mock",
		Extra: map[string]any{
			"aggregator_config": map[string]any{
				"aggregation_strategy": strategy,
				"timeout_ms":          5000,
			},
		},
	}
}

func loadConfig(path string) (*Config, error) {
	// G304: Validate file path to prevent path traversal
	// This example uses user-provided paths, so validation is essential
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path traversal detected in config file path")
	}

	data, err := os.ReadFile(cleanPath) //nolint:gosec // Path validated above
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func printStrategySummary() {
	fmt.Println("When to Use Each Strategy:")
	fmt.Println()

	strategies := []struct {
		name        string
		description string
		useCase     string
		pros        string
		cons        string
	}{
		{
			name:        "voting_majority",
			description: "Simple majority vote - most common answer wins",
			useCase:     "Democratic decision-making, crowd wisdom",
			pros:        "Simple, fast, handles disagreement gracefully",
			cons:        "May not reflect expertise levels or confidence",
		},
		{
			name:        "voting_unanimous",
			description: "Requires all agents to agree - fails on any disagreement",
			useCase:     "Critical safety decisions, consensus requirements",
			pros:        "Ensures complete agreement, highest confidence",
			cons:        "Fails if any agent disagrees, very strict",
		},
		{
			name:        "voting_weighted",
			description: "Weights votes by confidence scores",
			useCase:     "Expert panels with varying confidence levels",
			pros:        "Respects confidence levels, nuanced aggregation",
			cons:        "Requires accurate confidence scores",
		},
		{
			name:        "voting_confidence",
			description: "Selects the most confident agent's recommendation",
			useCase:     "Defer to expert judgment, trust-based systems",
			pros:        "Fast, respects expertise, clear decision maker",
			cons:        "Ignores other opinions, single point of failure",
		},
	}

	for _, s := range strategies {
		fmt.Printf("Strategy: %s\n", s.name)
		fmt.Printf("  Description: %s\n", s.description)
		fmt.Printf("  Use Case: %s\n", s.useCase)
		fmt.Printf("  Pros: %s\n", s.pros)
		fmt.Printf("  Cons: %s\n", s.cons)
		fmt.Println()
	}

	fmt.Println("Key Benefits of Deterministic Aggregation:")
	fmt.Println("  - Zero LLM costs - no API calls required")
	fmt.Println("  - Reproducible results - same inputs always produce same output")
	fmt.Println("  - Fast execution - millisecond latency")
	fmt.Println("  - Transparent decision-making - clear voting logic")
	fmt.Println("  - Suitable for production systems requiring auditability")
}
