package aixgo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	pubagent "github.com/aixgo-dev/aixgo/agent"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/graph"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/session"
	pb "github.com/aixgo-dev/aixgo/proto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// Runtime errors
var (
	// ErrAgentNotFound is returned when an agent is not registered
	ErrAgentNotFound = errors.New("agent not found")

	// ErrAgentAlreadyRegistered is returned when trying to register an agent with a duplicate name
	ErrAgentAlreadyRegistered = errors.New("agent already registered")

	// ErrAgentNotReady is returned when trying to execute an agent that is not ready
	ErrAgentNotReady = errors.New("agent not ready")

	// ErrRuntimeNotStarted is returned when trying to use a runtime that hasn't been started
	ErrRuntimeNotStarted = errors.New("runtime not started")

	// ErrRuntimeAlreadyStarted is returned when trying to start an already running runtime
	ErrRuntimeAlreadyStarted = errors.New("runtime already started")

	// ErrSessionManagerNotConfigured is returned when calling session methods without a session manager
	ErrSessionManagerNotConfigured = errors.New("session manager not configured")
)

// RuntimeConfig contains configuration options for creating a runtime
type RuntimeConfig struct {
	// ChannelBufferSize sets the buffer size for message channels
	// Default: 100
	ChannelBufferSize int

	// MaxConcurrentCalls limits parallel agent executions (0 = unlimited)
	// Default: 0 (unlimited)
	MaxConcurrentCalls int

	// EnableMetrics enables runtime performance metrics collection
	// Default: false (for backwards compatibility)
	EnableMetrics bool

	// EnableTracing enables OpenTelemetry tracing
	// Default: false (for backwards compatibility)
	EnableTracing bool

	// AgentStartTimeout is the maximum time to wait for an agent to become ready
	// Default: 30 seconds
	AgentStartTimeout time.Duration

	// SendTimeout is the timeout for Send operations
	// Default: 5 seconds
	SendTimeout time.Duration

	// ChannelFullWarningThreshold triggers a warning when channel utilization exceeds this percentage
	// Default: 80
	ChannelFullWarningThreshold int
}

// DefaultRuntimeConfig returns a RuntimeConfig with sensible defaults
func DefaultRuntimeConfig() *RuntimeConfig {
	return &RuntimeConfig{
		ChannelBufferSize:           100,
		MaxConcurrentCalls:          0,
		EnableMetrics:               false,
		EnableTracing:               false,
		AgentStartTimeout:           30 * time.Second,
		SendTimeout:                 5 * time.Second,
		ChannelFullWarningThreshold: 80,
	}
}

// RuntimeOption is a functional option for configuring a runtime
type RuntimeOption func(*RuntimeConfig)

// WithChannelBufferSize sets the channel buffer size
func WithChannelBufferSize(size int) RuntimeOption {
	return func(cfg *RuntimeConfig) {
		if size > 0 {
			cfg.ChannelBufferSize = size
		}
	}
}

// WithMaxConcurrentCalls sets the maximum number of concurrent agent calls
func WithMaxConcurrentCalls(max int) RuntimeOption {
	return func(cfg *RuntimeConfig) {
		cfg.MaxConcurrentCalls = max
	}
}

// WithMetrics enables or disables metrics collection
func WithMetrics(enabled bool) RuntimeOption {
	return func(cfg *RuntimeConfig) {
		cfg.EnableMetrics = enabled
	}
}

// WithTracing enables or disables OpenTelemetry tracing
func WithTracing(enabled bool) RuntimeOption {
	return func(cfg *RuntimeConfig) {
		cfg.EnableTracing = enabled
	}
}

// WithAgentStartTimeout sets the timeout for waiting for agents to become ready
func WithAgentStartTimeout(timeout time.Duration) RuntimeOption {
	return func(cfg *RuntimeConfig) {
		cfg.AgentStartTimeout = timeout
	}
}

