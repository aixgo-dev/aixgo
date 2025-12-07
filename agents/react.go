package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm"
	"github.com/aixgo-dev/aixgo/internal/llm/provider"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	"github.com/aixgo-dev/aixgo/pkg/security"
	pb "github.com/aixgo-dev/aixgo/proto"
	"github.com/sashabaranov/go-openai"
)

// OpenAIClient interface for testability
type OpenAIClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

type ReActAgent struct {
	*BaseAgent   // Provides Name(), Role(), Ready(), Stop()
	def          agent.AgentDef
	client       OpenAIClient
	provider     provider.Provider
	model        string
	tools        map[string]func(context.Context, map[string]any) (any, error)
	rt           agent.Runtime
	mcpClient    *mcp.Client
	mcpSessions  map[string]*mcp.Session
	toolRegistry *mcp.ToolRegistry
}

func init() {
	agent.Register("react", NewReActAgent)
}

// NewReActAgent creates a new ReActAgent with provider-based initialization
func NewReActAgent(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
	// Try to get provider from model name or default to OpenAI
	var prov provider.Provider
	var client OpenAIClient

	// Detect provider based on model name or explicit provider field
	providerName := detectProvider(def.Model)

	if providerName == "huggingface" {
		// HuggingFace provider initialization will be done in ConnectMCPServers
		prov = nil // Will be set later
		client = nil
	} else {
		// Default to OpenAI-compatible client with API key from environment
		apiKey := getAPIKeyFromEnv(def.Model)
		if apiKey == "" {
			return nil, fmt.Errorf("API key not found: please set the appropriate environment variable (XAI_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY, or HUGGINGFACE_API_KEY)")
		}
		client = openai.NewClient(apiKey)
	}

	return NewReActAgentWithProvider(def, rt, client, prov)
}

// NewReActAgentWithClient creates a new ReActAgent with a custom client (useful for testing)
func NewReActAgentWithClient(def agent.AgentDef, rt agent.Runtime, client OpenAIClient) (agent.Agent, error) {
	return NewReActAgentWithProvider(def, rt, client, nil)
}

// NewReActAgentWithProvider creates a new ReActAgent with custom client and provider
func NewReActAgentWithProvider(def agent.AgentDef, rt agent.Runtime, client OpenAIClient, prov provider.Provider) (agent.Agent, error) {
	tools := make(map[string]func(context.Context, map[string]any) (any, error))
	for _, t := range def.Tools {
		validator := llm.NewValidator(t.InputSchema)
		toolName := t.Name
		tools[toolName] = func(ctx context.Context, in map[string]any) (any, error) {
			if err := validator.Validate(in); err != nil {
				return nil, err
			}
			return map[string]string{"status": "ok"}, nil
		}
	}

	agent := &ReActAgent{
		BaseAgent:    NewBaseAgent(def),
		def:          def,
		client:       client,
		provider:     prov,
		model:        def.Model,
		tools:        tools,
		rt:           rt,
		mcpClient:    mcp.NewClient(),
		mcpSessions:  make(map[string]*mcp.Session),
		toolRegistry: mcp.NewToolRegistry(),
	}

	return agent, nil
}

// detectProvider detects the provider from model name
func detectProvider(model string) string {
	// Check for HuggingFace model patterns
	if isHuggingFaceModel(model) {
		return "huggingface"
	}

	// Default to OpenAI-compatible
	return "openai"
}

// isHuggingFaceModel checks if a model name is a HuggingFace model
func isHuggingFaceModel(model string) bool {
	// HuggingFace models typically have "/" in the name (e.g., "meta-llama/Llama-2-7b")
	// or are from common open-source model families
	hfPatterns := []string{
		"meta-llama/", "mistralai/", "tiiuae/", "EleutherAI/",
		"bigscience/", "facebook/", "google/", "microsoft/",
	}

	for _, pattern := range hfPatterns {
		if len(model) > len(pattern) && model[:len(pattern)] == pattern {
			return true
		}
	}

	return false
}

// ConnectMCPServers connects to MCP servers from config
func (r *ReActAgent) ConnectMCPServers(ctx context.Context, serverConfigs []mcp.ServerConfig) error {
	for _, config := range serverConfigs {
		session, err := r.mcpClient.Connect(ctx, config)
		if err != nil {
			log.Printf("Warning: Failed to connect to MCP server %s: %v", config.Name, err)
			continue
		}

		// Discover tools
		tools, err := session.ListTools(ctx)
		if err != nil {
			log.Printf("Warning: Failed to list tools from %s: %v", config.Name, err)
			continue
		}

		// Register tools
		_ = r.toolRegistry.Register(config.Name, tools)
		r.mcpSessions[config.Name] = session

		log.Printf("Connected to MCP server %s with %d tools", config.Name, len(tools))
	}

	return nil
}

