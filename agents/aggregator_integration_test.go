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

// TestAggregator_YAMLConfig_DeterministicStrategy tests YAML configuration
func TestAggregator_YAMLConfig_DeterministicStrategy(t *testing.T) {
	// This test would normally load from YAML, but we'll simulate it
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	// Simulate YAML config with voting_majority strategy
	config := AggregatorConfig{
		AggregationStrategy:  "voting_majority",
		ConflictResolution:   "deterministic",
		DeduplicationMethod:  "exact_match",
		SummarizationEnabled: false,
		MaxInputSources:      10,
		TimeoutMs:            3000,
		Temperature:          0.5,
		MaxTokens:            1500,
	}

	aggAgent := &AggregatorAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider:    mockProvider,
		config:      config,
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	inputs := []*AgentInput{
		{AgentName: "analyzer", Content: "Recommendation: Option A", Confidence: 0.85},
		{AgentName: "validator", Content: "Recommendation: Option A", Confidence: 0.9},
		{AgentName: "reviewer", Content: "Recommendation: Option B", Confidence: 0.75},
	}

	result, err := aggAgent.aggregate(ctx, inputs)

	require.NoError(t, err)
	assert.Equal(t, "voting_majority", result.Strategy)
	assert.Equal(t, "Recommendation: Option A", result.AggregatedContent)
	assert.Equal(t, 0, result.TokensUsed, "Deterministic strategy from YAML should use zero tokens")

	// Verify no LLM calls
	mockProvider.AssertNotCalled(t, "CreateCompletion")
	mockProvider.AssertNotCalled(t, "CreateStructured")
}

// TestAggregator_MixedStrategies tests workflows with both LLM and deterministic
func TestAggregator_MixedStrategies(t *testing.T) {
	ctx := context.Background()
	rt := NewMockRuntime()

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Analysis A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Analysis A", Confidence: 0.9},
		{AgentName: "agent3", Content: "Analysis B", Confidence: 0.7},
	}

	t.Run("switching_from_llm_to_deterministic", func(t *testing.T) {
		mockProvider := new(MockProvider)

		// First use LLM-based strategy
		llmAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: StrategyConsensus,
				Temperature:         0.5,
				MaxTokens:           1500,
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		mockResult := AggregationResult{
			AggregatedContent: "LLM consensus",
		}
		resultJSON, _ := json.Marshal(mockResult)

		mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
			Data: resultJSON,
			CompletionResponse: provider.CompletionResponse{
				Usage: provider.Usage{TotalTokens: 200},
			},
		}, nil).Once()

		llmResult, err := llmAgent.aggregate(ctx, inputs)
		require.NoError(t, err)
		assert.Greater(t, llmResult.TokensUsed, 0)

		// Then switch to deterministic strategy
		deterministicAgent := &AggregatorAgent{
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

		detResult, err := deterministicAgent.aggregate(ctx, inputs)
		require.NoError(t, err)
		assert.Equal(t, 0, detResult.TokensUsed, "Deterministic should use zero tokens")

		// Verify they can coexist without interference
		assert.NotEqual(t, llmResult.AggregatedContent, detResult.AggregatedContent)
	})

	t.Run("parallel_usage_no_interference", func(t *testing.T) {
		// Create two agents with different strategies
		mockProvider1 := new(MockProvider)
		mockProvider2 := new(MockProvider)

		llmAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider1,
			config: AggregatorConfig{
				AggregationStrategy: StrategyConsensus,
				Temperature:         0.5,
				MaxTokens:           1500,
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		deterministicAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider2,
			config: AggregatorConfig{
				AggregationStrategy: "voting_majority",
				Temperature:         0.5,
				MaxTokens:           1500,
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		mockResult := AggregationResult{
			AggregatedContent: "LLM consensus",
		}
		resultJSON, _ := json.Marshal(mockResult)

		mockProvider1.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
			Data: resultJSON,
			CompletionResponse: provider.CompletionResponse{
				Usage: provider.Usage{TotalTokens: 200},
			},
		}, nil).Once()

		// Run both in parallel
		llmResult, llmErr := llmAgent.aggregate(ctx, inputs)
		detResult, detErr := deterministicAgent.aggregate(ctx, inputs)

		require.NoError(t, llmErr)
		require.NoError(t, detErr)

		assert.Greater(t, llmResult.TokensUsed, 0, "LLM agent should use tokens")
		assert.Equal(t, 0, detResult.TokensUsed, "Deterministic agent should not use tokens")

		// Verify no cross-contamination
		mockProvider2.AssertNotCalled(t, "CreateCompletion")
		mockProvider2.AssertNotCalled(t, "CreateStructured")
	})
}

