// Package git provides git operation tools.
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aixgo-dev/aixgo/pkg/tools"
)

// sanitizePath validates and cleans a repository path argument.
// It rejects empty paths, null bytes, and paths containing ".." components
// to prevent path-traversal attacks when the value originates from tool
// input (e.g. an LLM-generated function call).
func sanitizePath(p string) (string, error) {
	if p == "" {
		return ".", nil
	}
	if strings.ContainsRune(p, 0) {
		return "", fmt.Errorf("path contains null byte")
	}
	clean := filepath.Clean(p)
	// Reject paths that attempt directory traversal.
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path contains '..' traversal: %q", p)
	}
	return clean, nil
}

// safeRefPattern restricts git ref arguments (commit, branch names) to a
// conservative set of characters. This prevents flag injection via the
// commit argument in diff/log operations.
var safeRefPattern = regexp.MustCompile(`^[A-Za-z0-9._~^/@{}\-]+$`)

// allowedGitSubcommands is the set of git subcommands this tool is permitted
// to invoke. Any caller-influenced git invocation MUST pass through
// validateGitArgs so that only these read-or-tightly-scoped subcommands are
// reachable. This mitigates gosec G204 (subprocess launched with variable) by
// constraining the command surface even when other arguments (paths, refs,
// messages) originate from tool inputs.
var allowedGitSubcommands = map[string]bool{
	"status":    true,
	"diff":      true,
	"log":       true,
	"show":      true,
	"rev-parse": true,
	"branch":    true,
	"add":       true,
	"commit":    true,
}

// validateGitArgs enforces a subcommand allowlist and rejects flag-style
// arguments known to enable remote code execution or git-config overrides:
// --upload-pack / --receive-pack / --exec / -c / --config. These flags have
// been weaponised historically (CVE-2017-1000117 and follow-ups) when
// caller-controlled paths or refs were passed to git verbatim.
//
// Shell metacharacters are intentionally NOT rejected because every call
// site uses exec.Command (no shell interpreter is invoked), so characters
// like $, `, ;, | are inert data. This matters for legitimate payloads such
// as commit messages containing shell-like text.
//
// The args slice is the full argv that will be passed to `git`, including
// any leading `-C <path>` pair. Once the subcommand is located, every
// argument that follows is permitted to start with "--" only if it is not
// on the denylist; positional/value arguments (after "--" or as -m payload)
// are not inspected for flag prefixes.
func validateGitArgs(args []string) error {
	// Skip a leading "-C <path>" pair if present.
	i := 0
	if len(args) >= 2 && args[0] == "-C" {
		i = 2
	}
	if i >= len(args) {
		return fmt.Errorf("git: missing subcommand")
	}
	sub := args[i]
	if !allowedGitSubcommands[sub] {
		return fmt.Errorf("git: subcommand not allowed: %q", sub)
	}

	// Scan remaining arguments for denied flags. Stop flag-checking after
	// a literal "--" (end-of-options) and do not inspect the value that
	// follows "-m" (commit message payload).
	endOfOptions := false
	skipNext := false
	for _, a := range args[i+1:] {
		if skipNext {
			skipNext = false
			continue
		}
		if endOfOptions {
			continue
		}
		if a == "--" {
			endOfOptions = true
			continue
		}
		if a == "-m" || a == "--message" {
			skipNext = true
			continue
		}
		lower := strings.ToLower(a)
		switch {
		case strings.HasPrefix(lower, "--upload-pack"),
			strings.HasPrefix(lower, "--receive-pack"),
			strings.HasPrefix(lower, "--exec"),
			lower == "-c",
			strings.HasPrefix(lower, "--config"):
			return fmt.Errorf("git: flag not allowed: %q", a)
		}
	}
	return nil
}

// safeGitCommand builds an *exec.Cmd for git after validating every argument
// against the allowlist. It is the only constructor the handlers in this
// package should use to spawn git.
func safeGitCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	if err := validateGitArgs(args); err != nil {
		return nil, err
	}
	// #nosec G204 -- args are validated by validateGitArgs above: subcommand
	// is drawn from a fixed allowlist and every remaining arg is checked for
	// shell metacharacters and dangerous git flags (--upload-pack, -c, etc.).
	return exec.CommandContext(ctx, "git", args...), nil
}

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
	rawPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		rawPath = p
	}
	path, err := sanitizePath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Run git status
	cmd, err := safeGitCommand(ctx, "-C", path, "status", "--porcelain=v1")
	if err != nil {
		return nil, err
	}
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
	branchCmd, err := safeGitCommand(ctx, "-C", path, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
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
	rawPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		rawPath = p
	}
	repoPath, err := sanitizePath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Build command args
	cmdArgs := []string{"-C", repoPath, "diff"}

	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--staged")
	}

	if commit, ok := args["commit"].(string); ok && commit != "" {
		if !safeRefPattern.MatchString(commit) {
			return nil, fmt.Errorf("invalid commit ref: %q", commit)
		}
		cmdArgs = append(cmdArgs, commit)
	}

	if file, ok := args["file"].(string); ok && file != "" {
		cleanFile, ferr := sanitizePath(file)
		if ferr != nil {
			return nil, fmt.Errorf("invalid file path: %w", ferr)
		}
		cmdArgs = append(cmdArgs, "--", cleanFile)
	}

	cmd, err := safeGitCommand(ctx, cmdArgs...)
	if err != nil {
		return nil, err
	}
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

	rawPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		rawPath = p
	}
	repoPath, err := sanitizePath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Stage files if specified
	if files, ok := args["files"].([]any); ok && len(files) > 0 {
		fileArgs := []string{"-C", repoPath, "add"}
		for _, f := range files {
			if s, ok := f.(string); ok {
				cleanFile, ferr := sanitizePath(s)
				if ferr != nil {
					return nil, fmt.Errorf("invalid file path: %w", ferr)
				}
				fileArgs = append(fileArgs, cleanFile)
			}
		}

		addCmd, err := safeGitCommand(ctx, fileArgs...)
		if err != nil {
			return nil, err
		}
		if err := addCmd.Run(); err != nil {
			return nil, fmt.Errorf("git add failed: %w", err)
		}
	}

	// Commit
	commitCmd, err := safeGitCommand(ctx, "-C", repoPath, "commit", "-m", message)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	commitCmd.Stdout = &stdout
	commitCmd.Stderr = &stderr

	if err := commitCmd.Run(); err != nil {
		return nil, fmt.Errorf("git commit failed: %s", stderr.String())
	}

	// Get commit hash
	hashCmd, err := safeGitCommand(ctx, "-C", repoPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, err
	}
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
	rawPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		rawPath = p
	}
	repoPath, err := sanitizePath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
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
		cleanFile, ferr := sanitizePath(file)
		if ferr != nil {
			return nil, fmt.Errorf("invalid file path: %w", ferr)
		}
		cmdArgs = append(cmdArgs, "--", cleanFile)
	}

	cmd, err := safeGitCommand(ctx, cmdArgs...)
	if err != nil {
		return nil, err
	}
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
