package provider

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/security"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrock/types"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// Note: This package uses crypto/rand for jitter generation to satisfy security
// scanners and provide better entropy. For non-security-critical randomness,
// math/rand would be sufficient, but crypto/rand is used for defense in depth.

const (
	bedrockMaxRetries    = 5
	bedrockBaseDelay     = 1 * time.Second
	bedrockMaxDelay      = 32 * time.Second
	bedrockJitterFactor  = 0.3
	bedrockClientTimeout = 30 * time.Second
	bedrockDefaultRegion = "us-east-1"
)

func init() {
	RegisterFactory("bedrock", func(cfg map[string]any) (Provider, error) {
		region := ""
		if r, ok := cfg["region"].(string); ok {
			region = r
		}
		if region == "" {
			region = os.Getenv("AWS_REGION")
		}
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
		}
		if region == "" {
			region = bedrockDefaultRegion
		}

		return NewBedrockProvider(region)
	})
}

// BedrockProvider implements Provider for Amazon Bedrock using the AWS SDK v2.
// It uses the Converse API for unified messaging across all Bedrock models.
type BedrockProvider struct {
	region        string
	runtimeClient *bedrockruntime.Client
	bedrockClient *bedrock.Client
}

// NewBedrockProvider creates a new Amazon Bedrock provider using AWS SDK v2.
// It uses the AWS SDK default credential chain for authentication:
// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
// 2. Shared credentials file (~/.aws/credentials)
// 3. IAM role for EC2/ECS/EKS
// 4. AWS SSO
//
// Security: All API calls respect the context deadline. Callers should set
// appropriate timeouts (recommended: 60-120s for completion, 180s for streaming).
func NewBedrockProvider(region string) (*BedrockProvider, error) {
	// Add timeout for client creation to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), bedrockClientTimeout)
	defer cancel()

	// Load AWS configuration with the specified region
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Verify credentials are available
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}
	if creds.AccessKeyID == "" {
		return nil, fmt.Errorf("AWS credentials not configured")
	}

	// Create Bedrock runtime client for inference
	runtimeClient := bedrockruntime.NewFromConfig(cfg)

	// Create Bedrock client for model listing
	bedrockClient := bedrock.NewFromConfig(cfg)

	// Only log in debug mode to avoid leaking region info.
	// #nosec G706 -- region is sanitised via security.SanitizeLogField before formatting.
	if os.Getenv("AIXGO_DEBUG") == "true" {
		log.Printf("[Bedrock] Initialized client (region=%s)", security.SanitizeLogField(region))
	}

	return &BedrockProvider{
		region:        region,
		runtimeClient: runtimeClient,
		bedrockClient: bedrockClient,
	}, nil
}

// Name returns the provider name
func (p *BedrockProvider) Name() string {
	return "bedrock"
}

// CreateCompletion creates a completion using the Converse API
func (p *BedrockProvider) CreateCompletion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	modelID := p.normalizeModelID(req.Model)
	if modelID == "" {
		modelID = "anthropic.claude-3-haiku-20240307-v1:0"
	}

	// Build Converse input
	input, err := p.buildConverseInput(req, modelID)
	if err != nil {
		return nil, err
	}

	var resp *bedrockruntime.ConverseOutput

	// Retry logic with exponential backoff and jitter
	for attempt := 0; attempt < bedrockMaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err = p.runtimeClient.Converse(ctx, input)
		if err == nil {
			break
		}

		// Check if error is retryable
		if !p.isRetryableError(err) {
			return nil, p.wrapError(err)
		}
	}

	if err != nil {
		return nil, p.wrapError(err)
	}

	return p.parseConverseResponse(resp)
}

