package e2e

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	"github.com/aixgo-dev/aixgo/pkg/security"
	pb "github.com/aixgo-dev/aixgo/proto"
)

// TestEnvironment provides a complete test environment for E2E tests
type TestEnvironment struct {
	t           *testing.T
	runtime     *MockE2ERuntime
	provider    *MockE2EProvider
	mcpServer   *MockMCPServer
	auditLogger *security.InMemoryAuditLogger
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewTestEnvironment creates a new test environment
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	env := &TestEnvironment{
		t:           t,
		runtime:     NewMockE2ERuntime(),
		provider:    NewMockE2EProvider(),
		mcpServer:   NewMockMCPServer(),
		auditLogger: security.NewInMemoryAuditLogger(),
		ctx:         ctx,
		cancel:      cancel,
	}

	return env
}

// Cleanup cleans up the test environment
func (e *TestEnvironment) Cleanup() {
	e.cancel()
	e.runtime.Close()
}

// Context returns the test context
func (e *TestEnvironment) Context() context.Context {
	return e.ctx
}

// Runtime returns the mock runtime
func (e *TestEnvironment) Runtime() *MockE2ERuntime {
	return e.runtime
}

// Provider returns the mock provider
func (e *TestEnvironment) Provider() *MockE2EProvider {
	return e.provider
}

// MCPServer returns the mock MCP server
func (e *TestEnvironment) MCPServer() *MockMCPServer {
	return e.mcpServer
}

// AuditLogger returns the audit logger
func (e *TestEnvironment) AuditLogger() *security.InMemoryAuditLogger {
	return e.auditLogger
}

// MockE2ERuntime is a comprehensive mock runtime for E2E testing
type MockE2ERuntime struct {
	channels  map[string]chan *agent.Message
	sendCalls []SendCall
	recvCalls []string
	mu        sync.RWMutex
}

// SendCall records a Send call
type SendCall struct {
	Target  string
	Message *agent.Message
}

// NewMockE2ERuntime creates a new mock runtime
func NewMockE2ERuntime() *MockE2ERuntime {
	return &MockE2ERuntime{
		channels:  make(map[string]chan *agent.Message),
		sendCalls: make([]SendCall, 0),
		recvCalls: make([]string, 0),
	}
}

// Send implements agent.Runtime
func (m *MockE2ERuntime) Send(target string, msg *agent.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sendCalls = append(m.sendCalls, SendCall{Target: target, Message: msg})

	if ch, ok := m.channels[target]; ok {
		select {
		case ch <- msg:
		case <-time.After(100 * time.Millisecond):
		}
	}

	return nil
}

// Recv implements agent.Runtime
func (m *MockE2ERuntime) Recv(source string) (<-chan *agent.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recvCalls = append(m.recvCalls, source)

	if _, ok := m.channels[source]; !ok {
		m.channels[source] = make(chan *agent.Message, 100)
	}

	return m.channels[source], nil
}

// SendMessage sends a message to a channel
func (m *MockE2ERuntime) SendMessage(source string, msg *agent.Message) {
	m.mu.Lock()
	if _, ok := m.channels[source]; !ok {
		m.channels[source] = make(chan *agent.Message, 100)
	}
	ch := m.channels[source]
	m.mu.Unlock()

	select {
	case ch <- msg:
	case <-time.After(100 * time.Millisecond):
	}
}

// GetSendCalls returns all send calls
func (m *MockE2ERuntime) GetSendCalls() []SendCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calls := make([]SendCall, len(m.sendCalls))
	copy(calls, m.sendCalls)
	return calls
}

// Close closes all channels
func (m *MockE2ERuntime) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.channels {
		close(ch)
	}
}

// MockE2EProvider is a mock LLM provider for E2E testing
type MockE2EProvider struct {
	responses    []*provider.CompletionResponse
	toolHandlers map[string]func(map[string]any) (string, error)
	calls        []provider.CompletionRequest
	callIndex    int
	mu           sync.Mutex
}

// NewMockE2EProvider creates a new mock provider
func NewMockE2EProvider() *MockE2EProvider {
	return &MockE2EProvider{
		responses:    make([]*provider.CompletionResponse, 0),
		toolHandlers: make(map[string]func(map[string]any) (string, error)),
		calls:        make([]provider.CompletionRequest, 0),
	}
}

// Name implements provider.Provider
func (m *MockE2EProvider) Name() string {
	return "mock-e2e-provider"
}

// CreateCompletion implements provider.Provider
func (m *MockE2EProvider) CreateCompletion(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, req)

	if m.callIndex < len(m.responses) {
		resp := m.responses[m.callIndex]
		m.callIndex++
		return resp, nil
	}

	return &provider.CompletionResponse{
		Content:      "Default mock response",
		FinishReason: "stop",
	}, nil
}

// CreateStructured implements provider.Provider
func (m *MockE2EProvider) CreateStructured(ctx context.Context, req provider.StructuredRequest) (*provider.StructuredResponse, error) {
	data, _ := json.Marshal(map[string]any{"result": "mock"})
	return &provider.StructuredResponse{
		Data: data,
		CompletionResponse: provider.CompletionResponse{
			Content:      string(data),
			FinishReason: "stop",
		},
	}, nil
}

