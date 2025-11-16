# Aixgo v0.1 Feature Implementation Specification

**Status:** Pre-Launch Implementation Required
**Priority:** Critical - Must be completed before website publication
**Estimated Effort:** 3-5 days of focused development

## Executive Summary

The Aixgo documentation website describes features that are not yet implemented in the codebase. This specification provides detailed implementation guidance for all missing features to ensure documentation accuracy before launch.

**Current State:** Basic multi-agent system with YAML config, local Go channels runtime, hardcoded xAI provider
**Target State:** Full-featured framework matching all documented capabilities

---

## Implementation Priority Order

### Phase 1: Foundation (Day 1-2)
1. Configuration schema extensions
2. Type-safe Go API
3. Provider abstraction and registry

### Phase 2: Core Features (Day 2-3)
4. Multi-provider implementations (OpenAI, Anthropic, Vertex AI, HuggingFace)
5. Enhanced supervisor orchestration
6. Retry and timeout logic

### Phase 3: Advanced (Day 3-4)
7. Enhanced observability with metrics and trace attributes
8. Distributed mode with gRPC (basic implementation)

### Phase 4: Extensions (Day 4-5)
9. Vector database integrations (Firestore, Qdrant)
10. Langfuse integration

---

## Feature 1: Type-Safe Go API (Priority: CRITICAL)

### Objective
Create a public Go API with builder pattern for programmatic agent creation, replacing YAML-only configuration.

### Current Problem
Documentation shows:
```go
agent := aixgo.NewAgent(
    aixgo.WithName("analyzer"),
    aixgo.WithModel("grok-beta"),
)
```

But this API doesn't exist. Only YAML configuration works.

### Implementation Details

#### 1.1 Create Public API Types

**File:** `types.go` (root package)

```go
package aixgo

import "time"

// AgentRole represents the type of agent
type AgentRole string

const (
    RoleProducer AgentRole = "producer"
    RoleReAct    AgentRole = "react"
    RoleLogger   AgentRole = "logger"
)

// AgentConfig holds agent configuration
type AgentConfig struct {
    Name        string
    Role        AgentRole
    Model       string
    Provider    string
    APIKey      string
    Endpoint    string
    Temperature *float64
    MaxTokens   *int
    Timeout     time.Duration
    Prompt      string
    Interval    time.Duration
    Inputs      []Input
    Outputs     []Output
    Tools       []Tool
    Retry       *RetryConfig
}

// Input defines agent input source
type Input struct {
    Source string
}

// Output defines agent output target
type Output struct {
    Target string
}

// Tool defines a tool available to ReAct agents
type Tool struct {
    Name        string
    Description string
    InputSchema interface{}
    Handler     ToolHandler
}

// ToolHandler processes tool invocations
type ToolHandler func(input interface{}) (interface{}, error)

// RetryConfig configures retry behavior
type RetryConfig struct {
    MaxAttempts    int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    Multiplier     float64
    RetryOn        []string
}

// Agent represents a running agent instance
type Agent interface {
    Start(ctx context.Context) error
    Stop() error
    Name() string
    Role() AgentRole
}
```

#### 1.2 Create Option Functions

**File:** `options.go` (root package)

```go
package aixgo

import "time"

// AgentOption configures an agent
type AgentOption func(*AgentConfig) error

// WithName sets the agent name
func WithName(name string) AgentOption {
    return func(c *AgentConfig) error {
        if name == "" {
            return fmt.Errorf("agent name cannot be empty")
        }
        c.Name = name
        return nil
    }
}

// WithRole sets the agent role
func WithRole(role AgentRole) AgentOption {
    return func(c *AgentConfig) error {
        if role != RoleProducer && role != RoleReAct && role != RoleLogger {
            return fmt.Errorf("invalid role: %s", role)
        }
        c.Role = role
        return nil
    }
}

// WithModel sets the LLM model
func WithModel(model string) AgentOption {
    return func(c *AgentConfig) error {
        c.Model = model
        return nil
    }
}

// WithProvider sets the LLM provider
func WithProvider(provider string) AgentOption {
    return func(c *AgentConfig) error {
        c.Provider = provider
        return nil
    }
}

// WithAPIKey sets the provider API key
func WithAPIKey(key string) AgentOption {
    return func(c *AgentConfig) error {
        c.APIKey = key
        return nil
    }
}

// WithTemperature sets the LLM temperature
func WithTemperature(temp float64) AgentOption {
    return func(c *AgentConfig) error {
        if temp < 0 || temp > 2 {
            return fmt.Errorf("temperature must be between 0 and 2")
        }
        c.Temperature = &temp
        return nil
    }
}

// WithMaxTokens sets the max tokens
func WithMaxTokens(tokens int) AgentOption {
    return func(c *AgentConfig) error {
        if tokens < 1 {
            return fmt.Errorf("max_tokens must be positive")
        }
        c.MaxTokens = &tokens
        return nil
    }
}

// WithTimeout sets the agent timeout
func WithTimeout(timeout time.Duration) AgentOption {
    return func(c *AgentConfig) error {
        c.Timeout = timeout
        return nil
    }
}

// WithPrompt sets the agent prompt
func WithPrompt(prompt string) AgentOption {
    return func(c *AgentConfig) error {
        c.Prompt = prompt
        return nil
    }
}

// WithInterval sets the producer interval
func WithInterval(interval time.Duration) AgentOption {
    return func(c *AgentConfig) error {
        c.Interval = interval
        return nil
    }
}

// WithInputs sets agent inputs
func WithInputs(inputs ...Input) AgentOption {
    return func(c *AgentConfig) error {
        c.Inputs = inputs
        return nil
    }
}

// WithOutputs sets agent outputs
func WithOutputs(outputs ...Output) AgentOption {
    return func(c *AgentConfig) error {
        c.Outputs = outputs
        return nil
    }
}

// WithTools sets agent tools
func WithTools(tools ...Tool) AgentOption {
    return func(c *AgentConfig) error {
        c.Tools = tools
        return nil
    }
}

// WithRetry sets retry configuration
func WithRetry(retry RetryConfig) AgentOption {
    return func(c *AgentConfig) error {
        c.Retry = &retry
        return nil
    }
}
```

#### 1.3 Create Agent Constructor

**File:** `agent.go` (root package)

```go
package aixgo

import (
    "context"
    "fmt"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/runtime"
)

// NewAgent creates a new agent with the given options
func NewAgent(opts ...AgentOption) (Agent, error) {
    // Apply options to config
    config := &AgentConfig{}
    for _, opt := range opts {
        if err := opt(config); err != nil {
            return nil, fmt.Errorf("invalid option: %w", err)
        }
    }

    // Validate required fields
    if config.Name == "" {
        return nil, fmt.Errorf("agent name is required")
    }
    if config.Role == "" {
        return nil, fmt.Errorf("agent role is required")
    }

    // Convert to internal AgentDef
    def := agent.AgentDef{
        Name:     config.Name,
        Role:     string(config.Role),
        Model:    config.Model,
        Prompt:   config.Prompt,
        Interval: config.Interval,
        Inputs:   convertInputs(config.Inputs),
        Outputs:  convertOutputs(config.Outputs),
        Tools:    convertTools(config.Tools),
    }

    // Create runtime (default to local)
    rt := runtime.NewSimpleRuntime()

    // Create agent using internal factory
    return agent.CreateAgent(def, rt)
}

// Helper functions to convert public types to internal types
func convertInputs(inputs []Input) []agent.Input {
    result := make([]agent.Input, len(inputs))
    for i, inp := range inputs {
        result[i] = agent.Input{Source: inp.Source}
    }
    return result
}

func convertOutputs(outputs []Output) []agent.Output {
    result := make([]agent.Output, len(outputs))
    for i, out := range outputs {
        result[i] = agent.Output{Target: out.Target}
    }
    return result
}

func convertTools(tools []Tool) []agent.Tool {
    result := make([]agent.Tool, len(tools))
    for i, tool := range tools {
        result[i] = agent.Tool{
            Name:        tool.Name,
            Description: tool.Description,
            InputSchema: tool.InputSchema,
        }
    }
    return result
}
```

