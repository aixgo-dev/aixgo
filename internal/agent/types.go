package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	pb "github.com/aixgo-dev/aixgo/proto"
)

type Agent interface {
	Start(ctx context.Context) error
}

type AgentDef struct {
	Name       string         `yaml:"name"`
	Role       string         `yaml:"role"`
	Interval   Duration       `yaml:"interval,omitempty"`
	Listen     string         `yaml:"listen,omitempty"`
	Inputs     []Input        `yaml:"inputs,omitempty"`
	Outputs    []Output       `yaml:"outputs,omitempty"`
	Model      string         `yaml:"model,omitempty"`
	Prompt     string         `yaml:"prompt,omitempty"`
	Tools      []Tool         `yaml:"tools,omitempty"`       // Deprecated: use MCPServers
	MCPServers []string       `yaml:"mcp_servers,omitempty"` // MCP server names
	Extra      map[string]any `yaml:",inline"`
}

type Input struct {
	Source string `yaml:"source"`
}
type Output struct {
	Target string `yaml:"target"`
	Addr   string `yaml:"addr,omitempty"`
}

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d *AgentDef) GetString(key, def string) string {
	if v, ok := d.Extra[key].(string); ok {
		return v
	}
	return def
}

func (d *AgentDef) UnmarshalKey(key string, v any) error {
	raw, exists := d.Extra[key]
	if !exists {
		return nil
	}

	// Marshal the raw value to JSON, then unmarshal into the target
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal key %q: %w", key, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal key %q: %w", key, err)
	}

	return nil
}

type Tool struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	InputSchema map[string]any `yaml:"input_schema"`
}

type Message struct{ *pb.Message }

// Registry for agent factory functions
type FactoryFunc func(AgentDef, Runtime) (Agent, error)

// Registry interface allows for testable registry implementations
type Registry interface {
	Register(role string, factory FactoryFunc)
	GetFactory(role string) (FactoryFunc, bool)
}

// DefaultRegistry is the global registry implementation
type DefaultRegistry struct {
	factories map[string]FactoryFunc
	mu        sync.RWMutex
}

var defaultRegistry = &DefaultRegistry{
	factories: make(map[string]FactoryFunc),
}

// NewRegistry creates a new registry instance (useful for testing)
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		factories: make(map[string]FactoryFunc),
	}
}

func (r *DefaultRegistry) Register(role string, factory FactoryFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[role] = factory
}

func (r *DefaultRegistry) GetFactory(role string) (FactoryFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[role]
	return f, ok
}

// Register registers a factory with the default registry
func Register(role string, factory FactoryFunc) {
	defaultRegistry.Register(role, factory)
}

// GetFactory retrieves a factory from the default registry
func GetFactory(role string) (FactoryFunc, bool) {
	return defaultRegistry.GetFactory(role)
}

// Runtime interface - placeholder, will be implemented based on your architecture
type Runtime interface {
	Send(target string, msg *Message) error
	Recv(source string) (<-chan *Message, error)
}

type RuntimeKey struct{}

// ErrRuntimeNotFound is returned when runtime is not found in context
var ErrRuntimeNotFound = errors.New("runtime not found in context")

// RuntimeFromContext safely extracts the Runtime from context.
// Returns the runtime and nil error if found, or nil and ErrRuntimeNotFound if not found.
func RuntimeFromContext(ctx context.Context) (Runtime, error) {
	rt, ok := ctx.Value(RuntimeKey{}).(Runtime)
	if !ok {
		return nil, ErrRuntimeNotFound
	}
	return rt, nil
}

// MustRuntimeFromContext extracts the Runtime from context and panics if not found.
// Deprecated: Use RuntimeFromContext instead and handle the error appropriately.
// This function is maintained for backward compatibility but should be avoided
// in new code as panics can cause unexpected application crashes.
func MustRuntimeFromContext(ctx context.Context) Runtime {
	rt, err := RuntimeFromContext(ctx)
	if err != nil {
		panic("runtime not found in context")
	}
	return rt
}