// CreateStreaming implements provider.Provider
func (m *MockE2EProvider) CreateStreaming(ctx context.Context, req provider.CompletionRequest) (provider.Stream, error) {
	return &MockStream{
		chunks: []*provider.StreamChunk{
			{Delta: "Mock stream response", FinishReason: "stop"},
		},
	}, nil
}

// AddResponse adds a response
func (m *MockE2EProvider) AddResponse(resp *provider.CompletionResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, resp)
}

// AddToolCallResponse adds a response with tool calls
func (m *MockE2EProvider) AddToolCallResponse(toolName string, args map[string]any) {
	argsJSON, _ := json.Marshal(args)
	resp := &provider.CompletionResponse{
		Content:      "",
		FinishReason: "tool_calls",
		ToolCalls: []provider.ToolCall{
			{
				ID:   "call_" + toolName,
				Type: "function",
				Function: provider.FunctionCall{
					Name:      toolName,
					Arguments: argsJSON,
				},
			},
		},
	}
	m.AddResponse(resp)
}

// AddTextResponse adds a text response
func (m *MockE2EProvider) AddTextResponse(content string) {
	m.AddResponse(&provider.CompletionResponse{
		Content:      content,
		FinishReason: "stop",
	})
}

// GetCalls returns all calls made
func (m *MockE2EProvider) GetCalls() []provider.CompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]provider.CompletionRequest, len(m.calls))
	copy(calls, m.calls)
	return calls
}

// Reset resets the mock
func (m *MockE2EProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.responses = make([]*provider.CompletionResponse, 0)
	m.calls = make([]provider.CompletionRequest, 0)
	m.callIndex = 0
}

// MockStream implements provider.Stream
type MockStream struct {
	chunks []*provider.StreamChunk
	index  int
	closed bool
}

// Recv implements provider.Stream
func (s *MockStream) Recv() (*provider.StreamChunk, error) {
	if s.closed || s.index >= len(s.chunks) {
		return nil, nil
	}
	chunk := s.chunks[s.index]
	s.index++
	return chunk, nil
}

// Close implements provider.Stream
func (s *MockStream) Close() error {
	s.closed = true
	return nil
}

// MockMCPServer is a mock MCP server for E2E testing
type MockMCPServer struct {
	tools    []mcp.Tool
	handlers map[string]mcp.ToolHandler
	calls    []ToolCall
	mu       sync.Mutex
}

// ToolCall records a tool call
type ToolCall struct {
	Name string
	Args map[string]any
}

// NewMockMCPServer creates a new mock MCP server
func NewMockMCPServer() *MockMCPServer {
	return &MockMCPServer{
		tools:    make([]mcp.Tool, 0),
		handlers: make(map[string]mcp.ToolHandler),
		calls:    make([]ToolCall, 0),
	}
}

// RegisterTool registers a tool
func (s *MockMCPServer) RegisterTool(name, description string, handler mcp.ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tools = append(s.tools, mcp.Tool{
		Name:        name,
		Description: description,
		Handler:     handler,
	})
	s.handlers[name] = handler
}

// CallTool calls a tool
func (s *MockMCPServer) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	s.mu.Lock()
	s.calls = append(s.calls, ToolCall{Name: name, Args: args})
	handler, ok := s.handlers[name]
	s.mu.Unlock()

	if !ok {
		return nil, nil
	}

	return handler(ctx, mcp.Args(args))
}

// GetTools returns all registered tools
func (s *MockMCPServer) GetTools() []mcp.Tool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tools := make([]mcp.Tool, len(s.tools))
	copy(tools, s.tools)
	return tools
}

// GetCalls returns all tool calls
func (s *MockMCPServer) GetCalls() []ToolCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	calls := make([]ToolCall, len(s.calls))
	copy(calls, s.calls)
	return calls
}

// CreateTestMessage creates a test message
func CreateTestMessage(id, msgType, payload string) *agent.Message {
	return &agent.Message{
		Message: &pb.Message{
			Id:        id,
			Type:      msgType,
			Payload:   payload,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
}

// CreateTestAgentDef creates a test agent definition
func CreateTestAgentDef(name, role, model string) agent.AgentDef {
	return agent.AgentDef{
		Name:     name,
		Role:     role,
		Model:    model,
		Prompt:   "You are a helpful assistant.",
		Inputs:   []agent.Input{{Source: "input-" + name}},
		Outputs:  []agent.Output{{Target: "output-" + name}},
		Interval: agent.Duration{Duration: time.Second},
		Tools:    []agent.Tool{},
		Extra:    make(map[string]any),
	}
}

// AssertEventuallyTrue waits for a condition to become true
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("condition not met within %v: %s", timeout, msg)
}

// AssertNoError fails if err is not nil
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertEqual fails if expected != actual
func AssertEqual(t *testing.T, expected, actual any, msg string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertContains fails if substr is not in s
func AssertContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	if len(substr) > 0 && len(s) >= len(substr) {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return
			}
		}
	}
	t.Errorf("%s: %q does not contain %q", msg, s, substr)
}
