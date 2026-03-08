// Package tools re-exports the shared tools package for backwards compatibility.
// New code should import from "github.com/aixgo-dev/aixgo/pkg/tools" directly.
//
// Deprecated: Use github.com/aixgo-dev/aixgo/pkg/tools instead.
package tools

import (
	"context"

	sharedtools "github.com/aixgo-dev/aixgo/pkg/tools"
)

// Re-export types for backwards compatibility
type (
	Tool                = sharedtools.Tool
	ToolHandler         = sharedtools.ToolHandler
	ConfirmationHandler = sharedtools.ConfirmationHandler
	Registry            = sharedtools.Registry
)

// Re-export functions for backwards compatibility
var (
	NewRegistry     = sharedtools.NewRegistry
	DefaultRegistry = sharedtools.DefaultRegistry
)

// Register registers a tool to the default registry.
func Register(tool *Tool) {
	sharedtools.Register(tool)
}

// Call invokes a tool from the default registry.
func Call(ctx context.Context, name string, args map[string]any) (any, error) {
	return sharedtools.Call(ctx, name, args)
}
