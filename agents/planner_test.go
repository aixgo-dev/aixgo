package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewPlannerAgent(t *testing.T) {
	// Skip this test as it requires provider initialization
	t.Skip("Skipping test that requires provider initialization")
}

func TestPlannerStrategies(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)
	rt := NewMockRuntime()

	plannerAgent := &PlannerAgent{
		def: agent.AgentDef{
			Model: "gpt-4",
		},
		provider: mockProvider,
		config: PlannerConfig{
			Temperature:    0.7,
			MaxTokens:     2000,
			MaxSteps:      20,
			ReasoningDepth: 3,
		},
		rt:            rt,
		planCache:     make(map[string]*ReasoningPlan),
		planHistory:   []PlanExecutionHistory{},
		metacognition: MetacognitionModule{
			LearningInsights: make(map[string]float64),
		},
	}

	problem := "Design a recommendation system for an e-commerce platform"

	// Test Chain-of-Thought Strategy
	t.Run("ChainOfThoughtStrategy", func(t *testing.T) {
		plannerAgent.config.PlanningStrategy = StrategyChainOfThought

		mockPlan := ReasoningPlan{
			Problem: problem,
			Analysis: ProblemAnalysis{
				Type:   "system_design",
				Domain: "e-commerce",
				Constraints: []string{"scalability", "real-time"},
				KeyChallenges: []string{"cold start", "data sparsity"},
			},
			Steps: []PlanStep{
				{
					StepNumber:      1,
					Action:          "Analyze user behavior data",
					Reasoning:       "Understanding patterns is foundational",
					ExpectedOutcome: "User segmentation and preferences",
					Complexity:      "medium",
					Confidence:      0.85,
				},
				{
					StepNumber:      2,
					Action:          "Select recommendation algorithm",
					Reasoning:       "Algorithm choice impacts performance",
					Prerequisites:   []int{1},
					ExpectedOutcome: "Chosen algorithm with justification",
					Complexity:      "high",
					Confidence:      0.8,
				},
			},
			SuccessCriteria: []string{"Accurate recommendations", "Low latency"},
			TotalComplexity: "high",
		}

		planJSON, _ := json.Marshal(mockPlan)

		mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
			Data: planJSON,
			CompletionResponse: provider.CompletionResponse{
				Usage: provider.Usage{TotalTokens: 500},
			},
		}, nil).Once()

		plan, err := plannerAgent.createPlan(ctx, problem)

		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, problem, plan.Problem)
		assert.Equal(t, 2, len(plan.Steps))
		assert.Equal(t, StrategyChainOfThought, plan.PlanningStrategy)
		assert.Equal(t, 500, plan.TokensUsed)
	})

	// Test Tree-of-Thought Strategy
	t.Run("TreeOfThoughtStrategy", func(t *testing.T) {
		// Clear cache to avoid pollution from previous test
		plannerAgent.planCache = make(map[string]*ReasoningPlan)
		plannerAgent.config.PlanningStrategy = StrategyTreeOfThought

		plan, err := plannerAgent.createPlan(ctx, problem)

		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, StrategyTreeOfThought, plan.PlanningStrategy)
	})

	// Test ReAct Planning Strategy
	t.Run("ReActStrategy", func(t *testing.T) {
		// Clear cache to avoid pollution from previous test
		plannerAgent.planCache = make(map[string]*ReasoningPlan)
		plannerAgent.config.PlanningStrategy = StrategyReActPlanning

		mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
			Content: "Thought: Need to analyze requirements\nAction: Gather user data\nObservation: Data collected",
			Usage:   provider.Usage{TotalTokens: 300},
		}, nil).Once()

		plan, err := plannerAgent.createPlan(ctx, problem)

		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, StrategyReActPlanning, plan.PlanningStrategy)
	})

	// Test Backward Chaining Strategy
	t.Run("BackwardChainingStrategy", func(t *testing.T) {
		// Clear cache to avoid pollution from previous test
		plannerAgent.planCache = make(map[string]*ReasoningPlan)
		plannerAgent.config.PlanningStrategy = StrategyBackwardChaining

		mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
			Content: "Goal: Working recommendation system\nStep -1: Deploy system\nStep -2: Test algorithms",
			Usage:   provider.Usage{TotalTokens: 250},
		}, nil).Once()

		plan, err := plannerAgent.createPlan(ctx, problem)

		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, StrategyBackwardChaining, plan.PlanningStrategy)
	})

	// Test Hierarchical Strategy
	t.Run("HierarchicalStrategy", func(t *testing.T) {
		// Clear cache to avoid pollution from previous test
		plannerAgent.planCache = make(map[string]*ReasoningPlan)
		plannerAgent.config.PlanningStrategy = StrategyHierarchicalPlan

		// Mock high-level plan
		highLevelPlan := ReasoningPlan{
			Steps: []PlanStep{
				{StepNumber: 1, Action: "Design architecture"},
				{StepNumber: 2, Action: "Implement core"},
			},
		}
		planJSON, _ := json.Marshal(highLevelPlan)

		mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
			Data: planJSON,
			CompletionResponse: provider.CompletionResponse{
				Usage: provider.Usage{TotalTokens: 400},
			},
		}, nil).Once()

		plan, err := plannerAgent.createPlan(ctx, problem)

		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, StrategyHierarchicalPlan, plan.PlanningStrategy)
	})

	// Test Monte Carlo Strategy
	t.Run("MonteCarloStrategy", func(t *testing.T) {
		// Clear cache to avoid pollution from previous test
		plannerAgent.planCache = make(map[string]*ReasoningPlan)
		plannerAgent.config.PlanningStrategy = StrategyMonteCarlo

		// Mock simulations
		simulationPlan := ReasoningPlan{
			Steps: []PlanStep{
				{StepNumber: 1, Action: "Simulation step", Confidence: 0.75},
			},
		}
		planJSON, _ := json.Marshal(simulationPlan)

		mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
			Data: planJSON,
			CompletionResponse: provider.CompletionResponse{
				Usage: provider.Usage{TotalTokens: 350},
			},
		}, nil)

		plan, err := plannerAgent.createPlan(ctx, problem)

		assert.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, StrategyMonteCarlo, plan.PlanningStrategy)
	})

	mockProvider.AssertExpectations(t)
}

