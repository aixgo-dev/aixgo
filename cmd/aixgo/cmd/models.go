package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// modelsCmd represents the models command for listing available models.
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available LLM models",
	Long: `List all available LLM models with their providers and pricing information.

Models are auto-detected based on environment variables:
  - ANTHROPIC_API_KEY for Claude models
  - OPENAI_API_KEY for GPT models
  - GOOGLE_API_KEY for Gemini models
  - XAI_API_KEY for Grok models

Example:
  aixgo models`,
	RunE: runModels,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}

type modelInfo struct {
	Name        string
	Provider    string
	Description string
	InputCost   float64 // per 1M tokens
	OutputCost  float64 // per 1M tokens
	Available   bool
}

func runModels(_ *cobra.Command, _ []string) error {
	models := []modelInfo{
		// Anthropic
		{"claude-opus-4", "Anthropic", "Most capable, best for complex tasks", 15.00, 75.00, hasKey("ANTHROPIC_API_KEY")},
		{"claude-3-5-sonnet", "Anthropic", "Balanced speed and capability (recommended)", 3.00, 15.00, hasKey("ANTHROPIC_API_KEY")},
		{"claude-3-5-haiku", "Anthropic", "Fastest, best for simple tasks", 0.25, 1.25, hasKey("ANTHROPIC_API_KEY")},

		// OpenAI
		{"gpt-4o", "OpenAI", "Latest GPT-4 with vision", 2.50, 10.00, hasKey("OPENAI_API_KEY")},
		{"gpt-4-turbo", "OpenAI", "GPT-4 optimized for speed", 10.00, 30.00, hasKey("OPENAI_API_KEY")},
		{"gpt-4o-mini", "OpenAI", "Smaller, faster GPT-4o", 0.15, 0.60, hasKey("OPENAI_API_KEY")},
		{"o1", "OpenAI", "Reasoning model for complex problems", 15.00, 60.00, hasKey("OPENAI_API_KEY")},
		{"o1-mini", "OpenAI", "Faster reasoning model", 3.00, 12.00, hasKey("OPENAI_API_KEY")},

		// Google
		{"gemini-2.0-flash", "Google", "Latest Gemini, fast and capable", 0.075, 0.30, hasKey("GOOGLE_API_KEY")},
		{"gemini-1.5-pro", "Google", "Gemini Pro with 1M context", 1.25, 5.00, hasKey("GOOGLE_API_KEY")},
		{"gemini-1.5-flash", "Google", "Fast Gemini model", 0.075, 0.30, hasKey("GOOGLE_API_KEY")},

		// xAI
		{"grok-2", "xAI", "Grok 2 for coding and reasoning", 2.00, 10.00, hasKey("XAI_API_KEY")},
		{"grok-2-mini", "xAI", "Faster Grok 2 variant", 0.30, 1.50, hasKey("XAI_API_KEY")},
	}

	fmt.Println("\nAvailable Models:")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-20s  %-10s  %-35s  %-12s  %s\n", "Model", "Provider", "Description", "Input/1M", "Output/1M")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")

	var availableCount int
	for _, m := range models {
		status := "✘"
		if m.Available {
			status = "✔"
			availableCount++
		}

		fmt.Printf("%s %-18s  %-10s  %-35s  $%-11.2f  $%.2f\n",
			status,
			m.Name,
			m.Provider,
			truncateDesc(m.Description, 35),
			m.InputCost,
			m.OutputCost,
		)
	}

	fmt.Println()
	fmt.Printf("Available: %d/%d models (set API keys to enable more)\n", availableCount, len(models))
	fmt.Println()
	fmt.Println("Environment variables needed:")
	fmt.Println("  ANTHROPIC_API_KEY  - For Claude models")
	fmt.Println("  OPENAI_API_KEY     - For GPT models")
	fmt.Println("  GOOGLE_API_KEY     - For Gemini models")
	fmt.Println("  XAI_API_KEY        - For Grok models")
	fmt.Println()
	fmt.Println("Start a chat with: aixgo chat --model <model-name>")

	return nil
}

func hasKey(envVar string) bool {
	return os.Getenv(envVar) != ""
}

func truncateDesc(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
