package runtime

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/observability"
	pb "github.com/aixgo-dev/aixgo/proto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// DistributedRuntime provides distributed agent execution using gRPC.
// Agents can run in separate processes or on different machines.
type DistributedRuntime struct {
	localAgents  map[string]agent.Agent         // Agents running in this process
	remoteAgents map[string]*remoteAgentClient  // Remote agent connections
	channels     map[string]chan *agent.Message // Local message channels
	config       *RuntimeConfig
	mu           sync.RWMutex
	started      bool
	ctx          context.Context
	cancel       context.CancelFunc
	server       *grpc.Server
	listenAddr   string
	semaphore    chan struct{} // For limiting concurrent calls
}

// remoteAgentClient represents a connection to a remote agent
type remoteAgentClient struct {
	name   string
	addr   string
	conn   *grpc.ClientConn
	client pb.AgentServiceClient
}

// DistributedRuntimeConfig extends RuntimeConfig with distributed-specific options
type DistributedRuntimeConfig struct {
	*RuntimeConfig
	ListenAddr string // Address to listen for gRPC connections (e.g., ":50051")
}

// NewDistributedRuntime creates a new DistributedRuntime
func NewDistributedRuntime(listenAddr string, opts ...Option) *DistributedRuntime {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	var sem chan struct{}
	if cfg.MaxConcurrentCalls > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrentCalls)
	}

	return &DistributedRuntime{
		localAgents:  make(map[string]agent.Agent),
		remoteAgents: make(map[string]*remoteAgentClient),
		channels:     make(map[string]chan *agent.Message),
		config:       cfg,
		listenAddr:   listenAddr,
		semaphore:    sem,
	}
}

// Register registers a local agent with the runtime
func (r *DistributedRuntime) Register(a agent.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if _, exists := r.localAgents[name]; exists {
		return fmt.Errorf("%w: %s", ErrAgentAlreadyRegistered, name)
	}
	if _, exists := r.remoteAgents[name]; exists {
		return fmt.Errorf("%w: %s (registered as remote)", ErrAgentAlreadyRegistered, name)
	}

	r.localAgents[name] = a
	r.channels[name] = make(chan *agent.Message, r.config.ChannelBufferSize)

	return nil
}

// Connect establishes a connection to a remote agent
func (r *DistributedRuntime) Connect(name, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.localAgents[name]; exists {
		return fmt.Errorf("%w: %s (already registered as local)", ErrAgentAlreadyRegistered, name)
	}
	if _, exists := r.remoteAgents[name]; exists {
		return fmt.Errorf("%w: %s", ErrAgentAlreadyRegistered, name)
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to remote agent %s at %s: %w", name, addr, err)
	}

	r.remoteAgents[name] = &remoteAgentClient{
		name:   name,
		addr:   addr,
		conn:   conn,
		client: pb.NewAgentServiceClient(conn),
	}

	return nil
}

// Unregister removes an agent from the runtime
func (r *DistributedRuntime) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check local agents
	if _, exists := r.localAgents[name]; exists {
		close(r.channels[name])
		delete(r.channels, name)
		delete(r.localAgents, name)
		return nil
	}

	// Check remote agents
	if remote, exists := r.remoteAgents[name]; exists {
		_ = remote.conn.Close()
		delete(r.remoteAgents, name)
		return nil
	}

	return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
}

// Get retrieves a registered agent by name (local only)
func (r *DistributedRuntime) Get(name string) (agent.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, exists := r.localAgents[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}

	return a, nil
}

// List returns all registered agent names (local + remote)
func (r *DistributedRuntime) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.localAgents)+len(r.remoteAgents))
	for name := range r.localAgents {
		names = append(names, name)
	}
	for name := range r.remoteAgents {
		names = append(names, name)
	}

	return names
}

// Send sends a message to a target agent asynchronously
func (r *DistributedRuntime) Send(target string, msg *agent.Message) error {
	r.mu.RLock()

	// Check local agents
	if ch, exists := r.channels[target]; exists {
		r.mu.RUnlock()
		select {
		case ch <- msg:
			return nil
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout sending message to %s", target)
		}
	}

	// Check remote agents
	remote, exists := r.remoteAgents[target]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, target)
	}

	// Send to remote agent via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := remote.client.Send(ctx, &pb.SendRequest{
		Message: msg.Message,
	})

	return err
}

// Recv returns a channel to receive messages from a source agent
func (r *DistributedRuntime) Recv(source string) (<-chan *agent.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, exists := r.channels[source]
	if !exists {
		return nil, fmt.Errorf("%w: %s (Recv only works for local agents)", ErrAgentNotFound, source)
	}

	return ch, nil
}

