package cmd

import (
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/assistant/session"
	"github.com/spf13/cobra"
)

// sessionCmd represents the session management command.
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage chat sessions",
	Long: `Manage chat sessions including listing, resuming, and deleting sessions.

Sessions are stored locally in ~/.aixgo/sessions/ as JSON files.

Examples:
  aixgo session list
  aixgo session resume abc123
  aixgo session delete abc123`,
}

// sessionListCmd lists all sessions.
var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	Long: `List all saved chat sessions with their IDs, models, and costs.

Example:
  aixgo session list`,
	RunE: runSessionList,
}

// sessionResumeCmd resumes an existing session.
var sessionResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Resume an existing session",
	Long: `Resume a previously saved chat session by its ID.

Example:
  aixgo session resume abc123`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSessionIDs,
	RunE:              runSessionResume,
}

// sessionDeleteCmd deletes a session.
var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a saved session",
	Long: `Delete a saved chat session by its ID.

Example:
  aixgo session delete abc123`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSessionIDs,
	RunE:              runSessionDelete,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionResumeCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
}

func runSessionList(_ *cobra.Command, _ []string) error {
	mgr, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := mgr.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions found.")
		fmt.Println("Start a new session with: aixgo chat")
		return nil
	}

	fmt.Println("\nSaved Sessions:")
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Printf("%-12s  %-20s  %-8s  %-10s  %s\n", "ID", "Model", "Messages", "Cost", "Last Updated")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	for _, s := range sessions {
		lastUpdated := s.UpdatedAt.Format(time.RFC3339)
		if time.Since(s.UpdatedAt) < 24*time.Hour {
			lastUpdated = s.UpdatedAt.Format("15:04")
		} else if time.Since(s.UpdatedAt) < 7*24*time.Hour {
			lastUpdated = s.UpdatedAt.Format("Mon 15:04")
		}

		fmt.Printf("%-12s  %-20s  %-8d  $%-9.4f  %s\n",
			s.ID[:12],
			truncate(s.Model, 20),
			len(s.Messages),
			s.TotalCost,
			lastUpdated,
		)
	}

	fmt.Println()
	fmt.Println("Resume a session with: aixgo chat --session <id>")
	fmt.Println("Or: aixgo session resume <id>")

	return nil
}

func runSessionResume(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// Set the session flag and run chat
	chatSessionID = sessionID
	return runChat(cmd, nil)
}

func runSessionDelete(_ *cobra.Command, args []string) error {
	sessionID := args[0]

	mgr, err := session.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	if err := mgr.Delete(sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Printf("Session %s deleted.\n", sessionID)
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
