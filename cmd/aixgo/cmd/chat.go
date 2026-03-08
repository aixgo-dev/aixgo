package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/assistant/coordinator"
	"github.com/aixgo-dev/aixgo/pkg/assistant/output"
	"github.com/aixgo-dev/aixgo/pkg/assistant/prompt"
	"github.com/aixgo-dev/aixgo/pkg/assistant/session"
	"github.com/spf13/cobra"
)

var (
	chatModel     string
	chatSessionID string
	chatNoStream  bool
)

// chatCmd represents the chat command for interactive coding assistant.
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive coding assistant session",
	Long: `Start an interactive chat session with an AI coding assistant.

The chat command provides a multi-model interactive coding assistant that can:
  - Answer coding questions
  - Read, write, and modify files
  - Execute git operations
  - Run terminal commands (with confirmation)
  - Track costs per session

Models are fetched dynamically. Run 'aixgo models' to see all available models.

Examples:
  aixgo chat
  aixgo chat --model claude-sonnet-4-6
  aixgo chat --model gpt-4o
  aixgo chat --session abc123  # Resume a session

In-session commands:
  /model <name>  - Switch to a different model
  /cost          - Show session cost summary
  /save          - Save the current session
  /clear         - Clear conversation history
  /help          - Show available commands
  /quit          - Exit the chat`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVarP(&chatModel, "model", "m", getEnv("AIXGO_MODEL", "claude-sonnet-4-6"), "Model to use for chat")
	chatCmd.Flags().StringVarP(&chatSessionID, "session", "s", "", "Resume an existing session by ID")
	chatCmd.Flags().BoolVar(&chatNoStream, "no-stream", false, "Disable streaming output")
}

func runChat(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nGoodbye!")
		cancel()
	}()

	// Initialize session manager
	sessionMgr, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	// Initialize or resume session
	var sess *session.Session
	if chatSessionID != "" {
		sess, err = sessionMgr.Get(chatSessionID)
		if err != nil {
			return fmt.Errorf("failed to resume session %s: %w", chatSessionID, err)
		}
		fmt.Printf("Resumed session: %s\n", sess.ID)
	} else {
		// Prompt for model selection if not specified via flag
		if chatModel == "" || chatModel == "claude-sonnet-4-6" {
			selectedModel, err := prompt.SelectModel()
			if err != nil {
				return fmt.Errorf("model selection failed: %w", err)
			}
			chatModel = selectedModel
		}

		sess, err = sessionMgr.Create(chatModel)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		fmt.Printf("New session: %s (model: %s)\n", sess.ID, chatModel)
	}

	// Initialize coordinator
	coord, err := coordinator.New(coordinator.Config{
		Model:     chatModel,
		Streaming: !chatNoStream,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize coordinator: %w", err)
	}

	// Initialize output renderer
	renderer := output.NewRenderer(output.Config{
		Streaming: !chatNoStream,
	})

	// Print welcome message
	printWelcome(chatModel)

	// Main chat loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")

		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle in-session commands
		if strings.HasPrefix(input, "/") {
			handled, err := handleCommand(ctx, input, sess, sessionMgr, coord)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if handled {
				if input == "/quit" || input == "/exit" {
					return nil
				}
				continue
			}
		}

		// Add user message to session
		sess.AddMessage(session.Message{
			Role:      "user",
			Content:   input,
			Timestamp: time.Now(),
		})

		// Get response from coordinator
		response, err := coord.Chat(ctx, sess.Messages)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Render the response
		if err := renderer.Render(response.Content); err != nil {
			fmt.Printf("Error rendering response: %v\n", err)
		}

		// Add assistant message to session
		sess.AddMessage(session.Message{
			Role:      "assistant",
			Content:   response.Content,
			Timestamp: time.Now(),
			Model:     chatModel,
			Cost:      response.Cost,
		})

		// Update session cost
		sess.TotalCost += response.Cost

		// Auto-save session
		if err := sessionMgr.Save(sess); err != nil {
			fmt.Printf("Warning: failed to save session: %v\n", err)
		}

		// Show cost if significant
		if response.Cost > 0.001 {
			fmt.Printf("\n[Cost: $%.4f | Session total: $%.4f]\n", response.Cost, sess.TotalCost)
		}
	}

	return nil
}

func handleCommand(_ context.Context, input string, sess *session.Session, mgr *session.Manager, coord *coordinator.Coordinator) (bool, error) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false, nil
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "/quit", "/exit":
		// Save before exiting
		if err := mgr.Save(sess); err != nil {
			fmt.Printf("Warning: failed to save session: %v\n", err)
		}
		fmt.Println("Session saved. Goodbye!")
		return true, nil

	case "/model":
		if len(parts) < 2 {
			fmt.Println("Usage: /model <model-name>")
			fmt.Println("Available models: claude-3-5-sonnet, gpt-4o, gemini-1.5-pro, grok-2")
			return true, nil
		}
		newModel := parts[1]
		if err := coord.SetModel(newModel); err != nil {
			return true, fmt.Errorf("failed to switch model: %w", err)
		}
		chatModel = newModel
		sess.Model = newModel
		fmt.Printf("Switched to model: %s\n", newModel)
		return true, nil

	case "/cost":
		fmt.Printf("\nSession Cost Summary:\n")
		fmt.Printf("  Total cost: $%.4f\n", sess.TotalCost)
		fmt.Printf("  Messages: %d\n", len(sess.Messages))
		fmt.Printf("  Model: %s\n", sess.Model)
		return true, nil

	case "/save":
		if err := mgr.Save(sess); err != nil {
			return true, fmt.Errorf("failed to save session: %w", err)
		}
		fmt.Printf("Session saved: %s\n", sess.ID)
		return true, nil

	case "/clear":
		confirmed, err := prompt.Confirm("Clear conversation history?")
		if err != nil {
			return true, err
		}
		if confirmed {
			sess.Messages = []session.Message{}
			coord.ClearHistory()
			fmt.Println("Conversation history cleared.")
		}
		return true, nil

	case "/help":
		printHelp()
		return true, nil

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Type /help for available commands.")
		return true, nil
	}
}

func printWelcome(model string) {
	fmt.Println()
	fmt.Println("╭──────────────────────────────────────────────────╮")
	fmt.Println("│           Aixgo Interactive Assistant            │")
	fmt.Println("╰──────────────────────────────────────────────────╯")
	fmt.Printf("  Model: %s\n", model)
	fmt.Println("  Type /help for commands, /quit to exit")
	fmt.Println()
}

func printHelp() {
	fmt.Print(`
Available commands:
  /model <name>  - Switch to a different model
  /cost          - Show session cost summary
  /save          - Save the current session
  /clear         - Clear conversation history
  /help          - Show this help message
  /quit          - Exit the chat

Models are fetched dynamically. Run 'aixgo models' to see all available models.

Tips:
  - Ask coding questions naturally
  - Request file operations: "Read main.go"
  - Run commands: "Run go test ./..."
  - Git operations: "Show git status"
`)
}
