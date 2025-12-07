package agents

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewAggregatorAgent(t *testing.T) {
	// Skip this test as it requires provider initialization
	t.Skip("Skipping test that requires provider initialization")

	// Test would validate configuration:
	// - aggregation_strategy: consensus
	// - timeout_ms: 3000
	// - semantic_similarity: 0.85
	// - consensus_threshold: 0.75
	// - summarization_enabled: true
}

func TestAggregatorBuffering(t *testing.T) {
	aggAgent := &AggregatorAgent{
		inputBuffer: make(map[string]*AgentInput),
	}

	// Test buffering inputs
	msg1 := &agent.Message{
		Message: &pb.Message{
			Payload: "First agent output",
		},
	}
	aggAgent.bufferInput("agent1", msg1)

	msg2 := &agent.Message{
		Message: &pb.Message{
			Payload: `{"content": "Second output", "confidence": 0.9}`,
		},
	}
	aggAgent.bufferInput("agent2", msg2)

	assert.Equal(t, 2, len(aggAgent.inputBuffer))
	assert.True(t, aggAgent.hasBufferedInputs())

	// Check metadata parsing
	agent2Input := aggAgent.inputBuffer["agent2"]
	assert.Equal(t, 0.9, agent2Input.Confidence)
	assert.NotNil(t, agent2Input.Metadata)
}

func TestAggregatorStrategies(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: AggregatorConfig{
			Temperature:        0.5,
			MaxTokens:          1500,
			SemanticSimilarity: 0.85,
			ConsensusThreshold: 0.7,
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Solution A is optimal", Confidence: 0.8},
		{AgentName: "agent2", Content: "Solution A with minor changes", Confidence: 0.7},
		{AgentName: "agent3", Content: "Solution B is better", Confidence: 0.6},
	}

	// Test Consensus Strategy
	t.Run("ConsensusStrategy", func(t *testing.T) {
		aggAgent.config.AggregationStrategy = StrategyConsensus

		mockResult := AggregationResult{
			AggregatedContent: "Consensus: Solution A is preferred with minor modifications",
			ConflictsSolved: []ConflictResolution{
				{
					Topic:      "Solution choice",
					Resolution: "Solution A chosen",
					Reasoning:  "Majority agreement",
				},
			},
		}
		resultJSON, _ := json.Marshal(mockResult)

		mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
			Data: resultJSON,
			CompletionResponse: provider.CompletionResponse{
				Usage: provider.Usage{TotalTokens: 200},
			},
		}, nil).Once()

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.NoError(t, err)
		assert.Equal(t, StrategyConsensus, result.Strategy)
		assert.Equal(t, 200, result.TokensUsed)
		assert.Contains(t, result.AggregatedContent, "Solution A")
		assert.Equal(t, 1, len(result.ConflictsSolved))
	})

	// Test Semantic Strategy
	t.Run("SemanticStrategy", func(t *testing.T) {
		aggAgent.config.AggregationStrategy = StrategySemantic

		mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
			Content: "Semantic aggregation: Common theme is optimization",
			Usage:   provider.Usage{TotalTokens: 180},
		}, nil).Once()

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.NoError(t, err)
		assert.Equal(t, StrategySemantic, result.Strategy)
		assert.Contains(t, result.AggregatedContent, "optimization")
	})

	// Test Weighted Strategy
	t.Run("WeightedStrategy", func(t *testing.T) {
		aggAgent.config.AggregationStrategy = StrategyWeighted
		aggAgent.config.WeightedAggregation = map[string]float64{
			"agent1": 0.5,
			"agent2": 0.3,
			"agent3": 0.2,
		}

		mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
			Content: "Weighted result favoring agent1's solution",
			Usage:   provider.Usage{TotalTokens: 150},
		}, nil).Once()

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.NoError(t, err)
		assert.Equal(t, StrategyWeighted, result.Strategy)
	})

	// Test Hierarchical Strategy
	t.Run("HierarchicalStrategy", func(t *testing.T) {
		aggAgent.config.AggregationStrategy = StrategyHierarchical

		// Mock summarization calls (2 groups need 2 summarization calls)
		mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
			Content: "Hierarchical summary",
			Usage:   provider.Usage{TotalTokens: 100},
		}, nil).Times(2)

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.NoError(t, err)
		assert.Equal(t, StrategyHierarchical, result.Strategy)
	})

	// Test RAG Strategy
	t.Run("RAGStrategy", func(t *testing.T) {
		aggAgent.config.AggregationStrategy = StrategyRAG

		mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
			Content: "RAG-based synthesis of all inputs",
			Usage:   provider.Usage{TotalTokens: 250},
		}, nil).Once()

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.NoError(t, err)
		assert.Equal(t, StrategyRAG, result.Strategy)
		assert.Equal(t, 0.85, result.ConsensusLevel) // RAG default consensus
	})

	mockProvider.AssertExpectations(t)
}

