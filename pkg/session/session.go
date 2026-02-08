package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/aixgo-dev/aixgo/agent"
	"github.com/google/uuid"
)

// Session represents an active agent session.
// Sessions are safe for concurrent use.
type Session interface {
	// ID returns the unique session identifier.
	ID() string

	// AgentName returns the name of the agent this session belongs to.
	AgentName() string

	// UserID returns the user identifier (may be empty).
	UserID() string

	// AppendMessage adds a message to the session history.
	AppendMessage(ctx context.Context, msg *agent.Message) error

	// GetMessages retrieves all messages in the session.
	GetMessages(ctx context.Context) ([]*agent.Message, error)

	// Checkpoint creates a restorable state snapshot.
	Checkpoint(ctx context.Context) (*Checkpoint, error)

	// Restore reverts the session to a previous checkpoint.
	Restore(ctx context.Context, checkpointID string) error

	// Close releases any resources held by the session.
	Close(ctx context.Context) error
}

// sessionImpl is the concrete implementation of Session.
type sessionImpl struct {
	meta    *SessionMetadata
	backend StorageBackend
	mu      sync.RWMutex

	// Cached entries for performance
	entries []*SessionEntry
	dirty   bool
}

// newSession creates a new session instance.
func newSession(meta *SessionMetadata, backend StorageBackend) *sessionImpl {
	return &sessionImpl{
		meta:    meta,
		backend: backend,
		entries: make([]*SessionEntry, 0),
	}
}

// ID returns the unique session identifier.
func (s *sessionImpl) ID() string {
	return s.meta.ID
}

// AgentName returns the name of the agent this session belongs to.
func (s *sessionImpl) AgentName() string {
	return s.meta.AgentName
}

// UserID returns the user identifier.
func (s *sessionImpl) UserID() string {
	return s.meta.UserID
}

// AppendMessage adds a message to the session history.
func (s *sessionImpl) AppendMessage(ctx context.Context, msg *agent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determine parent ID
	var parentID string
	if len(s.entries) > 0 {
		parentID = s.entries[len(s.entries)-1].ID
	}

	entry := &SessionEntry{
		ID:        uuid.New().String(),
		ParentID:  parentID,
		Timestamp: time.Now().UTC(),
		Type:      EntryTypeMessage,
		Data:      messageToData(msg),
	}

	// Persist to storage
	if err := s.backend.AppendEntry(ctx, s.meta.ID, entry); err != nil {
		return fmt.Errorf("append entry: %w", err)
	}

	// Update cache
	s.entries = append(s.entries, entry)
	s.meta.MessageCount++
	s.meta.UpdatedAt = time.Now().UTC()
	s.meta.CurrentLeaf = entry.ID
	s.dirty = true

	// Update metadata
	if err := s.backend.SaveSession(ctx, s.meta); err != nil {
		return fmt.Errorf("save session metadata: %w", err)
	}

	return nil
}

// GetMessages retrieves all messages in the session.
func (s *sessionImpl) GetMessages(ctx context.Context) ([]*agent.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Load entries if not cached
	if len(s.entries) == 0 {
		entries, err := s.backend.LoadEntries(ctx, s.meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load entries: %w", err)
		}
		s.mu.RUnlock()
		s.mu.Lock()
		s.entries = entries
		s.mu.Unlock()
		s.mu.RLock()
	}

	// Convert entries to messages
	messages := make([]*agent.Message, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Type == EntryTypeMessage {
			msg := dataToMessage(entry.Data)
			if msg != nil {
				messages = append(messages, msg)
			}
		}
	}

	return messages, nil
}

// Checkpoint creates a restorable state snapshot.
func (s *sessionImpl) Checkpoint(ctx context.Context) (*Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute checksum of all entries
	checksum := s.computeChecksum()

	// Get current leaf entry ID
	var entryID string
	if len(s.entries) > 0 {
		entryID = s.entries[len(s.entries)-1].ID
	}

	checkpoint := &Checkpoint{
		ID:        uuid.New().String(),
		SessionID: s.meta.ID,
		Timestamp: time.Now().UTC(),
		EntryID:   entryID,
		Checksum:  checksum,
	}

	if err := s.backend.SaveCheckpoint(ctx, checkpoint); err != nil {
		return nil, fmt.Errorf("save checkpoint: %w", err)
	}

	return checkpoint, nil
}

// Restore reverts the session to a previous checkpoint.
func (s *sessionImpl) Restore(ctx context.Context, checkpointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoint, err := s.backend.LoadCheckpoint(ctx, checkpointID)
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}

	if checkpoint.SessionID != s.meta.ID {
		return fmt.Errorf("checkpoint belongs to different session")
	}

	// Load all entries
	entries, err := s.backend.LoadEntries(ctx, s.meta.ID)
	if err != nil {
		return fmt.Errorf("load entries: %w", err)
	}

	// Find the entry at the checkpoint
	var restoreIdx int
	found := false
	for i, entry := range entries {
		if entry.ID == checkpoint.EntryID {
			restoreIdx = i + 1
			found = true
			break
		}
	}

	if !found && checkpoint.EntryID != "" {
		return fmt.Errorf("checkpoint entry not found")
	}

	// Truncate entries to checkpoint point
	s.entries = entries[:restoreIdx]
	s.meta.CurrentLeaf = checkpoint.EntryID
	s.meta.MessageCount = len(s.entries)
	s.meta.UpdatedAt = time.Now().UTC()
	s.dirty = true

	return nil
}

// Close releases any resources held by the session.
func (s *sessionImpl) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dirty {
		if err := s.backend.SaveSession(ctx, s.meta); err != nil {
			return fmt.Errorf("save session on close: %w", err)
		}
		s.dirty = false
	}

	return nil
}

// computeChecksum calculates a hash of all entry IDs.
func (s *sessionImpl) computeChecksum() string {
	h := sha256.New()
	for _, entry := range s.entries {
		h.Write([]byte(entry.ID))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// messageToData converts an agent.Message to a map for storage.
func messageToData(msg *agent.Message) map[string]any {
	if msg == nil {
		return make(map[string]any)
	}

	data := map[string]any{
		"id":        msg.ID,
		"type":      msg.Type,
		"payload":   msg.Payload,
		"timestamp": msg.Timestamp,
	}

	if msg.Metadata != nil {
		data["metadata"] = msg.Metadata
	}

	return data
}

// dataToMessage converts stored data back to an agent.Message.
func dataToMessage(data map[string]any) *agent.Message {
	if data == nil {
		return nil
	}

	msg := &agent.Message{
		Metadata: make(map[string]interface{}),
	}

	if id, ok := data["id"].(string); ok {
		msg.ID = id
	}
	if typ, ok := data["type"].(string); ok {
		msg.Type = typ
	}
	if payload, ok := data["payload"].(string); ok {
		msg.Payload = payload
	}
	if timestamp, ok := data["timestamp"].(string); ok {
		msg.Timestamp = timestamp
	}
	if metadata, ok := data["metadata"].(map[string]any); ok {
		for k, v := range metadata {
			msg.Metadata[k] = v
		}
	}

	return msg
}
