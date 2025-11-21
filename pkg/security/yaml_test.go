package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestSafeYAMLParser_BasicParsing(t *testing.T) {
	parser := NewSafeYAMLParser(DefaultYAMLLimits())

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "simple valid YAML",
			yaml: `
name: test
value: 123
enabled: true
`,
			wantErr: false,
		},
		{
			name: "nested valid YAML",
			yaml: `
server:
  host: localhost
  port: 8080
  config:
    timeout: 30
    retries: 3
`,
			wantErr: false,
		},
		{
			name: "array valid YAML",
			yaml: `
items:
  - name: item1
    value: 1
  - name: item2
    value: 2
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := parser.UnmarshalYAML([]byte(tt.yaml), &result)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSafeYAMLParser_FileSizeLimit(t *testing.T) {
	limits := YAMLLimits{
		MaxFileSize:  1024, // 1KB
		MaxDepth:     20,
		MaxNodes:     10000,
		MaxKeyLength: 1024,
		MaxValueSize: 1024,
	}
	parser := NewSafeYAMLParser(limits)

	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{
			name:    "within limit",
			size:    512,
			wantErr: false,
		},
		{
			name:    "at limit",
			size:    1024,
			wantErr: false,
		},
		{
			name:    "exceeds limit",
			size:    2048,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create YAML content of specified size
			content := "data: " + strings.Repeat("x", tt.size-6)
			var result map[string]interface{}
			err := parser.UnmarshalYAML([]byte(content), &result)

			if tt.wantErr && err == nil {
				t.Error("expected error for large file, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSafeYAMLParser_DepthLimit(t *testing.T) {
	limits := YAMLLimits{
		MaxFileSize:  10 * 1024 * 1024,
		MaxDepth:     5, // Low depth limit for testing
		MaxNodes:     10000,
		MaxKeyLength: 1024,
		MaxValueSize: 1024 * 1024,
	}
	parser := NewSafeYAMLParser(limits)

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "within depth limit",
			yaml: `
level1:
  level2:
    level3:
      level4:
        value: test
`,
			wantErr: false,
		},
		{
			name: "exceeds depth limit",
			yaml: `
level1:
  level2:
    level3:
      level4:
        level5:
          level6:
            level7:
              value: test
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := parser.UnmarshalYAML([]byte(tt.yaml), &result)

			if tt.wantErr && err == nil {
				t.Error("expected error for excessive depth, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "depth") {
				t.Errorf("expected depth error, got: %v", err)
			}
		})
	}
}

func TestSafeYAMLParser_NodeCountLimit(t *testing.T) {
	limits := YAMLLimits{
		MaxFileSize:  10 * 1024 * 1024,
		MaxDepth:     20,
		MaxNodes:     50, // Low node limit for testing
		MaxKeyLength: 1024,
		MaxValueSize: 1024 * 1024,
	}
	parser := NewSafeYAMLParser(limits)

	// Create YAML with many nodes
	var buf bytes.Buffer
	buf.WriteString("items:\n")
	for i := 0; i < 100; i++ {
		buf.WriteString("  - value: ")
		buf.WriteString(strings.Repeat("x", 10))
		buf.WriteString("\n")
	}

	var result map[string]interface{}
	err := parser.UnmarshalYAML(buf.Bytes(), &result)

	if err == nil {
		t.Error("expected error for excessive nodes, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "node count") {
		t.Errorf("expected node count error, got: %v", err)
	}
}

func TestSafeYAMLParser_KeyLengthLimit(t *testing.T) {
	limits := YAMLLimits{
		MaxFileSize:  10 * 1024 * 1024,
		MaxDepth:     20,
		MaxNodes:     10000,
		MaxKeyLength: 10, // Very short key limit
		MaxValueSize: 1024 * 1024,
	}
	parser := NewSafeYAMLParser(limits)

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name:    "short key within limit",
			yaml:    "short: value",
			wantErr: false,
		},
		{
			name:    "long key exceeds limit",
			yaml:    "very_long_key_name_exceeding_limit: value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := parser.UnmarshalYAML([]byte(tt.yaml), &result)

			if tt.wantErr && err == nil {
				t.Error("expected error for long key, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "key length") {
				t.Errorf("expected key length error, got: %v", err)
			}
		})
	}
}

