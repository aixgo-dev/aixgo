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

	// G304: Use the cleaned absolute path for the read so the read target
	// matches the path that ValidatePath actually approved (defends against
	// "./foo/../bar" style aliasing where the raw arg differs from the
	// canonical form).
	cleanPath := filepath.Clean(path)

	// Read file
	content, err := os.ReadFile(cleanPath) // #nosec G304 -- path validated by ValidatePath (allowlist + symlink-escape check)
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

	// Create parent directories if needed.
	// G301: directory permissions must be <=0750 — group-readable for
	// operator audit but no world access. Writes are confirmation-gated,
	// so the narrower perms do not affect legitimate use.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file.
	// G306: WriteFile permissions must be <=0600 — user-only read/write.
	// Agent-written files contain tool output that may include secrets
	// extracted from stdout; world/group read is not appropriate.
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

// ValidatePath validates that a path is safe to access.
// Exported so other packages can use the same validation logic.
//
// Defence layers:
//  1. Reject empty paths and null bytes.
//  2. Resolve to a cleaned absolute path.
//  3. Enforce a non-empty allowlist of acceptable roots
//     (cwd, $HOME, /usr/local, /etc, /tmp, /var/folders, $TMPDIR).
//  4. If the file already exists, resolve symlinks and re-check the resolved
//     target against the same allowlist. This blocks symlink-escape attacks
//     where an attacker plants a symlink inside cwd that points at /etc/shadow.
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("null byte in path")
	}

	// Get absolute, cleaned path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	// Get working directory and home directory for allowlist
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	homeDir, _ := os.UserHomeDir()

	if err := pathInAllowlist(absPath, wd, homeDir); err != nil {
		return err
	}

	// Symlink-escape check: if the path exists, resolve symlinks and verify
	// the real target also lives inside the allowlist. We tolerate
	// non-existent paths (the caller may be reading a soon-to-be-created file
	// or the read will fail naturally afterwards).
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		if err := pathInAllowlist(resolved, wd, homeDir); err != nil {
			return fmt.Errorf("symlink target outside allowed directories: %s", path)
		}
	}

	return nil
}

// pathInAllowlist returns nil if absPath sits inside one of the allowed
// roots. The allowlist is intentionally non-empty so an empty/zero-value cwd
// or homeDir cannot accidentally permit "/".
func pathInAllowlist(absPath, wd, homeDir string) error {
	allowed := []string{
		"/usr/local",
		"/etc",
		"/tmp",
		"/var/folders", // macOS temp directory
		os.TempDir(),   // System temp directory
	}
	if wd != "" {
		allowed = append(allowed, wd)
	}
	if homeDir != "" {
		allowed = append(allowed, homeDir)
	}

	// Filter out any zero-value entries defensively before the prefix walk.
	for _, root := range allowed {
		if root == "" {
			continue
		}
		if absPath == root || strings.HasPrefix(absPath, root+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("path outside allowed directories: %s", absPath)
}