func TestAggregatorPromptBuilding(t *testing.T) {
	aggAgent := &AggregatorAgent{
		config: AggregatorConfig{
			SemanticSimilarity: 0.85,
		},
	}

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Analysis shows positive trend"},
		{AgentName: "agent2", Content: "Data indicates growth"},
		{AgentName: "agent3", Content: "Metrics are declining"},
	}

	// Test consensus prompt
	consensusPrompt := aggAgent.buildConsensusPrompt(inputs)
	assert.Contains(t, consensusPrompt, "Agent agent1:")
	assert.Contains(t, consensusPrompt, "Analysis shows positive trend")
	assert.Contains(t, consensusPrompt, "Identify common themes")
	assert.Contains(t, consensusPrompt, "Resolve any conflicts")

	// Test semantic prompt with clusters
	clusters := []SemanticCluster{
		{
			ClusterID:   "cluster1",
			Members:     []string{"agent1", "agent2"},
			CoreConcept: "positive indicators",
			Similarity:  0.9,
		},
	}
	semanticPrompt := aggAgent.buildSemanticPrompt(inputs, clusters)
	assert.Contains(t, semanticPrompt, "cluster1")
	assert.Contains(t, semanticPrompt, "positive indicators")
	assert.Contains(t, semanticPrompt, "semantic groupings")

	// Test RAG context building
	ragContext := aggAgent.buildRAGContext(inputs)
	assert.Contains(t, ragContext, "[agent1]:")
	assert.Contains(t, ragContext, "[agent2]:")
	assert.Contains(t, ragContext, "[agent3]:")
}

func TestAggregatorSystemPrompts(t *testing.T) {
	aggAgent := &AggregatorAgent{}

	// Test different system prompts
	prompts := map[string]func() string{
		"aggregator":   aggAgent.getAggregatorSystemPrompt,
		"semantic":     aggAgent.getSemanticSystemPrompt,
		"weighted":     aggAgent.getWeightedSystemPrompt,
		"hierarchical": aggAgent.getHierarchicalSystemPrompt,
		"rag":          aggAgent.getRAGSystemPrompt,
	}

	for name, getPrompt := range prompts {
		prompt := getPrompt()
		assert.NotEmpty(t, prompt, "System prompt for %s should not be empty", name)

		// Each should have specific characteristics
		switch name {
		case "semantic":
			assert.Contains(t, prompt, "semantic")
			assert.Contains(t, prompt, "relationships")
		case "weighted":
			assert.Contains(t, prompt, "weight")
			assert.Contains(t, prompt, "importance")
		case "hierarchical":
			assert.Contains(t, prompt, "hierarchical")
		case "rag":
			assert.Contains(t, prompt, "RAG")
			assert.Contains(t, prompt, "retrieved context")
		}
	}
}

func TestAggregatorSchemaBuilding(t *testing.T) {
	aggAgent := &AggregatorAgent{}

	schemaJSON := aggAgent.buildAggregationSchema()

	var schema map[string]any
	err := json.Unmarshal(schemaJSON, &schema)

	assert.NoError(t, err)
	assert.Equal(t, "object", schema["type"])

	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "aggregated_content")
	assert.Contains(t, props, "conflicts_resolved")
	assert.Contains(t, props, "summary_insights")

	// Check required fields
	required := schema["required"].([]any)
	assert.Contains(t, required, "aggregated_content")
}

