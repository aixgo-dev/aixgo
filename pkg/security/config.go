package security

import (
	"fmt"
	"log"
)

// AuthMode defines how authentication is handled
type AuthMode string

const (
	// AuthModeDisabled - No authentication (local dev only)
	AuthModeDisabled AuthMode = "disabled"

	// AuthModeDelegated - Infrastructure handles auth (IAP, Istio, etc.)
	AuthModeDelegated AuthMode = "delegated"

	// AuthModeBuiltin - Application validates credentials
	AuthModeBuiltin AuthMode = "builtin"

	// AuthModeHybrid - Both infrastructure and app validation
	AuthModeHybrid AuthMode = "hybrid"
)

// SecurityConfig holds all security-related configuration
type SecurityConfig struct {
	// Environment: development, staging, production
	Environment string `yaml:"environment" env:"ENVIRONMENT"`

	// Auth mode selection
	AuthMode AuthMode `yaml:"auth_mode" env:"AUTH_MODE"`

	// Delegated auth configuration
	DelegatedAuth *DelegatedAuthConfig `yaml:"delegated_auth,omitempty"`

	// Builtin auth configuration
	BuiltinAuth *BuiltinAuthConfig `yaml:"builtin_auth,omitempty"`

	// Authorization configuration
	Authorization *AuthorizationConfig `yaml:"authorization"`

	// Audit configuration
	Audit *AuditConfig `yaml:"audit"`
}

// DelegatedAuthConfig for infrastructure-provided auth
type DelegatedAuthConfig struct {
	// Header containing identity
	IdentityHeader string `yaml:"identity_header"`

	// IAP configuration
	IAP *IAPConfig `yaml:"iap,omitempty"`

	// Header mapping for extracting identity fields
	HeaderMapping map[string]string `yaml:"header_mapping,omitempty"`
}

// IAPConfig for Google Cloud IAP
type IAPConfig struct {
	Enabled   bool   `yaml:"enabled"`
	VerifyJWT bool   `yaml:"verify_jwt"`
	Audience  string `yaml:"audience,omitempty"`
}

// BuiltinAuthConfig for app-level auth
type BuiltinAuthConfig struct {
	Method string `yaml:"method"` // api_key, jwt, oauth2

	// API Key configuration
	APIKeys *APIKeyConfig `yaml:"api_keys,omitempty"`
}

// APIKeyConfig for API key auth
type APIKeyConfig struct {
	Source    string `yaml:"source"` // environment, file
	FilePath  string `yaml:"file_path,omitempty"`
	EnvPrefix string `yaml:"env_prefix,omitempty"`
}

// AuthorizationConfig for access control
type AuthorizationConfig struct {
	Enabled     bool   `yaml:"enabled"`
	DefaultDeny bool   `yaml:"default_deny"`
	PolicyFile  string `yaml:"policy_file,omitempty"`
}

// AuditConfig for audit logging
type AuditConfig struct {
	Enabled          bool        `yaml:"enabled"`
	Backend          string      `yaml:"backend"` // memory, json, syslog
	LogAuthDecisions bool        `yaml:"log_auth_decisions"`
	SIEM             *SIEMConfig `yaml:"siem,omitempty"`
}

// DefaultSecurityConfig returns environment-appropriate defaults
func DefaultSecurityConfig(environment string) *SecurityConfig {
	switch environment {
	case "production":
		return &SecurityConfig{
			Environment: "production",
			AuthMode:    AuthModeBuiltin,
			Authorization: &AuthorizationConfig{
				Enabled:     true,
				DefaultDeny: true,
			},
			Audit: &AuditConfig{
				Enabled:          true,
				Backend:          "json",
				LogAuthDecisions: true,
			},
		}

	case "staging":
		return &SecurityConfig{
			Environment: "staging",
			AuthMode:    AuthModeBuiltin,
			Authorization: &AuthorizationConfig{
				Enabled:     true,
				DefaultDeny: true,
			},
			Audit: &AuditConfig{
				Enabled:          true,
				Backend:          "json",
				LogAuthDecisions: true,
			},
		}

	case "development":
		return &SecurityConfig{
			Environment: "development",
			AuthMode:    AuthModeDisabled,
			Authorization: &AuthorizationConfig{
				Enabled: false,
			},
			Audit: &AuditConfig{
				Enabled: false,
			},
		}

	default:
		// Unknown environment - be secure
		log.Printf("WARNING: Unknown environment '%s', using production defaults", environment)
		return DefaultSecurityConfig("production")
	}
}

// Validate checks security configuration for issues
func (sc *SecurityConfig) Validate() error {
	// Production must have auth
	if sc.Environment == "production" && sc.AuthMode == AuthModeDisabled {
		return fmt.Errorf("SECURITY ERROR: auth_mode=disabled is not allowed in production")
	}

	// Delegated mode needs configuration
	if sc.AuthMode == AuthModeDelegated && sc.DelegatedAuth == nil {
		return fmt.Errorf("delegated_auth configuration required when auth_mode=delegated")
	}

	// Builtin mode needs configuration
	if sc.AuthMode == AuthModeBuiltin && sc.BuiltinAuth == nil {
		return fmt.Errorf("builtin_auth configuration required when auth_mode=builtin")
	}

	// Hybrid needs both
	if sc.AuthMode == AuthModeHybrid {
		if sc.DelegatedAuth == nil || sc.BuiltinAuth == nil {
			return fmt.Errorf("both delegated_auth and builtin_auth required for hybrid mode")
		}
	}

	return nil
}

// PrintSecuritySummary logs the security configuration
func (sc *SecurityConfig) PrintSecuritySummary() {
	log.Println("=== SECURITY CONFIGURATION ===")
	log.Printf("Environment: %s", sc.Environment)
	log.Printf("Auth Mode: %s", sc.AuthMode)

	if sc.Authorization != nil {
		log.Printf("Authorization: %v", sc.Authorization.Enabled)
	}

	if sc.Audit != nil {
		log.Printf("Audit Logging: %v", sc.Audit.Enabled)
	}

	if sc.AuthMode == AuthModeDisabled {
		log.Println("⚠️  WARNING: AUTHENTICATION IS DISABLED")
		log.Println("⚠️  This configuration is NOT suitable for production")
	}

	log.Println("==============================")
}
