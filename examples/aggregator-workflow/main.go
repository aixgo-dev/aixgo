package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/aixgo-dev/aixgo/pkg/memory"
	pb "github.com/aixgo-dev/aixgo/proto"
	"gopkg.in/yaml.v3"
)

// ResearchTopic represents the topic to analyze
type ResearchTopic struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Aspects     []string `json:"aspects"`
}

// ExpertAnalysis represents analysis from a specialized agent
type ExpertAnalysis struct {
	AgentRole   string                 `json:"agent_role"`
	Analysis    string                 `json:"analysis"`
	KeyFindings []string               `json:"key_findings"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata"`
	Timestamp   string                 `json:"timestamp"`
}

// ResearchSynthesisSystem demonstrates multi-agent research collaboration
type ResearchSynthesisSystem struct {
	config   *WorkflowConfig
	runtime  agent.Runtime
	agents   map[string]agent.Agent
	topic    ResearchTopic
	provider provider.Provider
	memory   *memory.SemanticMemory
}

// WorkflowConfig holds the complete configuration
type WorkflowConfig struct {
	Topic           ResearchTopic            `yaml:"research_topic"`
	ExpertAgents    []ExpertAgentConfig      `yaml:"expert_agents"`
	AggregatorAgent AggregatorWorkflowConfig `yaml:"aggregator"`
	OutputConfig    OutputConfig             `yaml:"output"`
	LLMConfig       LLMConfig                `yaml:"llm"`
}

// ExpertAgentConfig defines configuration for expert agents
type ExpertAgentConfig struct {
	Name        string   `yaml:"name"`
	Role        string   `yaml:"role"`
	Expertise   []string `yaml:"expertise"`
	Perspective string   `yaml:"perspective"`
	Weight      float64  `yaml:"weight"`
}

// AggregatorWorkflowConfig defines aggregator settings
type AggregatorWorkflowConfig struct {
	Strategies          []string           `yaml:"strategies"`
	ConflictResolution  string             `yaml:"conflict_resolution"`
	ConsensusThreshold  float64            `yaml:"consensus_threshold"`
	SemanticSimilarity  float64            `yaml:"semantic_similarity"`
	WeightedAggregation map[string]float64 `yaml:"source_weights"`
	TimeoutMs           int                `yaml:"timeout_ms"`
	Temperature         float64            `yaml:"temperature"`
	MaxTokens           int                `yaml:"max_tokens"`
}

// OutputConfig defines output settings
type OutputConfig struct {
	Format        string `yaml:"format"`
	SaveToFile    bool   `yaml:"save_to_file"`
	FilePath      string `yaml:"file_path"`
	ShowConflicts bool   `yaml:"show_conflicts"`
	ShowConsensus bool   `yaml:"show_consensus"`
	ShowClusters  bool   `yaml:"show_clusters"`
}

