package orchestration

import (
	"context"
	"fmt"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

// Orchestrator defines the interface for agent orchestration patterns.
// All orchestration patterns implement this interface.
type Orchestrator interface {
	// Name returns the orchestrator identifier
	Name() string

	// Pattern returns the pattern type (e.g., "parallel", "router", "rag")
	Pattern() string

	// Execute runs the orchestration pattern synchronously
	Execute(ctx context.Context, input *agent.Message) (*agent.Message, error)

	// Start runs the orchestration pattern asynchronously
	Start(ctx context.Context) error

	// Stop gracefully shuts down the orchestrator
	Stop(ctx context.Context) error

	// Ready returns true if the orchestrator is ready
	Ready() bool
}

// BaseOrchestrator provides common functionality for orchestrators
type BaseOrchestrator struct {
	name    string
	pattern string
	runtime agent.Runtime
	ready   bool
	mu      sync.RWMutex
}

// NewBaseOrchestrator creates a new base orchestrator
func NewBaseOrchestrator(name, pattern string, runtime agent.Runtime) *BaseOrchestrator {
	if runtime == nil {
		panic("orchestrator runtime cannot be nil")
	}
	return &BaseOrchestrator{
		name:    name,
		pattern: pattern,
		runtime: runtime,
		ready:   false, // Start as not ready, concrete types must mark ready
	}
}

// Name returns the orchestrator identifier
func (b *BaseOrchestrator) Name() string {
	return b.name
}

// Pattern returns the pattern type
func (b *BaseOrchestrator) Pattern() string {
	return b.pattern
}

// Runtime returns the runtime
func (b *BaseOrchestrator) Runtime() agent.Runtime {
	return b.runtime
}

// Ready returns true if the orchestrator is ready
func (b *BaseOrchestrator) Ready() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ready
}

// SetReady sets the ready state
func (b *BaseOrchestrator) SetReady(ready bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ready = ready
}

// Execute provides a default implementation that returns an error
func (b *BaseOrchestrator) Execute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	return nil, fmt.Errorf("Execute not implemented for %s orchestrator", b.pattern)
}

// Start is a default no-op implementation
func (b *BaseOrchestrator) Start(ctx context.Context) error {
	return nil
}

// Stop is a default no-op implementation
func (b *BaseOrchestrator) Stop(ctx context.Context) error {
	return nil
}
