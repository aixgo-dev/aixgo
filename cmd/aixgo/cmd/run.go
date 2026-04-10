package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/pkg/observability"
	"github.com/spf13/cobra"
)

var (
	configFile string
	httpPort   int
	logLevel   string
)

// runCmd represents the run command for orchestrating agents.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the agent orchestrator",
	Long: `Run the Aixgo agent orchestrator with the specified configuration file.

This starts the agent runtime and HTTP observability server, executing
the defined agent workflows.

Example:
  aixgo run -config config/agents.yaml
  aixgo run -config workflow.yaml --http-port 9090`,
	RunE: runOrchestrator,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&configFile, "config", "c", getEnv("CONFIG_FILE", "config/agents.yaml"), "Agent configuration file")
	runCmd.Flags().IntVar(&httpPort, "http-port", getEnvInt("PORT", 8080), "HTTP server port for observability")
	runCmd.Flags().StringVar(&logLevel, "log-level", getEnv("LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")

	_ = runCmd.RegisterFlagCompletionFunc("config", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"yaml", "yml"}, cobra.ShellCompDirectiveFilterFileExt
	})
	_ = runCmd.RegisterFlagCompletionFunc("log-level", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func runOrchestrator(_ *cobra.Command, _ []string) error {
	log.Printf("Starting Aixgo Orchestrator v%s", Version)
	log.Printf("Config: %s, HTTP Port: %d", configFile, httpPort)

	// Initialize observability
	observability.InitMetrics()
	healthChecker := observability.InitHealthChecker()

	// Register health checks
	healthChecker.RegisterCheck(observability.PingCheck())

	// Start observability server
	obsServer := observability.NewServer(httpPort)
	errChan := make(chan error, 2)
	go func() {
		log.Printf("Starting HTTP server on :%d", httpPort)
		if err := obsServer.Start(); err != nil {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Start agent runtime in a goroutine
	go func() {
		if err := aixgo.Run(configFile); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Printf("Error: %v", err)
		return err
	case <-quit:
		log.Println("Shutting down orchestrator...")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := obsServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Orchestrator stopped")
	return nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}
