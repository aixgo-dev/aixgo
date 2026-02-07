package security

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test Log Format - Basic Event
func TestAuditEvent_BasicFormat(t *testing.T) {
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "tool.execution",
		UserID:    "user123",
		SessionID: "session456",
		IPAddress: "192.168.1.1",
		UserAgent: "TestAgent/1.0",
		Resource:  "test_tool",
		Action:    "execute",
		Result:    "success",
	}

	// Verify all fields are set
	if event.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
	if event.EventType == "" {
		t.Error("event type should be set")
	}
	if event.Resource == "" {
		t.Error("resource should be set")
	}
	if event.Action == "" {
		t.Error("action should be set")
	}
	if event.Result == "" {
		t.Error("result should be set")
	}
}

// Test Sensitive Data Masking
func TestAuditEvent_SensitiveDataMasking(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()

	// Log event with potentially sensitive data (test fixtures, not real secrets)
	args := map[string]interface{}{
		"api_key":  "<example-api-key>",
		"password": "<example-password>",
		"username": "john",
	}

	logger.LogToolExecution(ctx, "api_call", args, nil, nil)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Verify that argument values are not logged
	if metadata, ok := event.Metadata["args"]; ok {
		t.Errorf("actual argument values should not be logged, got: %v", metadata)
	}

	// Should only log arg count
	if argCount, ok := event.Metadata["args_count"]; !ok || argCount != 3 {
		t.Errorf("args_count should be 3, got: %v", argCount)
	}
}

// Test Log Completeness
func TestAuditLogger_LogCompleteness(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}

	authCtx := &AuthContext{
		Principal:   principal,
		SessionID:   "session456",
		IPAddress:   "192.168.1.1",
		UserAgent:   "TestAgent/1.0",
		RequestTime: time.Now(),
	}

	ctx := context.Background()
	ctx = WithAuthContext(ctx, authCtx)

	// Log a tool execution
	logger.LogToolExecution(ctx, "test_tool", nil, "success", nil)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Verify all context information is captured
	checks := []struct {
		name  string
		check func() bool
	}{
		{
			name:  "event type is set",
			check: func() bool { return event.EventType == "tool.execution" },
		},
		{
			name:  "user ID is captured",
			check: func() bool { return event.UserID == principal.ID },
		},
		{
			name:  "session ID is captured",
			check: func() bool { return event.SessionID == authCtx.SessionID },
		},
		{
			name:  "IP address is captured",
			check: func() bool { return event.IPAddress == authCtx.IPAddress },
		},
		{
			name:  "user agent is captured",
			check: func() bool { return event.UserAgent == authCtx.UserAgent },
		},
		{
			name:  "resource is set",
			check: func() bool { return event.Resource == "test_tool" },
		},
		{
			name:  "action is set",
			check: func() bool { return event.Action == "execute" },
		},
		{
			name:  "result is set",
			check: func() bool { return event.Result == "success" },
		},
		{
			name:  "timestamp is set",
			check: func() bool { return !event.Timestamp.IsZero() },
		},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !check.check() {
				t.Errorf("check failed: %s", check.name)
			}
		})
	}
}

// Test Concurrent Logging
func TestAuditLogger_ConcurrentLogging(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	var wg sync.WaitGroup
	numGoroutines := 100
	eventsPerGoroutine := 10

	ctx := context.Background()

	// Log events concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				logger.LogToolExecution(ctx, "test_tool", nil, nil, nil)
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were logged
	events := logger.GetEvents()
	expectedCount := numGoroutines * eventsPerGoroutine

	if len(events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(events))
	}
}

// Test Authentication Attempt Logging - Success
func TestAuditLogger_AuthAttemptSuccess(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()
	logger.LogAuthAttempt(ctx, true, nil)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	if event.EventType != "auth.attempt" {
		t.Errorf("event type = %s, want auth.attempt", event.EventType)
	}

	if event.Result != "success" {
		t.Errorf("result = %s, want success", event.Result)
	}

	if event.Error != "" {
		t.Error("error should be empty for successful auth")
	}
}