func TestAggregatorConsensusCalculation(t *testing.T) {
	aggAgent := &AggregatorAgent{
		config: AggregatorConfig{
			ConsensusThreshold: 0.7,
			SemanticSimilarity: 0.85,
		},
	}

	// Test basic consensus calculation
	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Solution A", Confidence: 0.9},
		{AgentName: "agent2", Content: "Solution A variant", Confidence: 0.8},
		{AgentName: "agent3", Content: "Solution B", Confidence: 0.6},
	}

	consensus := aggAgent.calculateConsensus(inputs, "Aggregated: Solution A")
	// Now properly calculates based on text similarity
	// Should be high since aggregated result is similar to Solution A
	assert.Greater(t, consensus, 0.4) // Reasonable consensus
	assert.Less(t, consensus, 1.0)    // Not perfect consensus

	// Test weighted consensus
	weightedConsensus := aggAgent.calculateWeightedConsensus(inputs)
	assert.Greater(t, weightedConsensus, 0.0)

	// Test semantic consensus
	clusters := []SemanticCluster{
		{Similarity: 0.9},
		{Similarity: 0.8},
		{Similarity: 0.7},
	}
	semanticConsensus := aggAgent.calculateSemanticConsensus(clusters)
	assert.InDelta(t, 0.8, semanticConsensus, 0.0001)
}

func TestAggregatorStatistics(t *testing.T) {
	aggAgent := &AggregatorAgent{
		aggregationStats: AggregationStats{},
	}

	// Update stats multiple times
	for i := 0; i < 5; i++ {
		result := &AggregationResult{
			TokensUsed:     100 + i*10,
			ConsensusLevel: 0.7 + float64(i)*0.02,
			ConflictsSolved: []ConflictResolution{
				{Topic: "test", Resolution: "resolved"},
			},
		}
		aggAgent.updateStats(result, time.Duration(100+i)*time.Millisecond)
	}

	stats := aggAgent.aggregationStats
	assert.Equal(t, 5, stats.TotalAggregations)
	assert.Equal(t, 5, stats.ConflictsResolved)
	assert.Greater(t, stats.AvgConsensusLevel, 0.7)
	assert.Equal(t, 5, len(stats.ProcessingTimes))
}

func TestAggregatorParallelGrouping(t *testing.T) {
	aggAgent := &AggregatorAgent{
		config: AggregatorConfig{
			WeightedAggregation: map[string]float64{
				"agent1": 1.0,
				"agent2": 0.8,
				"agent3": 0.6,
			},
		},
	}

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "High priority", Confidence: 0.0},
		{AgentName: "agent2", Content: "Medium priority", Confidence: 0.0},
		{AgentName: "agent3", Content: "Low priority", Confidence: 0.0},
	}

	// Apply weights
	weighted := aggAgent.applyWeights(inputs)

	// Check ordering by weight
	assert.Equal(t, "agent1", weighted[0].AgentName)
	assert.Equal(t, 1.0, weighted[0].Confidence)
	assert.Equal(t, "agent2", weighted[1].AgentName)
	assert.Equal(t, 0.8, weighted[1].Confidence)
	assert.Equal(t, "agent3", weighted[2].AgentName)
	assert.Equal(t, 0.6, weighted[2].Confidence)
}

func TestAggregatorHierarchicalGrouping(t *testing.T) {
	aggAgent := &AggregatorAgent{}

	inputs := []*AgentInput{
		{AgentName: "agent1"},
		{AgentName: "agent2"},
		{AgentName: "agent3"},
		{AgentName: "agent4"},
		{AgentName: "agent5"},
	}

	groups := aggAgent.createHierarchicalGroups(inputs)

	assert.Equal(t, 2, len(groups)) // 5 inputs, group size 3 = 2 groups
	assert.Equal(t, 3, len(groups[0]))
	assert.Equal(t, 2, len(groups[1]))
}