// WithSendTimeout sets the timeout for Send operations
func WithSendTimeout(timeout time.Duration) RuntimeOption {
	return func(cfg *RuntimeConfig) {
		cfg.SendTimeout = timeout
	}
}

// Runtime is the unified in-memory runtime for agent orchestration.
// It provides:
//   - Agent registration and lifecycle management
//   - Synchronous (Call) and asynchronous (Send/Recv) messaging
//   - Session persistence and session-aware execution
//   - Optional observability (metrics and tracing)
//   - Configurable concurrency limits
//
// This is the recommended runtime for single-process deployments.
// For multi-node deployments, use DistributedRuntime.
type Runtime struct {
	agents         map[string]agent.Agent
	channels       map[string]chan *agent.Message
	sessionManager session.Manager
	config         *RuntimeConfig
	mu             sync.RWMutex
	started        bool
	ctx            context.Context
	cancel         context.CancelFunc
	semaphore      chan struct{} // For limiting concurrent calls
	messagesSent   uint64        // Atomic counter for metrics
}

// NewRuntime creates a new Runtime with the given options.
//
// Example:
//
//	// Basic runtime (zero-config)
//	rt := aixgo.NewRuntime()
//
//	// With observability
//	rt := aixgo.NewRuntime(
//	    aixgo.WithMetrics(true),
//	    aixgo.WithTracing(true),
//	)
//
//	// With concurrency limits
//	rt := aixgo.NewRuntime(
//	    aixgo.WithMaxConcurrentCalls(10),
//	)
func NewRuntime(opts ...RuntimeOption) *Runtime {
	cfg := DefaultRuntimeConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var sem chan struct{}
	if cfg.MaxConcurrentCalls > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrentCalls)
	}

	return &Runtime{
		agents:    make(map[string]agent.Agent),
		channels:  make(map[string]chan *agent.Message),
		config:    cfg,
		semaphore: sem,
	}
}

// Config returns a copy of the runtime configuration.
func (r *Runtime) Config() RuntimeConfig {
	return *r.config
}

// Register registers an agent with the runtime
func (r *Runtime) Register(a agent.Agent) error {
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
func (r *Runtime) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[name]; !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}

	delete(r.agents, name)
	if ch, exists := r.channels[name]; exists {
		close(ch)
		delete(r.channels, name)
	}
	return nil
}

