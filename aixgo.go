package aixgo

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/llm/inference"
	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/mcp"
	"github.com/aixgo-dev/aixgo/pkg/security"
)

// Config represents the top-level configuration
type Config struct {
	Supervisor    SupervisorDef     `yaml:"supervisor,omitempty"`
	MCPServers    []MCPServerDef    `yaml:"mcp_servers,omitempty"`
	ModelServices []ModelServiceDef `yaml:"model_services,omitempty"`
	Agents        []agent.AgentDef  `yaml:"agents"`
	Session       SessionConfig     `yaml:"session,omitempty"`
}

// SessionConfig configures session persistence.
type SessionConfig struct {
	// Enabled determines whether sessions are active.
	// Default: true (sessions are enabled by default).
	Enabled bool `yaml:"enabled"`

	// Store specifies the storage backend type.
	// Options: "file", "firestore", "postgres"
	// Default: "file"
	Store string `yaml:"store"`

	// BaseDir is the base directory for file-based storage.
	// Default: ~/.aixgo/sessions
	BaseDir string `yaml:"base_dir"`

	// Checkpoint contains checkpoint configuration.
	Checkpoint CheckpointConfig `yaml:"checkpoint,omitempty"`
}

// CheckpointConfig holds checkpoint-specific settings.
type CheckpointConfig struct {
	// AutoSave enables automatic checkpoint creation.
	AutoSave bool `yaml:"auto_save"`

	// Interval is the auto-save interval (e.g., "5m").
	Interval string `yaml:"interval"`
}

// SupervisorDef represents supervisor configuration
type SupervisorDef struct {
	Name      string `yaml:"name"`
	Model     string `yaml:"model"`
	MaxRounds int    `yaml:"max_rounds,omitempty"`
}

// MCPServerDef represents an MCP server configuration
type MCPServerDef struct {
	Name      string      `yaml:"name"`
	Transport string      `yaml:"transport"` // "local" or "grpc"
	Address   string      `yaml:"address,omitempty"`
	TLS       bool        `yaml:"tls,omitempty"`
	Auth      *MCPAuthDef `yaml:"auth,omitempty"`
}

// MCPAuthDef represents MCP authentication configuration
type MCPAuthDef struct {
	Type     string `yaml:"type"` // "bearer", "oauth"
	Token    string `yaml:"token,omitempty"`
	TokenEnv string `yaml:"token_env,omitempty"`
}

// ModelServiceDef represents a model service configuration
type ModelServiceDef struct {
	Name      string         `yaml:"name"`
	Provider  string         `yaml:"provider"`  // "huggingface", "openai", etc.
	Model     string         `yaml:"model"`     // Model ID
	Runtime   string         `yaml:"runtime"`   // "ollama", "vllm", "cloud"
	Transport string         `yaml:"transport"` // "local", "grpc"
	Address   string         `yaml:"address,omitempty"`
	Config    map[string]any `yaml:"config,omitempty"`
}

// FileReader interface for reading files (testable)
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// OSFileReader implements FileReader using os.ReadFile
type OSFileReader struct{}

func (r *OSFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path) // #nosec G304 - path is from trusted config file input
}

// ConfigLoader loads configuration from a file
type ConfigLoader struct {
	fileReader FileReader
	yamlParser *security.SafeYAMLParser
}

// NewConfigLoader creates a new config loader with default security limits
func NewConfigLoader(fr FileReader) *ConfigLoader {
	return &ConfigLoader{
		fileReader: fr,
		yamlParser: security.NewSafeYAMLParser(security.DefaultYAMLLimits()),
	}
}

// NewConfigLoaderWithLimits creates a new config loader with custom YAML security limits
func NewConfigLoaderWithLimits(fr FileReader, limits security.YAMLLimits) *ConfigLoader {
	return &ConfigLoader{
		fileReader: fr,
		yamlParser: security.NewSafeYAMLParser(limits),
	}
}

