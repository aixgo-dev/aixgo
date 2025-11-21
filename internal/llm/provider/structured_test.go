package provider

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestJSONSchemaValidator_ValidateObject(t *testing.T) {
	validator := NewJSONSchemaValidator(false)

	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name:    "valid object",
			data:    map[string]any{"name": "John", "age": 30},
			wantErr: false,
		},
		{
			name:    "missing required field",
			data:    map[string]any{"age": 30},
			wantErr: true,
		},
		{
			name:    "wrong type",
			data:    map[string]any{"name": 123, "age": 30},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(schema, tt.data)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected validation error: %v", result.Errors)
			}
		})
	}
}

func TestJSONSchemaValidator_ValidateString(t *testing.T) {
	validator := NewJSONSchemaValidator(false)

	minLen := 3
	maxLen := 10
	schema := &Schema{
		Type:      "string",
		MinLength: &minLen,
		MaxLength: &maxLen,
	}

	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name:    "valid string",
			data:    "hello",
			wantErr: false,
		},
		{
			name:    "too short",
			data:    "hi",
			wantErr: true,
		},
		{
			name:    "too long",
			data:    "this is way too long",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(schema, tt.data)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected validation error: %v", result.Errors)
			}
		})
	}
}

func TestJSONSchemaValidator_ValidateNumber(t *testing.T) {
	validator := NewJSONSchemaValidator(false)

	min := 0.0
	max := 100.0
	schema := &Schema{
		Type:    "number",
		Minimum: &min,
		Maximum: &max,
	}

	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name:    "valid number",
			data:    50.5,
			wantErr: false,
		},
		{
			name:    "below minimum",
			data:    -1.0,
			wantErr: true,
		},
		{
			name:    "above maximum",
			data:    101.0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(schema, tt.data)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected validation error: %v", result.Errors)
			}
		})
	}
}

func TestJSONSchemaValidator_ValidateArray(t *testing.T) {
	validator := NewJSONSchemaValidator(false)

	schema := &Schema{
		Type: "array",
		Items: &Schema{
			Type: "string",
		},
	}

	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name:    "valid array",
			data:    []any{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "invalid item type",
			data:    []any{"a", 123, "c"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(schema, tt.data)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected validation error: %v", result.Errors)
			}
		})
	}
}

func TestJSONSchemaValidator_ValidateEnum(t *testing.T) {
	validator := NewJSONSchemaValidator(false)

	schema := &Schema{
		Type: "string",
		Enum: []any{"red", "green", "blue"},
	}

	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name:    "valid enum value",
			data:    "red",
			wantErr: false,
		},
		{
			name:    "invalid enum value",
			data:    "yellow",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(schema, tt.data)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected validation error: %v", result.Errors)
			}
		})
	}
}

func TestJSONSchemaValidator_StrictMode(t *testing.T) {
	validator := NewJSONSchemaValidator(true)

	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	data := map[string]any{
		"name":  "John",
		"extra": "field",
	}

	result := validator.Validate(schema, data)
	if result.Valid {
		t.Error("strict mode should reject unknown properties")
	}
}

func TestParseSchema(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name"]
	}`)

	schema, err := ParseSchema(schemaJSON)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Type)
	}

	if len(schema.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(schema.Properties))
	}

	if len(schema.Required) != 1 || schema.Required[0] != "name" {
		t.Errorf("expected required ['name'], got %v", schema.Required)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple json",
			input:    `{"name": "John"}`,
			expected: `{"name": "John"}`,
		},
		{
			name:     "json with prefix",
			input:    `Here is the result: {"name": "John"}`,
			expected: `{"name": "John"}`,
		},
		{
			name:     "json with suffix",
			input:    `{"name": "John"} is the answer`,
			expected: `{"name": "John"}`,
		},
		{
			name:     "nested json",
			input:    `{"person": {"name": "John", "age": 30}}`,
			expected: `{"person": {"name": "John", "age": 30}}`,
		},
		{
			name:     "no json",
			input:    "just plain text",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSchemaFromStruct(t *testing.T) {
	type TestStruct struct {
		Name   string   `json:"name" validate:"required"`
		Age    int      `json:"age"`
		Active bool     `json:"active"`
		Tags   []string `json:"tags"`
		Score  float64  `json:"score"`
	}

	schema := SchemaFromStruct(reflect.TypeOf(TestStruct{}))

	if schema.Type != "object" {
		t.Errorf("expected type 'object', got %q", schema.Type)
	}

	expectedProps := map[string]string{
		"name":   "string",
		"age":    "integer",
		"active": "boolean",
		"tags":   "array",
		"score":  "number",
	}

	for name, expectedType := range expectedProps {
		prop, exists := schema.Properties[name]
		if !exists {
			t.Errorf("missing property %q", name)
			continue
		}
		if prop.Type != expectedType {
			t.Errorf("property %q: expected type %q, got %q", name, expectedType, prop.Type)
		}
	}

	// Check required fields
	found := false
	for _, req := range schema.Required {
		if req == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'name' to be in required fields")
	}
}
