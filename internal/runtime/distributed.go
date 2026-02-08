package runtime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	publicAgent "github.com/aixgo-dev/aixgo/agent"
	"github.com/aixgo-dev/aixgo/internal/agent"
	"github.com/aixgo-dev/aixgo/internal/graph"
	"github.com/aixgo-dev/aixgo/internal/observability"
	"github.com/aixgo-dev/aixgo/pkg/session"
	pb "github.com/aixgo-dev/aixgo/proto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// DistributedRuntime provides distributed agent execution using gRPC.
// Agents can run in separate processes or on different machines.
type DistributedRuntime struct {
	localAgents    map[string]agent.Agent         // Agents running in this process
	remoteAgents   map[string]*remoteAgentClient  // Remote agent connections
	channels       map[string]chan *agent.Message // Local message channels
	config         *RuntimeConfig
	tlsConfig      *TLSConfig      // TLS configuration for secure connections
	sessionManager session.Manager // Session manager for persistence
	mu             sync.RWMutex
	started        bool
	ctx            context.Context
	cancel         context.CancelFunc
	server         *grpc.Server
	listener       net.Listener  // gRPC listener
	listenAddr     string
	semaphore      chan struct{} // For limiting concurrent calls
	messagesSent   uint64        // Atomic counter for metrics
}

// TLSConfig holds TLS configuration for gRPC connections.
type TLSConfig struct {
	// Enabled turns on TLS encryption.
	Enabled bool
	// CertFile is the path to the server certificate.
	CertFile string
	// KeyFile is the path to the server private key.
	KeyFile string
	// CAFile is the path to the CA certificate (for mTLS).
	CAFile string
	// ServerName is used for SNI verification.
	ServerName string
	// InsecureSkipVerify skips certificate verification (development only).
	// Warning: This logs a security warning. Do not use in production.
	InsecureSkipVerify bool
	// ExternalTLS indicates TLS is handled by a service mesh (Istio, Linkerd, etc.).
	// When true, app-level TLS is disabled entirely since the mesh sidecar handles
	// encryption. This takes precedence over other TLS settings.
	ExternalTLS bool
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
	ListenAddr string     // Address to listen for gRPC connections (e.g., ":50051")
	TLS        *TLSConfig // TLS configuration for secure connections
}

// DistributedOption configures a DistributedRuntime.
type DistributedOption func(*DistributedRuntime)

// WithTLS configures TLS for secure gRPC connections.
func WithTLS(cfg *TLSConfig) DistributedOption {
	return func(r *DistributedRuntime) {
		r.tlsConfig = cfg
	}
}

// WithDistributedSessionManager sets the session manager for the distributed runtime.
func WithDistributedSessionManager(sm session.Manager) DistributedOption {
	return func(r *DistributedRuntime) {
		r.sessionManager = sm
	}
}

// NewDistributedRuntime creates a new DistributedRuntime.
// The listenAddr is the address to listen for incoming gRPC connections (e.g., ":50051").
// Use DistributedOption to configure TLS and session management.
func NewDistributedRuntime(listenAddr string, opts ...any) *DistributedRuntime {
	cfg := DefaultConfig()

	var sem chan struct{}
	if cfg.MaxConcurrentCalls > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrentCalls)
	}

	r := &DistributedRuntime{
		localAgents:  make(map[string]agent.Agent),
		remoteAgents: make(map[string]*remoteAgentClient),
		channels:     make(map[string]chan *agent.Message),
		config:       cfg,
		listenAddr:   listenAddr,
		semaphore:    sem,
	}

	// Apply options (supports both Option and DistributedOption)
	for _, opt := range opts {
		switch o := opt.(type) {
		case Option:
			o(r.config)
		case DistributedOption:
			o(r)
		}
	}

	return r
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

