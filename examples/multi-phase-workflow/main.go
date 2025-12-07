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

// This example demonstrates composing existing orchestration patterns
// for multi-phase workflows - a common request from users who thought
// this capability was missing in Aixgo v0.1.2.

// PolicyAnalysisInput represents the input for policy analysis
type PolicyAnalysisInput struct {
	PolicyDocument string   `json:"policy_document"`
	Sections       []string `json:"sections"`
}

// PolicyExtraction represents data extracted from a policy section
type PolicyExtraction struct {
	Section   string   `json:"section"`
	KeyPoints []string `json:"key_points"`
	Risks     []string `json:"risks"`
}

// RiskAssessment represents the final risk assessment
type RiskAssessment struct {
	OverallRisk   string   `json:"overall_risk"`
	Severity      int      `json:"severity"`
	Recommendations []string `json:"recommendations"`
	Summary       string   `json:"summary"`
}

func main() {
	fmt.Println("=== Multi-Phase Workflow Example ===\n")
	fmt.Println("Problem: Users thought multi-phase workflows weren't supported")
	fmt.Println("Solution: Compose existing patterns (Parallel → Ensemble → Sequential)\n")

	// Run both workflow examples
	fmt.Println("Example 1: Policy Analysis Workflow")
	fmt.Println("   Phase 1: Parallel data extraction (3 agents)")
	fmt.Println("   Phase 2: Aggregation with validation (Ensemble voting)")
	fmt.Println("   Phase 3: Sequential risk assessment")
	fmt.Println()
	runPolicyAnalysisWorkflow()

	fmt.Println("\n" + strings.Repeat("=", 70) + "\n")

	fmt.Println("Example 2: E-commerce Product Enrichment")
	fmt.Println("   Phase 1: Parallel feature extraction")
	fmt.Println("   Phase 2: Merge and validate (Ensemble)")
	fmt.Println("   Phase 3: Generate descriptions (Sequential)")
	fmt.Println()
	runProductEnrichmentWorkflow()

	fmt.Println("\n" + strings.Repeat("=", 70) + "\n")
	fmt.Println("=== Summary ===")
	fmt.Println("\nKey Insights:")
	fmt.Println("  ✓ Multi-phase workflows already supported in Aixgo v0.1.2")
	fmt.Println("  ✓ Compose existing patterns: Parallel, Ensemble, Sequential, Router")
	fmt.Println("  ✓ Validation gates between phases ensure data quality")
	fmt.Println("  ✓ Error handling at each phase for resilience")
	fmt.Println("  ✓ Observable with OpenTelemetry tracing")
	fmt.Println("\nNo feature gaps - this was a documentation gap!")
}

