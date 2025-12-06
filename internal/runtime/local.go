package runtime

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// LocalRuntime provides in-process agent execution using Go channels.
// All agents run in the same process, ideal for single-binary deployments.
type LocalRuntime struct {
	agents       map[string]agent.Agent
	channels     map[string]chan *agent.Message
	config       *RuntimeConfig
	mu           sync.RWMutex
	started      bool
	ctx          context.Context
	cancel       context.CancelFunc
	semaphore    chan struct{} // For limiting concurrent calls
	messagesSent uint64        // Atomic counter for metrics
}

// NewLocalRuntime creates a new LocalRuntime with the given options
func NewLocalRuntime(opts ...Option) *LocalRuntime {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var sem chan struct{}
	if cfg.MaxConcurrentCalls > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrentCalls)
	}

	return &LocalRuntime{
		agents:    make(map[string]agent.Agent),
		channels:  make(map[string]chan *agent.Message),
		config:    cfg,
		semaphore: sem,
	}
}

// Register registers an agent with the runtime
func (r *LocalRuntime) Register(a agent.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("%w: %s", ErrAgentAlreadyRegistered, name)
	}

	r.agents[name] = a
	r.channels[name] = make(chan *agent.Message, r.config.ChannelBufferSize)

	return nil
}

// Unregister removes an agent from the runtime
func (r *LocalRuntime) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}

	// Close the channel and remove agent
	close(r.channels[name])
	delete(r.channels, name)
	delete(r.agents, name)

	return nil
}

// Get retrieves a registered agent by name
func (r *LocalRuntime) Get(name string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}

	return a, nil
}

// List returns all registered agent names
func (r *LocalRuntime) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}

	return names
}

// Send sends a message to a target agent asynchronously
func (r *LocalRuntime) Send(target string, msg *agent.Message) error {
	r.mu.RLock()
	ch, exists := r.channels[target]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, target)
	}

	// Warn if channel is >80% full
	utilization := len(ch) * 100 / cap(ch)
	if utilization > 80 {
		log.Printf("WARNING: Channel %s is %d%% full (%d/%d messages)",
			target, utilization, len(ch), cap(ch))
	}

	select {
	case ch <- msg:
		atomic.AddUint64(&r.messagesSent, 1)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending message to %s (channel full)", target)
	}
}

// GetChannelStats returns statistics for a channel
func (r *LocalRuntime) GetChannelStats(name string) (capacity, length int, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, exists := r.channels[name]
	if !exists {
		return 0, 0, ErrAgentNotFound
	}

	return cap(ch), len(ch), nil
}

// Recv returns a channel to receive messages from a source agent
func (r *LocalRuntime) Recv(source string) (<-chan *agent.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, exists := r.channels[source]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, source)
	}

	return ch, nil
}

// Call invokes an agent synchronously and waits for response
func (r *LocalRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
	if !r.started {
		return nil, ErrRuntimeNotStarted
	}

	// Acquire semaphore if concurrency limiting is enabled
	if r.semaphore != nil {
		select {
		case r.semaphore <- struct{}{}:
			defer func() { <-r.semaphore }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Get agent
	a, err := r.Get(target)
	if err != nil {
		return nil, err
	}

	// Check if agent is ready
	if !a.Ready() {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotReady, target)
	}

	// Create span for observability
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("runtime.call.%s", target),
		trace.WithAttributes(
			attribute.String("agent.name", target),
			attribute.String("agent.role", a.Role()),
		),
	)
	defer span.End()

	// Execute agent
	startTime := time.Now()
	result, err := a.Execute(ctx, input)
	duration := time.Since(startTime)

	// Record metrics
	if r.config.EnableMetrics {
		span.SetAttributes(
			attribute.Int64("execution.duration_ms", duration.Milliseconds()),
			attribute.Bool("execution.success", err == nil),
		)
	}

	return result, err
}

// CallParallel invokes multiple agents concurrently and returns all results
func (r *LocalRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, map[string]error) {
	results := make(map[string]*agent.Message)
	errors := make(map[string]error)
	var mu sync.Mutex

	ctx, span := observability.StartSpanWithOtel(ctx, "runtime.call_parallel",
		trace.WithAttributes(
			attribute.Int("agents.count", len(targets)),
			attribute.StringSlice("agents.names", targets),
		),
	)
	defer span.End()

	startTime := time.Now()

	// Use semaphore for concurrency limiting
	maxWorkers := 8 // Default worker pool size
	if r.config.MaxConcurrentCalls > 0 {
		maxWorkers = r.config.MaxConcurrentCalls
	}

	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)

		go func(t string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				errors[t] = ctx.Err()
				mu.Unlock()
				return
			}

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
	duration := time.Since(startTime)

	if r.config.EnableMetrics {
		span.SetAttributes(
			attribute.Int64("execution.duration_ms", duration.Milliseconds()),
			attribute.Int("execution.success_count", len(results)),
			attribute.Int("execution.error_count", len(errors)),
			attribute.Int("execution.max_workers", maxWorkers),
		)
	}

	return results, errors
}

// Broadcast sends a message to all registered agents asynchronously
func (r *LocalRuntime) Broadcast(msg *agent.Message) error {
	r.mu.RLock()
	targets := make([]string, 0, len(r.agents))
	for name := range r.agents {
		targets = append(targets, name)
	}
	r.mu.RUnlock()

	var firstErr error
	for _, target := range targets {
		if err := r.Send(target, msg); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Start starts the runtime (no-op for LocalRuntime, agents start individually)
func (r *LocalRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return ErrRuntimeAlreadyStarted
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.started = true

	return nil
}

// Stop gracefully shuts down the runtime
func (r *LocalRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}

	r.cancel()
	agents := make([]agent.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agents = append(agents, a)
	}
	r.mu.Unlock()

	// Stop all agents concurrently
	var wg sync.WaitGroup
	for _, a := range agents {
		wg.Add(1)
		go func(ag agent.Agent) {
			defer wg.Done()
			_ = ag.Stop(ctx)
		}(a)
	}

	// Wait for all agents to stop with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.mu.Lock()
		r.started = false
		r.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
