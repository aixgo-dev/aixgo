// Package file provides file operation tools.
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aixgo-dev/aixgo/pkg/tools"
)

func init() {
	tools.Register(ReadFileTool())
	tools.Register(WriteFileTool())
	tools.Register(GlobTool())
	tools.Register(GrepTool())
}

// ReadFileTool returns a tool for reading file contents.
func ReadFileTool() *tools.Tool {
	return &tools.Tool{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path. Returns the file contents as a string.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The file path to read"
				},
				"start_line": {
					"type": "integer",
					"description": "Optional start line number (1-indexed)"
				},
				"end_line": {
					"type": "integer",
					"description": "Optional end line number (1-indexed, inclusive)"
				}
			},
			"required": ["path"]
		}`),
		Handler:              readFileHandler,
		RequiresConfirmation: false,
	}
}

func readFileHandler(_ context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Validate path is within working directory or allowed paths
	if err := ValidatePath(path); err != nil {
		return nil, err
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Handle line range if specified
	startLine := 0
	if v, ok := args["start_line"].(float64); ok {
		startLine = int(v) - 1 // Convert to 0-indexed
	}

	endLine := -1
	if v, ok := args["end_line"].(float64); ok {
		endLine = int(v)
	}

	if startLine > 0 || endLine > 0 {
		lines := strings.Split(string(content), "\n")
		if startLine < 0 {
			startLine = 0
		}
		if endLine < 0 || endLine > len(lines) {
			endLine = len(lines)
		}
		if startLine >= len(lines) {
			return "", nil
		}
		return strings.Join(lines[startLine:endLine], "\n"), nil
	}

	return string(content), nil
}

// WriteFileTool returns a tool for writing file contents.
func WriteFileTool() *tools.Tool {
	return &tools.Tool{
		Name:        "write_file",
		Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The file path to write to"
				},
				"content": {
					"type": "string",
					"description": "The content to write"
				}
			},
			"required": ["path", "content"]
		}`),
		Handler:              writeFileHandler,
		RequiresConfirmation: true, // Always confirm writes
	}
}

func writeFileHandler(_ context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required")
	}

	// Validate path
	if err := ValidatePath(path); err != nil {
		return nil, err
	}

	// Create parent directories if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

// ValidatePath validates that a path is safe to access.
// Exported so other packages can use the same validation logic.
func ValidatePath(path string) error {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check path is within working directory or home directory
	homeDir, _ := os.UserHomeDir()

	if !strings.HasPrefix(absPath, wd) && !strings.HasPrefix(absPath, homeDir) {
		// Allow common system paths for reading
		allowedPrefixes := []string{
			"/usr/local",
			"/etc",
			"/tmp",
			"/var/folders", // macOS temp directory
			os.TempDir(),   // System temp directory
		}
		allowed := false
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(absPath, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path outside allowed directories: %s", path)
		}
	}

	// Check for path traversal
	if strings.Contains(path, "..") {
		cleanPath := filepath.Clean(absPath)
		if !strings.HasPrefix(cleanPath, wd) && !strings.HasPrefix(cleanPath, homeDir) {
			return fmt.Errorf("path traversal not allowed: %s", path)
		}
	}

	return nil
}