// TestAggregator_VotingMajority tests the voting_majority deterministic strategy
func TestAggregator_VotingMajority(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: AggregatorConfig{
			AggregationStrategy: "voting_majority",
			Temperature:         0.5,
			MaxTokens:           1500,
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	t.Run("clear_majority", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Solution A", Confidence: 0.8},
			{AgentName: "agent2", Content: "Solution A", Confidence: 0.9},
			{AgentName: "agent3", Content: "Solution A", Confidence: 0.7},
			{AgentName: "agent4", Content: "Solution B", Confidence: 0.85},
			{AgentName: "agent5", Content: "Solution C", Confidence: 0.75},
		}

		// No LLM calls should be made for deterministic voting
		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "voting_majority", result.Strategy)
		assert.Equal(t, "Solution A", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed, "Deterministic strategy should use zero tokens")
		assert.Equal(t, 5, len(result.Sources))

		// Verify no LLM calls were made
		mockProvider.AssertNotCalled(t, "CreateCompletion")
		mockProvider.AssertNotCalled(t, "CreateStructured")
	})

	t.Run("tie_broken_by_confidence", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option X", Confidence: 0.7},
			{AgentName: "agent2", Content: "Option X", Confidence: 0.8},
			{AgentName: "agent3", Content: "Option Y", Confidence: 0.95},
			{AgentName: "agent4", Content: "Option Y", Confidence: 0.9},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Option Y", result.AggregatedContent, "Higher average confidence should win tie")
		assert.Equal(t, 0, result.TokensUsed)
	})

	t.Run("single_input", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Only option", Confidence: 0.8},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Only option", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
	})
}

// TestAggregator_VotingUnanimous tests the voting_unanimous deterministic strategy
func TestAggregator_VotingUnanimous(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: AggregatorConfig{
			AggregationStrategy: "voting_unanimous",
			Temperature:         0.5,
			MaxTokens:           1500,
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	t.Run("all_agents_agree", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Unanimous decision", Confidence: 0.8},
			{AgentName: "agent2", Content: "Unanimous decision", Confidence: 0.9},
			{AgentName: "agent3", Content: "Unanimous decision", Confidence: 0.85},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "voting_unanimous", result.Strategy)
		assert.Equal(t, "Unanimous decision", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
		assert.Equal(t, 1.0, result.ConsensusLevel, "Unanimous vote should have perfect consensus")

		mockProvider.AssertNotCalled(t, "CreateCompletion")
		mockProvider.AssertNotCalled(t, "CreateStructured")
	})

	t.Run("one_agent_disagrees", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A", Confidence: 0.8},
			{AgentName: "agent2", Content: "Option A", Confidence: 0.9},
			{AgentName: "agent3", Content: "Option B", Confidence: 0.85},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.Error(t, err, "Unanimous voting should fail when agents disagree")
		assert.Nil(t, result)

		mockProvider.AssertNotCalled(t, "CreateCompletion")
		mockProvider.AssertNotCalled(t, "CreateStructured")
	})

	t.Run("single_input_unanimous", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Single vote", Confidence: 0.9},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Single vote", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
	})
}

// TestAggregator_VotingWeighted tests the voting_weighted deterministic strategy
func TestAggregator_VotingWeighted(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: AggregatorConfig{
			AggregationStrategy: "voting_weighted",
			Temperature:         0.5,
			MaxTokens:           1500,
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	t.Run("weighted_by_confidence", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A", Confidence: 0.9},
			{AgentName: "agent2", Content: "Option A", Confidence: 0.8},
			{AgentName: "agent3", Content: "Option B", Confidence: 0.5},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "voting_weighted", result.Strategy)
		assert.Equal(t, "Option A", result.AggregatedContent, "Higher weighted option should win")
		assert.Equal(t, 0, result.TokensUsed)

		mockProvider.AssertNotCalled(t, "CreateCompletion")
		mockProvider.AssertNotCalled(t, "CreateStructured")
	})

	t.Run("high_confidence_single_vote", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A", Confidence: 0.6},
			{AgentName: "agent2", Content: "Option A", Confidence: 0.6},
			{AgentName: "agent3", Content: "Option B", Confidence: 0.99},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		// Option A has total weight 1.2, Option B has 0.99
		assert.Equal(t, "Option A", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
	})

	t.Run("zero_confidence_ignored", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A", Confidence: 0.0},
			{AgentName: "agent2", Content: "Option B", Confidence: 0.8},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Option B", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
	})
}

