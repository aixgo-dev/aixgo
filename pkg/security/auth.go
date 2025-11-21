package security

import (
	"context"
	"crypto/subtle"
	"fmt"
	"sync"
	"time"
)

// Permission represents a security permission
type Permission string

const (
	PermRead    Permission = "read"
	PermWrite   Permission = "write"
	PermExecute Permission = "execute"
	PermAdmin   Permission = "admin"
)

// Principal represents an authenticated entity
type Principal struct {
	ID          string
	Name        string
	Roles       []string
	Permissions []Permission
	Metadata    map[string]string
}

// AuthContext contains authentication and authorization information
type AuthContext struct {
	Principal   *Principal
	SessionID   string
	IPAddress   string
	UserAgent   string
	RequestTime time.Time
}

// Authenticator handles authentication of requests
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*Principal, error)
}

// Authorizer handles authorization decisions
type Authorizer interface {
	Authorize(ctx context.Context, principal *Principal, resource string, permission Permission) error
}

// APIKeyAuthenticator implements simple API key authentication
type APIKeyAuthenticator struct {
	keys map[string]*Principal
	mu   sync.RWMutex
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator() *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		keys: make(map[string]*Principal),
	}
}

// AddKey registers an API key with associated principal
func (a *APIKeyAuthenticator) AddKey(apiKey string, principal *Principal) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keys[apiKey] = principal
}

// Authenticate verifies an API key and returns the associated principal
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, token string) (*Principal, error) {
	if token == "" {
		return nil, fmt.Errorf("missing authentication token")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Use constant-time comparison to prevent timing attacks
	for key, principal := range a.keys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(token)) == 1 {
			return principal, nil
		}
	}

	return nil, fmt.Errorf("invalid authentication token")
}

// RBACAuthorizer implements role-based access control
type RBACAuthorizer struct {
	rolePermissions map[string][]Permission
	mu              sync.RWMutex
}

// NewRBACAuthorizer creates a new RBAC authorizer
func NewRBACAuthorizer() *RBACAuthorizer {
	auth := &RBACAuthorizer{
		rolePermissions: make(map[string][]Permission),
	}

	// Set up default roles
	auth.rolePermissions["admin"] = []Permission{PermRead, PermWrite, PermExecute, PermAdmin}
	auth.rolePermissions["user"] = []Permission{PermRead, PermExecute}
	auth.rolePermissions["readonly"] = []Permission{PermRead}

	return auth
}

// AddRolePermission adds a permission to a role
func (a *RBACAuthorizer) AddRolePermission(role string, perm Permission) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.rolePermissions[role]; !exists {
		a.rolePermissions[role] = []Permission{}
	}

	// Check if permission already exists
	for _, existingPerm := range a.rolePermissions[role] {
		if existingPerm == perm {
			return
		}
	}

	a.rolePermissions[role] = append(a.rolePermissions[role], perm)
}

// Authorize checks if a principal has permission for a resource
func (a *RBACAuthorizer) Authorize(ctx context.Context, principal *Principal, resource string, permission Permission) error {
	if principal == nil {
		return fmt.Errorf("no principal provided")
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Check if principal has the permission directly
	for _, perm := range principal.Permissions {
		if perm == permission || perm == PermAdmin {
			return nil
		}
	}

	// Check if any of the principal's roles have the permission
	for _, role := range principal.Roles {
		if rolePerms, exists := a.rolePermissions[role]; exists {
			for _, perm := range rolePerms {
				if perm == permission || perm == PermAdmin {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("access denied: insufficient permissions")
}

// AllowAllAuthorizer allows all requests (use only for testing/development)
type AllowAllAuthorizer struct{}

// NewAllowAllAuthorizer creates a new allow-all authorizer (INSECURE - testing only)
func NewAllowAllAuthorizer() *AllowAllAuthorizer {
	return &AllowAllAuthorizer{}
}

// Authorize always returns nil (allows all access)
func (a *AllowAllAuthorizer) Authorize(ctx context.Context, principal *Principal, resource string, permission Permission) error {
	return nil
}

// NoAuthAuthenticator allows all requests without authentication (INSECURE - testing only)
type NoAuthAuthenticator struct {
	defaultPrincipal *Principal
}

// NewNoAuthAuthenticator creates a no-auth authenticator (INSECURE - testing only)
func NewNoAuthAuthenticator() *NoAuthAuthenticator {
	return &NoAuthAuthenticator{
		defaultPrincipal: &Principal{
			ID:          "anonymous",
			Name:        "Anonymous User",
			Roles:       []string{"admin"},
			Permissions: []Permission{PermRead, PermWrite, PermExecute, PermAdmin},
		},
	}
}

// Authenticate returns the default principal
func (a *NoAuthAuthenticator) Authenticate(ctx context.Context, token string) (*Principal, error) {
	return a.defaultPrincipal, nil
}

// contextKey is a private type for context keys
type contextKey string

const (
	authContextKey contextKey = "auth_context"
)

// WithAuthContext adds authentication context to the context
func WithAuthContext(ctx context.Context, authCtx *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey, authCtx)
}

// GetAuthContext retrieves authentication context from the context
func GetAuthContext(ctx context.Context) (*AuthContext, error) {
	authCtx, ok := ctx.Value(authContextKey).(*AuthContext)
	if !ok || authCtx == nil {
		return nil, fmt.Errorf("no authentication context found")
	}
	return authCtx, nil
}

// GetPrincipal retrieves the principal from the context
func GetPrincipal(ctx context.Context) (*Principal, error) {
	authCtx, err := GetAuthContext(ctx)
	if err != nil {
		return nil, err
	}
	return authCtx.Principal, nil
}
