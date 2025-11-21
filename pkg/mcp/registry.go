package mcp

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// ToolRegistry manages tools across multiple MCP servers
type ToolRegistry struct {
	tools         map[string]Tool   // tool_name -> Tool
	serverMapping map[string]string // tool_name -> server_name
	mu            sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:         make(map[string]Tool),
		serverMapping: make(map[string]string),
	}
}

// Register registers tools from a server with collision detection
func (r *ToolRegistry) Register(serverName string, tools []Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for conflicts first
	conflicts := []string{}
	for _, tool := range tools {
		if existingServer, exists := r.serverMapping[tool.Name]; exists {
			conflicts = append(conflicts,
				fmt.Sprintf("%s (existing: %s, new: %s)",
					tool.Name, existingServer, serverName))
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("tool name conflicts detected: %s", strings.Join(conflicts, ", "))
	}

	// Register tools with both namespaced and non-namespaced names
	for _, tool := range tools {
		// Always register with namespace
		namespacedName := fmt.Sprintf("%s:%s", serverName, tool.Name)
		r.tools[namespacedName] = tool
		r.serverMapping[namespacedName] = serverName

		// Also register without namespace for convenience (no conflict guaranteed above)
		r.tools[tool.Name] = tool
		r.serverMapping[tool.Name] = serverName
	}

	return nil
}

// RegisterWithOverride registers tools from a server, overriding existing tools (use with caution)
func (r *ToolRegistry) RegisterWithOverride(serverName string, tools []Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, tool := range tools {
		// Warn if overriding
		if existingServer, exists := r.serverMapping[tool.Name]; exists && existingServer != serverName {
			log.Printf("WARNING: Overriding tool %s from server %s with version from %s",
				tool.Name, existingServer, serverName)
		}

		// Register with both names
		namespacedName := fmt.Sprintf("%s:%s", serverName, tool.Name)
		r.tools[namespacedName] = tool
		r.serverMapping[namespacedName] = serverName
		r.tools[tool.Name] = tool
		r.serverMapping[tool.Name] = serverName
	}
}

// GetTool retrieves a tool by name
func (r *ToolRegistry) GetTool(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return Tool{}, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

// GetServer returns the server name that hosts a tool
func (r *ToolRegistry) GetServer(toolName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.serverMapping[toolName]
}

// ListTools returns all registered tools
func (r *ToolRegistry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// HasTool checks if a tool is registered
func (r *ToolRegistry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}
