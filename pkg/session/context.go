package session

import (
	"context"
	"errors"
)

// SessionKey is the context key for storing sessions.
// Following the RuntimeKey pattern from internal/agent/types.go.
type SessionKey struct{}

// ErrSessionNotInContext is returned when no session is found in context.
var ErrSessionNotInContext = errors.New("session not found in context")

// SessionFromContext retrieves a session from the context.
// Returns the session and true if found, or nil and false if not present.
func SessionFromContext(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(SessionKey{}).(Session)
	return sess, ok
}

// MustSessionFromContext retrieves a session from context and panics if not found.
// Prefer SessionFromContext with explicit error handling in production code.
func MustSessionFromContext(ctx context.Context) Session {
	sess, ok := SessionFromContext(ctx)
	if !ok {
		panic("session not found in context")
	}
	return sess
}

// ContextWithSession adds a session to the context.
func ContextWithSession(ctx context.Context, sess Session) context.Context {
	return context.WithValue(ctx, SessionKey{}, sess)
}