// Connect establishes a connection to a remote agent.
// Uses TLS if configured, otherwise falls back to insecure connection.
func (r *DistributedRuntime) Connect(name, addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.localAgents[name]; exists {
		return fmt.Errorf("%w: %s (already registered as local)", ErrAgentAlreadyRegistered, name)
	}
	if _, exists := r.remoteAgents[name]; exists {
		return fmt.Errorf("%w: %s", ErrAgentAlreadyRegistered, name)
	}

	// Build dial options
	dialOpts, err := r.buildDialOptions()
	if err != nil {
		return fmt.Errorf("failed to build dial options: %w", err)
	}

	// Create gRPC connection
	conn, err := grpc.NewClient(addr, dialOpts...)
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

// buildDialOptions creates gRPC dial options based on TLS configuration.
func (r *DistributedRuntime) buildDialOptions() ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	// ExternalTLS means TLS is handled by service mesh (Istio, Linkerd, etc.)
	// Use plaintext transport since the sidecar handles encryption
	if r.tlsConfig != nil && r.tlsConfig.ExternalTLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return opts, nil
	}

	if r.tlsConfig != nil && r.tlsConfig.Enabled {
		// SECURITY: Prevent InsecureSkipVerify in production environments
		// This enforces certificate verification when running in production
		// Empty/unset ENVIRONMENT is treated as production (fail-safe)
		if r.tlsConfig.InsecureSkipVerify {
			env := strings.ToLower(os.Getenv("ENVIRONMENT"))
			// Only allow InsecureSkipVerify in explicit non-production environments
			allowedNonProdEnvs := map[string]bool{
				"development": true,
				"dev":         true,
				"staging":     true,
				"local":       true,
				"test":        true,
			}
			if !allowedNonProdEnvs[env] {
				return nil, fmt.Errorf("SECURITY: InsecureSkipVerify cannot be enabled in production environment (ENVIRONMENT=%q). "+
					"Set ENVIRONMENT to 'development', 'dev', 'staging', 'local', or 'test' to allow insecure TLS", env)
			}

			// Log warning for non-production environments
			log.Printf("[DistributedRuntime] WARNING: TLS certificate verification is disabled (InsecureSkipVerify=true). "+
				"This is a security risk and should NEVER be used in production. "+
				"Connections are vulnerable to man-in-the-middle attacks. "+
				"Current ENVIRONMENT=%s", env)
		}

		tlsCfg := &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: r.tlsConfig.InsecureSkipVerify, // #nosec G402 -- intentionally configurable for dev/test; blocked in production by env check above
		}

		// Set server name for SNI
		if r.tlsConfig.ServerName != "" {
			tlsCfg.ServerName = r.tlsConfig.ServerName
		}

		// Load CA certificate if provided
		if r.tlsConfig.CAFile != "" {
			caData, err := os.ReadFile(r.tlsConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caData) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsCfg.RootCAs = caPool
		}

		// Load client certificate for mTLS if provided
		if r.tlsConfig.CertFile != "" && r.tlsConfig.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(r.tlsConfig.CertFile, r.tlsConfig.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts, nil
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

		// Warn if channel is getting full
		if r.config.EnableMetrics && cap(ch) > 0 {
			utilization := len(ch) * 100 / cap(ch)
			if utilization > r.config.ChannelFullWarningThreshold {
				log.Printf("[DistributedRuntime] WARNING: Channel %s is %d%% full (%d/%d messages)",
					target, utilization, len(ch), cap(ch))
			}
		}

		timeout := r.config.SendTimeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		select {
		case ch <- msg:
			atomic.AddUint64(&r.messagesSent, 1)
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("timeout sending message to %s (channel full)", target)
		}
	}

	// Check remote agents
	remote, exists := r.remoteAgents[target]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, target)
	}

	// Send to remote agent via gRPC
	timeout := r.config.SendTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err := remote.client.Send(ctx, &pb.SendRequest{
		Message: msg.Message,
	})

	if err == nil {
		atomic.AddUint64(&r.messagesSent, 1)
	}

	return err
}

// Recv returns a channel to receive messages from a source agent.
// For local agents, returns the local channel directly.
// For remote agents, establishes a gRPC streaming connection.
func (r *DistributedRuntime) Recv(source string) (<-chan *agent.Message, error) {
	r.mu.RLock()

	// Check local agents first
	if ch, exists := r.channels[source]; exists {
		r.mu.RUnlock()
		return ch, nil
	}

	// Check remote agents
	remote, exists := r.remoteAgents[source]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, source)
	}

	// Create streaming connection to remote agent
	return r.remoteRecv(remote, source)
}

// remoteRecv establishes a streaming connection to a remote agent.
func (r *DistributedRuntime) remoteRecv(remote *remoteAgentClient, source string) (<-chan *agent.Message, error) {
	if r.ctx == nil {
		return nil, errors.New("runtime not started: context is nil")
	}

	stream, err := remote.client.Listen(r.ctx, &pb.ListenRequest{AgentName: source})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to remote agent stream: %w", err)
	}

	ch := make(chan *agent.Message, r.config.ChannelBufferSize)

	go func() {
		defer close(ch)
		for {
			resp, err := stream.Recv()
			if err != nil {
				// Stream closed or error
				log.Printf("[DistributedRuntime] Stream from %s closed: %v", source, err)
				return
			}

			select {
			case ch <- &agent.Message{Message: resp.Message}:
			case <-r.ctx.Done():
				return
			}
		}
	}()

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