---

## Feature 2: Configuration Schema Extensions (Priority: CRITICAL)

### Objective
Extend internal configuration structs to support all documented YAML options.

### Current Problem
Documentation shows config options that don't exist in `AgentDef` struct.

### Implementation Details

#### 2.1 Update AgentDef

**File:** `internal/agent/types.go`

```go
package agent

import "time"

type AgentDef struct {
    Name     string        `yaml:"name"`
    Role     string        `yaml:"role"`
    Model    string        `yaml:"model,omitempty"`
    Prompt   string        `yaml:"prompt,omitempty"`

    // Provider configuration
    Provider    string        `yaml:"provider,omitempty"`
    APIKey      string        `yaml:"api_key,omitempty"`
    Endpoint    string        `yaml:"endpoint,omitempty"`

    // LLM parameters
    Temperature *float64      `yaml:"temperature,omitempty"`
    MaxTokens   *int          `yaml:"max_tokens,omitempty"`

    // Execution configuration
    Timeout     time.Duration `yaml:"timeout,omitempty"`
    Interval    time.Duration `yaml:"interval,omitempty"`

    // Retry configuration
    Retry       *RetryConfig  `yaml:"retry,omitempty"`

    // Message routing
    Inputs   []Input  `yaml:"inputs,omitempty"`
    Outputs  []Output `yaml:"outputs,omitempty"`

    // Tools (for ReAct agents)
    Tools    []Tool   `yaml:"tools,omitempty"`
}

type RetryConfig struct {
    MaxAttempts    int           `yaml:"max_attempts"`
    InitialBackoff time.Duration `yaml:"initial_backoff"`
    MaxBackoff     time.Duration `yaml:"max_backoff"`
    Multiplier     float64       `yaml:"multiplier"`
    RetryOn        []string      `yaml:"retry_on"`
}

// Existing types remain
type Input struct {
    Source string `yaml:"source"`
}

type Output struct {
    Target string `yaml:"target"`
}

type Tool struct {
    Name        string      `yaml:"name"`
    Description string      `yaml:"description"`
    InputSchema interface{} `yaml:"input_schema"`
}
```

#### 2.2 Update SupervisorDef

**File:** `internal/config/types.go`

```go
package config

import "time"

type SupervisorDef struct {
    Name      string `yaml:"name"`
    Model     string `yaml:"model,omitempty"`
    MaxRounds int    `yaml:"max_rounds"`

    // Runtime mode
    Mode        string        `yaml:"mode,omitempty"` // "local" or "distributed"
    Timeout     time.Duration `yaml:"timeout,omitempty"`
    FailureMode string        `yaml:"failure_mode,omitempty"` // "stop" or "continue"
}

type ObservabilityConfig struct {
    // Tracing
    Tracing      bool              `yaml:"tracing"`
    ServiceName  string            `yaml:"service_name,omitempty"`
    Exporter     string            `yaml:"exporter,omitempty"`
    Endpoint     string            `yaml:"endpoint,omitempty"`
    SamplingRate float64           `yaml:"sampling_rate,omitempty"`
    Headers      map[string]string `yaml:"headers,omitempty"`
    Debug        bool              `yaml:"debug,omitempty"`
    SpanTimeout  time.Duration     `yaml:"span_timeout,omitempty"`

    // Metrics
    Metrics     bool `yaml:"metrics,omitempty"`
    MetricsPort int  `yaml:"metrics_port,omitempty"`

    // LLM-specific observability
    LLMObservability *LLMObservabilityConfig `yaml:"llm_observability,omitempty"`

    // Cost tracking
    CostTracking        bool    `yaml:"cost_tracking,omitempty"`
    CostAlertThreshold  float64 `yaml:"cost_alert_threshold,omitempty"`
    TrackTokens         bool    `yaml:"track_tokens,omitempty"`
    DailyTokenLimit     int     `yaml:"daily_token_limit,omitempty"`
}

type LLMObservabilityConfig struct {
    Enabled   bool   `yaml:"enabled"`
    Provider  string `yaml:"provider"`  // "langfuse", etc.
    Endpoint  string `yaml:"endpoint"`
    PublicKey string `yaml:"public_key"`
    SecretKey string `yaml:"secret_key"`
}

type VectorStoreConfig struct {
    Provider       string `yaml:"provider"` // "firestore", "qdrant", "pgvector"
    ProjectID      string `yaml:"project_id,omitempty"`      // Firestore
    Collection     string `yaml:"collection,omitempty"`      // Firestore
    Endpoint       string `yaml:"endpoint,omitempty"`        // Qdrant
    ConnectionString string `yaml:"connection_string,omitempty"` // pgvector
    Table          string `yaml:"table,omitempty"`           // pgvector
    EmbeddingModel string `yaml:"embedding_model"`
}

type Config struct {
    Supervisor    SupervisorDef       `yaml:"supervisor"`
    Agents        []AgentDef          `yaml:"agents"`
    Observability ObservabilityConfig `yaml:"observability,omitempty"`
    VectorStore   *VectorStoreConfig  `yaml:"vector_store,omitempty"`
}
```

---

## Feature 3: Multi-Provider Support (Priority: CRITICAL)

### Objective
Support multiple LLM providers (OpenAI, Anthropic, Vertex AI, HuggingFace, xAI) with a unified interface.

### Current Problem
Only hardcoded xAI client exists. Documentation claims support for multiple providers.

### Implementation Details

#### 3.1 Create Provider Interface

**File:** `internal/llm/provider/provider.go`

```go
package provider

import (
    "context"
    "time"
)

// Provider is the interface all LLM providers must implement
type Provider interface {
    // Chat sends a chat completion request
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

    // Embeddings generates embeddings for text
    Embeddings(ctx context.Context, req EmbeddingsRequest) (*EmbeddingsResponse, error)

    // Name returns the provider name
    Name() string
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
    Model       string
    Messages    []Message
    Temperature *float64
    MaxTokens   *int
    Tools       []Tool
    ToolChoice  string
}

// Message represents a chat message
type Message struct {
    Role    string      `json:"role"`    // "system", "user", "assistant", "tool"
    Content string      `json:"content"`
    Name    string      `json:"name,omitempty"`
    ToolCallID string   `json:"tool_call_id,omitempty"`
}

// Tool represents a function/tool definition
type Tool struct {
    Type     string   `json:"type"` // "function"
    Function Function `json:"function"`
}

// Function defines a callable function
type Function struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Parameters  interface{} `json:"parameters"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
    ID      string
    Model   string
    Choices []Choice
    Usage   TokenUsage
}

// Choice represents a completion choice
type Choice struct {
    Message      Message
    FinishReason string
    Index        int
}

// TokenUsage tracks token consumption
type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

// EmbeddingsRequest represents an embedding request
type EmbeddingsRequest struct {
    Model string
    Input []string
}

// EmbeddingsResponse represents an embedding response
type EmbeddingsResponse struct {
    Model      string
    Embeddings [][]float64
    Usage      TokenUsage
}
```

#### 3.2 Create Provider Registry

**File:** `internal/llm/provider/registry.go`

```go
package provider

import (
    "fmt"
    "sync"
)

// ProviderFactory creates a provider instance
type ProviderFactory func(config Config) (Provider, error)

