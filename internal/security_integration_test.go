package internal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/security"
)

// Test End-to-End Authentication Flow
func TestSecurityIntegration_AuthenticationFlow(t *testing.T) {
	// Setup
	authenticator := security.NewAPIKeyAuthenticator()
	authorizer := security.NewRBACAuthorizer()
	auditLogger := security.NewInMemoryAuditLogger()

	// Create user
	apiKey := "test-fixture-not-a-real-key-1"
	principal := &security.Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
		Permissions: []security.Permission{
			security.PermRead,
			security.PermWrite,
		},
	}
	authenticator.AddKey(apiKey, principal)

	// Test 1: Successful authentication
	ctx := context.Background()
	authPrincipal, err := authenticator.Authenticate(ctx, apiKey)
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}

	auditLogger.LogAuthAttempt(ctx, true, nil)

	// Test 2: Authorization for allowed action
	authCtx := &security.AuthContext{
		Principal:   authPrincipal,
		SessionID:   "session123",
		IPAddress:   "192.168.1.1",
		RequestTime: time.Now(),
	}
	ctx = security.WithAuthContext(ctx, authCtx)

	err = authorizer.Authorize(ctx, authPrincipal, "document", security.PermRead)
	if err != nil {
		t.Errorf("authorization should succeed for allowed permission: %v", err)
	}

	auditLogger.LogAuthorizationCheck(ctx, "document", security.PermRead, true)

	// Test 3: Authorization for disallowed action
	err = authorizer.Authorize(ctx, authPrincipal, "system", security.PermAdmin)
	if err == nil {
		t.Error("authorization should fail for admin permission")
	}

	auditLogger.LogAuthorizationCheck(ctx, "system", security.PermAdmin, false)

	// Verify audit logs
	events := auditLogger.GetEvents()
	if len(events) != 3 {
		t.Errorf("expected 3 audit events, got %d", len(events))
	}

	t.Logf("Successfully completed authentication flow with %d audit events", len(events))
}

// Test Multiple Security Layers
func TestSecurityIntegration_MultipleSecurityLayers(t *testing.T) {
	// Setup all security components
	authenticator := security.NewAPIKeyAuthenticator()
	authorizer := security.NewRBACAuthorizer()
	rateLimiter := security.NewRateLimiter(5.0, 5)
	auditLogger := security.NewInMemoryAuditLogger()

	// Setup user
	apiKey := "test-fixture-not-a-real-key-2"
	principal := &security.Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}
	authenticator.AddKey(apiKey, principal)

	ctx := context.Background()

	// Layer 1: Authentication
	authPrincipal, err := authenticator.Authenticate(ctx, apiKey)
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}
	auditLogger.LogAuthAttempt(ctx, true, nil)

	authCtx := &security.AuthContext{
		Principal:   authPrincipal,
		SessionID:   "session123",
		IPAddress:   "192.168.1.1",
		RequestTime: time.Now(),
	}
	ctx = security.WithAuthContext(ctx, authCtx)

	// Layer 2: Authorization
	err = authorizer.Authorize(ctx, authPrincipal, "api", security.PermRead)
	if err != nil {
		t.Errorf("authorization failed: %v", err)
	}
	auditLogger.LogAuthorizationCheck(ctx, "api", security.PermRead, true)

	// Layer 3: Rate limiting
	clientID := authPrincipal.ID
	allowed := rateLimiter.Allow(clientID)
	if !allowed {
		t.Error("rate limiter should allow first request")
	}

	// Layer 4: Audit logging
	auditLogger.LogToolExecution(ctx, "api_call", nil, "success", nil)

	// Verify all layers worked
	events := auditLogger.GetEvents()
	if len(events) < 3 {
		t.Errorf("expected at least 3 audit events, got %d", len(events))
	}

	t.Log("All security layers successfully integrated")
}

// Test Attack Scenario - SQL Injection
func TestSecurityIntegration_SQLInjectionAttack(t *testing.T) {
	validator := &security.StringValidator{
		MaxLength:            100,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
		CheckSQLInjection:    true,
	}

	auditLogger := security.NewInMemoryAuditLogger()

	// Simulated attack payloads
	attacks := []string{
		"admin' OR '1'='1",
		"'; DROP TABLE users;--",
		"admin'--",
		"' UNION SELECT * FROM passwords--",
	}

	ctx := context.Background()
	successfulBlocks := 0

	for _, attack := range attacks {
		err := validator.Validate(attack)
		if err != nil {
			successfulBlocks++
			auditLogger.LogToolExecution(ctx, "sql_query", map[string]interface{}{
				"query": attack,
			}, nil, errors.New("validation failed: potential SQL injection"))
		}
	}

	if successfulBlocks != len(attacks) {
		t.Errorf("expected to block %d attacks, blocked %d", len(attacks), successfulBlocks)
	}

	t.Logf("Successfully blocked %d SQL injection attempts", successfulBlocks)
}

