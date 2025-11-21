package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	"github.com/aixgo-dev/aixgo/pkg/security"
)

func init() {
	// Register HuggingFace provider factory in global registry
	RegisterFactory("huggingface", func(config map[string]any) (Provider, error) {
		// Extract model from config
		model, ok := config["model"].(string)
		if !ok {
			return nil, fmt.Errorf("model not specified in config")
		}

		// Extract inference service from config
		// This will be set by the application when creating the provider
		inf, ok := config["inference"].(inference.InferenceService)
		if !ok {
			return nil, fmt.Errorf("inference service not provided in config")
		}

		return NewHuggingFaceProvider(inf, model), nil
	})
}

// HuggingFaceProvider implements LLM provider for HuggingFace models with ReAct
type HuggingFaceProvider struct {
	inference    inference.InferenceService
	mcpClient    *mcp.Client
	mcpSessions  map[string]*mcp.Session
	toolRegistry *mcp.ToolRegistry
	model        string
}

// NewHuggingFaceProvider creates a new HuggingFace provider
func NewHuggingFaceProvider(inf inference.InferenceService, model string) *HuggingFaceProvider {
	return &HuggingFaceProvider{
		inference:    inf,
		mcpClient:    mcp.NewClient(),
		mcpSessions:  make(map[string]*mcp.Session),
		toolRegistry: mcp.NewToolRegistry(),
		model:        model,
	}
}

// ConnectMCPServer connects to an MCP server
func (p *HuggingFaceProvider) ConnectMCPServer(ctx context.Context, config mcp.ServerConfig) error {
	session, err := p.mcpClient.Connect(ctx, config)
	if err != nil {
		return fmt.Errorf("connect to MCP server %s: %w", config.Name, err)
	}

	// Discover tools
	tools, err := session.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("list tools from %s: %w", config.Name, err)
	}

	// Register tools with collision detection
	if err := p.toolRegistry.Register(config.Name, tools); err != nil {
		return fmt.Errorf("register tools from %s: %w", config.Name, err)
	}
	p.mcpSessions[config.Name] = session

	return nil
}

// CreateCompletion creates a completion with ReAct tool calling
func (p *HuggingFaceProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	// Set overall timeout for ReAct loop (5 minutes)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Configure max iterations (default 5, max 10)
	maxIterations := 5
	if req.MaxIterations > 0 {
		maxIterations = req.MaxIterations
	}
	if maxIterations > 10 {
		return nil, fmt.Errorf("max iterations cannot exceed 10")
	}

	// Token budget for the entire loop
	maxTokens := req.TokenBudget
	if maxTokens == 0 {
		maxTokens = 4000
	}
	totalTokens := 0

	// Build ReAct prompt with tools
	prompt := p.buildReActPrompt(req.Messages, req.Tools)

	// Get list of allowed tools
	allowedTools := p.getAllowedToolNames()

	for i := 0; i < maxIterations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ReAct loop timeout")
		default:
		}

		// Check token budget
		if totalTokens >= maxTokens {
			return nil, fmt.Errorf("token budget exceeded")
		}

		// Generate response with reduced token limit
		remainingTokens := maxTokens - totalTokens
		requestTokens := req.MaxTokens
		if requestTokens > remainingTokens {
			requestTokens = remainingTokens
		}

		resp, err := p.inference.Generate(ctx, inference.GenerateRequest{
			Model:       p.model,
			Prompt:      prompt,
			MaxTokens:   requestTokens,
			Temperature: req.Temperature,
		})
		if err != nil {
			return nil, fmt.Errorf("generate: %w", err)
		}

		totalTokens += resp.Usage.TotalTokens

		// Detect injection attempts
		if err := detectInjectionAttempts(resp.Text); err != nil {
			return nil, fmt.Errorf("potential prompt injection detected: %w", err)
		}

		// Check for final answer
		if finalAnswer := extractFinalAnswer(resp.Text); finalAnswer != "" {
			return &CompletionResponse{
				Content:      finalAnswer,
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      totalTokens,
				},
			}, nil
		}

		// Parse tool call with validation
		toolCall, err := parseToolCallSecure(resp.Text, allowedTools)
		if err != nil {
			// Tool parsing/validation error - add as observation and continue loop
			prompt += fmt.Sprintf("\nObservation: Error: %v\nThought:", err)
			continue
		}

		if toolCall == nil {
			// No tool call and no final answer - treat as final answer
			return &CompletionResponse{
				Content:      resp.Text,
				FinishReason: "stop",
				Usage: Usage{
					PromptTokens:     resp.Usage.PromptTokens,
					CompletionTokens: resp.Usage.CompletionTokens,
					TotalTokens:      totalTokens,
				},
			}, nil
		}

		// Execute tool with timeout
		toolCtx, toolCancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := p.executeTool(toolCtx, toolCall)
		toolCancel()

		if err != nil {
			result = fmt.Sprintf("Error executing tool: %v", err)
		}

		// Add observation to prompt and continue
		prompt += fmt.Sprintf("\nObservation: %s\nThought:", result)
	}

	return nil, fmt.Errorf("max iterations reached without final answer")
}

