package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a large file (> 1MB)
	largeFile := filepath.Join(tmpDir, "large.yaml")
	data := strings.Repeat("x: value\n", 200000) // ~1.6MB
	err := os.WriteFile(largeFile, []byte(data), 0600)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err = LoadConfig(largeFile)
	if err == nil {
		t.Error("expected error for large file")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got: %v", err)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	validConfig := `
default_model: gpt-4
openai_key: test-key
max_tokens: 100
temperature: 0.5
`

	validFile := filepath.Join(tmpDir, "valid.yaml")
	err := os.WriteFile(validFile, []byte(validConfig), 0600)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg, err := LoadConfig(validFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Error("expected config, got nil")
	}
	if cfg != nil && cfg.DefaultModel != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", cfg.DefaultModel)
	}
}

func TestLoadConfig_NonexistentFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	invalidYAML := `
default_model: gpt-4
invalid yaml here: [[[
`

	invalidFile := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(invalidFile, []byte(invalidYAML), 0600)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	_, err = LoadConfig(invalidFile)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