// Get retrieves a registered agent by name
func (r *Runtime) Get(name string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, exists := r.agents[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	return a, nil
}

// List returns all registered agent names
func (r *Runtime) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Send sends a message to a target agent asynchronously.
// If the target channel doesn't exist, it will be created.
// Returns an error if the channel is full after the send timeout.
func (r *Runtime) Send(target string, msg *agent.Message) error {
	r.mu.RLock()
	ch, ok := r.channels[target]
	r.mu.RUnlock()

	if !ok {
		// Create channel if it doesn't exist
		r.mu.Lock()
		if _, exists := r.channels[target]; !exists {
			r.channels[target] = make(chan *agent.Message, r.config.ChannelBufferSize)
		}
		ch = r.channels[target]
		r.mu.Unlock()
	}

	// Warn if channel is getting full
	if r.config.EnableMetrics && cap(ch) > 0 {
		utilization := len(ch) * 100 / cap(ch)
		if utilization > r.config.ChannelFullWarningThreshold {
			log.Printf("[Runtime] WARNING: Channel %s is %d%% full (%d/%d messages)",
				target, utilization, len(ch), cap(ch))
		}
	}

	select {
	case ch <- msg:
		atomic.AddUint64(&r.messagesSent, 1)
		return nil
	case <-time.After(r.config.SendTimeout):
		return fmt.Errorf("timeout sending message to %s (channel full)", target)
	}
}

// Recv returns a channel to receive messages from a source agent.
// If the source channel doesn't exist, it will be created.
func (r *Runtime) Recv(source string) (<-chan *agent.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.channels[source]; !ok {
		r.channels[source] = make(chan *agent.Message, r.config.ChannelBufferSize)
	}

	return r.channels[source], nil
}

// Broadcast sends a message to all registered agents asynchronously.
// Returns the first error encountered, if any.
func (r *Runtime) Broadcast(msg *agent.Message) error {
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

// Call invokes an agent synchronously and waits for response.
// If tracing is enabled, this creates an OpenTelemetry span.
func (r *Runtime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
	r.mu.RLock()
	started := r.started
	r.mu.RUnlock()

	if !started {
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

	// Create span for observability (if enabled)
	if r.config.EnableTracing {
		var span trace.Span
		ctx, span = observability.StartSpanWithOtel(ctx, fmt.Sprintf("runtime.call.%s", target),
			trace.WithAttributes(
				attribute.String("agent.name", target),
				attribute.String("agent.role", a.Role()),
			),
		)
		defer span.End()
	}

	// Execute agent
	startTime := time.Now()
	result, err := a.Execute(ctx, input)
	duration := time.Since(startTime)

	// Record metrics (if enabled)
	if r.config.EnableMetrics && r.config.EnableTracing {
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.SetAttributes(
				attribute.Int64("execution.duration_ms", duration.Milliseconds()),
				attribute.Bool("execution.success", err == nil),
			)
		}
	}

	return result, err
}

// CallParallel invokes multiple agents concurrently and returns all results.
// The number of concurrent calls is limited by MaxConcurrentCalls if configured.
func (r *Runtime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, map[string]error) {
	results := make(map[string]*agent.Message)
	errs := make(map[string]error)
	var mu sync.Mutex

	// Create span for observability (if enabled)
	if r.config.EnableTracing {
		var span trace.Span
		ctx, span = observability.StartSpanWithOtel(ctx, "runtime.call_parallel",
			trace.WithAttributes(
				attribute.Int("agents.count", len(targets)),
				attribute.StringSlice("agents.names", targets),
			),
		)
		defer span.End()
	}

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
				errs[t] = ctx.Err()
				mu.Unlock()
				return
			}

			result, err := r.Call(ctx, t, input)

			mu.Lock()
			if err != nil {
				errs[t] = err
			} else {
				results[t] = result
			}
			mu.Unlock()
		}(target)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Record metrics (if enabled)
	if r.config.EnableMetrics && r.config.EnableTracing {
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.SetAttributes(
				attribute.Int64("execution.duration_ms", duration.Milliseconds()),
				attribute.Int("execution.success_count", len(results)),
				attribute.Int("execution.error_count", len(errs)),
				attribute.Int("execution.max_workers", maxWorkers),
			)
		}
	}

	return results, errs
}

// Start starts the runtime.
// Must be called before Call, CallParallel, or StartAgentsPhased.
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return ErrRuntimeAlreadyStarted
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.started = true
	return nil
}

// Stop gracefully shuts down the runtime.
// All agents are stopped and all channels are closed.
func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}

	if r.cancel != nil {
		r.cancel()
	}

	agents := make([]agent.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		agents = append(agents, a)
	}

	// Close all channels
	for _, ch := range r.channels {
		close(ch)
	}
	r.channels = make(map[string]chan *agent.Message)
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