// CreateStructured creates a structured response with schema validation
func (p *HuggingFaceProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	handler := NewStructuredOutputHandler(p.inference, req.StrictSchema)
	return handler.Generate(ctx, req, p.model)
}

// CreateStreaming creates a streaming response (simulated for non-streaming inference)
func (p *HuggingFaceProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	// Generate full response first
	resp, err := p.CreateCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	// Return simulated stream
	return NewSimulatedStream(resp.Content, resp.FinishReason, 20), nil
}

// Name returns the provider name
func (p *HuggingFaceProvider) Name() string {
	return "huggingface"
}

// buildReActPrompt builds a ReAct-style prompt with tools
func (p *HuggingFaceProvider) buildReActPrompt(messages []Message, tools []Tool) string {
	var sb strings.Builder

	// System prompt with tools
	sb.WriteString("You are an AI assistant with access to the following tools:\n\n")
	sb.WriteString("TOOLS:\n")

	allTools := p.toolRegistry.ListTools()
	for _, tool := range allTools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		if len(tool.Schema) > 0 {
			schemaJSON, _ := json.Marshal(tool.Schema)
			sb.WriteString(fmt.Sprintf("  Input schema: %s\n", string(schemaJSON)))
		}
	}

	sb.WriteString("\nUse this format:\n")
	sb.WriteString("Thought: [your reasoning about what to do]\n")
	sb.WriteString("Action: [tool name]\n")
	sb.WriteString("Action Input: {\"arg\": \"value\"}\n")
	sb.WriteString("Observation: [tool result will appear here]\n")
	sb.WriteString("... (repeat Thought/Action/Observation as needed)\n")
	sb.WriteString("Thought: I now have enough information\n")
	sb.WriteString("Final Answer: [your response to the user]\n\n")

	// Add messages
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		}
	}

	sb.WriteString("Let's solve this step by step:\nThought:")

	return sb.String()
}

// parseToolCall parses a ReAct-style tool call from LLM response (DEPRECATED: use parseToolCallSecure)
func parseToolCall(text string) *mcp.CallToolParams {
	params, _ := parseToolCallSecure(text, nil)
	return params
}

