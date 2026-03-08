// Package terminal re-exports the shared terminal tools for backwards compatibility.
// New code should import from "github.com/aixgo-dev/aixgo/pkg/tools/terminal" directly.
//
// Deprecated: Use github.com/aixgo-dev/aixgo/pkg/tools/terminal instead.
package terminal

import (
	sharedterminal "github.com/aixgo-dev/aixgo/pkg/tools/terminal"
)

// Re-export variables for backwards compatibility
var (
	AllowedCommands    = sharedterminal.AllowedCommands
	BlockedSubcommands = sharedterminal.BlockedSubcommands
)

// Re-export functions for backwards compatibility
var (
	ExecTool        = sharedterminal.ExecTool
	ValidateCommand = sharedterminal.ValidateCommand
	IsSafePipe      = sharedterminal.IsSafePipe
	IsSafeRedirect  = sharedterminal.IsSafeRedirect
)

// Re-export types
type ExecResult = sharedterminal.ExecResult
