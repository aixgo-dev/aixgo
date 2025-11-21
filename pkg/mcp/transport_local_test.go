package mcp

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRegisterLocalServer(t *testing.T) {
	// Clean up before test
	localMu.Lock()
	localServers = make(map[string]*Server)
	localMu.Unlock()

	server1 := NewServer("server1")
	server2 := NewServer("server2")

	RegisterLocalServer(server1)
	RegisterLocalServer(server2)

	localMu.RLock()
	defer localMu.RUnlock()

	if len(localServers) != 2 {
		t.Errorf("RegisterLocalServer() registered %d servers, want 2", len(localServers))
	}

	if localServers["server1"] != server1 {
		t.Error("RegisterLocalServer() did not register server1 correctly")
	}

	if localServers["server2"] != server2 {
		t.Error("RegisterLocalServer() did not register server2 correctly")
	}
}

func TestRegisterLocalServer_Overwrite(t *testing.T) {
	// Clean up before test
	localMu.Lock()
	localServers = make(map[string]*Server)
	localMu.Unlock()

	server1 := NewServer("overwrite-test")
	server2 := NewServer("overwrite-test")

	RegisterLocalServer(server1)
	RegisterLocalServer(server2)

	localMu.RLock()
	defer localMu.RUnlock()

	if localServers["overwrite-test"] != server2 {
		t.Error("RegisterLocalServer() did not overwrite server correctly")
	}
}

func TestNewLocalTransport(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("test-transport")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "test-transport")
		localMu.Unlock()
	}()

	tests := []struct {
		name       string
		serverName string
		wantErr    bool
	}{
		{
			name:       "create transport for existing server",
			serverName: "test-transport",
			wantErr:    false,
		},
		{
			name:       "create transport for non-existent server",
			serverName: "non-existent",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := NewLocalTransport(tt.serverName)

			if tt.wantErr {
				if err == nil {
					t.Error("NewLocalTransport() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("NewLocalTransport() unexpected error: %v", err)
				}
				if transport == nil {
					t.Error("NewLocalTransport() transport = nil, want non-nil")
					return
				}
				if transport.serverName != tt.serverName {
					t.Errorf("transport.serverName = %q, want %q", transport.serverName, tt.serverName)
				}
			}
		})
	}
}

func TestLocalTransport_Send_ToolsList(t *testing.T) {
	// Setup: register a local server with tools
	testServer := NewServer("list-test")
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
		delete(localServers, "list-test")
		localMu.Unlock()
	}()

	transport, err := NewLocalTransport("list-test")
	if err != nil {
		t.Fatalf("NewLocalTransport() failed: %v", err)
	}

	ctx := context.Background()
	result, err := transport.Send(ctx, "tools/list", nil)
	if err != nil {
		t.Fatalf("Send(tools/list) error = %v, want nil", err)
	}

	tools, ok := result.([]Tool)
	if !ok {
		t.Fatalf("Send(tools/list) result type = %T, want []Tool", result)
	}

	if len(tools) != 2 {
		t.Errorf("Send(tools/list) returned %d tools, want 2", len(tools))
	}
}

func TestLocalTransport_Send_ToolsCall(t *testing.T) {
	// Setup: register a local server with tools
	testServer := NewServer("call-test")
	_ = testServer.RegisterTool(Tool{
		Name:        "multiply",
		Description: "Multiplies two numbers",
		Handler: func(ctx context.Context, args Args) (any, error) {
			a := args.Int("a")
			b := args.Int("b")
			return a * b, nil
		},
	})
	_ = testServer.RegisterTool(Tool{
		Name:        "failing",
		Description: "Always fails",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return nil, errors.New("tool error")
		},
	})
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "call-test")
		localMu.Unlock()
	}()

	transport, err := NewLocalTransport("call-test")
	if err != nil {
		t.Fatalf("NewLocalTransport() failed: %v", err)
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
				Name:      "multiply",
				Arguments: map[string]any{"a": 6.0, "b": 7.0},
			},
			wantErr:     false,
			wantIsError: false,
			wantContent: "42",
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
			ctx := context.Background()
			result, err := transport.Send(ctx, "tools/call", tt.params)

			if tt.wantErr {
				if err == nil {
					t.Error("Send(tools/call) error = nil, want error")
				}
				return
			}

			if err != nil {
				t.Errorf("Send(tools/call) unexpected error: %v", err)
				return
			}

			callResult, ok := result.(*CallToolResult)
			if !ok {
				t.Fatalf("Send(tools/call) result type = %T, want *CallToolResult", result)
			}

			if callResult.IsError != tt.wantIsError {
				t.Errorf("Send(tools/call) IsError = %v, want %v", callResult.IsError, tt.wantIsError)
			}

			if len(callResult.Content) > 0 && callResult.Content[0].Text != tt.wantContent {
				t.Errorf("Send(tools/call) content = %q, want %q", callResult.Content[0].Text, tt.wantContent)
			}
		})
	}
}

func TestLocalTransport_Send_InvalidParams(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("invalid-params-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "invalid-params-test")
		localMu.Unlock()
	}()

	transport, err := NewLocalTransport("invalid-params-test")
	if err != nil {
		t.Fatalf("NewLocalTransport() failed: %v", err)
	}

	ctx := context.Background()

	// Send tools/call with wrong params type
	_, err = transport.Send(ctx, "tools/call", "invalid-params")
	if err == nil {
		t.Error("Send(tools/call) with invalid params error = nil, want error")
	}
}

func TestLocalTransport_Send_UnsupportedMethod(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("unsupported-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "unsupported-test")
		localMu.Unlock()
	}()

	transport, err := NewLocalTransport("unsupported-test")
	if err != nil {
		t.Fatalf("NewLocalTransport() failed: %v", err)
	}

	ctx := context.Background()
	_, err = transport.Send(ctx, "unsupported/method", nil)
	if err == nil {
		t.Error("Send(unsupported/method) error = nil, want error")
	}
}

func TestLocalTransport_Close(t *testing.T) {
	// Setup: register a local server
	testServer := NewServer("close-test")
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "close-test")
		localMu.Unlock()
	}()

	transport, err := NewLocalTransport("close-test")
	if err != nil {
		t.Fatalf("NewLocalTransport() failed: %v", err)
	}

	err = transport.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestLocalTransport_Concurrent(t *testing.T) {
	// Setup: register a local server with a tool
	testServer := NewServer("concurrent-test")
	_ = testServer.RegisterTool(Tool{
		Name:        "counter",
		Description: "Returns the input value",
		Handler: func(ctx context.Context, args Args) (any, error) {
			return args.Int("value"), nil
		},
	})
	RegisterLocalServer(testServer)
	defer func() {
		localMu.Lock()
		delete(localServers, "concurrent-test")
		localMu.Unlock()
	}()

	transport, err := NewLocalTransport("concurrent-test")
	if err != nil {
		t.Fatalf("NewLocalTransport() failed: %v", err)
	}

	// Call transport concurrently
	var wg sync.WaitGroup
	numGoroutines := 20
	wg.Add(numGoroutines)

	ctx := context.Background()
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(val int) {
			defer wg.Done()

			// Alternate between tools/list and tools/call
			if val%2 == 0 {
				_, err := transport.Send(ctx, "tools/list", nil)
				if err != nil {
					errors <- err
				}
			} else {
				params := CallToolParams{
					Name:      "counter",
					Arguments: map[string]any{"value": float64(val)},
				}
				_, err := transport.Send(ctx, "tools/call", params)
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent Send() error: %v", err)
	}
}
