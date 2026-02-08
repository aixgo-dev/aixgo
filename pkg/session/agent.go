package session

import (
	"context"

	"github.com/aixgo-dev/aixgo/agent"
)

// SessionAwareAgent is an optional interface that agents can implement
// to support session-aware execution. Agents that don't implement this
// interface will still work with sessions via the runtime's CallWithSession,
// but won't have access to session history during execution.
type SessionAwareAgent interface {
	// ExecuteWithSession performs execution with access to session context.
	// The session provides access to conversation history and memory.
	ExecuteWithSession(ctx context.Context, input *agent.Message, sess Session) (*agent.Message, error)
}

// CheckpointableAgent is an optional interface for agents that support
// creating and restoring from checkpoints with custom state.
type CheckpointableAgent interface {
	// CreateCheckpoint captures the agent's internal state for later restoration.
	// Returns the state as a map that will be stored in the checkpoint.
	CreateCheckpoint(ctx context.Context) (map[string]any, error)

	// RestoreFromCheckpoint restores the agent's internal state from a checkpoint.
	RestoreFromCheckpoint(ctx context.Context, state map[string]any) error
}

// MemoryAwareAgent is an optional interface for agents that want direct
// access to long-term memory during execution.
type MemoryAwareAgent interface {
	// SetMemory provides the agent with access to long-term memory.
	// Called before execution when sessions are enabled.
	SetMemory(ctx context.Context, memory MemoryReader) error
}

// MemoryReader provides read access to agent memory.
// This is a subset of the full memory manager for safety.
type MemoryReader interface {
	// Read retrieves a value from memory by key.
	Read(ctx context.Context, key string) (string, error)

	// Search performs semantic search over memory.
	Search(ctx context.Context, query string, limit int) ([]MemoryEntry, error)

	// List returns all memory keys.
	List(ctx context.Context) ([]string, error)
}

// MemoryEntry represents a single memory item.
type MemoryEntry struct {
	Key       string
	Value     string
	Timestamp string
	Source    string
}

// IsSessionAware checks if an agent implements SessionAwareAgent.
func IsSessionAware(a any) bool {
	_, ok := a.(SessionAwareAgent)
	return ok
}

// IsCheckpointable checks if an agent implements CheckpointableAgent.
func IsCheckpointable(a any) bool {
	_, ok := a.(CheckpointableAgent)
	return ok
}

// IsMemoryAware checks if an agent implements MemoryAwareAgent.
func IsMemoryAware(a any) bool {
	_, ok := a.(MemoryAwareAgent)
	return ok
}
