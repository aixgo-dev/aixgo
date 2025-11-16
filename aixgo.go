package aixgo

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"gopkg.in/yaml.v3"
)

// Config represents the top-level configuration
type Config struct {
	Agents []agent.AgentDef `yaml:"agents"`
}

// FileReader interface for reading files (testable)
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// OSFileReader implements FileReader using os.ReadFile
type OSFileReader struct{}

func (r *OSFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// ConfigLoader loads configuration from a file
type ConfigLoader struct {
	fileReader FileReader
}

// NewConfigLoader creates a new config loader
func NewConfigLoader(fr FileReader) *ConfigLoader {
	return &ConfigLoader{fileReader: fr}
}

// LoadConfig loads and parses a config file
func (cl *ConfigLoader) LoadConfig(configPath string) (*Config, error) {
	data, err := cl.fileReader.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Run starts the aixgo agent system from a config file
func Run(configPath string) error {
	// Initialize observability from environment variables
	if err := observability.InitFromEnv(); err != nil {
		log.Printf("Warning: Failed to initialize observability: %v", err)
		// Continue even if observability fails
	}

	loader := NewConfigLoader(&OSFileReader{})
	config, err := loader.LoadConfig(configPath)
	if err != nil {
		return err
	}

	return RunWithConfig(config)
}

// RunWithConfig starts the aixgo agent system with the provided config
func RunWithConfig(config *Config) error {
	return RunWithConfigAndRuntime(config, NewSimpleRuntime())
}

// RunWithConfigAndRuntime starts the system with a custom runtime (useful for testing)
func RunWithConfigAndRuntime(config *Config, rt agent.Runtime) error {
	// Create agents
	agents := make(map[string]agent.Agent)
	for _, def := range config.Agents {
		a, err := agent.CreateAgent(def, rt)
		if err != nil {
			return fmt.Errorf("failed to create agent %s: %w", def.Name, err)
		}
		agents[def.Name] = a
		log.Printf("Created agent: %s (role: %s)", def.Name, def.Role)
	}

	return StartAgents(agents, rt)
}

// StartAgents starts all agents with the given runtime
func StartAgents(agents map[string]agent.Agent, rt agent.Runtime) error {
	// Start agents
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add runtime to context
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	for name, a := range agents {
		go func(n string, ag agent.Agent) {
			if err := ag.Start(ctx); err != nil {
				log.Printf("Agent %s error: %v", n, err)
			}
		}(name, a)
	}

	log.Println("All agents started. Press Ctrl+C to stop.")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()

	// Shutdown observability
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()
	if err := observability.Shutdown(shutdownCtx); err != nil {
		log.Printf("Warning: Failed to shutdown observability: %v", err)
	}

	return nil
}