// CreateStructured creates a structured response using tool-based approach
func (p *BedrockProvider) CreateStructured(ctx context.Context, req StructuredRequest) (*StructuredResponse, error) {
	modelID := p.normalizeModelID(req.Model)
	if modelID == "" {
		modelID = "anthropic.claude-3-haiku-20240307-v1:0"
	}

	// Add schema as a tool if provided (same pattern as Anthropic)
	modReq := req.CompletionRequest
	if len(req.ResponseSchema) > 0 {
		modReq.Tools = append(modReq.Tools, Tool{
			Name:        "structured_output",
			Description: "Use this tool to provide your structured response",
			Parameters:  req.ResponseSchema,
		})
		// Append instruction to use the tool
		if len(modReq.Messages) > 0 {
			lastIdx := len(modReq.Messages) - 1
			modReq.Messages[lastIdx].Content += "\n\nPlease use the structured_output tool to provide your response in the required format."
		}
	}

	// Build Converse input
	input, err := p.buildConverseInput(modReq, modelID)
	if err != nil {
		return nil, err
	}

	var resp *bedrockruntime.ConverseOutput

	// Retry logic with exponential backoff and jitter
	for attempt := 0; attempt < bedrockMaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err = p.runtimeClient.Converse(ctx, input)
		if err == nil {
			break
		}

		if !p.isRetryableError(err) {
			return nil, p.wrapError(err)
		}
	}

	if err != nil {
		return nil, p.wrapError(err)
	}

	compResp, err := p.parseConverseResponse(resp)
	if err != nil {
		return nil, err
	}

	// Extract structured data from tool_use
	var data json.RawMessage
	for _, tc := range compResp.ToolCalls {
		if tc.Function.Name == "structured_output" {
			data = tc.Function.Arguments
			break
		}
	}

	if len(data) == 0 {
		data = json.RawMessage(compResp.Content)
	}

	return &StructuredResponse{
		Data:               data,
		CompletionResponse: *compResp,
	}, nil
}

// CreateStreaming creates a streaming response using ConverseStream
func (p *BedrockProvider) CreateStreaming(ctx context.Context, req CompletionRequest) (Stream, error) {
	modelID := p.normalizeModelID(req.Model)
	if modelID == "" {
		modelID = "anthropic.claude-3-haiku-20240307-v1:0"
	}

	// Build Converse input
	converseInput, err := p.buildConverseInput(req, modelID)
	if err != nil {
		return nil, err
	}

	// Convert to ConverseStreamInput
	streamInput := &bedrockruntime.ConverseStreamInput{
		ModelId:         converseInput.ModelId,
		Messages:        converseInput.Messages,
		System:          converseInput.System,
		InferenceConfig: converseInput.InferenceConfig,
		ToolConfig:      converseInput.ToolConfig,
	}

	// Start streaming
	resp, err := p.runtimeClient.ConverseStream(ctx, streamInput)
	if err != nil {
		return nil, p.wrapError(err)
	}

	// Use cancellable context to allow cleanup via Close()
	streamCtx, cancel := context.WithCancel(ctx)

	eventStream := resp.GetStream()

	return &bedrockStream{
		eventStream: eventStream,
		eventChan:   eventStream.Events(),
		ctx:         streamCtx,
		cancel:      cancel,
	}, nil
}

// ListModels returns available models from Amazon Bedrock
func (p *BedrockProvider) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// List foundation models from Bedrock
	input := &bedrock.ListFoundationModelsInput{
		// Only list models available for inference
		ByInferenceType: bedrocktypes.InferenceTypeOnDemand,
	}

	resp, err := p.bedrockClient.ListFoundationModels(ctx, input)
	if err != nil {
		// If API fails, return known models as fallback
		return p.getKnownModels(), nil
	}

	var models []ModelInfo
	for _, m := range resp.ModelSummaries {
		modelID := aws.ToString(m.ModelId)
		info := ModelInfo{
			ID:       modelID,
			Name:     aws.ToString(m.ModelName),
			Provider: "bedrock",
		}

		// Add pricing and description if known
		if pricing, ok := bedrockModelPricing[modelID]; ok {
			info.InputCost = pricing.input
			info.OutputCost = pricing.output
			info.Description = pricing.description
		} else {
			provider := aws.ToString(m.ProviderName)
			info.Description = fmt.Sprintf("%s model via Amazon Bedrock", provider)
		}

		// Set capabilities based on model features
		if m.OutputModalities != nil {
			for _, mod := range m.OutputModalities {
				info.Capabilities = append(info.Capabilities, string(mod))
			}
		}

		models = append(models, info)
	}

	return models, nil
}