// Test Authentication Attempt Logging - Failure
func TestAuditLogger_AuthAttemptFailure(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()
	authErr := errors.New("invalid credentials")
	logger.LogAuthAttempt(ctx, false, authErr)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	if event.EventType != "auth.attempt" {
		t.Errorf("event type = %s, want auth.attempt", event.EventType)
	}

	if event.Result != "failure" {
		t.Errorf("result = %s, want failure", event.Result)
	}

	if event.Error == "" {
		t.Error("error should be set for failed auth")
	}
}

// Test Authorization Check Logging - Allowed
func TestAuditLogger_AuthzCheckAllowed(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}

	authCtx := &AuthContext{
		Principal: principal,
		SessionID: "session456",
	}

	ctx := context.Background()
	ctx = WithAuthContext(ctx, authCtx)

	logger.LogAuthorizationCheck(ctx, "test_resource", PermRead, true)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	if event.EventType != "auth.authorization" {
		t.Errorf("event type = %s, want auth.authorization", event.EventType)
	}

	if event.Resource != "test_resource" {
		t.Errorf("resource = %s, want test_resource", event.Resource)
	}

	if event.Action != string(PermRead) {
		t.Errorf("action = %s, want %s", event.Action, PermRead)
	}

	if event.Result != "allowed" {
		t.Errorf("result = %s, want allowed", event.Result)
	}

	if event.UserID != principal.ID {
		t.Errorf("user ID = %s, want %s", event.UserID, principal.ID)
	}
}

// Test Authorization Check Logging - Denied
func TestAuditLogger_AuthzCheckDenied(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()
	logger.LogAuthorizationCheck(ctx, "test_resource", PermWrite, false)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	if event.Result != "denied" {
		t.Errorf("result = %s, want denied", event.Result)
	}
}

// Test Tool Execution Logging - Success
func TestAuditLogger_ToolExecutionSuccess(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()
	args := map[string]interface{}{
		"param1": "value1",
		"param2": 123,
	}

	logger.LogToolExecution(ctx, "calculator", args, 456, nil)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	if event.EventType != "tool.execution" {
		t.Errorf("event type = %s, want tool.execution", event.EventType)
	}

	if event.Resource != "calculator" {
		t.Errorf("resource = %s, want calculator", event.Resource)
	}

	if event.Result != "success" {
		t.Errorf("result = %s, want success", event.Result)
	}

	if event.Error != "" {
		t.Error("error should be empty for successful execution")
	}
}

// Test Tool Execution Logging - Failure
func TestAuditLogger_ToolExecutionFailure(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()
	execErr := errors.New("tool execution failed")

	logger.LogToolExecution(ctx, "failing_tool", nil, nil, execErr)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	if event.Result != "failure" {
		t.Errorf("result = %s, want failure", event.Result)
	}

	if event.Error == "" {
		t.Error("error should be set for failed execution")
	}
}

// Test Error Message Sanitization
func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		shouldNotContain []string
	}{
		{
			name:             "remove file paths",
			input:            "error reading /home/user/secret.txt",
			shouldNotContain: []string{"/home/user"},
		},
		{
			name:             "remove IP addresses",
			input:            "connection failed to 192.168.1.100:8080",
			shouldNotContain: []string{"192.168.1.100"},
		},
		{
			name:             "remove API keys",
			input:            "authentication failed with key sk-example-test-key",
			shouldNotContain: []string{"sk-example-test-key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := sanitizeErrorMessage(tt.input)

			for _, sensitive := range tt.shouldNotContain {
				if strings.Contains(sanitized, sensitive) {
					t.Errorf("sanitized message still contains sensitive data: %s", sensitive)
				}
			}

			t.Logf("Original: %s", tt.input)
			t.Logf("Sanitized: %s", sanitized)
		})
	}
}

// Test JSON Audit Logger
func TestJSONAuditLogger(t *testing.T) {
	logger := NewJSONAuditLogger()

	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "test.event",
		Resource:  "test_resource",
		Action:    "test_action",
		Result:    "success",
	}

	// This will output to stdout in the test
	logger.Log(event)

	// Close should not error
	if err := logger.Close(); err != nil {
		t.Errorf("close error: %v", err)
	}
}

