package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/security"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// ClassifierConfig holds AI-specific configuration for classification
type ClassifierConfig struct {
	Categories          []Category `yaml:"categories"`
	UseEmbeddings       bool       `yaml:"use_embeddings"`
	ConfidenceThreshold float64    `yaml:"confidence_threshold"`
	MultiLabel          bool       `yaml:"multi_label"`
	FewShotExamples     []Example  `yaml:"few_shot_examples"`
	Temperature         float64    `yaml:"temperature"`
	MaxTokens           int        `yaml:"max_tokens"`
}

// Category represents a classification category with metadata for better LLM understanding
type Category struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Keywords    []string `yaml:"keywords"`
	Examples    []string `yaml:"examples"`
}

// Example for few-shot learning
type Example struct {
	Input    string `yaml:"input"`
	Category string `yaml:"category"`
	Reason   string `yaml:"reason"`
}

// ClassificationResult with AI-specific metrics
type ClassificationResult struct {
	Category       string             `json:"category"`
	Confidence     float64            `json:"confidence"`
	Reasoning      string             `json:"reasoning"`
	Alternatives   []AlternativeClass `json:"alternatives,omitempty"`
	TokensUsed     int                `json:"tokens_used"`
	PromptStrategy string             `json:"prompt_strategy"`
}

type AlternativeClass struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

// ClassifierAgent implements AI-powered content classification
type ClassifierAgent struct {
	def      agent.AgentDef
	provider provider.Provider
	config   ClassifierConfig
	rt       agent.Runtime

	// AI-specific optimization fields
	promptCache     map[string]string
	categoryEmbeds  map[string][]float64
	performanceData []ClassificationMetrics

	// State management
	ready  bool
	ctx    context.Context
	cancel context.CancelFunc
}

// ClassificationMetrics for AI observability
type ClassificationMetrics struct {
	Timestamp       time.Time
	InputLength     int
	ResponseLatency time.Duration
	TokensUsed      int
	Confidence      float64
	Success         bool
}

func init() {
	agent.Register("classifier", NewClassifierAgent)
}