// Close closes the provider (no-op for Bedrock as SDK handles cleanup)
func (p *BedrockProvider) Close() error {
	return nil
}

// buildConverseInput builds the Converse API input from a CompletionRequest
func (p *BedrockProvider) buildConverseInput(req CompletionRequest, modelID string) (*bedrockruntime.ConverseInput, error) {
	var systemPrompts []types.SystemContentBlock
	var messages []types.Message

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompts = append(systemPrompts, &types.SystemContentBlockMemberText{
				Value: m.Content,
			})
			continue
		}

		role := p.convertRole(m.Role)
		messages = append(messages, types.Message{
			Role: role,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: m.Content},
			},
		})
	}

	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(modelID),
		Messages: messages,
	}

	if len(systemPrompts) > 0 {
		input.System = systemPrompts
	}

	// Build inference config
	inferenceConfig := &types.InferenceConfiguration{}
	hasConfig := false

	if req.Temperature > 0 {
		temp := float32(req.Temperature)
		inferenceConfig.Temperature = &temp
		hasConfig = true
	}

	if req.MaxTokens > 0 {
		// Safe int to int32 conversion with bounds checking
		maxTokens, err := security.SafeIntToInt32(req.MaxTokens)
		if err != nil {
			return nil, fmt.Errorf("max tokens out of range: %w", err)
		}
		inferenceConfig.MaxTokens = &maxTokens
		hasConfig = true
	}

	if hasConfig {
		input.InferenceConfig = inferenceConfig
	}

	// Add tools if provided
	if len(req.Tools) > 0 {
		toolConfig, err := p.buildToolConfig(req.Tools)
		if err != nil {
			return nil, err
		}
		input.ToolConfig = toolConfig
	}

	return input, nil
}

// buildToolConfig builds the ToolConfiguration for the Converse API
func (p *BedrockProvider) buildToolConfig(tools []Tool) (*types.ToolConfiguration, error) {
	var toolSpecs []types.Tool

	for _, t := range tools {
		// Parse JSON schema for the tool
		var schema map[string]any
		if len(t.Parameters) > 0 {
			if err := json.Unmarshal(t.Parameters, &schema); err != nil {
				return nil, fmt.Errorf("invalid tool schema: %w", err)
			}
		}

		// Build input schema document using NewLazyDocument
		inputSchema := &types.ToolInputSchemaMemberJson{
			Value: document.NewLazyDocument(schema),
		}

		toolSpec := &types.ToolMemberToolSpec{
			Value: types.ToolSpecification{
				Name:        aws.String(t.Name),
				Description: aws.String(t.Description),
				InputSchema: inputSchema,
			},
		}

		toolSpecs = append(toolSpecs, toolSpec)
	}

	return &types.ToolConfiguration{
		Tools: toolSpecs,
	}, nil
}

