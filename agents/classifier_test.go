package agents

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProvider for testing LLM interactions
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) CreateCompletion(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*provider.CompletionResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockProvider) CreateStructured(ctx context.Context, req provider.StructuredRequest) (*provider.StructuredResponse, error) {
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*provider.StructuredResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockProvider) CreateStreaming(ctx context.Context, req provider.CompletionRequest) (provider.Stream, error) {
	args := m.Called(ctx, req)
	if stream := args.Get(0); stream != nil {
		return stream.(provider.Stream), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockProvider) Name() string {
	return "mock"
}

// MockRuntime for testing agent communication
type MockRuntime struct {
	mock.Mock
	channels map[string]chan *agent.Message
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		channels: make(map[string]chan *agent.Message),
	}
}

func (m *MockRuntime) Send(target string, msg *agent.Message) error {
	args := m.Called(target, msg)
	if ch, exists := m.channels[target]; exists {
		ch <- msg
	}
	return args.Error(0)
}

func (m *MockRuntime) Recv(source string) (<-chan *agent.Message, error) {
	args := m.Called(source)
	if ch := args.Get(0); ch != nil {
		return ch.(<-chan *agent.Message), args.Error(1)
	}
	// Create a channel if not mocked
	ch := make(chan *agent.Message, 10)
	m.channels[source] = ch
	return ch, nil
}

func (m *MockRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
	return input, nil
}

func (m *MockRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, map[string]error) {
	results := make(map[string]*agent.Message)
	for _, t := range targets {
		results[t] = input
	}
	return results, nil
}

func (m *MockRuntime) Broadcast(msg *agent.Message) error {
	return nil
}

func (m *MockRuntime) Register(a agent.Agent) error {
	return nil
}

func (m *MockRuntime) Unregister(name string) error {
	return nil
}

func (m *MockRuntime) Get(name string) (agent.Agent, error) {
	return nil, agent.ErrAgentNotFound
}

func (m *MockRuntime) List() []string {
	return []string{}
}

func (m *MockRuntime) Start(ctx context.Context) error {
	return nil
}

func (m *MockRuntime) Stop(ctx context.Context) error {
	return nil
}

// Test Classifier Agent

func TestNewClassifierAgent(t *testing.T) {
	// Skip this test as it requires provider initialization
	t.Skip("Skipping test that requires provider initialization")
}

func TestClassifierAgentClassify(t *testing.T) {
	ctx := context.Background()

	// Setup
	def := agent.AgentDef{
		Name:  "test-classifier",
		Model: "gpt-4",
		Extra: map[string]any{
			"classifier_config": map[string]any{
				"categories": []map[string]any{
					{
						"name":        "technical",
						"description": "Technical content",
					},
					{
						"name":        "business",
						"description": "Business content",
					},
				},
			},
		},
	}

	rt := NewMockRuntime()
	mockProvider := new(MockProvider)

	// Create agent with mocked provider
	classifierAgent := &ClassifierAgent{
		def:      def,
		provider: mockProvider,
		config: ClassifierConfig{
			Categories: []Category{
				{Name: "technical", Description: "Technical content"},
				{Name: "business", Description: "Business content"},
			},
			Temperature:         0.3,
			MaxTokens:          500,
			ConfidenceThreshold: 0.7,
		},
		rt:              rt,
		promptCache:     make(map[string]string),
		performanceData: []ClassificationMetrics{},
	}

	// Setup mock response
	mockResult := ClassificationResult{
		Category:   "technical",
		Confidence: 0.85,
		Reasoning:  "Contains programming terminology",
	}

	resultJSON, _ := json.Marshal(mockResult)

	mockProvider.On("CreateStructured", ctx, mock.Anything).Return(&provider.StructuredResponse{
		Data: resultJSON,
		CompletionResponse: provider.CompletionResponse{
			Usage: provider.Usage{
				TotalTokens: 150,
			},
		},
	}, nil)

	// Test classification
	result, err := classifierAgent.classify(ctx, "How to implement a binary search algorithm in Python")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "technical", result.Category)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Equal(t, 150, result.TokensUsed)

	mockProvider.AssertExpectations(t)
}