func TestPlannerPromptBuilding(t *testing.T) {
	plannerAgent := &PlannerAgent{
		config: PlannerConfig{
			ExamplePlans: []ExamplePlan{
				{
					Problem:     "Build a REST API",
					Steps:       []string{"Design endpoints", "Implement handlers", "Add tests"},
					Explanation: "Standard API development flow",
				},
			},
		},
	}

	problem := "Create a machine learning pipeline"
	prompt := plannerAgent.buildChainOfThoughtPrompt(problem)

	// Verify prompt components
	assert.Contains(t, prompt, problem)
	assert.Contains(t, prompt, "Chain-of-Thought reasoning")
	assert.Contains(t, prompt, "Break down the problem")
	assert.Contains(t, prompt, "logical, sequential steps")
	assert.Contains(t, prompt, "Think step by step")

	// Verify examples are included
	assert.Contains(t, prompt, "Build a REST API")
	assert.Contains(t, prompt, "Design endpoints")
}

func TestPlannerSystemPrompts(t *testing.T) {
	plannerAgent := &PlannerAgent{}

	// Test Chain-of-Thought system prompt
	cotPrompt := plannerAgent.getChainOfThoughtSystemPrompt()
	assert.Contains(t, cotPrompt, "Chain-of-Thought reasoning")
	assert.Contains(t, cotPrompt, "Systematic problem decomposition")
	assert.Contains(t, cotPrompt, "step-by-step")

	// Test ReAct system prompt
	reactPrompt := plannerAgent.getReActSystemPrompt()
	assert.Contains(t, reactPrompt, "ReAct")
	assert.Contains(t, reactPrompt, "Thought")
	assert.Contains(t, reactPrompt, "Action")
	assert.Contains(t, reactPrompt, "Observation")

	// Test Backward Chaining system prompt
	backwardPrompt := plannerAgent.getBackwardChainingSystemPrompt()
	assert.Contains(t, backwardPrompt, "backward chaining")
	assert.Contains(t, backwardPrompt, "goal state")
}

