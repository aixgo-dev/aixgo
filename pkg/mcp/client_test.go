package mcp

import (
	"context"
	"errors"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient() returned nil")
		return
	}
	if client.sessions == nil {
		t.Error("client.sessions is nil, want initialized map")
	}
}

func TestClient_Connect(t *testing.T) {
	// Setup: register a local server for testing
	testServer := NewServer("test-local")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "test-local")
		localMu.Unlock()
	}()

	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "connect to local server",
			config: ServerConfig{
				Name:      "test-local",
				Transport: "local",
			},
			wantErr: false,
		},
		{
			name: "connect to non-existent local server",
			config: ServerConfig{
				Name:      "non-existent",
				Transport: "local",
			},
			wantErr: true,
		},
		{
			name: "unsupported transport",
			config: ServerConfig{
				Name:      "test",
				Transport: "websocket",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			ctx := context.Background()

			session, err := client.Connect(ctx, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("Connect() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("Connect() unexpected error: %v", err)
				}
				if session == nil {
					t.Error("Connect() session = nil, want non-nil")
					return
				}
				if session.name != tt.config.Name {
					t.Errorf("session.name = %q, want %q", session.name, tt.config.Name)
				}
			}
		})
	}
}

func TestClient_Connect_Reuse(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("reuse-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "reuse-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	config := ServerConfig{
		Name:      "reuse-test",
		Transport: "local",
	}

	// First connection
	session1, err := client.Connect(ctx, config)
	if err != nil {
		t.Fatalf("First Connect() failed: %v", err)
	}

	// Second connection should return the same session
	session2, err := client.Connect(ctx, config)
	if err != nil {
		t.Fatalf("Second Connect() failed: %v", err)
	}

	if session1 != session2 {
		t.Error("Connect() returned different sessions for same server, want same session")
	}
}

func TestClient_GetSession(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("session-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "session-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	// Connect first
	config := ServerConfig{
		Name:      "session-test",
		Transport: "local",
	}
	_, err := client.Connect(ctx, config)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	tests := []struct {
		name        string
		sessionName string
		wantErr     bool
	}{
		{
			name:        "get existing session",
			sessionName: "session-test",
			wantErr:     false,
		},
		{
			name:        "get non-existent session",
			sessionName: "non-existent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := client.GetSession(tt.sessionName)

			if tt.wantErr {
				if err == nil {
					t.Error("GetSession() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("GetSession() unexpected error: %v", err)
				}
				if session == nil {
					t.Error("GetSession() session = nil, want non-nil")
				}
			}
		})
	}
}

func TestClient_Close(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("close-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "close-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	// Connect to server
	config := ServerConfig{
		Name:      "close-test",
		Transport: "local",
	}
	_, err := client.Connect(ctx, config)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Close client
	err = client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}

	// Verify sessions are cleared
	if len(client.sessions) != 0 {
		t.Errorf("Close() left %d sessions, want 0", len(client.sessions))
	}

	// GetSession should fail after close
	_, err = client.GetSession("close-test")
	if err == nil {
		t.Error("GetSession() after Close() error = nil, want error")
	}
}

func TestSession_ListTools(t *testing.T) {
	// Setup: register a local server with tools
	testServer := NewServer("tools-test")
	_ = testServer.RegisterTool(Tool{
		Name:        "tool1",
		Description: "First tool",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return "result1", nil
		},
	})
	_ = testServer.RegisterTool(Tool{
		Name:        "tool2",
		Description: "Second tool",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return "result2", nil
		},
	})
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "tools-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	session, err := client.Connect(ctx, ServerConfig{
		Name:      "tools-test",
		Transport: "local",
	})
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	tools, err := session.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools() error = %v, want nil", err)
	}

	if len(tools) != 2 {
		t.Errorf("ListTools() returned %d tools, want 2", len(tools))
	}

	// Verify tools are cached
	if len(session.tools) != 2 {
		t.Errorf("Session cached %d tools, want 2", len(session.tools))
	}
}

func TestSession_CallTool(t *testing.T) {
	// Setup: register a local server with tools
	testServer := NewServer("call-test")
	_ = testServer.RegisterTool(Tool{
		Name:        "greet",
		Description: "Returns a greeting",
		Handler: func(ctx context.Context, args Args) (any, error) {
			name := args.String("name")
			return "Hello, " + name, nil
		},
	})
	_ = testServer.RegisterTool(Tool{
		Name:        "failing",
		Description: "Always fails",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return nil, errors.New("intentional failure")
		},
	})
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "call-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	session, err := client.Connect(ctx, ServerConfig{
		Name:      "call-test",
		Transport: "local",
	})
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	tests := []struct {
		name        string
		params      CallToolParams
		wantErr     bool
		wantIsError bool
		wantContent string
	}{
		{
			name: "call successful tool",
			params: CallToolParams{
				Name:      "greet",
				Arguments: map[string]any{"name": "World"},
			},
			wantErr:     false,
			wantIsError: false,
			wantContent: "Hello, World",
		},
		{
			name: "call failing tool",
			params: CallToolParams{
				Name:      "failing",
				Arguments: map[string]any{},
			},
			wantErr:     false,
			wantIsError: true,
			wantContent: "tool execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, tt.params)

			if tt.wantErr {
				if err == nil {
					t.Error("CallTool() error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Errorf("CallTool() unexpected error: %v", err)
				return
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("CallTool() IsError = %v, want %v", result.IsError, tt.wantIsError)
			}

			if len(result.Content) > 0 && result.Content[0].Text != tt.wantContent {
				t.Errorf("CallTool() content = %q, want %q", result.Content[0].Text, tt.wantContent)
			}
		})
	}
}

func TestSession_GetTool(t *testing.T) {
	// Setup: register a local server with tools
	testServer := NewServer("gettool-test")
	_ = testServer.RegisterTool(Tool{
		Name:        "existing",
		Description: "An existing tool",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return "result", nil
		},
	})
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "gettool-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	session, err := client.Connect(ctx, ServerConfig{
		Name:      "gettool-test",
		Transport: "local",
	})
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// List tools first to populate cache
	_, err = session.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}

	tests := []struct {
		name       string
		toolName   string
		wantExists bool
	}{
		{
			name:       "get existing tool",
			toolName:   "existing",
			wantExists: true,
		},
		{
			name:       "get non-existent tool",
			toolName:   "non-existent",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, exists := session.GetTool(tt.toolName)

			if exists != tt.wantExists {
				t.Errorf("GetTool() exists = %v, want %v", exists, tt.wantExists)
			}

			if tt.wantExists && tool.Name != tt.toolName {
				t.Errorf("GetTool() name = %q, want %q", tool.Name, tt.toolName)
			}
		})
	}
}

func TestSession_Close(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("session-close-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "session-close-test")
		localMu.Unlock()
	}()

	client := NewClient()
	ctx := context.Background()

	session, err := client.Connect(ctx, ServerConfig{
		Name:      "session-close-test",
		Transport: "local",
	})
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	err = session.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}