// Config holds provider configuration
type Config struct {
    APIKey      string
    Endpoint    string
    ProjectID   string            // For Vertex AI
    Location    string            // For Vertex AI
    Headers     map[string]string // Custom headers
}

var (
    registry = make(map[string]ProviderFactory)
    mu       sync.RWMutex
)

// Register registers a provider factory
func Register(name string, factory ProviderFactory) {
    mu.Lock()
    defer mu.Unlock()
    registry[name] = factory
}

// Get returns a provider by name
func Get(name string, config Config) (Provider, error) {
    mu.RLock()
    factory, ok := registry[name]
    mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("unknown provider: %s", name)
    }

    return factory(config)
}

// List returns all registered provider names
func List() []string {
    mu.RLock()
    defer mu.RUnlock()

    names := make([]string, 0, len(registry))
    for name := range registry {
        names = append(names, name)
    }
    return names
}
```

#### 3.3 Implement OpenAI Provider

**File:** `internal/llm/provider/openai/openai.go`

```go
package openai

import (
    "context"
    "fmt"

    "github.com/aixgo-dev/aixgo/internal/llm/provider"
    "github.com/sashabaranov/go-openai"
)

func init() {
    provider.Register("openai", New)
}

type OpenAIProvider struct {
    client *openai.Client
}

func New(config provider.Config) (provider.Provider, error) {
    if config.APIKey == "" {
        return nil, fmt.Errorf("OpenAI API key is required")
    }

    clientConfig := openai.DefaultConfig(config.APIKey)
    if config.Endpoint != "" {
        clientConfig.BaseURL = config.Endpoint
    }

    return &OpenAIProvider{
        client: openai.NewClientWithConfig(clientConfig),
    }, nil
}

func (p *OpenAIProvider) Name() string {
    return "openai"
}

func (p *OpenAIProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
    // Convert provider.ChatRequest to openai.ChatCompletionRequest
    messages := make([]openai.ChatCompletionMessage, len(req.Messages))
    for i, msg := range req.Messages {
        messages[i] = openai.ChatCompletionMessage{
            Role:    msg.Role,
            Content: msg.Content,
            Name:    msg.Name,
        }
    }

    oaiReq := openai.ChatCompletionRequest{
        Model:    req.Model,
        Messages: messages,
    }

    if req.Temperature != nil {
        oaiReq.Temperature = float32(*req.Temperature)
    }

    if req.MaxTokens != nil {
        oaiReq.MaxTokens = *req.MaxTokens
    }

    if len(req.Tools) > 0 {
        tools := make([]openai.Tool, len(req.Tools))
        for i, tool := range req.Tools {
            tools[i] = openai.Tool{
                Type: openai.ToolType(tool.Type),
                Function: &openai.FunctionDefinition{
                    Name:        tool.Function.Name,
                    Description: tool.Function.Description,
                    Parameters:  tool.Function.Parameters,
                },
            }
        }
        oaiReq.Tools = tools
    }

    // Make API call
    resp, err := p.client.CreateChatCompletion(ctx, oaiReq)
    if err != nil {
        return nil, fmt.Errorf("OpenAI API error: %w", err)
    }

    // Convert response
    choices := make([]provider.Choice, len(resp.Choices))
    for i, choice := range resp.Choices {
        choices[i] = provider.Choice{
            Message: provider.Message{
                Role:    choice.Message.Role,
                Content: choice.Message.Content,
            },
            FinishReason: string(choice.FinishReason),
            Index:        choice.Index,
        }
    }

    return &provider.ChatResponse{
        ID:      resp.ID,
        Model:   resp.Model,
        Choices: choices,
        Usage: provider.TokenUsage{
            InputTokens:  resp.Usage.PromptTokens,
            OutputTokens: resp.Usage.CompletionTokens,
            TotalTokens:  resp.Usage.TotalTokens,
        },
    }, nil
}

func (p *OpenAIProvider) Embeddings(ctx context.Context, req provider.EmbeddingsRequest) (*provider.EmbeddingsResponse, error) {
    oaiReq := openai.EmbeddingRequest{
        Model: openai.EmbeddingModel(req.Model),
        Input: req.Input,
    }

    resp, err := p.client.CreateEmbeddings(ctx, oaiReq)
    if err != nil {
        return nil, fmt.Errorf("OpenAI embeddings error: %w", err)
    }

    embeddings := make([][]float64, len(resp.Data))
    for i, data := range resp.Data {
        embeddings[i] = make([]float64, len(data.Embedding))
        for j, val := range data.Embedding {
            embeddings[i][j] = float64(val)
        }
    }

    return &provider.EmbeddingsResponse{
        Model:      string(resp.Model),
        Embeddings: embeddings,
        Usage: provider.TokenUsage{
            InputTokens: resp.Usage.PromptTokens,
            TotalTokens: resp.Usage.TotalTokens,
        },
    }, nil
}
```

#### 3.4 Implement Anthropic Provider

**File:** `internal/llm/provider/anthropic/anthropic.go`

```go
package anthropic

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/aixgo-dev/aixgo/internal/llm/provider"
)

func init() {
    provider.Register("anthropic", New)
}

const defaultEndpoint = "https://api.anthropic.com/v1"

type AnthropicProvider struct {
    apiKey   string
    endpoint string
    client   *http.Client
}

func New(config provider.Config) (provider.Provider, error) {
    if config.APIKey == "" {
        return nil, fmt.Errorf("Anthropic API key is required")
    }

    endpoint := config.Endpoint
    if endpoint == "" {
        endpoint = defaultEndpoint
    }

    return &AnthropicProvider{
        apiKey:   config.APIKey,
        endpoint: endpoint,
        client:   &http.Client{},
    }, nil
}

func (p *AnthropicProvider) Name() string {
    return "anthropic"
}

type anthropicRequest struct {
    Model       string              `json:"model"`
    Messages    []anthropicMessage  `json:"messages"`
    MaxTokens   int                 `json:"max_tokens"`
    Temperature *float64            `json:"temperature,omitempty"`
    Tools       []anthropicTool     `json:"tools,omitempty"`
}

type anthropicMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type anthropicTool struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
    ID      string `json:"id"`
    Type    string `json:"type"`
    Role    string `json:"role"`
    Content []struct {
        Type string `json:"type"`
        Text string `json:"text"`
    } `json:"content"`
    Model string `json:"model"`
    Usage struct {
        InputTokens  int `json:"input_tokens"`
        OutputTokens int `json:"output_tokens"`
    } `json:"usage"`
}

func (p *AnthropicProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
    // Convert to Anthropic format
    messages := make([]anthropicMessage, 0, len(req.Messages))
    for _, msg := range req.Messages {
        if msg.Role != "system" { // Anthropic doesn't use system role in messages
            messages = append(messages, anthropicMessage{
                Role:    msg.Role,
                Content: msg.Content,
            })
        }
    }

    maxTokens := 1024
    if req.MaxTokens != nil {
        maxTokens = *req.MaxTokens
    }

    anthropicReq := anthropicRequest{
        Model:       req.Model,
        Messages:    messages,
        MaxTokens:   maxTokens,
        Temperature: req.Temperature,
    }

    if len(req.Tools) > 0 {
        tools := make([]anthropicTool, len(req.Tools))
        for i, tool := range req.Tools {
            tools[i] = anthropicTool{
                Name:        tool.Function.Name,
                Description: tool.Function.Description,
                InputSchema: tool.Function.Parameters,
            }
        }
        anthropicReq.Tools = tools
    }

    // Marshal request
    body, err := json.Marshal(anthropicReq)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/messages", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-api-key", p.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")

    // Make API call
    httpResp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("API request failed: %w", err)
    }
    defer httpResp.Body.Close()

    if httpResp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(httpResp.Body)
        return nil, fmt.Errorf("API error %d: %s", httpResp.StatusCode, string(bodyBytes))
    }

    // Parse response
    var anthropicResp anthropicResponse
    if err := json.NewDecoder(httpResp.Body).Decode(&anthropicResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    // Convert to provider response
    content := ""
    if len(anthropicResp.Content) > 0 {
        content = anthropicResp.Content[0].Text
    }

    return &provider.ChatResponse{
        ID:    anthropicResp.ID,
        Model: anthropicResp.Model,
        Choices: []provider.Choice{
            {
                Message: provider.Message{
                    Role:    anthropicResp.Role,
                    Content: content,
                },
                Index: 0,
            },
        },
        Usage: provider.TokenUsage{
            InputTokens:  anthropicResp.Usage.InputTokens,
            OutputTokens: anthropicResp.Usage.OutputTokens,
            TotalTokens:  anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
        },
    }, nil
}