// Call invokes an agent synchronously and waits for response
func (r *DistributedRuntime) Call(ctx context.Context, target string, input *agent.Message) (*agent.Message, error) {
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

	r.mu.RLock()

	// Try local agent first
	if a, exists := r.localAgents[target]; exists {
		r.mu.RUnlock()

		if !a.Ready() {
			return nil, fmt.Errorf("%w: %s", ErrAgentNotReady, target)
		}

		ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("runtime.call.%s", target),
			trace.WithAttributes(
				attribute.String("agent.name", target),
				attribute.String("agent.role", a.Role()),
				attribute.String("runtime.type", "local"),
			),
		)
		defer span.End()

		startTime := time.Now()
		result, err := a.Execute(ctx, input)
		duration := time.Since(startTime)

		if r.config.EnableMetrics {
			span.SetAttributes(
				attribute.Int64("execution.duration_ms", duration.Milliseconds()),
				attribute.Bool("execution.success", err == nil),
			)
		}

		return result, err
	}

	// Try remote agent
	remote, exists := r.remoteAgents[target]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, target)
	}

	// Call remote agent via gRPC
	ctx, span := observability.StartSpanWithOtel(ctx, fmt.Sprintf("runtime.call.%s", target),
		trace.WithAttributes(
			attribute.String("agent.name", target),
			attribute.String("runtime.type", "remote"),
			attribute.String("remote.addr", remote.addr),
		),
	)
	defer span.End()

	startTime := time.Now()
	resp, err := remote.client.Execute(ctx, &pb.ExecuteRequest{
		Input: input.Message,
	})
	duration := time.Since(startTime)

	if r.config.EnableMetrics {
		span.SetAttributes(
			attribute.Int64("execution.duration_ms", duration.Milliseconds()),
			attribute.Bool("execution.success", err == nil),
		)
	}

	if err != nil {
		return nil, err
	}

	return &agent.Message{Message: resp.Output}, nil
}

// CallParallel invokes multiple agents concurrently and returns all results
func (r *DistributedRuntime) CallParallel(ctx context.Context, targets []string, input *agent.Message) (map[string]*agent.Message, map[string]error) {
	results := make(map[string]*agent.Message)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx, span := observability.StartSpanWithOtel(ctx, "runtime.call_parallel",
		trace.WithAttributes(
			attribute.Int("agents.count", len(targets)),
			attribute.StringSlice("agents.names", targets),
		),
	)
	defer span.End()

	startTime := time.Now()

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
	duration := time.Since(startTime)

	if r.config.EnableMetrics {
		span.SetAttributes(
			attribute.Int64("execution.duration_ms", duration.Milliseconds()),
			attribute.Int("execution.success_count", len(results)),
			attribute.Int("execution.error_count", len(errors)),
		)
	}

	return results, errors
}

// Broadcast sends a message to all registered agents asynchronously
func (r *DistributedRuntime) Broadcast(msg *agent.Message) error {
	r.mu.RLock()
	targets := make([]string, 0, len(r.localAgents)+len(r.remoteAgents))
	for name := range r.localAgents {
		targets = append(targets, name)
	}
	for name := range r.remoteAgents {
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

// Start starts the runtime and gRPC server
func (r *DistributedRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return ErrRuntimeAlreadyStarted
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.started = true

	// Start gRPC server
	if r.listenAddr != "" {
		r.server = grpc.NewServer()
		pb.RegisterAgentServiceServer(r.server, &agentServiceServer{runtime: r})

		// TODO: Start listening in a goroutine
		// This requires implementing the gRPC service methods
	}

	return nil
}

// Stop gracefully shuts down the runtime
func (r *DistributedRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}

	r.cancel()

	// Stop gRPC server
	if r.server != nil {
		r.server.GracefulStop()
	}

	// Close remote connections
	for _, remote := range r.remoteAgents {
		_ = remote.conn.Close()
	}

	// Stop local agents
	agents := make([]agent.Agent, 0, len(r.localAgents))
	for _, a := range r.localAgents {
		agents = append(agents, a)
	}
	r.mu.Unlock()

	var wg sync.WaitGroup
	for _, a := range agents {
		wg.Add(1)
		go func(ag agent.Agent) {
			defer wg.Done()
			_ = ag.Stop(ctx)
		}(a)
	}

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

// agentServiceServer implements the gRPC AgentService
type agentServiceServer struct {
	pb.UnimplementedAgentServiceServer
	runtime *DistributedRuntime
}

func (s *agentServiceServer) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	// 1. Validate request
	if req.AgentName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "agent_name is required")
	}
	if req.Input == nil {
		return nil, status.Errorf(codes.InvalidArgument, "input is required")
	}

	// 2. Validate agent name format
	if !isValidAgentNameGRPC(req.AgentName) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent name format")
	}

	// 3. Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := s.runtime.Call(ctx, req.AgentName, &agent.Message{Message: req.Input})
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return nil, status.Errorf(codes.NotFound, "agent not found: %s", req.AgentName)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, status.Errorf(codes.DeadlineExceeded, "execution timeout")
		}
		return nil, status.Errorf(codes.Internal, "execution failed: %v", err)
	}

	return &pb.ExecuteResponse{Output: result.Message}, nil
}

func (s *agentServiceServer) Send(ctx context.Context, req *pb.SendRequest) (*pb.SendResponse, error) {
	// Validate
	if req.Target == "" {
		return nil, status.Errorf(codes.InvalidArgument, "target is required")
	}
	if req.Message == nil {
		return nil, status.Errorf(codes.InvalidArgument, "message is required")
	}

	if !isValidAgentNameGRPC(req.Target) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid target name")
	}

	// Send message
	err := s.runtime.Send(req.Target, &agent.Message{Message: req.Message})
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return nil, status.Errorf(codes.NotFound, "agent not found: %s", req.Target)
		}
		return nil, status.Errorf(codes.Internal, "send failed: %v", err)
	}

	return &pb.SendResponse{Success: true}, nil
}

// isValidAgentNameGRPC validates agent name for gRPC requests
func isValidAgentNameGRPC(name string) bool {
	// Only allow lowercase alphanumeric, hyphens, underscores, max 64 chars
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9_-]{0,63}$`, name)
	return matched
}
