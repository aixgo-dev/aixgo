package aixgo

import (
	"context"
	"fmt"
	"sync"

	"github.com/aixgo-dev/aixgo/internal/agent"
)

// SimpleRuntime is a basic in-memory implementation of the Runtime interface
type SimpleRuntime struct {
	agents   map[string]agent.Agent
	channels map[string]chan *agent.Message
	mu       sync.RWMutex
}

// NewSimpleRuntime creates a new SimpleRuntime
func NewSimpleRuntime() *SimpleRuntime {
	return &SimpleRuntime{
		agents:   make(map[string]agent.Agent),
		channels: make(map[string]chan *agent.Message),
	}
}

// Send sends a message to a target channel
func (r *SimpleRuntime) Send(target string, msg *agent.Message) error {
	r.mu.RLock()
	ch, ok := r.channels[target]
	r.mu.RUnlock()

	if !ok {
		// Create channel if it doesn't exist
		r.mu.Lock()
		if _, exists := r.channels[target]; !exists {
			r.channels[target] = make(chan *agent.Message, 100)
		}
		ch = r.channels[target]
		r.mu.Unlock()
	}

	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("channel %s is full", target)
	}
}

// Recv returns a channel to receive messages from a source
func (r *SimpleRuntime) Recv(source string) (<-chan *agent.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.channels[source]; !ok {
		r.channels[source] = make(chan *agent.Message, 100)
	}

	return r.channels[source], nil
}

// Broadcast sends a message to all channels (stub implementation for compatibility)
func (r *SimpleRuntime) Broadcast(msg *agent.Message) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var firstErr error
	for target := range r.channels {
		if err := r.Send(target, msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Register registers an agent with the runtime
func (r *SimpleRuntime) Register(a agent.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent %s already registered", name)
	}

	r.agents[name] = a
	r.channels[name] = make(chan *agent.Message, 100)
	return nil
}

// Unregister removes an agent from the runtime
func (r *SimpleRuntime) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	delete(r.agents, name)
	if ch, exists := r.channels[name]; exists {
		close(ch)
		delete(r.channels, name)
	}
	return nil
}

// Get retrieves a registered agent by name
func (r *SimpleRuntime) Get(name string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", name)
	}
	return a, nil
}

// List returns all registered agent names
func (r *SimpleRuntime) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Call invokes an agent synchronously and waits for response
func (r *SimpleRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
	r.mu.RLock()
	a, exists := r.agents[target]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("agent %s not found", target)
	}

	if !a.Ready() {
		return nil, fmt.Errorf("agent %s not ready", target)
	}

	return a.Execute(ctx, input)
}

// CallParallel invokes multiple agents concurrently and returns all results
func (r *SimpleRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, map[string]error) {
	results := make(map[string]*agent.Message)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()

			result, err := r.Call(ctx, t, input)

			mu.Lock()
			if err != nil {
				errors[t] = err
			} else {
				results[t] = result
			}
			mu.Unlock()
		}(target)
	}

	wg.Wait()
	return results, errors
}

// Start starts the runtime
func (r *SimpleRuntime) Start(ctx context.Context) error {
	// Simple runtime doesn't need startup logic
	return nil
}

// Stop gracefully shuts down the runtime
func (r *SimpleRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close all channels
	for _, ch := range r.channels {
		close(ch)
	}
	r.channels = make(map[string]chan *agent.Message)

	// Stop all agents
	for _, a := range r.agents {
		_ = a.Stop(ctx)
	}

	return nil
}
