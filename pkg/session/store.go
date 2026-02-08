package session

import (
	"context"
	"errors"
)

// Common errors for storage operations.
var (
	// ErrSessionNotFound is returned when a session doesn't exist.
	ErrSessionNotFound = errors.New("session not found")
	// ErrCheckpointNotFound is returned when a checkpoint doesn't exist.
	ErrCheckpointNotFound = errors.New("checkpoint not found")
	// ErrStorageClosed is returned when operating on a closed storage backend.
	ErrStorageClosed = errors.New("storage backend is closed")
)

// StorageBackend abstracts session persistence.
// Implementations must be safe for concurrent use.
type StorageBackend interface {
	// SaveSession creates or updates session metadata.
	SaveSession(ctx context.Context, meta *SessionMetadata) error

	// LoadSession retrieves session metadata by ID.
	// Returns ErrSessionNotFound if the session doesn't exist.
	LoadSession(ctx context.Context, sessionID string) (*SessionMetadata, error)

	// DeleteSession removes a session and all its entries.
	DeleteSession(ctx context.Context, sessionID string) error

	// ListSessions returns sessions for an agent matching the filter options.
	ListSessions(ctx context.Context, agentName string, opts ListOptions) ([]*SessionMetadata, error)

	// AppendEntry adds an entry to a session (append-only).
	AppendEntry(ctx context.Context, sessionID string, entry *SessionEntry) error

	// LoadEntries retrieves all entries for a session in order.
	LoadEntries(ctx context.Context, sessionID string) ([]*SessionEntry, error)

	// SaveCheckpoint stores a checkpoint.
	SaveCheckpoint(ctx context.Context, checkpoint *Checkpoint) error

	// LoadCheckpoint retrieves a checkpoint by ID.
	// Returns ErrCheckpointNotFound if the checkpoint doesn't exist.
	LoadCheckpoint(ctx context.Context, checkpointID string) (*Checkpoint, error)

	// Close releases any resources held by the backend.
	Close() error
}

// ListOptions provides filtering for session listing.
type ListOptions struct {
	// UserID filters sessions by user.
	UserID string
	// Limit caps the number of results.
	Limit int
	// Offset skips the first N results.
	Offset int
}
