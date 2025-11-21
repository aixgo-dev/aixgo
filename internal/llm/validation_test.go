package llm

import (
	"testing"
)

func TestNewValidator(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
	}{
		{
			name:   "nil schema",
			schema: nil,
		},
		{
			name:   "empty schema",
			schema: map[string]any{},
		},
		{
			name: "schema with properties",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			if v == nil {
				t.Fatal("NewValidator returned nil")
			}
			if tt.schema != nil && v.Schema == nil {
				t.Error("Validator.Schema is nil")
			}
		})
	}
}

func TestValidator_Validate_MissingProperties(t *testing.T) {
	schema := map[string]any{
		"required": []any{"name"},
	}

	v := NewValidator(schema)
	input := map[string]any{"name": "test"}

	err := v.Validate(input)
	if err == nil {
		t.Error("expected error for missing properties in schema, got nil")
	}
	if err.Error() != "missing 'properties' in schema" {
		t.Errorf("error = %v, want 'missing 'properties' in schema'", err)
	}
}

func TestValidator_Validate_RequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]any
		input   map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name: "all required fields present",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "number"},
				},
				"required": []any{"name", "age"},
			},
			input: map[string]any{
				"name": "John",
				"age":  float64(30),
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "number"},
				},
				"required": []any{"name", "age"},
			},
			input: map[string]any{
				"name": "John",
			},
			wantErr: true,
			errMsg:  "missing required field: age",
		},
		{
			name: "no required fields",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			input: map[string]any{
				"name": "John",
			},
			wantErr: false,
		},
		{
			name: "empty input with required fields",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"name"},
			},
			input:   map[string]any{},
			wantErr: true,
			errMsg:  "missing required field: name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			err := v.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidator_Validate_TypeChecking(t *testing.T) {
	tests := []struct {
		name        string
		schema      map[string]any
		input       map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name: "valid string type",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			input: map[string]any{
				"name": "John",
			},
			wantErr: false,
		},
		{
			name: "invalid string type",
			schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			input: map[string]any{
				"name": 123,
			},
			wantErr:     true,
			errContains: "must be string",
		},
		{
			name: "valid number type - float64",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{"type": "number"},
				},
			},
			input: map[string]any{
				"age": float64(30),
			},
			wantErr: false,
		},
		{
			name: "valid number type - int",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{"type": "number"},
				},
			},
			input: map[string]any{
				"age": 30,
			},
			wantErr: false,
		},
		{
			name: "invalid number type",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{"type": "number"},
				},
			},
			input: map[string]any{
				"age": "thirty",
			},
			wantErr:     true,
			errContains: "must be number",
		},
		{
			name: "valid boolean type",
			schema: map[string]any{
				"properties": map[string]any{
					"active": map[string]any{"type": "boolean"},
				},
			},
			input: map[string]any{
				"active": true,
			},
			wantErr: false,
		},
		{
			name: "invalid boolean type",
			schema: map[string]any{
				"properties": map[string]any{
					"active": map[string]any{"type": "boolean"},
				},
			},
			input: map[string]any{
				"active": "true",
			},
			wantErr:     true,
			errContains: "must be boolean",
		},
		{
			name: "valid object type",
			schema: map[string]any{
				"properties": map[string]any{
					"metadata": map[string]any{"type": "object"},
				},
			},
			input: map[string]any{
				"metadata": map[string]any{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "invalid object type",
			schema: map[string]any{
				"properties": map[string]any{
					"metadata": map[string]any{"type": "object"},
				},
			},
			input: map[string]any{
				"metadata": "not an object",
			},
			wantErr:     true,
			errContains: "must be object",
		},
		{
			name: "valid array type",
			schema: map[string]any{
				"properties": map[string]any{
					"tags": map[string]any{"type": "array"},
				},
			},
			input: map[string]any{
				"tags": []string{"tag1", "tag2"},
			},
			wantErr: false,
		},
		{
			name: "invalid array type",
			schema: map[string]any{
				"properties": map[string]any{
					"tags": map[string]any{"type": "array"},
				},
			},
			input: map[string]any{
				"tags": "not an array",
			},
			wantErr:     true,
			errContains: "must be array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			err := v.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" {
					if !contains(err.Error(), tt.errContains) {
						t.Errorf("error = %v, want to contain %v", err, tt.errContains)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidator_Validate_Minimum(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]any
		input   map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name: "value above minimum",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{
						"type":    "number",
						"minimum": float64(18),
					},
				},
			},
			input: map[string]any{
				"age": float64(25),
			},
			wantErr: false,
		},
		{
			name: "value equal to minimum",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{
						"type":    "number",
						"minimum": float64(18),
					},
				},
			},
			input: map[string]any{
				"age": float64(18),
			},
			wantErr: false,
		},
		{
			name: "value below minimum",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{
						"type":    "number",
						"minimum": float64(18),
					},
				},
			},
			input: map[string]any{
				"age": float64(15),
			},
			wantErr: true,
			errMsg:  "age below minimum 18",
		},
		{
			name: "negative minimum",
			schema: map[string]any{
				"properties": map[string]any{
					"temp": map[string]any{
						"type":    "number",
						"minimum": float64(-10),
					},
				},
			},
			input: map[string]any{
				"temp": float64(-5),
			},
			wantErr: false,
		},
		{
			name: "zero minimum",
			schema: map[string]any{
				"properties": map[string]any{
					"count": map[string]any{
						"type":    "number",
						"minimum": float64(0),
					},
				},
			},
			input: map[string]any{
				"count": float64(0),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			err := v.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidator_Validate_Maximum(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]any
		input   map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name: "value below maximum",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{
						"type":    "number",
						"maximum": float64(100),
					},
				},
			},
			input: map[string]any{
				"age": float64(50),
			},
			wantErr: false,
		},
		{
			name: "value equal to maximum",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{
						"type":    "number",
						"maximum": float64(100),
					},
				},
			},
			input: map[string]any{
				"age": float64(100),
			},
			wantErr: false,
		},
		{
			name: "value above maximum",
			schema: map[string]any{
				"properties": map[string]any{
					"age": map[string]any{
						"type":    "number",
						"maximum": float64(100),
					},
				},
			},
			input: map[string]any{
				"age": float64(150),
			},
			wantErr: true,
			errMsg:  "age above maximum 100",
		},
		{
			name: "minimum and maximum range valid",
			schema: map[string]any{
				"properties": map[string]any{
					"score": map[string]any{
						"type":    "number",
						"minimum": float64(0),
						"maximum": float64(100),
					},
				},
			},
			input: map[string]any{
				"score": float64(75),
			},
			wantErr: false,
		},
		{
			name: "minimum and maximum range - below minimum",
			schema: map[string]any{
				"properties": map[string]any{
					"score": map[string]any{
						"type":    "number",
						"minimum": float64(0),
						"maximum": float64(100),
					},
				},
			},
			input: map[string]any{
				"score": float64(-5),
			},
			wantErr: true,
			errMsg:  "score below minimum 0",
		},
		{
			name: "minimum and maximum range - above maximum",
			schema: map[string]any{
				"properties": map[string]any{
					"score": map[string]any{
						"type":    "number",
						"minimum": float64(0),
						"maximum": float64(100),
					},
				},
			},
			input: map[string]any{
				"score": float64(105),
			},
			wantErr: true,
			errMsg:  "score above maximum 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			err := v.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidator_Validate_UnknownField(t *testing.T) {
	schema := map[string]any{
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}

	v := NewValidator(schema)
	input := map[string]any{
		"name":    "John",
		"unknown": "field",
	}

	err := v.Validate(input)
	if err == nil {
		t.Error("expected error for unknown field, got nil")
	}
	if err.Error() != "unknown field: unknown" {
		t.Errorf("error = %v, want 'unknown field: unknown'", err)
	}
}