// Start starts the runtime and gRPC server.
// The gRPC server will listen on the address provided during construction.
func (r *DistributedRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return ErrRuntimeAlreadyStarted
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	// Start gRPC server if listen address is configured
	if r.listenAddr != "" {
		// Create listener
		lis, err := net.Listen("tcp", r.listenAddr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", r.listenAddr, err)
		}
		r.listener = lis

		// Configure server options (TLS if enabled)
		serverOpts, err := r.buildServerOptions()
		if err != nil {
			_ = lis.Close()
			return fmt.Errorf("failed to configure server: %w", err)
		}

		r.server = grpc.NewServer(serverOpts...)
		pb.RegisterAgentServiceServer(r.server, &agentServiceServer{runtime: r})

		// Start gRPC server in goroutine
		go func() {
			log.Printf("[DistributedRuntime] gRPC server listening on %s", r.listenAddr)
			if err := r.server.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				log.Printf("[DistributedRuntime] gRPC server error: %v", err)
			}
		}()
	}

	r.started = true
	return nil
}

// buildServerOptions creates gRPC server options based on TLS configuration.
func (r *DistributedRuntime) buildServerOptions() ([]grpc.ServerOption, error) {
	var opts []grpc.ServerOption

	// ExternalTLS means TLS is handled by service mesh - no server-side TLS needed
	if r.tlsConfig != nil && r.tlsConfig.ExternalTLS {
		return opts, nil
	}

	if r.tlsConfig != nil && r.tlsConfig.Enabled {
		// Load server certificate
		cert, err := tls.LoadX509KeyPair(r.tlsConfig.CertFile, r.tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load server certificate: %w", err)
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		// Load CA for mTLS if provided
		if r.tlsConfig.CAFile != "" {
			caData, err := os.ReadFile(r.tlsConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caData) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsCfg.ClientCAs = caPool
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		}

		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	return opts, nil
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

// SetSessionManager sets the session manager for this runtime.
func (r *DistributedRuntime) SetSessionManager(sm session.Manager) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionManager = sm
}

// SessionManager returns the configured session manager.
func (r *DistributedRuntime) SessionManager() session.Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionManager
}

// CallWithSession invokes an agent with session context.
// The input message is appended to the session before execution,
// and the result is appended after execution.
//
// If the agent implements sessionAwareAgent, it will receive
// the full conversation history during execution.
func (r *DistributedRuntime) CallWithSession(
	ctx context.Context,
	target string,
	input *publicAgent.Message,
	sessionID string,
) (*publicAgent.Message, error) {
	if r.sessionManager == nil {
		return nil, errors.New("session manager not configured")
	}

	// Get the session
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

	// Get the agent (local only for session-aware execution)
	r.mu.RLock()
	a, isLocal := r.localAgents[target]
	r.mu.RUnlock()

	var result *publicAgent.Message

	if isLocal {
		if !a.Ready() {
			return nil, fmt.Errorf("%w: %s", ErrAgentNotReady, target)
		}

		// Check if agent supports session-aware execution
		if sessionAware, ok := a.(sessionAwareAgent); ok {
			// Use session-aware execution path
			result, err = sessionAware.ExecuteWithSession(ctx, input, sess)
			if err != nil {
				return nil, err
			}
		} else {
			// Fall back to standard execution
			internalInput := publicToInternalMessage(input)
			internalResult, err := a.Execute(ctx, internalInput)
			if err != nil {
				return nil, err
			}
			result = internalToPublicMessage(internalResult)
		}
	} else {
		// Remote agent - use standard Call path
		internalInput := publicToInternalMessage(input)
		internalResult, err := r.Call(ctx, target, internalInput)
		if err != nil {
			return nil, err
		}
		result = internalToPublicMessage(internalResult)
	}

	// Append result to session
	if err := sess.AppendMessage(ctx, result); err != nil {
		return nil, fmt.Errorf("append result: %w", err)
	}

	return result, nil
}

// sessionAwareAgent is the interface for agents that support session-aware execution.
type sessionAwareAgent interface {
	ExecuteWithSession(ctx context.Context, input *publicAgent.Message, sess session.Session) (*publicAgent.Message, error)
}

// internalToPublicMessage converts an internal agent.Message to a public agent.Message.
func internalToPublicMessage(msg *agent.Message) *publicAgent.Message {
	if msg == nil || msg.Message == nil {
		return nil
	}

	// Copy metadata if present, otherwise create empty map
	metadata := make(map[string]interface{})
	if msg.Metadata != nil {
		for k, v := range msg.Metadata {
			metadata[k] = v
		}
	}

	return &publicAgent.Message{
		ID:        msg.Id,
		Type:      msg.Type,
		Payload:   msg.Payload,
		Timestamp: msg.Timestamp,
		Metadata:  metadata,
	}
}

// publicToInternalMessage converts a public agent.Message to an internal agent.Message.
func publicToInternalMessage(msg *publicAgent.Message) *agent.Message {
	if msg == nil {
		return nil
	}
	return &agent.Message{
		Message: &pb.Message{
			Id:        msg.ID,
			Type:      msg.Type,
			Payload:   msg.Payload,
			Timestamp: msg.Timestamp,
		},
	}
}

// ListenAddr returns the address the gRPC server is listening on.
// Returns empty string if server is not started.
func (r *DistributedRuntime) ListenAddr() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.listener != nil {
		return r.listener.Addr().String()
	}
	return r.listenAddr
}