// Test Attack Scenario - Path Traversal
func TestSecurityIntegration_PathTraversalAttack(t *testing.T) {
	auditLogger := security.NewInMemoryAuditLogger()

	// Simulated path traversal attacks
	attacks := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"files/../../../etc/shadow",
		"/etc/passwd",
	}

	baseDir := "/var/app/files"
	successfulBlocks := 0
	ctx := context.Background()

	for _, attack := range attacks {
		_, err := security.SanitizeFilePath(attack, baseDir)
		if err != nil {
			successfulBlocks++
			auditLogger.LogToolExecution(ctx, "file_read", map[string]interface{}{
				"path": attack,
			}, nil, errors.New("path traversal detected"))
		}
	}

	if successfulBlocks != len(attacks) {
		t.Errorf("expected to block %d attacks, blocked %d", len(attacks), successfulBlocks)
	}

	t.Logf("Successfully blocked %d path traversal attempts", successfulBlocks)
}

// Test Attack Scenario - Brute Force Authentication
func TestSecurityIntegration_BruteForceAttack(t *testing.T) {
	authenticator := security.NewAPIKeyAuthenticator()
	rateLimiter := security.NewRateLimiter(5.0, 5)
	auditLogger := security.NewInMemoryAuditLogger()

	// Setup valid user
	validKey := "test-fixture-not-a-real-key-3"
	principal := &security.Principal{
		ID:   "user123",
		Name: "Test User",
	}
	authenticator.AddKey(validKey, principal)

	ctx := context.Background()
	attackerIP := "192.168.1.100"

	// Simulate brute force attack
	attempts := 100
	failedAttempts := 0
	rateLimited := 0

	for i := 0; i < attempts; i++ {
		// Check rate limit first
		if !rateLimiter.Allow(attackerIP) {
			rateLimited++
			continue
		}

		// Try authentication with invalid key
		invalidKey := "invalid-key-" + string(rune(i))
		_, err := authenticator.Authenticate(ctx, invalidKey)
		if err != nil {
			failedAttempts++
			auditLogger.LogAuthAttempt(ctx, false, err)
		}
	}

	// Verify rate limiting kicked in
	if rateLimited == 0 {
		t.Error("rate limiter should have blocked some attempts")
	}

	t.Logf("Brute force attack: %d attempts, %d failed auth, %d rate limited",
		attempts, failedAttempts, rateLimited)

	// Verify audit logs captured the attack
	events := auditLogger.GetEvents()
	if len(events) == 0 {
		t.Error("audit log should contain failed authentication attempts")
	}
}

// Test Error Handling with Security
func TestSecurityIntegration_ErrorHandling(t *testing.T) {
	auditLogger := security.NewInMemoryAuditLogger()

	// Simulate various error scenarios
	testCases := []struct {
		name      string
		operation string
		err       error
	}{
		{
			name:      "authentication failure",
			operation: "authenticate",
			err:       errors.New("invalid API key"),
		},
		{
			name:      "authorization failure",
			operation: "authorize",
			err:       errors.New("insufficient permissions"),
		},
		{
			name:      "validation failure",
			operation: "validate_input",
			err:       errors.New("input validation failed"),
		},
		{
			name:      "rate limit exceeded",
			operation: "rate_limit",
			err:       errors.New("rate limit exceeded"),
		},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Log the error
			auditLogger.LogToolExecution(ctx, tc.operation, nil, nil, tc.err)

			// Verify error was logged
			events := auditLogger.GetEvents()
			if len(events) == 0 {
				t.Error("error should be logged")
			}

			lastEvent := events[len(events)-1]
			if lastEvent.Result != "failure" {
				t.Errorf("result = %s, want failure", lastEvent.Result)
			}

			if lastEvent.Error == "" {
				t.Error("error message should be logged")
			}
		})
	}
}

// Test Secure Configuration Loading
func TestSecurityIntegration_SecureConfigLoading(t *testing.T) {
	// Test that sensitive configuration is handled securely
	testConfig := map[string]string{
		"api_key":      "sk-secret123",
		"database_url": "postgres://user:pass@host:5432/db",
		"jwt_secret":   "super-secret-jwt-key",
	}

	// Verify secrets are masked when logged
	for key, value := range testConfig {
		masked := security.MaskSecret(value)

		// Masked value should not equal original
		if masked == value && len(value) > 8 {
			t.Errorf("secret for %s should be masked", key)
		}

		// Masked value should still have some info
		if masked == "" {
			t.Errorf("masked value for %s should not be empty", key)
		}

		t.Logf("%s: %s -> %s", key, value, masked)
	}
}

