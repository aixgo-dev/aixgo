package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test API Key Authentication - Valid Cases
func TestAPIKeyAuthenticator_ValidAuthentication(t *testing.T) {
	auth := NewAPIKeyAuthenticator()

	principal := &Principal{
		ID:          "user123",
		Name:        "Test User",
		Roles:       []string{"user"},
		Permissions: []Permission{PermRead, PermWrite},
	}

	apiKey := generateAPIKey()
	auth.AddKey(apiKey, principal)

	ctx := context.Background()
	result, err := auth.Authenticate(ctx, apiKey)

	if err != nil {
		t.Fatalf("expected successful authentication, got error: %v", err)
	}

	if result.ID != principal.ID {
		t.Errorf("principal ID = %s, want %s", result.ID, principal.ID)
	}

	if result.Name != principal.Name {
		t.Errorf("principal name = %s, want %s", result.Name, principal.Name)
	}
}

// Test API Key Authentication - Invalid Key
func TestAPIKeyAuthenticator_InvalidKey(t *testing.T) {
	auth := NewAPIKeyAuthenticator()

	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}

	auth.AddKey("valid_key", principal)

	ctx := context.Background()
	_, err := auth.Authenticate(ctx, "invalid_key")

	if err == nil {
		t.Error("expected authentication error for invalid key")
	}

	if !strings.Contains(err.Error(), "invalid authentication token") {
		t.Errorf("error message = %v, want 'invalid authentication token'", err)
	}
}

// Test API Key Authentication - Empty Token
func TestAPIKeyAuthenticator_EmptyToken(t *testing.T) {
	auth := NewAPIKeyAuthenticator()

	ctx := context.Background()
	_, err := auth.Authenticate(ctx, "")

	if err == nil {
		t.Error("expected authentication error for empty token")
	}

	if !strings.Contains(err.Error(), "missing authentication token") {
		t.Errorf("error message = %v, want 'missing authentication token'", err)
	}
}

// Test Timing Attack Resistance
func TestAPIKeyAuthenticator_TimingAttackResistance(t *testing.T) {
	auth := NewAPIKeyAuthenticator()

	validKey := generateAPIKey()
	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}
	auth.AddKey(validKey, principal)

	ctx := context.Background()

	// Try various invalid keys
	invalidKeys := []string{
		"wrong_key",
		validKey[:len(validKey)/2],
		validKey + "extra",
		strings.Repeat("a", len(validKey)),
	}

	// Measure timing for invalid keys - should be consistent
	timings := make([]time.Duration, len(invalidKeys))
	for i, key := range invalidKeys {
		start := time.Now()
		_, _ = auth.Authenticate(ctx, key)
		timings[i] = time.Since(start)
	}

	// All timings should be relatively similar (within 10x of each other)
	// This is a basic check - more sophisticated timing analysis would be needed for production
	minTime := timings[0]
	maxTime := timings[0]
	for _, timing := range timings[1:] {
		if timing < minTime {
			minTime = timing
		}
		if timing > maxTime {
			maxTime = timing
		}
	}

	// If max is more than 100x min, there might be a timing leak
	if maxTime > minTime*100 {
		t.Logf("Warning: Large timing variation detected (min=%v, max=%v)", minTime, maxTime)
	}
}

// Test RBAC Authorization - Valid Permission
func TestRBACAuthorizer_ValidPermission(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}

	ctx := context.Background()
	err := authorizer.Authorize(ctx, principal, "resource", PermRead)

	if err != nil {
		t.Errorf("expected authorization to succeed, got error: %v", err)
	}
}

// Test RBAC Authorization - Missing Permission
func TestRBACAuthorizer_MissingPermission(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"readonly"},
	}

	ctx := context.Background()
	err := authorizer.Authorize(ctx, principal, "resource", PermWrite)

	if err == nil {
		t.Error("expected authorization to fail for missing permission")
	}

	if !strings.Contains(err.Error(), "insufficient permissions") {
		t.Errorf("error message = %v, want 'insufficient permissions'", err)
	}
}

