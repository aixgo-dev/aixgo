package security

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// YAMLLimits defines security limits for YAML parsing
type YAMLLimits struct {
	MaxFileSize  int64 // Maximum file size in bytes (default: 10MB)
	MaxDepth     int   // Maximum nesting depth (default: 20)
	MaxNodes     int   // Maximum number of nodes (default: 10000)
	MaxKeyLength int   // Maximum key length in bytes (default: 1024)
	MaxValueSize int64 // Maximum value size in bytes (default: 1MB)
}

// DefaultYAMLLimits returns secure default limits for YAML parsing
func DefaultYAMLLimits() YAMLLimits {
	return YAMLLimits{
		MaxFileSize:  10 * 1024 * 1024, // 10MB
		MaxDepth:     20,
		MaxNodes:     10000,
		MaxKeyLength: 1024,
		MaxValueSize: 1024 * 1024, // 1MB
	}
}

// SafeYAMLParser provides secure YAML parsing with resource limits
type SafeYAMLParser struct {
	limits YAMLLimits
}

// NewSafeYAMLParser creates a new YAML parser with security limits
func NewSafeYAMLParser(limits YAMLLimits) *SafeYAMLParser {
	return &SafeYAMLParser{limits: limits}
}

// UnmarshalYAML safely unmarshals YAML data with security limits
func (p *SafeYAMLParser) UnmarshalYAML(data []byte, v any) error {
	// Check file size
	if int64(len(data)) > p.limits.MaxFileSize {
		return fmt.Errorf("YAML file size %d bytes exceeds maximum %d bytes", len(data), p.limits.MaxFileSize)
	}

	// Parse and validate structure
	var rootNode yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&rootNode); err != nil {
		return fmt.Errorf("YAML parse error: %w", err)
	}

	// Validate depth and node count
	validator := &yamlValidator{
		limits:    p.limits,
		nodeCount: 0,
	}
	if err := validator.validateNode(&rootNode, 0); err != nil {
		return err
	}

	// If validation passes, unmarshal into target structure
	return yaml.Unmarshal(data, v)
}

// UnmarshalYAMLFromReader safely unmarshals YAML from a reader with size limits
func (p *SafeYAMLParser) UnmarshalYAMLFromReader(r io.Reader, v any) error {
	// Read with size limit
	limitedReader := io.LimitedReader{
		R: r,
		N: p.limits.MaxFileSize + 1, // Read one extra byte to detect overflow
	}

	data, err := io.ReadAll(&limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read YAML: %w", err)
	}

	if int64(len(data)) > p.limits.MaxFileSize {
		return fmt.Errorf("YAML input exceeds maximum size %d bytes", p.limits.MaxFileSize)
	}

	return p.UnmarshalYAML(data, v)
}

// yamlValidator validates YAML structure against security limits
type yamlValidator struct {
	limits    YAMLLimits
	nodeCount int
}

// validateNode recursively validates a YAML node
func (v *yamlValidator) validateNode(node *yaml.Node, depth int) error {
	// Check depth limit
	if depth > v.limits.MaxDepth {
		return fmt.Errorf("YAML nesting depth %d exceeds maximum %d", depth, v.limits.MaxDepth)
	}

	// Check node count limit
	v.nodeCount++
	if v.nodeCount > v.limits.MaxNodes {
		return fmt.Errorf("YAML node count %d exceeds maximum %d", v.nodeCount, v.limits.MaxNodes)
	}

	// Validate based on node kind
	switch node.Kind {
	case yaml.DocumentNode:
		// Document node - validate children
		for _, child := range node.Content {
			if err := v.validateNode(child, depth); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		// Mapping (object) - validate keys and values
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("invalid YAML mapping: odd number of elements")
		}

		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			// Validate key length
			if len(keyNode.Value) > v.limits.MaxKeyLength {
				return fmt.Errorf("YAML key length %d exceeds maximum %d", len(keyNode.Value), v.limits.MaxKeyLength)
			}

			// Recursively validate key and value
			if err := v.validateNode(keyNode, depth+1); err != nil {
				return err
			}
			if err := v.validateNode(valueNode, depth+1); err != nil {
				return err
			}
		}

	case yaml.SequenceNode:
		// Array - validate elements
		for _, child := range node.Content {
			if err := v.validateNode(child, depth+1); err != nil {
				return err
			}
		}

	case yaml.ScalarNode:
		// Scalar value - check value size
		if int64(len(node.Value)) > v.limits.MaxValueSize {
			return fmt.Errorf("YAML value size %d bytes exceeds maximum %d bytes", len(node.Value), v.limits.MaxValueSize)
		}

	case yaml.AliasNode:
		// Alias (anchor reference) - validate target
		if node.Alias != nil {
			if err := v.validateNode(node.Alias, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateYAMLFile validates a YAML file's structure without unmarshaling
func ValidateYAMLFile(data []byte, limits YAMLLimits) error {
	parser := NewSafeYAMLParser(limits)
	var dummy any
	return parser.UnmarshalYAML(data, &dummy)
}