// Test NoOp Audit Logger
func TestNoOpAuditLogger(t *testing.T) {
	logger := NewNoOpAuditLogger()

	ctx := context.Background()

	// All operations should succeed without errors
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "test.event",
	}

	logger.Log(event)
	logger.LogToolExecution(ctx, "tool", nil, nil, nil)
	logger.LogAuthAttempt(ctx, true, nil)
	logger.LogAuthorizationCheck(ctx, "resource", PermRead, true)

	if err := logger.Close(); err != nil {
		t.Errorf("close error: %v", err)
	}
}

// Test Audit Log Rotation (Conceptual)
func TestAuditLogger_LogRotation(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()

	// Generate many events
	for i := 0; i < 1000; i++ {
		logger.LogToolExecution(ctx, "test_tool", nil, nil, nil)
	}

	events := logger.GetEvents()

	if len(events) != 1000 {
		t.Errorf("expected 1000 events, got %d", len(events))
	}

	// In a real implementation with file-based logging,
	// we would test log rotation here
	t.Log("Log rotation should be implemented for production use")
}

// Test Event Filtering
func TestAuditLogger_EventFiltering(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()

	// Log different types of events
	logger.LogAuthAttempt(ctx, true, nil)
	logger.LogAuthAttempt(ctx, false, errors.New("failed"))
	logger.LogToolExecution(ctx, "tool1", nil, nil, nil)
	logger.LogAuthorizationCheck(ctx, "resource", PermRead, true)

	events := logger.GetEvents()

	// Count event types
	authAttempts := 0
	toolExecutions := 0
	authzChecks := 0

	for _, event := range events {
		switch event.EventType {
		case "auth.attempt":
			authAttempts++
		case "tool.execution":
			toolExecutions++
		case "auth.authorization":
			authzChecks++
		}
	}

	if authAttempts != 2 {
		t.Errorf("expected 2 auth attempts, got %d", authAttempts)
	}
	if toolExecutions != 1 {
		t.Errorf("expected 1 tool execution, got %d", toolExecutions)
	}
	if authzChecks != 1 {
		t.Errorf("expected 1 authz check, got %d", authzChecks)
	}
}

// Test Timestamp Accuracy
func TestAuditLogger_TimestampAccuracy(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	before := time.Now()
	ctx := context.Background()
	logger.LogToolExecution(ctx, "test_tool", nil, nil, nil)
	after := time.Now()

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Timestamp should be between before and after
	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Errorf("timestamp %v is outside expected range [%v, %v]",
			event.Timestamp, before, after)
	}
}

// Test Metadata Handling
func TestAuditLogger_Metadata(t *testing.T) {
	logger := NewInMemoryAuditLogger()

	ctx := context.Background()
	args := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	logger.LogToolExecution(ctx, "test_tool", args, nil, nil)

	events := logger.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Verify metadata is present
	if event.Metadata == nil {
		t.Error("metadata should not be nil")
	}

	// Verify args_count is logged
	if argCount, ok := event.Metadata["args_count"]; !ok {
		t.Error("args_count should be in metadata")
	} else if argCount != 3 {
		t.Errorf("args_count = %v, want 3", argCount)
	}
}

// Test Logger Interface Compliance
func TestAuditLogger_InterfaceCompliance(t *testing.T) {
	// Verify all implementations satisfy the interface
	var _ AuditLogger = &InMemoryAuditLogger{}
	var _ AuditLogger = &JSONAuditLogger{}
	var _ AuditLogger = &NoOpAuditLogger{}
}

// Benchmark audit logging
func BenchmarkInMemoryAuditLogger_Log(b *testing.B) {
	logger := NewInMemoryAuditLogger()
	event := &AuditEvent{
		Timestamp: time.Now(),
		EventType: "test.event",
		Resource:  "resource",
		Action:    "action",
		Result:    "success",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Log(event)
	}
}

func BenchmarkInMemoryAuditLogger_LogToolExecution(b *testing.B) {
	logger := NewInMemoryAuditLogger()
	ctx := context.Background()
	args := map[string]interface{}{
		"key": "value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogToolExecution(ctx, "test_tool", args, nil, nil)
	}
}

func BenchmarkSanitizeErrorMessage(b *testing.B) {
	msg := "error at /home/user/file.txt with IP 192.168.1.1 and key <example-key>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeErrorMessage(msg)
	}
}