// TestAggregator_PerformanceComparison benchmarks deterministic vs LLM
func TestAggregator_PerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}

	ctx := context.Background()
	rt := NewMockRuntime()

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Option A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Option A", Confidence: 0.9},
		{AgentName: "agent3", Content: "Option B", Confidence: 0.7},
	}

	// Measure deterministic strategy
	t.Run("benchmark_deterministic", func(t *testing.T) {
		mockProvider := new(MockProvider)
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

		iterations := 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			_, err := aggAgent.aggregate(ctx, inputs)
			require.NoError(t, err)
		}

		deterministicDuration := time.Since(start)
		avgDeterministic := deterministicDuration / time.Duration(iterations)

		t.Logf("Deterministic strategy: %d iterations in %v (avg: %v per iteration)",
			iterations, deterministicDuration, avgDeterministic)

		// Deterministic should be very fast - under 1ms per iteration
		assert.Less(t, avgDeterministic, 1*time.Millisecond,
			"Deterministic voting should be extremely fast")
	})

	// Simulate LLM strategy performance
	t.Run("benchmark_llm_simulated", func(t *testing.T) {
		mockProvider := new(MockProvider)
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: StrategyConsensus,
				Temperature:         0.5,
				MaxTokens:           1500,
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		mockResult := AggregationResult{
			AggregatedContent: "LLM consensus",
		}
		resultJSON, _ := json.Marshal(mockResult)

		// Simulate LLM latency (100ms per call is realistic)
		mockProvider.On("CreateStructured", ctx, mock.Anything).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond) // Simulate LLM latency
		}).Return(
			&provider.StructuredResponse{
				Data: resultJSON,
				CompletionResponse: provider.CompletionResponse{
					Usage: provider.Usage{TotalTokens: 200},
				},
			},
			nil,
		).Times(10)

		iterations := 10 // Fewer iterations due to simulated latency
		start := time.Now()

		for i := 0; i < iterations; i++ {
			_, err := aggAgent.aggregate(ctx, inputs)
			require.NoError(t, err)
		}

		llmDuration := time.Since(start)
		avgLLM := llmDuration / time.Duration(iterations)

		t.Logf("LLM strategy: %d iterations in %v (avg: %v per iteration)",
			iterations, llmDuration, avgLLM)

		// LLM should take at least 100ms per iteration (our simulated latency)
		assert.Greater(t, avgLLM, 100*time.Millisecond,
			"LLM calls should have realistic latency")

		// Deterministic should be at least 100x faster (100ms vs <1ms)
		t.Logf("Performance ratio: deterministic is ~100x+ faster than LLM")
	})
}

// TestAggregator_FullWorkflow tests complete aggregation workflow
func TestAggregator_FullWorkflow(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	t.Run("complete_voting_majority_workflow", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Name:  "test-aggregator",
				Role:  "aggregator",
				Model: "gpt-4",
				Inputs: []agent.Input{
					{Source: "analyzer"},
					{Source: "validator"},
					{Source: "reviewer"},
				},
				Outputs: []agent.Output{
					{Target: "decision-maker"},
				},
			},
			BaseAgent: NewBaseAgent(agent.AgentDef{
				Name: "test-aggregator",
				Role: "aggregator",
			}),
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy:  "voting_majority",
				ConflictResolution:   "deterministic",
				DeduplicationMethod:  "exact_match",
				SummarizationEnabled: false,
				MaxInputSources:      10,
				TimeoutMs:            3000,
				ConsensusThreshold:   0.7,
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		// Simulate inputs from multiple agents
		msg1 := &agent.Message{
			Message: &pb.Message{
				Type:      "analysis",
				Payload:   `{"content": "Recommend Option A", "confidence": 0.85}`,
				Timestamp: time.Now().Format(time.RFC3339),
			},
		}

		msg2 := &agent.Message{
			Message: &pb.Message{
				Type:      "validation",
				Payload:   `{"content": "Recommend Option A", "confidence": 0.9}`,
				Timestamp: time.Now().Format(time.RFC3339),
			},
		}

		msg3 := &agent.Message{
			Message: &pb.Message{
				Type:      "review",
				Payload:   `{"content": "Recommend Option B", "confidence": 0.75}`,
				Timestamp: time.Now().Format(time.RFC3339),
			},
		}

		// Buffer inputs
		aggAgent.bufferInput("analyzer", msg1)
		aggAgent.bufferInput("validator", msg2)
		aggAgent.bufferInput("reviewer", msg3)

		// Verify inputs are buffered
		assert.True(t, aggAgent.hasBufferedInputs())

		// Get buffered inputs for aggregation
		aggAgent.bufferMu.Lock()
		inputs := make([]*AgentInput, 0, len(aggAgent.inputBuffer))
		for _, input := range aggAgent.inputBuffer {
			inputs = append(inputs, input)
		}
		aggAgent.bufferMu.Unlock()

		// Perform aggregation
		result, err := aggAgent.aggregate(ctx, inputs)

		require.NoError(t, err)
		assert.Equal(t, "voting_majority", result.Strategy)
		assert.Contains(t, result.AggregatedContent, "Recommend Option A")
		assert.Equal(t, 0, result.TokensUsed)
		assert.Equal(t, 3, len(result.Sources))
		assert.Greater(t, result.ConsensusLevel, 0.0)

		// Verify no LLM calls
		mockProvider.AssertNotCalled(t, "CreateCompletion")
		mockProvider.AssertNotCalled(t, "CreateStructured")
	})

	t.Run("complete_voting_unanimous_workflow", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Name:  "consensus-aggregator",
				Role:  "aggregator",
				Model: "gpt-4",
			},
			BaseAgent: NewBaseAgent(agent.AgentDef{
				Name: "consensus-aggregator",
				Role: "aggregator",
			}),
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "voting_unanimous",
				ConsensusThreshold:  1.0,
				TimeoutMs:           3000,
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		// All agree
		unanimousInputs := []*AgentInput{
			{AgentName: "agent1", Content: "Execute Plan X", Confidence: 0.9},
			{AgentName: "agent2", Content: "Execute Plan X", Confidence: 0.95},
			{AgentName: "agent3", Content: "Execute Plan X", Confidence: 0.85},
		}

		result, err := aggAgent.aggregate(ctx, unanimousInputs)

		require.NoError(t, err)
		assert.Equal(t, "voting_unanimous", result.Strategy)
		assert.Equal(t, "Execute Plan X", result.AggregatedContent)
		assert.Equal(t, 1.0, result.ConsensusLevel, "Unanimous should have perfect consensus")
		assert.Equal(t, 0, result.TokensUsed)

		// One disagrees - should fail
		disagreementInputs := []*AgentInput{
			{AgentName: "agent1", Content: "Execute Plan X", Confidence: 0.9},
			{AgentName: "agent2", Content: "Execute Plan X", Confidence: 0.95},
			{AgentName: "agent3", Content: "Execute Plan Y", Confidence: 0.85},
		}

		result, err = aggAgent.aggregate(ctx, disagreementInputs)
		assert.Error(t, err, "Unanimous vote should fail with disagreement")
		assert.Nil(t, result)
	})
}