// TestAggregator_VotingConfidence tests the voting_confidence deterministic strategy
func TestAggregator_VotingConfidence(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: AggregatorConfig{
			AggregationStrategy: "voting_confidence",
			Temperature:         0.5,
			MaxTokens:           1500,
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	t.Run("highest_confidence_wins", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A", Confidence: 0.7},
			{AgentName: "agent2", Content: "Option B", Confidence: 0.95},
			{AgentName: "agent3", Content: "Option C", Confidence: 0.6},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "voting_confidence", result.Strategy)
		assert.Equal(t, "Option B", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)

		mockProvider.AssertNotCalled(t, "CreateCompletion")
		mockProvider.AssertNotCalled(t, "CreateStructured")
	})

	t.Run("ignore_content_only_confidence", func(t *testing.T) {
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Very long detailed comprehensive answer", Confidence: 0.6},
			{AgentName: "agent2", Content: "Short", Confidence: 0.99},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Short", result.AggregatedContent, "Content length should not matter")
		assert.Equal(t, 0, result.TokensUsed)
	})
}

// TestAggregator_NoLLMCallsForDeterministic verifies zero LLM usage
func TestAggregator_NoLLMCallsForDeterministic(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Option A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Option A", Confidence: 0.9},
		{AgentName: "agent3", Content: "Option B", Confidence: 0.7},
	}

	deterministicStrategies := []string{
		"voting_majority",
		"voting_unanimous",
		"voting_weighted",
		"voting_confidence",
	}

	for _, strategy := range deterministicStrategies {
		t.Run(strategy, func(t *testing.T) {
			aggAgent := &AggregatorAgent{
				def: agent.AgentDef{
					Model: "gpt-4",
				},
				provider: mockProvider,
				config: AggregatorConfig{
					AggregationStrategy: strategy,
					Temperature:         0.5,
					MaxTokens:           1500,
				},
				rt:          rt,
				inputBuffer: make(map[string]*AgentInput),
			}

			// Use unanimous inputs to avoid errors for voting_unanimous
			unanimousInputs := []*AgentInput{
				{AgentName: "agent1", Content: "Same", Confidence: 0.8},
				{AgentName: "agent2", Content: "Same", Confidence: 0.9},
				{AgentName: "agent3", Content: "Same", Confidence: 0.7},
			}

			inputsToUse := inputs
			if strategy == "voting_unanimous" {
				inputsToUse = unanimousInputs
			}

			result, err := aggAgent.aggregate(ctx, inputsToUse)

			require.NoError(t, err)
			assert.Equal(t, 0, result.TokensUsed, "%s should use zero tokens", strategy)
			assert.NotEmpty(t, result.AggregatedContent)

			// Verify mock was never called
			mockProvider.AssertNotCalled(t, "CreateCompletion")
			mockProvider.AssertNotCalled(t, "CreateStructured")
		})
	}
}

// TestAggregator_DeterministicReproducibility tests reproducibility
func TestAggregator_DeterministicReproducibility(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Answer A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Answer A", Confidence: 0.7},
		{AgentName: "agent3", Content: "Answer B", Confidence: 0.9},
		{AgentName: "agent4", Content: "Answer C", Confidence: 0.6},
	}

	deterministicStrategies := []string{
		"voting_majority",
		"voting_weighted",
		"voting_confidence",
	}

	for _, strategy := range deterministicStrategies {
		t.Run(strategy+"_50_iterations", func(t *testing.T) {
			aggAgent := &AggregatorAgent{
				def: agent.AgentDef{
					Model: "gpt-4",
				},
				provider: mockProvider,
				config: AggregatorConfig{
					AggregationStrategy: strategy,
					Temperature:         0.5,
					MaxTokens:           1500,
				},
				rt:          rt,
				inputBuffer: make(map[string]*AgentInput),
			}

			var firstResult string
			for i := 0; i < 50; i++ {
				result, err := aggAgent.aggregate(ctx, inputs)
				require.NoError(t, err)

				if i == 0 {
					firstResult = result.AggregatedContent
				} else {
					assert.Equal(t, firstResult, result.AggregatedContent,
						"%s should produce identical results on iteration %d", strategy, i)
				}

				// Verify zero token usage every iteration
				assert.Equal(t, 0, result.TokensUsed)
			}
		})
	}
}