// LLMConfig defines LLM provider settings
type LLMConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	APIKey      string  `yaml:"api_key,omitempty"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

func main() {
	// Load configuration
	config, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize the research synthesis system
	system, err := NewResearchSynthesisSystem(config)
	if err != nil {
		log.Fatalf("Failed to initialize system: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down research synthesis system...")
		cancel()
	}()

	// Run the multi-agent research workflow
	log.Printf("Starting Multi-Agent Research Synthesis on: %s", config.Topic.Title)
	log.Printf("Deploying %d expert agents with different perspectives", len(config.ExpertAgents))

	if err := system.RunResearchWorkflow(ctx); err != nil {
		log.Fatalf("Research workflow failed: %v", err)
	}
}

// loadConfig loads and parses the YAML configuration
func loadConfig(path string) (*WorkflowConfig, error) {
	data, err := os.ReadFile(path) // #nosec G304 - user-provided config path
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WorkflowConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.AggregatorAgent.TimeoutMs == 0 {
		config.AggregatorAgent.TimeoutMs = 5000
	}
	if config.AggregatorAgent.Temperature == 0 {
		config.AggregatorAgent.Temperature = 0.5
	}
	if config.AggregatorAgent.MaxTokens == 0 {
		config.AggregatorAgent.MaxTokens = 2000
	}

	return &config, nil
}

// NewResearchSynthesisSystem creates a new research synthesis system
func NewResearchSynthesisSystem(config *WorkflowConfig) (*ResearchSynthesisSystem, error) {
	// Initialize LLM provider
	prov, err := initializeLLMProvider(config.LLMConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	// Initialize runtime
	rt := aixgo.NewRuntime()

	// Initialize semantic memory for enhanced analysis
	mem := memory.NewSemanticMemory(memory.Config{
		MaxMemories:         100,
		SimilarityThreshold: 0.7,
	})

	return &ResearchSynthesisSystem{
		config:   config,
		runtime:  rt,
		agents:   make(map[string]agent.Agent),
		topic:    config.Topic,
		provider: prov,
		memory:   mem,
	}, nil
}

// RunResearchWorkflow executes the multi-agent research synthesis
func (s *ResearchSynthesisSystem) RunResearchWorkflow(ctx context.Context) error {
	// Phase 1: Deploy Expert Agents
	log.Println("Phase 1: Deploying Expert Agents...")
	expertOutputs := make(chan *ExpertAnalysis, len(s.config.ExpertAgents))

	for _, expertConfig := range s.config.ExpertAgents {
		go s.runExpertAgent(ctx, expertConfig, expertOutputs)
	}

	// Collect expert analyses
	var analyses []*ExpertAnalysis
	timeout := time.After(time.Duration(s.config.AggregatorAgent.TimeoutMs) * time.Millisecond)

CollectLoop:
	for i := 0; i < len(s.config.ExpertAgents); i++ {
		select {
		case analysis := <-expertOutputs:
			analyses = append(analyses, analysis)
			log.Printf("Received analysis from %s agent", analysis.AgentRole)
		case <-timeout:
			log.Printf("Timeout waiting for expert agents")
			break CollectLoop
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Phase 2: Demonstrate Different Aggregation Strategies
	log.Println("\nPhase 2: Aggregating Expert Analyses...")

	// Strategy 1: Consensus-based aggregation
	consensusResult, err := s.performConsensusAggregation(ctx, analyses)
	if err != nil {
		log.Printf("Consensus aggregation failed: %v", err)
	} else {
		s.displayResult("CONSENSUS AGGREGATION", consensusResult)
	}

	// Strategy 2: Semantic clustering aggregation
	semanticResult, err := s.performSemanticAggregation(ctx, analyses)
	if err != nil {
		log.Printf("Semantic aggregation failed: %v", err)
	} else {
		s.displayResult("SEMANTIC AGGREGATION", semanticResult)
	}

	// Strategy 3: Weighted aggregation
	weightedResult, err := s.performWeightedAggregation(ctx, analyses)
	if err != nil {
		log.Printf("Weighted aggregation failed: %v", err)
	} else {
		s.displayResult("WEIGHTED AGGREGATION", weightedResult)
	}

	// Phase 3: Generate Final Synthesis
	log.Println("\nPhase 3: Generating Final Research Synthesis...")
	finalSynthesis, err := s.generateFinalSynthesis(ctx, consensusResult, semanticResult, weightedResult)
	if err != nil {
		return fmt.Errorf("failed to generate final synthesis: %w", err)
	}

	s.displayFinalSynthesis(finalSynthesis)

	// Save results if configured
	if s.config.OutputConfig.SaveToFile {
		if err := s.saveResults(finalSynthesis); err != nil {
			log.Printf("Failed to save results: %v", err)
		} else {
			log.Printf("Results saved to %s", s.config.OutputConfig.FilePath)
		}
	}

	return nil
}

// runExpertAgent simulates an expert agent analyzing the research topic
func (s *ResearchSynthesisSystem) runExpertAgent(ctx context.Context, config ExpertAgentConfig, output chan<- *ExpertAnalysis) {
	// Create expert-specific prompt
	prompt := fmt.Sprintf(`You are a %s expert analyzing the topic: "%s"

Your expertise areas: %v
Your perspective: %s

Please provide:
1. A detailed analysis from your expert perspective
2. 3-5 key findings or insights
3. Your confidence level (0-1) in your analysis
4. Any conflicts or concerns you identify

