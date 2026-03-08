// Package file re-exports the shared file tools for backwards compatibility.
// New code should import from "github.com/aixgo-dev/aixgo/pkg/tools/file" directly.
//
// Deprecated: Use github.com/aixgo-dev/aixgo/pkg/tools/file instead.
package file

import (
	sharedfile "github.com/aixgo-dev/aixgo/pkg/tools/file"
)

// Re-export functions for backwards compatibility
var (
	ReadFileTool  = sharedfile.ReadFileTool
	WriteFileTool = sharedfile.WriteFileTool
	GlobTool      = sharedfile.GlobTool
	GrepTool      = sharedfile.GrepTool
	ValidatePath  = sharedfile.ValidatePath
)

// Re-export types
type GrepMatch = sharedfile.GrepMatch
