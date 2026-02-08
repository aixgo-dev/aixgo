package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages session lifecycle.
// Manager is safe for concurrent use.
type Manager interface {
	// Create creates a new session for an agent.
	Create(ctx context.Context, agentName string, opts CreateOptions) (Session, error)

	// Get retrieves an existing session by ID.
	// Returns ErrSessionNotFound if the session doesn't exist.
	Get(ctx context.Context, sessionID string) (Session, error)

	// GetOrCreate returns an existing session or creates a new one.
	// If userID is provided, it looks for existing sessions for that user.
	GetOrCreate(ctx context.Context, agentName, userID string) (Session, error)

	// List returns sessions for an agent matching the filter options.
	List(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error)

	// Delete removes a session and all its data.
	Delete(ctx context.Context, sessionID string) error

	// Close releases resources held by the manager.
	Close() error
}

// CreateOptions configures session creation.
type CreateOptions struct {
	// UserID identifies the user for this session.
	UserID string
	// Metadata contains optional session metadata.
	Metadata map[string]any
}

// managerImpl is the concrete implementation of Manager.
type managerImpl struct {
	backend  StorageBackend
	sessions map[string]*sessionImpl
	mu       sync.RWMutex
}

// NewManager creates a new session manager with the given storage backend.
func NewManager(backend StorageBackend) Manager {
	return &managerImpl{
		backend:  backend,
		sessions: make(map[string]*sessionImpl),
	}
}

// Create creates a new session for an agent.
func (m *managerImpl) Create(ctx context.Context, agentName string, opts CreateOptions) (Session, error) {
	now := time.Now().UTC()

	meta := &SessionMetadata{
		ID:           uuid.New().String(),
		AgentName:    agentName,
		UserID:       opts.UserID,
		CreatedAt:    now,
		UpdatedAt:    now,
		MessageCount: 0,
	}

	// Persist metadata
	if err := m.backend.SaveSession(ctx, meta); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	// Create session instance
	sess := newSession(meta, m.backend)

	// Cache the session
	m.mu.Lock()
	m.sessions[meta.ID] = sess
	m.mu.Unlock()

	return sess, nil
}

// Get retrieves an existing session by ID.
func (m *managerImpl) Get(ctx context.Context, sessionID string) (Session, error) {
	// Check cache first
	m.mu.RLock()
	if sess, ok := m.sessions[sessionID]; ok {
		m.mu.RUnlock()
		return sess, nil
	}
	m.mu.RUnlock()

	// Load from storage
	meta, err := m.backend.LoadSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Create session instance
	sess := newSession(meta, m.backend)

	// Load entries into cache
	entries, err := m.backend.LoadEntries(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	sess.entries = entries

	// Cache the session
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()

	return sess, nil
}

// GetOrCreate returns an existing session or creates a new one.
func (m *managerImpl) GetOrCreate(ctx context.Context, agentName, userID string) (Session, error) {
	// If userID is provided, look for existing sessions
	if userID != "" {
		sessions, err := m.backend.ListSessions(ctx, agentName, ListOptions{
			UserID: userID,
			Limit:  1,
		})
		if err != nil {
			return nil, fmt.Errorf("list sessions: %w", err)
		}

		if len(sessions) > 0 {
			return m.Get(ctx, sessions[0].ID)
		}
	}

	// Create a new session
	return m.Create(ctx, agentName, CreateOptions{
		UserID: userID,
	})
}

// List returns sessions for an agent matching the filter options.
func (m *managerImpl) List(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error) {
	return m.backend.ListSessions(ctx, agentName, opts)
}

// Delete removes a session and all its data.
func (m *managerImpl) Delete(ctx context.Context, sessionID string) error {
	// Remove from cache
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Delete from storage
	return m.backend.DeleteSession(ctx, sessionID)
}

// Close releases resources held by the manager.
func (m *managerImpl) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all cached sessions
	ctx := context.Background()
	for _, sess := range m.sessions {
		_ = sess.Close(ctx)
	}
	m.sessions = make(map[string]*sessionImpl)

	// Close the backend
	return m.backend.Close()
}