// parseToolCallSecure parses a ReAct-style tool call with security validation
func parseToolCallSecure(text string, allowedTools []string) (*mcp.CallToolParams, error) {
	// Extract Action and Action Input
	actionRe := regexp.MustCompile(`Action:\s*(\w+)`)
	inputRe := regexp.MustCompile(`Action Input:\s*(\{.*?\})`)

	actionMatch := actionRe.FindStringSubmatch(text)
	inputMatch := inputRe.FindStringSubmatch(text)

	if actionMatch == nil || inputMatch == nil {
		return nil, nil
	}

	toolName := actionMatch[1]
	inputJSON := inputMatch[1]

	// Validate tool name
	if err := security.ValidateToolName(toolName); err != nil {
		return nil, fmt.Errorf("invalid tool name: %w", err)
	}

	// Validate tool exists in allowlist
	if allowedTools != nil {
		toolAllowed := false
		for _, allowed := range allowedTools {
			if toolName == allowed {
				toolAllowed = true
				break
			}
		}
		if !toolAllowed {
			return nil, fmt.Errorf("tool not in allowlist: %s", toolName)
		}
	}

	// Parse JSON input
	var args map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &args); err != nil {
		// Try to fix common JSON errors
		fixedJSON := fixCommonJSONErrors(inputJSON)
		if err := json.Unmarshal([]byte(fixedJSON), &args); err != nil {
			return nil, fmt.Errorf("invalid JSON input: %w", err)
		}
	}

	return &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}, nil
}

// extractFinalAnswer extracts the final answer from ReAct response
func extractFinalAnswer(text string) string {
	answerRe := regexp.MustCompile(`Final Answer:\s*(.+?)(?:\n|$)`)
	match := answerRe.FindStringSubmatch(text)
	if match != nil {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// executeTool executes a tool via MCP
func (p *HuggingFaceProvider) executeTool(ctx context.Context, params *mcp.CallToolParams) (string, error) {
	// Find which server hosts this tool
	serverName := p.toolRegistry.GetServer(params.Name)
	if serverName == "" {
		return "", fmt.Errorf("tool not found: %s", params.Name)
	}

	session, exists := p.mcpSessions[serverName]
	if !exists {
		return "", fmt.Errorf("session not found for server: %s", serverName)
	}

	// Call tool
	result, err := session.CallTool(ctx, *params)
	if err != nil {
		return "", err
	}

	// Format result as string
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", fmt.Errorf("empty result from tool: %s", params.Name)
}

// fixCommonJSONErrors attempts to fix common JSON errors
func fixCommonJSONErrors(s string) string {
	// Remove block comments
	s = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(s, "")

	// Remove line comments but preserve the newline
	s = regexp.MustCompile(`//.*`).ReplaceAllString(s, "")

	// Fix single quotes
	s = strings.ReplaceAll(s, "'", "\"")

	// Remove trailing commas
	s = regexp.MustCompile(`,\s*}`).ReplaceAllString(s, "}")
	s = regexp.MustCompile(`,\s*]`).ReplaceAllString(s, "]")

	return s
}

// detectInjectionAttempts detects potential prompt injection attempts in LLM output
func detectInjectionAttempts(llmOutput string) error {
	// Check for multiple Action: markers (suspicious)
	actionCount := strings.Count(llmOutput, "Action:")
	if actionCount > 1 {
		return fmt.Errorf("multiple actions detected in single output")
	}

	// Check for Observation: in LLM output (LLM shouldn't generate observations)
	if strings.Contains(llmOutput, "Observation:") {
		return fmt.Errorf("llm output contains observation marker")
	}

	// Check for output that includes the prompt template
	suspiciousPatterns := []string{
		"TOOLS:",
		"Use this format:",
		"Let's solve this step by step:",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(llmOutput, pattern) {
			return fmt.Errorf("llm output contains prompt template")
		}
	}

	// Check for suspicious commands or attempts to break out
	dangerousPatterns := []string{
		"Ignore previous instructions",
		"Ignore all previous",
		"system:",
		"<|system|>",
		"You are now",
		"Your instructions are",
	}

	lowerOutput := strings.ToLower(llmOutput)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerOutput, strings.ToLower(pattern)) {
			return fmt.Errorf("potential injection attempt detected")
		}
	}

	return nil
}

// getAllowedToolNames returns a list of all registered tool names
func (p *HuggingFaceProvider) getAllowedToolNames() []string {
	tools := p.toolRegistry.ListTools()
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}
