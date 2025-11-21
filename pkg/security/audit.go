package security

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// AuditEvent represents a security-relevant event
type AuditEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"event_type"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	IPAddress string                 `json:"ip_address,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Resource  string                 `json:"resource"`
	Action    string                 `json:"action"`
	Result    string                 `json:"result"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// AuditLogger defines the interface for audit logging
type AuditLogger interface {
	Log(event *AuditEvent)
	LogToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result interface{}, err error)
	LogAuthAttempt(ctx context.Context, success bool, err error)
	LogAuthorizationCheck(ctx context.Context, resource string, permission Permission, allowed bool)
	Close() error
}

// InMemoryAuditLogger stores audit events in memory (for testing)
type InMemoryAuditLogger struct {
	events []AuditEvent
	mu     sync.RWMutex
}

// NewInMemoryAuditLogger creates a new in-memory audit logger
func NewInMemoryAuditLogger() *InMemoryAuditLogger {
	return &InMemoryAuditLogger{
		events: make([]AuditEvent, 0),
	}
}

// Log records an audit event
func (l *InMemoryAuditLogger) Log(event *AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, *event)
}

// LogToolExecution logs a tool execution event
func (l *InMemoryAuditLogger) LogToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result interface{}, err error) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "tool.execution",
		Resource:  toolName,
		Action:    "execute",
		Metadata:  make(map[string]interface{}),
	}

	// Extract auth context if available
	if authCtx, authErr := GetAuthContext(ctx); authErr == nil {
		event.UserID = authCtx.Principal.ID
		event.SessionID = authCtx.SessionID
		event.IPAddress = authCtx.IPAddress
		event.UserAgent = authCtx.UserAgent
	}

	// Sanitize arguments (remove sensitive data)
	if args != nil {
		event.Metadata["args_count"] = len(args)
		// Don't log actual argument values for security
	}

	if err != nil {
		event.Result = "failure"
		event.Error = sanitizeErrorMessage(err.Error())
	} else {
		event.Result = "success"
	}

	l.Log(event)
}

// LogAuthAttempt logs an authentication attempt
func (l *InMemoryAuditLogger) LogAuthAttempt(ctx context.Context, success bool, err error) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "auth.attempt",
		Action:    "authenticate",
		Resource:  "system",
	}

	if success {
		event.Result = "success"
	} else {
		event.Result = "failure"
		if err != nil {
			event.Error = sanitizeErrorMessage(err.Error())
		}
	}

	l.Log(event)
}

// LogAuthorizationCheck logs an authorization check
func (l *InMemoryAuditLogger) LogAuthorizationCheck(ctx context.Context, resource string, permission Permission, allowed bool) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "auth.authorization",
		Resource:  resource,
		Action:    string(permission),
		Metadata:  make(map[string]interface{}),
	}

	// Extract auth context if available
	if authCtx, authErr := GetAuthContext(ctx); authErr == nil {
		event.UserID = authCtx.Principal.ID
		event.SessionID = authCtx.SessionID
	}

	if allowed {
		event.Result = "allowed"
	} else {
		event.Result = "denied"
	}

	l.Log(event)
}

// GetEvents returns all logged events (for testing)
func (l *InMemoryAuditLogger) GetEvents() []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy
	events := make([]AuditEvent, len(l.events))
	copy(events, l.events)
	return events
}

// Close closes the audit logger
func (l *InMemoryAuditLogger) Close() error {
	return nil
}

// JSONAuditLogger logs audit events to a JSON file or stdout
type JSONAuditLogger struct {
	mu sync.Mutex
}

// NewJSONAuditLogger creates a new JSON audit logger
func NewJSONAuditLogger() *JSONAuditLogger {
	return &JSONAuditLogger{}
}

// Log records an audit event as JSON
func (l *JSONAuditLogger) Log(event *AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	jsonData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal audit event: %v", err)
		return
	}

	// Log to stdout (in production, would log to file or SIEM)
	fmt.Println(string(jsonData))
}

// LogToolExecution logs a tool execution event
func (l *JSONAuditLogger) LogToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result interface{}, err error) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "tool.execution",
		Resource:  toolName,
		Action:    "execute",
		Metadata:  make(map[string]interface{}),
	}

	// Extract auth context if available
	if authCtx, authErr := GetAuthContext(ctx); authErr == nil {
		event.UserID = authCtx.Principal.ID
		event.SessionID = authCtx.SessionID
		event.IPAddress = authCtx.IPAddress
		event.UserAgent = authCtx.UserAgent
	}

	// Sanitize arguments
	if args != nil {
		event.Metadata["args_count"] = len(args)
	}

	if err != nil {
		event.Result = "failure"
		event.Error = sanitizeErrorMessage(err.Error())
	} else {
		event.Result = "success"
	}

	l.Log(event)
}

// LogAuthAttempt logs an authentication attempt
func (l *JSONAuditLogger) LogAuthAttempt(ctx context.Context, success bool, err error) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "auth.attempt",
		Action:    "authenticate",
		Resource:  "system",
	}

	if success {
		event.Result = "success"
	} else {
		event.Result = "failure"
		if err != nil {
			event.Error = sanitizeErrorMessage(err.Error())
		}
	}

	l.Log(event)
}

// LogAuthorizationCheck logs an authorization check
func (l *JSONAuditLogger) LogAuthorizationCheck(ctx context.Context, resource string, permission Permission, allowed bool) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "auth.authorization",
		Resource:  resource,
		Action:    string(permission),
		Metadata:  make(map[string]interface{}),
	}

	// Extract auth context if available
	if authCtx, authErr := GetAuthContext(ctx); authErr == nil {
		event.UserID = authCtx.Principal.ID
		event.SessionID = authCtx.SessionID
	}

	if allowed {
		event.Result = "allowed"
	} else {
		event.Result = "denied"
	}

	l.Log(event)
}

// Close closes the audit logger
func (l *JSONAuditLogger) Close() error {
	return nil
}

// NoOpAuditLogger is a no-op implementation (for when audit logging is disabled)
type NoOpAuditLogger struct{}

// NewNoOpAuditLogger creates a new no-op audit logger
func NewNoOpAuditLogger() *NoOpAuditLogger {
	return &NoOpAuditLogger{}
}

// Log does nothing
func (l *NoOpAuditLogger) Log(event *AuditEvent) {}

// LogToolExecution does nothing
func (l *NoOpAuditLogger) LogToolExecution(ctx context.Context, toolName string, args map[string]interface{}, result interface{}, err error) {
}

// LogAuthAttempt does nothing
func (l *NoOpAuditLogger) LogAuthAttempt(ctx context.Context, success bool, err error) {}

// LogAuthorizationCheck does nothing
func (l *NoOpAuditLogger) LogAuthorizationCheck(ctx context.Context, resource string, permission Permission, allowed bool) {
}

// Close does nothing
func (l *NoOpAuditLogger) Close() error {
	return nil
}
