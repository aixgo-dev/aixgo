package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/aixgo-dev/aixgo/pkg/llm/provider"
	"github.com/spf13/cobra"
)

var (
	modelsRefresh bool
)

// modelsCmd represents the models command for listing available models.
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available LLM models",
	Long: `List all available LLM models with their providers and pricing information.

Models are fetched dynamically from each provider's API based on your configured API keys:
  - ANTHROPIC_API_KEY for Claude models
  - OPENAI_API_KEY for GPT models
  - GOOGLE_API_KEY for Gemini models
  - XAI_API_KEY for Grok models

Example:
  aixgo models
  aixgo models --refresh  # Force refresh from APIs`,
	RunE: runModels,
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.Flags().BoolVar(&modelsRefresh, "refresh", false, "Force refresh models from provider APIs")
}

func runModels(_ *cobra.Command, _ []string) error {
	// Clear cache if refresh requested
	if modelsRefresh {
		provider.DefaultModelAggregator.ClearCache()
	}

	// Check if any API keys are configured
	availableProviders := provider.GetAvailableProviderNames()
	if len(availableProviders) == 0 {
		fmt.Println("\nNo API keys configured. Set at least one of the following:")
		fmt.Println("  ANTHROPIC_API_KEY  - For Claude models")
		fmt.Println("  OPENAI_API_KEY     - For GPT models")
		fmt.Println("  GOOGLE_API_KEY     - For Gemini models")
		fmt.Println("  XAI_API_KEY        - For Grok models")
		return nil
	}

	fmt.Println("\nFetching available models...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := provider.ListAllModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	// Filter to chat models only
	models = provider.FilterChatModels(models)

	if len(models) == 0 {
		fmt.Println("No models found. Check your API key configuration.")
		return nil
	}

	fmt.Println("\nAvailable Models:")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-28s  %-10s  %-30s  %-10s  %s\n", "Model", "Provider", "Description", "Input/1M", "Output/1M")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")

	for _, m := range models {
		desc := m.Description
		if desc == "" {
			desc = "-"
		}

		inputCost := "-"
		outputCost := "-"
		if m.InputCost > 0 {
			inputCost = fmt.Sprintf("$%.2f", m.InputCost)
		}
		if m.OutputCost > 0 {
			outputCost = fmt.Sprintf("$%.2f", m.OutputCost)
		}

		fmt.Printf("%-28s  %-10s  %-30s  %-10s  %s\n",
			truncateStr(m.ID, 28),
			m.Provider,
			truncateStr(desc, 30),
			inputCost,
			outputCost,
		)
	}

	fmt.Println()
	fmt.Printf("Total: %d models from %d providers\n", len(models), len(availableProviders))
	fmt.Println()
	fmt.Println("Configured providers:", formatProviders(availableProviders))
	fmt.Println()
	fmt.Println("Start a chat with: aixgo chat --model <model-name>")
	fmt.Println("Force refresh:      aixgo models --refresh")

	return nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatProviders(providers []string) string {
	if len(providers) == 0 {
		return "none"
	}
	result := ""
	for i, p := range providers {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
