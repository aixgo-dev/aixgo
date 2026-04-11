package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// Test actual NewManager which creates ~/.aixgo/sessions/
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.sessionsDir == "" {
		t.Error("sessionsDir should not be empty")
	}
}

func TestManager(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "aixgo-session-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create manager with custom sessions dir
	mgr := &Manager{
		sessionsDir: tmpDir,
	}

	t.Run("Create", func(t *testing.T) {
		sess, err := mgr.Create("claude-3-5-sonnet")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if sess.ID == "" {
			t.Error("Session ID should not be empty")
		}
		if sess.Model != "claude-3-5-sonnet" {
			t.Errorf("Model = %v, want claude-3-5-sonnet", sess.Model)
		}
		if len(sess.Messages) != 0 {
			t.Errorf("Messages should be empty, got %d", len(sess.Messages))
		}
	})

	t.Run("Get", func(t *testing.T) {
		// Create a session first
		created, err := mgr.Create("gpt-4o")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Get by full ID
		sess, err := mgr.Get(created.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if sess.ID != created.ID {
			t.Errorf("ID = %v, want %v", sess.ID, created.ID)
		}

		// Get by partial ID
		sess, err = mgr.Get(created.ID[:6])
		if err != nil {
			t.Fatalf("Get by prefix failed: %v", err)
		}
		if sess.ID != created.ID {
			t.Errorf("ID = %v, want %v", sess.ID, created.ID)
		}
	})

	t.Run("Save", func(t *testing.T) {
		sess, err := mgr.Create("gemini-1.5-pro")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Add a message
		sess.AddMessage(Message{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now(),
		})

		// Save
		if err := mgr.Save(sess); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Verify file exists
		path := filepath.Join(tmpDir, sess.ID+".json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Session file not created at %s", path)
		}

		// Reload and verify
		loaded, err := mgr.Get(sess.ID)
		if err != nil {
			t.Fatalf("Get after save failed: %v", err)
		}
		if len(loaded.Messages) != 1 {
			t.Errorf("Messages = %d, want 1", len(loaded.Messages))
		}
		if loaded.Messages[0].Content != "Hello" {
			t.Errorf("Content = %v, want Hello", loaded.Messages[0].Content)
		}
	})

	t.Run("List", func(t *testing.T) {
		// Clear existing sessions
		entries, _ := os.ReadDir(tmpDir)
		for _, e := range entries {
			_ = os.Remove(filepath.Join(tmpDir, e.Name()))
		}

		// Create multiple sessions
		for i := 0; i < 3; i++ {
			_, err := mgr.Create("test-model")
			if err != nil {
				t.Fatalf("Create failed: %v", err)
			}
		}

		// List
		sessions, err := mgr.List()
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(sessions) != 3 {
			t.Errorf("List returned %d sessions, want 3", len(sessions))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		sess, err := mgr.Create("test-model")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Delete
		if err := mgr.Delete(sess.ID); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify not found
		_, err = mgr.Get(sess.ID)
		if err == nil {
			t.Error("Get should fail after delete")
		}
	})
}

func TestSession_AddMessage(t *testing.T) {
	sess := &Session{
		ID:        "test",
		Model:     "test-model",
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	initialUpdate := sess.UpdatedAt

	time.Sleep(time.Millisecond)

	sess.AddMessage(Message{
		Role:      "user",
		Content:   "Test message",
		Timestamp: time.Now(),
	})

	if len(sess.Messages) != 1 {
		t.Errorf("Messages = %d, want 1", len(sess.Messages))
	}

	if !sess.UpdatedAt.After(initialUpdate) {
		t.Error("UpdatedAt should be updated after AddMessage")
	}
}

func TestSession_LastMessage(t *testing.T) {
	sess := &Session{
		Messages: []Message{
			{Role: "user", Content: "First"},
			{Role: "assistant", Content: "Second"},
		},
	}

	last := sess.LastMessage()
	if last == nil {
		t.Fatal("LastMessage returned nil")
	}
	if last.Content != "Second" {
		t.Errorf("Content = %v, want Second", last.Content)
	}

	// Empty session
	empty := &Session{}
	if empty.LastMessage() != nil {
		t.Error("LastMessage should return nil for empty session")
	}
}

func TestSession_UserMessages(t *testing.T) {
	sess := &Session{
		Messages: []Message{
			{Role: "user", Content: "Q1"},
			{Role: "assistant", Content: "A1"},
			{Role: "user", Content: "Q2"},
			{Role: "assistant", Content: "A2"},
			{Role: "user", Content: "Q3"},
		},
	}

	userMsgs := sess.UserMessages()
	if len(userMsgs) != 3 {
		t.Errorf("len(UserMessages) = %d, want 3", len(userMsgs))
	}

	for _, msg := range userMsgs {
		if msg.Role != "user" {
			t.Errorf("Expected user role, got %s", msg.Role)
		}
	}
}

func TestSession_AssistantMessages(t *testing.T) {
	sess := &Session{
		Messages: []Message{
			{Role: "user", Content: "Q1"},
			{Role: "assistant", Content: "A1"},
			{Role: "user", Content: "Q2"},
			{Role: "assistant", Content: "A2"},
		},
	}

	assistantMsgs := sess.AssistantMessages()
	if len(assistantMsgs) != 2 {
		t.Errorf("len(AssistantMessages) = %d, want 2", len(assistantMsgs))
	}

	for _, msg := range assistantMsgs {
		if msg.Role != "assistant" {
			t.Errorf("Expected assistant role, got %s", msg.Role)
		}
	}
}

func TestSession_ToSummary(t *testing.T) {
	now := time.Now()
	sess := &Session{
		ID:        "test123",
		Model:     "gpt-4o",
		Messages:  []Message{{}, {}, {}},
		TotalCost: 0.123,
		UpdatedAt: now,
	}

	summary := sess.ToSummary()

	if summary.ID != "test123" {
		t.Errorf("ID = %v, want test123", summary.ID)
	}
	if summary.Model != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", summary.Model)
	}
	if summary.Messages != 3 {
		t.Errorf("Messages = %d, want 3", summary.Messages)
	}
	if summary.TotalCost != 0.123 {
		t.Errorf("TotalCost = %v, want 0.123", summary.TotalCost)
	}
	if !summary.UpdatedAt.Equal(now) {
		t.Error("UpdatedAt should match")
	}
}

func TestManager_EmptyDirectory(t *testing.T) {
	// Test listing when directory is empty
	tmpDir, err := os.MkdirTemp("", "aixgo-session-empty")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mgr := &Manager{sessionsDir: tmpDir}

	sessions, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected empty list, got %d sessions", len(sessions))
	}
}

