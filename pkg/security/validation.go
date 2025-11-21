package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ArgValidator defines an interface for validating arguments
type ArgValidator interface {
	Validate(value any) error
}

// StringValidator validates string arguments with various constraints
type StringValidator struct {
	Pattern              *regexp.Regexp
	MaxLength            int
	MinLength            int
	AllowedVals          []string
	DisallowNullBytes    bool
	DisallowControlChars bool
	CheckSQLInjection    bool
}

// Validate checks if the value meets all string validation constraints
func (v *StringValidator) Validate(value any) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}

	if v.MinLength > 0 && len(str) < v.MinLength {
		return fmt.Errorf("string too short: minimum %d characters", v.MinLength)
	}

	if v.MaxLength > 0 && len(str) > v.MaxLength {
		return fmt.Errorf("string exceeds max length %d", v.MaxLength)
	}

	if v.DisallowNullBytes && strings.Contains(str, "\x00") {
		return fmt.Errorf("string contains null bytes")
	}

	if v.DisallowControlChars {
		for _, r := range str {
			if r < 32 && r != '\n' && r != '\t' && r != '\r' {
				return fmt.Errorf("string contains control characters")
			}
		}
	}

	if v.CheckSQLInjection {
		if err := checkSQLInjection(str); err != nil {
			return err
		}
	}

	if v.Pattern != nil && !v.Pattern.MatchString(str) {
		return fmt.Errorf("string does not match required pattern")
	}

	if len(v.AllowedVals) > 0 {
		for _, allowed := range v.AllowedVals {
			if str == allowed {
				return nil
			}
		}
		return fmt.Errorf("string not in allowlist")
	}

	return nil
}

// IntValidator validates integer arguments
type IntValidator struct {
	Min *int
	Max *int
}

// Validate checks if the value is an integer within specified bounds
func (v *IntValidator) Validate(value any) error {
	var intVal int

	switch val := value.(type) {
	case int:
		intVal = val
	case int64:
		intVal = int(val)
	case float64:
		intVal = int(val)
	default:
		return fmt.Errorf("expected integer, got %T", value)
	}

	if v.Min != nil && intVal < *v.Min {
		return fmt.Errorf("integer %d is less than minimum %d", intVal, *v.Min)
	}

	if v.Max != nil && intVal > *v.Max {
		return fmt.Errorf("integer %d exceeds maximum %d", intVal, *v.Max)
	}

	return nil
}

// FloatValidator validates floating-point arguments
type FloatValidator struct {
	Min *float64
	Max *float64
}

// Validate checks if the value is a float within specified bounds
func (v *FloatValidator) Validate(value any) error {
	var floatVal float64

	switch val := value.(type) {
	case float64:
		floatVal = val
	case float32:
		floatVal = float64(val)
	case int:
		floatVal = float64(val)
	case int64:
		floatVal = float64(val)
	default:
		return fmt.Errorf("expected number, got %T", value)
	}

	if v.Min != nil && floatVal < *v.Min {
		return fmt.Errorf("number %f is less than minimum %f", floatVal, *v.Min)
	}

	if v.Max != nil && floatVal > *v.Max {
		return fmt.Errorf("number %f exceeds maximum %f", floatVal, *v.Max)
	}

	return nil
}

// SanitizeFilePath prevents path traversal attacks
func SanitizeFilePath(path string, baseDir string) (string, error) {
	// Clean the path
	cleaned := filepath.Clean(path)

	// Prevent path traversal
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path traversal detected")
	}

	// If baseDir is provided, ensure the path is within it
	if baseDir != "" {
		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			return "", fmt.Errorf("invalid base directory: %w", err)
		}

		var absPath string
		if filepath.IsAbs(cleaned) {
			absPath = cleaned
		} else {
			absPath = filepath.Join(absBase, cleaned)
		}

		absPath = filepath.Clean(absPath)

		// Ensure the resolved path is within baseDir
		if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
			return "", fmt.Errorf("path is outside allowed directory")
		}

		return absPath, nil
	}

	return cleaned, nil
}

// SanitizeString removes potentially dangerous characters from strings
func SanitizeString(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Remove control characters except newline, tab, and carriage return
	var cleaned strings.Builder
	for _, r := range input {
		if r >= 32 || r == '\n' || r == '\t' || r == '\r' {
			cleaned.WriteRune(r)
		}
	}

	return cleaned.String()
}

// ValidateToolName checks if a tool name is valid and safe
func ValidateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if len(name) > 100 {
		return fmt.Errorf("tool name too long")
	}

	// Tool names should only contain alphanumeric, underscore, hyphen, and colon (for namespacing)
	validName := regexp.MustCompile(`^[a-zA-Z0-9_:-]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid tool name: must contain only alphanumeric, underscore, hyphen, and colon")
	}

	return nil
}

// ValidateJSONObject validates that a value is a valid JSON object
func ValidateJSONObject(value any) error {
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}

	switch value.(type) {
	case map[string]any:
		return nil
	default:
		return fmt.Errorf("expected JSON object, got %T", value)
	}
}

// checkSQLInjection checks for common SQL injection patterns
func checkSQLInjection(input string) error {
	// Convert to lowercase for case-insensitive matching
	lower := strings.ToLower(input)

	// Common SQL injection patterns
	sqlPatterns := []string{
		"' or '1'='1",
		"' or 1=1",
		"\" or \"1\"=\"1",
		"\" or 1=1",
		"'; drop",
		"\"; drop",
		"' union",
		"\" union",
		"'--",
		"\"--",
		"';--",
		"\";--",
	}

	for _, pattern := range sqlPatterns {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("potential SQL injection detected")
		}
	}

	// Check for SQL keywords after quotes (common injection pattern)
	sqlKeywords := []string{"select", "insert", "update", "delete", "drop", "union", "exec", "execute"}
	if strings.Contains(lower, "'") {
		quoteIndex := strings.Index(lower, "'")
		remaining := lower[quoteIndex:]
		for _, kw := range sqlKeywords {
			if strings.Contains(remaining, kw) {
				return fmt.Errorf("potential SQL injection detected")
			}
		}
	}

	return nil
}