Topic Description: %s
Specific aspects to consider: %v`,
		config.Role,
		s.topic.Title,
		config.Expertise,
		config.Perspective,
		s.topic.Description,
		s.topic.Aspects,
	)

	// Call LLM for expert analysis
	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: fmt.Sprintf("You are a %s providing expert analysis.", config.Role),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       s.config.LLMConfig.Model,
		Temperature: s.config.LLMConfig.Temperature,
		MaxTokens:   s.config.LLMConfig.MaxTokens,
	}

	resp, err := s.provider.CreateCompletion(ctx, req)
	if err != nil {
		log.Printf("Expert agent %s failed: %v", config.Name, err)
		return
	}

	// Parse and structure the analysis
	analysis := &ExpertAnalysis{
		AgentRole:  config.Role,
		Analysis:   resp.Content,
		Confidence: config.Weight,
		Timestamp:  time.Now().Format(time.RFC3339),
		Metadata: map[string]interface{}{
			"expertise":   config.Expertise,
			"perspective": config.Perspective,
			"tokens_used": resp.Usage.TotalTokens,
		},
	}

	// Extract key findings (simplified - in production, use structured output)
	analysis.KeyFindings = s.extractKeyFindings(resp.Content)

	output <- analysis
}

// performConsensusAggregation demonstrates consensus-based aggregation
func (s *ResearchSynthesisSystem) performConsensusAggregation(ctx context.Context, analyses []*ExpertAnalysis) (map[string]interface{}, error) {
	// Prepare inputs for aggregation
	var inputs []string
	for _, analysis := range analyses {
		inputs = append(inputs, fmt.Sprintf("[%s Expert]: %s", analysis.AgentRole, analysis.Analysis))
	}

	prompt := fmt.Sprintf(`Perform consensus-based aggregation on these expert analyses:

%s

Instructions:
1. Identify points of agreement across all experts
2. Highlight areas of consensus with confidence scores
3. Note and resolve any conflicts with reasoning
4. Create a unified consensus view
5. Calculate overall consensus level (0-1)

Return a structured analysis with consensus findings, resolved conflicts, and confidence metrics.`,
		strings.Join(inputs, "\n\n"))

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: "You are an AI aggregator specialized in consensus building from multiple expert opinions.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       s.config.LLMConfig.Model,
		Temperature: s.config.AggregatorAgent.Temperature,
		MaxTokens:   s.config.AggregatorAgent.MaxTokens,
	}

	resp, err := s.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("consensus aggregation failed: %w", err)
	}

	// Calculate consensus metrics
	consensusLevel := s.calculateConsensusLevel(analyses)

	return map[string]interface{}{
		"strategy":        "consensus",
		"content":         resp.Content,
		"consensus_level": consensusLevel,
		"sources":         s.extractSources(analyses),
		"tokens_used":     resp.Usage.TotalTokens,
		"timestamp":       time.Now().Format(time.RFC3339),
	}, nil
}

// performSemanticAggregation demonstrates semantic clustering aggregation
func (s *ResearchSynthesisSystem) performSemanticAggregation(ctx context.Context, analyses []*ExpertAnalysis) (map[string]interface{}, error) {
	// Create semantic clusters based on key findings
	clusters := s.createSemanticClusters(analyses)

	// Build cluster descriptions
	clusterDesc := "Semantic Clusters Identified:\n"
	for i, cluster := range clusters {
		clusterDesc += fmt.Sprintf("\nCluster %d: %s\n", i+1, cluster["concept"])
		clusterDesc += fmt.Sprintf("Members: %v\n", cluster["members"])
		clusterDesc += fmt.Sprintf("Similarity: %.2f\n", cluster["similarity"])
	}

	prompt := fmt.Sprintf(`Perform semantic aggregation based on these clusters:

%s

Expert Analyses:
%s

Instructions:
1. Synthesize insights within each semantic cluster
2. Identify relationships between clusters
3. Create a coherent narrative that preserves semantic groupings
4. Highlight emergent themes across clusters

