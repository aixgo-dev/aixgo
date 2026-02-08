package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo/agent"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManagerCreate(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	tests := []struct {
		name      string
		agentName string
		opts      CreateOptions
	}{
		{
			name:      "basic session",
			agentName: "test-agent",
			opts:      CreateOptions{},
		},
		{
			name:      "session with user",
			agentName: "test-agent",
			opts:      CreateOptions{UserID: "user-123"},
		},
		{
			name:      "session with metadata",
			agentName: "another-agent",
			opts: CreateOptions{
				UserID:   "user-456",
				Metadata: map[string]any{"key": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, err := mgr.Create(ctx, tt.agentName, tt.opts)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			if sess.ID() == "" {
				t.Error("session ID should not be empty")
			}
			if sess.AgentName() != tt.agentName {
				t.Errorf("AgentName() = %v, want %v", sess.AgentName(), tt.agentName)
			}
			if sess.UserID() != tt.opts.UserID {
				t.Errorf("UserID() = %v, want %v", sess.UserID(), tt.opts.UserID)
			}
		})
	}
}

func TestManagerGet(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	// Create a session
	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{UserID: "user-123"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get the session
	retrieved, err := mgr.Get(ctx, sess.ID())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.ID() != sess.ID() {
		t.Errorf("Get() ID = %v, want %v", retrieved.ID(), sess.ID())
	}

	// Get non-existent session
	_, err = mgr.Get(ctx, "non-existent-id")
	if err != ErrSessionNotFound {
		t.Errorf("Get() error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestManagerGetOrCreate(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	// First call should create
	sess1, err := mgr.GetOrCreate(ctx, "test-agent", "user-123")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	// Second call should return the same session
	sess2, err := mgr.GetOrCreate(ctx, "test-agent", "user-123")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if sess1.ID() != sess2.ID() {
		t.Errorf("GetOrCreate() should return same session, got %v and %v", sess1.ID(), sess2.ID())
	}

	// Different user should create new session
	sess3, err := mgr.GetOrCreate(ctx, "test-agent", "user-456")
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if sess1.ID() == sess3.ID() {
		t.Error("GetOrCreate() should create different session for different user")
	}
}

func TestManagerList(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	// Create multiple sessions
	_, err = mgr.Create(ctx, "agent-1", CreateOptions{UserID: "user-a"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = mgr.Create(ctx, "agent-1", CreateOptions{UserID: "user-b"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = mgr.Create(ctx, "agent-2", CreateOptions{UserID: "user-a"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// List all sessions for agent-1
	sessions, err := mgr.List(ctx, "agent-1", ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("List() returned %d sessions, want 2", len(sessions))
	}

	// List sessions for agent-1 with user filter
	sessions, err = mgr.List(ctx, "agent-1", ListOptions{UserID: "user-a"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List() returned %d sessions, want 1", len(sessions))
	}

	// List with limit
	sessions, err = mgr.List(ctx, "agent-1", ListOptions{Limit: 1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("List() returned %d sessions, want 1", len(sessions))
	}
}

func TestManagerDelete(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	// Create a session
	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete the session
	err = mgr.Delete(ctx, sess.ID())
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Get should fail
	_, err = mgr.Get(ctx, sess.ID())
	if err != ErrSessionNotFound {
		t.Errorf("Get() after Delete() error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestSessionAppendMessage(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Append messages
	msg1 := agent.NewMessage("user", map[string]string{"content": "Hello"})
	if err := sess.AppendMessage(ctx, msg1); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	msg2 := agent.NewMessage("assistant", map[string]string{"content": "Hi there!"})
	if err := sess.AppendMessage(ctx, msg2); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	// Get messages
	messages, err := sess.GetMessages(ctx)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("GetMessages() returned %d messages, want 2", len(messages))
	}
}

func TestSessionCheckpointRestore(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add some messages
	msg1 := agent.NewMessage("user", "Hello")
	if err := sess.AppendMessage(ctx, msg1); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	// Create checkpoint
	checkpoint, err := sess.Checkpoint(ctx)
	if err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}

	if checkpoint.ID == "" {
		t.Error("Checkpoint ID should not be empty")
	}

	// Add more messages
	msg2 := agent.NewMessage("assistant", "Hi!")
	if err := sess.AppendMessage(ctx, msg2); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	messages, err := sess.GetMessages(ctx)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("GetMessages() returned %d messages, want 2", len(messages))
	}

	// Restore to checkpoint
	if err := sess.Restore(ctx, checkpoint.ID); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Should only have 1 message now
	messages, err = sess.GetMessages(ctx)
	if err != nil {
		t.Fatalf("GetMessages() after Restore() error = %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("GetMessages() after Restore() returned %d messages, want 1", len(messages))
	}
}

func TestSessionPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and populate a session
	func() {
		backend, err := NewFileBackend(tmpDir)
		if err != nil {
			t.Fatalf("NewFileBackend() error = %v", err)
		}
		defer func() { _ = backend.Close() }()

		mgr := NewManager(backend)
		ctx := context.Background()

		sess, err := mgr.Create(ctx, "test-agent", CreateOptions{UserID: "user-123"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		msg := agent.NewMessage("user", "Hello")
		if err := sess.AppendMessage(ctx, msg); err != nil {
			t.Fatalf("AppendMessage() error = %v", err)
		}

		t.Logf("Created session %s", sess.ID())
	}()

	// Reopen and verify
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	// List sessions to find the one we created
	sessions, err := mgr.List(ctx, "test-agent", ListOptions{UserID: "user-123"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("List() returned %d sessions, want 1", len(sessions))
	}

	sess, err := mgr.Get(ctx, sessions[0].ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	messages, err := sess.GetMessages(ctx)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("GetMessages() returned %d messages, want 1", len(messages))
	}
}

func TestFileBackendBasics(t *testing.T) {
	tmpDir := t.TempDir()

	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test SaveSession
	meta := &SessionMetadata{
		ID:           "test-session-1",
		AgentName:    "test-agent",
		UserID:       "user-1",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		MessageCount: 0,
	}

	if err := backend.SaveSession(ctx, meta); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	// Test LoadSession
	loaded, err := backend.LoadSession(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}

	if loaded.ID != meta.ID {
		t.Errorf("LoadSession() ID = %v, want %v", loaded.ID, meta.ID)
	}

	// Test AppendEntry
	entry := &SessionEntry{
		ID:        "entry-1",
		Timestamp: time.Now().UTC(),
		Type:      EntryTypeMessage,
		Data:      map[string]any{"content": "hello"},
	}

	if err := backend.AppendEntry(ctx, "test-session-1", entry); err != nil {
		t.Fatalf("AppendEntry() error = %v", err)
	}

	// Test LoadEntries
	entries, err := backend.LoadEntries(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("LoadEntries() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("LoadEntries() returned %d entries, want 1", len(entries))
	}

	// Test SaveCheckpoint
	checkpoint := &Checkpoint{
		ID:        "checkpoint-1",
		SessionID: "test-session-1",
		Timestamp: time.Now().UTC(),
		EntryID:   "entry-1",
		Checksum:  "abc123",
	}

	if err := backend.SaveCheckpoint(ctx, checkpoint); err != nil {
		t.Fatalf("SaveCheckpoint() error = %v", err)
	}

	// Test LoadCheckpoint
	loadedCP, err := backend.LoadCheckpoint(ctx, "checkpoint-1")
	if err != nil {
		t.Fatalf("LoadCheckpoint() error = %v", err)
	}

	if loadedCP.ID != checkpoint.ID {
		t.Errorf("LoadCheckpoint() ID = %v, want %v", loadedCP.ID, checkpoint.ID)
	}

	// Test DeleteSession
	if err := backend.DeleteSession(ctx, "test-session-1"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	_, err = backend.LoadSession(ctx, "test-session-1")
	if err != ErrSessionNotFound {
		t.Errorf("LoadSession() after delete error = %v, want %v", err, ErrSessionNotFound)
	}
}

func TestFileBackendDefaultDir(t *testing.T) {
	// Test that empty baseDir uses default
	backend, err := NewFileBackend("")
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	home, _ := os.UserHomeDir()
	expectedDir := filepath.Join(home, ".aixgo", "sessions")

	if backend.baseDir != expectedDir {
		t.Errorf("baseDir = %v, want %v", backend.baseDir, expectedDir)
	}
}

func TestContextHelpers(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test ContextWithSession and SessionFromContext
	ctxWithSession := ContextWithSession(ctx, sess)

	retrieved, ok := SessionFromContext(ctxWithSession)
	if !ok {
		t.Fatal("SessionFromContext() returned false")
	}

	if retrieved.ID() != sess.ID() {
		t.Errorf("SessionFromContext() ID = %v, want %v", retrieved.ID(), sess.ID())
	}

	// Test with context without session
	_, ok = SessionFromContext(ctx)
	if ok {
		t.Error("SessionFromContext() should return false for context without session")
	}

	// Test MustSessionFromContext panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustSessionFromContext() should panic for context without session")
		}
	}()
	_ = MustSessionFromContext(ctx)
}

func TestSessionClose(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	msg := agent.NewMessage("user", "Hello")
	if err := sess.AppendMessage(ctx, msg); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	// Close should save dirty state
	if err := sess.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestManagerClose(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	mgr := NewManager(backend)
	ctx := context.Background()

	_, err = mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Close manager
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFileBackendClosed(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}

	ctx := context.Background()

	// Create a session first
	meta := &SessionMetadata{
		ID:           "test-session",
		AgentName:    "test-agent",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := backend.SaveSession(ctx, meta); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	// Close the backend
	if err := backend.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// All operations should return ErrStorageClosed
	if _, err := backend.LoadSession(ctx, "test-session"); err != ErrStorageClosed {
		t.Errorf("LoadSession() after close error = %v, want %v", err, ErrStorageClosed)
	}

	if err := backend.SaveSession(ctx, meta); err != ErrStorageClosed {
		t.Errorf("SaveSession() after close error = %v, want %v", err, ErrStorageClosed)
	}

	if _, err := backend.ListSessions(ctx, "test-agent", ListOptions{}); err != ErrStorageClosed {
		t.Errorf("ListSessions() after close error = %v, want %v", err, ErrStorageClosed)
	}

	if err := backend.DeleteSession(ctx, "test-session"); err != ErrStorageClosed {
		t.Errorf("DeleteSession() after close error = %v, want %v", err, ErrStorageClosed)
	}

	entry := &SessionEntry{ID: "e1", Type: EntryTypeMessage}
	if err := backend.AppendEntry(ctx, "test-session", entry); err != ErrStorageClosed {
		t.Errorf("AppendEntry() after close error = %v, want %v", err, ErrStorageClosed)
	}

	if _, err := backend.LoadEntries(ctx, "test-session"); err != ErrStorageClosed {
		t.Errorf("LoadEntries() after close error = %v, want %v", err, ErrStorageClosed)
	}

	cp := &Checkpoint{ID: "cp1", SessionID: "test-session"}
	if err := backend.SaveCheckpoint(ctx, cp); err != ErrStorageClosed {
		t.Errorf("SaveCheckpoint() after close error = %v, want %v", err, ErrStorageClosed)
	}

	if _, err := backend.LoadCheckpoint(ctx, "cp1"); err != ErrStorageClosed {
		t.Errorf("LoadCheckpoint() after close error = %v, want %v", err, ErrStorageClosed)
	}
}

func TestListOptionsOffset(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		_, err := mgr.Create(ctx, "test-agent", CreateOptions{})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Test offset
	sessions, err := mgr.List(ctx, "test-agent", ListOptions{Offset: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("List() with offset returned %d sessions, want 3", len(sessions))
	}

	// Test offset beyond length
	sessions, err = mgr.List(ctx, "test-agent", ListOptions{Offset: 10})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("List() with large offset returned %d sessions, want 0", len(sessions))
	}
}

func TestSessionRestoreErrors(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	mgr := NewManager(backend)
	ctx := context.Background()

	sess, err := mgr.Create(ctx, "test-agent", CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Try to restore with non-existent checkpoint
	err = sess.Restore(ctx, "non-existent")
	if err == nil {
		t.Error("Restore() should fail with non-existent checkpoint")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("DefaultConfig() Enabled should be true")
	}
	if cfg.Store != "file" {
		t.Errorf("DefaultConfig() Store = %v, want file", cfg.Store)
	}
	if cfg.BaseDir != "" {
		t.Errorf("DefaultConfig() BaseDir = %v, want empty", cfg.BaseDir)
	}
}

func TestMessageToDataNil(t *testing.T) {
	data := messageToData(nil)
	if len(data) != 0 {
		t.Errorf("messageToData(nil) should return empty map, got %v", data)
	}
}

func TestDataToMessageNil(t *testing.T) {
	msg := dataToMessage(nil)
	if msg != nil {
		t.Errorf("dataToMessage(nil) should return nil, got %v", msg)
	}
}

func TestDataToMessageWithMetadata(t *testing.T) {
	data := map[string]any{
		"id":        "msg-1",
		"type":      "user",
		"payload":   "hello",
		"timestamp": "2024-01-01T00:00:00Z",
		"metadata":  map[string]any{"key": "value"},
	}

	msg := dataToMessage(data)
	if msg == nil {
		t.Fatal("dataToMessage() returned nil")
	}

	if msg.ID != "msg-1" {
		t.Errorf("ID = %v, want msg-1", msg.ID)
	}
	if msg.Type != "user" {
		t.Errorf("Type = %v, want user", msg.Type)
	}
	if msg.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want value", msg.Metadata["key"])
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFileBackend() error = %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()

	// Test path traversal in agent name
	traversalCases := []struct {
		name      string
		agentName string
		sessionID string
	}{
		{"slash in agent name", "../etc", "valid-session"},
		{"backslash in agent name", "..\\etc", "valid-session"},
		{"dotdot in agent name", "foo..bar", "valid-session"},
		{"slash in session ID", "valid-agent", "../../../etc/passwd"},
		{"empty agent name", "", "valid-session"},
		{"empty session ID", "valid-agent", ""},
	}

	for _, tc := range traversalCases {
		t.Run(tc.name, func(t *testing.T) {
			meta := &SessionMetadata{
				ID:        tc.sessionID,
				AgentName: tc.agentName,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			err := backend.SaveSession(ctx, meta)
			if err == nil {
				t.Errorf("SaveSession() should reject path traversal attempt: agent=%q session=%q", tc.agentName, tc.sessionID)
			}
		})
	}

	// Test path traversal in checkpoint ID
	t.Run("slash in checkpoint ID", func(t *testing.T) {
		// First create a valid session
		validMeta := &SessionMetadata{
			ID:        "valid-session",
			AgentName: "valid-agent",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := backend.SaveSession(ctx, validMeta); err != nil {
			t.Fatalf("SaveSession() error = %v", err)
		}

		checkpoint := &Checkpoint{
			ID:        "../../../etc/passwd",
			SessionID: "valid-session",
			Timestamp: time.Now().UTC(),
		}

		err := backend.SaveCheckpoint(ctx, checkpoint)
		if err == nil {
			t.Error("SaveCheckpoint() should reject path traversal in checkpoint ID")
		}
	})
}
