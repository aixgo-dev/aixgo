package context

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/llm/prompt"
)

// TokenEstimator estimates token count for text
type TokenEstimator interface {
	EstimateTokens(text string) int
}

// SimpleTokenEstimator provides basic token estimation
type SimpleTokenEstimator struct{}

func (e *SimpleTokenEstimator) EstimateTokens(text string) int {
	// Rough estimate: 1 token ≈ 4 characters for English
	// Adjust based on your model's tokenizer
	return len(text) / 4
}

// ContextWindow represents the context window state
type ContextWindow struct {
	MaxTokens      int
	CurrentTokens  int
	Messages       []Message
	Tools          []Tool
	SystemPrompt   string
	ReservedTokens int // Reserved for output generation
}

// Message represents a conversation message with token count
type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Tokens    int    `json:"tokens"`
	Timestamp int64  `json:"timestamp"`
	Priority  int    `json:"priority"` // Higher priority messages are kept longer
}

// Tool represents a tool with token count
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
	Tokens      int            `json:"tokens"`
	Essential   bool           `json:"essential"` // Essential tools are always included
}

// ContextManager manages context windows for different models
type ContextManager struct {
	estimator       TokenEstimator
	modelConfigs    map[string]*ModelContextConfig
	summaryCache    *SummaryCache
	toolSchemaCache map[string]string
	mu              sync.RWMutex
}

// ModelContextConfig contains model-specific context configuration
type ModelContextConfig struct {
	ModelName        string
	MaxContextTokens int
	OutputReserve    int // Tokens reserved for output
	CompressionRatio float64
}

// SummaryCache caches conversation summaries
type SummaryCache struct {
	summaries map[string]string
}

// NewContextManager creates a new context manager
func NewContextManager() *ContextManager {
	return &ContextManager{
		estimator: &SimpleTokenEstimator{},
		modelConfigs: map[string]*ModelContextConfig{
			"phi3.5": {
				ModelName:        "phi3.5",
				MaxContextTokens: 4096,
				OutputReserve:    512,
				CompressionRatio: 0.7,
			},
			"gemma:2b": {
				ModelName:        "gemma:2b",
				MaxContextTokens: 8192,
				OutputReserve:    512,
				CompressionRatio: 0.75,
			},
			"default": {
				ModelName:        "default",
				MaxContextTokens: 4096,
				OutputReserve:    512,
				CompressionRatio: 0.7,
			},
		},
		summaryCache:    &SummaryCache{summaries: make(map[string]string)},
		toolSchemaCache: make(map[string]string),
	}
}

// CreateWindow creates a new context window for a model
func (cm *ContextManager) CreateWindow(modelName string) *ContextWindow {
	config := cm.getModelConfig(modelName)
	return &ContextWindow{
		MaxTokens:      config.MaxContextTokens,
		CurrentTokens:  0,
		Messages:       make([]Message, 0),
		Tools:          make([]Tool, 0),
		ReservedTokens: config.OutputReserve,
	}
}

// OptimizePrompt optimizes a prompt to fit within the context window
func (cm *ContextManager) OptimizePrompt(
	window *ContextWindow,
	messages []prompt.Message,
	tools []prompt.Tool,
	systemPrompt string,
) (string, error) {

	// Convert to internal format and estimate tokens
	window.SystemPrompt = systemPrompt
	systemTokens := cm.estimator.EstimateTokens(systemPrompt)

	// Process tools with caching
	toolsTokens := 0
	for _, tool := range tools {
		toolKey := fmt.Sprintf("%s:%s", tool.Name, tool.Description)

		cm.mu.RLock()
		cachedSchema, exists := cm.toolSchemaCache[toolKey]
		cm.mu.RUnlock()

		if !exists {
			schemaJSON, _ := json.Marshal(tool.Schema)
			cachedSchema = cm.compressToolSchema(string(schemaJSON))

			cm.mu.Lock()
			cm.toolSchemaCache[toolKey] = cachedSchema
			cm.mu.Unlock()
		}

		toolTokens := cm.estimator.EstimateTokens(tool.Name + tool.Description + cachedSchema)
		window.Tools = append(window.Tools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Schema:      tool.Schema,
			Tokens:      toolTokens,
			Essential:   cm.isEssentialTool(tool.Name),
		})
		toolsTokens += toolTokens
	}

	// Process messages
	messageTokens := 0
	for i, msg := range messages {
		tokens := cm.estimator.EstimateTokens(msg.Content)
		window.Messages = append(window.Messages, Message{
			Role:     msg.Role,
			Content:  msg.Content,
			Tokens:   tokens,
			Priority: len(messages) - i, // Recent messages have higher priority
		})
		messageTokens += tokens
	}

	// Calculate total tokens
	totalTokens := systemTokens + toolsTokens + messageTokens + window.ReservedTokens

	// If within limits, build and return prompt
	if totalTokens <= window.MaxTokens {
		return cm.buildPrompt(window), nil
	}

	// Apply optimization strategies
	window = cm.applyOptimizationStrategies(window, totalTokens)
	return cm.buildPrompt(window), nil
}