// parseConverseResponse parses the Converse API response
func (p *BedrockProvider) parseConverseResponse(resp *bedrockruntime.ConverseOutput) (*CompletionResponse, error) {
	if resp == nil || resp.Output == nil {
		return nil, NewProviderError("bedrock", ErrorCodeUnknown, "no output in response", nil)
	}

	var content string
	var toolCalls []ToolCall

	// Extract content from response
	if msgOutput, ok := resp.Output.(*types.ConverseOutputMemberMessage); ok {
		for _, block := range msgOutput.Value.Content {
			switch b := block.(type) {
			case *types.ContentBlockMemberText:
				content += b.Value
			case *types.ContentBlockMemberToolUse:
				// Marshal tool input to JSON
				args, err := json.Marshal(b.Value.Input)
				if err != nil {
					args = []byte("{}")
				}

				toolCalls = append(toolCalls, ToolCall{
					ID:   aws.ToString(b.Value.ToolUseId),
					Type: "function",
					Function: FunctionCall{
						Name:      aws.ToString(b.Value.Name),
						Arguments: args,
					},
				})
			}
		}
	}

	// Convert stop reason
	finishReason := string(resp.StopReason)
	switch resp.StopReason {
	case types.StopReasonEndTurn:
		finishReason = "stop"
	case types.StopReasonToolUse:
		finishReason = "tool_calls"
	case types.StopReasonMaxTokens:
		finishReason = "length"
	case types.StopReasonStopSequence:
		finishReason = "stop"
	case types.StopReasonContentFiltered:
		finishReason = "content_filter"
	}

	// Get usage stats
	var usage Usage
	if resp.Usage != nil {
		usage.PromptTokens = int(aws.ToInt32(resp.Usage.InputTokens))
		usage.CompletionTokens = int(aws.ToInt32(resp.Usage.OutputTokens))
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return &CompletionResponse{
		Content:      content,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
		Usage:        usage,
		Raw:          resp,
	}, nil
}

// convertRole converts a message role to Bedrock ConversationRole
func (p *BedrockProvider) convertRole(role string) types.ConversationRole {
	switch role {
	case "assistant":
		return types.ConversationRoleAssistant
	default:
		return types.ConversationRoleUser
	}
}

// normalizeModelID normalizes model IDs by stripping the bedrock/ prefix
func (p *BedrockProvider) normalizeModelID(model string) string {
	if strings.HasPrefix(model, "bedrock/") {
		return model[8:]
	}
	return model
}

// wrapError converts AWS errors to ProviderError
func (p *BedrockProvider) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// Determine error code based on error message (case-insensitive)
	code := ErrorCodeUnknown
	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "accessdenied") || strings.Contains(errMsg, "forbidden") ||
		strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "credential"):
		code = ErrorCodeAuthentication
	case strings.Contains(errMsg, "throttling") || strings.Contains(errMsg, "rate") ||
		strings.Contains(errMsg, "too many requests"):
		code = ErrorCodeRateLimit
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "does not exist"):
		code = ErrorCodeModelNotFound
	case strings.Contains(errMsg, "validation") || strings.Contains(errMsg, "invalid"):
		code = ErrorCodeInvalidRequest
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline"):
		code = ErrorCodeTimeout
	case strings.Contains(errMsg, "service") || strings.Contains(errMsg, "internal"):
		code = ErrorCodeServerError
	case strings.Contains(errMsg, "content filter") || strings.Contains(errMsg, "guardrail"):
		code = ErrorCodeContentFiltered
	}

	return &ProviderError{
		Provider:      "bedrock",
		Code:          code,
		Message:       err.Error(), // Keep original case for display
		IsRetryable:   code == ErrorCodeRateLimit || code == ErrorCodeServerError || code == ErrorCodeTimeout,
		OriginalError: err,
	}
}

// isRetryableError checks if an error is retryable
func (p *BedrockProvider) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "throttling") ||
		strings.Contains(errMsg, "rate") ||
		strings.Contains(errMsg, "too many requests") ||
		strings.Contains(errMsg, "service") ||
		strings.Contains(errMsg, "internal") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline") ||
		strings.Contains(errMsg, "unavailable")
}

// calculateBackoff returns the backoff duration with jitter for a given attempt
func (p *BedrockProvider) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s (capped at maxDelay)
	// Clamp shift to [0, 31] using min/max built-ins for safe uint conversion
	shift := uint(max(0, min(attempt-1, 31))) // #nosec G115 -- clamped to [0,31]
	delay := min(time.Duration(1<<shift)*bedrockBaseDelay, bedrockMaxDelay)
	// Add jitter: delay ± 30% using crypto/rand for security compliance
	jitter := time.Duration(float64(delay) * bedrockJitterFactor * (bedrockCryptoRandFloat64()*2 - 1))
	return delay + jitter
}

