package agents

import (
	"context"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

// BaseAgent provides common functionality for all agents
// Embed this in your agent structs to automatically implement the Agent interface
type BaseAgent struct {
	name   string
	role   string
	ready  bool
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(def agent.AgentDef) *BaseAgent {
	return &BaseAgent{
		name:  def.Name,
		role:  def.Role,
		ready: true,
	}
}

// Name returns the agent name
func (b *BaseAgent) Name() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.name
}

// Role returns the agent role
func (b *BaseAgent) Role() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.role
}

// Ready returns whether the agent is ready
func (b *BaseAgent) Ready() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ready
}

// SetReady sets the ready state
func (b *BaseAgent) SetReady(ready bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ready = ready
}

// InitContext initializes the context for async execution
func (b *BaseAgent) InitContext(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Cancel previous context if exists to prevent leaks
	if b.cancel != nil {
		b.cancel()
	}

	b.ctx, b.cancel = context.WithCancel(ctx)
}

// GetContext returns the agent's context
// If the context is not initialized, returns context.Background()
func (b *BaseAgent) GetContext() context.Context {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

// Stop gracefully stops the agent
func (b *BaseAgent) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cancel != nil {
		b.cancel()
	}
	b.ready = false
	return nil
}

// DefaultExecute provides a default Execute implementation that returns not implemented
// Override this in your agent
func (b *BaseAgent) DefaultExecute(ctx context.Context, input *agent.Message) (*agent.Message, error) {
	return nil, &agent.NotImplementedError{
		AgentName: b.Name(),
		Method:    "Execute",
	}
}

// DefaultStart provides a default Start implementation that returns not implemented
// Override this in your agent
func (b *BaseAgent) DefaultStart(ctx context.Context) error {
	return &agent.NotImplementedError{
		AgentName: b.Name(),
		Method:    "Start",
	}
}
