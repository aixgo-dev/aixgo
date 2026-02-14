package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	ctxmgr "github.com/aixgo-dev/aixgo/internal/llm/context"
	"github.com/aixgo-dev/aixgo/internal/llm/inference"
	"github.com/aixgo-dev/aixgo/internal/llm/parser"
	"github.com/aixgo-dev/aixgo/internal/llm/prompt"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
)

// OptimizedHuggingFaceProvider implements an optimized LLM provider for HuggingFace models
type OptimizedHuggingFaceProvider struct {
	inference      inference.InferenceService
	mcpClient      *mcp.Client
	mcpSessions    map[string]*mcp.Session
	toolRegistry   *mcp.ToolRegistry
	model          string
	contextManager *ctxmgr.ContextManager
	parser         *parser.ReActParser
	template       *prompt.ReActTemplate
	modelConfig    *prompt.ModelConfig
	cache          *ResponseCache
	metrics        *PerformanceMetrics
}

// ResponseCache caches recent completions for faster responses
type ResponseCache struct {
	entries map[string]*CacheEntry
	maxSize int
	ttl     time.Duration
	mu      sync.RWMutex
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Response  *CompletionResponse
	Timestamp time.Time
}

// PerformanceMetrics tracks performance metrics
type PerformanceMetrics struct {
	TotalRequests    int64
	SuccessfulCalls  int64
	FailedCalls      int64
	CacheHits        int64
	AverageLatency   time.Duration
	TokensProcessed  int64
	ToolCallAccuracy float64
	mu               sync.RWMutex
}

// NewOptimizedHuggingFaceProvider creates a new optimized provider
func NewOptimizedHuggingFaceProvider(inf inference.InferenceService, model string) *OptimizedHuggingFaceProvider {
	modelConfig := prompt.GetModelConfig(model)
	template := prompt.GetReActTemplate(model)

	return &OptimizedHuggingFaceProvider{
		inference:      inf,
		mcpClient:      mcp.NewClient(),
		mcpSessions:    make(map[string]*mcp.Session),
		toolRegistry:   mcp.NewToolRegistry(),
		model:          model,
		contextManager: ctxmgr.NewContextManager(),
		parser:         parser.NewReActParser(model, false), // Non-strict mode for flexibility
		template:       template,
		modelConfig:    modelConfig,
		cache: &ResponseCache{
			entries: make(map[string]*CacheEntry),
			maxSize: 100,
			ttl:     5 * time.Minute,
		},
		metrics: &PerformanceMetrics{},
	}
}

