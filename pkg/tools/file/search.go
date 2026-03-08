package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aixgo-dev/aixgo/pkg/tools"
)

// GlobTool returns a tool for finding files matching a pattern.
func GlobTool() *tools.Tool {
	return &tools.Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern (e.g., '**/*.go', 'src/**/*.ts'). Returns list of matching file paths.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {
					"type": "string",
					"description": "Glob pattern to match files"
				},
				"path": {
					"type": "string",
					"description": "Base directory to search from (default: current directory)"
				}
			},
			"required": ["pattern"]
		}`),
		Handler:              globHandler,
		RequiresConfirmation: false,
	}
}

func globHandler(_ context.Context, args map[string]any) (any, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	basePath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		basePath = p
	}

	// Validate base path
	if err := ValidatePath(basePath); err != nil {
		return nil, err
	}

	var matches []string

	// Handle ** pattern (recursive)
	if strings.Contains(pattern, "**") {
		err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if info.IsDir() {
				return nil
			}

			// Convert pattern for matching
			matched, err := matchGlob(pattern, path)
			if err != nil {
				return nil
			}
			if matched {
				matches = append(matches, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk error: %w", err)
		}
	} else {
		// Simple glob
		fullPattern := filepath.Join(basePath, pattern)
		var err error
		matches, err = filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("glob error: %w", err)
		}
	}

	// Limit results
	if len(matches) > 100 {
		return map[string]any{
			"files":     matches[:100],
			"truncated": true,
			"total":     len(matches),
		}, nil
	}

	return map[string]any{
		"files": matches,
		"total": len(matches),
	}, nil
}

// matchGlob matches a file path against a glob pattern with ** support.
func matchGlob(pattern, path string) (bool, error) {
	// Convert glob pattern to regex
	regexPattern := globToRegex(pattern)
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(path), nil
}

// globToRegex converts a glob pattern to a regex pattern.
func globToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")

	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** matches any path
				result.WriteString(".*")
				i++ // Skip second *
				// Skip following /
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
				}
			} else {
				// * matches anything except /
				result.WriteString("[^/]*")
			}
		case '?':
			result.WriteString("[^/]")
		case '.', '+', '^', '$', '(', ')', '[', ']', '{', '}', '|', '\\':
			result.WriteString("\\")
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
		i++
	}

	result.WriteString("$")
	return result.String()
}

// GrepTool returns a tool for searching file contents.
func GrepTool() *tools.Tool {
	return &tools.Tool{
		Name:        "grep",
		Description: "Search for a pattern in files. Returns matching lines with file paths and line numbers.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {
					"type": "string",
					"description": "Regular expression pattern to search for"
				},
				"path": {
					"type": "string",
					"description": "File or directory to search in (default: current directory)"
				},
				"file_pattern": {
					"type": "string",
					"description": "Glob pattern to filter files (e.g., '*.go', '*.ts')"
				},
				"case_insensitive": {
					"type": "boolean",
					"description": "Case insensitive search (default: false)"
				}
			},
			"required": ["pattern"]
		}`),
		Handler:              grepHandler,
		RequiresConfirmation: false,
	}
}

// GrepMatch represents a grep match result.
type GrepMatch struct {
	File       string `json:"file"`
	LineNumber int    `json:"line_number"`
	Line       string `json:"line"`
}

func grepHandler(_ context.Context, args map[string]any) (any, error) {
	pattern, ok := args["pattern"].(string)
	if !ok || pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	basePath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		basePath = p
	}

	filePattern := ""
	if p, ok := args["file_pattern"].(string); ok {
		filePattern = p
	}

	caseInsensitive := false
	if ci, ok := args["case_insensitive"].(bool); ok {
		caseInsensitive = ci
	}

	// Validate base path
	if err := ValidatePath(basePath); err != nil {
		return nil, err
	}

	// Compile regex
	if caseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var matches []GrepMatch

	// Determine if searching a file or directory
	info, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}

	if info.IsDir() {
		// Walk directory
		err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Skip hidden directories
				if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}

			// Apply file pattern filter
			if filePattern != "" {
				matched, err := filepath.Match(filePattern, info.Name())
				if err != nil || !matched {
					return nil
				}
			}

			fileMatches, err := grepFile(path, re)
			if err != nil {
				return nil // Skip files we can't read
			}
			matches = append(matches, fileMatches...)

			// Limit total matches
			if len(matches) > 500 {
				return fmt.Errorf("too many matches")
			}
			return nil
		})
		if err != nil && err.Error() != "too many matches" {
			return nil, err
		}
	} else {
		// Search single file
		matches, err = grepFile(basePath, re)
		if err != nil {
			return nil, err
		}
	}

	truncated := false
	if len(matches) > 100 {
		matches = matches[:100]
		truncated = true
	}

	return map[string]any{
		"matches":   matches,
		"total":     len(matches),
		"truncated": truncated,
	}, nil
}

// grepFile searches a single file for matches.
func grepFile(path string, re *regexp.Regexp) ([]GrepMatch, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []GrepMatch
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, GrepMatch{
				File:       path,
				LineNumber: lineNum,
				Line:       truncateLine(line, 200),
			})
		}
	}

	return matches, scanner.Err()
}

// truncateLine truncates a line to max length.
func truncateLine(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