// NewClassifierAgent creates a new AI-powered classifier agent
func NewClassifierAgent(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
	var config ClassifierConfig
	if err := def.UnmarshalKey("classifier_config", &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal classifier config: %w", err)
	}

	// Set AI-optimized defaults
	if config.Temperature == 0 {
		config.Temperature = 0.3 // Lower temperature for consistent classification
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 500 // Sufficient for classification reasoning
	}
	if config.ConfidenceThreshold == 0 {
		config.ConfidenceThreshold = 0.7
	}

	// Initialize provider based on model
	prov, err := initializeProvider(def.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	return &ClassifierAgent{
		def:             def,
		provider:        prov,
		config:          config,
		rt:              rt,
		promptCache:     make(map[string]string),
		categoryEmbeds:  make(map[string][]float64),
		performanceData: make([]ClassificationMetrics, 0, 1000),
		ready:           true,
	}, nil
}

// Name returns the agent name
func (c *ClassifierAgent) Name() string {
	return c.def.Name
}

// Role returns the agent role
func (c *ClassifierAgent) Role() string {
	return c.def.Role
}

// Ready returns whether the agent is ready
func (c *ClassifierAgent) Ready() bool {
	return c.ready
}

// Stop gracefully stops the agent
func (c *ClassifierAgent) Stop(ctx context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.ready = false
	return nil
}

// Execute performs synchronous classification
func (c *ClassifierAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	if !c.ready {
		return nil, fmt.Errorf("agent not ready")
	}

	// Input validation for security
	validator := &security.StringValidator{
		MaxLength:            100000,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
	}

	if err := validator.Validate(input.Payload); err != nil {
		return nil, fmt.Errorf("input validation error: %w", err)
	}

	span := observability.StartSpan("classifier.execute", map[string]any{
		"input_length": len(input.Payload),
		"categories":   len(c.config.Categories),
	})
	defer span.End()

	result, err := c.classify(ctx, input.Payload)
	if err != nil {
		return nil, err
	}

	// Convert result to message
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &agent.Message{Message: &pb.Message{
		Id:        input.Id,
		Type:      "classification",
		Payload:   string(resultJSON),
		Timestamp: time.Now().Format(time.RFC3339),
	}}, nil
}

// Start begins the classification agent's processing loop (async mode)
func (c *ClassifierAgent) Start(ctx context.Context) error {
	if len(c.def.Inputs) == 0 {
		return fmt.Errorf("no inputs defined for ClassifierAgent")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	ch, err := c.rt.Recv(c.def.Inputs[0].Source)
	if err != nil {
		return fmt.Errorf("failed to receive from %s: %w", c.def.Inputs[0].Source, err)
	}

	// Input validation for security
	validator := &security.StringValidator{
		MaxLength:            100000,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
	}

	for {
		select {
		case <-c.ctx.Done():
			c.ready = false
			return c.ctx.Err()
		case m, ok := <-ch:
			if !ok {
				c.ready = false
				return nil
			}

			if err := validator.Validate(m.Payload); err != nil {
				log.Printf("Classifier input validation error: %v", err)
				continue
			}

			span := observability.StartSpan("classifier.classify", map[string]any{
				"input_length": len(m.Payload),
				"categories":   len(c.config.Categories),
			})

			result, err := c.classify(c.ctx, m.Payload)
			span.End()

			if err != nil {
				log.Printf("Classification error: %v", err)
				continue
			}

			c.sendResult(result, m)
		}
	}
}

// classify performs AI-powered classification with advanced prompting
func (c *ClassifierAgent) classify(ctx context.Context, input string) (*ClassificationResult, error) {
	startTime := time.Now()

	// Build optimized prompt using Chain-of-Thought for reasoning
	prompt := c.buildClassificationPrompt(input)

	// Prepare structured request for better LLM output
	schema := c.buildResponseSchema()

	req := provider.StructuredRequest{
		CompletionRequest: provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "system", Content: c.getSystemPrompt()},
				{Role: "user", Content: prompt},
			},
			Model:       c.def.Model,
			Temperature: c.config.Temperature,
			MaxTokens:   c.config.MaxTokens,
		},
		ResponseSchema: schema,
		StrictSchema:   true,
	}

	// Call LLM with structured output
	resp, err := c.provider.CreateStructured(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM classification failed: %w", err)
	}

	// Parse structured response
	var result ClassificationResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse classification result: %w", err)
	}

	// Add AI metrics
	result.TokensUsed = resp.Usage.TotalTokens
	result.PromptStrategy = c.getPromptStrategy()

	// Record performance metrics for optimization
	c.recordMetrics(ClassificationMetrics{
		Timestamp:       startTime,
		InputLength:     len(input),
		ResponseLatency: time.Since(startTime),
		TokensUsed:      resp.Usage.TotalTokens,
		Confidence:      result.Confidence,
		Success:         result.Confidence >= c.config.ConfidenceThreshold,
	})

	return &result, nil
}

// buildClassificationPrompt creates an optimized prompt with few-shot examples
func (c *ClassifierAgent) buildClassificationPrompt(input string) string {
	// Check cache first
	if cached, exists := c.promptCache[input]; exists {
		return cached
	}

	prompt := fmt.Sprintf(`Classify the following text into one of the predefined categories.

Categories:
%s

%s

Text to classify:
"%s"

Think step by step:
1. Identify key features and context
2. Match against category descriptions and keywords
3. Consider confidence level
4. Provide reasoning for your choice

Return a structured JSON response.`,
		c.formatCategories(),
		c.formatFewShotExamples(),
		input)

	// Cache the prompt for reuse
	c.promptCache[input] = prompt
	return prompt
}

// getSystemPrompt returns the AI-optimized system prompt
func (c *ClassifierAgent) getSystemPrompt() string {
	basePrompt := `You are an expert classification AI agent. Your task is to accurately categorize content based on semantic understanding and pattern recognition.

Key responsibilities:
- Analyze content deeply for semantic meaning
- Consider context and nuance
- Provide confidence scores based on certainty
- Explain reasoning transparently
- Handle edge cases with alternative classifications`

	if c.def.Prompt != "" {
		return fmt.Sprintf("%s\n\nAdditional context: %s", basePrompt, c.def.Prompt)
	}
	return basePrompt
}