// applyOptimizationStrategies applies various strategies to fit within context
func (cm *ContextManager) applyOptimizationStrategies(window *ContextWindow, currentTokens int) *ContextWindow {
	targetTokens := window.MaxTokens - window.ReservedTokens

	// Strategy 1: Remove non-essential tools
	if currentTokens > targetTokens {
		var essentialTools []Tool
		removedTokens := 0
		for _, tool := range window.Tools {
			if tool.Essential {
				essentialTools = append(essentialTools, tool)
			} else {
				removedTokens += tool.Tokens
			}
		}
		window.Tools = essentialTools
		currentTokens -= removedTokens
	}

	// Strategy 2: Summarize old messages
	if currentTokens > targetTokens && len(window.Messages) > 3 {
		summarizedMessages := cm.summarizeOldMessages(window.Messages)
		window.Messages = summarizedMessages
		currentTokens = cm.recalculateTokens(window)
	}

	// Strategy 3: Truncate very long messages
	if currentTokens > targetTokens {
		for i := range window.Messages {
			if window.Messages[i].Tokens > 500 {
				truncated := cm.truncateMessage(window.Messages[i].Content, 400)
				window.Messages[i].Content = truncated
				window.Messages[i].Tokens = cm.estimator.EstimateTokens(truncated)
			}
		}
		currentTokens = cm.recalculateTokens(window)
	}

	// Strategy 4: Keep only essential recent messages
	if currentTokens > targetTokens && len(window.Messages) > 2 {
		// Keep the first (usually context) and last few messages
		if len(window.Messages) > 4 {
			window.Messages = append(window.Messages[:1], window.Messages[len(window.Messages)-3:]...)
			_ = cm.recalculateTokens(window)
		}
	}

	return window
}

// summarizeOldMessages creates summaries of older conversation parts
func (cm *ContextManager) summarizeOldMessages(messages []Message) []Message {
	if len(messages) <= 3 {
		return messages
	}

	// Group older messages for summarization
	oldMessages := messages[:len(messages)-3]
	recentMessages := messages[len(messages)-3:]

	// Create a simple summary (in production, use an LLM for this)
	summaryContent := cm.createSimpleSummary(oldMessages)
	summaryMessage := Message{
		Role:     "system",
		Content:  fmt.Sprintf("[Previous conversation summary: %s]", summaryContent),
		Tokens:   cm.estimator.EstimateTokens(summaryContent),
		Priority: 0,
	}

	// Return summary + recent messages
	return append([]Message{summaryMessage}, recentMessages...)
}

// createSimpleSummary creates a simple summary of messages
func (cm *ContextManager) createSimpleSummary(messages []Message) string {
	// In production, use an LLM to generate summaries
	// This is a simple implementation for demonstration
	var topics []string
	for _, msg := range messages {
		if msg.Role == "user" {
			// Extract key topics (simplified)
			if len(msg.Content) > 50 {
				topics = append(topics, msg.Content[:50]+"...")
			} else {
				topics = append(topics, msg.Content)
			}
		}
	}

	if len(topics) == 0 {
		return "Previous conversation about various topics"
	}

	return fmt.Sprintf("Discussed: %s", strings.Join(topics, "; "))
}