func TestPlannerSchemaBuilding(t *testing.T) {
	plannerAgent := &PlannerAgent{}

	schemaJSON := plannerAgent.buildPlanSchema()

	var schema map[string]any
	err := json.Unmarshal(schemaJSON, &schema)

	assert.NoError(t, err)
	assert.Equal(t, "object", schema["type"])

	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "problem")
	assert.Contains(t, props, "analysis")
	assert.Contains(t, props, "steps")
	assert.Contains(t, props, "success_criteria")
	assert.Contains(t, props, "total_complexity")

	// Check step structure
	stepsSchema := props["steps"].(map[string]any)
	assert.Equal(t, "array", stepsSchema["type"])
	items := stepsSchema["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	assert.Contains(t, itemProps, "step_number")
	assert.Contains(t, itemProps, "action")
	assert.Contains(t, itemProps, "reasoning")
	assert.Contains(t, itemProps, "confidence")
}

func TestPlannerAnalysis(t *testing.T) {
	plannerAgent := &PlannerAgent{
		config: PlannerConfig{
			ParallelizableSteps: true,
		},
	}

	plan := &ReasoningPlan{
		Steps: []PlanStep{
			{StepNumber: 1, CanParallelize: false, Prerequisites: []int{}},
			{StepNumber: 2, CanParallelize: true, Prerequisites: []int{1}},
			{StepNumber: 3, CanParallelize: true, Prerequisites: []int{1}},
			{StepNumber: 4, CanParallelize: false, Prerequisites: []int{2, 3}},
		},
	}

	plannerAgent.analyzePlanStructure(plan)

	// Check critical path identification
	assert.NotNil(t, plan.CriticalPath)
	assert.Contains(t, plan.CriticalPath, 1)

	// Check parallel groups identification
	assert.NotNil(t, plan.ParallelGroups)

	// Check execution strategy
	if len(plan.ParallelGroups) > 0 {
		assert.Equal(t, "parallel_optimized", plan.ExecutionStrategy)
	} else {
		assert.Equal(t, "sequential", plan.ExecutionStrategy)
	}
}

func TestPlannerSelfCritique(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)

	plannerAgent := &PlannerAgent{
		def: agent.AgentDef{Model: "gpt-4"},
		provider: mockProvider,
		config: PlannerConfig{
			EnableSelfCritique: true,
		},
	}

	plan := &ReasoningPlan{
		Problem:          "Test problem",
		Steps:            []PlanStep{{StepNumber: 1, Action: "Test step"}},
		PlanningStrategy: StrategyChainOfThought,
	}

	mockProvider.On("CreateCompletion", ctx, mock.Anything).Return(&provider.CompletionResponse{
		Content: "Critique: Plan lacks error handling and could be more efficient",
		Usage:   provider.Usage{TotalTokens: 100},
	}, nil).Once()

	critique, err := plannerAgent.selfCritique(ctx, plan)

	assert.NoError(t, err)
	assert.Contains(t, critique, "error handling")
	assert.Contains(t, critique, "efficient")

	mockProvider.AssertExpectations(t)
}

func TestPlannerCaching(t *testing.T) {
	ctx := context.Background()
	mockProvider := new(MockProvider)

	plannerAgent := &PlannerAgent{
		def:       agent.AgentDef{Model: "gpt-4"},
		provider:  mockProvider,
		config:    PlannerConfig{PlanningStrategy: StrategyChainOfThought},
		planCache: make(map[string]*ReasoningPlan),
	}

	problem := "Cached problem"
	cachedPlan := &ReasoningPlan{
		Problem: problem,
		Steps:   []PlanStep{{StepNumber: 1, Action: "Cached step"}},
	}

	// Pre-populate cache
	plannerAgent.planCache[problem] = cachedPlan

	// Should use cached plan without calling provider
	plan, err := plannerAgent.createPlan(ctx, problem)

	assert.NoError(t, err)
	assert.Equal(t, cachedPlan, plan)

	// Provider should not be called
	mockProvider.AssertNotCalled(t, "CreateStructured")
}

func TestPlannerLearning(t *testing.T) {
	plannerAgent := &PlannerAgent{
		planHistory: []PlanExecutionHistory{},
		metacognition: MetacognitionModule{
			LearningInsights: make(map[string]float64),
		},
	}

	// Record successful plans
	for i := 0; i < 5; i++ {
		history := PlanExecutionHistory{
			PlanID:     fmt.Sprintf("plan_%d", i),
			Problem:    "optimize database queries",
			TotalSteps: 5,
			Success:    true,
			TokensUsed: 200 + i*10,
		}
		plannerAgent.planHistory = append(plannerAgent.planHistory, history)
	}

	// Update learning insights
	plannerAgent.updateLearningInsights()

	// Check that insights were updated
	assert.Greater(t, plannerAgent.metacognition.LearningInsights["optimize"], 0.0)

	// Test optimization with learning
	plan := &ReasoningPlan{
		Problem: "optimize API performance",
		Steps: []PlanStep{
			{StepNumber: 1, Confidence: 0.7},
			{StepNumber: 2, Confidence: 0.8},
		},
	}

	plannerAgent.optimizePlanWithLearning(plan)

	// Confidence should be adjusted based on insights
	assert.GreaterOrEqual(t, plan.Steps[0].Confidence, 0.7)
}

