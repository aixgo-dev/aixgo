// Package tools provides MCP-compatible tools for AI agents.
// This is a shared package used by both the assistant CLI and daemon agents.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Tool represents a callable tool.
type Tool struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Parameters           json.RawMessage `json:"parameters"` // JSON Schema
	Handler              ToolHandler     `json:"-"`
	RequiresConfirmation bool            `json:"requires_confirmation"`
}

// ToolHandler is a function that handles tool invocations.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// ConfirmationHandler handles confirmation prompts for tools that require user approval.
type ConfirmationHandler interface {
	Confirm(ctx context.Context, tool *Tool, args map[string]any) (bool, error)
}

// Registry manages available tools.
type Registry struct {
	tools               map[string]*Tool
	mu                  sync.RWMutex
	confirmationHandler ConfirmationHandler
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*Tool),
	}
}

// SetConfirmationHandler sets the handler for tool confirmations.
func (r *Registry) SetConfirmationHandler(h ConfirmationHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.confirmationHandler = h
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools.
func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Call invokes a tool by name.
func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	if tool.Handler == nil {
		return nil, fmt.Errorf("tool has no handler: %s", name)
	}

	// Check if confirmation is required
	if tool.RequiresConfirmation && r.confirmationHandler != nil {
		confirmed, err := r.confirmationHandler.Confirm(ctx, tool, args)
		if err != nil {
			return nil, fmt.Errorf("confirmation failed: %w", err)
		}
		if !confirmed {
			return nil, fmt.Errorf("tool execution cancelled by user")
		}
	}

	return tool.Handler(ctx, args)
}

// ToMCPTools converts tools to MCP-compatible format.
func (r *Registry) ToMCPTools() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mcpTools := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		mcpTool := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if tool.Parameters != nil {
			var params map[string]any
			if err := json.Unmarshal(tool.Parameters, &params); err == nil {
				mcpTool["parameters"] = params
			}
		}
		mcpTools = append(mcpTools, mcpTool)
	}

	return mcpTools
}

// DefaultRegistry is the global tool registry.
var DefaultRegistry = NewRegistry()

// Register registers a tool to the default registry.
func Register(tool *Tool) {
	DefaultRegistry.Register(tool)
}

// Call invokes a tool from the default registry.
func Call(ctx context.Context, name string, args map[string]any) (any, error) {
	return DefaultRegistry.Call(ctx, name, args)
}