func TestManager_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aixgo-session-notfound")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mgr := &Manager{sessionsDir: tmpDir}

	_, err = mgr.Get("nonexistent123")
	if err == nil {
		t.Error("Expected error for nonexistent session")
	}
}

func TestManager_DeleteNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aixgo-session-deletenotfound")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mgr := &Manager{sessionsDir: tmpDir}

	err = mgr.Delete("nonexistent123")
	if err == nil {
		t.Error("Expected error for deleting nonexistent session")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("generateID should not return empty string")
	}
	if len(id1) != 12 {
		t.Errorf("generateID should return 12-char ID, got %d", len(id1))
	}
	if !sessionIDPattern.MatchString(id1) {
		t.Errorf("generateID = %q, want match for %s", id1, sessionIDPattern)
	}
	if id1 == id2 {
		t.Error("generateID should return unique IDs")
	}
}

func TestManager_Validation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "aixgo-session-validation")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	mgr := &Manager{sessionsDir: tmpDir}

	t.Run("Get rejects empty id", func(t *testing.T) {
		if _, err := mgr.Get(""); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Get(\"\") error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Get rejects short id", func(t *testing.T) {
		if _, err := mgr.Get("abc"); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Get(\"abc\") error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Delete rejects empty id", func(t *testing.T) {
		if err := mgr.Delete(""); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Delete(\"\") error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Delete rejects short id", func(t *testing.T) {
		if err := mgr.Delete("xyz"); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Delete(\"xyz\") error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Save rejects malformed id", func(t *testing.T) {
		bad := &Session{ID: "not-hex!"}
		if err := mgr.Save(bad); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Save(bad) error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Save rejects nil session", func(t *testing.T) {
		if err := mgr.Save(nil); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Save(nil) error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Save rejects path traversal id", func(t *testing.T) {
		bad := &Session{ID: "../../etc/pa"}
		if err := mgr.Save(bad); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Save(traversal) error = %v, want ErrInvalidSessionID", err)
		}
	})
	t.Run("Save accepts legacy 11-char hex id", func(t *testing.T) {
		// Pre-v0.5.x sessions used an 11-character hex ID (the old generateID
		// stripped a single UUID dash from a 12-char slice). Resuming such a
		// session must remain possible: Save MUST accept it so the chat loop's
		// auto-save does not error out on every turn.
		legacy := &Session{ID: "54e95d35cae", Model: "gpt-4o", Messages: []Message{}}
		if err := mgr.Save(legacy); err != nil {
			t.Errorf("Save(legacy 11-char id) error = %v, want nil", err)
		}
	})
	t.Run("Save accepts canonical 12-char hex id", func(t *testing.T) {
		ok := &Session{ID: "abcdef012345", Model: "gpt-4o", Messages: []Message{}}
		if err := mgr.Save(ok); err != nil {
			t.Errorf("Save(12-char id) error = %v, want nil", err)
		}
	})
	t.Run("Save rejects 10-char id", func(t *testing.T) {
		bad := &Session{ID: "abcdef0123"}
		if err := mgr.Save(bad); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("Save(10-char id) error = %v, want ErrInvalidSessionID", err)
		}
	})
}
