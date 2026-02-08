package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aixgo-dev/aixgo"
	"github.com/aixgo-dev/aixgo/pkg/observability"
)

var (
	// Version information (set via ldflags)
	Version = "dev"

	// Command line flags
	configFile = flag.String("config", getEnv("CONFIG_FILE", "config/agents.yaml"), "Agent configuration file")
	httpPort   = flag.Int("http-port", getEnvInt("PORT", 8080), "HTTP server port")
	_          = flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Log level")
)

func main() {
	flag.Parse()

	log.Printf("Starting Aixgo Orchestrator v%s", Version)
	log.Printf("Config: %s, HTTP Port: %d", *configFile, *httpPort)

	// Initialize observability
	observability.InitMetrics()
	healthChecker := observability.InitHealthChecker()

	// Register health checks
	healthChecker.RegisterCheck(observability.PingCheck())

	// Start observability server
	obsServer := observability.NewServer(*httpPort)
	errChan := make(chan error, 2)
	go func() {
		log.Printf("Starting HTTP server on :%d", *httpPort)
		if err := obsServer.Start(); err != nil {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Start agent runtime in a goroutine
	go func() {
		if err := aixgo.Run(*configFile); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Printf("Error: %v", err)
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