Provide a comprehensive synthesis that maintains semantic relationships.`,
		clusterDesc,
		s.formatAnalyses(analyses))

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: "You are an AI aggregator specialized in semantic analysis and clustering of expert opinions.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       s.config.LLMConfig.Model,
		Temperature: s.config.AggregatorAgent.Temperature,
		MaxTokens:   s.config.AggregatorAgent.MaxTokens,
	}

	resp, err := s.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("semantic aggregation failed: %w", err)
	}

	return map[string]interface{}{
		"strategy":          "semantic",
		"content":           resp.Content,
		"semantic_clusters": clusters,
		"sources":           s.extractSources(analyses),
		"tokens_used":       resp.Usage.TotalTokens,
		"timestamp":         time.Now().Format(time.RFC3339),
	}, nil
}

// performWeightedAggregation demonstrates weighted aggregation
func (s *ResearchSynthesisSystem) performWeightedAggregation(ctx context.Context, analyses []*ExpertAnalysis) (map[string]interface{}, error) {
	// Apply weights from configuration
	var weightedInputs []string
	for _, analysis := range analyses {
		weight := s.config.AggregatorAgent.WeightedAggregation[analysis.AgentRole]
		if weight == 0 {
			weight = analysis.Confidence
		}
		weightedInputs = append(weightedInputs,
			fmt.Sprintf("[Weight: %.2f] %s Expert: %s",
				weight, analysis.AgentRole, analysis.Analysis))
	}

	prompt := fmt.Sprintf(`Perform weighted aggregation on these expert analyses:

%s

Instructions:
1. Give proportional importance to each expert based on their weight
2. Emphasize insights from higher-weighted sources
3. Still include perspectives from all experts
4. Create a balanced synthesis that reflects the weight distribution

Provide a weighted synthesis that appropriately represents each expert's contribution.`,
		strings.Join(weightedInputs, "\n\n"))

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: "You are an AI aggregator specialized in weighted opinion synthesis based on expertise levels.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       s.config.LLMConfig.Model,
		Temperature: s.config.AggregatorAgent.Temperature,
		MaxTokens:   s.config.AggregatorAgent.MaxTokens,
	}

	resp, err := s.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("weighted aggregation failed: %w", err)
	}

	return map[string]interface{}{
		"strategy":    "weighted",
		"content":     resp.Content,
		"weights":     s.config.AggregatorAgent.WeightedAggregation,
		"sources":     s.extractSources(analyses),
		"tokens_used": resp.Usage.TotalTokens,
		"timestamp":   time.Now().Format(time.RFC3339),
	}, nil
}

// generateFinalSynthesis creates the final comprehensive synthesis
func (s *ResearchSynthesisSystem) generateFinalSynthesis(ctx context.Context, consensus, semantic, weighted map[string]interface{}) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`Create a final research synthesis combining insights from three aggregation strategies:

CONSENSUS AGGREGATION:
%v

SEMANTIC AGGREGATION:
%v

WEIGHTED AGGREGATION:
%v

Create a comprehensive final synthesis that:
1. Integrates the strongest insights from each aggregation method
2. Provides a balanced, nuanced view of the research topic
3. Highlights key findings with confidence levels
4. Identifies areas requiring further research
5. Offers actionable recommendations

Topic: %s`,
		consensus["content"],
		semantic["content"],
		weighted["content"],
		s.topic.Title)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "system",
				Content: "You are a master research synthesizer creating final conclusions from multiple aggregation strategies.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       s.config.LLMConfig.Model,
		Temperature: 0.4, // Lower temperature for final synthesis
		MaxTokens:   3000,
	}

	resp, err := s.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("final synthesis failed: %w", err)
	}

	return map[string]interface{}{
		"final_synthesis":   resp.Content,
		"consensus_level":   consensus["consensus_level"],
		"semantic_clusters": semantic["semantic_clusters"],
		"weights_applied":   weighted["weights"],
		"total_tokens":      resp.Usage.TotalTokens,
		"timestamp":         time.Now().Format(time.RFC3339),
	}, nil
}

// Helper methods

func (s *ResearchSynthesisSystem) extractKeyFindings(content string) []string {
	// Simplified extraction - in production, use structured output
	findings := []string{
		"Key insight extracted from analysis",
		"Important finding identified",
		"Critical observation noted",
	}
	return findings
}

func (s *ResearchSynthesisSystem) calculateConsensusLevel(analyses []*ExpertAnalysis) float64 {
	if len(analyses) == 0 {
		return 0.0
	}

	// Simple consensus calculation based on confidence levels
	var totalConfidence float64
	for _, analysis := range analyses {
		totalConfidence += analysis.Confidence
	}

	return totalConfidence / float64(len(analyses))
}