// CreateCompletion creates an optimized completion with ReAct tool calling
func (p *OptimizedHuggingFaceProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	startTime := time.Now()
	p.recordRequest()

	// Check cache first
	cacheKey := p.generateCacheKey(req)
	if cached := p.getFromCache(cacheKey); cached != nil {
		p.recordCacheHit()
		return cached, nil
	}

	// Create context window
	window := p.contextManager.CreateWindow(p.model)

	// Convert provider tools to prompt tools
	promptTools := p.convertTools(req.Tools)

	// Build optimized prompt with context management
	optimizedPrompt, err := p.contextManager.OptimizePrompt(
		window,
		p.convertMessages(req.Messages),
		promptTools,
		p.template.SystemPrompt,
	)
	if err != nil {
		p.recordFailure()
		return nil, fmt.Errorf("optimize prompt: %w", err)
	}

	// Add few-shot examples if context allows
	if stats := window.GetStatistics(); stats["available"].(int) > 500 {
		optimizedPrompt = p.addFewShotExamples(optimizedPrompt)
	}

	// Configure inference parameters
	inferenceReq := p.buildInferenceRequest(optimizedPrompt, req)

	// ReAct loop with optimizations
	maxIterations := p.calculateMaxIterations(req)
	var lastResponse *CompletionResponse

	for i := 0; i < maxIterations; i++ {
		// Generate response with retry logic
		resp, err := p.generateWithRetry(ctx, inferenceReq)
		if err != nil {
			p.recordFailure()
			return nil, fmt.Errorf("generate (iteration %d): %w", i, err)
		}

		// Parse response with robust parser
		parseResult, err := p.parser.Parse(resp.Text)
		if err != nil {
			// Parser should not fail, but handle gracefully
			p.recordFailure()
			return nil, fmt.Errorf("parse response: %w", err)
		}

		// Check for final answer
		if parseResult.FinalAnswer != "" {
			response := &CompletionResponse{
				Content:      parseResult.FinalAnswer,
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      resp.Usage.TotalTokens,
				},
				Raw: map[string]any{
					"iterations": i + 1,
					"confidence": parseResult.Confidence,
					"model":      p.model,
				},
			}

			// Cache successful response
			p.cacheResponse(cacheKey, response)
			p.recordSuccess(time.Since(startTime))
			return response, nil
		}

		// Handle tool call
		if parseResult.ToolCall != nil {
			// Execute tool with timeout
			toolCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			result, err := p.executeToolWithMetrics(toolCtx, parseResult.ToolCall)
			cancel()

			if err != nil {
				result = fmt.Sprintf("Tool error: %v", err)
			}

			// Update prompt with observation
			observation := fmt.Sprintf("\n%s%s\n%s",
				p.template.OutputFormat.ObservationPrefix,
				result,
				p.template.OutputFormat.ThoughtPrefix)

			// Check context window before adding
			newPrompt := inferenceReq.Prompt + observation
			if p.EstimateTokens(newPrompt) > window.MaxTokens-window.ReservedTokens {
				// Compress context
				newPrompt = p.compressContext(inferenceReq.Prompt, observation, window)
			}
			inferenceReq.Prompt = newPrompt

		} else if parseResult.Confidence < 0.5 {
			// Low confidence, try to guide the model
			inferenceReq.Prompt += "\nPlease follow the exact format specified. Start with 'Thought:' and either call a tool or provide a 'Final Answer:'.\n"
		}

		lastResponse = &CompletionResponse{
			Content:      resp.Text,
			FinishReason: "length",
			Usage: Usage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			},
		}
	}

	// Max iterations reached
	if lastResponse != nil {
		p.recordPartialSuccess(time.Since(startTime))
		return lastResponse, nil
	}

	p.recordFailure()
	return nil, fmt.Errorf("max iterations (%d) reached without resolution", maxIterations)
}

