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

// Agent interface supports both synchronous (Execute) and asynchronous (Start) execution.
// Agents can implement one or both methods depending on their use case.
type Agent interface {
	// Name returns the unique identifier for this agent instance
	Name() string

	// Role returns the agent type/role (e.g., "react", "classifier", "planner")
	Role() string

	// Start runs the agent asynchronously (e.g., listening on inputs)
	// Returns when context is canceled or agent encounters fatal error
	Start(ctx context.Context) error

	// Execute performs synchronous request-response execution
	// Used by orchestration patterns for direct invocation
	Execute(ctx context.Context, input *Message) (*Message, error)

	// Stop gracefully shuts down the agent
	Stop(ctx context.Context) error

	// Ready returns true if the agent is ready to accept requests
	Ready() bool
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

// Runtime interface provides agent execution and message passing capabilities.
// Supports both local (single binary) and distributed (gRPC) deployment modes.
type Runtime interface {
	// Send sends a message to a target agent asynchronously
	Send(target string, msg *Message) error

	// Recv returns a channel to receive messages from a source agent
	Recv(source string) (<-chan *Message, error)

	// Call invokes an agent synchronously and waits for response
	// Used by orchestration patterns for request-response execution
	Call(ctx context.Context, target string, input *Message) (*Message, error)

	// CallParallel invokes multiple agents concurrently and returns all results
	// Execution continues even if some agents fail (partial results returned)
	CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, map[string]error)

	// Broadcast sends a message to all registered agents asynchronously
	Broadcast(msg *Message) error

	// Register registers an agent instance with the runtime
	Register(agent Agent) error

	// Unregister removes an agent from the runtime
	Unregister(name string) error

	// Get retrieves a registered agent by name
	Get(name string) (Agent, error)

	// List returns all registered agent names
	List() []string

	// Start starts the runtime (e.g., gRPC server for distributed mode)
	Start(ctx context.Context) error

	// Stop gracefully shuts down the runtime
	Stop(ctx context.Context) error
}

type RuntimeKey struct{}

// ErrRuntimeNotFound is returned when runtime is not found in context
var ErrRuntimeNotFound = errors.New("runtime not found in context")

// ErrAgentNotFound is returned when an agent is not found in the runtime
var ErrAgentNotFound = errors.New("agent not found")

// NotImplementedError is returned when a method is not implemented by an agent
type NotImplementedError struct {
	AgentName string
	Method    string
}

func (e *NotImplementedError) Error() string {
	return fmt.Sprintf("agent %s does not implement %s", e.AgentName, e.Method)
}

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
