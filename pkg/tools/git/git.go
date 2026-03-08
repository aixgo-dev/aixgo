// Package git provides git operation tools.
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aixgo-dev/aixgo/pkg/tools"
)

func init() {
	tools.Register(StatusTool())
	tools.Register(DiffTool())
	tools.Register(CommitTool())
	tools.Register(LogTool())
}

// StatusTool returns a tool for git status.
func StatusTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_status",
		Description: "Show the working tree status. Returns modified, staged, and untracked files.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Repository path (default: current directory)"
				}
			}
		}`),
		Handler:              statusHandler,
		RequiresConfirmation: false,
	}
}

func statusHandler(ctx context.Context, args map[string]any) (any, error) {
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	// Run git status
	cmd := exec.CommandContext(ctx, "git", "-C", path, "status", "--porcelain=v1")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	// Parse status output
	var staged, modified, untracked []string

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		file := strings.TrimSpace(line[3:])

		switch {
		case status[0] != ' ' && status[0] != '?':
			staged = append(staged, file)
		case status[1] == 'M' || status[1] == 'D':
			modified = append(modified, file)
		case status[0] == '?':
			untracked = append(untracked, file)
		}
	}

	// Get current branch
	branchCmd := exec.CommandContext(ctx, "git", "-C", path, "branch", "--show-current")
	branchOutput, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchOutput))

	return map[string]any{
		"branch":    branch,
		"staged":    staged,
		"modified":  modified,
		"untracked": untracked,
		"clean":     len(staged) == 0 && len(modified) == 0 && len(untracked) == 0,
	}, nil
}

// DiffTool returns a tool for git diff.
func DiffTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_diff",
		Description: "Show changes between commits, commit and working tree, etc.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Repository path (default: current directory)"
				},
				"file": {
					"type": "string",
					"description": "Specific file to diff (optional)"
				},
				"staged": {
					"type": "boolean",
					"description": "Show staged changes (default: false)"
				},
				"commit": {
					"type": "string",
					"description": "Commit to diff against (e.g., HEAD~1)"
				}
			}
		}`),
		Handler:              diffHandler,
		RequiresConfirmation: false,
	}
}

func diffHandler(ctx context.Context, args map[string]any) (any, error) {
	repoPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		repoPath = p
	}

	// Build command args
	cmdArgs := []string{"-C", repoPath, "diff"}

	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--staged")
	}

	if commit, ok := args["commit"].(string); ok && commit != "" {
		cmdArgs = append(cmdArgs, commit)
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		// Empty diff is not an error
		if len(output) == 0 {
			return map[string]any{
				"diff":  "",
				"empty": true,
			}, nil
		}
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	// Truncate very large diffs
	diff := string(output)
	truncated := false
	if len(diff) > 50000 {
		diff = diff[:50000] + "\n... (truncated)"
		truncated = true
	}

	return map[string]any{
		"diff":      diff,
		"truncated": truncated,
	}, nil
}

// CommitTool returns a tool for git commit.
func CommitTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_commit",
		Description: "Record changes to the repository. Stages and commits specified files.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {
					"type": "string",
					"description": "Commit message"
				},
				"files": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Files to stage and commit (default: all staged files)"
				},
				"path": {
					"type": "string",
					"description": "Repository path (default: current directory)"
				}
			},
			"required": ["message"]
		}`),
		Handler:              commitHandler,
		RequiresConfirmation: true, // Always confirm commits
	}
}

func commitHandler(ctx context.Context, args map[string]any) (any, error) {
	message, ok := args["message"].(string)
	if !ok || message == "" {
		return nil, fmt.Errorf("commit message is required")
	}

	repoPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		repoPath = p
	}

	// Stage files if specified
	if files, ok := args["files"].([]any); ok && len(files) > 0 {
		fileArgs := []string{"-C", repoPath, "add"}
		for _, f := range files {
			if s, ok := f.(string); ok {
				fileArgs = append(fileArgs, s)
			}
		}

		addCmd := exec.CommandContext(ctx, "git", fileArgs...)
		if err := addCmd.Run(); err != nil {
			return nil, fmt.Errorf("git add failed: %w", err)
		}
	}

	// Commit
	commitCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "commit", "-m", message)
	var stdout, stderr bytes.Buffer
	commitCmd.Stdout = &stdout
	commitCmd.Stderr = &stderr

	if err := commitCmd.Run(); err != nil {
		return nil, fmt.Errorf("git commit failed: %s", stderr.String())
	}

	// Get commit hash
	hashCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--short", "HEAD")
	hashOutput, _ := hashCmd.Output()
	hash := strings.TrimSpace(string(hashOutput))

	return map[string]any{
		"success": true,
		"hash":    hash,
		"message": message,
	}, nil
}

// LogTool returns a tool for git log.
func LogTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_log",
		Description: "Show commit logs.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "Repository path (default: current directory)"
				},
				"count": {
					"type": "integer",
					"description": "Number of commits to show (default: 10)"
				},
				"file": {
					"type": "string",
					"description": "Show commits for a specific file"
				}
			}
		}`),
		Handler:              logHandler,
		RequiresConfirmation: false,
	}
}

func logHandler(ctx context.Context, args map[string]any) (any, error) {
	repoPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		repoPath = p
	}

	count := 10
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	cmdArgs := []string{"-C", repoPath, "log",
		fmt.Sprintf("-n%d", count),
		"--pretty=format:%H|%h|%an|%ae|%s|%ci",
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cmdArgs = append(cmdArgs, "--", file)
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []map[string]string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) >= 6 {
			commits = append(commits, map[string]string{
				"hash":       parts[0],
				"short_hash": parts[1],
				"author":     parts[2],
				"email":      parts[3],
				"message":    parts[4],
				"date":       parts[5],
			})
		}
	}

	return map[string]any{
		"commits": commits,
		"count":   len(commits),
	}, nil
}