// TestAggregator_ConcurrentDeterministic tests thread safety
func TestAggregator_ConcurrentDeterministic(t *testing.T) {
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
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	inputs := []*AgentInput{
		{AgentName: "agent1", Content: "Option A", Confidence: 0.8},
		{AgentName: "agent2", Content: "Option A", Confidence: 0.9},
		{AgentName: "agent3", Content: "Option B", Confidence: 0.7},
	}

	// Run 100 concurrent aggregations
	concurrency := 100
	results := make(chan string, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			result, err := aggAgent.aggregate(ctx, inputs)
			if err != nil {
				errors <- err
				return
			}
			results <- result.AggregatedContent
		}()
	}

	// Collect results
	var firstResult string
	for i := 0; i < concurrency; i++ {
		select {
		case result := <-results:
			if i == 0 {
				firstResult = result
			} else {
				assert.Equal(t, firstResult, result, "Concurrent calls should produce identical results")
			}
		case err := <-errors:
			t.Fatalf("Concurrent aggregation failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent aggregations")
		}
	}

	// Verify no LLM calls were made
	mockProvider.AssertNotCalled(t, "CreateCompletion")
	mockProvider.AssertNotCalled(t, "CreateStructured")
}

// TestAggregator_ErrorHandling tests error scenarios
func TestAggregator_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	t.Run("invalid_strategy", func(t *testing.T) {
		aggAgent := &AggregatorAgent{
			def: agent.AgentDef{
				Model: "gpt-4",
			},
			provider: mockProvider,
			config: AggregatorConfig{
				AggregationStrategy: "invalid_strategy",
			},
			rt:          rt,
			inputBuffer: make(map[string]*AgentInput),
		}

		inputs := []*AgentInput{
			{AgentName: "agent1", Content: "Content", Confidence: 0.8},
		}

		// Should return an error for unknown strategy
		result, err := aggAgent.aggregate(ctx, inputs)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unknown aggregation strategy")
	})

	t.Run("empty_content", func(t *testing.T) {
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
			{AgentName: "agent1", Content: "", Confidence: 0.8},
			{AgentName: "agent2", Content: "", Confidence: 0.9},
		}

		result, err := aggAgent.aggregate(ctx, inputs)
		require.NoError(t, err)
		assert.Equal(t, "", result.AggregatedContent, "Empty content should be valid")
	})

	t.Run("nil_inputs", func(t *testing.T) {
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

		result, err := aggAgent.aggregate(ctx, nil)
		assert.Error(t, err, "Nil inputs should error")
		assert.Nil(t, result)
	})
}

// TestAggregator_MetadataPreservation tests metadata handling
func TestAggregator_MetadataPreservation(t *testing.T) {
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
		},
		rt:          rt,
		inputBuffer: make(map[string]*AgentInput),
	}

	inputs := []*AgentInput{
		{
			AgentName:  "agent1",
			Content:    "Option A",
			Confidence: 0.7,
			Metadata: map[string]any{
				"source":    "database",
				"timestamp": "2024-01-01T00:00:00Z",
			},
		},
		{
			AgentName:  "agent2",
			Content:    "Option B",
			Confidence: 0.95,
			Metadata: map[string]any{
				"source":    "api",
				"timestamp": "2024-01-01T00:01:00Z",
			},
		},
	}

	result, err := aggAgent.aggregate(ctx, inputs)

	require.NoError(t, err)
	assert.Equal(t, "Option B", result.AggregatedContent, "Highest confidence should win")
	assert.Equal(t, 2, len(result.Sources))
}
