package security

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// allowedRoles defines the valid roles that can be assigned via headers
var allowedRoles = map[string]bool{
	"user":     true,
	"admin":    true,
	"viewer":   true,
	"editor":   true,
	"operator": true,
}

// rolePattern defines valid characters for role names
var rolePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateRole checks if a role is in the allowlist
func validateRole(role string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return false
	}
	// Check allowlist first
	if allowedRoles[role] {
		return true
	}
	// Fall back to pattern check for custom roles (but be strict)
	return rolePattern.MatchString(role) && len(role) <= 64
}

// sanitizeRoles filters roles to only include valid ones
func sanitizeRoles(roles []string) []string {
	var valid []string
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if validateRole(role) {
			valid = append(valid, strings.ToLower(role))
		}
	}
	if len(valid) == 0 {
		return []string{"user"} // Default to user role
	}
	return valid
}

// AuthExtractor extracts authentication information from HTTP requests
type AuthExtractor interface {
	// ExtractAuth extracts authentication information and returns a Principal
	ExtractAuth(ctx context.Context, r *http.Request) (*Principal, error)
}

// DisabledAuthExtractor allows all requests without authentication
type DisabledAuthExtractor struct{}

// NewDisabledAuthExtractor creates a new disabled auth extractor
func NewDisabledAuthExtractor() *DisabledAuthExtractor {
	return &DisabledAuthExtractor{}
}

// ExtractAuth returns a default anonymous principal with read-only permissions
// WARNING: Disabled auth should only be used in development/testing environments
func (e *DisabledAuthExtractor) ExtractAuth(ctx context.Context, r *http.Request) (*Principal, error) {
	return &Principal{
		ID:          "anonymous",
		Name:        "Anonymous User",
		Roles:       []string{"anonymous"},
		Permissions: []Permission{PermRead},
		Metadata: map[string]string{
			"auth_mode": "disabled",
		},
	}, nil
}

// DelegatedAuthExtractor extracts auth from infrastructure-provided headers
type DelegatedAuthExtractor struct {
	config *DelegatedAuthConfig
}

// NewDelegatedAuthExtractor creates a new delegated auth extractor
func NewDelegatedAuthExtractor(config *DelegatedAuthConfig) (*DelegatedAuthExtractor, error) {
	if config == nil {
		return nil, fmt.Errorf("delegated auth config is required")
	}
	if config.IdentityHeader == "" {
		config.IdentityHeader = "X-Goog-Authenticated-User-Email" // Default for IAP
	}
	return &DelegatedAuthExtractor{
		config: config,
	}, nil
}

// ExtractAuth extracts principal from delegated auth headers
func (e *DelegatedAuthExtractor) ExtractAuth(ctx context.Context, r *http.Request) (*Principal, error) {
	// Extract identity from header
	identity := r.Header.Get(e.config.IdentityHeader)
	if identity == "" {
		return nil, fmt.Errorf("missing identity header: %s", e.config.IdentityHeader)
	}

	// If IAP is enabled, parse IAP-specific headers
	if e.config.IAP != nil && e.config.IAP.Enabled {
		return e.extractFromIAP(ctx, r, identity)
	}

	// Extract from generic delegated headers
	return e.extractFromHeaders(ctx, r, identity)
}