// formatCategories formats categories with AI-friendly descriptions
func (c *ClassifierAgent) formatCategories() string {
	var result string
	for _, cat := range c.config.Categories {
		result += fmt.Sprintf("\n- %s: %s", cat.Name, cat.Description)
		if len(cat.Keywords) > 0 {
			result += fmt.Sprintf("\n  Keywords: %v", cat.Keywords)
		}
		if len(cat.Examples) > 0 {
			result += fmt.Sprintf("\n  Examples: %v", cat.Examples[:min(2, len(cat.Examples))])
		}
	}
	return result
}

// formatFewShotExamples formats few-shot learning examples
func (c *ClassifierAgent) formatFewShotExamples() string {
	if len(c.config.FewShotExamples) == 0 {
		return ""
	}

	result := "\nExamples for reference:"
	for i, ex := range c.config.FewShotExamples {
		if i >= 3 { // Limit to 3 examples to save tokens
			break
		}
		result += fmt.Sprintf("\n\nExample %d:\nInput: %s\nCategory: %s\nReasoning: %s",
			i+1, ex.Input, ex.Category, ex.Reason)
	}
	return result
}

// buildResponseSchema creates JSON schema for structured output
func (c *ClassifierAgent) buildResponseSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category": map[string]any{
				"type": "string",
				"enum": c.getCategoryNames(),
			},
			"confidence": map[string]any{
				"type":    "number",
				"minimum": 0,
				"maximum": 1,
			},
			"reasoning": map[string]any{
				"type": "string",
			},
			"alternatives": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category":   map[string]any{"type": "string"},
						"confidence": map[string]any{"type": "number"},
					},
				},
			},
		},
		"required": []string{"category", "confidence", "reasoning"},
	}

	data, _ := json.Marshal(schema)
	return data
}

// getCategoryNames extracts category names for schema
func (c *ClassifierAgent) getCategoryNames() []string {
	names := make([]string, len(c.config.Categories))
	for i, cat := range c.config.Categories {
		names[i] = cat.Name
	}
	return names
}

// getPromptStrategy returns the current prompting strategy
func (c *ClassifierAgent) getPromptStrategy() string {
	if len(c.config.FewShotExamples) > 0 {
		return "few-shot"
	}
	return "zero-shot"
}

// recordMetrics records performance metrics for AI observability
func (c *ClassifierAgent) recordMetrics(metrics ClassificationMetrics) {
	c.performanceData = append(c.performanceData, metrics)

	// Keep only last 1000 records for memory efficiency
	if len(c.performanceData) > 1000 {
		c.performanceData = c.performanceData[len(c.performanceData)-1000:]
	}

	// Log performance insights periodically
	if len(c.performanceData)%100 == 0 {
		c.logPerformanceInsights()
	}
}

// logPerformanceInsights analyzes and logs AI performance
func (c *ClassifierAgent) logPerformanceInsights() {
	if len(c.performanceData) == 0 {
		return
	}

	var totalTokens, successCount int
	var totalLatency time.Duration
	var avgConfidence float64

	for _, m := range c.performanceData {
		totalTokens += m.TokensUsed
		totalLatency += m.ResponseLatency
		avgConfidence += m.Confidence
		if m.Success {
			successCount++
		}
	}

	n := len(c.performanceData)
	log.Printf("Classifier Performance Insights: Avg Tokens: %d, Avg Latency: %v, Success Rate: %.2f%%, Avg Confidence: %.2f",
		totalTokens/n, totalLatency/time.Duration(n), float64(successCount)*100/float64(n), avgConfidence/float64(n))
}

// sendResult sends classification results to outputs
func (c *ClassifierAgent) sendResult(result *ClassificationResult, originalMsg *agent.Message) {
	resultJSON, _ := json.Marshal(result)

	out := &agent.Message{Message: &pb.Message{
		Id:        originalMsg.Id,
		Type:      "classification",
		Payload:   string(resultJSON),
		Timestamp: time.Now().Format(time.RFC3339),
	}}

	for _, o := range c.def.Outputs {
		if err := c.rt.Send(o.Target, out); err != nil {
			log.Printf("Error sending classification to %s: %v", o.Target, err)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