// --- Metrics ---

// Config returns a copy of the runtime configuration.
func (r *DistributedRuntime) Config() RuntimeConfig {
	return *r.config
}

// GetChannelStats returns statistics for a channel.
// Returns capacity, current length, and an error if the channel doesn't exist.
func (r *DistributedRuntime) GetChannelStats(name string) (capacity, length int, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, exists := r.channels[name]
	if !exists {
		return 0, 0, ErrAgentNotFound
	}

	return cap(ch), len(ch), nil
}

// MessagesSent returns the total number of messages sent via Send().
func (r *DistributedRuntime) MessagesSent() uint64 {
	return atomic.LoadUint64(&r.messagesSent)
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

// Listen implements server-side streaming for receiving messages from an agent.
// This allows remote clients to subscribe to messages from a local agent.
func (s *agentServiceServer) Listen(req *pb.ListenRequest, stream pb.AgentService_ListenServer) error {
	// Validate
	if req.AgentName == "" {
		return status.Errorf(codes.InvalidArgument, "agent_name is required")
	}

	if !isValidAgentNameGRPC(req.AgentName) {
		return status.Errorf(codes.InvalidArgument, "invalid agent name format")
	}

	// Get the channel for this agent
	ch, err := s.runtime.Recv(req.AgentName)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return status.Errorf(codes.NotFound, "agent not found: %s", req.AgentName)
		}
		return status.Errorf(codes.Internal, "failed to get channel: %v", err)
	}

	// Stream messages until context is cancelled
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				// Channel closed
				return nil
			}
			if err := stream.Send(&pb.ListenResponse{Message: msg.Message}); err != nil {
				return err
			}
		}
	}
}

// isValidAgentNameGRPC validates agent name for gRPC requests
func isValidAgentNameGRPC(name string) bool {
	// Only allow lowercase alphanumeric, hyphens, underscores, max 64 chars
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9_-]{0,63}$`, name)
	return matched
}

// StartAgentsPhased starts all registered LOCAL agents in dependency order.
// Remote agents are assumed to be already running on their respective nodes.
//
// Agents are started in phases based on their dependencies:
//   - Phase 0: Agents with no dependencies
//   - Phase N: Agents whose dependencies are all in phases < N
//
// Within each phase, agents are started concurrently and the method waits
// for all of them to report Ready() before proceeding to the next phase.
//
// This method should be called after all agents are registered and after
// Start() has been called to initialize the runtime.
func (r *DistributedRuntime) StartAgentsPhased(ctx context.Context, agentDefs map[string]agent.AgentDef) error {
	if !r.started {
		return ErrRuntimeNotStarted
	}

	// Build dependency graph for LOCAL agents only
	// Remote agents are not started by this runtime
	depGraph := graph.NewDependencyGraph()
	for name, def := range agentDefs {
		// Only add local agents to the graph
		r.mu.RLock()
		_, isLocal := r.localAgents[name]
		r.mu.RUnlock()

		if isLocal {
			depGraph.AddNode(name, def.DependsOn)
		}
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
				if err := r.waitForReady(gctx, a, r.config.AgentStartTimeout); err != nil {
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
func (r *DistributedRuntime) waitForReady(ctx context.Context, a agent.Agent, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}

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
