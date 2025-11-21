package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aixgo-dev/aixgo/pkg/security"
)

// Tool represents an MCP tool definition
type Tool struct {
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Handler            ToolHandler         `json:"-"`
	Schema             Schema              `json:"input_schema"`
	RequiredPermission security.Permission `json:"-"` // Required permission to execute this tool
	AllowedRoles       []string            `json:"-"` // Allowed roles to execute this tool
}

// ToolHandler is the function signature for tool handlers
type ToolHandler func(context.Context, Args) (any, error)

// Schema represents a JSON Schema for tool input validation
type Schema map[string]SchemaField

// SchemaField represents a single field in the schema
type SchemaField struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`   // Regex pattern for strings
	MaxLength   int      `json:"maxLength,omitempty"` // Max length for strings
	MinLength   int      `json:"minLength,omitempty"` // Min length for strings
	Enum        []any    `json:"enum,omitempty"`      // Allowed values
	Minimum     *float64 `json:"minimum,omitempty"`   // Minimum for numbers
	Maximum     *float64 `json:"maximum,omitempty"`   // Maximum for numbers
}

// Args provides type-safe access to tool arguments
type Args map[string]any

// String returns a string argument
func (a Args) String(key string) string {
	if v, ok := a[key].(string); ok {
		return v
	}
	return ""
}

// Int returns an integer argument
func (a Args) Int(key string) int {
	switch v := a[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

// Float returns a float argument
func (a Args) Float(key string) float64 {
	switch v := a[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	return 0.0
}

// Bool returns a boolean argument
func (a Args) Bool(key string) bool {
	if v, ok := a[key].(bool); ok {
		return v
	}
	return false
}

// Map returns a map argument
func (a Args) Map(key string) map[string]any {
	if v, ok := a[key].(map[string]any); ok {
		return v
	}
	return nil
}

// ValidatedString returns a validated string argument
func (a Args) ValidatedString(key string, validator security.ArgValidator) (string, error) {
	val, ok := a[key]
	if !ok {
		return "", fmt.Errorf("missing required arg: %s", key)
	}

	if validator != nil {
		if err := validator.Validate(val); err != nil {
			return "", fmt.Errorf("validation failed for %s: %w", key, err)
		}
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("arg %s: expected string, got %T", key, val)
	}

	return str, nil
}

// ValidatedInt returns a validated integer argument
func (a Args) ValidatedInt(key string, validator security.ArgValidator) (int, error) {
	val, ok := a[key]
	if !ok {
		return 0, fmt.Errorf("missing required arg: %s", key)
	}

	if validator != nil {
		if err := validator.Validate(val); err != nil {
			return 0, fmt.Errorf("validation failed for %s: %w", key, err)
		}
	}

	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), nil
		}
	}

	return 0, fmt.Errorf("arg %s: expected integer, got %T", key, val)
}

// ValidatedFloat returns a validated float argument
func (a Args) ValidatedFloat(key string, validator security.ArgValidator) (float64, error) {
	val, ok := a[key]
	if !ok {
		return 0, fmt.Errorf("missing required arg: %s", key)
	}

	if validator != nil {
		if err := validator.Validate(val); err != nil {
			return 0, fmt.Errorf("validation failed for %s: %w", key, err)
		}
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
	}

	return 0, fmt.Errorf("arg %s: expected number, got %T", key, val)
}

// ValidateSchema validates arguments against the tool's schema
func (s Schema) ValidateArgs(args Args) error {
	for fieldName, field := range s {
		val, exists := args[fieldName]

		// Check required fields
		if field.Required && !exists {
			return fmt.Errorf("missing required field: %s", fieldName)
		}

		if !exists {
			continue
		}

		// Type validation
		if err := validateFieldType(fieldName, val, field); err != nil {
			return err
		}
	}

	return nil
}

// validateFieldType validates a field against its schema definition
func validateFieldType(fieldName string, val any, field SchemaField) error {
	switch field.Type {
	case "string":
		str, ok := val.(string)
		if !ok {
			return fmt.Errorf("field %s: expected string, got %T", fieldName, val)
		}

		// Validate string constraints
		if field.MinLength > 0 && len(str) < field.MinLength {
			return fmt.Errorf("field %s: string too short (min %d)", fieldName, field.MinLength)
		}

		if field.MaxLength > 0 && len(str) > field.MaxLength {
			return fmt.Errorf("field %s: string too long (max %d)", fieldName, field.MaxLength)
		}

		// Validate enum
		if len(field.Enum) > 0 {
			found := false
			for _, allowed := range field.Enum {
				if allowedStr, ok := allowed.(string); ok && allowedStr == str {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("field %s: value not in allowed list", fieldName)
			}
		}

	case "number", "integer":
		var numVal float64
		switch v := val.(type) {
		case float64:
			numVal = v
		case int:
			numVal = float64(v)
		case int64:
			numVal = float64(v)
		default:
			return fmt.Errorf("field %s: expected number, got %T", fieldName, val)
		}

		// Validate numeric constraints
		if field.Minimum != nil && numVal < *field.Minimum {
			return fmt.Errorf("field %s: value %f below minimum %f", fieldName, numVal, *field.Minimum)
		}

		if field.Maximum != nil && numVal > *field.Maximum {
			return fmt.Errorf("field %s: value %f above maximum %f", fieldName, numVal, *field.Maximum)
		}

	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("field %s: expected boolean, got %T", fieldName, val)
		}

	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("field %s: expected object, got %T", fieldName, val)
		}

	case "array":
		// Check if value is a slice
		switch val.(type) {
		case []any:
			// Valid array type
		default:
			return fmt.Errorf("field %s: expected array, got %T", fieldName, val)
		}
	}

	return nil
}

// CallToolParams represents parameters for calling a tool
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// CallToolResult represents the result of a tool call
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents tool result content
type Content struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

// Transport defines the interface for MCP communication
type Transport interface {
	Send(ctx context.Context, method string, params any) (any, error)
	Close() error
}

// ServerConfig represents MCP server configuration
type ServerConfig struct {
	Name      string
	Transport string // "local" or "grpc"
	Address   string
	TLS       bool
	Auth      *AuthConfig
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type     string // "bearer", "oauth"
	Token    string
	TokenEnv string
}
