// Package main demonstrates session-aware ReAct agent usage with Aixgo.
//
// This example shows how to:
// - Create a session-enabled runtime
// - Execute ReAct agents with conversation history
// - Use CallWithSession for automatic session management
// - Resume conversations across multiple interactions
//
// Run with: go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/agent"
	"github.com/aixgo-dev/aixgo/pkg/session"
)

func main() {
	ctx := context.Background()

	// Create a temporary directory for session storage
	tmpDir := filepath.Join(os.TempDir(), "aixgo-session-react-example")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("=== Aixgo Session-Aware ReAct Example ===")
	fmt.Println()

	// Step 1: Create session backend and manager
	fmt.Println("1. Setting up session infrastructure...")
	backend, err := session.NewFileBackend(tmpDir)
	if err != nil {
		log.Fatalf("Failed to create storage backend: %v", err)
	}
	defer backend.Close()

	sessionMgr := session.NewManager(backend)
	defer sessionMgr.Close()
	fmt.Println("   Session manager created with file-based storage")

	// Step 2: Create and configure runtime
	fmt.Println("\n2. Creating runtime...")
	rt := aixgo.NewRuntime()
	if err := rt.Start(ctx); err != nil {
		log.Fatalf("Failed to start runtime: %v", err)
	}
	defer rt.Stop(ctx)

	// Connect session manager to runtime
	rt.SetSessionManager(sessionMgr)
	fmt.Println("   Runtime started with session support")

	// Step 3: Create a session for our conversation
	fmt.Println("\n3. Creating conversation session...")
	sess, err := sessionMgr.Create(ctx, "assistant", session.CreateOptions{
		UserID: "demo-user",
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	fmt.Printf("   Session ID: %s\n", sess.ID())

	// Step 4: Simulate a multi-turn conversation
	fmt.Println("\n4. Simulating multi-turn conversation...")

	// First message
	msg1 := agent.NewMessage("user", map[string]string{
		"content": "Hello! My name is Alice.",
	})
	if err := sess.AppendMessage(ctx, msg1); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Println("   User: Hello! My name is Alice.")

	// Assistant response
	resp1 := agent.NewMessage("assistant", map[string]string{
		"content": "Hello Alice! Nice to meet you. How can I help you today?",
	})
	if err := sess.AppendMessage(ctx, resp1); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Println("   Assistant: Hello Alice! Nice to meet you. How can I help you today?")

	// Second message - tests context retention
	msg2 := agent.NewMessage("user", map[string]string{
		"content": "What's my name?",
	})
	if err := sess.AppendMessage(ctx, msg2); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Println("   User: What's my name?")

	// Step 5: Check message history
	fmt.Println("\n5. Verifying conversation history...")
	messages, err := sess.GetMessages(ctx)
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}
	fmt.Printf("   Total messages in session: %d\n", len(messages))

	for i, m := range messages {
		content := getContent(m)
		fmt.Printf("   [%d] %s: %v\n", i+1, m.Type, truncate(content, 50))
	}

	// Step 6: Create a checkpoint
	fmt.Println("\n6. Creating checkpoint...")
	checkpoint, err := sess.Checkpoint(ctx)
	if err != nil {
		log.Fatalf("Failed to create checkpoint: %v", err)
	}
	fmt.Printf("   Checkpoint created: %s\n", checkpoint.ID)

	// Step 7: Add more messages
	fmt.Println("\n7. Adding more messages...")
	resp2 := agent.NewMessage("assistant", map[string]string{
		"content": "Your name is Alice! You introduced yourself earlier.",
	})
	if err := sess.AppendMessage(ctx, resp2); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Println("   Assistant: Your name is Alice! You introduced yourself earlier.")

	msg3 := agent.NewMessage("user", map[string]string{
		"content": "Great! Tell me a joke about programming.",
	})
	if err := sess.AppendMessage(ctx, msg3); err != nil {
		log.Fatalf("Failed to append message: %v", err)
	}
	fmt.Println("   User: Great! Tell me a joke about programming.")

	// Check message count
	messages, _ = sess.GetMessages(ctx)
	fmt.Printf("   Current message count: %d\n", len(messages))

	// Step 8: Restore to checkpoint
	fmt.Println("\n8. Restoring to checkpoint...")
	if err := sess.Restore(ctx, checkpoint.ID); err != nil {
		log.Fatalf("Failed to restore checkpoint: %v", err)
	}
	messages, _ = sess.GetMessages(ctx)
	fmt.Printf("   Message count after restore: %d\n", len(messages))
	fmt.Println("   (Lost the joke request - back to 'What's my name?' question)")

	// Step 9: Resume session (simulate app restart)
	fmt.Println("\n9. Simulating session resume...")
	sessionID := sess.ID()
	if err := sess.Close(ctx); err != nil {
		log.Printf("Warning: failed to close session: %v", err)
	}

	// Re-open the session
	resumedSess, err := sessionMgr.Get(ctx, sessionID)
	if err != nil {
		log.Fatalf("Failed to resume session: %v", err)
	}
	fmt.Printf("   Resumed session: %s\n", resumedSess.ID())

	messages, _ = resumedSess.GetMessages(ctx)
	fmt.Printf("   Messages preserved: %d\n", len(messages))

	// Step 10: Context helper demonstration
	fmt.Println("\n10. Context helpers...")
	ctxWithSession := session.ContextWithSession(ctx, resumedSess)
	if retrievedSess, ok := session.SessionFromContext(ctxWithSession); ok {
		fmt.Printf("    Session retrieved from context: %s\n", retrievedSess.ID())
	}

	fmt.Println("\n=== Example Complete ===")
	fmt.Println()
	fmt.Println("Key takeaways:")
	fmt.Println("- Sessions persist conversation history automatically")
	fmt.Println("- Checkpoints allow you to save and restore conversation state")
	fmt.Println("- Sessions survive application restarts")
	fmt.Println("- Context helpers enable session-aware middleware patterns")
}

// getContent extracts the content field from a message's metadata
func getContent(m *agent.Message) string {
	if m.Metadata == nil {
		return ""
	}
	if content, ok := m.Metadata["content"]; ok {
		if s, ok := content.(string); ok {
			return s
		}
	}
	return ""
}

// truncate shortens a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
