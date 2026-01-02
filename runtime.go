package aixgo

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/graph"
	"golang.org/x/sync/errgroup"
)

// SimpleRuntime is a basic in-memory implementation of the Runtime interface
type SimpleRuntime struct {
	agents   map[string]agent.Agent
	channels map[string]chan *agent.Message
	mu       sync.RWMutex
	started  bool
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
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return fmt.Errorf("runtime already started")
	}
	r.started = true
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

	r.started = false
	return nil
}

// StartAgentsPhased starts all registered agents in dependency order.
// Agents are started in phases based on their dependencies:
//   - Phase 0: Agents with no dependencies
//   - Phase N: Agents whose dependencies are all in phases < N
//
// Within each phase, agents are started concurrently and the method waits
// for all of them to report Ready() before proceeding to the next phase.
func (r *SimpleRuntime) StartAgentsPhased(ctx context.Context, agentDefs map[string]agent.AgentDef) error {
	r.mu.RLock()
	started := r.started
	r.mu.RUnlock()

	if !started {
		return fmt.Errorf("runtime not started")
	}

	// Build dependency graph
	depGraph := graph.NewDependencyGraph()
	for name, def := range agentDefs {
		depGraph.AddNode(name, def.DependsOn)
	}

	// Get topological levels
	levels, err := depGraph.TopologicalLevels()
	if err != nil {
		return fmt.Errorf("dependency graph error: %w", err)
	}

	// Start each level in parallel, wait for Ready() before next level
	for levelIdx, level := range levels {
		log.Printf("[Runtime] Starting agent phase %d: %v", levelIdx, level)

		g, gctx := errgroup.WithContext(ctx)

		for _, name := range level {
			name := name // capture for goroutine

			g.Go(func() error {
				a, err := r.Get(name)
				if err != nil {
					return fmt.Errorf("agent %s not registered: %w", name, err)
				}

				// Start agent in goroutine (non-blocking)
				go func() {
					if err := a.Start(gctx); err != nil {
						log.Printf("[Runtime] Agent %s error: %v", name, err)
					}
				}()

				// Wait for agent to be Ready
				if err := r.waitForReady(gctx, a, 30*time.Second); err != nil {
					return fmt.Errorf("agent %s failed to become ready: %w", name, err)
				}

				log.Printf("[Runtime] Agent %s is ready", name)
				return nil
			})
		}

		// Wait for all agents in this level to be ready
		if err := g.Wait(); err != nil {
			return fmt.Errorf("phase %d startup failed: %w", levelIdx, err)
		}

		log.Printf("[Runtime] Phase %d complete, all agents ready", levelIdx)
	}

	return nil
}

// waitForReady polls until the agent is Ready() or the context/timeout expires.
func (r *SimpleRuntime) waitForReady(ctx context.Context, a agent.Agent, timeout time.Duration) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutCh:
			return fmt.Errorf("timeout after %v waiting for agent %s to be ready", timeout, a.Name())
		case <-ticker.C:
			if a.Ready() {
				return nil
			}
		}
	}
}