func TestClassifierPromptBuilding(t *testing.T) {
	classifierAgent := &ClassifierAgent{
		config: ClassifierConfig{
			Categories: []Category{
				{
					Name:        "spam",
					Description: "Spam or unwanted content",
					Keywords:    []string{"buy now", "click here"},
					Examples:    []string{"Get rich quick!", "Limited time offer"},
				},
				{
					Name:        "legitimate",
					Description: "Legitimate content",
					Keywords:    []string{"information", "update"},
				},
			},
			FewShotExamples: []Example{
				{
					Input:    "Buy now and save 50%!",
					Category: "spam",
					Reason:   "Promotional language with urgency",
				},
			},
		},
		promptCache: make(map[string]string),
	}

	prompt := classifierAgent.buildClassificationPrompt("Check out this new product")

	// Verify prompt contains categories
	assert.Contains(t, prompt, "spam")
	assert.Contains(t, prompt, "legitimate")

	// Verify few-shot examples are included
	assert.Contains(t, prompt, "Buy now and save 50%!")
	assert.Contains(t, prompt, "Promotional language with urgency")

	// Verify Chain-of-Thought instructions
	assert.Contains(t, prompt, "Think step by step")
	assert.Contains(t, prompt, "Identify key features")
}

func TestClassifierResponseSchema(t *testing.T) {
	classifierAgent := &ClassifierAgent{
		config: ClassifierConfig{
			Categories: []Category{
				{Name: "category1"},
				{Name: "category2"},
			},
		},
	}

	schemaJSON := classifierAgent.buildResponseSchema()

	var schema map[string]any
	err := json.Unmarshal(schemaJSON, &schema)

	assert.NoError(t, err)
	assert.Equal(t, "object", schema["type"])

	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "category")
	assert.Contains(t, props, "confidence")
	assert.Contains(t, props, "reasoning")
	assert.Contains(t, props, "alternatives")

	// Check enum for categories
	categoryProp := props["category"].(map[string]any)
	enum := categoryProp["enum"].([]any)
	assert.Contains(t, enum, "category1")
	assert.Contains(t, enum, "category2")
}

func TestClassifierPerformanceTracking(t *testing.T) {
	classifierAgent := &ClassifierAgent{
		performanceData: []ClassificationMetrics{},
	}

	// Record multiple metrics
	for i := 0; i < 5; i++ {
		classifierAgent.recordMetrics(ClassificationMetrics{
			Timestamp:       time.Now(),
			InputLength:     100 + i*10,
			ResponseLatency: time.Duration(100+i) * time.Millisecond,
			TokensUsed:      150 + i*5,
			Confidence:      0.7 + float64(i)*0.05,
			Success:         i%2 == 0,
		})
	}

	assert.Equal(t, 5, len(classifierAgent.performanceData))

	// Verify metrics are being tracked
	lastMetric := classifierAgent.performanceData[4]
	assert.Equal(t, 140, lastMetric.InputLength)
	assert.Equal(t, 170, lastMetric.TokensUsed)
	assert.InDelta(t, 0.9, lastMetric.Confidence, 0.0001)
}

func TestClassifierCaching(t *testing.T) {
	classifierAgent := &ClassifierAgent{
		config: ClassifierConfig{
			Categories: []Category{
				{Name: "cat1"},
			},
		},
		promptCache: make(map[string]string),
	}

	input := "Test input for caching"

	// First call - should create and cache
	prompt1 := classifierAgent.buildClassificationPrompt(input)
	assert.Equal(t, 1, len(classifierAgent.promptCache))

	// Second call - should use cache
	prompt2 := classifierAgent.buildClassificationPrompt(input)
	assert.Equal(t, prompt1, prompt2)
	assert.Equal(t, 1, len(classifierAgent.promptCache))
}

func TestClassifierPromptStrategies(t *testing.T) {
	// Test zero-shot
	zeroShotAgent := &ClassifierAgent{
		config: ClassifierConfig{
			FewShotExamples: []Example{},
		},
	}
	assert.Equal(t, "zero-shot", zeroShotAgent.getPromptStrategy())

	// Test few-shot
	fewShotAgent := &ClassifierAgent{
		config: ClassifierConfig{
			FewShotExamples: []Example{
				{Input: "example", Category: "cat", Reason: "reason"},
			},
		},
	}
	assert.Equal(t, "few-shot", fewShotAgent.getPromptStrategy())
}

func TestClassifierSystemPrompt(t *testing.T) {
	// Test default system prompt
	agent1 := &ClassifierAgent{
		def: agent.AgentDef{},
	}
	prompt1 := agent1.getSystemPrompt()
	assert.Contains(t, prompt1, "expert classification AI")
	assert.Contains(t, prompt1, "semantic understanding")

	// Test with custom prompt
	agent2 := &ClassifierAgent{
		def: agent.AgentDef{
			Prompt: "Custom context for classification",
		},
	}
	prompt2 := agent2.getSystemPrompt()
	assert.Contains(t, prompt2, "Custom context for classification")
	assert.Contains(t, prompt2, "expert classification AI")
}