// SetProvider sets the LLM provider for this agent
func (r *ReActAgent) SetProvider(prov provider.Provider) {
	r.provider = prov
}

// Execute performs synchronous ReAct execution
func (r *ReActAgent) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	if !r.Ready() {
		return nil, fmt.Errorf("agent not ready")
	}

	// Extract string from message
	inputStr := ""
	if input != nil && input.Message != nil {
		inputStr = input.Payload
	}

	// Use the existing think method to process the input
	result, err := r.think(ctx, inputStr)
	if err != nil {
		return nil, err
	}

	// Convert result back to Message
	return &agent.Message{
		Message: &pb.Message{
			Type:    "react_response",
			Payload: result,
		},
	}, nil
}

func (r *ReActAgent) Start(ctx context.Context) error {
	r.InitContext(ctx) // Initialize context from BaseAgent
	if len(r.def.Inputs) == 0 {
		return fmt.Errorf("no inputs defined for ReActAgent")
	}

	ch, err := r.rt.Recv(r.def.Inputs[0].Source)
	if err != nil {
		return fmt.Errorf("failed to receive from %s: %w", r.def.Inputs[0].Source, err)
	}

	// Create input validator
	inputValidator := &security.StringValidator{
		MaxLength:            100000, // 100KB max input
		DisallowNullBytes:    true,
		DisallowControlChars: true,
	}

	// Create prompt injection detector
	injectionDetector := security.NewPromptInjectionDetector(security.SensitivityMedium)

	for m := range ch {
		// Validate input message
		if err := inputValidator.Validate(m.Payload); err != nil {
			log.Printf("ReAct input validation error: %v", err)
			continue
		}

		// Check for prompt injection
		inputPayload := m.Payload
		detectionResult := injectionDetector.Detect(inputPayload)
		if detectionResult.Detected {
			log.Printf("Potential prompt injection detected (confidence: %.2f): %v",
				detectionResult.Confidence, detectionResult.MatchedPatterns)
			// Sanitize by wrapping in delimiters
			inputPayload = "<<<USER_INPUT_START>>>\n" + inputPayload + "\n<<<USER_INPUT_END>>>"
		}

		span := observability.StartSpan("react.think", map[string]any{"input": inputPayload})
		res, err := r.think(ctx, inputPayload)
		span.End()
		if err != nil {
			log.Printf("ReAct error: %v", err)
			continue
		}
		out := &agent.Message{Message: &pb.Message{
			Id:        m.Id,
			Type:      "analysis",
			Payload:   res,
			Timestamp: time.Now().Format(time.RFC3339),
		}}
		for _, o := range r.def.Outputs {
			if err := r.rt.Send(o.Target, out); err != nil {
				log.Printf("Error sending to %s: %v", o.Target, err)
			}
		}
	}
	return nil
}

func (r *ReActAgent) think(ctx context.Context, input string) (string, error) {
	// Use provider if available (HuggingFace or other provider-based implementations)
	if r.provider != nil {
		return r.thinkWithProvider(ctx, input)
	}

	// Fall back to OpenAI client
	if r.client == nil {
		return "", fmt.Errorf("no LLM client or provider configured")
	}

	return r.thinkWithOpenAI(ctx, input)
}

func (r *ReActAgent) thinkWithProvider(ctx context.Context, input string) (string, error) {
	// Build tools from both agent definition and MCP servers
	allTools := r.buildProviderTools()

	// Build messages
	messages := []provider.Message{
		{Role: "system", Content: r.def.Prompt},
		{Role: "user", Content: input},
	}

	req := provider.CompletionRequest{
		Messages:    messages,
		Model:       r.model,
		Tools:       allTools,
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	resp, err := r.provider.CreateCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("provider completion: %w", err)
	}

	// Handle tool calls
	if len(resp.ToolCalls) > 0 {
		call := resp.ToolCalls[0]
		result, err := r.executeProviderTool(ctx, call)
		if err != nil {
			return "", fmt.Errorf("tool execution: %w", err)
		}
		return fmt.Sprintf("Tool %s → %v", call.Function.Name, result), nil
	}

	return resp.Content, nil
}