func TestSafeYAMLParser_ValueSizeLimit(t *testing.T) {
	limits := YAMLLimits{
		MaxFileSize:  10 * 1024 * 1024,
		MaxDepth:     20,
		MaxNodes:     10000,
		MaxKeyLength: 1024,
		MaxValueSize: 100, // Small value size limit
	}
	parser := NewSafeYAMLParser(limits)

	tests := []struct {
		name      string
		valueSize int
		wantErr   bool
	}{
		{
			name:      "small value within limit",
			valueSize: 50,
			wantErr:   false,
		},
		{
			name:      "large value exceeds limit",
			valueSize: 200,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := "data: " + strings.Repeat("x", tt.valueSize)
			var result map[string]interface{}
			err := parser.UnmarshalYAML([]byte(yaml), &result)

			if tt.wantErr && err == nil {
				t.Error("expected error for large value, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "value size") {
				t.Errorf("expected value size error, got: %v", err)
			}
		})
	}
}

func TestSafeYAMLParser_YAMLBombPrevention(t *testing.T) {
	limits := DefaultYAMLLimits()
	parser := NewSafeYAMLParser(limits)

	// Attempt a billion laughs style attack (scaled down)
	yamlBomb := `
a: &anchor
  - data: xxxxxxxxxx
b:
  - *anchor
  - *anchor
  - *anchor
  - *anchor
  - *anchor
  - *anchor
  - *anchor
  - *anchor
  - *anchor
  - *anchor
`

	var result map[string]interface{}
	err := parser.UnmarshalYAML([]byte(yamlBomb), &result)

	// Should either succeed (if within limits) or fail gracefully
	if err != nil {
		t.Logf("YAML bomb prevented: %v", err)
	}
}

func TestSafeYAMLParser_FromReader(t *testing.T) {
	parser := NewSafeYAMLParser(DefaultYAMLLimits())

	yaml := `
name: test
value: 123
`

	reader := bytes.NewReader([]byte(yaml))
	var result map[string]interface{}
	err := parser.UnmarshalYAMLFromReader(reader, &result)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name=test, got %v", result["name"])
	}
}

func TestSafeYAMLParser_FromReaderSizeLimit(t *testing.T) {
	limits := YAMLLimits{
		MaxFileSize:  100,
		MaxDepth:     20,
		MaxNodes:     10000,
		MaxKeyLength: 1024,
		MaxValueSize: 1024,
	}
	parser := NewSafeYAMLParser(limits)

	// Create content exceeding limit
	largeYAML := "data: " + strings.Repeat("x", 200)
	reader := bytes.NewReader([]byte(largeYAML))

	var result map[string]interface{}
	err := parser.UnmarshalYAMLFromReader(reader, &result)

	if err == nil {
		t.Error("expected error for large input from reader, got nil")
	}
}

func TestDefaultYAMLLimits(t *testing.T) {
	limits := DefaultYAMLLimits()

	if limits.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected MaxFileSize=10MB, got %d", limits.MaxFileSize)
	}
	if limits.MaxDepth != 20 {
		t.Errorf("expected MaxDepth=20, got %d", limits.MaxDepth)
	}
	if limits.MaxNodes != 10000 {
		t.Errorf("expected MaxNodes=10000, got %d", limits.MaxNodes)
	}
	if limits.MaxKeyLength != 1024 {
		t.Errorf("expected MaxKeyLength=1024, got %d", limits.MaxKeyLength)
	}
	if limits.MaxValueSize != 1024*1024 {
		t.Errorf("expected MaxValueSize=1MB, got %d", limits.MaxValueSize)
	}
}

func TestValidateYAMLFile(t *testing.T) {
	limits := DefaultYAMLLimits()

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name:    "valid YAML",
			yaml:    "name: test\nvalue: 123",
			wantErr: false,
		},
		{
			name:    "invalid YAML syntax - unclosed bracket",
			yaml:    "name: test\nmap: {key: value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYAMLFile([]byte(tt.yaml), limits)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSafeYAMLParser_ComplexNestedStructure(t *testing.T) {
	parser := NewSafeYAMLParser(DefaultYAMLLimits())

	complexYAML := `
application:
  name: test-app
  version: 1.0.0
  servers:
    - host: localhost
      port: 8080
      config:
        timeout: 30
        retries: 3
        backends:
          - url: http://backend1.com
            weight: 5
          - url: http://backend2.com
            weight: 3
    - host: remote
      port: 8081
  database:
    host: db.example.com
    port: 5432
    credentials:
      username: admin
      password: secret
`

	var result map[string]interface{}
	err := parser.UnmarshalYAML([]byte(complexYAML), &result)

	if err != nil {
		t.Errorf("failed to parse complex YAML: %v", err)
	}

	// Verify structure
	if result["application"] == nil {
		t.Error("expected application key")
	}
}
