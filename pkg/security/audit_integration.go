package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditEventType defines the types of audit events
type AuditEventType string

const (
	AuditEventToolCall     AuditEventType = "tool.call"
	AuditEventToolResult   AuditEventType = "tool.result"
	AuditEventAuthSuccess  AuditEventType = "auth.success"
	AuditEventAuthFailure  AuditEventType = "auth.failure"
	AuditEventAuthzAllowed AuditEventType = "authz.allowed"
	AuditEventAuthzDenied  AuditEventType = "authz.denied"
	AuditEventRateLimit    AuditEventType = "ratelimit.exceeded"
	AuditEventValidation   AuditEventType = "validation.error"
	AuditEventError        AuditEventType = "error"
)

// StructuredAuditEvent represents a structured audit event with request tracing
type StructuredAuditEvent struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Type       AuditEventType         `json:"type"`
	RequestID  string                 `json:"request_id,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	SpanID     string                 `json:"span_id,omitempty"`
	Principal  *PrincipalInfo         `json:"principal,omitempty"`
	Resource   string                 `json:"resource,omitempty"`
	Action     string                 `json:"action,omitempty"`
	Result     string                 `json:"result"`
	Error      string                 `json:"error,omitempty"`
	Duration   time.Duration          `json:"duration_ns,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ClientInfo *ClientInfo            `json:"client_info,omitempty"`
}

// PrincipalInfo contains sanitized principal information for audit logs
type PrincipalInfo struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Roles []string `json:"roles,omitempty"`
}

// ClientInfo contains client connection information
type ClientInfo struct {
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// AuditBackend defines the interface for audit storage backends
type AuditBackend interface {
	Write(event *StructuredAuditEvent) error
	Close() error
}

// MemoryAuditBackend stores events in memory (for testing)
type MemoryAuditBackend struct {
	events []StructuredAuditEvent
	mu     sync.RWMutex
}

// NewMemoryAuditBackend creates a new in-memory audit backend
func NewMemoryAuditBackend() *MemoryAuditBackend {
	return &MemoryAuditBackend{
		events: make([]StructuredAuditEvent, 0),
	}
}

// Write stores an event in memory
func (b *MemoryAuditBackend) Write(event *StructuredAuditEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, *event)
	return nil
}

// Events returns all stored events
func (b *MemoryAuditBackend) Events() []StructuredAuditEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]StructuredAuditEvent, len(b.events))
	copy(result, b.events)
	return result
}

// Close closes the backend
func (b *MemoryAuditBackend) Close() error {
	return nil
}

// FileAuditBackend writes events to a file as JSON lines
type FileAuditBackend struct {
	writer io.WriteCloser
	mu     sync.Mutex
}

// NewFileAuditBackend creates a file-based audit backend
func NewFileAuditBackend(path string) (*FileAuditBackend, error) {
	// Validate file path to prevent path traversal attacks
	if err := ValidateFilePath(path); err != nil {
		return nil, fmt.Errorf("invalid audit file path: %w", err)
	}

	// G304: Path is validated above using ValidateFilePath
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}
	return &FileAuditBackend{writer: f}, nil
}

// NewFileAuditBackendWithWriter creates a file backend with a custom writer
func NewFileAuditBackendWithWriter(w io.WriteCloser) *FileAuditBackend {
	return &FileAuditBackend{writer: w}
}

// Write writes an event to the file
func (b *FileAuditBackend) Write(event *StructuredAuditEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	if _, err := b.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// Close closes the file
func (b *FileAuditBackend) Close() error {
	return b.writer.Close()
}

// IntegratedAuditLogger provides comprehensive audit logging with multiple backends
type IntegratedAuditLogger struct {
	backends []AuditBackend
	mu       sync.RWMutex
}

// NewIntegratedAuditLogger creates a new integrated audit logger
func NewIntegratedAuditLogger(backends ...AuditBackend) *IntegratedAuditLogger {
	return &IntegratedAuditLogger{
		backends: backends,
	}
}

// AddBackend adds a new backend to the logger
func (l *IntegratedAuditLogger) AddBackend(backend AuditBackend) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.backends = append(l.backends, backend)
}

// Log implements the AuditLogger interface
func (l *IntegratedAuditLogger) Log(event *AuditEvent) {
	structured := &StructuredAuditEvent{
		ID:        uuid.New().String(),
		Timestamp: event.Timestamp,
		Type:      AuditEventType(event.EventType),
		Resource:  event.Resource,
		Action:    event.Action,
		Result:    event.Result,
		Error:     event.Error,
		Metadata:  event.Metadata,
	}

	if event.UserID != "" {
		structured.Principal = &PrincipalInfo{ID: event.UserID}
	}

	if event.IPAddress != "" || event.UserAgent != "" {
		structured.ClientInfo = &ClientInfo{
			IPAddress: event.IPAddress,
			UserAgent: event.UserAgent,
		}
	}

	l.write(structured)
}

// LogToolExecution implements the AuditLogger interface
func (l *IntegratedAuditLogger) LogToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result interface{}, err error) {
	event := &StructuredAuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  toolName,
		Action:    "execute",
		Metadata:  make(map[string]interface{}),
	}

	// Extract tracing info from context
	l.extractTracingInfo(ctx, event)

	// Extract auth context
	if authCtx, authErr := GetAuthContext(ctx); authErr == nil && authCtx != nil {
		event.Principal = &PrincipalInfo{
			ID:    authCtx.Principal.ID,
			Type:  "user",
			Roles: authCtx.Principal.Roles,
		}
		event.ClientInfo = &ClientInfo{
			IPAddress: authCtx.IPAddress,
			UserAgent: authCtx.UserAgent,
		}
	}

	// Sanitize and include metadata
	if args != nil {
		event.Metadata["args_count"] = len(args)
		// Include field names but not values for security
		keys := make([]string, 0, len(args))
		for k := range args {
			keys = append(keys, k)
		}
		event.Metadata["args_keys"] = keys
	}

	if err != nil {
		event.Result = "failure"
		event.Error = sanitizeErrorMessage(err.Error())
		event.Type = AuditEventError
	} else {
		event.Result = "success"
		event.Type = AuditEventToolResult
	}

	l.write(event)
}

