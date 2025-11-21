package security

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"
)

func TestMemoryAuditBackend(t *testing.T) {
	backend := NewMemoryAuditBackend()

	event := &StructuredAuditEvent{
		ID:        "test-id",
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  "test_tool",
		Action:    "execute",
		Result:    "success",
	}

	if err := backend.Write(event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := backend.Events()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	if events[0].Resource != "test_tool" {
		t.Errorf("expected resource 'test_tool', got '%s'", events[0].Resource)
	}
}

type mockWriteCloser struct {
	buf    *bytes.Buffer
	closed bool
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return nil
}

func TestFileAuditBackend(t *testing.T) {
	mock := &mockWriteCloser{buf: &bytes.Buffer{}}
	backend := NewFileAuditBackendWithWriter(mock)

	event := &StructuredAuditEvent{
		ID:        "test-id",
		Timestamp: time.Now(),
		Type:      AuditEventToolCall,
		Resource:  "test_tool",
		Action:    "execute",
		Result:    "success",
	}

	if err := backend.Write(event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify JSON output
	var written StructuredAuditEvent
	if err := json.Unmarshal(mock.buf.Bytes(), &written); err != nil {
		t.Fatalf("failed to unmarshal written event: %v", err)
	}

	if written.Resource != "test_tool" {
		t.Errorf("expected resource 'test_tool', got '%s'", written.Resource)
	}

	if err := backend.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}

	if !mock.closed {
		t.Error("expected writer to be closed")
	}
}

func TestIntegratedAuditLogger(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithTraceID(ctx, "trace-456")

	// Test LogToolExecution
	logger.LogToolExecution(ctx, "test_tool", map[string]interface{}{"key": "value"}, "result", nil)

	events := backend.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.RequestID != "req-123" {
		t.Errorf("expected request_id 'req-123', got '%s'", event.RequestID)
	}
	if event.TraceID != "trace-456" {
		t.Errorf("expected trace_id 'trace-456', got '%s'", event.TraceID)
	}
	if event.Result != "success" {
		t.Errorf("expected result 'success', got '%s'", event.Result)
	}
}

func TestIntegratedAuditLoggerWithError(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	ctx := context.Background()
	logger.LogToolExecution(ctx, "test_tool", nil, nil, errors.New("test error"))

	events := backend.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Result != "failure" {
		t.Errorf("expected result 'failure', got '%s'", events[0].Result)
	}
	if events[0].Type != AuditEventError {
		t.Errorf("expected type 'error', got '%s'", events[0].Type)
	}
}

func TestIntegratedAuditLoggerAuthAttempt(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	ctx := context.Background()

	// Test success
	logger.LogAuthAttempt(ctx, true, nil)
	events := backend.Events()
	if events[0].Type != AuditEventAuthSuccess {
		t.Errorf("expected type 'auth.success', got '%s'", events[0].Type)
	}

	// Test failure
	logger.LogAuthAttempt(ctx, false, errors.New("invalid token"))
	events = backend.Events()
	if events[1].Type != AuditEventAuthFailure {
		t.Errorf("expected type 'auth.failure', got '%s'", events[1].Type)
	}
}

func TestIntegratedAuditLoggerAuthorizationCheck(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	ctx := context.Background()

	// Test allowed
	logger.LogAuthorizationCheck(ctx, "test_resource", PermExecute, true)
	events := backend.Events()
	if events[0].Type != AuditEventAuthzAllowed {
		t.Errorf("expected type 'authz.allowed', got '%s'", events[0].Type)
	}

	// Test denied
	logger.LogAuthorizationCheck(ctx, "test_resource", PermExecute, false)
	events = backend.Events()
	if events[1].Type != AuditEventAuthzDenied {
		t.Errorf("expected type 'authz.denied', got '%s'", events[1].Type)
	}
}

func TestIntegratedAuditLoggerRateLimit(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	ctx := context.Background()
	logger.LogRateLimitExceeded(ctx, "test_tool", "client-123")

	events := backend.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != AuditEventRateLimit {
		t.Errorf("expected type 'ratelimit.exceeded', got '%s'", events[0].Type)
	}
}

func TestIntegratedAuditLoggerValidationError(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	ctx := context.Background()
	logger.LogValidationError(ctx, "test_field", errors.New("invalid value"))

	events := backend.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != AuditEventValidation {
		t.Errorf("expected type 'validation.error', got '%s'", events[0].Type)
	}
}

func TestAuditMiddleware(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)
	middleware := NewAuditMiddleware(logger)

	handler := func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return "result", nil
	}

	wrapped := middleware.WrapHandler("test_tool", handler)

	ctx := context.Background()
	result, err := wrapped(ctx, map[string]interface{}{"key": "value"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "result" {
		t.Errorf("expected result 'result', got '%v'", result)
	}

	events := backend.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Resource != "test_tool" {
		t.Errorf("expected resource 'test_tool', got '%s'", events[0].Resource)
	}

	// Verify duration was recorded
	if events[0].Duration == 0 {
		t.Error("expected duration to be recorded")
	}
}

func TestMultipleBackends(t *testing.T) {
	backend1 := NewMemoryAuditBackend()
	backend2 := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend1, backend2)

	ctx := context.Background()
	logger.LogToolExecution(ctx, "test_tool", nil, nil, nil)

	// Both backends should have the event
	if len(backend1.Events()) != 1 {
		t.Error("expected backend1 to have 1 event")
	}
	if len(backend2.Events()) != 1 {
		t.Error("expected backend2 to have 1 event")
	}
}

func TestAddBackend(t *testing.T) {
	backend1 := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend1)

	backend2 := NewMemoryAuditBackend()
	logger.AddBackend(backend2)

	ctx := context.Background()
	logger.LogToolExecution(ctx, "test_tool", nil, nil, nil)

	if len(backend1.Events()) != 1 {
		t.Error("expected backend1 to have 1 event")
	}
	if len(backend2.Events()) != 1 {
		t.Error("expected backend2 to have 1 event")
	}
}

func TestContextTracingHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithTraceID(ctx, "trace-456")
	ctx = WithSpanID(ctx, "span-789")

	if GetRequestID(ctx) != "req-123" {
		t.Error("expected request_id 'req-123'")
	}

	// Test empty context
	if GetRequestID(context.Background()) != "" {
		t.Error("expected empty request_id for empty context")
	}
}

// Test that IntegratedAuditLogger implements AuditLogger interface
func TestIntegratedAuditLoggerImplementsInterface(t *testing.T) {
	backend := NewMemoryAuditBackend()
	logger := NewIntegratedAuditLogger(backend)

	// This should compile if the interface is implemented correctly
	var _ AuditLogger = logger
}

type failingWriter struct{}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (f *failingWriter) Close() error {
	return nil
}

func TestFileBackendWriteError(t *testing.T) {
	backend := NewFileAuditBackendWithWriter(&failingWriter{})

	event := &StructuredAuditEvent{
		ID:       "test",
		Resource: "test",
	}

	err := backend.Write(event)
	if err == nil {
		t.Error("expected write error")
	}
}