func (p *AnthropicProvider) Embeddings(ctx context.Context, req provider.EmbeddingsRequest) (*provider.EmbeddingsResponse, error) {
    return nil, fmt.Errorf("Anthropic does not support embeddings API")
}
```

#### 3.5 Implement Vertex AI Provider

**File:** `internal/llm/provider/vertexai/vertexai.go`

```go
package vertexai

import (
    "context"
    "fmt"

    aiplatform "cloud.google.com/go/aiplatform/apiv1"
    "cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
    "github.com/aixgo-dev/aixgo/internal/llm/provider"
    "google.golang.org/api/option"
    "google.golang.org/protobuf/types/known/structpb"
)

func init() {
    provider.Register("vertexai", New)
}

type VertexAIProvider struct {
    client    *aiplatform.PredictionClient
    projectID string
    location  string
}

func New(config provider.Config) (provider.Provider, error) {
    if config.ProjectID == "" {
        return nil, fmt.Errorf("GCP project ID is required")
    }

    location := config.Location
    if location == "" {
        location = "us-central1"
    }

    ctx := context.Background()
    var opts []option.ClientOption
    if config.Endpoint != "" {
        opts = append(opts, option.WithEndpoint(config.Endpoint))
    }

    client, err := aiplatform.NewPredictionClient(ctx, opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
    }

    return &VertexAIProvider{
        client:    client,
        projectID: config.ProjectID,
        location:  location,
    }, nil
}

func (p *VertexAIProvider) Name() string {
    return "vertexai"
}

func (p *VertexAIProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
    // Build endpoint
    endpoint := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s",
        p.projectID, p.location, req.Model)

    // Convert messages to Vertex AI format
    contents := make([]*aiplatformpb.Content, 0, len(req.Messages))
    for _, msg := range req.Messages {
        role := "user"
        if msg.Role == "assistant" {
            role = "model"
        }

        contents = append(contents, &aiplatformpb.Content{
            Role: role,
            Parts: []*aiplatformpb.Part{
                {
                    Data: &aiplatformpb.Part_Text{
                        Text: msg.Content,
                    },
                },
            },
        })
    }

    // Build parameters
    params := map[string]interface{}{}
    if req.Temperature != nil {
        params["temperature"] = *req.Temperature
    }
    if req.MaxTokens != nil {
        params["maxOutputTokens"] = *req.MaxTokens
    }

    paramsValue, err := structpb.NewStruct(params)
    if err != nil {
        return nil, fmt.Errorf("failed to create parameters: %w", err)
    }

    // Make prediction request
    predReq := &aiplatformpb.PredictRequest{
        Endpoint:  endpoint,
        Instances: []*structpb.Value{},
        Parameters: paramsValue,
    }

    resp, err := p.client.Predict(ctx, predReq)
    if err != nil {
        return nil, fmt.Errorf("Vertex AI prediction failed: %w", err)
    }

    // Parse response
    // Note: This is simplified - actual Vertex AI response parsing is more complex
    if len(resp.Predictions) == 0 {
        return nil, fmt.Errorf("no predictions returned")
    }

    // Convert to provider response
    return &provider.ChatResponse{
        ID:    "vertex-" + fmt.Sprintf("%d", resp.GetDeployedModelId()),
        Model: req.Model,
        Choices: []provider.Choice{
            {
                Message: provider.Message{
                    Role:    "assistant",
                    Content: "Response from Vertex AI", // TODO: Parse actual content
                },
                Index: 0,
            },
        },
        Usage: provider.TokenUsage{
            // TODO: Extract token usage from metadata
        },
    }, nil
}

func (p *VertexAIProvider) Embeddings(ctx context.Context, req provider.EmbeddingsRequest) (*provider.EmbeddingsResponse, error) {
    // TODO: Implement Vertex AI embeddings
    return nil, fmt.Errorf("Vertex AI embeddings not yet implemented")
}
```

#### 3.6 Implement HuggingFace Provider

**File:** `internal/llm/provider/huggingface/huggingface.go`

```go
package huggingface

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/aixgo-dev/aixgo/internal/llm/provider"
)

func init() {
    provider.Register("huggingface", New)
}

const defaultEndpoint = "https://api-inference.huggingface.co"

type HuggingFaceProvider struct {
    apiKey   string
    endpoint string
    client   *http.Client
}

func New(config provider.Config) (provider.Provider, error) {
    if config.APIKey == "" {
        return nil, fmt.Errorf("HuggingFace API key is required")
    }

    endpoint := config.Endpoint
    if endpoint == "" {
        endpoint = defaultEndpoint
    }

    return &HuggingFaceProvider{
        apiKey:   config.APIKey,
        endpoint: endpoint,
        client:   &http.Client{},
    }, nil
}

func (p *HuggingFaceProvider) Name() string {
    return "huggingface"
}