func TestPlannerFeatureExtraction(t *testing.T) {
	plannerAgent := &PlannerAgent{}

	testCases := []struct {
		problem  string
		expected []string
	}{
		{
			problem:  "Optimize the database performance",
			expected: []string{"optimize"},
		},
		{
			problem:  "Design and implement a new feature",
			expected: []string{"design", "implement"},
		},
		{
			problem:  "Debug and refactor the authentication module",
			expected: []string{"debug", "refactor"},
		},
		{
			problem:  "Analyze user behavior patterns",
			expected: []string{"analyze"},
		},
	}

	for _, tc := range testCases {
		features := plannerAgent.extractProblemFeatures(tc.problem)
		for _, exp := range tc.expected {
			assert.Contains(t, features, exp, "Problem: %s should contain feature: %s", tc.problem, exp)
		}
	}
}

func TestPlannerScoreCalculation(t *testing.T) {
	plannerAgent := &PlannerAgent{}

	plan1 := &ReasoningPlan{
		Steps: []PlanStep{
			{Confidence: 0.9},
			{Confidence: 0.8},
			{Confidence: 0.85},
		},
	}

	plan2 := &ReasoningPlan{
		Steps: []PlanStep{
			{Confidence: 0.7},
			{Confidence: 0.6},
			{Confidence: 0.65},
		},
	}

	score1 := plannerAgent.calculatePlanScore(plan1)
	score2 := plannerAgent.calculatePlanScore(plan2)

	assert.Greater(t, score1, score2)
	assert.InDelta(t, 0.85, score1, 0.01)
	assert.InDelta(t, 0.65, score2, 0.01)

	// Test empty plan
	emptyPlan := &ReasoningPlan{Steps: []PlanStep{}}
	emptyScore := plannerAgent.calculatePlanScore(emptyPlan)
	assert.Equal(t, 0.0, emptyScore)
}

func TestPlannerHistory(t *testing.T) {
	plannerAgent := &PlannerAgent{
		planHistory: []PlanExecutionHistory{},
	}

	plan := &ReasoningPlan{
		Problem:    "Test problem",
		Steps:      []PlanStep{{}, {}, {}},
		TokensUsed: 500,
	}

	// Record plan
	plannerAgent.recordPlanHistory(plan)

	assert.Equal(t, 1, len(plannerAgent.planHistory))
	history := plannerAgent.planHistory[0]
	assert.Equal(t, "Test problem", history.Problem)
	assert.Equal(t, 3, history.TotalSteps)
	assert.Equal(t, 500, history.TokensUsed)

	// Test history limit (should keep last 100)
	for i := 0; i < 105; i++ {
		plannerAgent.recordPlanHistory(plan)
	}

	assert.LessOrEqual(t, len(plannerAgent.planHistory), 100)
}

func TestPlannerMessageSending(t *testing.T) {
	rt := NewMockRuntime()
	rt.On("Send", mock.Anything, mock.Anything).Return(nil)

	plannerAgent := &PlannerAgent{
		def: agent.AgentDef{
			Outputs: []agent.Output{
				{Target: "output1"},
				{Target: "output2"},
			},
		},
		rt: rt,
	}

	plan := &ReasoningPlan{
		Problem: "Test",
		Steps:   []PlanStep{{Action: "Step1"}},
	}

	originalMsg := &agent.Message{
		Message: &pb.Message{Id: "msg123"},
	}

	plannerAgent.sendPlan(plan, originalMsg)

	// Verify Send was called for each output
	rt.AssertNumberOfCalls(t, "Send", 2)
	rt.AssertCalled(t, "Send", "output1", mock.Anything)
	rt.AssertCalled(t, "Send", "output2", mock.Anything)
}