package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/aixgo-dev/aixgo/internal/llm/inference"
)

// JSONSchemaValidator validates JSON data against a JSON Schema
type JSONSchemaValidator struct {
	strictMode bool
}

// NewJSONSchemaValidator creates a new schema validator
func NewJSONSchemaValidator(strict bool) *JSONSchemaValidator {
	return &JSONSchemaValidator{
		strictMode: strict,
	}
}

// Schema represents a JSON Schema
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Enum        []any              `json:"enum,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty"`
	MinLength   *int               `json:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty"`
	Pattern     string             `json:"pattern,omitempty"`
	Description string             `json:"description,omitempty"`
	Default     any                `json:"default,omitempty"`
	OneOf       []*Schema          `json:"oneOf,omitempty"`
	AnyOf       []*Schema          `json:"anyOf,omitempty"`
	AllOf       []*Schema          `json:"allOf,omitempty"`
}

// ParseSchema parses a JSON Schema from raw JSON
func ParseSchema(raw json.RawMessage) (*Schema, error) {
	var schema Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	return &schema, nil
}

// ValidationResult contains the result of schema validation
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// Validate validates data against the schema
func (v *JSONSchemaValidator) Validate(schema *Schema, data any) *ValidationResult {
	result := &ValidationResult{Valid: true}
	v.validateValue(schema, data, "", result)
	return result
}

// validateValue recursively validates a value against a schema
func (v *JSONSchemaValidator) validateValue(schema *Schema, value any, path string, result *ValidationResult) {
	if schema == nil {
		return
	}

	// Type validation
	if schema.Type != "" {
		if !v.checkType(schema.Type, value) {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: expected type %s, got %T", pathOrRoot(path), schema.Type, value))
			return
		}
	}

	switch schema.Type {
	case "object":
		v.validateObject(schema, value, path, result)
	case "array":
		v.validateArray(schema, value, path, result)
	case "string":
		v.validateString(schema, value, path, result)
	case "number", "integer":
		v.validateNumber(schema, value, path, result)
	}

	// Enum validation
	if len(schema.Enum) > 0 {
		v.validateEnum(schema.Enum, value, path, result)
	}
}

// checkType checks if a value matches the expected JSON Schema type
func (v *JSONSchemaValidator) checkType(schemaType string, value any) bool {
	if value == nil {
		return schemaType == "null"
	}

	switch schemaType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32:
			return true
		}
		return false
	case "integer":
		switch val := value.(type) {
		case int, int64, int32:
			return true
		case float64:
			return val == float64(int64(val))
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		rv := reflect.ValueOf(value)
		return rv.Kind() == reflect.Slice
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "null":
		return value == nil
	}
	return false
}

// validateObject validates an object against schema
func (v *JSONSchemaValidator) validateObject(schema *Schema, value any, path string, result *ValidationResult) {
	obj, ok := value.(map[string]any)
	if !ok {
		return
	}

	// Check required fields
	for _, reqField := range schema.Required {
		if _, exists := obj[reqField]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: missing required field '%s'", pathOrRoot(path), reqField))
		}
	}

	// Validate properties
	for propName, propSchema := range schema.Properties {
		propPath := joinPath(path, propName)
		if propValue, exists := obj[propName]; exists {
			v.validateValue(propSchema, propValue, propPath, result)
		}
	}

	// In strict mode, reject unknown properties
	if v.strictMode {
		for propName := range obj {
			if _, defined := schema.Properties[propName]; !defined {
				result.Valid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: unknown property '%s'", pathOrRoot(path), propName))
			}
		}
	}
}

// validateArray validates an array against schema
func (v *JSONSchemaValidator) validateArray(schema *Schema, value any, path string, result *ValidationResult) {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return
	}

	if schema.Items != nil {
		for i := 0; i < rv.Len(); i++ {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			v.validateValue(schema.Items, rv.Index(i).Interface(), itemPath, result)
		}
	}
}

// validateString validates a string against schema constraints
func (v *JSONSchemaValidator) validateString(schema *Schema, value any, path string, result *ValidationResult) {
	str, ok := value.(string)
	if !ok {
		return
	}

	if schema.MinLength != nil && len(str) < *schema.MinLength {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s: string length %d is less than minimum %d", pathOrRoot(path), len(str), *schema.MinLength))
	}

	if schema.MaxLength != nil && len(str) > *schema.MaxLength {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s: string length %d is greater than maximum %d", pathOrRoot(path), len(str), *schema.MaxLength))
	}
}

// validateNumber validates a number against schema constraints
func (v *JSONSchemaValidator) validateNumber(schema *Schema, value any, path string, result *ValidationResult) {
	var num float64
	switch val := value.(type) {
	case float64:
		num = val
	case int:
		num = float64(val)
	case int64:
		num = float64(val)
	default:
		return
	}

	if schema.Minimum != nil && num < *schema.Minimum {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s: value %v is less than minimum %v", pathOrRoot(path), num, *schema.Minimum))
	}

	if schema.Maximum != nil && num > *schema.Maximum {
		result.Valid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("%s: value %v is greater than maximum %v", pathOrRoot(path), num, *schema.Maximum))
	}
}

