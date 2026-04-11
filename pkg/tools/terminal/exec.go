// Package terminal provides terminal execution tools.
package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/tools"
)

// AllowedCommands is the list of commands allowed for execution.
// This can be extended based on security requirements.
var AllowedCommands = map[string]bool{
	// Build tools
	"go":     true,
	"make":   true,
	"npm":    true,
	"yarn":   true,
	"pnpm":   true,
	"cargo":  true,
	"gradle": true,
	"mvn":    true,
	"pip":    true,
	"poetry": true,

	// Version control
	"git": true,

	// File operations (read-only)
	"ls":       true,
	"cat":      true,
	"head":     true,
	"tail":     true,
	"wc":       true,
	"find":     true,
	"grep":     true,
	"which":    true,
	"file":     true,
	"basename": true,

	// System info
	"pwd":     true,
	"whoami":  true,
	"uname":   true,
	"date":    true,
	"env":     true,
	"echo":    true,
	"printf":  true,
	"dirname": true,

	// Process
	"ps": true,

	// Network (read-only)
	"curl":     true,
	"wget":     true,
	"ping":     true,
	"nslookup": true,
	"host":     true,

	// Utilities
	"jq":   true,
	"yq":   true,
	"sed":  true,
	"awk":  true,
	"sort": true,
	"uniq": true,
	"diff": true,
	"tree": true,

	// Docker (read-only by default)
	"docker": true,

	// Cloud CLIs (read-only operations encouraged)
	"gcloud":  true,
	"aws":     true,
	"az":      true,
	"kubectl": true,
}

// BlockedSubcommands lists dangerous subcommands to block.
var BlockedSubcommands = map[string][]string{
	"git":    {"push", "reset", "rebase", "force-push"},
	"rm":     {"-rf", "-r", "--recursive"},
	"docker": {"rm", "rmi", "prune", "system prune"},
}

func init() {
	tools.Register(ExecTool())
}

// ExecTool returns a tool for executing terminal commands.
func ExecTool() *tools.Tool {
	return &tools.Tool{
		Name:        "exec",
		Description: "Execute a terminal command. Only allowed commands can be run. Dangerous operations require confirmation.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The command to execute (e.g., 'go test ./...')"
				},
				"working_dir": {
					"type": "string",
					"description": "Working directory for command execution"
				},
				"timeout_seconds": {
					"type": "integer",
					"description": "Command timeout in seconds (default: 120, max: 600)"
				}
			},
			"required": ["command"]
		}`),
		Handler:              execHandler,
		RequiresConfirmation: true, // Always require confirmation
	}
}

// ExecResult represents the result of command execution.
type ExecResult struct {
	Command   string `json:"command"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Duration  string `json:"duration"`
	Truncated bool   `json:"truncated"`
}

func execHandler(ctx context.Context, args map[string]any) (any, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	workingDir := "."
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		workingDir = wd
	}

	timeout := 120 * time.Second
	if t, ok := args["timeout_seconds"].(float64); ok {
		timeout = time.Duration(t) * time.Second
		if timeout > 600*time.Second {
			timeout = 600 * time.Second
		}
	}

	// Validate command
	if err := ValidateCommand(command); err != nil {
		return nil, err
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute command
	start := time.Now()
	// Threat model for gosec G204 (subprocess launched with variable):
	//  (a) Caller authorization is enforced upstream by the agent policy
	//      layer and this tool's RequiresConfirmation=true gate, so only a
	//      confirmed, authorized agent action reaches this point.
	//  (b) The `command` string has already passed ValidateCommand above,
	//      which enforces a base-command allowlist (AllowedCommands), a
	//      per-command subcommand denylist (BlockedSubcommands), and a
	//      shell-operator denylist (&&, ||, ;, |, >, <, `, $( ) with a
	//      narrow safe-pipe / safe-redirect carve-out.
	//  (c) This tool is intended to run inside a sandbox/cgroup-isolated
	//      deployment (container, jailed user, or VM) per
	//      docs/SECURITY_BEST_PRACTICES.md; untrusted input must never
	//      reach an unsandboxed host.
	// #nosec G204 -- see threat model above; command is allowlist-validated.
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command timed out after %v", timeout)
		} else {
			return nil, fmt.Errorf("command execution failed: %w", err)
		}
	}

	// Truncate output if too long
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	truncated := false

	if len(stdoutStr) > 50000 {
		stdoutStr = stdoutStr[:50000] + "\n... (output truncated)"
		truncated = true
	}
	if len(stderrStr) > 10000 {
		stderrStr = stderrStr[:10000] + "\n... (error output truncated)"
		truncated = true
	}

	return &ExecResult{
		Command:   command,
		ExitCode:  exitCode,
		Stdout:    stdoutStr,
		Stderr:    stderrStr,
		Duration:  duration.String(),
		Truncated: truncated,
	}, nil
}

// ValidateCommand checks if a command is allowed to be executed.
// Exported so other packages can use this validation logic.
func ValidateCommand(command string) error {
	// Parse command to get the base command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	baseCmd := parts[0]

	// Check if command is in allowlist
	if !AllowedCommands[baseCmd] {
		return fmt.Errorf("command not allowed: %s (allowed: %s)",
			baseCmd, formatAllowedCommands())
	}

	// Check for blocked subcommands
	if blocked, ok := BlockedSubcommands[baseCmd]; ok {
		fullCmd := strings.ToLower(command)
		for _, sub := range blocked {
			if strings.Contains(fullCmd, strings.ToLower(sub)) {
				return fmt.Errorf("dangerous operation blocked: %s %s", baseCmd, sub)
			}
		}
	}

	// Block shell operators that could bypass restrictions
	dangerousOperators := []string{"&&", "||", ";", "|", ">", "<", "`", "$("}
	for _, op := range dangerousOperators {
		if strings.Contains(command, op) {
			// Allow some safe cases
			if op == "|" && IsSafePipe(command) {
				continue
			}
			if op == ">" && IsSafeRedirect(command) {
				continue
			}
			return fmt.Errorf("shell operator not allowed for security: %s", op)
		}
	}

	return nil
}

// IsSafePipe checks if a pipe operation is safe.
func IsSafePipe(command string) bool {
	// Allow piping to safe commands
	safePipeTargets := []string{"grep", "head", "tail", "wc", "sort", "uniq", "jq", "awk", "sed"}
	parts := strings.Split(command, "|")
	for i := 1; i < len(parts); i++ {
		target := strings.TrimSpace(parts[i])
		targetCmd := strings.Fields(target)[0]
		safe := false
		for _, s := range safePipeTargets {
			if targetCmd == s {
				safe = true
				break
			}
		}
		if !safe {
			return false
		}
	}
	return true
}

// IsSafeRedirect checks if a redirect is safe.
func IsSafeRedirect(command string) bool {
	// Only allow redirecting to /dev/null or specific safe patterns
	if strings.Contains(command, ">/dev/null") || strings.Contains(command, "> /dev/null") {
		return true
	}
	if strings.Contains(command, "2>&1") {
		return true
	}
	return false
}

// formatAllowedCommands formats the allowed commands for display.
func formatAllowedCommands() string {
	cmds := make([]string, 0, len(AllowedCommands))
	for cmd := range AllowedCommands {
		cmds = append(cmds, cmd)
	}
	// Return first few as example
	if len(cmds) > 10 {
		return strings.Join(cmds[:10], ", ") + ", ..."
	}
	return strings.Join(cmds, ", ")
}
