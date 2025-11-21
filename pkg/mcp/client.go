package mcp

import (
	"context"
	"fmt"
	"sync"
)

// Client represents an MCP client that connects to servers
type Client struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewClient creates a new MCP client
func NewClient() *Client {
	return &Client{
		sessions: make(map[string]*Session),
	}
}

// Connect establishes a connection to an MCP server
func (c *Client) Connect(ctx context.Context, config ServerConfig) (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if session already exists
	if session, exists := c.sessions[config.Name]; exists {
		return session, nil
	}

	// Create transport based on config
	var transport Transport
	var err error

	switch config.Transport {
	case "local":
		transport, err = NewLocalTransport(config.Name)
	case "grpc":
		transport, err = NewGRPCTransport(config)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", config.Transport)
	}

	if err != nil {
		return nil, fmt.Errorf("create transport: %w", err)
	}

	session := &Session{
		name:      config.Name,
		transport: transport,
		tools:     make(map[string]Tool),
	}

	c.sessions[config.Name] = session
	return session, nil
}

// GetSession retrieves an existing session by name
func (c *Client) GetSession(name string) (*Session, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, exists := c.sessions[name]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", name)
	}
	return session, nil
}

// Close closes all sessions
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, session := range c.sessions {
		if err := session.Close(); err != nil {
			return err
		}
	}
	c.sessions = make(map[string]*Session)
	return nil
}

// Session represents a connection to an MCP server
type Session struct {
	name      string
	transport Transport
	tools     map[string]Tool
	mu        sync.RWMutex
}

// ListTools retrieves available tools from the server
func (s *Session) ListTools(ctx context.Context) ([]Tool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Call server's tools/list method
	result, err := s.transport.Send(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	// Parse result
	tools, ok := result.([]Tool)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	// Cache tools
	for _, tool := range tools {
		s.tools[tool.Name] = tool
	}

	return tools, nil
}

// CallTool executes a tool on the server
func (s *Session) CallTool(ctx context.Context, params CallToolParams) (*CallToolResult, error) {
	// Call server's tools/call method
	result, err := s.transport.Send(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("call tool: %w", err)
	}

	// Parse result
	toolResult, ok := result.(*CallToolResult)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return toolResult, nil
}

// GetTool retrieves a cached tool definition
func (s *Session) GetTool(name string) (Tool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tool, exists := s.tools[name]
	return tool, exists
}

// Close closes the session
func (s *Session) Close() error {
	return s.transport.Close()
}