// validateEnum validates a value against enum options
func (v *JSONSchemaValidator) validateEnum(enum []any, value any, path string, result *ValidationResult) {
	for _, option := range enum {
		if reflect.DeepEqual(option, value) {
			return
		}
	}
	result.Valid = false
	result.Errors = append(result.Errors,
		fmt.Sprintf("%s: value %v is not one of allowed values %v", pathOrRoot(path), value, enum))
}

// Helper functions
func pathOrRoot(path string) string {
	if path == "" {
		return "root"
	}
	return path
}

func joinPath(base, field string) string {
	if base == "" {
		return field
	}
	return base + "." + field
}

// StructuredOutputHandler handles structured output generation with schema validation
type StructuredOutputHandler struct {
	inference inference.InferenceService
	validator *JSONSchemaValidator
}

// NewStructuredOutputHandler creates a new structured output handler
func NewStructuredOutputHandler(inf inference.InferenceService, strict bool) *StructuredOutputHandler {
	return &StructuredOutputHandler{
		inference: inf,
		validator: NewJSONSchemaValidator(strict),
	}
}

// Generate generates a structured output with schema validation
func (h *StructuredOutputHandler) Generate(ctx context.Context, req StructuredRequest, model string) (*StructuredResponse, error) {
	// Parse the response schema
	schema, err := ParseSchema(req.ResponseSchema)
	if err != nil {
		return nil, fmt.Errorf("invalid response schema: %w", err)
	}

	// Build prompt with schema instructions
	prompt := h.buildStructuredPrompt(req.Messages, schema)

	// Generate response
	resp, err := h.inference.Generate(ctx, inference.GenerateRequest{
		Model:       model,
		Prompt:      prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	// Extract JSON from response
	jsonStr := extractJSON(resp.Text)
	if jsonStr == "" {
		return nil, fmt.Errorf("no valid JSON found in response")
	}

	// Parse JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("parse JSON response: %w", err)
	}

	// Validate against schema
	result := h.validator.Validate(schema, data)
	if !result.Valid {
		return nil, fmt.Errorf("schema validation failed: %s", strings.Join(result.Errors, "; "))
	}

	return &StructuredResponse{
		Data: json.RawMessage(jsonStr),
		CompletionResponse: CompletionResponse{
			Content:      jsonStr,
			FinishReason: resp.FinishReason,
			Usage: Usage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
			},
		},
	}, nil
}

// buildStructuredPrompt builds a prompt that instructs the model to return structured JSON
func (h *StructuredOutputHandler) buildStructuredPrompt(messages []Message, schema *Schema) string {
	var sb strings.Builder

	sb.WriteString("You are an AI assistant that responds only with valid JSON.\n\n")
	sb.WriteString("Your response MUST be a valid JSON object that matches this schema:\n")

	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
	sb.WriteString("```json\n")
	sb.Write(schemaJSON)
	sb.WriteString("\n```\n\n")

	sb.WriteString("IMPORTANT: Respond ONLY with the JSON object. No explanations, no markdown code blocks, just the raw JSON.\n\n")

	// Add messages
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sb.WriteString(fmt.Sprintf("System: %s\n\n", msg.Content))
		case "user":
			sb.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
		}
	}

	sb.WriteString("JSON Response:\n")

	return sb.String()
}

// extractJSON extracts a JSON object from text
func extractJSON(text string) string {
	// Try to find JSON object
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	inString := false
	escape := false

	for i := start; i < len(text); i++ {
		c := text[i]

		if escape {
			escape = false
			continue
		}

		switch c {
		case '\\':
			if inString {
				escape = true
			}
		case '"':
			inString = !inString
		case '{':
			if !inString {
				depth++
			}
		case '}':
			if !inString {
				depth--
				if depth == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}

// SchemaFromStruct generates a JSON Schema from a Go struct type
func SchemaFromStruct(t reflect.Type) *Schema {
	return generateSchemaFromType(t)
}

func generateSchemaFromType(t reflect.Type) *Schema {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := &Schema{}

	switch t.Kind() {
	case reflect.String:
		schema.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema.Type = "integer"
	case reflect.Float32, reflect.Float64:
		schema.Type = "number"
	case reflect.Bool:
		schema.Type = "boolean"
	case reflect.Slice, reflect.Array:
		schema.Type = "array"
		schema.Items = generateSchemaFromType(t.Elem())
	case reflect.Struct:
		schema.Type = "object"
		schema.Properties = make(map[string]*Schema)
		schema.Required = make([]string, 0)

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}

			// Get JSON field name
			jsonTag := field.Tag.Get("json")
			fieldName := field.Name
			if jsonTag != "" && jsonTag != "-" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" {
					fieldName = parts[0]
				}
			}

			propSchema := generateSchemaFromType(field.Type)

			// Add description from tag
			if desc := field.Tag.Get("description"); desc != "" {
				propSchema.Description = desc
			}

			schema.Properties[fieldName] = propSchema

			// Check for required tag
			validateTag := field.Tag.Get("validate")
			if strings.Contains(validateTag, "required") {
				schema.Required = append(schema.Required, fieldName)
			}
		}
	case reflect.Map:
		schema.Type = "object"
	}

	return schema
}
