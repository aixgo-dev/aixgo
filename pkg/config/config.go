package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// API Keys
	OpenAIKey      string `yaml:"openai_key"`
	AnthropicKey   string `yaml:"anthropic_key"`
	HuggingFaceKey string `yaml:"huggingface_key"`

	// GCP Configuration
	GCPProject     string `yaml:"gcp_project"`
	GCPCredentials string `yaml:"gcp_credentials"`

	// Model Configuration
	DefaultModel    string `yaml:"default_model"`
	EmbeddingModel  string `yaml:"embedding_model"`
	MaxTokens       int    `yaml:"max_tokens"`
	Temperature     float64 `yaml:"temperature"`

	// Vector Store
	VectorProvider string            `yaml:"vector_provider"` // memory, firestore, pinecone
	VectorConfig   map[string]string `yaml:"vector_config"`

	// Agents Configuration
	Agents map[string]AgentConfig `yaml:"agents"`

	// Runtime Configuration
	Runtime RuntimeConfig `yaml:"runtime"`
}

// AgentConfig holds configuration for a single agent
type AgentConfig struct {
	Name     string                 `yaml:"name"`
	Role     string                 `yaml:"role"`
	Model    string                 `yaml:"model"`
	Prompt   string                 `yaml:"prompt"`
	Settings map[string]interface{} `yaml:"settings"`
}

// RuntimeConfig holds runtime configuration
type RuntimeConfig struct {
	ChannelBufferSize  int  `yaml:"channel_buffer_size"`
	MaxConcurrentCalls int  `yaml:"max_concurrent_calls"`
	EnableMetrics      bool `yaml:"enable_metrics"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1000
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.Runtime.ChannelBufferSize == 0 {
		cfg.Runtime.ChannelBufferSize = 100
	}

	// Load API keys from environment if not in config
	if cfg.OpenAIKey == "" {
		cfg.OpenAIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.AnthropicKey == "" {
		cfg.AnthropicKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if cfg.HuggingFaceKey == "" {
		cfg.HuggingFaceKey = os.Getenv("HUGGINGFACE_API_KEY")
	}
	if cfg.GCPProject == "" {
		cfg.GCPProject = os.Getenv("GCP_PROJECT")
	}
	if cfg.GCPCredentials == "" {
		cfg.GCPCredentials = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}

	return &cfg, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DefaultModel == "" {
		return fmt.Errorf("default_model is required")
	}

	if c.OpenAIKey == "" && c.AnthropicKey == "" && c.HuggingFaceKey == "" {
		return fmt.Errorf("at least one API key must be configured")
	}

	return nil
}
