package security

import (
	"strings"
	"testing"
)

func TestDefaultSecurityConfig(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantMode    AuthMode
		wantAuthz   bool
		wantAudit   bool
	}{
		{
			name:        "production defaults",
			environment: "production",
			wantMode:    AuthModeBuiltin,
			wantAuthz:   true,
			wantAudit:   true,
		},
		{
			name:        "staging defaults",
			environment: "staging",
			wantMode:    AuthModeBuiltin,
			wantAuthz:   true,
			wantAudit:   true,
		},
		{
			name:        "development defaults",
			environment: "development",
			wantMode:    AuthModeDisabled,
			wantAuthz:   false,
			wantAudit:   false,
		},
		{
			name:        "unknown environment uses production defaults",
			environment: "unknown",
			wantMode:    AuthModeBuiltin,
			wantAuthz:   true,
			wantAudit:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultSecurityConfig(tt.environment)

			if config.AuthMode != tt.wantMode {
				t.Errorf("AuthMode = %v, want %v", config.AuthMode, tt.wantMode)
			}

			if config.Authorization.Enabled != tt.wantAuthz {
				t.Errorf("Authorization.Enabled = %v, want %v", config.Authorization.Enabled, tt.wantAuthz)
			}

			if config.Audit.Enabled != tt.wantAudit {
				t.Errorf("Audit.Enabled = %v, want %v", config.Audit.Enabled, tt.wantAudit)
			}
		})
	}
}

func TestSecurityConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *SecurityConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid disabled config for development",
			config: &SecurityConfig{
				Environment: "development",
				AuthMode:    AuthModeDisabled,
			},
			wantErr: false,
		},
		{
			name: "invalid disabled config for production",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeDisabled,
			},
			wantErr: true,
			errMsg:  "auth_mode=disabled is not allowed in production",
		},
		{
			name: "valid builtin config",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeBuiltin,
				BuiltinAuth: &BuiltinAuthConfig{
					Method: "api_key",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid builtin config - missing config",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeBuiltin,
			},
			wantErr: true,
			errMsg:  "builtin_auth configuration required",
		},
		{
			name: "valid delegated config",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeDelegated,
				DelegatedAuth: &DelegatedAuthConfig{
					IdentityHeader: "X-User-Email",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid delegated config - missing config",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeDelegated,
			},
			wantErr: true,
			errMsg:  "delegated_auth configuration required",
		},
		{
			name: "valid hybrid config",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeHybrid,
				DelegatedAuth: &DelegatedAuthConfig{
					IdentityHeader: "X-User-Email",
				},
				BuiltinAuth: &BuiltinAuthConfig{
					Method: "api_key",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid hybrid config - missing delegated",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeHybrid,
				BuiltinAuth: &BuiltinAuthConfig{
					Method: "api_key",
				},
			},
			wantErr: true,
			errMsg:  "both delegated_auth and builtin_auth required",
		},
		{
			name: "invalid hybrid config - missing builtin",
			config: &SecurityConfig{
				Environment: "production",
				AuthMode:    AuthModeHybrid,
				DelegatedAuth: &DelegatedAuthConfig{
					IdentityHeader: "X-User-Email",
				},
			},
			wantErr: true,
			errMsg:  "both delegated_auth and builtin_auth required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestAuthModeValues(t *testing.T) {
	// Test that auth mode constants have expected values
	tests := []struct {
		mode AuthMode
		want string
	}{
		{AuthModeDisabled, "disabled"},
		{AuthModeDelegated, "delegated"},
		{AuthModeBuiltin, "builtin"},
		{AuthModeHybrid, "hybrid"},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if string(tt.mode) != tt.want {
				t.Errorf("AuthMode = %v, want %v", tt.mode, tt.want)
			}
		})
	}
}

func TestSecurityConfigPrintSecuritySummary(t *testing.T) {
	// Test that PrintSecuritySummary doesn't panic
	configs := []*SecurityConfig{
		DefaultSecurityConfig("development"),
		DefaultSecurityConfig("production"),
		{
			Environment: "production",
			AuthMode:    AuthModeDelegated,
			DelegatedAuth: &DelegatedAuthConfig{
				IdentityHeader: "X-User-Email",
			},
			Authorization: &AuthorizationConfig{
				Enabled: true,
			},
			Audit: &AuditConfig{
				Enabled: true,
			},
		},
	}

	for i, config := range configs {
		t.Run(config.Environment, func(t *testing.T) {
			// This should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PrintSecuritySummary() panicked for config %d: %v", i, r)
				}
			}()

			config.PrintSecuritySummary()
		})
	}
}
