// Package session provides session persistence for Aixgo agents.
// Sessions enable agents to maintain conversation history, checkpoint state,
// and resume from previous interactions.
package session

import (
	"time"
)

// EntryType defines the type of session entry.
type EntryType string

const (
	// EntryTypeMessage represents a conversation message.
	EntryTypeMessage EntryType = "message"
	// EntryTypeCheckpoint represents a state checkpoint.
	EntryTypeCheckpoint EntryType = "checkpoint"
	// EntryTypeMetadata represents session metadata updates.
	EntryTypeMetadata EntryType = "metadata"
)

// SessionEntry represents a single entry in the session log.
// Entries are append-only and immutable once written.
type SessionEntry struct {
	// ID is the unique identifier for this entry.
	ID string `json:"id"`
	// ParentID links to the previous entry (for future branching support).
	ParentID string `json:"parentId,omitempty"`
	// Timestamp is when the entry was created.
	Timestamp time.Time `json:"timestamp"`
	// Type indicates what kind of entry this is.
	Type EntryType `json:"type"`
	// Data contains the entry payload.
	Data map[string]any `json:"data"`
}

// SessionMetadata holds session summary information.
// This is stored separately for quick listing without loading all entries.
type SessionMetadata struct {
	// ID is the unique session identifier.
	ID string `json:"id"`
	// AgentName is the name of the agent this session belongs to.
	AgentName string `json:"agentName"`
	// UserID identifies the user (optional).
	UserID string `json:"userId,omitempty"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the session was last modified.
	UpdatedAt time.Time `json:"updatedAt"`
	// MessageCount is the number of messages in the session.
	MessageCount int `json:"messageCount"`
	// CurrentLeaf is the ID of the current leaf entry (for future branching).
	CurrentLeaf string `json:"currentLeaf,omitempty"`
}

// Checkpoint represents a restorable state snapshot.
// Checkpoints allow resuming a session from a specific point.
type Checkpoint struct {
	// ID is the unique checkpoint identifier.
	ID string `json:"id"`
	// SessionID links to the parent session.
	SessionID string `json:"sessionId"`
	// Timestamp is when the checkpoint was created.
	Timestamp time.Time `json:"timestamp"`
	// EntryID is the session entry this checkpoint was created at.
	EntryID string `json:"entryId"`
	// Checksum verifies integrity of entries up to this checkpoint.
	Checksum string `json:"checksum"`
	// Metadata contains optional checkpoint-specific data.
	Metadata map[string]any `json:"metadata,omitempty"`
}