// TestAggregator_DeterministicEdgeCases tests edge cases for deterministic strategies
func TestAggregator_DeterministicEdgeCases(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	t.Run("empty_input_buffer", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "voting_majority",
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		inputs := []*AgentInput{}

		result, err := aggAgent.aggregate(ctx, inputs)

		assert.Error(t, err, "Empty inputs should return error")
		assert.Nil(t, result)
	})

	t.Run("all_same_content", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "voting_majority",
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Same", Confidence: 0.8},
			{AgentName: "agent2", Content: "Same", Confidence: 0.9},
			{AgentName: "agent3", Content: "Same", Confidence: 0.7},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Same", result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
		assert.Equal(t, 1.0, result.ConsensusLevel, "All same should have perfect consensus")
	})

	t.Run("all_different_content", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "voting_majority",
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A", Confidence: 0.8},
			{AgentName: "agent2", Content: "Option B", Confidence: 0.9},
			{AgentName: "agent3", Content: "Option C", Confidence: 0.7},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.NotEmpty(t, result.AggregatedContent, "Should pick one option")
		assert.Equal(t, 0, result.TokensUsed)
	})

	t.Run("missing_confidence", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "voting_confidence",
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Option A"}, // No confidence
			{AgentName: "agent2", Content: "Option B", Confidence: 0.8},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "Option B", result.AggregatedContent, "Should pick option with confidence")
	})

	t.Run("extremely_long_content", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "voting_majority",
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		longContent := string(make([]byte, 100000)) // 100KB
		inputs := []*AgentInput{
			{AgentName: "agent1", Content: longContent, Confidence: 0.8},
			{AgentName: "agent2", Content: longContent, Confidence: 0.9},
			{AgentName: "agent3", Content: "Short", Confidence: 0.7},
		}

		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, longContent, result.AggregatedContent)
		assert.Equal(t, 0, result.TokensUsed)
	})
}

// TestAggregator_ExistingStrategiesUnchanged tests backwards compatibility
func TestAggregator_ExistingStrategiesUnchanged(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Analysis A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Analysis B", Confidence: 0.9},
	}

	existingStrategies := []string{
		StrategyConsensus,
		StrategyWeighted,
		StrategySemantic,
		StrategyHierarchical,
		StrategyRAG,
	}

	for _, strategy := range existingStrategies {
		t.Run(strategy+"_still_calls_llm", func(t *testing.T) {
			aggAgent := &AggregatorAgent{
				def: agent.AgentDef{
					Model: "gpt-4",
				},
				provider: mockProvider,
				config: AggregatorConfig{
					AggregationStrategy: strategy,
					Temperature:         0.5,
					MaxTokens:           1500,
					SemanticSimilarity:  0.85,
					ConsensusThreshold:  0.7,
				},
				rt:          rt,
				inputBuffer: make(map[string]*AgentInput),
			}

			// Mock LLM responses
			if strategy == StrategyConsensus {
				mockResult := AggregationResult{
					AggregatedContent: "Consensus result",
				}
				resultJSON, _ := json.Marshal(mockResult)

				mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
					Data: resultJSON,
					CompletionResponse: provider.CompletionResponse{
						Usage: provider.Usage{TotalTokens: 200},
					},
				}, nil).Once()
			} else {
				mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
					Content: "LLM aggregated result",
					Usage:   provider.Usage{TotalTokens: 200},
				}, nil).Maybe()
			}

			result, err := aggAgent.aggregate(ctx, inputs)

			require.NoError(t, err)
			assert.Greater(t, result.TokensUsed, 0, "%s should use LLM tokens", strategy)

			// Verify LLM was called
			if strategy == StrategyConsensus {
				mockProvider.AssertCalled(t, "CreateStructured", ctx, mock.Anything)
			}
		})
	}
}

// TestAggregator_DefaultStrategy tests that default strategy is still consensus
func TestAggregator_DefaultStrategy(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: AggregatorConfig{
			// No strategy specified - should default to consensus
			Temperature: 0.5,
			MaxTokens:   1500,
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Content A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Content B", Confidence: 0.9},
	}

	mockResult := AggregationResult{
		AggregatedContent: "Default consensus result",
	}
	resultJSON, _ := json.Marshal(mockResult)

	mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
		Data: resultJSON,
		CompletionResponse: provider.CompletionResponse{
			Usage: provider.Usage{TotalTokens: 200},
		},
	}, nil).Once()

	// aggregate() should use the config's strategy, which should be empty and default to consensus
	// This test assumes the aggregate() function handles empty strategy
	result, err := aggAgent.aggregate(ctx, inputs)

	require.NoError(t, err)
	assert.Greater(t, result.TokensUsed, 0, "Default strategy should use LLM")
}
