//go:build integration

package session_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/agent"
	"github.com/aixgo-dev/aixgo/pkg/session"
)

// TestRuntimeSessionIntegration tests the full integration between
// Runtime and Session management.
func TestRuntimeSessionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Setup temporary storage
	tmpDir := filepath.Join(os.TempDir(), "aixgo-integration-test")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create session infrastructure
	backend, err := session.NewFileBackend(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	mgr := session.NewManager(backend)
	defer mgr.Close()

	// Create runtime with session support
	rt := aixgo.NewRuntime()
	if err := rt.Start(ctx); err != nil {
		t.Fatalf("Failed to start runtime: %v", err)
	}
	defer rt.Stop(ctx)

	rt.SetSessionManager(mgr)

	// Verify session manager is accessible
	if rt.SessionManager() == nil {
		t.Fatal("SessionManager() returned nil")
	}

	// Create a session
	sess, err := mgr.Create(ctx, "test-agent", session.CreateOptions{
		UserID: "integration-user",
	})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test session operations
	t.Run("AppendAndGetMessages", func(t *testing.T) {
		msg := agent.NewMessage("user", map[string]string{"content": "Hello"})
		if err := sess.AppendMessage(ctx, msg); err != nil {
			t.Fatalf("AppendMessage failed: %v", err)
		}

		messages, err := sess.GetMessages(ctx)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}
		if len(messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(messages))
		}
	})

	t.Run("CheckpointAndRestore", func(t *testing.T) {
		// Add more messages
		for i := 0; i < 3; i++ {
			msg := agent.NewMessage("user", map[string]string{"content": "test"})
			if err := sess.AppendMessage(ctx, msg); err != nil {
				t.Fatalf("AppendMessage failed: %v", err)
			}
		}

		// Create checkpoint
		checkpoint, err := sess.Checkpoint(ctx)
		if err != nil {
			t.Fatalf("Checkpoint failed: %v", err)
		}

		// Get count before adding more
		msgsBefore, _ := sess.GetMessages(ctx)
		countBefore := len(msgsBefore)

		// Add more messages
		msg := agent.NewMessage("user", map[string]string{"content": "after checkpoint"})
		if err := sess.AppendMessage(ctx, msg); err != nil {
			t.Fatalf("AppendMessage failed: %v", err)
		}

		// Restore
		if err := sess.Restore(ctx, checkpoint.ID); err != nil {
			t.Fatalf("Restore failed: %v", err)
		}

		// Verify count is back to before
		msgsAfter, _ := sess.GetMessages(ctx)
		if len(msgsAfter) != countBefore {
			t.Errorf("Expected %d messages after restore, got %d", countBefore, len(msgsAfter))
		}
	})

	t.Run("SessionResume", func(t *testing.T) {
		sessionID := sess.ID()
		sess.Close(ctx)

		// Resume session
		resumed, err := mgr.Get(ctx, sessionID)
		if err != nil {
			t.Fatalf("Failed to resume session: %v", err)
		}

		if resumed.ID() != sessionID {
			t.Errorf("Resumed session ID mismatch: got %s, want %s", resumed.ID(), sessionID)
		}

		// Messages should be preserved
		messages, err := resumed.GetMessages(ctx)
		if err != nil {
			t.Fatalf("GetMessages on resumed session failed: %v", err)
		}
		if len(messages) == 0 {
			t.Error("Expected messages to be preserved after resume")
		}
	})
}

// TestSessionContextHelpers tests context integration.
func TestSessionContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Setup
	tmpDir := filepath.Join(os.TempDir(), "aixgo-context-test")
	os.MkdirAll(tmpDir, 0700)
	defer os.RemoveAll(tmpDir)

	backend, _ := session.NewFileBackend(tmpDir)
	defer backend.Close()

	mgr := session.NewManager(backend)
	defer mgr.Close()

	sess, _ := mgr.Create(ctx, "test", session.CreateOptions{})

	// Test ContextWithSession and SessionFromContext
	ctxWithSession := session.ContextWithSession(ctx, sess)
	retrieved, ok := session.SessionFromContext(ctxWithSession)

	if !ok {
		t.Fatal("SessionFromContext returned false")
	}
	if retrieved.ID() != sess.ID() {
		t.Errorf("Session ID mismatch: got %s, want %s", retrieved.ID(), sess.ID())
	}

	// Test context without session
	_, ok = session.SessionFromContext(ctx)
	if ok {
		t.Error("Expected false for context without session")
	}
}

// TestConcurrentSessionAccess tests thread safety.
func TestConcurrentSessionAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Setup
	tmpDir := filepath.Join(os.TempDir(), "aixgo-concurrent-test")
	os.MkdirAll(tmpDir, 0700)
	defer os.RemoveAll(tmpDir)

	backend, _ := session.NewFileBackend(tmpDir)
	defer backend.Close()

	mgr := session.NewManager(backend)
	defer mgr.Close()

	sess, _ := mgr.Create(ctx, "concurrent-test", session.CreateOptions{})

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				msg := agent.NewMessage("user", map[string]string{
					"content": "concurrent message",
				})
				sess.AppendMessage(ctx, msg)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent writes")
		}
	}

	// Verify all messages were written
	messages, err := sess.GetMessages(ctx)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(messages) != 100 {
		t.Errorf("Expected 100 messages, got %d", len(messages))
	}
}

// TestGetOrCreatePattern tests the common pattern.
func TestGetOrCreatePattern(t *testing.T) {
	ctx := context.Background()

	// Setup
	tmpDir := filepath.Join(os.TempDir(), "aixgo-getorcreate-test")
	os.MkdirAll(tmpDir, 0700)
	defer os.RemoveAll(tmpDir)

	backend, _ := session.NewFileBackend(tmpDir)
	defer backend.Close()

	mgr := session.NewManager(backend)
	defer mgr.Close()

	// First call creates
	sess1, err := mgr.GetOrCreate(ctx, "agent", "user-1")
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	// Second call returns same session
	sess2, err := mgr.GetOrCreate(ctx, "agent", "user-1")
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	if sess1.ID() != sess2.ID() {
		t.Errorf("GetOrCreate should return same session: %s vs %s", sess1.ID(), sess2.ID())
	}

	// Different user gets different session
	sess3, err := mgr.GetOrCreate(ctx, "agent", "user-2")
	if err != nil {
		t.Fatalf("GetOrCreate failed: %v", err)
	}

	if sess3.ID() == sess1.ID() {
		t.Error("Different users should get different sessions")
	}
}
