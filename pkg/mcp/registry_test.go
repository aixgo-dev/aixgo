package mcp

import (
	"context"
	"sync"
	"testing"
)

func TestNewToolRegistry(t *testing.T) {
	registry := NewToolRegistry()
	if registry == nil {
		t.Fatal("NewToolRegistry() returned nil")
		return
	}
	if registry.tools == nil {
		t.Error("registry.tools is nil, want initialized map")
	}
	if registry.serverMapping == nil {
		t.Error("registry.serverMapping is nil, want initialized map")
	}
}

func TestToolRegistry_Register(t *testing.T) {
	registry := NewToolRegistry()

	tools := []Tool{
		{
			Name:        "tool1",
			Description: "First tool",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result1", nil
			},
		},
		{
			Name:        "tool2",
			Description: "Second tool",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result2", nil
			},
		},
	}

	_ = registry.Register("server1", tools)

	// Verify tools are registered (both namespaced and non-namespaced)
	// Each tool is registered twice: "tool1", "server1:tool1", "tool2", "server1:tool2"
	if len(registry.tools) != 4 {
		t.Errorf("Register() registered %d tools, want 4", len(registry.tools))
	}

	// Verify server mapping
	if registry.serverMapping["tool1"] != "server1" {
		t.Errorf("serverMapping[tool1] = %q, want 'server1'", registry.serverMapping["tool1"])
	}
	if registry.serverMapping["tool2"] != "server1" {
		t.Errorf("serverMapping[tool2] = %q, want 'server1'", registry.serverMapping["tool2"])
	}
}

func TestToolRegistry_Register_MultipleServers(t *testing.T) {
	registry := NewToolRegistry()

	tools1 := []Tool{
		{
			Name:        "server1_tool",
			Description: "Tool from server1",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result1", nil
			},
		},
	}

	tools2 := []Tool{
		{
			Name:        "server2_tool",
			Description: "Tool from server2",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result2", nil
			},
		},
	}

	_ = registry.Register("server1", tools1)
	_ = registry.Register("server2", tools2)

	// Verify both servers' tools are registered (both namespaced and non-namespaced)
	// Each tool is registered twice: "server1_tool", "server1:server1_tool", "server2_tool", "server2:server2_tool"
	if len(registry.tools) != 4 {
		t.Errorf("Register() registered %d tools, want 4", len(registry.tools))
	}

	if registry.GetServer("server1_tool") != "server1" {
		t.Error("server1_tool not mapped to server1")
	}

	if registry.GetServer("server2_tool") != "server2" {
		t.Error("server2_tool not mapped to server2")
	}
}

func TestToolRegistry_Register_Overwrite(t *testing.T) {
	registry := NewToolRegistry()

	// Register tool from server1
	tools1 := []Tool{
		{
			Name:        "shared_tool",
			Description: "First version",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result1", nil
			},
		},
	}
	registry.RegisterWithOverride("server1", tools1)

	// Register same tool from server2 (should overwrite using RegisterWithOverride)
	tools2 := []Tool{
		{
			Name:        "shared_tool",
			Description: "Second version",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result2", nil
			},
		},
	}
	registry.RegisterWithOverride("server2", tools2)

	// Verify tool is mapped to server2 (non-namespaced name)
	if registry.GetServer("shared_tool") != "server2" {
		t.Errorf("shared_tool mapped to %q, want 'server2'", registry.GetServer("shared_tool"))
	}

	// Verify namespaced version is also mapped to server2
	if registry.GetServer("server2:shared_tool") != "server2" {
		t.Errorf("server2:shared_tool mapped to %q, want 'server2'", registry.GetServer("server2:shared_tool"))
	}

	// Verify the non-namespaced tool has the second version
	tool, err := registry.GetTool("shared_tool")
	if err != nil {
		t.Fatalf("GetTool() error = %v, want nil", err)
	}
	if tool.Description != "Second version" {
		t.Errorf("tool.Description = %q, want 'Second version'", tool.Description)
	}

	// Verify the namespaced tool from server1 still exists with first version
	tool1, err := registry.GetTool("server1:shared_tool")
	if err != nil {
		t.Fatalf("GetTool('server1:shared_tool') error = %v, want nil", err)
	}
	if tool1.Description != "First version" {
		t.Errorf("server1:shared_tool.Description = %q, want 'First version'", tool1.Description)
	}
}

func TestToolRegistry_GetTool(t *testing.T) {
	registry := NewToolRegistry()

	tools := []Tool{
		{
			Name:        "existing_tool",
			Description: "An existing tool",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result", nil
			},
		},
	}
	_ = registry.Register("server1", tools)

	tests := []struct {
		name     string
		toolName string
		wantErr  bool
	}{
		{
			name:     "get existing tool",
			toolName: "existing_tool",
			wantErr:  false,
		},
		{
			name:     "get non-existent tool",
			toolName: "non_existent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := registry.GetTool(tt.toolName)

			if tt.wantErr {
				if err == nil {
					t.Error("GetTool() error = nil, want error")
				}
			} else {
				if err != nil {
					t.Errorf("GetTool() unexpected error: %v", err)
				}
				if tool.Name != tt.toolName {
					t.Errorf("GetTool() name = %q, want %q", tool.Name, tt.toolName)
				}
			}
		})
	}
}