// compressToolSchema compresses tool schema representation
func (cm *ContextManager) compressToolSchema(schema string) string {
	// Remove unnecessary whitespace
	compressed := strings.ReplaceAll(schema, "\n", " ")
	compressed = strings.ReplaceAll(compressed, "\t", " ")
	compressed = strings.ReplaceAll(compressed, "  ", " ")

	// Remove optional fields for compression
	compressed = strings.ReplaceAll(compressed, `"required":false,`, "")
	compressed = strings.ReplaceAll(compressed, `"optional":true,`, "")

	return compressed
}

// truncateMessage intelligently truncates a message
func (cm *ContextManager) truncateMessage(content string, maxTokens int) string {
	// Estimate character count for target tokens
	maxChars := maxTokens * 4

	if len(content) <= maxChars {
		return content
	}

	// Try to truncate at sentence boundary
	truncated := content[:maxChars]
	lastPeriod := strings.LastIndex(truncated, ".")
	if lastPeriod > maxChars/2 {
		truncated = truncated[:lastPeriod+1]
	}

	return truncated + "... [truncated]"
}

// buildPrompt builds the final prompt from the optimized window
func (cm *ContextManager) buildPrompt(window *ContextWindow) string {
	var sb strings.Builder

	// System prompt
	sb.WriteString(window.SystemPrompt)
	sb.WriteString("\n\n")

	// Tools
	if len(window.Tools) > 0 {
		sb.WriteString("Available Tools:\n")
		for _, tool := range window.Tools {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
			if len(tool.Schema) > 0 {
				cm.mu.RLock()
				cachedSchema := cm.toolSchemaCache[fmt.Sprintf("%s:%s", tool.Name, tool.Description)]
				cm.mu.RUnlock()
				if cachedSchema != "" {
					sb.WriteString(fmt.Sprintf("  Schema: %s\n", cachedSchema))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Messages
	for _, msg := range window.Messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		case "system":
			sb.WriteString(fmt.Sprintf("%s\n\n", msg.Content))
		}
	}

	return sb.String()
}

// Helper methods

func (cm *ContextManager) getModelConfig(modelName string) *ModelContextConfig {
	// Check for exact match
	if config, exists := cm.modelConfigs[modelName]; exists {
		return config
	}

	// Check for partial match
	modelLower := strings.ToLower(modelName)
	for key, config := range cm.modelConfigs {
		if strings.Contains(modelLower, key) {
			return config
		}
	}

	return cm.modelConfigs["default"]
}

func (cm *ContextManager) isEssentialTool(toolName string) bool {
	// Define essential tools that should always be included
	essentialTools := map[string]bool{
		"get_weather": true,
		"calculate":   true,
		"search":      true,
	}
	return essentialTools[toolName]
}

func (cm *ContextManager) recalculateTokens(window *ContextWindow) int {
	total := cm.estimator.EstimateTokens(window.SystemPrompt)
	for _, tool := range window.Tools {
		total += tool.Tokens
	}
	for _, msg := range window.Messages {
		total += msg.Tokens
	}
	return total + window.ReservedTokens
}

// GetStatistics returns current window statistics
func (window *ContextWindow) GetStatistics() map[string]any {
	messageTokens := 0
	for _, msg := range window.Messages {
		messageTokens += msg.Tokens
	}

	toolTokens := 0
	for _, tool := range window.Tools {
		toolTokens += tool.Tokens
	}

	systemTokens := len(window.SystemPrompt) / 4 // Rough estimate

	return map[string]any{
		"max_tokens":      window.MaxTokens,
		"reserved_tokens": window.ReservedTokens,
		"system_tokens":   systemTokens,
		"message_tokens":  messageTokens,
		"tool_tokens":     toolTokens,
		"total_used":      systemTokens + messageTokens + toolTokens,
		"available":       window.MaxTokens - window.ReservedTokens - systemTokens - messageTokens - toolTokens,
		"message_count":   len(window.Messages),
		"tool_count":      len(window.Tools),
	}
}