func (r *ReActAgent) thinkWithOpenAI(ctx context.Context, input string) (string, error) {
	tools := make([]openai.Tool, len(r.def.Tools))
	for i, t := range r.def.Tools {
		tools[i] = openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  json.RawMessage(mustMarshal(t.InputSchema)),
			},
		}
	}

	req := openai.ChatCompletionRequest{
		Model: r.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: r.def.Prompt},
			{Role: "user", Content: input},
		},
		Tools: tools,
	}

	resp, err := r.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		call := resp.Choices[0].Message.ToolCalls[0]
		var args map[string]any
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		toolFunc, ok := r.tools[call.Function.Name]
		if !ok {
			return "", fmt.Errorf("unknown tool: %s", call.Function.Name)
		}

		result, err := toolFunc(ctx, args)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Tool %s → %v", call.Function.Name, result), nil
	}
	return resp.Choices[0].Message.Content, nil
}

// buildProviderTools builds tools from agent definition and MCP servers
func (r *ReActAgent) buildProviderTools() []provider.Tool {
	var tools []provider.Tool

	// Add agent-defined tools
	for _, t := range r.def.Tools {
		tools = append(tools, provider.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  json.RawMessage(mustMarshal(t.InputSchema)),
		})
	}

	// Add MCP tools
	mcpTools := r.toolRegistry.ListTools()
	for _, mcpTool := range mcpTools {
		schemaJSON, _ := json.Marshal(mcpTool.Schema)
		tools = append(tools, provider.Tool{
			Name:        mcpTool.Name,
			Description: mcpTool.Description,
			Parameters:  schemaJSON,
		})
	}

	return tools
}

// executeProviderTool executes a tool call from the provider
func (r *ReActAgent) executeProviderTool(ctx context.Context, call provider.ToolCall) (any, error) {
	// 1. Validate tool name format
	if err := security.ValidateToolName(call.Function.Name); err != nil {
		return nil, fmt.Errorf("invalid tool name: %w", err)
	}

	// 2. Unmarshal arguments
	var args map[string]any
	if err := json.Unmarshal(call.Function.Arguments, &args); err != nil {
		return nil, fmt.Errorf("unmarshal arguments: %w", err)
	}

	// 3. Sanitize string arguments
	for key, value := range args {
		if str, ok := value.(string); ok {
			// Check for suspicious content
			if containsSuspiciousContent(str) {
				return nil, fmt.Errorf("suspicious content in argument %s", key)
			}
			// Sanitize
			args[key] = security.SanitizeString(str)
		}
	}

	// 4. Check if agent-defined tool
	if toolFunc, ok := r.tools[call.Function.Name]; ok {
		return toolFunc(ctx, args)
	}

	// 5. Check if MCP tool
	if r.toolRegistry.HasTool(call.Function.Name) {
		return r.executeMCPTool(ctx, call.Function.Name, args)
	}

	return nil, fmt.Errorf("unknown tool: %s", call.Function.Name)
}

// containsSuspiciousContent checks for injection patterns in tool arguments
func containsSuspiciousContent(s string) bool {
	suspicious := []string{
		"../",                // Path traversal
		"&&", "||", ";", "|", // Command injection
		"<script",     // XSS
		"javascript:", // XSS
		"drop table",  // SQL injection
		"delete from", // SQL injection
	}

	lower := strings.ToLower(s)
	for _, pattern := range suspicious {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// executeMCPTool executes an MCP tool
func (r *ReActAgent) executeMCPTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	// Find which server hosts this tool
	serverName := r.toolRegistry.GetServer(toolName)
	if serverName == "" {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	session, exists := r.mcpSessions[serverName]
	if !exists {
		return "", fmt.Errorf("session not found for server: %s", serverName)
	}

	// Call tool
	result, err := session.CallTool(ctx, mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}

	// Format result as string
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}

	return "", fmt.Errorf("empty result from tool: %s", toolName)
}

// getAPIKeyFromEnv returns the appropriate API key from environment variables based on model name
func getAPIKeyFromEnv(model string) string {
	modelLower := strings.ToLower(model)

	// Try model-specific keys first
	if strings.Contains(modelLower, "grok") || strings.Contains(modelLower, "xai") {
		if key := os.Getenv("XAI_API_KEY"); key != "" {
			return key
		}
	}

	if strings.Contains(modelLower, "gpt") || strings.Contains(modelLower, "openai") {
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			return key
		}
	}

	if strings.Contains(modelLower, "claude") || strings.Contains(modelLower, "anthropic") {
		if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
			return key
		}
	}

	// For HuggingFace models
	if isHuggingFaceModel(model) {
		if key := os.Getenv("HUGGINGFACE_API_KEY"); key != "" {
			return key
		}
	}

	// Fall back to generic keys
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("HUGGINGFACE_API_KEY"); key != "" {
		return key
	}

	return ""
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// This should never happen with valid schema, but handle it gracefully
		log.Printf("Warning: failed to marshal value: %v", err)
		return []byte("{}")
	}
	return b
}
