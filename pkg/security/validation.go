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

// ValidateFilePath validates a file path for safe file operations
// It prevents path traversal attacks and ensures the path doesn't contain dangerous patterns
// This is critical for TLS certificate loading and other file read operations
func ValidateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Clean the path to resolve any . or .. components
	cleaned := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path traversal detected in file path")
	}

	// Prevent null byte injection
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null byte detected in file path")
	}

	// Check for suspicious patterns that could be used for attacks
	suspicious := []string{
		"\n", "\r", // newlines
		"|", "&", ";", "`", // command injection chars
		"$", // variable expansion
	}
	for _, s := range suspicious {
		if strings.Contains(path, s) {
			return fmt.Errorf("suspicious character detected in file path")
		}
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

// Deployment Input Validation
// These validators prevent command injection in deployment scripts

var (
	// GCP Project IDs: alphanumeric, hyphens, max 30 chars
	// Must start with a letter, end with alphanumeric
	projectIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

	// GCP Regions: alphanumeric, hyphens (e.g., us-central1, europe-west1)
	regionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

	// Service names: alphanumeric, hyphens, max 63 chars
	// Must start with a letter, end with alphanumeric
	serviceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}[a-z0-9]$`)

	// Repository names: alphanumeric, hyphens, underscores, max 63 chars
	repositoryNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-_]{0,62}$`)

	// Image names: alphanumeric, hyphens, underscores, slashes for paths
	imageNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-_/]{0,255}$`)

	// Environment names: alphanumeric, hyphens, underscores
	environmentPattern = regexp.MustCompile(`^[a-z][a-z0-9-_]{0,31}$`)
)

// ValidateProjectID validates a GCP project ID
// Returns error if the project ID doesn't match GCP naming requirements
// This prevents command injection by ensuring only safe characters are used
func ValidateProjectID(projectID string) error {
	if projectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}
	if len(projectID) < 6 || len(projectID) > 30 {
		return fmt.Errorf("project ID must be between 6 and 30 characters")
	}
	if !projectIDPattern.MatchString(projectID) {
		return fmt.Errorf("invalid project ID format: must start with lowercase letter, contain only lowercase letters, digits, and hyphens")
	}
	return nil
}

// ValidateRegion validates a GCP region name
// This prevents command injection in region parameters
func ValidateRegion(region string) error {
	if region == "" {
		return fmt.Errorf("region cannot be empty")
	}
	if len(region) > 63 {
		return fmt.Errorf("region name too long (max 63 characters)")
	}
	if !regionPattern.MatchString(region) {
		return fmt.Errorf("invalid region format: must contain only lowercase letters, digits, and hyphens")
	}
	return nil
}

// ValidateServiceName validates a Cloud Run service name
// This prevents command injection in service name parameters
func ValidateServiceName(serviceName string) error {
	if serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if len(serviceName) < 2 || len(serviceName) > 63 {
		return fmt.Errorf("service name must be between 2 and 63 characters")
	}
	if !serviceNamePattern.MatchString(serviceName) {
		return fmt.Errorf("invalid service name format: must start with lowercase letter, contain only lowercase letters, digits, and hyphens, end with alphanumeric")
	}
	return nil
}

// ValidateRepositoryName validates an Artifact Registry repository name
// This prevents command injection in repository name parameters
func ValidateRepositoryName(repoName string) error {
	if repoName == "" {
		return fmt.Errorf("repository name cannot be empty")
	}
	if len(repoName) > 63 {
		return fmt.Errorf("repository name too long (max 63 characters)")
	}
	if !repositoryNamePattern.MatchString(repoName) {
		return fmt.Errorf("invalid repository name format: must contain only lowercase letters, digits, hyphens, and underscores")
	}
	return nil
}

// ValidateImageName validates a container image name
// This prevents command injection in image name parameters
func ValidateImageName(imageName string) error {
	if imageName == "" {
		return fmt.Errorf("image name cannot be empty")
	}
	if len(imageName) > 256 {
		return fmt.Errorf("image name too long (max 256 characters)")
	}
	if !imageNamePattern.MatchString(imageName) {
		return fmt.Errorf("invalid image name format: must contain only lowercase letters, digits, hyphens, underscores, and slashes")
	}
	return nil
}

// ValidateEnvironment validates an environment name
// This prevents command injection in environment parameters
func ValidateEnvironment(env string) error {
	if env == "" {
		return fmt.Errorf("environment cannot be empty")
	}
	if len(env) > 32 {
		return fmt.Errorf("environment name too long (max 32 characters)")
	}
	if !environmentPattern.MatchString(env) {
		return fmt.Errorf("invalid environment format: must start with lowercase letter, contain only lowercase letters, digits, hyphens, and underscores")
	}
	return nil
}

// ValidateDeploymentInputs validates all deployment-related inputs at once
// This provides a convenient function to validate all parameters before executing any commands
// Prevents command injection by ensuring all inputs match safe patterns
func ValidateDeploymentInputs(projectID, region, serviceName, repoName, imageName, environment string) error {
	if err := ValidateProjectID(projectID); err != nil {
		return fmt.Errorf("invalid project ID: %w", err)
	}
	if err := ValidateRegion(region); err != nil {
		return fmt.Errorf("invalid region: %w", err)
	}
	if err := ValidateServiceName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}
	if err := ValidateRepositoryName(repoName); err != nil {
		return fmt.Errorf("invalid repository name: %w", err)
	}
	if err := ValidateImageName(imageName); err != nil {
		return fmt.Errorf("invalid image name: %w", err)
	}
	if err := ValidateEnvironment(environment); err != nil {
		return fmt.Errorf("invalid environment: %w", err)
	}
	return nil
}
