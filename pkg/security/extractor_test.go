package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDisabledAuthExtractor(t *testing.T) {
	extractor := NewDisabledAuthExtractor()
	req := httptest.NewRequest("GET", "/test", nil)

	principal, err := extractor.ExtractAuth(context.Background(), req)
	if err != nil {
		t.Errorf("ExtractAuth() error = %v, want nil", err)
	}

	if principal == nil {
		t.Fatal("ExtractAuth() returned nil principal")
		return
	}

	if principal.ID != "anonymous" {
		t.Errorf("Principal.ID = %v, want anonymous", principal.ID)
	}

	if principal.Metadata["auth_mode"] != "disabled" {
		t.Errorf("Principal.Metadata[auth_mode] = %v, want disabled", principal.Metadata["auth_mode"])
	}
}

func TestDelegatedAuthExtractor(t *testing.T) {
	tests := []struct {
		name       string
		config     *DelegatedAuthConfig
		setupReq   func(*http.Request)
		wantErr    bool
		checkPrinc func(*testing.T, *Principal)
	}{
		{
			name: "valid IAP identity",
			config: &DelegatedAuthConfig{
				IdentityHeader: "X-Goog-Authenticated-User-Email",
				IAP: &IAPConfig{
					Enabled:   true,
					VerifyJWT: false,
				},
			},
			setupReq: func(r *http.Request) {
				r.Header.Set("X-Goog-Authenticated-User-Email", "accounts.google.com:user@example.com")
			},
			wantErr: false,
			checkPrinc: func(t *testing.T, p *Principal) {
				if p.ID != "user@example.com" {
					t.Errorf("Principal.ID = %v, want user@example.com", p.ID)
				}
				if p.Metadata["auth_mode"] != "delegated_iap" {
					t.Errorf("auth_mode = %v, want delegated_iap", p.Metadata["auth_mode"])
				}
			},
		},
		{
			name: "missing identity header",
			config: &DelegatedAuthConfig{
				IdentityHeader: "X-User-Email",
			},
			setupReq: func(r *http.Request) {},
			wantErr:  true,
		},
		{
			name: "generic delegated auth with header mapping",
			config: &DelegatedAuthConfig{
				IdentityHeader: "X-User-Email",
				HeaderMapping: map[string]string{
					"name":  "X-User-Name",
					"roles": "X-User-Roles",
				},
			},
			setupReq: func(r *http.Request) {
				r.Header.Set("X-User-Email", "user@example.com")
				r.Header.Set("X-User-Name", "John Doe")
				r.Header.Set("X-User-Roles", "admin,user")
			},
			wantErr: false,
			checkPrinc: func(t *testing.T, p *Principal) {
				if p.ID != "user@example.com" {
					t.Errorf("Principal.ID = %v, want user@example.com", p.ID)
				}
				if p.Name != "John Doe" {
					t.Errorf("Principal.Name = %v, want John Doe", p.Name)
				}
				if len(p.Roles) != 2 {
					t.Errorf("len(Roles) = %v, want 2", len(p.Roles))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor, err := NewDelegatedAuthExtractor(tt.config)
			if err != nil {
				t.Fatalf("NewDelegatedAuthExtractor() error = %v", err)
			}

			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupReq(req)

			principal, err := extractor.ExtractAuth(context.Background(), req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractAuth() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractAuth() unexpected error = %v", err)
				return
			}

			if principal == nil {
				t.Fatal("ExtractAuth() returned nil principal")
			}

			if tt.checkPrinc != nil {
				tt.checkPrinc(t, principal)
			}
		})
	}
}

func TestBuiltinAuthExtractor(t *testing.T) {
	// Set up test API key in environment
	testAPIKey := "test-api-key-123"
	_ = os.Setenv("AIXGO_API_KEY_testuser", testAPIKey)
	defer func() { _ = os.Unsetenv("AIXGO_API_KEY_testuser") }()

	tests := []struct {
		name       string
		config     *BuiltinAuthConfig
		setupReq   func(*http.Request)
		wantErr    bool
		checkPrinc func(*testing.T, *Principal)
	}{
		{
			name: "valid API key",
			config: &BuiltinAuthConfig{
				Method: "api_key",
				APIKeys: &APIKeyConfig{
					Source:    "environment",
					EnvPrefix: "AIXGO_API_KEY_",
				},
			},
			setupReq: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+testAPIKey)
			},
			wantErr: false,
			checkPrinc: func(t *testing.T, p *Principal) {
				if p.ID != "testuser" {
					t.Errorf("Principal.ID = %v, want testuser", p.ID)
				}
				if p.Metadata["auth_mode"] != "builtin" {
					t.Errorf("auth_mode = %v, want builtin", p.Metadata["auth_mode"])
				}
			},
		},
		{
			name: "missing authorization header",
			config: &BuiltinAuthConfig{
				Method: "api_key",
				APIKeys: &APIKeyConfig{
					Source:    "environment",
					EnvPrefix: "AIXGO_API_KEY_",
				},
			},
			setupReq: func(r *http.Request) {},
			wantErr:  true,
		},
		{
			name: "invalid bearer token",
			config: &BuiltinAuthConfig{
				Method: "api_key",
				APIKeys: &APIKeyConfig{
					Source:    "environment",
					EnvPrefix: "AIXGO_API_KEY_",
				},
			},
			setupReq: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer invalid-key")
			},
			wantErr: true,
		},
		{
			name: "wrong auth scheme",
			config: &BuiltinAuthConfig{
				Method: "api_key",
				APIKeys: &APIKeyConfig{
					Source:    "environment",
					EnvPrefix: "AIXGO_API_KEY_",
				},
			},
			setupReq: func(r *http.Request) {
				r.Header.Set("Authorization", "Basic dGVzdDp0ZXN0")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor, err := NewBuiltinAuthExtractor(tt.config)
			if err != nil {
				t.Fatalf("NewBuiltinAuthExtractor() error = %v", err)
			}

			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupReq(req)

			principal, err := extractor.ExtractAuth(context.Background(), req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractAuth() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractAuth() unexpected error = %v", err)
				return
			}

			if principal == nil {
				t.Fatal("ExtractAuth() returned nil principal")
			}

			if tt.checkPrinc != nil {
				tt.checkPrinc(t, principal)
			}
		})
	}
}