// StartAgentsPhased starts all registered agents in dependency order.
// Agents are started in phases based on their dependencies:
//   - Phase 0: Agents with no dependencies
//   - Phase N: Agents whose dependencies are all in phases < N
//
// Within each phase, agents are started concurrently and the method waits
// for all of them to report Ready() before proceeding to the next phase.
func (r *Runtime) StartAgentsPhased(ctx context.Context, agentDefs map[string]agent.AgentDef) error {
	r.mu.RLock()
	started := r.started
	r.mu.RUnlock()

	if !started {
		return ErrRuntimeNotStarted
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
				timeout := r.config.AgentStartTimeout
				if timeout == 0 {
					timeout = 30 * time.Second
				}
				if err := r.waitForReady(gctx, a, timeout); err != nil {
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
func (r *Runtime) waitForReady(ctx context.Context, a agent.Agent, timeout time.Duration) error {
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

// --- Session Support ---

// SetSessionManager sets the session manager for the runtime.
// This enables session-aware agent calls via CallWithSession.
func (r *Runtime) SetSessionManager(sm session.Manager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionManager = sm
}

// SessionManager returns the session manager, if configured.
func (r *Runtime) SessionManager() session.Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionManager
}

// CallWithSession invokes an agent with session context.
// The input message is appended to the session before execution,
// and the result is appended after execution.
//
// If the agent implements session.SessionAwareAgent, it will receive
// the full conversation history during execution. Otherwise, the
// session is still used for persistence but the agent won't have
// direct access to history.
func (r *Runtime) CallWithSession(
	ctx context.Context,
	target string,
	input *pubagent.Message,
	sessionID string,
) (*pubagent.Message, error) {
	if r.sessionManager == nil {
		return nil, ErrSessionManagerNotConfigured
	}

	sess, err := r.sessionManager.Get(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Append input to session
	if err := sess.AppendMessage(ctx, input); err != nil {
		return nil, fmt.Errorf("append input: %w", err)
	}

	// Add session to context
	ctx = session.ContextWithSession(ctx, sess)

	// Get the agent
	r.mu.RLock()
	a, exists := r.agents[target]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, target)
	}

	if !a.Ready() {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotReady, target)
	}

	var pubResult *pubagent.Message

	// Check if agent supports session-aware execution
	if sessionAware, ok := a.(sessionAwareAgent); ok {
		// Use session-aware execution path
		result, err := sessionAware.ExecuteWithSession(ctx, input, sess)
		if err != nil {
			return nil, err
		}
		pubResult = result
	} else {
		// Fall back to standard execution
		internalInput := &agent.Message{
			Message: toProtoMessage(input),
		}

		result, err := a.Execute(ctx, internalInput)
		if err != nil {
			return nil, err
		}

		pubResult = fromProtoMessage(result)
	}

	// Append result to session
	if err := sess.AppendMessage(ctx, pubResult); err != nil {
		return nil, fmt.Errorf("append result: %w", err)
	}

	return pubResult, nil
}

// --- Metrics ---

// GetChannelStats returns statistics for a channel.
// Returns capacity, current length, and an error if the channel doesn't exist.
func (r *Runtime) GetChannelStats(name string) (capacity, length int, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, exists := r.channels[name]
	if !exists {
		return 0, 0, ErrAgentNotFound
	}

	return cap(ch), len(ch), nil
}

// MessagesSent returns the total number of messages sent via Send().
func (r *Runtime) MessagesSent() uint64 {
	return atomic.LoadUint64(&r.messagesSent)
}

// --- Internal helpers ---

// sessionAwareAgent is the interface for agents that support session-aware execution.
// This is defined locally to avoid import cycles.
type sessionAwareAgent interface {
	ExecuteWithSession(ctx context.Context, input *pubagent.Message, sess session.Session) (*pubagent.Message, error)
}

// toProtoMessage converts a public agent.Message to internal proto.Message
func toProtoMessage(msg *pubagent.Message) *pb.Message {
	if msg == nil {
		return nil
	}
	return &pb.Message{
		Id:        msg.ID,
		Type:      msg.Type,
		Payload:   msg.Payload,
		Timestamp: msg.Timestamp,
		Metadata:  msg.Metadata,
	}
}

// fromProtoMessage converts an internal agent.Message to public agent.Message
func fromProtoMessage(msg *agent.Message) *pubagent.Message {
	if msg == nil || msg.Message == nil {
		return nil
	}
	return &pubagent.Message{
		ID:        msg.Id,
		Type:      msg.Type,
		Payload:   msg.Payload,
		Timestamp: msg.Timestamp,
		Metadata:  msg.Metadata,
	}
}
