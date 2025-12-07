package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/aggregation"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/security"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// AggregatorConfig holds AI-specific configuration for aggregation
type AggregatorConfig struct {
	AggregationStrategy  string             `yaml:"aggregation_strategy"`
	ConflictResolution   string             `yaml:"conflict_resolution"`
	DeduplicationMethod  string             `yaml:"deduplication_method"`
	SummarizationEnabled bool               `yaml:"summarization_enabled"`
	MaxInputSources      int                `yaml:"max_input_sources"`
	TimeoutMs            int                `yaml:"timeout_ms"`
	SemanticSimilarity   float64            `yaml:"semantic_similarity_threshold"`
	WeightedAggregation  map[string]float64 `yaml:"source_weights"`
	ConsensusThreshold   float64            `yaml:"consensus_threshold"`
	Temperature          float64            `yaml:"temperature"`
	MaxTokens            int                `yaml:"max_tokens"`
}

// AgentInput represents input from a single agent
type AgentInput struct {
	AgentName  string         `json:"agent_name"`
	Content    string         `json:"content"`
	Timestamp  time.Time      `json:"timestamp"`
	Confidence float64        `json:"confidence,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Embedding  []float64      `json:"embedding,omitempty"`
}

// AggregationResult with AI-enhanced insights
type AggregationResult struct {
	AggregatedContent string               `json:"aggregated_content"`
	Sources           []string             `json:"sources"`
	Strategy          string               `json:"strategy_used"`
	ConflictsSolved   []ConflictResolution `json:"conflicts_resolved,omitempty"`
	ConsensusLevel    float64              `json:"consensus_level"`
	SummaryInsights   string               `json:"summary_insights,omitempty"`
	TokensUsed        int                  `json:"tokens_used"`
	ProcessingTimeMs  int64                `json:"processing_time_ms"`
	SemanticClusters  []SemanticCluster    `json:"semantic_clusters,omitempty"`
}

// ConflictResolution describes how conflicts were resolved
type ConflictResolution struct {
	Topic      string   `json:"topic"`
	Sources    []string `json:"conflicting_sources"`
	Resolution string   `json:"resolution"`
	Reasoning  string   `json:"reasoning"`
}

// SemanticCluster groups semantically similar inputs
type SemanticCluster struct {
	ClusterID   string   `json:"cluster_id"`
	Members     []string `json:"member_agents"`
	CoreConcept string   `json:"core_concept"`
	Similarity  float64  `json:"avg_similarity"`
}

// AggregatorAgent implements AI-powered output aggregation
type AggregatorAgent struct {
	*BaseAgent
	def      agent.AgentDef
	provider provider.Provider
	config   AggregatorConfig
	rt       agent.Runtime

	// AI-specific fields for aggregation
	inputBuffer      map[string]*AgentInput
	bufferMu         sync.RWMutex
	aggregationStats AggregationStats
	statsMu          sync.Mutex
}

// AggregationStats tracks AI performance metrics
type AggregationStats struct {
	TotalAggregations int
	AvgConsensusLevel float64
	ConflictsResolved int
	TokensUsed        int
	ProcessingTimes   []time.Duration
}

// Aggregation strategies
const (
	// LLM-powered strategies
	StrategyConsensus    = "consensus"
	StrategyWeighted     = "weighted"
	StrategySemantic     = "semantic"
	StrategyHierarchical = "hierarchical"
	StrategyRAG          = "rag_based"

	// Deterministic strategies (non-LLM)
	StrategyVotingMajority   = "voting_majority"
	StrategyVotingUnanimous  = "voting_unanimous"
	StrategyVotingWeighted   = "voting_weighted"
	StrategyVotingConfidence = "voting_confidence"
)

func init() {
	agent.Register("aggregator", NewAggregatorAgent)
}

// NewAggregatorAgent creates a new AI-powered aggregator agent
func NewAggregatorAgent(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
	var config AggregatorConfig
	if err := def.UnmarshalKey("aggregator_config", &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal aggregator config: %w", err)
	}

	// Set AI-optimized defaults
	if config.Temperature == 0 {
		config.Temperature = 0.5 // Balanced creativity for synthesis
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 1500 // More tokens for comprehensive aggregation
	}
	if config.TimeoutMs == 0 {
		config.TimeoutMs = 5000
	}
	if config.SemanticSimilarity == 0 {
		config.SemanticSimilarity = 0.85
	}
	if config.ConsensusThreshold == 0 {
		config.ConsensusThreshold = 0.7
	}
	if config.AggregationStrategy == "" {
		config.AggregationStrategy = StrategyConsensus
	}

	// Initialize provider
	prov, err := initializeProvider(def.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	return &AggregatorAgent{
		BaseAgent:   NewBaseAgent(def),
		def:         def,
		provider:    prov,
		config:      config,
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}, nil
}

// Execute performs synchronous aggregation
func (a *AggregatorAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	if !a.Ready() {
		return nil, fmt.Errorf("agent not ready")
	}

	// Convert input message to AgentInput
	agentInput := &AgentInput{
		AgentName: "input",
		Content:   input.Payload,
		Timestamp: time.Now(),
	}

	// Perform aggregation and return result
	result, err := a.aggregate(ctx, []*AgentInput{agentInput})
	if err != nil {
		return nil, err
	}

	// Convert AggregationResult to agent.Message
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal aggregation result: %w", err)
	}

	return &agent.Message{
		Message: &pb.Message{
			Type:    "aggregation_result",
			Payload: string(resultJSON),
		},
	}, nil
}

// Start begins the aggregation agent's processing loop
func (a *AggregatorAgent) Start(ctx context.Context) error {
	a.InitContext(ctx)
	if len(a.def.Inputs) == 0 {
		return fmt.Errorf("no inputs defined for AggregatorAgent")
	}

	// Create channels for all input sources
	inputChannels := make([]<-chan *agent.Message, len(a.def.Inputs))
	for i, input := range a.def.Inputs {
		ch, err := a.rt.Recv(input.Source)
		if err != nil {
			return fmt.Errorf("failed to receive from %s: %w", input.Source, err)
		}
		inputChannels[i] = ch
	}

	// Process inputs with timeout-based aggregation windows
	ticker := time.NewTicker(time.Duration(a.config.TimeoutMs) * time.Millisecond)
	defer ticker.Stop()

	validator := &security.StringValidator{
		MaxLength:            100000,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Process buffered inputs
			if a.hasBufferedInputs() {
				a.processAggregation(ctx)
			}
		default:
			// Collect inputs from all channels
			for i, ch := range inputChannels {
				select {
				case msg := <-ch:
					if msg != nil {
						if err := validator.Validate(msg.Payload); err != nil {
							log.Printf("Aggregator input validation error from source %d: %v", i, err)
							continue
						}
						a.bufferInput(a.def.Inputs[i].Source, msg)
					}
				default:
					continue
				}
			}
		}
	}
}

// bufferInput adds an input to the aggregation buffer
func (a *AggregatorAgent) bufferInput(source string, msg *agent.Message) {
	a.bufferMu.Lock()
	defer a.bufferMu.Unlock()

	input := &AgentInput{
		AgentName: source,
		Content:   msg.Payload,
		Timestamp: time.Now(),
	}

	// Parse additional metadata if available
	var metadata map[string]any
	if err := json.Unmarshal([]byte(msg.Payload), &metadata); err == nil {
		if conf, ok := metadata["confidence"].(float64); ok {
			input.Confidence = conf
		}
		input.Metadata = metadata
	}

	a.inputBuffer[source] = input
}

// hasBufferedInputs checks if there are inputs to process
func (a *AggregatorAgent) hasBufferedInputs() bool {
	a.bufferMu.RLock()
	defer a.bufferMu.RUnlock()
	return len(a.inputBuffer) > 0
}

// processAggregation performs AI-powered aggregation
func (a *AggregatorAgent) processAggregation(ctx context.Context) {
	startTime := time.Now()

	a.bufferMu.Lock()
	inputs := make([]*AgentInput, 0, len(a.inputBuffer))
	for _, input := range a.inputBuffer {
		inputs = append(inputs, input)
	}
	// Clear buffer
	a.inputBuffer = make(map[string]*AgentInput)
	a.bufferMu.Unlock()

	if len(inputs) == 0 {
		return
	}

	span := observability.StartSpan("aggregator.aggregate", map[string]any{
		"input_count": len(inputs),
		"strategy":    a.config.AggregationStrategy,
	})
	defer span.End()

	// Perform aggregation based on strategy
	result, err := a.aggregate(ctx, inputs)
	if err != nil {
		log.Printf("Aggregation error: %v", err)
		return
	}

	result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
	a.sendResult(result)

	// Update statistics
	a.updateStats(result, time.Since(startTime))
}

// aggregate performs the actual AI-powered aggregation
func (a *AggregatorAgent) aggregate(ctx context.Context, inputs []*AgentInput) (*AggregationResult, error) {
	strategy := a.config.AggregationStrategy
	// Default to consensus if strategy is empty
	if strategy == "" {
		strategy = StrategyConsensus
	}

	switch strategy {
	// LLM-powered strategies
	case StrategyConsensus:
		return a.aggregateByConsensus(ctx, inputs)
	case StrategyWeighted:
		return a.aggregateByWeight(ctx, inputs)
	case StrategySemantic:
		return a.aggregateBySemantic(ctx, inputs)
	case StrategyHierarchical:
		return a.aggregateHierarchical(ctx, inputs)
	case StrategyRAG:
		return a.aggregateWithRAG(ctx, inputs)

	// Deterministic strategies (non-LLM)
	case StrategyVotingMajority:
		return a.aggregateByVotingMajority(inputs)
	case StrategyVotingUnanimous:
		return a.aggregateByVotingUnanimous(inputs)
	case StrategyVotingWeighted:
		return a.aggregateByVotingWeighted(inputs)
	case StrategyVotingConfidence:
		return a.aggregateByVotingConfidence(inputs)

	default:
		return nil, fmt.Errorf("unknown aggregation strategy: %s", a.config.AggregationStrategy)
	}
}

// aggregateByConsensus uses LLM to find consensus among inputs
func (a *AggregatorAgent) aggregateByConsensus(ctx context.Context, inputs []*AgentInput) (*AggregationResult, error) {
	// Build prompt for consensus finding
	prompt := a.buildConsensusPrompt(inputs)

	// Create structured request for better output
	schema := a.buildAggregationSchema()

	req := provider.StructuredRequest{
		CompletionRequest: provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "system", Content: a.getAggregatorSystemPrompt()},
				{Role: "user", Content: prompt},
			},
			Model:       a.def.Model,
			Temperature: a.config.Temperature,
			MaxTokens:   a.config.MaxTokens,
		},
		ResponseSchema: schema,
		StrictSchema:   true,
	}

	resp, err := a.provider.CreateStructured(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM aggregation failed: %w", err)
	}

	// Parse response
	var result AggregationResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse aggregation result: %w", err)
	}

	// Add metadata
	result.Strategy = StrategyConsensus
	result.TokensUsed = resp.Usage.TotalTokens
	result.Sources = a.extractSources(inputs)

	// Calculate consensus level
	result.ConsensusLevel = a.calculateConsensus(inputs, result.AggregatedContent)

	return &result, nil
}

// aggregateBySemantic groups inputs by semantic similarity
func (a *AggregatorAgent) aggregateBySemantic(ctx context.Context, inputs []*AgentInput) (*AggregationResult, error) {
	// Group inputs into semantic clusters
	clusters := a.createSemanticClusters(inputs)

	// Build prompt with cluster information
	prompt := a.buildSemanticPrompt(inputs, clusters)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: a.getSemanticSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		Model:       a.def.Model,
		Temperature: a.config.Temperature,
		MaxTokens:   a.config.MaxTokens,
	}

	resp, err := a.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("semantic aggregation failed: %w", err)
	}

	return &AggregationResult{
		AggregatedContent: resp.Content,
		Strategy:          StrategySemantic,
		TokensUsed:        resp.Usage.TotalTokens,
		Sources:           a.extractSources(inputs),
		SemanticClusters:  clusters,
		ConsensusLevel:    a.calculateSemanticConsensus(clusters),
	}, nil
}

// aggregateByWeight applies weighted aggregation based on agent importance
func (a *AggregatorAgent) aggregateByWeight(ctx context.Context, inputs []*AgentInput) (*AggregationResult, error) {
	// Sort inputs by weight
	weightedInputs := a.applyWeights(inputs)

	prompt := a.buildWeightedPrompt(weightedInputs)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: a.getWeightedSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		Model:       a.def.Model,
		Temperature: a.config.Temperature,
		MaxTokens:   a.config.MaxTokens,
	}

	resp, err := a.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("weighted aggregation failed: %w", err)
	}

	return &AggregationResult{
		AggregatedContent: resp.Content,
		Strategy:          StrategyWeighted,
		TokensUsed:        resp.Usage.TotalTokens,
		Sources:           a.extractSources(inputs),
		ConsensusLevel:    a.calculateWeightedConsensus(weightedInputs),
	}, nil
}

// aggregateHierarchical performs multi-level aggregation
func (a *AggregatorAgent) aggregateHierarchical(ctx context.Context, inputs []*AgentInput) (*AggregationResult, error) {
	// First level: Group and summarize
	groups := a.createHierarchicalGroups(inputs)
	summaries := make([]string, 0, len(groups))

	for _, group := range groups {
		summary, err := a.summarizeGroup(ctx, group)
		if err != nil {
			log.Printf("Failed to summarize group: %v", err)
			continue
		}
		summaries = append(summaries, summary)
	}

	// Second level: Aggregate summaries
	finalPrompt := a.buildHierarchicalFinalPrompt(summaries)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: a.getHierarchicalSystemPrompt()},
			{Role: "user", Content: finalPrompt},
		},
		Model:       a.def.Model,
		Temperature: a.config.Temperature,
		MaxTokens:   a.config.MaxTokens,
	}

	resp, err := a.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("hierarchical aggregation failed: %w", err)
	}

	return &AggregationResult{
		AggregatedContent: resp.Content,
		Strategy:          StrategyHierarchical,
		TokensUsed:        resp.Usage.TotalTokens,
		Sources:           a.extractSources(inputs),
		ConsensusLevel:    0.8, // Hierarchical typically has good consensus
	}, nil
}

// aggregateWithRAG uses retrieval-augmented generation
func (a *AggregatorAgent) aggregateWithRAG(ctx context.Context, inputs []*AgentInput) (*AggregationResult, error) {
	// Build RAG context from inputs
	ragContext := a.buildRAGContext(inputs)

	prompt := fmt.Sprintf(`Using the following retrieved context from multiple agents, synthesize a comprehensive answer:

Context:
%s

Task: Create a unified, coherent response that incorporates insights from all sources while maintaining accuracy and completeness.`, ragContext)

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: a.getRAGSystemPrompt()},
			{Role: "user", Content: prompt},
		},
		Model:       a.def.Model,
		Temperature: a.config.Temperature,
		MaxTokens:   a.config.MaxTokens,
	}

	resp, err := a.provider.CreateCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("RAG aggregation failed: %w", err)
	}

	return &AggregationResult{
		AggregatedContent: resp.Content,
		Strategy:          StrategyRAG,
		TokensUsed:        resp.Usage.TotalTokens,
		Sources:           a.extractSources(inputs),
		ConsensusLevel:    0.85, // RAG typically produces high consensus
	}, nil
}

// Deterministic aggregation methods (non-LLM)

// aggregateByVotingMajority uses simple majority voting
func (a *AggregatorAgent) aggregateByVotingMajority(inputs []*AgentInput) (*AggregationResult, error) {
	votingInputs := a.convertToVotingInputs(inputs)
	result, err := aggregation.MajorityVote(votingInputs)
	if err != nil {
		return nil, err
	}

	return &AggregationResult{
		AggregatedContent: result.SelectedContent,
		Strategy:          StrategyVotingMajority,
		ConsensusLevel:    result.Agreement,
		Sources:           a.extractSources(inputs),
		TokensUsed:        0, // No LLM calls
		SummaryInsights:   result.Explanation,
	}, nil
}

// aggregateByVotingUnanimous requires all inputs to agree
func (a *AggregatorAgent) aggregateByVotingUnanimous(inputs []*AgentInput) (*AggregationResult, error) {
	votingInputs := a.convertToVotingInputs(inputs)
	result, err := aggregation.UnanimousVote(votingInputs)
	if err != nil {
		return nil, err
	}

	return &AggregationResult{
		AggregatedContent: result.SelectedContent,
		Strategy:          StrategyVotingUnanimous,
		ConsensusLevel:    result.Agreement,
		Sources:           a.extractSources(inputs),
		TokensUsed:        0, // No LLM calls
		SummaryInsights:   result.Explanation,
	}, nil
}

// aggregateByVotingWeighted uses confidence-weighted voting
func (a *AggregatorAgent) aggregateByVotingWeighted(inputs []*AgentInput) (*AggregationResult, error) {
	votingInputs := a.convertToVotingInputs(inputs)
	result, err := aggregation.WeightedVote(votingInputs)
	if err != nil {
		return nil, err
	}

	return &AggregationResult{
		AggregatedContent: result.SelectedContent,
		Strategy:          StrategyVotingWeighted,
		ConsensusLevel:    result.Agreement,
		Sources:           a.extractSources(inputs),
		TokensUsed:        0, // No LLM calls
		SummaryInsights:   result.Explanation,
	}, nil
}

// aggregateByVotingConfidence selects input with highest confidence
func (a *AggregatorAgent) aggregateByVotingConfidence(inputs []*AgentInput) (*AggregationResult, error) {
	votingInputs := a.convertToVotingInputs(inputs)
	result, err := aggregation.ConfidenceVote(votingInputs)
	if err != nil {
		return nil, err
	}

	return &AggregationResult{
		AggregatedContent: result.SelectedContent,
		Strategy:          StrategyVotingConfidence,
		ConsensusLevel:    result.Agreement,
		Sources:           a.extractSources(inputs),
		TokensUsed:        0, // No LLM calls
		SummaryInsights:   result.Explanation,
	}, nil
}

// convertToVotingInputs converts AgentInput to aggregation.VotingInput
func (a *AggregatorAgent) convertToVotingInputs(inputs []*AgentInput) []aggregation.VotingInput {
	result := make([]aggregation.VotingInput, len(inputs))
	for i, input := range inputs {
		result[i] = aggregation.VotingInput{
			Source:     input.AgentName,
			Content:    input.Content,
			Confidence: input.Confidence,
			Metadata:   input.Metadata,
		}
	}
	return result
}

// Helper methods for prompt building

func (a *AggregatorAgent) buildConsensusPrompt(inputs []*AgentInput) string {
	var inputTexts []string
	for _, input := range inputs {
		inputTexts = append(inputTexts, fmt.Sprintf("Agent %s: %s", input.AgentName, input.Content))
	}

	return fmt.Sprintf(`Analyze the following inputs from multiple agents and create a consensus-based aggregation:

%s

Instructions:
1. Identify common themes and agreements
2. Resolve any conflicts with reasoning
3. Synthesize a unified response
4. Note areas of disagreement if critical
5. Provide confidence in the consensus

Return structured JSON with the aggregated content and analysis.`, strings.Join(inputTexts, "\n\n"))
}

func (a *AggregatorAgent) buildSemanticPrompt(inputs []*AgentInput, clusters []SemanticCluster) string {
	clusterDesc := "Semantic clusters identified:\n"
	for _, cluster := range clusters {
		clusterDesc += fmt.Sprintf("- Cluster %s: %s (members: %v)\n",
			cluster.ClusterID, cluster.CoreConcept, cluster.Members)
	}

	return fmt.Sprintf(`%s

Based on these semantic groupings, create a comprehensive aggregation that:
1. Represents each semantic cluster appropriately
2. Maintains the relationships between concepts
3. Provides a coherent narrative`, clusterDesc)
}

func (a *AggregatorAgent) getAggregatorSystemPrompt() string {
	return `You are an expert AI aggregation agent specialized in synthesizing multiple inputs into coherent, comprehensive outputs.

Your capabilities:
- Identify consensus and conflicts
- Perform semantic deduplication
- Maintain information fidelity
- Create structured summaries
- Resolve contradictions intelligently

Always provide clear, actionable aggregations with transparency about the synthesis process.`
}

func (a *AggregatorAgent) getSemanticSystemPrompt() string {
	return `You are a semantic aggregation specialist. Your task is to understand the deep meaning and relationships between different inputs, grouping them by semantic similarity and creating a unified narrative that preserves the essential information from each semantic cluster.`
}

func (a *AggregatorAgent) getWeightedSystemPrompt() string {
	return `You are a weighted aggregation expert. Consider the importance and credibility weights assigned to each input source. Give more prominence to higher-weighted sources while still incorporating insights from all contributors.`
}

func (a *AggregatorAgent) getHierarchicalSystemPrompt() string {
	return `You are performing hierarchical aggregation. First summarize groups of related inputs, then synthesize these summaries into a final, comprehensive output. Maintain the hierarchical structure of information.`
}

func (a *AggregatorAgent) getRAGSystemPrompt() string {
	return `You are a RAG-based aggregation system. Use the retrieved context from multiple agents to generate a comprehensive, accurate response. Ensure factual consistency and cite sources when appropriate.`
}

// Utility methods

func (a *AggregatorAgent) buildAggregationSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"aggregated_content": map[string]any{"type": "string"},
			"conflicts_resolved": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"topic":      map[string]any{"type": "string"},
						"resolution": map[string]any{"type": "string"},
						"reasoning":  map[string]any{"type": "string"},
					},
				},
			},
			"summary_insights": map[string]any{"type": "string"},
		},
		"required": []string{"aggregated_content"},
	}

	data, _ := json.Marshal(schema)
	return data
}

func (a *AggregatorAgent) extractSources(inputs []*AgentInput) []string {
	sources := make([]string, len(inputs))
	for i, input := range inputs {
		sources[i] = input.AgentName
	}
	return sources
}

func (a *AggregatorAgent) calculateConsensus(inputs []*AgentInput, aggregated string) float64 {
	// Calculate consensus based on similarity of inputs to the aggregated result
	if len(inputs) == 0 {
		return 0.0
	}

	// Calculate how similar each input is to the aggregated result
	totalSimilarity := 0.0
	for _, input := range inputs {
		similarity := a.calculateTextSimilarity(input.Content, aggregated)
		// Weight by confidence if available
		weight := input.Confidence
		if weight == 0 {
			weight = 1.0
		}
		totalSimilarity += similarity * weight
	}

	// Also consider pairwise similarity among inputs
	pairwiseSimilarity := 0.0
	pairCount := 0
	for i, input1 := range inputs {
		for j := i + 1; j < len(inputs); j++ {
			input2 := inputs[j]
			similarity := a.calculateTextSimilarity(input1.Content, input2.Content)
			pairwiseSimilarity += similarity
			pairCount++
		}
	}

	// Average pairwise similarity
	avgPairwise := 0.0
	if pairCount > 0 {
		avgPairwise = pairwiseSimilarity / float64(pairCount)
	} else if len(inputs) == 1 {
		// Single input case - just check similarity to aggregated
		avgPairwise = 1.0
	}

	// Weighted average of aggregated similarity and pairwise similarity
	// Higher weight on aggregated similarity (60%) vs pairwise (40%)
	totalWeights := 0.0
	for _, input := range inputs {
		weight := input.Confidence
		if weight == 0 {
			weight = 1.0
		}
		totalWeights += weight
	}

	if totalWeights == 0 {
		totalWeights = float64(len(inputs))
	}

	aggregatedSimilarity := totalSimilarity / totalWeights
	consensus := 0.6*aggregatedSimilarity + 0.4*avgPairwise

	// Ensure consensus is between 0 and 1
	if consensus > 1.0 {
		consensus = 1.0
	} else if consensus < 0.0 {
		consensus = 0.0
	}

	return consensus
}

func (a *AggregatorAgent) calculateSemanticConsensus(clusters []SemanticCluster) float64 {
	if len(clusters) == 0 {
		return 0
	}
	var totalSim float64
	for _, cluster := range clusters {
		totalSim += cluster.Similarity
	}
	return totalSim / float64(len(clusters))
}

func (a *AggregatorAgent) calculateWeightedConsensus(inputs []*AgentInput) float64 {
	// Handle edge cases
	if len(inputs) == 0 {
		return 0.0
	}
	if len(inputs) == 1 {
		return 1.0 // Single input has perfect consensus
	}

	// Weighted consensus based on confidence scores
	// This calculates a weighted average where each agent's contribution
	// is weighted by their confidence score
	var totalWeight, weightedSum float64

	// First pass: calculate similarity scores between inputs
	// We'll use a simple approach: if contents are similar, they get higher consensus
	similarityMatrix := make([][]float64, len(inputs))
	for i := range similarityMatrix {
		similarityMatrix[i] = make([]float64, len(inputs))
	}

	// Calculate pairwise similarities
	for i, input1 := range inputs {
		for j, input2 := range inputs {
			if i == j {
				similarityMatrix[i][j] = 1.0
			} else {
				// Simple similarity based on content overlap
				sim := a.calculateTextSimilarity(input1.Content, input2.Content)
				similarityMatrix[i][j] = sim
			}
		}
	}

	// Calculate weighted consensus based on similarity and confidence
	for i, input := range inputs {
		weight := input.Confidence
		if weight == 0 {
			weight = 0.5
		}

		// Calculate average similarity with other inputs
		avgSimilarity := 0.0
		for j, otherInput := range inputs {
			if i != j {
				otherWeight := otherInput.Confidence
				if otherWeight == 0 {
					otherWeight = 0.5
				}
				avgSimilarity += similarityMatrix[i][j] * otherWeight
			}
		}
		if len(inputs) > 1 {
			avgSimilarity /= float64(len(inputs) - 1)
		}

		totalWeight += weight
		weightedSum += avgSimilarity * weight
	}

	if totalWeight == 0 {
		return 0
	}
	return weightedSum / totalWeight
}

func (a *AggregatorAgent) createSemanticClusters(inputs []*AgentInput) []SemanticCluster {
	// Implement text similarity-based clustering
	// This is a simple hierarchical clustering approach
	if len(inputs) == 0 {
		return []SemanticCluster{}
	}

	// Calculate similarity matrix
	n := len(inputs)
	similarities := make([][]float64, n)
	for i := range similarities {
		similarities[i] = make([]float64, n)
	}

	// Compute pairwise similarities
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				similarities[i][j] = 1.0
			} else {
				similarities[i][j] = a.calculateTextSimilarity(inputs[i].Content, inputs[j].Content)
			}
		}
	}

	// Simple clustering: group inputs with similarity above threshold
	threshold := a.config.SemanticSimilarity
	if threshold == 0 {
		threshold = 0.7 // Default threshold
	}

	clusters := []SemanticCluster{}
	clustered := make([]bool, n)
	clusterID := 0

	for i := 0; i < n; i++ {
		if clustered[i] {
			continue
		}

		// Start new cluster
		cluster := SemanticCluster{
			ClusterID:   fmt.Sprintf("cluster_%d", clusterID),
			CoreConcept: fmt.Sprintf("concept_%s", inputs[i].AgentName),
			Members:     []string{inputs[i].AgentName},
			Similarity:  1.0, // Initial similarity for the seed
		}
		clustered[i] = true
		clusterID++

		// Find similar inputs to add to this cluster
		totalSim := 1.0
		memberCount := 1

		for j := i + 1; j < n; j++ {
			if clustered[j] {
				continue
			}

			// Check similarity with all current cluster members
			avgSim := 0.0
			for k := 0; k < n; k++ {
				if clustered[k] && k != j {
					// Check if k is in current cluster
					inCluster := false
					for _, member := range cluster.Members {
						if inputs[k].AgentName == member {
							inCluster = true
							break
						}
					}
					if inCluster {
						avgSim += similarities[j][k]
					}
				}
			}
			avgSim /= float64(len(cluster.Members))

			// Add to cluster if similarity is above threshold
			if avgSim >= threshold {
				cluster.Members = append(cluster.Members, inputs[j].AgentName)
				clustered[j] = true
				totalSim += avgSim
				memberCount++
			}
		}

		// Update cluster similarity as average
		cluster.Similarity = totalSim / float64(memberCount)
		clusters = append(clusters, cluster)
	}

	// If no meaningful clusters were formed, create a single cluster
	if len(clusters) == 0 {
		cluster := SemanticCluster{
			ClusterID:   "default",
			CoreConcept: "aggregated_content",
			Similarity:  a.config.SemanticSimilarity,
		}
		for _, input := range inputs {
			cluster.Members = append(cluster.Members, input.AgentName)
		}
		return []SemanticCluster{cluster}
	}

	return clusters
}

func (a *AggregatorAgent) applyWeights(inputs []*AgentInput) []*AgentInput {
	// Apply configured weights
	for _, input := range inputs {
		if weight, exists := a.config.WeightedAggregation[input.AgentName]; exists {
			input.Confidence = weight
		}
	}
	// Sort by weight
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].Confidence > inputs[j].Confidence
	})
	return inputs
}

func (a *AggregatorAgent) createHierarchicalGroups(inputs []*AgentInput) [][]*AgentInput {
	// Simple grouping - in production, use more sophisticated grouping
	groupSize := 3
	var groups [][]*AgentInput
	for i := 0; i < len(inputs); i += groupSize {
		end := i + groupSize
		if end > len(inputs) {
			end = len(inputs)
		}
		groups = append(groups, inputs[i:end])
	}
	return groups
}

func (a *AggregatorAgent) summarizeGroup(ctx context.Context, group []*AgentInput) (string, error) {
	var contents []string
	for _, input := range group {
		contents = append(contents, input.Content)
	}

	req := provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "Summarize the following inputs concisely:"},
			{Role: "user", Content: strings.Join(contents, "\n")},
		},
		Model:       a.def.Model,
		Temperature: 0.3,
		MaxTokens:   200,
	}

	resp, err := a.provider.CreateCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (a *AggregatorAgent) buildWeightedPrompt(inputs []*AgentInput) string {
	var weighted []string
	for _, input := range inputs {
		weighted = append(weighted, fmt.Sprintf("[Weight: %.2f] %s: %s",
			input.Confidence, input.AgentName, input.Content))
	}
	return fmt.Sprintf("Aggregate with weights:\n%s", strings.Join(weighted, "\n"))
}

func (a *AggregatorAgent) buildHierarchicalFinalPrompt(summaries []string) string {
	return fmt.Sprintf("Final aggregation of summaries:\n%s", strings.Join(summaries, "\n"))
}

func (a *AggregatorAgent) buildRAGContext(inputs []*AgentInput) string {
	var context []string
	for _, input := range inputs {
		context = append(context, fmt.Sprintf("[%s]: %s", input.AgentName, input.Content))
	}
	return strings.Join(context, "\n\n")
}

func (a *AggregatorAgent) updateStats(result *AggregationResult, duration time.Duration) {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()

	a.aggregationStats.TotalAggregations++
	a.aggregationStats.TokensUsed += result.TokensUsed
	a.aggregationStats.ProcessingTimes = append(a.aggregationStats.ProcessingTimes, duration)

	// Update average consensus
	prev := a.aggregationStats.AvgConsensusLevel
	a.aggregationStats.AvgConsensusLevel = (prev*float64(a.aggregationStats.TotalAggregations-1) + result.ConsensusLevel) / float64(a.aggregationStats.TotalAggregations)

	if len(result.ConflictsSolved) > 0 {
		a.aggregationStats.ConflictsResolved += len(result.ConflictsSolved)
	}

	// Log stats periodically
	if a.aggregationStats.TotalAggregations%10 == 0 {
		a.logAggregationStats()
	}
}

func (a *AggregatorAgent) logAggregationStats() {
	a.statsMu.Lock()
	defer a.statsMu.Unlock()

	var avgTime time.Duration
	if len(a.aggregationStats.ProcessingTimes) > 0 {
		var total time.Duration
		for _, t := range a.aggregationStats.ProcessingTimes {
			total += t
		}
		avgTime = total / time.Duration(len(a.aggregationStats.ProcessingTimes))
	}

	log.Printf("Aggregator Stats: Total: %d, Avg Consensus: %.2f, Conflicts: %d, Avg Time: %v, Tokens: %d",
		a.aggregationStats.TotalAggregations,
		a.aggregationStats.AvgConsensusLevel,
		a.aggregationStats.ConflictsResolved,
		avgTime,
		a.aggregationStats.TokensUsed)
}

func (a *AggregatorAgent) sendResult(result *AggregationResult) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		log.Printf("Failed to marshal aggregation result: %v", err)
		return
	}

	out := &agent.Message{Message: &pb.Message{
		Type:      "aggregation",
		Payload:   string(resultJSON),
		Timestamp: time.Now().Format(time.RFC3339),
	}}

	for _, o := range a.def.Outputs {
		if err := a.rt.Send(o.Target, out); err != nil {
			log.Printf("Error sending aggregation to %s: %v", o.Target, err)
		}
	}
}

// calculateTextSimilarity computes similarity between two text strings
// using Levenshtein distance normalized to 0-1 range
func (a *AggregatorAgent) calculateTextSimilarity(text1, text2 string) float64 {
	if text1 == text2 {
		return 1.0
	}
	if text1 == "" || text2 == "" {
		return 0.0
	}

	// Calculate Levenshtein distance
	distance := levenshteinDistance(text1, text2)
	maxLen := maxOf2(len(text1), len(text2))
	if maxLen == 0 {
		return 1.0
	}

	// Normalize to 0-1 range (1 = identical, 0 = completely different)
	similarity := 1.0 - float64(distance)/float64(maxLen)
	if similarity < 0 {
		similarity = 0
	}
	return similarity
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first column and row
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = minOf3(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// minOf3 returns the minimum of three integers
func minOf3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// maxOf2 returns the maximum of two integers
func maxOf2(a, b int) int {
	if a > b {
		return a
	}
	return b
}
