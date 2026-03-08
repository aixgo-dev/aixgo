// Package daemon provides a Kubernetes-native daemon runtime with graceful lifecycle management.
// This package enables building long-running services with health checks, task management,
// and graceful shutdown handling.
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds daemon configuration.
type Config struct {
	// HealthPort is the port for health check endpoints (default: 9090)
	HealthPort int
	// ShutdownTimeout is the maximum time to wait for graceful shutdown
	ShutdownTimeout time.Duration
	// Logger is the structured logger to use
	Logger *slog.Logger
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		HealthPort:      9090,
		ShutdownTimeout: 30 * time.Second,
		Logger:          slog.Default(),
	}
}

// Daemon manages the lifecycle of a long-running service.
type Daemon struct {
	config       Config
	healthServer *http.Server
	ready        bool
	readyMu      sync.RWMutex
	tasks        []Task
	tasksMu      sync.Mutex
}

// Task represents a background task managed by the daemon.
type Task interface {
	// Name returns a human-readable name for logging
	Name() string
	// Start begins the task. It should block until ctx is cancelled.
	Start(ctx context.Context) error
	// Stop performs cleanup. Called after Start returns.
	Stop(ctx context.Context) error
}

// New creates a new Daemon with the given configuration.
func New(cfg Config) *Daemon {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Daemon{
		config: cfg,
		tasks:  make([]Task, 0),
	}
}

// RegisterTask adds a task to be managed by the daemon.
func (d *Daemon) RegisterTask(t Task) {
	d.tasksMu.Lock()
	defer d.tasksMu.Unlock()
	d.tasks = append(d.tasks, t)
}

// SetReady marks the daemon as ready to receive traffic.
func (d *Daemon) SetReady(ready bool) {
	d.readyMu.Lock()
	defer d.readyMu.Unlock()
	d.ready = ready
}

// IsReady returns whether the daemon is ready.
func (d *Daemon) IsReady() bool {
	d.readyMu.RLock()
	defer d.readyMu.RUnlock()
	return d.ready
}

// Run starts the daemon and blocks until shutdown signal is received.
func (d *Daemon) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start health server
	if err := d.startHealthServer(); err != nil {
		return err
	}

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start all tasks
	var wg sync.WaitGroup
	errCh := make(chan error, len(d.tasks))

	d.tasksMu.Lock()
	for _, task := range d.tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			d.config.Logger.Info("starting task", "task", t.Name())
			if err := t.Start(ctx); err != nil && ctx.Err() == nil {
				d.config.Logger.Error("task failed", "task", t.Name(), "error", err)
				errCh <- err
			}
		}(task)
	}
	d.tasksMu.Unlock()

	// Mark as ready
	d.SetReady(true)
	d.config.Logger.Info("daemon ready", "health_port", d.config.HealthPort)

	// Wait for shutdown signal or task failure
	select {
	case sig := <-sigCh:
		d.config.Logger.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		d.config.Logger.Error("task failure triggered shutdown", "error", err)
	case <-ctx.Done():
		d.config.Logger.Info("context cancelled")
	}

	// Begin graceful shutdown
	d.SetReady(false)
	cancel()

	// Wait for tasks to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		d.config.Logger.Info("all tasks stopped gracefully")
	case <-time.After(d.config.ShutdownTimeout):
		d.config.Logger.Warn("shutdown timeout exceeded, forcing exit")
	}

	// Stop tasks
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	d.tasksMu.Lock()
	for _, task := range d.tasks {
		if err := task.Stop(stopCtx); err != nil {
			d.config.Logger.Error("task stop failed", "task", task.Name(), "error", err)
		}
	}
	d.tasksMu.Unlock()

	// Stop health server
	if err := d.healthServer.Shutdown(stopCtx); err != nil {
		d.config.Logger.Error("health server shutdown failed", "error", err)
	}

	return nil
}

func (d *Daemon) startHealthServer() error {
	mux := http.NewServeMux()

	// Liveness probe - always returns 200 if process is running
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Readiness probe - returns 200 only when ready to serve
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		d.readyMu.RLock()
		ready := d.ready
		d.readyMu.RUnlock()

		if ready {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
		}
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	d.healthServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", d.config.HealthPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := d.healthServer.ListenAndServe(); err != http.ErrServerClosed {
			d.config.Logger.Error("health server error", "error", err)
		}
	}()

	return nil
}
