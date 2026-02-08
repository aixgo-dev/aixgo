package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *RedisBackend) {
	t.Helper()

	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	backend := NewRedisBackendFromClient(client, "test:", 0)

	t.Cleanup(func() {
		_ = backend.Close()
	})

	return mr, backend
}

func TestRedisBackend_SaveAndLoadSession(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	meta := &SessionMetadata{
		ID:           "sess-123",
		AgentName:    "test-agent",
		UserID:       "user-456",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		MessageCount: 0,
	}

	// Save session
	err := backend.SaveSession(ctx, meta)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Load session
	loaded, err := backend.LoadSession(ctx, "sess-123")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.ID != meta.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, meta.ID)
	}
	if loaded.AgentName != meta.AgentName {
		t.Errorf("AgentName mismatch: got %s, want %s", loaded.AgentName, meta.AgentName)
	}
	if loaded.UserID != meta.UserID {
		t.Errorf("UserID mismatch: got %s, want %s", loaded.UserID, meta.UserID)
	}
}

func TestRedisBackend_LoadSession_NotFound(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	_, err := backend.LoadSession(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestRedisBackend_DeleteSession(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	meta := &SessionMetadata{
		ID:        "sess-to-delete",
		AgentName: "test-agent",
		UserID:    "user-123",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Save and verify exists
	if err := backend.SaveSession(ctx, meta); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Delete
	if err := backend.DeleteSession(ctx, "sess-to-delete"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify deleted
	_, err := backend.LoadSession(ctx, "sess-to-delete")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestRedisBackend_ListSessions(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		meta := &SessionMetadata{
			ID:        "sess-" + string(rune('a'+i)),
			AgentName: "test-agent",
			UserID:    "user-1",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := backend.SaveSession(ctx, meta); err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
	}

	// List all
	sessions, err := backend.ListSessions(ctx, "test-agent", ListOptions{})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 5 {
		t.Errorf("expected 5 sessions, got %d", len(sessions))
	}

	// List with limit
	sessions, err = backend.ListSessions(ctx, "test-agent", ListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("ListSessions with limit failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestRedisBackend_ListSessions_ByUser(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	// Create sessions for different users
	for i := 0; i < 3; i++ {
		meta := &SessionMetadata{
			ID:        "sess-user1-" + string(rune('a'+i)),
			AgentName: "test-agent",
			UserID:    "user-1",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := backend.SaveSession(ctx, meta); err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		meta := &SessionMetadata{
			ID:        "sess-user2-" + string(rune('a'+i)),
			AgentName: "test-agent",
			UserID:    "user-2",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := backend.SaveSession(ctx, meta); err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
	}

	// List for user-1
	sessions, err := backend.ListSessions(ctx, "test-agent", ListOptions{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions for user-1, got %d", len(sessions))
	}

	// List for user-2
	sessions, err = backend.ListSessions(ctx, "test-agent", ListOptions{UserID: "user-2"})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for user-2, got %d", len(sessions))
	}
}

func TestRedisBackend_AppendAndLoadEntries(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	sessionID := "sess-entries-test"

	// Append entries
	for i := 0; i < 5; i++ {
		entry := &SessionEntry{
			ID:        "entry-" + string(rune('a'+i)),
			Timestamp: time.Now().UTC(),
			Type:      EntryTypeMessage,
			Data: map[string]any{
				"content": "message " + string(rune('a'+i)),
			},
		}
		if err := backend.AppendEntry(ctx, sessionID, entry); err != nil {
			t.Fatalf("AppendEntry failed: %v", err)
		}
	}

	// Load entries
	entries, err := backend.LoadEntries(ctx, sessionID)
	if err != nil {
		t.Fatalf("LoadEntries failed: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}

	// Verify order
	for i, entry := range entries {
		expectedID := "entry-" + string(rune('a'+i))
		if entry.ID != expectedID {
			t.Errorf("entry %d: expected ID %s, got %s", i, expectedID, entry.ID)
		}
	}
}

func TestRedisBackend_SaveAndLoadCheckpoint(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	checkpoint := &Checkpoint{
		ID:        "cp-123",
		SessionID: "sess-456",
		Timestamp: time.Now().UTC(),
		EntryID:   "entry-789",
		Checksum:  "abc123",
		Metadata: map[string]any{
			"reason": "test checkpoint",
		},
	}

	// Save checkpoint
	if err := backend.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Load checkpoint
	loaded, err := backend.LoadCheckpoint(ctx, "cp-123")
	if err != nil {
		t.Fatalf("LoadCheckpoint failed: %v", err)
	}

	if loaded.ID != checkpoint.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, checkpoint.ID)
	}
	if loaded.SessionID != checkpoint.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", loaded.SessionID, checkpoint.SessionID)
	}
	if loaded.EntryID != checkpoint.EntryID {
		t.Errorf("EntryID mismatch: got %s, want %s", loaded.EntryID, checkpoint.EntryID)
	}
}

func TestRedisBackend_LoadCheckpoint_NotFound(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	_, err := backend.LoadCheckpoint(ctx, "nonexistent")
	if err != ErrCheckpointNotFound {
		t.Errorf("expected ErrCheckpointNotFound, got %v", err)
	}
}

func TestRedisBackend_Close(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	// Close should succeed
	if err := backend.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail
	_, err := backend.LoadSession(ctx, "test")
	if err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed after close, got %v", err)
	}
}

func TestRedisBackend_Ping(t *testing.T) {
	_, backend := setupMiniredis(t)
	ctx := context.Background()

	if err := backend.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestRedisBackend_TTL(t *testing.T) {
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create backend with 1 hour TTL
	backend := NewRedisBackendFromClient(client, "test:", 1*time.Hour)
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	meta := &SessionMetadata{
		ID:        "sess-ttl-test",
		AgentName: "test-agent",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := backend.SaveSession(ctx, meta); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Fast-forward time in miniredis
	mr.FastForward(2 * time.Hour)

	// Session should be expired
	_, err := backend.LoadSession(ctx, "sess-ttl-test")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after TTL expiry, got %v", err)
	}
}
