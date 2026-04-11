package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/assistant/coordinator"
	"github.com/aixgo-dev/aixgo/pkg/assistant/output"
	"github.com/aixgo-dev/aixgo/pkg/assistant/prompt"
	"github.com/aixgo-dev/aixgo/pkg/assistant/session"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
)

// chatSlashCommands is the set of in-session commands used for tab-completion.
var chatSlashCommands = []string{"/model", "/cost", "/save", "/clear", "/help", "/quit", "/exit"}

// chatSecretPattern matches common secret/token shapes that must never be
// appended to the readline history file. Keep this conservative: prefer false
// negatives over false positives, but cover the well-known provider prefixes.
var chatSecretPattern = regexp.MustCompile(
	`(?i)` +
		`sk-[A-Za-z0-9_\-]{20,}` + // OpenAI / Anthropic style
		`|xai-[A-Za-z0-9_\-]{20,}` + // xAI
		`|ghp_[A-Za-z0-9]{20,}` + // GitHub personal access token
		`|ghs_[A-Za-z0-9]{20,}` + // GitHub server token
		`|AKIA[0-9A-Z]{16}` + // AWS access key id
		`|xoxb-[A-Za-z0-9\-]{10,}` + // Slack bot token
		`|xoxp-[A-Za-z0-9\-]{10,}` + // Slack user token
		`|Bearer\s+[A-Za-z0-9._\-]{20,}` + // generic bearer token
		`|eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+` + // JWT (header.payload)
		`|-----BEGIN [A-Z ]*PRIVATE KEY-----`, // PEM private key block
)

var (
	chatModel        string
	chatSessionID    string
	chatNoStream     bool
	chatPrompt       string
	chatStdin        bool
	chatOutput       string
	chatNoHistory    bool
	chatMaxTokens    int
	chatMaxOutputKiB int
)

// chatDefaultMaxOutputKiB is the default soft byte cap on non-interactive
// chat output (1 MiB). It bounds the worst-case blast radius for scripts
// that pipe the response into another tool without breaking typical use.
const chatDefaultMaxOutputKiB = 1024

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
  aixgo chat --session abc123                      # Resume a session
  aixgo chat -p "Explain this error"               # One-shot prompt
  git diff | aixgo chat -p "Review this diff"      # Pipe stdin
  aixgo chat -p "List providers" --output json     # Machine-readable output

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
	chatCmd.Flags().StringVarP(&chatPrompt, "prompt", "p", "", "Run a one-shot prompt and exit (non-interactive)")
	chatCmd.Flags().BoolVar(&chatStdin, "stdin", false, "Append piped stdin to the prompt (auto-enabled when stdin is not a TTY)")
	chatCmd.Flags().StringVarP(&chatOutput, "output", "o", "text", "Output format for non-interactive mode: text, json")
	chatCmd.Flags().BoolVar(&chatNoHistory, "no-history", false, "Disable loading and saving readline history for this session")
	chatCmd.Flags().IntVar(&chatMaxTokens, "max-tokens", 0, "Maximum response tokens (0 = provider default)")
	chatCmd.Flags().IntVar(&chatMaxOutputKiB, "max-output-kib", chatDefaultMaxOutputKiB, "Soft cap on non-interactive output in KiB; oversized responses are truncated")

	_ = chatCmd.RegisterFlagCompletionFunc("model", completeModelNames)
	_ = chatCmd.RegisterFlagCompletionFunc("session", completeSessionIDs)
	_ = chatCmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func runChat(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	if chatOutput != "text" && chatOutput != "json" {
		return fmt.Errorf("invalid --output %q: must be 'text' or 'json'", chatOutput)
	}

	// Non-interactive one-shot mode: -p provided OR stdin is piped.
	stdinPiped := !isTerminal(os.Stdin)
	if chatPrompt != "" || chatStdin || (stdinPiped && chatPrompt == "") {
		return runChatOneShot(ctx, stdinPiped)
	}

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
		MaxTokens: chatMaxTokens,
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

	// Initialize readline with history, tab completion, and Ctrl+C handling.
	line := liner.NewLiner()
	line.SetCtrlCAborts(true)
	line.SetCompleter(func(prefix string) []string {
		if !strings.HasPrefix(prefix, "/") {
			return nil
		}
		var out []string
		for _, c := range chatSlashCommands {
			if strings.HasPrefix(c, prefix) {
				out = append(out, c)
			}
		}
		return out
	})

	historyPath := chatHistoryFilePath()
	if !chatNoHistory {
		// #nosec G304 -- historyPath is constructed from os.UserHomeDir() and a fixed
		// relative path (.aixgo/chat_history); not influenced by untrusted input.
		if f, err := os.Open(historyPath); err == nil {
			_, _ = line.ReadHistory(f)
			_ = f.Close()
		}
	}
	defer func() {
		if !chatNoHistory {
			persistChatHistory(line, historyPath)
		}
		_ = line.Close()
	}()

	// Main chat loop
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		raw, err := line.Prompt("\n> ")
		if err != nil {
			if errors.Is(err, liner.ErrPromptAborted) || errors.Is(err, io.EOF) {
				fmt.Println("\nGoodbye!")
				return nil
			}
			return fmt.Errorf("read input: %w", err)
		}

		input := strings.TrimSpace(raw)
		if input == "" {
			continue
		}
		appendChatHistorySafe(line, input)

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
}

