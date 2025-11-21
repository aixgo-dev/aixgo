package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Server provides HTTP endpoints for observability
type Server struct {
	httpServer *http.Server
	port       int
}

// NewServer creates a new observability server
func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

// Start starts the observability server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/health", HealthHandler())
	mux.HandleFunc("/health/live", LivenessHandler())
	mux.HandleFunc("/health/ready", ReadinessHandler())

	// Metrics endpoint
	mux.Handle("/metrics", MetricsHandler())

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
