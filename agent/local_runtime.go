package agent

import (
	"context"
	"fmt"
	"sync"
)

// LocalRuntime is a single-process runtime for agent coordination.
// It uses in-memory channels for message passing and is suitable for
// applications that run all agents in a single Go binary.
//
// LocalRuntime is thread-safe and can be used concurrently.
type LocalRuntime struct {
	mu       sync.RWMutex
	agents   map[string]Agent
	channels map[string]chan *Message
	order    []string // Registration order for deterministic startup
	started  bool
}

// NewLocalRuntime creates a new local runtime.
func NewLocalRuntime() *LocalRuntime {
	return &LocalRuntime{
		agents:   make(map[string]Agent),
		channels: make(map[string]chan *Message),
		order:    make([]string, 0),
	}
}

// Register adds an agent to the runtime.
func (r *LocalRuntime) Register(agent Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := agent.Name()
	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent %s already registered", name)
	}

	r.agents[name] = agent
	r.channels[name] = make(chan *Message, 100)
	r.order = append(r.order, name)
	return nil
}

// Unregister removes an agent from the runtime.
func (r *LocalRuntime) Unregister(name string) error {
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

	// Remove from order slice
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}

	return nil
}

// Get retrieves a registered agent by name.
func (r *LocalRuntime) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", name)
	}
	return a, nil
}

// List returns all registered agent names.
func (r *LocalRuntime) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.order))
	copy(names, r.order)
	return names
}

// Call sends a message to an agent and waits for a synchronous response.
func (r *LocalRuntime) Call(ctx context.Context, target string, input *Message) (*Message, error) {
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

// CallParallel invokes multiple agents concurrently and returns all results.
func (r *LocalRuntime) CallParallel(ctx context.Context, targets []string, input *Message) (map[string]*Message, map[string]error) {
	results := make(map[string]*Message)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()

			result, err := r.Call(ctx, t, input)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors[t] = err
			} else {
				results[t] = result
			}
		}(target)
	}

	wg.Wait()
	return results, errors
}

// Send sends a message to an agent asynchronously.
func (r *LocalRuntime) Send(target string, msg *Message) error {
	r.mu.RLock()
	ch, ok := r.channels[target]
	r.mu.RUnlock()

	if !ok {
		// Create channel if it doesn't exist
		r.mu.Lock()
		if _, exists := r.channels[target]; !exists {
			r.channels[target] = make(chan *Message, 100)
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

// Recv returns a channel to receive messages from an agent.
func (r *LocalRuntime) Recv(source string) (<-chan *Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.channels[source]; !ok {
		r.channels[source] = make(chan *Message, 100)
	}

	return r.channels[source], nil
}

// Broadcast sends a message to all registered agents asynchronously.
func (r *LocalRuntime) Broadcast(msg *Message) error {
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

// Start starts all registered agents in registration order.
func (r *LocalRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return fmt.Errorf("runtime already started")
	}
	r.started = true
	agentOrder := make([]string, len(r.order))
	copy(agentOrder, r.order)
	r.mu.Unlock()

	// Start agents in order
	for _, name := range agentOrder {
		r.mu.RLock()
		agent := r.agents[name]
		r.mu.RUnlock()

		if agent == nil {
			continue
		}

		// Start each agent in a goroutine
		go func(a Agent) {
			if err := a.Start(ctx); err != nil {
				// Log error but don't stop other agents
				// In production, you might want to use a proper logger here
				fmt.Printf("agent %s start error: %v\n", a.Name(), err)
			}
		}(agent)
	}

	return nil
}

// Stop gracefully shuts down all registered agents.
func (r *LocalRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return fmt.Errorf("runtime not started")
	}
	agents := make([]Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agents = append(agents, a)
	}
	r.started = false
	r.mu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	// Stop all agents concurrently
	for _, agent := range agents {
		wg.Add(1)
		go func(a Agent) {
			defer wg.Done()
			if err := a.Stop(ctx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(agent)
	}

	wg.Wait()
	return firstErr
}
