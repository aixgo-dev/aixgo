package agent

import (
	"context"
	pb "github.com/aixgo-dev/aixgo/proto"
	"sync"
	"time"
)

type Agent interface {
	Start(ctx context.Context) error
}

type AgentDef struct {
	Name     string         `yaml:"name"`
	Role     string         `yaml:"role"`
	Interval Duration       `yaml:"interval,omitempty"`
	Listen   string         `yaml:"listen,omitempty"`
	Inputs   []Input        `yaml:"inputs,omitempty"`
	Outputs  []Output       `yaml:"outputs,omitempty"`
	Model    string         `yaml:"model,omitempty"`
	Prompt   string         `yaml:"prompt,omitempty"`
	Tools    []Tool         `yaml:"tools,omitempty"`
	Extra    map[string]any `yaml:",inline"`
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
	// TODO: Implement proper unmarshaling when needed
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

// RuntimeFromContext safely extracts the Runtime from context
func RuntimeFromContext(ctx context.Context) (Runtime, bool) {
	rt, ok := ctx.Value(RuntimeKey{}).(Runtime)
	return rt, ok
}

// MustRuntimeFromContext extracts the Runtime from context and panics if not found
// This is useful for maintaining backward compatibility with existing code
func MustRuntimeFromContext(ctx context.Context) Runtime {
	rt, ok := RuntimeFromContext(ctx)
	if !ok {
		panic("runtime not found in context")
	}
	return rt
}
