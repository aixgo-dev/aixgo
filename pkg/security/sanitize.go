package security

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// ErrorCode represents a standardized error code for API responses
type ErrorCode string

const (
	ErrCodeInternal      ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized  ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden     ErrorCode = "FORBIDDEN"
	ErrCodeRateLimit     ErrorCode = "RATE_LIMIT"
	ErrCodeTimeout       ErrorCode = "TIMEOUT"
	ErrCodeToolNotFound  ErrorCode = "TOOL_NOT_FOUND"
	ErrCodeToolExecution ErrorCode = "TOOL_EXECUTION_ERROR"
	ErrCodeValidation    ErrorCode = "VALIDATION_ERROR"
)

// SecureError represents a sanitized error safe to return to clients
type SecureError struct {
	Code    ErrorCode              `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *SecureError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// SanitizeError converts an internal error to a safe error for clients
func SanitizeError(err error, debugMode bool) *SecureError {
	if err == nil {
		return nil
	}

	// Log full error server-side (without sensitive data)
	log.Printf("Internal error: %v", sanitizeLogMessage(err.Error()))

	// Create sanitized error for client
	secureErr := &SecureError{
		Code:    ErrCodeInternal,
		Message: "An internal error occurred",
	}

	// In debug mode, include sanitized error details
	if debugMode {
		secureErr.Details = map[string]interface{}{
			"error": sanitizeErrorMessage(err.Error()),
		}
	}

	return secureErr
}

// SanitizeErrorWithCode creates a secure error with a specific error code
func SanitizeErrorWithCode(err error, code ErrorCode, message string, debugMode bool) *SecureError {
	if err == nil {
		return nil
	}

	// Log full error server-side
	log.Printf("Error [%s]: %v", code, sanitizeLogMessage(err.Error()))

	secureErr := &SecureError{
		Code:    code,
		Message: message,
	}

	if debugMode {
		secureErr.Details = map[string]interface{}{
			"error": sanitizeErrorMessage(err.Error()),
		}
	}

	return secureErr
}

// sanitizeErrorMessage removes sensitive information from error messages
func sanitizeErrorMessage(msg string) string {
	// Remove file paths
	msg = removeFilePaths(msg)

	// Remove IP addresses
	msg = removeIPAddresses(msg)

	// Remove potential secrets (patterns like API keys)
	msg = removeSecretPatterns(msg)

	// Remove stack traces
	msg = removeStackTraces(msg)

	return msg
}

// sanitizeLogMessage sanitizes messages for logging (less aggressive than client-facing)
func sanitizeLogMessage(msg string) string {
	// Only remove obvious secrets from logs
	return removeSecretPatterns(msg)
}

// removeFilePaths removes file system paths from error messages
func removeFilePaths(msg string) string {
	// Remove Unix-style paths
	msg = strings.ReplaceAll(msg, "/Users/", "/home/")
	msg = strings.ReplaceAll(msg, "/home/", "[PATH]/")
	msg = strings.ReplaceAll(msg, "/var/", "[PATH]/")
	msg = strings.ReplaceAll(msg, "/etc/", "[PATH]/")
	msg = strings.ReplaceAll(msg, "/opt/", "[PATH]/")
	msg = strings.ReplaceAll(msg, "/tmp/", "[PATH]/")

	// Remove Windows-style paths
	for _, drive := range []string{"C:", "D:", "E:", "F:"} {
		if strings.Contains(msg, drive) {
			msg = strings.ReplaceAll(msg, drive+"\\", "[PATH]\\")
		}
	}

	return msg
}

// removeIPAddresses removes IP addresses from messages
func removeIPAddresses(msg string) string {
	// Simple IP address pattern removal
	parts := strings.Fields(msg)
	var cleaned []string

	for _, part := range parts {
		// Check if it looks like an IP address
		if strings.Count(part, ".") == 3 || strings.Count(part, ":") > 2 {
			// Simple heuristic: if it has 3 dots or multiple colons, might be an IP
			octets := strings.Split(part, ".")
			if len(octets) == 4 {
				cleaned = append(cleaned, "[IP_ADDRESS]")
				continue
			}
		}
		cleaned = append(cleaned, part)
	}

	return strings.Join(cleaned, " ")
}

// removeSecretPatterns removes patterns that look like API keys or secrets
func removeSecretPatterns(msg string) string {
	// Remove things that look like API keys
	patterns := []struct {
		prefix string
		length int
	}{
		{"sk-", 32},
		{"xai-", 32},
		{"api_key=", 20},
		{"apiKey=", 20},
		{"token=", 20},
		{"Bearer ", 20},
	}

	for _, pattern := range patterns {
		idx := strings.Index(msg, pattern.prefix)
		if idx != -1 {
			endIdx := idx + len(pattern.prefix) + pattern.length
			if endIdx > len(msg) {
				endIdx = len(msg)
			}
			msg = msg[:idx] + "[REDACTED]" + msg[endIdx:]
		}
	}

	return msg
}

// removeStackTraces removes Go stack traces from error messages
func removeStackTraces(msg string) string {
	// Remove goroutine stack traces (goroutine X [status]:)
	goroutinePattern := regexp.MustCompile(`goroutine \d+ \[[^\]]+\]:[\s\S]*?(?:\n\n|\z)`)
	msg = goroutinePattern.ReplaceAllString(msg, "[STACK_TRACE_REMOVED]")

	// Remove file:line patterns common in stack traces (e.g., "file.go:123")
	fileLinePattern := regexp.MustCompile(`\S+\.go:\d+`)
	msg = fileLinePattern.ReplaceAllString(msg, "[FILE:LINE]")

	// Remove function signatures with memory addresses (e.g., "0x12345678")
	addrPattern := regexp.MustCompile(`0x[0-9a-fA-F]+`)
	msg = addrPattern.ReplaceAllString(msg, "[ADDR]")

	// Remove panic messages
	panicPattern := regexp.MustCompile(`panic:.*`)
	msg = panicPattern.ReplaceAllString(msg, "panic: [DETAILS_REMOVED]")

	return msg
}

// MaskSecret masks a secret for logging purposes
func MaskSecret(secret string) string {
	if secret == "" {
		return ""
	}

	if len(secret) <= 8 {
		return "****"
	}

	return secret[:4] + "****" + secret[len(secret)-4:]
}

// IsValidAPIKeyFormat validates the format of an API key
func IsValidAPIKeyFormat(key string) bool {
	if len(key) < 16 {
		return false
	}

	// Check for common API key prefixes
	validPrefixes := []string{"sk-", "xai-", "hf_", "pk-", "Bearer "}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	// If no prefix, check if it's a long alphanumeric string
	if len(key) >= 32 {
		return true
	}

	return false
}