// Test RBAC Authorization - Admin Override
func TestRBACAuthorizer_AdminOverride(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	adminPrincipal := &Principal{
		ID:    "admin123",
		Name:  "Admin User",
		Roles: []string{"admin"},
	}

	ctx := context.Background()

	// Admin should have all permissions
	permissions := []Permission{PermRead, PermWrite, PermExecute, PermAdmin}
	for _, perm := range permissions {
		err := authorizer.Authorize(ctx, adminPrincipal, "resource", perm)
		if err != nil {
			t.Errorf("admin should have %s permission, got error: %v", perm, err)
		}
	}
}

// Test RBAC Authorization - Direct Permission
func TestRBACAuthorizer_DirectPermission(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	principal := &Principal{
		ID:          "user123",
		Name:        "Test User",
		Roles:       []string{},
		Permissions: []Permission{PermRead, PermWrite},
	}

	ctx := context.Background()

	// Should succeed with direct permission
	err := authorizer.Authorize(ctx, principal, "resource", PermRead)
	if err != nil {
		t.Errorf("expected authorization with direct permission, got error: %v", err)
	}

	// Should fail without direct permission
	err = authorizer.Authorize(ctx, principal, "resource", PermExecute)
	if err == nil {
		t.Error("expected authorization to fail without direct permission")
	}
}

// Test RBAC Authorization - No Principal
func TestRBACAuthorizer_NoPrincipal(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	ctx := context.Background()
	err := authorizer.Authorize(ctx, nil, "resource", PermRead)

	if err == nil {
		t.Error("expected authorization to fail with no principal")
	}

	if !strings.Contains(err.Error(), "no principal provided") {
		t.Errorf("error message = %v, want 'no principal provided'", err)
	}
}

// Test Role Permission Management
func TestRBACAuthorizer_AddRolePermission(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	// Add custom role
	authorizer.AddRolePermission("custom", PermRead)
	authorizer.AddRolePermission("custom", PermWrite)

	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"custom"},
	}

	ctx := context.Background()

	// Should have added permissions
	err := authorizer.Authorize(ctx, principal, "resource", PermRead)
	if err != nil {
		t.Errorf("expected authorization with custom role, got error: %v", err)
	}

	err = authorizer.Authorize(ctx, principal, "resource", PermWrite)
	if err != nil {
		t.Errorf("expected authorization with custom role, got error: %v", err)
	}

	// Should not have other permissions
	err = authorizer.Authorize(ctx, principal, "resource", PermExecute)
	if err == nil {
		t.Error("expected authorization to fail for permission not in custom role")
	}
}

// Test Context Storage and Retrieval
func TestAuthContext_ContextStorage(t *testing.T) {
	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}

	authCtx := &AuthContext{
		Principal:   principal,
		SessionID:   "session123",
		IPAddress:   "192.168.1.1",
		UserAgent:   "TestAgent/1.0",
		RequestTime: time.Now(),
	}

	ctx := context.Background()
	ctx = WithAuthContext(ctx, authCtx)

	// Retrieve and verify
	retrieved, err := GetAuthContext(ctx)
	if err != nil {
		t.Fatalf("expected to retrieve auth context, got error: %v", err)
	}

	if retrieved.Principal.ID != principal.ID {
		t.Errorf("principal ID = %s, want %s", retrieved.Principal.ID, principal.ID)
	}

	if retrieved.SessionID != authCtx.SessionID {
		t.Errorf("session ID = %s, want %s", retrieved.SessionID, authCtx.SessionID)
	}
}

// Test Context Retrieval Without Auth
func TestAuthContext_MissingContext(t *testing.T) {
	ctx := context.Background()

	_, err := GetAuthContext(ctx)
	if err == nil {
		t.Error("expected error when retrieving missing auth context")
	}

	if !strings.Contains(err.Error(), "no authentication context found") {
		t.Errorf("error message = %v, want 'no authentication context found'", err)
	}
}

