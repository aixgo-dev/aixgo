// Package git re-exports the shared git tools for backwards compatibility.
// New code should import from "github.com/aixgo-dev/aixgo/pkg/tools/git" directly.
//
// Deprecated: Use github.com/aixgo-dev/aixgo/pkg/tools/git instead.
package git

import (
	sharedgit "github.com/aixgo-dev/aixgo/pkg/tools/git"
)

// Re-export functions for backwards compatibility
var (
	StatusTool = sharedgit.StatusTool
	DiffTool   = sharedgit.DiffTool
	CommitTool = sharedgit.CommitTool
	LogTool    = sharedgit.LogTool
)