type hfRequest struct {
    Inputs     string                 `json:"inputs"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type hfResponse []struct {
    GeneratedText string `json:"generated_text"`
}

func (p *HuggingFaceProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
    // Build prompt from messages
    prompt := ""
    for _, msg := range req.Messages {
        prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
    }

    params := make(map[string]interface{})
    if req.Temperature != nil {
        params["temperature"] = *req.Temperature
    }
    if req.MaxTokens != nil {
        params["max_new_tokens"] = *req.MaxTokens
    }

    hfReq := hfRequest{
        Inputs:     prompt,
        Parameters: params,
    }

    body, err := json.Marshal(hfReq)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    url := fmt.Sprintf("%s/models/%s", p.endpoint, req.Model)
    httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

    httpResp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("API request failed: %w", err)
    }
    defer httpResp.Body.Close()

    if httpResp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(httpResp.Body)
        return nil, fmt.Errorf("API error %d: %s", httpResp.StatusCode, string(bodyBytes))
    }

    var hfResp hfResponse
    if err := json.NewDecoder(httpResp.Body).Decode(&hfResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    content := ""
    if len(hfResp) > 0 {
        content = hfResp[0].GeneratedText
    }

    return &provider.ChatResponse{
        ID:    "hf-" + req.Model,
        Model: req.Model,
        Choices: []provider.Choice{
            {
                Message: provider.Message{
                    Role:    "assistant",
                    Content: content,
                },
                Index: 0,
            },
        },
        Usage: provider.TokenUsage{
            // HuggingFace Inference API doesn't return token counts
        },
    }, nil
}

func (p *HuggingFaceProvider) Embeddings(ctx context.Context, req provider.EmbeddingsRequest) (*provider.EmbeddingsResponse, error) {
    // TODO: Implement HuggingFace embeddings
    return nil, fmt.Errorf("HuggingFace embeddings not yet implemented")
}
```

#### 3.7 Migrate xAI to Provider System

**File:** `internal/llm/provider/xai/xai.go`

```go
package xai

import (
    "context"
    "fmt"

    "github.com/aixgo-dev/aixgo/internal/llm/provider"
    "github.com/sashabaranov/go-openai"
)

func init() {
    provider.Register("xai", New)
}

const xaiEndpoint = "https://api.x.ai/v1"

type XAIProvider struct {
    client *openai.Client
}

func New(config provider.Config) (provider.Provider, error) {
    if config.APIKey == "" {
        return nil, fmt.Errorf("xAI API key is required")
    }

    clientConfig := openai.DefaultConfig(config.APIKey)
    clientConfig.BaseURL = xaiEndpoint

    return &XAIProvider{
        client: openai.NewClientWithConfig(clientConfig),
    }, nil
}

func (p *XAIProvider) Name() string {
    return "xai"
}

func (p *XAIProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
    // xAI uses OpenAI-compatible API, so implementation is similar to OpenAI provider
    messages := make([]openai.ChatCompletionMessage, len(req.Messages))
    for i, msg := range req.Messages {
        messages[i] = openai.ChatCompletionMessage{
            Role:    msg.Role,
            Content: msg.Content,
        }
    }

    oaiReq := openai.ChatCompletionRequest{
        Model:    req.Model,
        Messages: messages,
    }

    if req.Temperature != nil {
        oaiReq.Temperature = float32(*req.Temperature)
    }

    if req.MaxTokens != nil {
        oaiReq.MaxTokens = *req.MaxTokens
    }

    resp, err := p.client.CreateChatCompletion(ctx, oaiReq)
    if err != nil {
        return nil, fmt.Errorf("xAI API error: %w", err)
    }

    choices := make([]provider.Choice, len(resp.Choices))
    for i, choice := range resp.Choices {
        choices[i] = provider.Choice{
            Message: provider.Message{
                Role:    choice.Message.Role,
                Content: choice.Message.Content,
            },
            FinishReason: string(choice.FinishReason),
            Index:        choice.Index,
        }
    }

    return &provider.ChatResponse{
        ID:      resp.ID,
        Model:   resp.Model,
        Choices: choices,
        Usage: provider.TokenUsage{
            InputTokens:  resp.Usage.PromptTokens,
            OutputTokens: resp.Usage.CompletionTokens,
            TotalTokens:  resp.Usage.TotalTokens,
        },
    }, nil
}

func (p *XAIProvider) Embeddings(ctx context.Context, req provider.EmbeddingsRequest) (*provider.EmbeddingsResponse, error) {
    return nil, fmt.Errorf("xAI does not support embeddings API")
}
```

#### 3.8 Update ReAct Agent to Use Provider Registry

**File:** `internal/agents/react.go` (modify existing)

```go
package agents

import (
    "context"
    "fmt"
    "os"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/llm/provider"
    _ "github.com/aixgo-dev/aixgo/internal/llm/provider/anthropic"
    _ "github.com/aixgo-dev/aixgo/internal/llm/provider/huggingface"
    _ "github.com/aixgo-dev/aixgo/internal/llm/provider/openai"
    _ "github.com/aixgo-dev/aixgo/internal/llm/provider/vertexai"
    _ "github.com/aixgo-dev/aixgo/internal/llm/provider/xai"
)

func NewReActAgent(def agent.AgentDef, rt agent.Runtime) (agent.Agent, error) {
    // Get provider from config or default to xai
    providerName := def.Provider
    if providerName == "" {
        providerName = "xai" // Default
    }

    // Get API key from config or environment
    apiKey := def.APIKey
    if apiKey == "" {
        apiKey = os.Getenv(fmt.Sprintf("%s_API_KEY", strings.ToUpper(providerName)))
    }

    // Create provider
    prov, err := provider.Get(providerName, provider.Config{
        APIKey:    apiKey,
        Endpoint:  def.Endpoint,
        ProjectID: os.Getenv("GCP_PROJECT_ID"), // For Vertex AI
        Location:  "us-central1",                // For Vertex AI
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create provider: %w", err)
    }

    return NewReActAgentWithProvider(def, rt, prov)
}

func NewReActAgentWithProvider(def agent.AgentDef, rt agent.Runtime, prov provider.Provider) (agent.Agent, error) {
    return &ReActAgent{
        name:     def.Name,
        role:     def.Role,
        model:    def.Model,
        prompt:   def.Prompt,
        provider: prov,
        runtime:  rt,
        inputs:   def.Inputs,
        outputs:  def.Outputs,
        tools:    def.Tools,
    }, nil
}
```

---

## Feature 4: Enhanced Supervisor Orchestration (Priority: HIGH)

### Objective
Implement actual supervisor orchestration including dependency-aware startup, message routing, and execution control.

### Current Problem
Supervisor exists but does nothing. Runtime handles routing instead.

### Implementation Details

#### 4.1 Implement Dependency Graph

**File:** `internal/supervisor/graph.go`

```go
package supervisor

import (
    "fmt"

    "github.com/aixgo-dev/aixgo/internal/agent"
)

// DependencyGraph represents agent dependencies
type DependencyGraph struct {
    agents map[string]*GraphNode
}

// GraphNode represents an agent in the dependency graph
type GraphNode struct {
    Name         string
    Dependencies []string  // Agents this node depends on
    Dependents   []string  // Agents that depend on this node
}

// BuildDependencyGraph constructs a dependency graph from agent definitions
func BuildDependencyGraph(agents []agent.AgentDef) (*DependencyGraph, error) {
    graph := &DependencyGraph{
        agents: make(map[string]*GraphNode),
    }

    // Create nodes
    for _, agentDef := range agents {
        graph.agents[agentDef.Name] = &GraphNode{
            Name:         agentDef.Name,
            Dependencies: make([]string, 0),
            Dependents:   make([]string, 0),
        }
    }

    // Build edges from inputs/outputs
    for _, agentDef := range agents {
        node := graph.agents[agentDef.Name]

        // Dependencies are agents we receive from
        for _, input := range agentDef.Inputs {
            node.Dependencies = append(node.Dependencies, input.Source)

            // Add reverse edge
            if sourceNode, ok := graph.agents[input.Source]; ok {
                sourceNode.Dependents = append(sourceNode.Dependents, agentDef.Name)
            }
        }
    }

    // Validate no cycles
    if err := graph.detectCycles(); err != nil {
        return nil, err
    }

    return graph, nil
}

// TopologicalSort returns agents in dependency order
func (g *DependencyGraph) TopologicalSort() ([][]string, error) {
    visited := make(map[string]bool)
    tiers := [][]string{}

    for len(visited) < len(g.agents) {
        tier := []string{}

        // Find agents with no unvisited dependencies
        for name, node := range g.agents {
            if visited[name] {
                continue
            }

            canStart := true
            for _, dep := range node.Dependencies {
                if !visited[dep] {
                    canStart = false
                    break
                }
            }

            if canStart {
                tier = append(tier, name)
            }
        }

        if len(tier) == 0 {
            return nil, fmt.Errorf("circular dependency detected")
        }

        for _, name := range tier {
            visited[name] = true
        }

        tiers = append(tiers, tier)
    }

    return tiers, nil
}

// detectCycles checks for circular dependencies
func (g *DependencyGraph) detectCycles() error {
    visited := make(map[string]bool)
    recStack := make(map[string]bool)

    var visit func(string) error
    visit = func(name string) error {
        visited[name] = true
        recStack[name] = true

        node := g.agents[name]
        for _, dep := range node.Dependencies {
            if !visited[dep] {
                if err := visit(dep); err != nil {
                    return err
                }
            } else if recStack[dep] {
                return fmt.Errorf("circular dependency: %s -> %s", name, dep)
            }
        }

        recStack[name] = false
        return nil
    }

    for name := range g.agents {
        if !visited[name] {
            if err := visit(name); err != nil {
                return err
            }
        }
    }

    return nil
}
```

#### 4.2 Implement Message Routing

**File:** `internal/supervisor/router.go`

```go
package supervisor

import (
    "context"
    "fmt"
    "log"

    "github.com/aixgo-dev/aixgo/internal/agent"
)

// Router handles message routing between agents
type Router struct {
    agents  map[string]agent.Agent
    runtime agent.Runtime
    routes  map[string][]string  // agent name -> output targets
}

// NewRouter creates a new message router
func NewRouter(agents map[string]agent.Agent, defs []agent.AgentDef, runtime agent.Runtime) *Router {
    routes := make(map[string][]string)

    // Build routing table from agent definitions
    for _, def := range defs {
        targets := make([]string, len(def.Outputs))
        for i, output := range def.Outputs {
            targets[i] = output.Target
        }
        routes[def.Name] = targets
    }

    return &Router{
        agents:  agents,
        runtime: runtime,
        routes:  routes,
    }
}

// Start begins routing messages
func (r *Router) Start(ctx context.Context) error {
    // Start receiving messages from each agent
    for name := range r.agents {
        go r.routeFromAgent(ctx, name)
    }

    return nil
}

// routeFromAgent routes messages from a specific agent
func (r *Router) routeFromAgent(ctx context.Context, agentName string) {
    msgChan, err := r.runtime.Receive(agentName)
    if err != nil {
        log.Printf("[ROUTER] Error receiving from %s: %v", agentName, err)
        return
    }

    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-msgChan:
            // Route message to all configured outputs
            targets := r.routes[agentName]
            for _, target := range targets {
                if err := r.runtime.Send(target, msg); err != nil {
                    log.Printf("[ROUTER] Error routing to %s: %v", target, err)
                }
            }
        }
    }
}
```

#### 4.3 Update Supervisor with Orchestration Logic

**File:** `internal/supervisor/supervisor.go` (replace existing)

```go
package supervisor

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/config"
)

type Supervisor struct {
    def     config.SupervisorDef
    agents  map[string]agent.Agent
    runtime agent.Runtime
    router  *Router

    roundCount int
    mu         sync.Mutex
}

func NewSupervisor(def config.SupervisorDef, agents []agent.Agent, runtime agent.Runtime) *Supervisor {
    agentMap := make(map[string]agent.Agent)
    for _, a := range agents {
        agentMap[a.Name()] = a
    }

    return &Supervisor{
        def:     def,
        agents:  agentMap,
        runtime: runtime,
    }
}

func (s *Supervisor) Start(ctx context.Context) error {
    log.Printf("[SUPERVISOR] %s online (model: %s, max_rounds: %d)",
        s.def.Name, s.def.Model, s.def.MaxRounds)

    // Build dependency graph
    agentDefs := make([]agent.AgentDef, 0, len(s.agents))
    for _, a := range s.agents {
        // Get AgentDef from agent - need to add this method
        // For now, skip dependency ordering
    }

    graph, err := BuildDependencyGraph(agentDefs)
    if err != nil {
        return fmt.Errorf("invalid dependency graph: %w", err)
    }

    // Get startup order
    tiers, err := graph.TopologicalSort()
    if err != nil {
        return fmt.Errorf("failed to determine startup order: %w", err)
    }

    log.Printf("[SUPERVISOR] Startup order: %v", tiers)

    // Start agents in dependency order
    for _, tier := range tiers {
        var wg sync.WaitGroup
        for _, agentName := range tier {
            wg.Add(1)
            go func(name string) {
                defer wg.Done()
                if err := s.agents[name].Start(ctx); err != nil {
                    log.Printf("[SUPERVISOR] Error starting %s: %v", name, err)
                }
            }(agentName)
        }
        wg.Wait()  // Wait for tier to start before next tier

        // Small delay between tiers
        time.Sleep(100 * time.Millisecond)
    }

    // Start message router
    s.router = NewRouter(s.agents, agentDefs, s.runtime)
    if err := s.router.Start(ctx); err != nil {
        return fmt.Errorf("failed to start router: %w", err)
    }

    // Monitor execution
    return s.monitorExecution(ctx)
}

func (s *Supervisor) monitorExecution(ctx context.Context) error {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return s.shutdown()
        case <-ticker.Tick:
            s.mu.Lock()
            rounds := s.roundCount
            s.mu.Unlock()

            // Check max rounds
            if s.def.MaxRounds > 0 && rounds >= s.def.MaxRounds {
                log.Printf("[SUPERVISOR] Max rounds (%d) reached, shutting down", s.def.MaxRounds)
                return s.shutdown()
            }
        }
    }
}

func (s *Supervisor) IncrementRound() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.roundCount++
}

func (s *Supervisor) shutdown() error {
    log.Printf("[SUPERVISOR] Initiating graceful shutdown")

    // Stop all agents
    for name, agent := range s.agents {
        log.Printf("[SUPERVISOR] Stopping %s", name)
        if err := agent.Stop(); err != nil {
            log.Printf("[SUPERVISOR] Error stopping %s: %v", name, err)
        }
    }

    return nil
}
```

---

## Feature 5: Distributed Mode with gRPC (Priority: MEDIUM)

### Objective
Implement basic distributed mode using gRPC for inter-agent communication.

### Implementation Details

#### 5.1 Create Protocol Buffers Definition

**File:** `proto/agent.proto`

```protobuf
syntax = "proto3";

package aixgo.v1;

option go_package = "github.com/aixgo-dev/aixgo/internal/proto/agentpb";

// AgentService handles agent-to-agent communication
service AgentService {
    // SendMessage sends a message to an agent
    rpc SendMessage(Message) returns (Ack);

    // ReceiveMessages streams messages for an agent
    rpc ReceiveMessages(ReceiveRequest) returns (stream Message);
}

// Message represents an inter-agent message
message Message {
    string id = 1;
    string from = 2;
    string to = 3;
    bytes payload = 4;
    map<string, string> metadata = 5;
    int64 timestamp = 6;
}

// Ack acknowledges message receipt
message Ack {
    string message_id = 1;
    bool success = 2;
    string error = 3;
}

// ReceiveRequest requests messages for an agent
message ReceiveRequest {
    string agent_id = 1;
}
```

**Generate code:**
```bash
# Add to Makefile
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent.proto
```

#### 5.2 Implement gRPC Runtime

**File:** `internal/runtime/grpc_runtime.go`

```go
package runtime

import (
    "context"
    "fmt"
    "sync"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/proto/agentpb"
    "google.golang.org/grpc"
)

// GRPCRuntime implements distributed runtime using gRPC
type GRPCRuntime struct {
    agentID    string
    endpoints  map[string]string  // agent name -> gRPC endpoint
    clients    map[string]agentpb.AgentServiceClient
    server     *grpc.Server
    msgBuffers map[string]chan *agent.Message
    mu         sync.RWMutex
}

// NewGRPCRuntime creates a new gRPC runtime
func NewGRPCRuntime(agentID string, endpoints map[string]string) (*GRPCRuntime, error) {
    return &GRPCRuntime{
        agentID:    agentID,
        endpoints:  endpoints,
        clients:    make(map[string]agentpb.AgentServiceClient),
        msgBuffers: make(map[string]chan *agent.Message),
    }, nil
}

// Send sends a message to a target agent via gRPC
func (r *GRPCRuntime) Send(target string, msg *agent.Message) error {
    client, err := r.getClient(target)
    if err != nil {
        return fmt.Errorf("failed to get client for %s: %w", target, err)
    }

    // Convert to protobuf message
    pbMsg := &agentpb.Message{
        Id:       msg.ID,
        From:     msg.From,
        To:       target,
        Payload:  msg.Payload,
        Metadata: msg.Metadata,
    }

    // Send via gRPC
    ctx := context.Background()
    _, err = client.SendMessage(ctx, pbMsg)
    if err != nil {
        return fmt.Errorf("gRPC send failed: %w", err)
    }

    return nil
}

// Receive returns a channel for receiving messages
func (r *GRPCRuntime) Receive(agent string) (<-chan *agent.Message, error) {
    r.mu.Lock()
    defer r.mu.Unlock()

    if ch, ok := r.msgBuffers[agent]; ok {
        return ch, nil
    }

    ch := make(chan *agent.Message, 100)
    r.msgBuffers[agent] = ch

    // Start gRPC stream to receive messages
    go r.receiveStream(agent, ch)

    return ch, nil
}

// getClient gets or creates a gRPC client for an agent
func (r *GRPCRuntime) getClient(agentName string) (agentpb.AgentServiceClient, error) {
    r.mu.RLock()
    if client, ok := r.clients[agentName]; ok {
        r.mu.RUnlock()
        return client, nil
    }
    r.mu.RUnlock()

    endpoint, ok := r.endpoints[agentName]
    if !ok {
        return nil, fmt.Errorf("no endpoint configured for %s", agentName)
    }

    conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
    if err != nil {
        return nil, fmt.Errorf("failed to connect to %s: %w", endpoint, err)
    }

    client := agentpb.NewAgentServiceClient(conn)

    r.mu.Lock()
    r.clients[agentName] = client
    r.mu.Unlock()

    return client, nil
}

// receiveStream receives messages via gRPC stream
func (r *GRPCRuntime) receiveStream(agentName string, ch chan *agent.Message) {
    client, err := r.getClient(agentName)
    if err != nil {
        log.Printf("[GRPC] Failed to get client: %v", err)
        return
    }

    stream, err := client.ReceiveMessages(context.Background(), &agentpb.ReceiveRequest{
        AgentId: agentName,
    })
    if err != nil {
        log.Printf("[GRPC] Failed to create stream: %v", err)
        return
    }

    for {
        pbMsg, err := stream.Recv()
        if err != nil {
            log.Printf("[GRPC] Stream error: %v", err)
            return
        }

        // Convert from protobuf
        msg := &agent.Message{
            ID:       pbMsg.Id,
            From:     pbMsg.From,
            To:       pbMsg.To,
            Payload:  pbMsg.Payload,
            Metadata: pbMsg.Metadata,
        }

        ch <- msg
    }
}

// Close closes the gRPC runtime
func (r *GRPCRuntime) Close() error {
    r.mu.Lock()
    defer r.mu.Unlock()

    for _, ch := range r.msgBuffers {
        close(ch)
    }

    if r.server != nil {
        r.server.GracefulStop()
    }

    return nil
}
```

#### 5.3 Implement Runtime Factory

**File:** `internal/runtime/factory.go`

```go
package runtime

import (
    "fmt"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/config"
)

// NewRuntime creates the appropriate runtime based on configuration
func NewRuntime(cfg config.Config) (agent.Runtime, error) {
    mode := cfg.Supervisor.Mode
    if mode == "" {
        mode = "local"
    }

    switch mode {
    case "local":
        return NewSimpleRuntime(), nil

    case "distributed":
        // Build endpoint map from agent configs
        endpoints := make(map[string]string)
        for _, agentDef := range cfg.Agents {
            if agentDef.Endpoint != "" {
                endpoints[agentDef.Name] = agentDef.Endpoint
            }
        }

        return NewGRPCRuntime("supervisor", endpoints)

    default:
        return nil, fmt.Errorf("unknown runtime mode: %s", mode)
    }
}
```

---

## Feature 6: Enhanced Observability (Priority: MEDIUM)

### Objective
Add comprehensive observability with trace attributes, Prometheus metrics, and Langfuse integration.

### Implementation Details

#### 6.1 Enhanced Tracing with Attributes

**File:** `internal/observability/tracing.go` (enhance existing)

```go
package observability

import (
    "context"

    "github.com/aixgo-dev/aixgo/internal/agent"
    "github.com/aixgo-dev/aixgo/internal/llm/provider"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/aixgo-dev/aixgo")

// InstrumentAgent wraps agent processing with tracing
func InstrumentAgent(ctx context.Context, agent agent.Agent, msg *agent.Message) (context.Context, trace.Span) {
    ctx, span := tracer.Start(ctx, "agent.process",
        trace.WithAttributes(
            attribute.String("agent.name", agent.Name()),
            attribute.String("agent.role", agent.Role()),
            attribute.String("message.id", msg.ID),
            attribute.String("message.from", msg.From),
        ))

    return ctx, span
}

// InstrumentLLMCall wraps LLM API calls with tracing
func InstrumentLLMCall(ctx context.Context, providerName, model string) (context.Context, trace.Span) {
    ctx, span := tracer.Start(ctx, "llm.call",
        trace.WithAttributes(
            attribute.String("llm.provider", providerName),
            attribute.String("llm.model", model),
        ))

    return ctx, span
}

// RecordLLMMetrics records LLM call metrics in span
func RecordLLMMetrics(span trace.Span, usage provider.TokenUsage, latencyMs int64) {
    span.SetAttributes(
        attribute.Int("llm.tokens.input", usage.InputTokens),
        attribute.Int("llm.tokens.output", usage.OutputTokens),
        attribute.Int("llm.tokens.total", usage.TotalTokens),
        attribute.Int64("llm.latency_ms", latencyMs),
    )
}

// InstrumentToolCall wraps tool execution with tracing
func InstrumentToolCall(ctx context.Context, toolName string) (context.Context, trace.Span) {
    ctx, span := tracer.Start(ctx, "tool.execute",
        trace.WithAttributes(
            attribute.String("tool.name", toolName),
        ))

    return ctx, span
}

// RecordToolMetrics records tool execution metrics
func RecordToolMetrics(span trace.Span, durationMs int64) {
    span.SetAttributes(
        attribute.Int64("tool.duration_ms", durationMs),
    )
}
```

#### 6.2 Prometheus Metrics

**File:** `internal/observability/metrics.go`

```go
package observability

import (
    "net/http"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    // Agent metrics
    agentMessagesTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aixgo_agent_messages_total",
            Help: "Total messages processed by agent",
        },
        []string{"agent", "role"},
    )

    agentErrorsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aixgo_agent_errors_total",
            Help: "Total errors by agent",
        },
        []string{"agent", "role", "error_type"},
    )

    agentDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "aixgo_agent_duration_seconds",
            Help:    "Agent processing duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"agent", "role"},
    )

    // LLM metrics
    llmRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aixgo_llm_requests_total",
            Help: "Total LLM API requests",
        },
        []string{"provider", "model"},
    )

    llmTokensInput = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aixgo_llm_tokens_input",
            Help: "Input tokens consumed",
        },
        []string{"provider", "model"},
    )

    llmTokensOutput = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aixgo_llm_tokens_output",
            Help: "Output tokens generated",
        },
        []string{"provider", "model"},
    )

    llmLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "aixgo_llm_latency_seconds",
            Help:    "LLM API latency",
            Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"provider", "model"},
    )

    // Supervisor metrics
    supervisorRoundsTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "aixgo_supervisor_rounds_total",
            Help: "Total workflow rounds",
        },
    )

    supervisorActiveAgents = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "aixgo_supervisor_active_agents",
            Help: "Number of active agents",
        },
    )

    supervisorMessageQueueSize = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "aixgo_supervisor_message_queue_size",
            Help: "Pending messages in queue",
        },
        []string{"agent"},
    )
)

// RecordAgentMessage records an agent message
func RecordAgentMessage(agent, role string) {
    agentMessagesTotal.WithLabelValues(agent, role).Inc()
}

// RecordAgentError records an agent error
func RecordAgentError(agent, role, errorType string) {
    agentErrorsTotal.WithLabelValues(agent, role, errorType).Inc()
}

// RecordAgentDuration records agent processing duration
func RecordAgentDuration(agent, role string, duration time.Duration) {
    agentDuration.WithLabelValues(agent, role).Observe(duration.Seconds())
}

// RecordLLMRequest records an LLM request
func RecordLLMRequest(provider, model string, inputTokens, outputTokens int, latency time.Duration) {
    llmRequestsTotal.WithLabelValues(provider, model).Inc()
    llmTokensInput.WithLabelValues(provider, model).Add(float64(inputTokens))
    llmTokensOutput.WithLabelValues(provider, model).Add(float64(outputTokens))
    llmLatency.WithLabelValues(provider, model).Observe(latency.Seconds())
}

// RecordSupervisorRound increments supervisor round counter
func RecordSupervisorRound() {
    supervisorRoundsTotal.Inc()
}

// SetActiveAgents sets the number of active agents
func SetActiveAgents(count int) {
    supervisorActiveAgents.Set(float64(count))
}

// SetMessageQueueSize sets the message queue size for an agent
func SetMessageQueueSize(agent string, size int) {
    supervisorMessageQueueSize.WithLabelValues(agent).Set(float64(size))
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(port int) error {
    http.Handle("/metrics", promhttp.Handler())
    return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
```

---

## Feature 7: Vector Database Integrations (Priority: LOW)

### Objective
Implement vector database support for Firestore and Qdrant.

### Implementation Details

See the detailed specification document for vector database implementation. Due to length constraints, I've included the most critical features above.

Key interfaces needed:
- `vectordb/vectordb.go` - Base interface
- `vectordb/firestore/firestore.go` - Firestore implementation
- `vectordb/qdrant/qdrant.go` - Qdrant implementation

---

## Testing Requirements

### Unit Tests
Create unit tests for each new package:
- `types_test.go`, `options_test.go`, `agent_test.go`
- Provider tests for each implementation
- Supervisor orchestration tests
- gRPC runtime tests

### Integration Tests
**File:** `tests/integration/multi_provider_test.go`

Test scenarios:
1. Agent creation via Go API
2. Multi-provider configuration
3. Supervisor orchestration with dependency graph
4. Distributed mode message passing
5. Observability instrumentation

### Example Integration Test:

```go
package integration

import (
    "context"
    "testing"
    "time"

    "github.com/aixgo-dev/aixgo"
)

func TestMultiProviderAgents(t *testing.T) {
    ctx := context.Background()

    // Create agents with different providers
    openaiAgent, err := aixgo.NewAgent(
        aixgo.WithName("openai-analyzer"),
        aixgo.WithRole(aixgo.RoleReAct),
        aixgo.WithModel("gpt-4-turbo"),
        aixgo.WithProvider("openai"),
        aixgo.WithAPIKey("test-key"),
    )
    if err != nil {
        t.Fatal(err)
    }

    anthropicAgent, err := aixgo.NewAgent(
        aixgo.WithName("claude-analyzer"),
        aixgo.WithRole(aixgo.RoleReAct),
        aixgo.WithModel("claude-3-sonnet"),
        aixgo.WithProvider("anthropic"),
        aixgo.WithAPIKey("test-key"),
    )
    if err != nil {
        t.Fatal(err)
    }

    // Test execution
    if err := openaiAgent.Start(ctx); err != nil {
        t.Errorf("OpenAI agent failed: %v", err)
    }

    if err := anthropicAgent.Start(ctx); err != nil {
        t.Errorf("Anthropic agent failed: %v", err)
    }
}
```

---

## Implementation Checklist

### Phase 1: Foundation
- [ ] Create public API types (`types.go`)
- [ ] Implement option functions (`options.go`)
- [ ] Create agent constructor (`agent.go`)
- [ ] Update `AgentDef` schema (`internal/agent/types.go`)
- [ ] Update `SupervisorDef` and add `ObservabilityConfig` (`internal/config/types.go`)
- [ ] Create provider interface (`internal/llm/provider/provider.go`)
- [ ] Create provider registry (`internal/llm/provider/registry.go`)

### Phase 2: Providers
- [ ] Implement OpenAI provider (`internal/llm/provider/openai/openai.go`)
- [ ] Implement Anthropic provider (`internal/llm/provider/anthropic/anthropic.go`)
- [ ] Implement Vertex AI provider (`internal/llm/provider/vertexai/vertexai.go`)
- [ ] Implement HuggingFace provider (`internal/llm/provider/huggingface/huggingface.go`)
- [ ] Migrate xAI to provider system (`internal/llm/provider/xai/xai.go`)
- [ ] Update ReAct agent to use provider registry (`internal/agents/react.go`)

### Phase 3: Orchestration
- [ ] Implement dependency graph (`internal/supervisor/graph.go`)
- [ ] Implement message router (`internal/supervisor/router.go`)
- [ ] Update supervisor with orchestration (`internal/supervisor/supervisor.go`)
- [ ] Add retry logic decorator
- [ ] Add timeout handling

### Phase 4: Distributed Mode
- [ ] Create protobuf definitions (`proto/agent.proto`)
- [ ] Generate gRPC code (`make proto`)
- [ ] Implement gRPC runtime (`internal/runtime/grpc_runtime.go`)
- [ ] Create runtime factory (`internal/runtime/factory.go`)
- [ ] Update supervisor to support distributed mode

### Phase 5: Observability
- [ ] Enhance tracing with attributes (`internal/observability/tracing.go`)
- [ ] Add Prometheus metrics (`internal/observability/metrics.go`)
- [ ] Implement Langfuse client (`internal/observability/langfuse/langfuse.go`)
- [ ] Update agents to use instrumentation

### Phase 6: Testing
- [ ] Write unit tests for all packages
- [ ] Create integration tests
- [ ] Add example programs
- [ ] Update README with examples

---

## Dependencies to Add

Update `go.mod`:

```go
require (
    cloud.google.com/go/aiplatform v1.50.0
    github.com/prometheus/client_golang v1.17.0
    go.opentelemetry.io/otel v1.21.0
    go.opentelemetry.io/otel/trace v1.21.0
    google.golang.org/grpc v1.59.0
    google.golang.org/protobuf v1.31.0
    // Existing dependencies...
)
```

---

## Documentation Updates After Implementation

Once implemented, update:
1. README.md with Go API examples
2. examples/ directory with working examples for each feature
3. Add godoc comments to all public APIs
4. Create CHANGELOG.md documenting v0.1 features

---

## Success Criteria

Implementation is complete when:
1. ✅ All code compiles without errors
2. ✅ All unit tests pass
3. ✅ Integration tests demonstrate each feature working
4. ✅ Documentation examples execute successfully
5. ✅ Website documentation accurately reflects implemented features
6. ✅ No claims of unimplemented features in docs

---

## Timeline Estimate

- **Day 1:** Foundation + Provider interface (Phase 1)
- **Day 2:** Multi-provider implementations (Phase 2)
- **Day 3:** Supervisor orchestration + Retry logic (Phase 3)
- **Day 4:** Distributed mode + Observability (Phases 4-5)
- **Day 5:** Testing + Documentation + Polish (Phase 6)

**Total:** 5 days of focused implementation

---

## Notes for Implementation

1. **Start with tests:** Write failing tests first, then implement features
2. **Incremental commits:** Commit after each feature is working
3. **Run tests frequently:** `go test ./...` after each change
4. **Check documentation:** Verify examples work as you implement
5. **Update examples:** Add working examples to `examples/` directory
6. **Godoc comments:** Add documentation to all public APIs

Good luck with the implementation! This spec should provide everything needed to bring Aixgo to feature parity with the documentation.