// bedrockCryptoRandFloat64 returns a cryptographically secure random float64 in [0.0, 1.0)
func bedrockCryptoRandFloat64() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to deterministic value on error (should never happen)
		return 0.5
	}
	// Use top 53 bits to create a float64 in [0, 1)
	return float64(binary.BigEndian.Uint64(b[:])>>11) / (1 << 53)
}

// getKnownModels returns a list of known Bedrock models as fallback
func (p *BedrockProvider) getKnownModels() []ModelInfo {
	knownModels := []string{
		"anthropic.claude-3-5-sonnet-20240620-v1:0",
		"anthropic.claude-3-haiku-20240307-v1:0",
		"anthropic.claude-3-opus-20240229-v1:0",
		"anthropic.claude-3-sonnet-20240229-v1:0",
		"amazon.nova-pro-v1:0",
		"amazon.nova-lite-v1:0",
		"amazon.nova-micro-v1:0",
		"meta.llama3-70b-instruct-v1:0",
		"meta.llama3-8b-instruct-v1:0",
		"mistral.mistral-large-2407-v1:0",
		"amazon.titan-text-express-v1",
		"amazon.titan-text-lite-v1",
	}

	var models []ModelInfo
	for _, modelID := range knownModels {
		info := ModelInfo{
			ID:       modelID,
			Name:     modelID,
			Provider: "bedrock",
		}

		if pricing, ok := bedrockModelPricing[modelID]; ok {
			info.InputCost = pricing.input
			info.OutputCost = pricing.output
			info.Description = pricing.description
		} else {
			info.Description = "Amazon Bedrock model"
		}

		models = append(models, info)
	}

	return models
}

// bedrockModelPricing contains known pricing for Bedrock models (per 1M tokens)
var bedrockModelPricing = map[string]struct {
	input       float64
	output      float64
	description string
}{
	// Anthropic Claude on Bedrock
	"anthropic.claude-3-5-sonnet-20240620-v1:0": {3.00, 15.00, "Claude 3.5 Sonnet - intelligent model for complex tasks"},
	"anthropic.claude-3-haiku-20240307-v1:0":    {0.25, 1.25, "Claude 3 Haiku - fastest Claude model"},
	"anthropic.claude-3-opus-20240229-v1:0":     {15.00, 75.00, "Claude 3 Opus - most capable Claude model"},
	"anthropic.claude-3-sonnet-20240229-v1:0":   {3.00, 15.00, "Claude 3 Sonnet - balanced performance"},
	"anthropic.claude-opus-4-20250514-v1:0":     {15.00, 75.00, "Claude Opus 4 - latest Opus model"},

	// Amazon Nova
	"amazon.nova-pro-v1:0":   {0.80, 3.20, "Amazon Nova Pro - best multimodal understanding"},
	"amazon.nova-lite-v1:0":  {0.06, 0.24, "Amazon Nova Lite - cost-effective multimodal"},
	"amazon.nova-micro-v1:0": {0.035, 0.14, "Amazon Nova Micro - text-only speed optimized"},

	// Meta Llama
	"meta.llama3-70b-instruct-v1:0":   {2.65, 3.50, "Llama 3 70B - large instruction-tuned"},
	"meta.llama3-8b-instruct-v1:0":    {0.30, 0.60, "Llama 3 8B - efficient instruction-tuned"},
	"meta.llama4-maverick-17b-v1:0":   {0.50, 1.00, "Llama 4 Maverick 17B - optimized for speed"},
	"meta.llama4-scout-17b-v1:0":      {0.50, 1.00, "Llama 4 Scout 17B - exploration optimized"},
	"meta.llama4-behemoth-405b-v1:0":  {5.00, 15.00, "Llama 4 Behemoth 405B - largest Llama model"},

	// Mistral
	"mistral.mistral-large-2407-v1:0": {4.00, 12.00, "Mistral Large - flagship model"},
	"mistral.mistral-7b-instruct-v0:2": {0.15, 0.20, "Mistral 7B Instruct"},

	// Amazon Titan
	"amazon.titan-text-express-v1": {0.20, 0.60, "Titan Text Express - fast and efficient"},
	"amazon.titan-text-lite-v1":    {0.15, 0.20, "Titan Text Lite - lightweight model"},
	"amazon.titan-text-premier-v1:0": {0.50, 1.50, "Titan Text Premier - advanced capabilities"},

	// Cohere
	"cohere.command-r-plus-v1:0": {3.00, 15.00, "Command R+ - enterprise RAG"},
	"cohere.command-r-v1:0":      {0.50, 1.50, "Command R - efficient RAG"},

	// AI21 Labs
	"ai21.jamba-1-5-large-v1:0": {2.00, 8.00, "Jamba 1.5 Large - SSM architecture"},
	"ai21.jamba-1-5-mini-v1:0":  {0.20, 0.40, "Jamba 1.5 Mini - efficient SSM"},
}