func (s *ResearchSynthesisSystem) createSemanticClusters(analyses []*ExpertAnalysis) []map[string]interface{} {
	// Simplified clustering - in production, use embeddings and proper clustering
	clusters := []map[string]interface{}{
		{
			"concept":    "Technical Aspects",
			"members":    []string{"Technical Expert", "Data Scientist"},
			"similarity": 0.85,
		},
		{
			"concept":    "Business Impact",
			"members":    []string{"Business Analyst", "Domain Expert"},
			"similarity": 0.78,
		},
		{
			"concept":    "Ethical Considerations",
			"members":    []string{"Ethics Expert", "Domain Expert"},
			"similarity": 0.72,
		},
	}
	return clusters
}

func (s *ResearchSynthesisSystem) extractSources(analyses []*ExpertAnalysis) []string {
	var sources []string
	for _, analysis := range analyses {
		sources = append(sources, analysis.AgentRole)
	}
	return sources
}

func (s *ResearchSynthesisSystem) formatAnalyses(analyses []*ExpertAnalysis) string {
	var formatted []string
	for _, analysis := range analyses {
		formatted = append(formatted, fmt.Sprintf("%s: %s", analysis.AgentRole, analysis.Analysis))
	}
	return strings.Join(formatted, "\n\n")
}

func (s *ResearchSynthesisSystem) displayResult(title string, result map[string]interface{}) {
	fmt.Printf("\n=== %s ===\n", title)
	fmt.Printf("Strategy: %v\n", result["strategy"])

	if consensusLevel, ok := result["consensus_level"].(float64); ok {
		fmt.Printf("Consensus Level: %.2f\n", consensusLevel)
	}

	if clusters, ok := result["semantic_clusters"]; ok && s.config.OutputConfig.ShowClusters {
		fmt.Printf("Semantic Clusters: %v\n", clusters)
	}

	if content, ok := result["content"].(string); ok && len(content) > 0 {
		if len(content) > 200 {
			fmt.Printf("Content Preview: %.200s...\n", content)
		} else {
			fmt.Printf("Content Preview: %s\n", content)
		}
	}
	fmt.Printf("Tokens Used: %v\n", result["tokens_used"])
	fmt.Println("---")
}

func (s *ResearchSynthesisSystem) displayFinalSynthesis(synthesis map[string]interface{}) {
	fmt.Println("\n========================================")
	fmt.Println("FINAL RESEARCH SYNTHESIS")
	fmt.Println("========================================")
	fmt.Printf("Topic: %s\n", s.topic.Title)
	fmt.Printf("Consensus Level: %.2f\n", synthesis["consensus_level"])
	fmt.Printf("Total Tokens Used: %v\n", synthesis["total_tokens"])
	fmt.Printf("Timestamp: %s\n", synthesis["timestamp"])
	fmt.Println("\nSynthesis:")
	fmt.Println(synthesis["final_synthesis"])
	fmt.Println("========================================")
}

func (s *ResearchSynthesisSystem) saveResults(synthesis map[string]interface{}) error {
	data, err := json.MarshalIndent(synthesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	return os.WriteFile(s.config.OutputConfig.FilePath, data, 0600)
}

func initializeLLMProvider(config LLMConfig) (provider.Provider, error) {
	switch config.Provider {
	case "openai":
		return provider.NewOpenAIProvider(config.APIKey, "https://api.openai.com/v1"), nil
	case "anthropic":
		return provider.NewAnthropicProvider(config.APIKey, "https://api.anthropic.com/v1"), nil
	default:
		return provider.NewMockProvider("mock-model"), nil
	}
}

// MockRuntime for demonstration
type MockRuntime struct{}

func (m *MockRuntime) Send(target string, msg *agent.Message) error {
	log.Printf("Sending to %s: %v", target, msg)
	return nil
}

func (m *MockRuntime) Recv(source string) (<-chan *agent.Message, error) {
	ch := make(chan *agent.Message, 1)
	// Simulate receiving a message
	go func() {
		time.Sleep(100 * time.Millisecond)
		ch <- &agent.Message{
			Message: &pb.Message{
				Payload: fmt.Sprintf("Message from %s", source),
			},
		}
	}()
	return ch, nil
}