func TestToolRegistry_GetServer(t *testing.T) {
	registry := NewToolRegistry()

	tools := []Tool{
		{
			Name:        "tool1",
			Description: "First tool",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result", nil
			},
		},
	}
	_ = registry.Register("test-server", tools)

	tests := []struct {
		name       string
		toolName   string
		wantServer string
	}{
		{
			name:       "get server for existing tool",
			toolName:   "tool1",
			wantServer: "test-server",
		},
		{
			name:       "get server for non-existent tool",
			toolName:   "non_existent",
			wantServer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := registry.GetServer(tt.toolName)
			if server != tt.wantServer {
				t.Errorf("GetServer() = %q, want %q", server, tt.wantServer)
			}
		})
	}
}

func TestToolRegistry_ListTools(t *testing.T) {
	registry := NewToolRegistry()

	// Empty registry
	tools := registry.ListTools()
	if len(tools) != 0 {
		t.Errorf("ListTools() on empty registry returned %d tools, want 0", len(tools))
	}

	// Register tools from multiple servers
	tools1 := []Tool{
		{
			Name:        "server1_tool1",
			Description: "Tool 1 from server 1",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result1", nil
			},
		},
		{
			Name:        "server1_tool2",
			Description: "Tool 2 from server 1",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result2", nil
			},
		},
	}
	tools2 := []Tool{
		{
			Name:        "server2_tool1",
			Description: "Tool 1 from server 2",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result3", nil
			},
		},
	}

	_ = registry.Register("server1", tools1)
	_ = registry.Register("server2", tools2)

	// List all tools (both namespaced and non-namespaced versions)
	// 3 tools x 2 registrations each (namespaced + non-namespaced) = 6 total
	allTools := registry.ListTools()
	if len(allTools) != 6 {
		t.Errorf("ListTools() returned %d tools, want 6", len(allTools))
	}

	// Verify tool names - we should have both namespaced and non-namespaced versions
	toolNames := make(map[string]bool)
	for _, tool := range allTools {
		toolNames[tool.Name] = true
	}

	// Expected non-namespaced names
	expectedNames := []string{"server1_tool1", "server1_tool2", "server2_tool1"}
	for _, name := range expectedNames {
		if !toolNames[name] {
			t.Errorf("ListTools() missing tool %q", name)
		}
	}

	// Note: The tool names in the registry are the keys, but the Tool.Name field stays as original
	// So we're checking that we can retrieve tools, not necessarily that the keys are present in Tool.Name
}

func TestToolRegistry_HasTool(t *testing.T) {
	registry := NewToolRegistry()

	tools := []Tool{
		{
			Name:        "existing_tool",
			Description: "An existing tool",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result", nil
			},
		},
	}
	_ = registry.Register("server1", tools)

	tests := []struct {
		name     string
		toolName string
		want     bool
	}{
		{
			name:     "has existing tool",
			toolName: "existing_tool",
			want:     true,
		},
		{
			name:     "has non-existent tool",
			toolName: "non_existent",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			has := registry.HasTool(tt.toolName)
			if has != tt.want {
				t.Errorf("HasTool() = %v, want %v", has, tt.want)
			}
		})
	}
}

func TestToolRegistry_Concurrent(t *testing.T) {
	registry := NewToolRegistry()

	var wg sync.WaitGroup
	numGoroutines := 20
	wg.Add(numGoroutines)

	// Concurrent registration and reading
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Register tools
			if id%2 == 0 {
				tools := []Tool{
					{
						Name:        "tool_" + intToStr(id),
						Description: "Tool from goroutine " + intToStr(id),
						Handler: func(ctx context.Context, args Args) (any, error) {
							return "result", nil
						},
					},
				}
				_ = registry.Register("server_"+intToStr(id), tools)
			} else {
				// Read operations
				registry.ListTools()
				registry.HasTool("tool_0")
				registry.GetServer("tool_0")
			}
		}(i)
	}

	wg.Wait()

	// Verify some tools were registered
	tools := registry.ListTools()
	if len(tools) == 0 {
		t.Error("Concurrent operations resulted in no tools registered")
	}
}

func TestToolRegistry_EmptyServerRegistration(t *testing.T) {
	registry := NewToolRegistry()

	// Register empty tool list
	_ = registry.Register("empty-server", []Tool{})

	// Verify no tools were registered
	if len(registry.tools) != 0 {
		t.Errorf("Register() with empty tools registered %d tools, want 0", len(registry.tools))
	}

	// But we should be able to register tools for this server later
	tools := []Tool{
		{
			Name:        "late_tool",
			Description: "A tool added later",
			Handler: func(ctx context.Context, args Args) (any, error) {
				return "result", nil
			},
		},
	}
	_ = registry.Register("empty-server", tools)

	// Should have 2 tools: "late_tool" and "empty-server:late_tool"
	if len(registry.tools) != 2 {
		t.Errorf("Register() after empty registration has %d tools, want 2", len(registry.tools))
	}

	// Verify both namespaced and non-namespaced versions
	if registry.GetServer("late_tool") != "empty-server" {
		t.Error("late_tool not mapped to empty-server")
	}

	if registry.GetServer("empty-server:late_tool") != "empty-server" {
		t.Error("empty-server:late_tool not mapped to empty-server")
	}
}

// Helper function for converting int to string (same as in factory_test.go)
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := "0123456789"
	var result []byte
	for n > 0 {
		result = append([]byte{digits[n%10]}, result...)
		n /= 10
	}
	return string(result)
}