func TestHybridAuthExtractor(t *testing.T) {
	// Set up test API key
	testAPIKey := "test-hybrid-key"
	_ = os.Setenv("AIXGO_API_KEY_hybriduser", testAPIKey)
	defer func() { _ = os.Unsetenv("AIXGO_API_KEY_hybriduser") }()

	delegatedConfig := &DelegatedAuthConfig{
		IdentityHeader: "X-User-Email",
	}

	builtinConfig := &BuiltinAuthConfig{
		Method: "api_key",
		APIKeys: &APIKeyConfig{
			Source:    "environment",
			EnvPrefix: "AIXGO_API_KEY_",
		},
	}

	extractor, err := NewHybridAuthExtractor(delegatedConfig, builtinConfig)
	if err != nil {
		t.Fatalf("NewHybridAuthExtractor() error = %v", err)
	}

	tests := []struct {
		name       string
		setupReq   func(*http.Request)
		wantErr    bool
		checkPrinc func(*testing.T, *Principal)
	}{
		{
			name: "delegated auth succeeds",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-User-Email", "iapuser@example.com")
			},
			wantErr: false,
			checkPrinc: func(t *testing.T, p *Principal) {
				if p.ID != "iapuser@example.com" {
					t.Errorf("Principal.ID = %v, want iapuser@example.com", p.ID)
				}
			},
		},
		{
			name: "fallback to builtin auth",
			setupReq: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+testAPIKey)
			},
			wantErr: false,
			checkPrinc: func(t *testing.T, p *Principal) {
				if p.ID != "hybriduser" {
					t.Errorf("Principal.ID = %v, want hybriduser", p.ID)
				}
				if p.Metadata["auth_mode"] != "hybrid" {
					t.Errorf("auth_mode = %v, want hybrid", p.Metadata["auth_mode"])
				}
			},
		},
		{
			name: "both auth methods fail",
			setupReq: func(r *http.Request) {
				// No auth headers
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupReq(req)

			principal, err := extractor.ExtractAuth(context.Background(), req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractAuth() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractAuth() unexpected error = %v", err)
				return
			}

			if principal == nil {
				t.Fatal("ExtractAuth() returned nil principal")
			}

			if tt.checkPrinc != nil {
				tt.checkPrinc(t, principal)
			}
		})
	}
}

func TestNewAuthExtractorFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *SecurityConfig
		wantErr bool
	}{
		{
			name: "disabled mode",
			config: &SecurityConfig{
				AuthMode: AuthModeDisabled,
			},
			wantErr: false,
		},
		{
			name: "delegated mode",
			config: &SecurityConfig{
				AuthMode: AuthModeDelegated,
				DelegatedAuth: &DelegatedAuthConfig{
					IdentityHeader: "X-User-Email",
				},
			},
			wantErr: false,
		},
		{
			name: "builtin mode",
			config: &SecurityConfig{
				AuthMode: AuthModeBuiltin,
				BuiltinAuth: &BuiltinAuthConfig{
					Method: "api_key",
					APIKeys: &APIKeyConfig{
						Source:    "environment",
						EnvPrefix: "TEST_",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "hybrid mode",
			config: &SecurityConfig{
				AuthMode: AuthModeHybrid,
				DelegatedAuth: &DelegatedAuthConfig{
					IdentityHeader: "X-User-Email",
				},
				BuiltinAuth: &BuiltinAuthConfig{
					Method: "api_key",
					APIKeys: &APIKeyConfig{
						Source:    "environment",
						EnvPrefix: "TEST_",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid mode",
			config: &SecurityConfig{
				AuthMode: AuthMode("invalid"),
			},
			wantErr: true,
		},
		{
			name: "delegated mode missing config",
			config: &SecurityConfig{
				AuthMode: AuthModeDelegated,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor, err := NewAuthExtractorFromConfig(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewAuthExtractorFromConfig() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("NewAuthExtractorFromConfig() unexpected error = %v", err)
				return
			}

			if extractor == nil {
				t.Fatal("NewAuthExtractorFromConfig() returned nil extractor")
			}
		})
	}
}

func TestExtractAuthContext(t *testing.T) {
	extractor := NewDisabledAuthExtractor()
	middleware := ExtractAuthContext(extractor)

	// Create a test handler that checks auth context
	var receivedPrincipal *Principal
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, err := GetAuthContext(r.Context())
		if err != nil {
			t.Errorf("GetAuthContext() error = %v", err)
			return
		}
		receivedPrincipal = authCtx.Principal
		w.WriteHeader(http.StatusOK)
	})

	// Wrap handler with middleware
	wrappedHandler := middleware(handler)

	// Test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", rec.Code, http.StatusOK)
	}

	if receivedPrincipal == nil {
		t.Fatal("Principal was not set in context")
	}

	if receivedPrincipal.ID != "anonymous" {
		t.Errorf("Principal.ID = %v, want anonymous", receivedPrincipal.ID)
	}
}

func TestDisabledAuthExtractor_ReadOnlyPermissions(t *testing.T) {
	extractor := NewDisabledAuthExtractor()
	req := httptest.NewRequest("GET", "/test", nil)

	principal, err := extractor.ExtractAuth(context.Background(), req)
	if err != nil {
		t.Fatalf("ExtractAuth() error = %v", err)
	}

	// Verify anonymous users only have read permission (not admin)
	if len(principal.Permissions) != 1 || principal.Permissions[0] != PermRead {
		t.Errorf("DisabledAuth should only grant PermRead, got %v", principal.Permissions)
	}

	// Verify role is not admin
	for _, role := range principal.Roles {
		if role == "admin" {
			t.Error("DisabledAuth should not grant admin role")
		}
	}
}

func TestRoleValidation(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		isValid bool
	}{
		{"valid_user", "user", true},
		{"valid_admin", "admin", true},
		{"valid_viewer", "viewer", true},
		{"valid_custom", "custom-role", true},
		{"invalid_injection", "admin\nX-Injected: header", false},
		{"invalid_special_chars", "role<script>", false},
		{"invalid_path", "../admin", false},
		{"empty", "", false},
		{"too_long", string(make([]byte, 100)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateRole(tt.role)
			if result != tt.isValid {
				t.Errorf("validateRole(%q) = %v, want %v", tt.role, result, tt.isValid)
			}
		})
	}
}

func TestSanitizeRoles(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"valid_roles", []string{"admin", "user"}, []string{"admin", "user"}},
		{"with_invalid", []string{"admin", "invalid\nheader", "user"}, []string{"admin", "user"}},
		{"all_invalid", []string{"../admin", "role<script>"}, []string{"user"}}, // Defaults to user
		{"empty_list", []string{}, []string{"user"}},                            // Defaults to user
		{"normalized_case", []string{"ADMIN", "User"}, []string{"admin", "user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRoles(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("sanitizeRoles(%v) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, r := range result {
				if r != tt.expected[i] {
					t.Errorf("sanitizeRoles(%v)[%d] = %q, want %q", tt.input, i, r, tt.expected[i])
				}
			}
		})
	}
}

func TestHeaderRoleInjection(t *testing.T) {
	config := &DelegatedAuthConfig{
		IdentityHeader: "X-User-Email",
		HeaderMapping: map[string]string{
			"roles": "X-User-Roles",
		},
	}

	extractor, err := NewDelegatedAuthExtractor(config)
	if err != nil {
		t.Fatalf("NewDelegatedAuthExtractor() error = %v", err)
	}

	// Test with malicious role header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-Email", "user@example.com")
	req.Header.Set("X-User-Roles", "admin\nX-Injected: evil,user,../../../etc/passwd")

	principal, err := extractor.ExtractAuth(context.Background(), req)
	if err != nil {
		t.Fatalf("ExtractAuth() error = %v", err)
	}

	// Verify malicious roles are filtered out
	for _, role := range principal.Roles {
		if role == "admin\nX-Injected: evil" || role == "../../../etc/passwd" {
			t.Errorf("Malicious role not filtered: %q", role)
		}
	}

	// Should only have 'admin' and 'user' from the original header (filtered)
	hasValidRole := false
	for _, role := range principal.Roles {
		if role == "admin" || role == "user" {
			hasValidRole = true
		}
	}
	if !hasValidRole {
		t.Error("Expected at least one valid role after sanitization")
	}
}