// LoadConfig loads and parses a config file with security limits
func (cl *ConfigLoader) LoadConfig(configPath string) (*Config, error) {
	data, err := cl.fileReader.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	// Use secure YAML parser with size/depth/complexity limits
	if err := cl.yamlParser.UnmarshalYAML(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Run starts the aixgo agent system from a config file
func Run(configPath string) error {
	return RunWithMCP(configPath, nil)
}

// RunWithMCP starts the aixgo agent system with optional MCP servers
func RunWithMCP(configPath string, servers ...*mcp.Server) error {
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

	// Register local MCP servers
	for _, server := range servers {
		if server != nil {
			mcp.RegisterLocalServer(server)
			log.Printf("Registered local MCP server: %s", server.Name())
		}
	}

	return RunWithConfig(config)
}

// RunWithConfig starts the aixgo agent system with the provided config
func RunWithConfig(config *Config) error {
	return RunWithConfigAndRuntime(config, NewRuntime())
}

// RunWithConfigAndRuntime starts the system with a custom runtime (useful for testing)
func RunWithConfigAndRuntime(config *Config, rt agent.Runtime) error {
	ctx := context.Background()

	// Initialize MCP servers from config
	mcpServers, err := initializeMCPServers(ctx, config.MCPServers)
	if err != nil {
		log.Printf("Warning: Failed to initialize MCP servers: %v", err)
		// Continue even if MCP initialization fails
	}

	// Initialize model services from config
	modelServices, err := initializeModelServices(config.ModelServices)
	if err != nil {
		log.Printf("Warning: Failed to initialize model services: %v", err)
		// Continue even if model service initialization fails
	}

	// Create agents
	agents := make(map[string]agent.Agent)
	for _, def := range config.Agents {
		a, err := agent.CreateAgent(def, rt)
		if err != nil {
			return fmt.Errorf("failed to create agent %s: %w", def.Name, err)
		}

		// If this is a ReAct agent with MCP servers configured, connect them
		if len(def.MCPServers) > 0 {
			if err := connectAgentToMCP(ctx, a, def.MCPServers, mcpServers); err != nil {
				log.Printf("Warning: Failed to connect agent %s to MCP servers: %v", def.Name, err)
			}
		}

		// If this is a HuggingFace model, set up the provider
		if isHuggingFaceModel(def.Model) {
			if err := setupHuggingFaceProvider(a, def.Model, modelServices); err != nil {
				log.Printf("Warning: Failed to setup HuggingFace provider for agent %s: %v", def.Name, err)
			}
		}

		agents[def.Name] = a
		log.Printf("Created agent: %s (role: %s)", def.Name, def.Role)
	}

	return StartAgents(agents, config.Agents, rt)
}

// PhasedStarter is implemented by runtimes that support phased agent startup.
// This enables dependency-aware startup ordering.
type PhasedStarter interface {
	StartAgentsPhased(ctx context.Context, agentDefs map[string]agent.AgentDef) error
}

// StartAgents starts all agents with the given runtime using dependency-aware phased startup.
// If the runtime supports PhasedStarter interface, agents are started in topological order
// based on their depends_on declarations. Otherwise, agents are started concurrently.
func StartAgents(agents map[string]agent.Agent, agentDefs []agent.AgentDef, rt agent.Runtime) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add runtime to context
	ctx = context.WithValue(ctx, agent.RuntimeKey{}, rt)

	// Register all agents with the runtime first
	for name, a := range agents {
		if err := rt.Register(a); err != nil {
			return fmt.Errorf("failed to register agent %s: %w", name, err)
		}
	}

	// Start the runtime
	if err := rt.Start(ctx); err != nil {
		return fmt.Errorf("failed to start runtime: %w", err)
	}

	// Build agent defs map for phased startup
	defsMap := make(map[string]agent.AgentDef)
	for _, def := range agentDefs {
		defsMap[def.Name] = def
	}

	// Check if runtime supports phased startup
	if ps, ok := rt.(PhasedStarter); ok {
		log.Println("Using phased agent startup (dependency-aware)")
		if err := ps.StartAgentsPhased(ctx, defsMap); err != nil {
			return fmt.Errorf("phased startup failed: %w", err)
		}
	} else {
		// Fallback to concurrent startup for runtimes that don't support phased
		log.Println("Using concurrent agent startup (no dependency ordering)")
		for name, a := range agents {
			go func(n string, ag agent.Agent) {
				if err := ag.Start(ctx); err != nil {
					log.Printf("Agent %s error: %v", n, err)
				}
			}(name, a)
		}
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

// Helper functions for initialization

// initializeMCPServers initializes MCP server connections from config
func initializeMCPServers(ctx context.Context, serverDefs []MCPServerDef) (map[string]mcp.ServerConfig, error) {
	servers := make(map[string]mcp.ServerConfig)

	for _, def := range serverDefs {
		config := mcp.ServerConfig{
			Name:      def.Name,
			Transport: def.Transport,
			Address:   def.Address,
			TLS:       def.TLS,
		}

		if def.Auth != nil {
			config.Auth = &mcp.AuthConfig{
				Type:     def.Auth.Type,
				Token:    def.Auth.Token,
				TokenEnv: def.Auth.TokenEnv,
			}
		}

		servers[def.Name] = config
		log.Printf("Registered MCP server config: %s (transport: %s)", def.Name, def.Transport)
	}

	return servers, nil
}

// initializeModelServices initializes model services from config
func initializeModelServices(serviceDefs []ModelServiceDef) (map[string]any, error) {
	services := make(map[string]any)

	for _, def := range serviceDefs {
		log.Printf("Registered model service: %s (provider: %s, model: %s)", def.Name, def.Provider, def.Model)
		services[def.Name] = def
	}

	return services, nil
}

// connectAgentToMCP connects an agent to its configured MCP servers
func connectAgentToMCP(ctx context.Context, a agent.Agent, serverNames []string, mcpServers map[string]mcp.ServerConfig) error {
	// Type assert to ReActAgent to access MCP connection methods
	type MCPConnector interface {
		ConnectMCPServers(ctx context.Context, configs []mcp.ServerConfig) error
	}

	connector, ok := a.(MCPConnector)
	if !ok {
		return fmt.Errorf("agent does not support MCP connections")
	}

	// Build server configs for the agent
	var configs []mcp.ServerConfig
	for _, name := range serverNames {
		config, exists := mcpServers[name]
		if !exists {
			log.Printf("Warning: MCP server %s not found in config", name)
			continue
		}
		configs = append(configs, config)
	}

	if len(configs) == 0 {
		return fmt.Errorf("no valid MCP servers found")
	}

	return connector.ConnectMCPServers(ctx, configs)
}

// setupHuggingFaceProvider sets up a HuggingFace provider for an agent
func setupHuggingFaceProvider(a agent.Agent, model string, modelServices map[string]any) error {
	// Type assert to access provider setter
	type ProviderSetter interface {
		SetProvider(prov provider.Provider)
	}

	setter, ok := a.(ProviderSetter)
	if !ok {
		return fmt.Errorf("agent does not support provider setting")
	}

	// Look for a matching model service in config
	var modelServiceDef *ModelServiceDef
	for _, svc := range modelServices {
		if def, ok := svc.(ModelServiceDef); ok {
			if def.Model == model || def.Name == model {
				modelServiceDef = &def
				break
			}
		}
	}

	// Determine which inference service to use
	var inf inference.InferenceService
	var useOptimized bool

	if modelServiceDef != nil {
		// Check config for optimized variant
		if variant, ok := modelServiceDef.Config["variant"].(string); ok && variant == "optimized" {
			useOptimized = true
		}

		// Create inference service based on runtime type
		switch modelServiceDef.Runtime {
		case "ollama":
			// Get Ollama address from config or use default
			address := "http://localhost:11434"
			if addr, ok := modelServiceDef.Config["address"].(string); ok && addr != "" {
				address = addr
			}
			ollamaInf, err := inference.NewOllamaService(address)
			if err != nil {
				return fmt.Errorf("failed to create Ollama service for model %s: %w", model, err)
			}
			inf = ollamaInf
			log.Printf("Created Ollama inference service for model: %s (address: %s)", model, address)

		case "vllm":
			// Get vLLM address and API key from config
			address := "http://localhost:8000"
			if addr, ok := modelServiceDef.Config["address"].(string); ok && addr != "" {
				address = addr
			}
			apiKey := ""
			if key, ok := modelServiceDef.Config["api_key"].(string); ok {
				apiKey = key
			}
			inf = inference.NewVLLMService(address, apiKey)
			log.Printf("Created vLLM inference service for model: %s (address: %s)", model, address)

		case "cloud":
			// Get HuggingFace token from config or environment
			token := os.Getenv("HF_TOKEN")
			if t, ok := modelServiceDef.Config["token"].(string); ok && t != "" {
				token = t
			}
			endpoint := "https://api-inference.huggingface.co"
			if ep, ok := modelServiceDef.Config["endpoint"].(string); ok && ep != "" {
				endpoint = ep
			}
			inf = inference.NewHuggingFaceService(endpoint, token)
			log.Printf("Created HuggingFace cloud inference service for model: %s", model)

		default:
			return fmt.Errorf("unknown runtime: %s", modelServiceDef.Runtime)
		}
	} else {
		// No model service defined - use mock service with warning
		log.Printf("Warning: No model service configured for HuggingFace model %s", model)
		log.Printf("Using mock inference service. For production, add a model_services entry in your config")
		log.Printf("Example config:")
		log.Printf("  model_services:")
		log.Printf("    - name: my-llama")
		log.Printf("      provider: huggingface")
		log.Printf("      model: %s", model)
		log.Printf("      runtime: ollama  # or vllm, cloud")
		log.Printf("      config:")
		log.Printf("        variant: optimized  # optional: use optimized provider")
		inf = inference.NewMockInferenceService(model)
	}

	// Create the appropriate HuggingFace provider variant
	var prov provider.Provider
	if useOptimized {
		prov = provider.NewOptimizedHuggingFaceProvider(inf, model)
		log.Printf("Created optimized HuggingFace provider for model: %s", model)
	} else {
		prov = provider.NewHuggingFaceProvider(inf, model)
		log.Printf("Created basic HuggingFace provider for model: %s", model)
	}

	// Set the provider on the agent
	setter.SetProvider(prov)
	return nil
}

// isHuggingFaceModel checks if a model name is a HuggingFace model
func isHuggingFaceModel(model string) bool {
	// HuggingFace models typically have "/" in the name (e.g., "meta-llama/Llama-2-7b")
	hfPatterns := []string{
		"meta-llama/", "mistralai/", "tiiuae/", "EleutherAI/",
		"bigscience/", "facebook/", "google/", "microsoft/",
	}

	for _, pattern := range hfPatterns {
		if len(model) >= len(pattern) && model[:len(pattern)] == pattern {
			return true
		}
	}

	return false
}
