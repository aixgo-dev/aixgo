package mcp

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
	}{
		{
			name:       "create server with name",
			serverName: "test-server",
		},
		{
			name:       "create server with empty name",
			serverName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.serverName)
			if server == nil {
				t.Fatal("NewServer returned nil")
				return
			}
			if server.name != tt.serverName {
				t.Errorf("server.name = %q, want %q", server.name, tt.serverName)
			}
			if server.tools == nil {
				t.Error("server.tools is nil, want initialized map")
			}
		})
	}
}

func TestServer_RegisterTool(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		wantErr bool
		errMsg  string
	}{
		{
			name: "register valid tool",
			tool: Tool{
				Name:        "test_tool",
				Description: "A test tool",
				Handler: func(ctx context.Context, args Args) (any, error) {
					return "result", nil
				},
			},
			wantErr: false,
		},
		{
			name: "register tool with empty name",
			tool: Tool{
				Name:        "",
				Description: "A test tool",
				Handler: func(ctx context.Context, args Args) (any, error) {
					return "result", nil
				},
			},
			wantErr: true,
			errMsg:  "tool name cannot be empty",
		},
		{
			name: "register tool with nil handler",
			tool: Tool{
				Name:        "test_tool",
				Description: "A test tool",
				Handler:     nil,
			},
			wantErr: true,
			errMsg:  "tool handler cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer("test")
			err := server.RegisterTool(tt.tool)

			if tt.wantErr {
				if err == nil {
					t.Error("RegisterTool() error = nil, want error")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("RegisterTool() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("RegisterTool() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestServer_RegisterTool_Duplicate(t *testing.T) {
	server := NewServer("test")

	tool := Tool{
		Name:        "duplicate_tool",
		Description: "First registration",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return "first", nil
		},
	}

	// First registration should succeed
	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("First RegisterTool() failed: %v", err)
	}

	// Second registration with same name should fail
	tool.Description = "Second registration"
	err := server.RegisterTool(tool)
	if err == nil {
		t.Error("RegisterTool() with duplicate name error = nil, want error")
	}
	if err.Error() != "tool duplicate_tool already registered" {
		t.Errorf("RegisterTool() error = %q, want 'tool duplicate_tool already registered'", err.Error())
	}
}

func TestServer_ListTools(t *testing.T) {
	server := NewServer("test")

	// Empty list initially
	tools := server.ListTools()
	if len(tools) != 0 {
		t.Errorf("ListTools() on empty server returned %d tools, want 0", len(tools))
	}

	// Register tools
	tool1 := Tool{
		Name:        "tool1",
		Description: "First tool",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return "result1", nil
		},
	}
	tool2 := Tool{
		Name:        "tool2",
		Description: "Second tool",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return "result2", nil
		},
	}

	_ = server.RegisterTool(tool1)
	_ = server.RegisterTool(tool2)

	tools = server.ListTools()
	if len(tools) != 2 {
		t.Errorf("ListTools() returned %d tools, want 2", len(tools))
	}

	// Verify tools are in the list
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["tool1"] {
		t.Error("ListTools() missing tool1")
	}
	if !toolNames["tool2"] {
		t.Error("ListTools() missing tool2")
	}
}

func TestServer_CallTool(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*Server)
		params      CallToolParams
		wantErr     bool
		wantIsError bool
		wantContent string
	}{
		{
			name: "call existing tool",
			setup: func(s *Server) {
				_ = s.RegisterTool(Tool{
					Name:        "greeting",
					Description: "Returns a greeting",
					Handler: func(ctx context.Context, args Args) (any, error) {
						name := args.String("name")
						return "Hello, " + name, nil
					},
				})
			},
			params: CallToolParams{
				Name:      "greeting",
				Arguments: map[string]any{"name": "Alice"},
			},
			wantErr:     false,
			wantIsError: false,
			wantContent: "Hello, Alice",
		},
		{
			name: "call non-existent tool",
			setup: func(s *Server) {
				// No tools registered
			},
			params: CallToolParams{
				Name:      "nonexistent",
				Arguments: map[string]any{},
			},
			wantErr:     false,
			wantIsError: true,
			wantContent: "tool not found: nonexistent",
		},
		{
			name: "tool returns error",
			setup: func(s *Server) {
				_ = s.RegisterTool(Tool{
					Name:        "failing_tool",
					Description: "A tool that fails",
					Handler: func(ctx context.Context, args Args) (any, error) {
						return nil, errors.New("tool execution failed")
					},
				})
			},
			params: CallToolParams{
				Name:      "failing_tool",
				Arguments: map[string]any{},
			},
			wantErr:     false,
			wantIsError: true,
			wantContent: "tool execution failed",
		},
		{
			name: "tool returns map",
			setup: func(s *Server) {
				_ = s.RegisterTool(Tool{
					Name:        "data_tool",
					Description: "Returns structured data",
					Handler: func(ctx context.Context, args Args) (any, error) {
						return map[string]any{
							"status": "success",
							"count":  42,
						}, nil
					},
				})
			},
			params: CallToolParams{
				Name:      "data_tool",
				Arguments: map[string]any{},
			},
			wantErr:     false,
			wantIsError: false,
			wantContent: `{"count":42,"status":"success"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer("test")
			tt.setup(server)

			ctx := context.Background()
			result, err := server.CallTool(ctx, tt.params)

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

			if len(result.Content) == 0 {
				t.Fatal("CallTool() result.Content is empty")
			}

			if result.Content[0].Text != tt.wantContent {
				t.Errorf("CallTool() content = %q, want %q", result.Content[0].Text, tt.wantContent)
			}
		})
	}
}

func TestServer_CallTool_Concurrent(t *testing.T) {
	server := NewServer("test")

	// Register a tool
	_ = server.RegisterTool(Tool{
		Name:        "counter",
		Description: "Increments a counter",
		Handler: func(ctx context.Context, args Args) (any, error) {
			num := args.Int("value")
			return num + 1, nil
		},
	})

	// Call tool concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	wg.Add(numGoroutines)

	ctx := context.Background()
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(val int) {
			defer wg.Done()
			_, err := server.CallTool(ctx, CallToolParams{
				Name:      "counter",
				Arguments: map[string]any{"value": val},
			})
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent CallTool() error: %v", err)
	}
}

func TestServer_Name(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
	}{
		{
			name:       "get server name",
			serverName: "my-server",
		},
		{
			name:       "get empty server name",
			serverName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.serverName)
			if server.Name() != tt.serverName {
				t.Errorf("Name() = %q, want %q", server.Name(), tt.serverName)
			}
		})
	}
}

func TestServer_Serve(t *testing.T) {
	server := NewServer("test")

	// Test with nil transport
	err := server.Serve(nil)
	if err != nil {
		t.Errorf("Serve(nil) error = %v, want nil", err)
	}

	// Test with mock transport
	mockTransport := &mockTransport{}
	_ = server.Serve(mockTransport)
	if err != nil {
		t.Errorf("Serve(mockTransport) error = %v, want nil", err)
	}
}

func TestServer_Close(t *testing.T) {
	server := NewServer("test")

	// Test close without transport
	err := server.Close()
	if err != nil {
		t.Errorf("Close() without transport error = %v, want nil", err)
	}

	// Test close with mock transport
	mockTrans := &mockTransport{closeErr: nil}
	_ = server.Serve(mockTrans)

	err = server.Close()
	if err != nil {
		t.Errorf("Close() with transport error = %v, want nil", err)
	}

	if !mockTrans.closed {
		t.Error("Close() did not call transport.Close()")
	}

	// Test close with transport that returns error
	server2 := NewServer("test2")
	mockTrans2 := &mockTransport{closeErr: errors.New("close failed")}
	_ = server2.Serve(mockTrans2)

	err = server2.Close()
	if err == nil {
		t.Error("Close() error = nil, want error")
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		wantType string
		wantText string
	}{
		{
			name:     "string result",
			input:    "hello world",
			wantType: "text",
			wantText: "hello world",
		},
		{
			name:     "map result",
			input:    map[string]any{"key": "value"},
			wantType: "text",
			wantText: `{"key":"value"}`,
		},
		{
			name:     "slice result",
			input:    []any{"a", "b", "c"},
			wantType: "text",
			wantText: `["a","b","c"]`,
		},
		{
			name:     "integer result",
			input:    42,
			wantType: "text",
			wantText: "42",
		},
		{
			name:     "boolean result",
			input:    true,
			wantType: "text",
			wantText: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := formatResult(tt.input)

			if content.Type != tt.wantType {
				t.Errorf("formatResult() type = %q, want %q", content.Type, tt.wantType)
			}

			if content.Text != tt.wantText {
				t.Errorf("formatResult() text = %q, want %q", content.Text, tt.wantText)
			}
		})
	}
}

// Mock transport for testing
type mockTransport struct {
	closed   bool
	closeErr error
}

func (m *mockTransport) Send(ctx context.Context, method string, params any) (any, error) {
	return nil, nil
}

func (m *mockTransport) Close() error {
	m.closed = true
	return m.closeErr
}