// bedrockStream implements Stream for Amazon Bedrock using ConverseStream
type bedrockStream struct {
	eventStream *bedrockruntime.ConverseStreamEventStream
	eventChan   <-chan types.ConverseStreamOutput
	ctx         context.Context
	cancel      context.CancelFunc
	done        bool
}

func (s *bedrockStream) Recv() (*StreamChunk, error) {
	if s.done {
		return &StreamChunk{FinishReason: "stop"}, io.EOF
	}

	// Use select to read from event channel with context cancellation
	select {
	case <-s.ctx.Done():
		s.done = true
		return nil, s.ctx.Err()
	case event, ok := <-s.eventChan:
		if !ok {
			// Channel closed
			s.done = true
			// Check for stream errors
			if err := s.eventStream.Err(); err != nil {
				return nil, err
			}
			return &StreamChunk{FinishReason: "stop"}, io.EOF
		}

		// Process stream events
		switch e := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if textDelta, ok := e.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
				return &StreamChunk{Delta: textDelta.Value}, nil
			}
			if toolDelta, ok := e.Value.Delta.(*types.ContentBlockDeltaMemberToolUse); ok {
				return &StreamChunk{
					ToolCallDeltas: []ToolCallDelta{{
						Index:         int(aws.ToInt32(e.Value.ContentBlockIndex)),
						ArgumentDelta: aws.ToString(toolDelta.Value.Input),
					}},
				}, nil
			}

		case *types.ConverseStreamOutputMemberContentBlockStart:
			if toolStart, ok := e.Value.Start.(*types.ContentBlockStartMemberToolUse); ok {
				return &StreamChunk{
					ToolCallDeltas: []ToolCallDelta{{
						Index:        int(aws.ToInt32(e.Value.ContentBlockIndex)),
						ID:           aws.ToString(toolStart.Value.ToolUseId),
						Type:         "function",
						FunctionName: aws.ToString(toolStart.Value.Name),
					}},
				}, nil
			}

		case *types.ConverseStreamOutputMemberMessageStop:
			s.done = true
			finishReason := "stop"
			switch e.Value.StopReason {
			case types.StopReasonToolUse:
				finishReason = "tool_calls"
			case types.StopReasonMaxTokens:
				finishReason = "length"
			case types.StopReasonContentFiltered:
				finishReason = "content_filter"
			}
			return &StreamChunk{FinishReason: finishReason}, nil

		case *types.ConverseStreamOutputMemberMetadata:
			// Usage metadata - continue to next event
		}

		// Return empty chunk for unhandled events
		return &StreamChunk{}, nil
	}
}

func (s *bedrockStream) Close() error {
	if s.done {
		return nil
	}
	s.done = true

	// Cancel the streaming context to signal cleanup
	if s.cancel != nil {
		s.cancel()
	}

	// Close the event stream
	if s.eventStream != nil {
		return s.eventStream.Close()
	}

	return nil
}