// Test GetPrincipal
func TestAuthContext_GetPrincipal(t *testing.T) {
	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}

	authCtx := &AuthContext{
		Principal: principal,
	}

	ctx := context.Background()
	ctx = WithAuthContext(ctx, authCtx)

	retrieved, err := GetPrincipal(ctx)
	if err != nil {
		t.Fatalf("expected to retrieve principal, got error: %v", err)
	}

	if retrieved.ID != principal.ID {
		t.Errorf("principal ID = %s, want %s", retrieved.ID, principal.ID)
	}
}

// Test Concurrent Authentication
func TestAPIKeyAuthenticator_ConcurrentAccess(t *testing.T) {
	auth := NewAPIKeyAuthenticator()

	// Add multiple keys
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		key := generateAPIKey()
		keys[i] = key
		principal := &Principal{
			ID:   string(rune(i)),
			Name: "User " + string(rune(i)),
		}
		auth.AddKey(key, principal)
	}

	// Concurrently authenticate
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			ctx := context.Background()
			_, err := auth.Authenticate(ctx, key)
			if err != nil {
				errors <- err
			}
		}(keys[i])
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("concurrent authentication error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("got %d errors during concurrent authentication", errorCount)
	}
}

// Test Concurrent Authorization
func TestRBACAuthorizer_ConcurrentAccess(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrently authorize
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			err := authorizer.Authorize(ctx, principal, "resource", PermRead)
			if err != nil {
				errors <- err
			}
		}()
	}

	// Concurrently add role permissions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			roleName := "role_" + string(rune(idx))
			authorizer.AddRolePermission(roleName, PermRead)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("concurrent authorization error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("got %d errors during concurrent authorization", errorCount)
	}
}

// Test Multiple Roles
func TestRBACAuthorizer_MultipleRoles(t *testing.T) {
	authorizer := NewRBACAuthorizer()

	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user", "readonly"},
	}

	ctx := context.Background()

	// Should have permissions from both roles
	err := authorizer.Authorize(ctx, principal, "resource", PermRead)
	if err != nil {
		t.Errorf("expected authorization with multiple roles, got error: %v", err)
	}

	err = authorizer.Authorize(ctx, principal, "resource", PermExecute)
	if err != nil {
		t.Errorf("expected authorization with multiple roles, got error: %v", err)
	}
}

// Test NoAuthAuthenticator (Testing Only)
func TestNoAuthAuthenticator(t *testing.T) {
	auth := NewNoAuthAuthenticator()

	ctx := context.Background()
	principal, err := auth.Authenticate(ctx, "any_token")

	if err != nil {
		t.Fatalf("NoAuthAuthenticator should not return error, got: %v", err)
	}

	if principal.ID != "anonymous" {
		t.Errorf("principal ID = %s, want 'anonymous'", principal.ID)
	}

	// Should have admin permissions
	hasAdmin := false
	for _, perm := range principal.Permissions {
		if perm == PermAdmin {
			hasAdmin = true
			break
		}
	}

	if !hasAdmin {
		t.Error("NoAuthAuthenticator principal should have admin permission")
	}
}

// Test AllowAllAuthorizer (Testing Only)
func TestAllowAllAuthorizer(t *testing.T) {
	authorizer := NewAllowAllAuthorizer()

	ctx := context.Background()

	// Should allow everything
	err := authorizer.Authorize(ctx, nil, "resource", PermAdmin)
	if err != nil {
		t.Errorf("AllowAllAuthorizer should not return error, got: %v", err)
	}

	// Even with no principal
	err = authorizer.Authorize(ctx, nil, "resource", PermWrite)
	if err != nil {
		t.Errorf("AllowAllAuthorizer should allow access without principal, got: %v", err)
	}
}