// appendChatHistorySafe appends input to the liner history only when it does
// not look like a secret (API key, bearer token, JWT, PEM block, etc.). This
// is a best-effort defense against pasted credentials being written to the
// on-disk history file. For fully guaranteed suppression, users should run
// with --no-history.
func appendChatHistorySafe(line *liner.State, input string) {
	if line == nil || input == "" {
		return
	}
	if chatSecretPattern.MatchString(input) {
		return
	}
	line.AppendHistory(input)
}

// chatHistoryFilePath returns the path to the persisted readline history file.
func chatHistoryFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Clean(filepath.Join(home, ".aixgo", "chat_history"))
}

// persistChatHistory writes the current liner history to disk, capped at
// chatHistoryMaxLines entries. Failures are non-fatal.
func persistChatHistory(line *liner.State, path string) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	// History may contain sensitive prompt content (including pasted secrets),
	// so the file must be readable only by the owner. Use OpenFile with an
	// explicit 0o600 mode rather than os.Create (which uses 0o666 & ~umask).
	// #nosec G304 -- path is constructed from os.UserHomeDir() and a fixed
	// relative path (.aixgo/chat_history); not influenced by untrusted input.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	// Defensively re-apply 0o600 in case the file pre-existed with wider perms
	// (OpenFile honors existing modes when the file already exists).
	_ = os.Chmod(path, 0o600)
	_, _ = line.WriteHistory(f)
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

// isTerminal reports whether f is a character device (TTY).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runChatOneShot executes a single non-interactive chat turn and exits.
// The prompt is built from --prompt and/or piped stdin, sent to the
// coordinator as a single user message, and the response is printed to
// stdout in the requested format (text or json).
func runChatOneShot(ctx context.Context, stdinPiped bool) error {
	userInput := chatPrompt
	if chatStdin || (stdinPiped && userInput == "") {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		piped := strings.TrimSpace(string(data))
		if piped != "" {
			if userInput == "" {
				// Stdin is the entire prompt (no user-supplied -p); pass through
				// verbatim as direct user input.
				userInput = piped
			} else {
				// L2 defense-in-depth: when piped stdin is appended to a
				// user-supplied --prompt, wrap it in delimiters so downstream
				// models can distinguish operator instructions from potentially
				// untrusted external content (prompt-injection mitigation).
				// Neutralize any literal closing tag inside the piped content
				// so it cannot escape the wrapper and re-enter "trusted" scope.
				safe := strings.ReplaceAll(piped, "</untrusted_input>", "<\\/untrusted_input>")
				userInput = userInput + "\n\n<untrusted_input>\n" + safe + "\n</untrusted_input>"
			}
		}
	}
	if strings.TrimSpace(userInput) == "" {
		return fmt.Errorf("no prompt provided (use --prompt or pipe input via stdin)")
	}

	sessionMgr, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	appendedToExisting := false
	var sess *session.Session
	if chatSessionID != "" {
		sess, err = sessionMgr.Get(chatSessionID)
		if err != nil {
			return fmt.Errorf("failed to resume session %s: %w", chatSessionID, err)
		}
		appendedToExisting = true
		// M3: make the append-to-existing-session behavior explicit so users
		// are not silently mutating history in one-shot mode.
		fmt.Fprintf(os.Stderr, "appending to existing session %s\n", sess.ID)
	} else {
		sess, err = sessionMgr.Create(chatModel)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	coord, err := coordinator.New(coordinator.Config{
		Model:     chatModel,
		Streaming: false,
		MaxTokens: chatMaxTokens,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize coordinator: %w", err)
	}

	sess.AddMessage(session.Message{
		Role:      "user",
		Content:   userInput,
		Timestamp: time.Now(),
	})

	response, err := coord.Chat(ctx, sess.Messages)
	if err != nil {
		return fmt.Errorf("chat failed: %w", err)
	}

	sess.AddMessage(session.Message{
		Role:      "assistant",
		Content:   response.Content,
		Timestamp: time.Now(),
		Model:     chatModel,
		Cost:      response.Cost,
	})
	sess.TotalCost += response.Cost

	if saveErr := sessionMgr.Save(sess); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save session: %v\n", saveErr)
	}

	content, truncated := truncateForOutput(response.Content, chatMaxOutputKiB)

	switch chatOutput {
	case "json":
		out := map[string]any{
			"content":                      content,
			"cost":                         response.Cost,
			"model":                        chatModel,
			"session_id":                   sess.ID,
			"input_tokens":                 response.InputTokens,
			"output_tokens":                response.OutputTokens,
			"finish_reason":                response.FinishReason,
			"appended_to_existing_session": appendedToExisting,
			"truncated":                    truncated,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	default:
		fmt.Println(content)
		if truncated {
			fmt.Fprintf(os.Stderr, "warning: output truncated to %d KiB (use --max-output-kib to raise the cap)\n", chatMaxOutputKiB)
		}
		return nil
	}
}

// truncateForOutput applies a soft byte cap to content. If maxKiB is 0 or
// negative the content is returned unchanged. Truncation is byte-based on
// a rune boundary so the result is always valid UTF-8.
func truncateForOutput(content string, maxKiB int) (string, bool) {
	if maxKiB <= 0 {
		return content, false
	}
	limit := maxKiB * 1024
	if len(content) <= limit {
		return content, false
	}
	cut := limit
	for cut > 0 && (content[cut]&0xC0) == 0x80 {
		cut--
	}
	return content[:cut], true
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