// LogAuthAttempt implements the AuditLogger interface
func (l *IntegratedAuditLogger) LogAuthAttempt(ctx context.Context, success bool, err error) {
	event := &StructuredAuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Resource:  "authentication",
		Action:    "authenticate",
		Metadata:  make(map[string]interface{}),
	}

	l.extractTracingInfo(ctx, event)

	if success {
		event.Type = AuditEventAuthSuccess
		event.Result = "success"
	} else {
		event.Type = AuditEventAuthFailure
		event.Result = "failure"
		if err != nil {
			event.Error = sanitizeErrorMessage(err.Error())
		}
	}

	l.write(event)
}

// LogAuthorizationCheck implements the AuditLogger interface
func (l *IntegratedAuditLogger) LogAuthorizationCheck(ctx context.Context, resource string, permission Permission, allowed bool) {
	event := &StructuredAuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Resource:  resource,
		Action:    string(permission),
		Metadata:  make(map[string]interface{}),
	}

	l.extractTracingInfo(ctx, event)

	if authCtx, err := GetAuthContext(ctx); err == nil && authCtx != nil {
		event.Principal = &PrincipalInfo{
			ID:    authCtx.Principal.ID,
			Type:  "user",
			Roles: authCtx.Principal.Roles,
		}
	}

	if allowed {
		event.Type = AuditEventAuthzAllowed
		event.Result = "allowed"
	} else {
		event.Type = AuditEventAuthzDenied
		event.Result = "denied"
	}

	l.write(event)
}

// LogRateLimitExceeded logs a rate limit event
func (l *IntegratedAuditLogger) LogRateLimitExceeded(ctx context.Context, resource string, clientID string) {
	event := &StructuredAuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Type:      AuditEventRateLimit,
		Resource:  resource,
		Action:    "rate_limit",
		Result:    "exceeded",
		Metadata: map[string]interface{}{
			"client_id": clientID,
		},
	}

	l.extractTracingInfo(ctx, event)
	l.write(event)
}

// LogValidationError logs a validation error event
func (l *IntegratedAuditLogger) LogValidationError(ctx context.Context, resource string, err error) {
	event := &StructuredAuditEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Type:      AuditEventValidation,
		Resource:  resource,
		Action:    "validate",
		Result:    "failure",
		Error:     sanitizeErrorMessage(err.Error()),
	}

	l.extractTracingInfo(ctx, event)
	l.write(event)
}

// Close closes all backends
func (l *IntegratedAuditLogger) Close() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var lastErr error
	for _, backend := range l.backends {
		if err := backend.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// write writes an event to all backends
func (l *IntegratedAuditLogger) write(event *StructuredAuditEvent) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, backend := range l.backends {
		if err := backend.Write(event); err != nil {
			// Log audit write failures to stderr as fallback to ensure visibility
			// This prevents silent failures while not affecting the main flow
			fmt.Fprintf(os.Stderr, "AUDIT_FALLBACK: failed to write audit event %s (type=%s, resource=%s): %v\n",
				event.ID, event.Type, event.Resource, err)
		}
	}
}

// extractTracingInfo extracts tracing information from context
func (l *IntegratedAuditLogger) extractTracingInfo(ctx context.Context, event *StructuredAuditEvent) {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		event.RequestID = reqID
	}
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		event.TraceID = traceID
	}
	if spanID, ok := ctx.Value(spanIDKey).(string); ok {
		event.SpanID = spanID
	}
}

// Context keys for tracing
type auditContextKey string

const (
	requestIDKey auditContextKey = "audit_request_id"
	traceIDKey   auditContextKey = "audit_trace_id"
	spanIDKey    auditContextKey = "audit_span_id"
)

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// WithSpanID adds a span ID to the context
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey, spanID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// AuditMiddleware provides middleware functionality for MCP servers
type AuditMiddleware struct {
	logger *IntegratedAuditLogger
}

// NewAuditMiddleware creates a new audit middleware
func NewAuditMiddleware(logger *IntegratedAuditLogger) *AuditMiddleware {
	return &AuditMiddleware{logger: logger}
}

// WrapHandler wraps a tool handler with audit logging
func (m *AuditMiddleware) WrapHandler(toolName string, handler func(context.Context, map[string]interface{}) (interface{}, error)) func(context.Context, map[string]interface{}) (interface{}, error) {
	return func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		start := time.Now()

		// Ensure request ID exists
		if GetRequestID(ctx) == "" {
			ctx = WithRequestID(ctx, uuid.New().String())
		}

		result, err := handler(ctx, args)

		// Log with duration
		event := &StructuredAuditEvent{
			ID:        uuid.New().String(),
			Timestamp: start,
			Type:      AuditEventToolCall,
			Resource:  toolName,
			Action:    "execute",
			Duration:  time.Since(start),
			Metadata:  make(map[string]interface{}),
		}

		m.logger.extractTracingInfo(ctx, event)

		if args != nil {
			event.Metadata["args_count"] = len(args)
		}

		if err != nil {
			event.Result = "failure"
			event.Error = sanitizeErrorMessage(err.Error())
		} else {
			event.Result = "success"
		}

		m.logger.write(event)

		return result, err
	}
}

// Logger returns the underlying audit logger
func (m *AuditMiddleware) Logger() *IntegratedAuditLogger {
	return m.logger
}