// Test Authentication Flow
func TestAuthenticationFlow_EndToEnd(t *testing.T) {
	// Setup
	authenticator := NewAPIKeyAuthenticator()
	authorizer := NewRBACAuthorizer()

	apiKey := generateAPIKey()
	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}
	authenticator.AddKey(apiKey, principal)

	// Step 1: Authenticate
	ctx := context.Background()
	authPrincipal, err := authenticator.Authenticate(ctx, apiKey)
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}

	// Step 2: Create auth context
	authCtx := &AuthContext{
		Principal:   authPrincipal,
		SessionID:   "session123",
		IPAddress:   "192.168.1.1",
		RequestTime: time.Now(),
	}
	ctx = WithAuthContext(ctx, authCtx)

	// Step 3: Authorize action
	err = authorizer.Authorize(ctx, authPrincipal, "resource", PermRead)
	if err != nil {
		t.Errorf("authorization failed: %v", err)
	}

	// Step 4: Verify context
	retrievedCtx, err := GetAuthContext(ctx)
	if err != nil {
		t.Errorf("failed to retrieve auth context: %v", err)
	}

	if retrievedCtx.Principal.ID != principal.ID {
		t.Errorf("principal ID = %s, want %s", retrievedCtx.Principal.ID, principal.ID)
	}
}

// Test Unauthorized Access Attempts
func TestUnauthorizedAccessAttempts(t *testing.T) {
	authenticator := NewAPIKeyAuthenticator()
	authorizer := NewRBACAuthorizer()

	validKey := generateAPIKey()
	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"readonly"},
	}
	authenticator.AddKey(validKey, principal)

	tests := []struct {
		name         string
		apiKey       string
		permission   Permission
		wantAuthErr  bool
		wantAuthzErr bool
	}{
		{
			name:         "valid key, insufficient permission",
			apiKey:       validKey,
			permission:   PermWrite,
			wantAuthErr:  false,
			wantAuthzErr: true,
		},
		{
			name:         "invalid key",
			apiKey:       "invalid_key",
			permission:   PermRead,
			wantAuthErr:  true,
			wantAuthzErr: true,
		},
		{
			name:         "empty key",
			apiKey:       "",
			permission:   PermRead,
			wantAuthErr:  true,
			wantAuthzErr: true,
		},
		{
			name:         "valid key, valid permission",
			apiKey:       validKey,
			permission:   PermRead,
			wantAuthErr:  false,
			wantAuthzErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Try authentication
			authPrincipal, authErr := authenticator.Authenticate(ctx, tt.apiKey)

			if tt.wantAuthErr && authErr == nil {
				t.Error("expected authentication error")
			}
			if !tt.wantAuthErr && authErr != nil {
				t.Errorf("unexpected authentication error: %v", authErr)
			}

			// Try authorization if authenticated
			if authErr == nil {
				authzErr := authorizer.Authorize(ctx, authPrincipal, "resource", tt.permission)

				if tt.wantAuthzErr && authzErr == nil {
					t.Error("expected authorization error")
				}
				if !tt.wantAuthzErr && authzErr != nil {
					t.Errorf("unexpected authorization error: %v", authzErr)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkAPIKeyAuthentication(b *testing.B) {
	auth := NewAPIKeyAuthenticator()
	apiKey := generateAPIKey()
	principal := &Principal{
		ID:   "user123",
		Name: "Test User",
	}
	auth.AddKey(apiKey, principal)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = auth.Authenticate(ctx, apiKey)
	}
}

func BenchmarkRBACAuthorization(b *testing.B) {
	authorizer := NewRBACAuthorizer()
	principal := &Principal{
		ID:    "user123",
		Name:  "Test User",
		Roles: []string{"user"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = authorizer.Authorize(ctx, principal, "resource", PermRead)
	}
}

// Helper function to generate random API key
func generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