// Test Defense in Depth
func TestSecurityIntegration_DefenseInDepth(t *testing.T) {
	// Setup multiple layers of defense
	_ = security.NewAPIKeyAuthenticator()
	_ = security.NewRBACAuthorizer()
	rateLimiter := security.NewRateLimiter(10.0, 10)
	auditLogger := security.NewInMemoryAuditLogger()
	inputValidator := &security.StringValidator{
		MaxLength:            1000,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
		CheckSQLInjection:    true,
	}

	// Test request with multiple attack vectors
	ctx := context.Background()
	maliciousInput := "'; DROP TABLE users;--"

	// Layer 1: Input validation
	err := inputValidator.Validate(maliciousInput)
	if err == nil {
		t.Error("input validation should reject malicious input")
	}

	// Even with invalid input, system should not crash
	// Layer 2: Rate limiting
	if !rateLimiter.Allow("attacker") {
		t.Log("Request rate limited")
	}

	// Layer 3: Audit logging
	auditLogger.LogToolExecution(ctx, "test_operation", map[string]interface{}{
		"input": maliciousInput,
	}, nil, errors.New("validation failed"))

	// Verify system remained secure through all layers
	events := auditLogger.GetEvents()
	if len(events) == 0 {
		t.Error("attack attempt should be logged")
	}

	t.Log("Defense in depth successfully prevented attack")
}

// Test Context Propagation with Security
func TestSecurityIntegration_ContextPropagation(t *testing.T) {
	// Setup
	authenticator := security.NewAPIKeyAuthenticator()
	auditLogger := security.NewInMemoryAuditLogger()

	apiKey := "test-fixture-not-a-real-key-4"
	principal := &security.Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}
	authenticator.AddKey(apiKey, principal)

	// Authenticate
	ctx := context.Background()
	authPrincipal, err := authenticator.Authenticate(ctx, apiKey)
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}

	// Add auth context
	authCtx := &security.AuthContext{
		Principal:   authPrincipal,
		SessionID:   "session123",
		IPAddress:   "192.168.1.1",
		RequestTime: time.Now(),
	}
	ctx = security.WithAuthContext(ctx, authCtx)

	// Simulate nested function calls that need auth context
	simulateNestedCall := func(ctx context.Context) error {
		// Retrieve auth context
		retrievedCtx, err := security.GetAuthContext(ctx)
		if err != nil {
			return err
		}

		if retrievedCtx.Principal.ID != principal.ID {
			return errors.New("principal ID mismatch")
		}

		// Log operation with context
		auditLogger.LogToolExecution(ctx, "nested_operation", nil, "success", nil)

		return nil
	}

	// Call nested function
	err = simulateNestedCall(ctx)
	if err != nil {
		t.Errorf("nested call failed: %v", err)
	}

	// Verify audit log captured context
	events := auditLogger.GetEvents()
	if len(events) == 0 {
		t.Error("audit log should contain event")
	}

	event := events[len(events)-1]
	if event.UserID != principal.ID {
		t.Error("audit log should capture user ID from context")
	}

	t.Log("Context successfully propagated through call stack")
}

// Benchmark integration tests
func BenchmarkSecurityIntegration_FullAuthFlow(b *testing.B) {
	authenticator := security.NewAPIKeyAuthenticator()
	authorizer := security.NewRBACAuthorizer()
	auditLogger := security.NewInMemoryAuditLogger()

	apiKey := "test-fixture-not-a-real-key-5"
	principal := &security.Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}
	authenticator.AddKey(apiKey, principal)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()

		// Authenticate
		authPrincipal, _ := authenticator.Authenticate(ctx, apiKey)

		// Authorize
		_ = authorizer.Authorize(ctx, authPrincipal, "resource", security.PermRead)

		// Audit
		auditLogger.LogToolExecution(ctx, "test", nil, nil, nil)
	}
}

func BenchmarkSecurityIntegration_ValidationAndAudit(b *testing.B) {
	validator := &security.StringValidator{
		MaxLength:            100,
		DisallowNullBytes:    true,
		DisallowControlChars: true,
	}
	auditLogger := security.NewInMemoryAuditLogger()

	input := "test input string"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validator.Validate(input)
		if err == nil {
			auditLogger.LogToolExecution(ctx, "test", nil, "success", nil)
		}
	}
}