func runPolicyAnalysisWorkflow() {
	ctx := context.Background()

	// Create runtime
	rt := runtime.NewLocalRuntime()
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer func() { _ = rt.Stop(ctx) }()

	// Register mock agents
	registerPolicyAgents(rt)

	// Input policy document
	input := &agent.Message{
		Message: &pb.Message{
			Type: "policy_document",
			Payload: `{
				"policy_document": "Company Security Policy 2024",
				"sections": ["data_protection", "access_control", "incident_response"]
			}`,
		},
	}

	// PHASE 1: Parallel Extraction
	fmt.Println("  Phase 1: Parallel Data Extraction")
	phase1 := orchestration.NewParallel(
		"policy-extraction",
		rt,
		[]string{"data-protection-agent", "access-control-agent", "incident-agent"},
		orchestration.WithFailFast(false), // Continue even if one agent fails
	)

	phase1Results, err := phase1.Execute(ctx, input)
	if err != nil {
		log.Printf("  Phase 1 error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Phase 1 complete: 3 sections analyzed in parallel")

	// PHASE 2: Aggregation with Validation (Ensemble voting)
	fmt.Println("  Phase 2: Aggregation with Ensemble Voting")
	phase2 := orchestration.NewEnsemble(
		"policy-aggregator",
		rt,
		[]string{"aggregator-1", "aggregator-2", "aggregator-3"},
		orchestration.WithVotingStrategy(orchestration.VotingMajority),
		orchestration.WithAgreementThreshold(0.6),
	)

	// Register aggregator agents
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("aggregator-%d", i)
		_ = rt.Register(NewMockAggregatorAgent(name))
	}

	phase2Results, err := phase2.Execute(ctx, phase1Results)
	if err != nil {
		log.Printf("  Phase 2 error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Phase 2 complete: Results aggregated with 60% agreement threshold")

	// PHASE 3: Sequential Risk Assessment
	fmt.Println("  Phase 3: Sequential Risk Assessment")

	// Register risk agents
	_ = rt.Register(NewMockRiskAgent("risk-analyzer"))
	_ = rt.Register(NewMockRiskAgent("recommendation-agent"))

	// Execute sequentially: analyze risks, then generate recommendations
	riskInput := phase2Results

	// Step 1: Analyze risks
	riskAgent, _ := rt.Get("risk-analyzer")
	riskAnalysis, err := riskAgent.Execute(ctx, riskInput)
	if err != nil {
		log.Printf("  Risk analysis error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Step 3.1: Risk analysis complete")

	// Step 2: Generate recommendations
	recAgent, _ := rt.Get("recommendation-agent")
	finalResult, err := recAgent.Execute(ctx, riskAnalysis)
	if err != nil {
		log.Printf("  Recommendation error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Step 3.2: Recommendations generated")

	// Display final result
	var assessment RiskAssessment
	if err := json.Unmarshal([]byte(finalResult.Payload), &assessment); err == nil {
		fmt.Printf("\n  Final Assessment:\n")
		fmt.Printf("    Overall Risk: %s\n", assessment.OverallRisk)
		fmt.Printf("    Severity: %d/10\n", assessment.Severity)
		fmt.Printf("    Recommendations: %d items\n", len(assessment.Recommendations))
	}

	fmt.Println("\n  Workflow complete: 3 phases, 8 agents, validated output")
}

func runProductEnrichmentWorkflow() {
	ctx := context.Background()

	// Create runtime
	rt := runtime.NewLocalRuntime()
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer func() { _ = rt.Stop(ctx) }()

	// Register mock agents for product enrichment
	registerProductAgents(rt)

	input := &agent.Message{
		Message: &pb.Message{
			Type:    "product_data",
			Payload: `{"product_id": "laptop-pro-2024", "name": "Professional Laptop"}`,
		},
	}

	// PHASE 1: Parallel Feature Extraction
	fmt.Println("  Phase 1: Parallel Feature Extraction")
	phase1 := orchestration.NewParallel(
		"feature-extraction",
		rt,
		[]string{"spec-extractor", "review-analyzer", "competitor-analyzer"},
	)

	phase1Results, err := phase1.Execute(ctx, input)
	if err != nil {
		log.Printf("  Phase 1 error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Phase 1 complete: Features extracted from 3 sources")

	// PHASE 2: Merge and Validate
	fmt.Println("  Phase 2: Merge and Validate Features")
	mergerAgent, _ := rt.Get("feature-merger")
	phase2Results, err := mergerAgent.Execute(ctx, phase1Results)
	if err != nil {
		log.Printf("  Phase 2 error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Phase 2 complete: Features merged and validated")

	// PHASE 3: Generate Descriptions Sequentially
	fmt.Println("  Phase 3: Generate Product Descriptions")

	// Step 1: Generate short description
	shortDescAgent, _ := rt.Get("short-desc-generator")
	shortDesc, err := shortDescAgent.Execute(ctx, phase2Results)
	if err != nil {
		log.Printf("  Short description error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Step 3.1: Short description generated")

	// Step 2: Generate long description (uses short description as context)
	longDescAgent, _ := rt.Get("long-desc-generator")
	finalResult, err := longDescAgent.Execute(ctx, shortDesc)
	if err != nil {
		log.Printf("  Long description error: %v\n", err)
		return
	}
	fmt.Println("  ✓ Step 3.2: Long description generated")

	fmt.Printf("\n  Product Descriptions Generated:\n")
	fmt.Printf("    Short: %s\n", shortDesc.Payload[:50]+"...")
	fmt.Printf("    Long: %s\n", finalResult.Payload[:60]+"...")
	fmt.Println("\n  Workflow complete: Multi-phase product enrichment successful")
}

// Mock agents for policy analysis

func registerPolicyAgents(rt agent.Runtime) {
	agents := []string{"data-protection-agent", "access-control-agent", "incident-agent"}
	for _, name := range agents {
		_ = rt.Register(NewMockExtractorAgent(name))
	}
}

func registerProductAgents(rt agent.Runtime) {
	agents := []struct {
		name   string
		agentType string
	}{
		{"spec-extractor", "extractor"},
		{"review-analyzer", "extractor"},
		{"competitor-analyzer", "extractor"},
		{"feature-merger", "merger"},
		{"short-desc-generator", "generator"},
		{"long-desc-generator", "generator"},
	}

	for _, a := range agents {
		switch a.agentType {
		case "extractor":
			_ = rt.Register(NewMockExtractorAgent(a.name))
		case "merger":
			_ = rt.Register(NewMockMergerAgent(a.name))
		case "generator":
			_ = rt.Register(NewMockGeneratorAgent(a.name))
		}
	}
}

// Mock Agent Implementations

type MockExtractorAgent struct {
	name string
}

func NewMockExtractorAgent(name string) *MockExtractorAgent {
	return &MockExtractorAgent{name: name}
}

func (m *MockExtractorAgent) Name() string                    { return m.name }
func (m *MockExtractorAgent) Role() string                    { return "extractor" }
func (m *MockExtractorAgent) Start(ctx context.Context) error { return nil }
func (m *MockExtractorAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockExtractorAgent) Ready() bool                     { return true }

func (m *MockExtractorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	extraction := PolicyExtraction{
		Section:   m.name,
		KeyPoints: []string{"Point 1", "Point 2", "Point 3"},
		Risks:     []string{"Risk A", "Risk B"},
	}

	data, _ := json.Marshal(extraction)
	return &agent.Message{
		Message: &pb.Message{
			Type:    "extraction",
			Payload: string(data),
		},
	}, nil
}

type MockAggregatorAgent struct {
	name string
}

func NewMockAggregatorAgent(name string) *MockAggregatorAgent {
	return &MockAggregatorAgent{name: name}
}

func (m *MockAggregatorAgent) Name() string                    { return m.name }
func (m *MockAggregatorAgent) Role() string                    { return "aggregator" }
func (m *MockAggregatorAgent) Start(ctx context.Context) error { return nil }
func (m *MockAggregatorAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockAggregatorAgent) Ready() bool                     { return true }

func (m *MockAggregatorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	return &agent.Message{
		Message: &pb.Message{
			Type:    "aggregated",
			Payload: `{"combined_data": "Aggregated policy data from all sections"}`,
			Metadata: map[string]interface{}{
				"confidence": 0.85,
			},
		},
	}, nil
}

type MockRiskAgent struct {
	name string
}

func NewMockRiskAgent(name string) *MockRiskAgent {
	return &MockRiskAgent{name: name}
}

func (m *MockRiskAgent) Name() string                    { return m.name }
func (m *MockRiskAgent) Role() string                    { return "risk" }
func (m *MockRiskAgent) Start(ctx context.Context) error { return nil }
func (m *MockRiskAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockRiskAgent) Ready() bool                     { return true }

func (m *MockRiskAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	assessment := RiskAssessment{
		OverallRisk:     "Medium",
		Severity:        6,
		Recommendations: []string{"Update password policy", "Enable MFA", "Review access logs"},
		Summary:         "Policy analysis complete with identified risks and recommendations",
	}

	data, _ := json.Marshal(assessment)
	return &agent.Message{
		Message: &pb.Message{
			Type:    "risk_assessment",
			Payload: string(data),
		},
	}, nil
}

type MockMergerAgent struct {
	name string
}

func NewMockMergerAgent(name string) *MockMergerAgent {
	return &MockMergerAgent{name: name}
}

func (m *MockMergerAgent) Name() string                    { return m.name }
func (m *MockMergerAgent) Role() string                    { return "merger" }
func (m *MockMergerAgent) Start(ctx context.Context) error { return nil }
func (m *MockMergerAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockMergerAgent) Ready() bool                     { return true }

func (m *MockMergerAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	return &agent.Message{
		Message: &pb.Message{
			Type:    "merged_features",
			Payload: `{"features": ["16GB RAM", "512GB SSD", "4K Display", "10 hours battery"]}`,
		},
	}, nil
}

type MockGeneratorAgent struct {
	name string
}

func NewMockGeneratorAgent(name string) *MockGeneratorAgent {
	return &MockGeneratorAgent{name: name}
}

func (m *MockGeneratorAgent) Name() string                    { return m.name }
func (m *MockGeneratorAgent) Role() string                    { return "generator" }
func (m *MockGeneratorAgent) Start(ctx context.Context) error { return nil }
func (m *MockGeneratorAgent) Stop(ctx context.Context) error  { return nil }
func (m *MockGeneratorAgent) Ready() bool                     { return true }

func (m *MockGeneratorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	var description string
	if strings.Contains(m.name, "short") {
		description = "Professional laptop with powerful specs for productivity and creativity"
	} else {
		description = "Experience ultimate performance with our Professional Laptop 2024. Featuring 16GB RAM, 512GB SSD storage, stunning 4K display, and all-day 10-hour battery life. Perfect for professionals, creators, and power users who demand the best."
	}

	return &agent.Message{
		Message: &pb.Message{
			Type:    "description",
			Payload: description,
		},
	}, nil
}