// generateWithRetry implements retry logic with exponential backoff
func (p *OptimizedHuggingFaceProvider) generateWithRetry(ctx context.Context, req inference.GenerateRequest) (*inference.GenerateResponse, error) {
	maxRetries := 3
	backoff := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		resp, err := p.inference.Generate(ctx, req)
		if err == nil {
			return resp, nil
		}

		// Check if error is retryable
		if !p.isRetryableError(err) {
			return nil, err
		}

		if i < maxRetries-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// buildInferenceRequest builds an optimized inference request
func (p *OptimizedHuggingFaceProvider) buildInferenceRequest(prompt string, req CompletionRequest) inference.GenerateRequest {
	// Use model-specific configuration
	temperature := float64(p.modelConfig.Temperature)
	if req.Temperature > 0 {
		temperature = req.Temperature
	}

	maxTokens := 256 // Default for tool calling
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	return inference.GenerateRequest{
		Model:       p.model,
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stop:        p.modelConfig.StopSequences,
	}
}

// executeToolWithMetrics executes a tool and records metrics
func (p *OptimizedHuggingFaceProvider) executeToolWithMetrics(ctx context.Context, toolCall *parser.ToolCall) (string, error) {
	startTime := time.Now()

	// Convert to MCP params
	params := &mcp.CallToolParams{
		Name:      toolCall.Action,
		Arguments: toolCall.ActionInput,
	}

	// Find server and execute
	serverName := p.toolRegistry.GetServer(params.Name)
	if serverName == "" {
		return "", fmt.Errorf("tool not found: %s", params.Name)
	}

	session, exists := p.mcpSessions[serverName]
	if !exists {
		return "", fmt.Errorf("session not found for server: %s", serverName)
	}

	result, err := session.CallTool(ctx, *params)

	// Record metrics
	latency := time.Since(startTime)
	p.recordToolCall(err == nil, latency)

	if err != nil {
		return "", err
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", fmt.Errorf("empty result from tool: %s", params.Name)
}

// Helper methods for caching

func (p *OptimizedHuggingFaceProvider) generateCacheKey(req CompletionRequest) string {
	// Simple cache key based on messages and tools
	var parts []string
	for _, msg := range req.Messages {
		parts = append(parts, msg.Content)
	}
	for _, tool := range req.Tools {
		parts = append(parts, tool.Name)
	}
	return fmt.Sprintf("%x", strings.Join(parts, "|"))
}

func (p *OptimizedHuggingFaceProvider) getFromCache(key string) *CompletionResponse {
	p.cache.mu.RLock()
	defer p.cache.mu.RUnlock()

	entry, exists := p.cache.entries[key]
	if !exists {
		return nil
	}

	if time.Since(entry.Timestamp) > p.cache.ttl {
		return nil
	}

	return entry.Response
}

func (p *OptimizedHuggingFaceProvider) cacheResponse(key string, response *CompletionResponse) {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()

	// Evict old entries if cache is full
	if len(p.cache.entries) >= p.cache.maxSize {
		// Simple LRU eviction
		var oldestKey string
		var oldestTime time.Time
		for k, v := range p.cache.entries {
			if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		delete(p.cache.entries, oldestKey)
	}

	p.cache.entries[key] = &CacheEntry{
		Response:  response,
		Timestamp: time.Now(),
	}
}

// Helper methods for context management

func (p *OptimizedHuggingFaceProvider) compressContext(prompt string, observation string, window *ctxmgr.ContextWindow) string {
	// Simple compression: keep system prompt and recent exchanges
	lines := strings.Split(prompt, "\n")

	// Keep first part (system + tools)
	systemEnd := p.findSystemEndIndex(lines)
	compressed := strings.Join(lines[:systemEnd], "\n")

	// Keep last interaction
	if len(lines) > systemEnd+10 {
		compressed += "\n[Previous interactions summarized]\n"
		compressed += strings.Join(lines[len(lines)-5:], "\n")
	} else {
		compressed += "\n" + strings.Join(lines[systemEnd:], "\n")
	}

	compressed += observation
	return compressed
}

func (p *OptimizedHuggingFaceProvider) findSystemEndIndex(lines []string) int {
	for i, line := range lines {
		if strings.Contains(line, "User:") || strings.Contains(line, "## Conversation") {
			return i
		}
	}
	return len(lines) / 3 // Fallback: keep first third
}

func (p *OptimizedHuggingFaceProvider) addFewShotExamples(prompt string) string {
	// Add examples if not already present
	if strings.Contains(prompt, "## Examples") {
		return prompt
	}

	var examples strings.Builder
	examples.WriteString("\n## Examples\n\n")

	for _, example := range p.template.FewShotExamples[:1] { // Add one example to save tokens
		examples.WriteString(fmt.Sprintf("User: %s\n", example.Query))
		examples.WriteString(fmt.Sprintf("%s%s\n", p.template.OutputFormat.ThoughtPrefix, example.Thought))
		examples.WriteString(fmt.Sprintf("%s%s\n", p.template.OutputFormat.ActionPrefix, example.Action))
		examples.WriteString(fmt.Sprintf("%s%s\n\n", p.template.OutputFormat.FinalAnswerPrefix, example.FinalAnswer))
	}

	// Insert examples before conversation
	parts := strings.Split(prompt, "## Conversation")
	if len(parts) == 2 {
		return parts[0] + examples.String() + "## Conversation" + parts[1]
	}

	return prompt + examples.String()
}

// Utility methods

func (p *OptimizedHuggingFaceProvider) calculateMaxIterations(req CompletionRequest) int {
	// Adjust iterations based on model and request
	base := 5
	if strings.Contains(p.model, "gemma") {
		base = 4 // Gemma needs fewer iterations
	}
	if len(req.Tools) > 5 {
		base += 2 // More tools may need more iterations
	}
	return base
}

func (p *OptimizedHuggingFaceProvider) isRetryableError(err error) bool {
	errStr := err.Error()
	retryableErrors := []string{
		"timeout",
		"connection",
		"temporary",
		"rate limit",
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}
	return false
}

func (p *OptimizedHuggingFaceProvider) convertTools(tools []Tool) []prompt.Tool {
	converted := make([]prompt.Tool, len(tools))
	for i, tool := range tools {
		// Unmarshal Parameters (json.RawMessage) to map[string]any
		var schema map[string]any
		if len(tool.Parameters) > 0 {
			_ = json.Unmarshal(tool.Parameters, &schema)
		}
		converted[i] = prompt.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Schema:      schema,
		}
	}
	return converted
}

func (p *OptimizedHuggingFaceProvider) convertMessages(messages []Message) []prompt.Message {
	converted := make([]prompt.Message, len(messages))
	for i, msg := range messages {
		converted[i] = prompt.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return converted
}

// Metrics methods

func (p *OptimizedHuggingFaceProvider) recordRequest() {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.TotalRequests++
}

func (p *OptimizedHuggingFaceProvider) recordSuccess(latency time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.SuccessfulCalls++
	p.updateAverageLatency(latency)
}

func (p *OptimizedHuggingFaceProvider) recordPartialSuccess(latency time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.SuccessfulCalls++
	p.updateAverageLatency(latency)
}

func (p *OptimizedHuggingFaceProvider) recordFailure() {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.FailedCalls++
}

func (p *OptimizedHuggingFaceProvider) recordCacheHit() {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	p.metrics.CacheHits++
}

func (p *OptimizedHuggingFaceProvider) recordToolCall(success bool, latency time.Duration) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	// Simple accuracy tracking
	total := float64(p.metrics.SuccessfulCalls + p.metrics.FailedCalls)
	if total > 0 {
		if success {
			p.metrics.ToolCallAccuracy = (p.metrics.ToolCallAccuracy*total + 1) / (total + 1)
		} else {
			p.metrics.ToolCallAccuracy = (p.metrics.ToolCallAccuracy * total) / (total + 1)
		}
	}
}

func (p *OptimizedHuggingFaceProvider) updateAverageLatency(latency time.Duration) {
	if p.metrics.AverageLatency == 0 {
		p.metrics.AverageLatency = latency
	} else {
		// Moving average
		p.metrics.AverageLatency = (p.metrics.AverageLatency*9 + latency) / 10
	}
}

// GetMetrics returns current performance metrics
func (p *OptimizedHuggingFaceProvider) GetMetrics() map[string]any {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()

	successRate := float64(0)
	if p.metrics.TotalRequests > 0 {
		successRate = float64(p.metrics.SuccessfulCalls) / float64(p.metrics.TotalRequests)
	}

	cacheHitRate := float64(0)
	if p.metrics.TotalRequests > 0 {
		cacheHitRate = float64(p.metrics.CacheHits) / float64(p.metrics.TotalRequests)
	}

	return map[string]any{
		"total_requests":     p.metrics.TotalRequests,
		"successful_calls":   p.metrics.SuccessfulCalls,
		"failed_calls":       p.metrics.FailedCalls,
		"success_rate":       successRate,
		"cache_hits":         p.metrics.CacheHits,
		"cache_hit_rate":     cacheHitRate,
		"average_latency_ms": p.metrics.AverageLatency.Milliseconds(),
		"tool_call_accuracy": p.metrics.ToolCallAccuracy,
		"model":              p.model,
	}
}

// EstimateTokens estimates token count (simple implementation)
func (p *OptimizedHuggingFaceProvider) EstimateTokens(text string) int {
	// Simple estimation: ~4 characters per token
	// In production, use the actual tokenizer
	return len(text) / 4
}

// Interface implementations

func (p *OptimizedHuggingFaceProvider) Name() string {
	return "huggingface-optimized"
}

func (p *OptimizedHuggingFaceProvider) ConnectMCPServer(ctx context.Context, config mcp.ServerConfig) error {
	session, err := p.mcpClient.Connect(ctx, config)
	if err != nil {
		return fmt.Errorf("connect to MCP server %s: %w", config.Name, err)
	}

	tools, err := session.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools from %s: %w", config.Name, err)
	}

	_ = p.toolRegistry.Register(config.Name, tools)
	p.mcpSessions[config.Name] = session

	return nil
}

func (p *OptimizedHuggingFaceProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	handler := NewStructuredOutputHandler(p.inference, req.StrictSchema)
	return handler.Generate(ctx, req, p.model)
}

func (p *OptimizedHuggingFaceProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	// Generate full response first
	resp, err := p.CreateCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	// Return simulated stream with smaller chunks for smoother output
	return NewSimulatedStream(resp.Content, resp.FinishReason, 15), nil
}
