// Package cmd provides the CLI command structure for aixgo.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set via ldflags at build time.
	Version = "dev"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "aixgo",
	Short: "Aixgo - Production-grade AI agent framework for Go",
	Long: `Aixgo is a production-grade AI agent framework for Go that enables
secure, scalable multi-agent systems without Python dependencies.

Features:
  - 13 orchestration patterns (Supervisor, Sequential, Parallel, etc.)
  - 6 agent types (ReAct, Classifier, Aggregator, Planner, etc.)
  - 7+ LLM providers (OpenAI, Anthropic, Gemini, xAI, etc.)
  - MCP support for tool calling
  - Cost tracking and observability

Use "aixgo [command] --help" for more information about a command.`,
	Version: Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

// SetVersion sets the version string for the CLI.
func SetVersion(v string) {
	Version = v
	rootCmd.Version = v
}