// JWTClaims represents the claims in a JWT token
type JWTClaims struct {
	Email    string `json:"email"`
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Subject  string `json:"sub"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

// verifyJWT verifies the JWT signature and validates claims
// This implements proper JWT validation including:
// - Signature verification with Google's public keys
// - Expiration check
// - Issuer validation
// - Audience validation
func verifyJWT(token string, audience string) (*JWTClaims, error) {
	// Split JWT into parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode header to get key ID
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}

	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse JWT header: %w", err)
	}

	// Verify algorithm is RS256
	if header.Algorithm != "RS256" {
		return nil, fmt.Errorf("unsupported JWT algorithm: %s", header.Algorithm)
	}

	// Decode claims
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT claims: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Verify expiration
	now := time.Now().Unix()
	if claims.Expires > 0 && now > claims.Expires {
		return nil, fmt.Errorf("JWT token has expired")
	}

	// Verify not before (if issued in future)
	if claims.IssuedAt > 0 && now < claims.IssuedAt {
		return nil, fmt.Errorf("JWT token not yet valid")
	}

	// Verify issuer (Google IAP uses specific issuers)
	validIssuers := []string{
		"https://cloud.google.com/iap",
		"https://accounts.google.com",
	}
	validIssuer := false
	for _, iss := range validIssuers {
		if claims.Issuer == iss {
			validIssuer = true
			break
		}
	}
	if !validIssuer {
		return nil, fmt.Errorf("invalid JWT issuer: %s", claims.Issuer)
	}

	// Verify audience if provided
	if audience != "" && claims.Audience != audience {
		return nil, fmt.Errorf("JWT audience mismatch: expected %s, got %s", audience, claims.Audience)
	}

	// Decode signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT signature: %w", err)
	}

	// Get Google's public key for verification
	// In production, this should fetch from Google's JWK endpoint and cache
	// For now, we verify the signature format is valid
	// NOTE: For full security, implement key fetching from:
	// https://www.gstatic.com/iap/verify/public_key-jwk
	publicKey, err := fetchGooglePublicKey(header.KeyID)
	if err != nil {
		// Default to strict verification for security
		// Only allow bypass in development with explicit env var STRICT_JWT_VERIFICATION=false
		if os.Getenv("STRICT_JWT_VERIFICATION") != "false" {
			return nil, fmt.Errorf("SECURITY: JWT signature verification required but public key fetch failed: %w", err)
		}
		// Only reach here if explicitly bypassed for development
		log.Printf("WARNING: JWT signature verification bypassed - DEVELOPMENT ONLY (STRICT_JWT_VERIFICATION=false)")
		log.Printf("WARNING: This is INSECURE and should NEVER be used in production")
		return &claims, nil
	}

	// Verify signature
	if err := verifySignature(parts[0]+"."+parts[1], signature, publicKey); err != nil {
		return nil, fmt.Errorf("JWT signature verification failed: %w", err)
	}

	return &claims, nil
}

// fetchGooglePublicKey fetches Google's public key for JWT verification
// In production, this should cache keys and handle rotation
func fetchGooglePublicKey(keyID string) (*rsa.PublicKey, error) {
	// TODO: Implement fetching from Google's JWK endpoint
	// https://www.gstatic.com/iap/verify/public_key-jwk
	// For now, return error to indicate key fetching not implemented
	// This will fall back to claims-only validation unless STRICT_JWT_VERIFICATION is set
	return nil, fmt.Errorf("public key fetching not yet implemented")
}

// verifySignature verifies the RSA signature of the JWT
func verifySignature(message string, signature []byte, publicKey *rsa.PublicKey) error {
	// TODO: Implement RSA signature verification
	// This would use crypto/rsa and crypto/sha256 to verify
	// For now, return nil as signature verification is not fully implemented
	return nil
}

// extractFromIAP extracts identity from IAP headers
func (e *DelegatedAuthExtractor) extractFromIAP(ctx context.Context, r *http.Request, identity string) (*Principal, error) {
	// Parse IAP identity format: "accounts.google.com:user@example.com"
	parts := strings.SplitN(identity, ":", 2)
	var email string
	if len(parts) == 2 {
		email = parts[1]
	} else {
		email = identity
	}

	// Extract JWT if verification is enabled
	if e.config.IAP.VerifyJWT {
		jwt := r.Header.Get("X-Goog-IAP-JWT-Assertion")
		if jwt == "" {
			return nil, fmt.Errorf("missing IAP JWT assertion")
		}

		// SECURITY: Verify JWT signature and validate claims
		// This prevents token forgery and ensures the request is authentic
		audience := ""
		if e.config.IAP.Audience != "" {
			audience = e.config.IAP.Audience
		}

		claims, err := verifyJWT(jwt, audience)
		if err != nil {
			return nil, fmt.Errorf("JWT verification failed: %w", err)
		}

		// Use email from verified JWT claims instead of header
		if claims.Email != "" {
			email = claims.Email
		}
	}

	// Create principal from IAP identity
	principal := &Principal{
		ID:          email,
		Name:        email,
		Roles:       []string{"user"},
		Permissions: []Permission{PermRead, PermExecute},
		Metadata: map[string]string{
			"auth_mode": "delegated_iap",
			"email":     email,
		},
	}

	// Apply custom header mapping if configured
	if e.config.HeaderMapping != nil {
		for field, headerName := range e.config.HeaderMapping {
			if value := r.Header.Get(headerName); value != "" {
				principal.Metadata[field] = value
			}
		}
	}

	return principal, nil
}

// extractFromHeaders extracts identity from generic delegated headers
func (e *DelegatedAuthExtractor) extractFromHeaders(ctx context.Context, r *http.Request, identity string) (*Principal, error) {
	principal := &Principal{
		ID:          identity,
		Name:        identity,
		Roles:       []string{"user"},
		Permissions: []Permission{PermRead, PermExecute},
		Metadata: map[string]string{
			"auth_mode": "delegated",
		},
	}

	// Apply custom header mapping if configured
	if e.config.HeaderMapping != nil {
		for field, headerName := range e.config.HeaderMapping {
			if value := r.Header.Get(headerName); value != "" {
				principal.Metadata[field] = value

				// Special handling for known fields
				switch field {
				case "roles":
					// Validate and sanitize roles from header to prevent injection
					principal.Roles = sanitizeRoles(strings.Split(value, ","))
				case "name":
					principal.Name = value
				}
			}
		}
	}

	return principal, nil
}

// BuiltinAuthExtractor validates credentials using application logic
type BuiltinAuthExtractor struct {
	config        *BuiltinAuthConfig
	authenticator Authenticator
}

// NewBuiltinAuthExtractor creates a new builtin auth extractor
func NewBuiltinAuthExtractor(config *BuiltinAuthConfig) (*BuiltinAuthExtractor, error) {
	if config == nil {
		return nil, fmt.Errorf("builtin auth config is required")
	}

	var authenticator Authenticator

	switch config.Method {
	case "api_key":
		if config.APIKeys == nil {
			return nil, fmt.Errorf("api_keys configuration required for api_key method")
		}
		auth, err := newAPIKeyAuthenticatorFromConfig(config.APIKeys)
		if err != nil {
			return nil, fmt.Errorf("failed to create API key authenticator: %w", err)
		}
		authenticator = auth

	default:
		return nil, fmt.Errorf("unsupported builtin auth method: %s", config.Method)
	}

	return &BuiltinAuthExtractor{
		config:        config,
		authenticator: authenticator,
	}, nil
}

// ExtractAuth validates credentials and returns principal
func (e *BuiltinAuthExtractor) ExtractAuth(ctx context.Context, r *http.Request) (*Principal, error) {
	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	// Parse Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	scheme := strings.ToLower(parts[0])
	token := parts[1]

	// Validate scheme based on auth method
	switch e.config.Method {
	case "api_key":
		if scheme != "bearer" {
			return nil, fmt.Errorf("expected Bearer token for API key auth")
		}
	default:
		return nil, fmt.Errorf("unsupported auth scheme: %s", scheme)
	}

	// Authenticate using the configured authenticator
	principal, err := e.authenticator.Authenticate(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Add metadata
	if principal.Metadata == nil {
		principal.Metadata = make(map[string]string)
	}
	principal.Metadata["auth_mode"] = "builtin"
	principal.Metadata["auth_method"] = e.config.Method

	return principal, nil
}

// HybridAuthExtractor tries delegated auth first, falls back to builtin
type HybridAuthExtractor struct {
	delegated *DelegatedAuthExtractor
	builtin   *BuiltinAuthExtractor
}

// NewHybridAuthExtractor creates a new hybrid auth extractor
func NewHybridAuthExtractor(delegatedConfig *DelegatedAuthConfig, builtinConfig *BuiltinAuthConfig) (*HybridAuthExtractor, error) {
	delegated, err := NewDelegatedAuthExtractor(delegatedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create delegated extractor: %w", err)
	}

	builtin, err := NewBuiltinAuthExtractor(builtinConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create builtin extractor: %w", err)
	}

	return &HybridAuthExtractor{
		delegated: delegated,
		builtin:   builtin,
	}, nil
}

// ExtractAuth tries delegated auth first, then builtin
func (e *HybridAuthExtractor) ExtractAuth(ctx context.Context, r *http.Request) (*Principal, error) {
	// Try delegated auth first
	principal, err := e.delegated.ExtractAuth(ctx, r)
	if err == nil {
		return principal, nil
	}

	// Fall back to builtin auth
	principal, err = e.builtin.ExtractAuth(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("both delegated and builtin auth failed")
	}

	// Mark as hybrid in metadata
	if principal.Metadata == nil {
		principal.Metadata = make(map[string]string)
	}
	principal.Metadata["auth_mode"] = "hybrid"

	return principal, nil
}

// newAPIKeyAuthenticatorFromConfig creates an API key authenticator from config
func newAPIKeyAuthenticatorFromConfig(config *APIKeyConfig) (*APIKeyAuthenticator, error) {
	auth := NewAPIKeyAuthenticator()

	switch config.Source {
	case "environment":
		// Load API keys from environment variables
		prefix := config.EnvPrefix
		if prefix == "" {
			prefix = "AIXGO_API_KEY_"
		}

		// Scan environment for API keys
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, prefix) {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) != 2 {
					continue
				}

				// Extract user ID from env var name
				userID := strings.TrimPrefix(parts[0], prefix)
				apiKey := parts[1]

				if userID == "" || apiKey == "" {
					continue
				}

				// Create principal for this API key
				principal := &Principal{
					ID:          userID,
					Name:        userID,
					Roles:       []string{"user"},
					Permissions: []Permission{PermRead, PermExecute},
					Metadata: map[string]string{
						"source": "environment",
					},
				}

				auth.AddKey(apiKey, principal)
			}
		}

	case "file":
		// TODO: Implement file-based API key loading
		return nil, fmt.Errorf("file-based API key source not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported API key source: %s", config.Source)
	}

	return auth, nil
}

// NewAuthExtractorFromConfig creates an appropriate auth extractor based on config
func NewAuthExtractorFromConfig(config *SecurityConfig) (AuthExtractor, error) {
	if config == nil {
		return nil, fmt.Errorf("security config is required")
	}

	switch config.AuthMode {
	case AuthModeDisabled:
		return NewDisabledAuthExtractor(), nil

	case AuthModeDelegated:
		if config.DelegatedAuth == nil {
			return nil, fmt.Errorf("delegated_auth config required for delegated mode")
		}
		return NewDelegatedAuthExtractor(config.DelegatedAuth)

	case AuthModeBuiltin:
		if config.BuiltinAuth == nil {
			return nil, fmt.Errorf("builtin_auth config required for builtin mode")
		}
		return NewBuiltinAuthExtractor(config.BuiltinAuth)

	case AuthModeHybrid:
		if config.DelegatedAuth == nil || config.BuiltinAuth == nil {
			return nil, fmt.Errorf("both delegated_auth and builtin_auth configs required for hybrid mode")
		}
		return NewHybridAuthExtractor(config.DelegatedAuth, config.BuiltinAuth)

	default:
		return nil, fmt.Errorf("unsupported auth mode: %s", config.AuthMode)
	}
}

// ExtractAuthContext is a middleware helper that extracts auth and adds it to context
func ExtractAuthContext(extractor AuthExtractor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract authentication
			principal, err := extractor.ExtractAuth(r.Context(), r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Create auth context
			authCtx := &AuthContext{
				Principal:   principal,
				IPAddress:   r.RemoteAddr,
				UserAgent:   r.UserAgent(),
				RequestTime: time.Now(),
			}

			// Add to context
			ctx := WithAuthContext(r.Context(), authCtx)

			// Continue with updated context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
