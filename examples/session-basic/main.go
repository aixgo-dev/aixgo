// Package main demonstrates basic session usage with Aixgo agents.
//
// This example shows how to:
// - Create a session manager with file-based storage
// - Create and resume sessions
// - Append messages and retrieve history
// - Create and restore checkpoints
//
// Run with: go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aixgo-dev/aixgo/agent"
	"github.com/aixgo-dev/aixgo/pkg/session"
)

func main() {
	ctx := context.Background()

	// Create a temporary directory for this example
	tmpDir := filepath.Join(os.TempDir(), "aixgo-session-example")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir) // Cleanup after example

	// Create file-based storage backend
	backend, err := session.NewFileBackend(tmpDir)
	if err != nil {
		log.Fatalf("Failed to create storage backend: %v", err)
	}
	defer backend.Close()

	// Create session manager
	mgr := session.NewManager(backend)
	defer mgr.Close()

	fmt.Println("=== Aixgo Sessions Example ===")
	fmt.Println()

	// Example 1: Create a new session
	fmt.Println("1. Creating a new session...")
	sess, err := mgr.Create(ctx, "assistant", session.CreateOptions{
		UserID: "user-123",
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	fmt.Printf("   Session created: %s\n", sess.ID())
	fmt.Printf("   Agent: %s, User: %s\n\n", sess.AgentName(), sess.UserID())

	// Example 2: Append messages to session
	fmt.Println("2. Appending messages...")
	messages := []struct {
		msgType string
		content string
	}{
		{"user", "Hello! What's your name?"},
		{"assistant", "I'm your AI assistant. How can I help you today?"},
		{"user", "What's the weather like?"},
		{"assistant", "I don't have access to real-time weather data."},
	}

	for _, m := range messages {
		msg := agent.NewMessage(m.msgType, map[string]string{"content": m.content})
		if err := sess.AppendMessage(ctx, msg); err != nil {
			log.Fatalf("Failed to append message: %v", err)
		}
		fmt.Printf("   Added: [%s] %s\n", m.msgType, m.content)
	}
	fmt.Println()

	// Example 3: Create a checkpoint
	fmt.Println("3. Creating checkpoint...")
	checkpoint, err := sess.Checkpoint(ctx)
	if err != nil {
		log.Fatalf("Failed to create checkpoint: %v", err)
	}
	fmt.Printf("   Checkpoint ID: %s\n", checkpoint.ID)
	fmt.Printf("   Entry ID: %s\n\n", checkpoint.EntryID)

	// Example 4: Add more messages after checkpoint
	fmt.Println("4. Adding messages after checkpoint...")
	msg := agent.NewMessage("user", map[string]string{"content": "Tell me a joke"})
	if err := sess.AppendMessage(ctx, msg); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Printf("   Added: [user] Tell me a joke\n")

	msg = agent.NewMessage("assistant", map[string]string{"content": "Why did the gopher go to therapy? It had too many go routines!"})
	if err := sess.AppendMessage(ctx, msg); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Printf("   Added: [assistant] Why did the gopher go to therapy?...\n\n")

	// Example 5: Get all messages
	fmt.Println("5. Current message count...")
	allMessages, err := sess.GetMessages(ctx)
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}
	fmt.Printf("   Total messages: %d\n\n", len(allMessages))

	// Example 6: Restore to checkpoint
	fmt.Println("6. Restoring to checkpoint...")
	if err := sess.Restore(ctx, checkpoint.ID); err != nil {
		log.Fatalf("Failed to restore checkpoint: %v", err)
	}
	fmt.Printf("   Restored to checkpoint %s\n", checkpoint.ID)

	allMessages, err = sess.GetMessages(ctx)
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}
	fmt.Printf("   Message count after restore: %d\n\n", len(allMessages))

	// Example 7: Resume session (simulating application restart)
	fmt.Println("7. Simulating session resume...")
	sessionID := sess.ID()
	sess.Close(ctx) // Close current session

	// Get session by ID (as if resuming after restart)
	resumedSess, err := mgr.Get(ctx, sessionID)
	if err != nil {
		log.Fatalf("Failed to resume session: %v", err)
	}
	fmt.Printf("   Resumed session: %s\n", resumedSess.ID())

	resumedMessages, err := resumedSess.GetMessages(ctx)
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}
	fmt.Printf("   Messages preserved: %d\n\n", len(resumedMessages))

	// Example 8: List sessions
	fmt.Println("8. Listing sessions...")
	sessions, err := mgr.List(ctx, "assistant", session.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}
	fmt.Printf("   Found %d session(s) for 'assistant'\n\n", len(sessions))

	// Example 9: GetOrCreate pattern
	fmt.Println("9. GetOrCreate pattern...")
	existingSess, err := mgr.GetOrCreate(ctx, "assistant", "user-123")
	if err != nil {
		log.Fatalf("Failed to get or create session: %v", err)
	}
	if existingSess.ID() == resumedSess.ID() {
		fmt.Printf("   Found existing session for user-123\n")
	} else {
		fmt.Printf("   Created new session for user-123\n")
	}

	newUserSess, err := mgr.GetOrCreate(ctx, "assistant", "user-456")
	if err != nil {
		log.Fatalf("Failed to get or create session: %v", err)
	}
	fmt.Printf("   Created new session for user-456: %s\n\n", newUserSess.ID())

	// Example 10: Context helpers
	fmt.Println("10. Context helpers...")
	ctxWithSession := session.ContextWithSession(ctx, resumedSess)
	retrievedSess, ok := session.SessionFromContext(ctxWithSession)
	if ok {
		fmt.Printf("    Session retrieved from context: %s\n", retrievedSess.ID())
	}

	fmt.Println("\n=== Example Complete ===")
}