func TestValidator_Validate_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]any
		input   map[string]any
		wantErr bool
	}{
		{
			name: "multiple fields all valid",
			schema: map[string]any{
				"properties": map[string]any{
					"name":     map[string]any{"type": "string"},
					"age":      map[string]any{"type": "number", "minimum": float64(0), "maximum": float64(150)},
					"active":   map[string]any{"type": "boolean"},
					"tags":     map[string]any{"type": "array"},
					"metadata": map[string]any{"type": "object"},
				},
				"required": []any{"name", "age"},
			},
			input: map[string]any{
				"name":     "Alice",
				"age":      float64(30),
				"active":   true,
				"tags":     []string{"tag1", "tag2"},
				"metadata": map[string]any{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "empty input empty schema",
			schema: map[string]any{
				"properties": map[string]any{},
			},
			input:   map[string]any{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.schema)
			err := v.Validate(tt.input)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCheckType(t *testing.T) {
	tests := []struct {
		name    string
		val     any
		typ     string
		wantErr bool
		errMsg  string
	}{
		// String tests
		{"string valid", "test", "string", false, ""},
		{"string invalid", 123, "string", true, "must be string"},

		// Number tests
		{"number valid float64", float64(3.14), "number", false, ""},
		{"number valid int", 42, "number", false, ""},
		{"number invalid", "not a number", "number", true, "must be number"},

		// Boolean tests
		{"boolean valid", true, "boolean", false, ""},
		{"boolean invalid", "true", "boolean", true, "must be boolean"},

		// Object tests
		{"object valid", map[string]any{"key": "value"}, "object", false, ""},
		{"object invalid", "not an object", "object", true, "must be object"},

		// Array tests
		{"array valid", []string{"a", "b"}, "array", false, ""},
		{"array invalid", "not an array", "array", true, "must be array"},

		// Unknown type (should pass)
		{"unknown type", "anything", "unknown", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkType(tt.val, tt.typ)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if err.Error() != tt.errMsg {
					t.Errorf("error = %v, want %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
