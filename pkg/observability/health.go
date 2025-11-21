package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// HealthStatus represents the health status of the service
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a single health check
type HealthCheck struct {
	Name        string
	CheckFunc   func(context.Context) error
	Timeout     time.Duration
	Critical    bool
	lastChecked time.Time
	lastStatus  error
	mu          sync.RWMutex
}

// HealthChecker manages health checks
type HealthChecker struct {
	checks map[string]*HealthCheck
	mu     sync.RWMutex
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Uptime    time.Duration          `json:"uptime"`
	Checks    map[string]CheckStatus `json:"checks"`
	System    SystemInfo             `json:"system"`
}

// CheckStatus represents the status of a health check
type CheckStatus struct {
	Status      HealthStatus `json:"status"`
	Message     string       `json:"message,omitempty"`
	LastChecked time.Time    `json:"last_checked"`
	Duration    string       `json:"duration,omitempty"`
}

// SystemInfo represents system information
type SystemInfo struct {
	NumGoroutines int    `json:"num_goroutines"`
	NumCPU        int    `json:"num_cpu"`
	MemAlloc      uint64 `json:"mem_alloc_mb"`
	MemSys        uint64 `json:"mem_sys_mb"`
}

var (
	globalChecker  *HealthChecker
	startTime      time.Time
	version        = "1.0.0"
	initHealthOnce sync.Once
)

func init() {
	startTime = time.Now()
}

// InitHealthChecker initializes the global health checker
func InitHealthChecker() *HealthChecker {
	initHealthOnce.Do(func() {
		globalChecker = &HealthChecker{
			checks: make(map[string]*HealthCheck),
		}
	})
	return globalChecker
}

// GetHealthChecker returns the global health checker
func GetHealthChecker() *HealthChecker {
	if globalChecker == nil {
		return InitHealthChecker()
	}
	return globalChecker
}

// RegisterCheck registers a new health check
func (hc *HealthChecker) RegisterCheck(check *HealthCheck) {
	if check.Timeout == 0 {
		check.Timeout = 5 * time.Second
	}
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[check.Name] = check
}

// Check performs all health checks
func (hc *HealthChecker) Check(ctx context.Context) HealthResponse {
	hc.mu.RLock()
	checks := make(map[string]*HealthCheck, len(hc.checks))
	for k, v := range hc.checks {
		checks[k] = v
	}
	hc.mu.RUnlock()

	checkResults := make(map[string]CheckStatus)
	overallStatus := HealthStatusHealthy

	for name, check := range checks {
		status := hc.performCheck(ctx, check)
		checkResults[name] = status

		// Update overall status
		if status.Status == HealthStatusUnhealthy && check.Critical {
			overallStatus = HealthStatusUnhealthy
		} else if status.Status == HealthStatusDegraded && overallStatus == HealthStatusHealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	return HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   version,
		Uptime:    time.Since(startTime),
		Checks:    checkResults,
		System:    getSystemInfo(),
	}
}

// performCheck performs a single health check
func (hc *HealthChecker) performCheck(ctx context.Context, check *HealthCheck) CheckStatus {
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- check.CheckFunc(checkCtx)
	}()

	var err error
	select {
	case err = <-errChan:
	case <-checkCtx.Done():
		err = checkCtx.Err()
	}

	duration := time.Since(start)

	check.mu.Lock()
	check.lastChecked = time.Now()
	check.lastStatus = err
	check.mu.Unlock()

	status := CheckStatus{
		LastChecked: check.lastChecked,
		Duration:    duration.String(),
	}

	if err != nil {
		if check.Critical {
			status.Status = HealthStatusUnhealthy
		} else {
			status.Status = HealthStatusDegraded
		}
		status.Message = err.Error()
	} else {
		status.Status = HealthStatusHealthy
		status.Message = "OK"
	}

	return status
}

// HealthHandler returns an HTTP handler for health checks
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checker := GetHealthChecker()
		response := checker.Check(r.Context())

		w.Header().Set("Content-Type", "application/json")

		// Set HTTP status code based on health status
		switch response.Status {
		case HealthStatusHealthy:
			w.WriteHeader(http.StatusOK)
		case HealthStatusDegraded:
			w.WriteHeader(http.StatusOK) // Still OK but degraded
		case HealthStatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		_ = json.NewEncoder(w).Encode(response)
	}
}

// LivenessHandler returns a simple liveness probe handler
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

// ReadinessHandler returns a readiness probe handler
func ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checker := GetHealthChecker()
		response := checker.Check(r.Context())

		w.Header().Set("Content-Type", "application/json")

		// Only ready if healthy
		if response.Status == HealthStatusHealthy {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "ready",
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "not ready",
			})
		}
	}
}

// getSystemInfo returns system information
func getSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemInfo{
		NumGoroutines: runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		MemAlloc:      m.Alloc / 1024 / 1024,
		MemSys:        m.Sys / 1024 / 1024,
	}
}

// Common health check functions

// PingCheck creates a simple ping health check
func PingCheck() *HealthCheck {
	return &HealthCheck{
		Name: "ping",
		CheckFunc: func(ctx context.Context) error {
			return nil
		},
		Timeout:  1 * time.Second,
		Critical: false,
	}
}

// DatabaseCheck creates a database health check
func DatabaseCheck(pingFunc func(context.Context) error) *HealthCheck {
	return &HealthCheck{
		Name:      "database",
		CheckFunc: pingFunc,
		Timeout:   5 * time.Second,
		Critical:  true,
	}
}

// ExternalServiceCheck creates an external service health check
func ExternalServiceCheck(name string, checkFunc func(context.Context) error) *HealthCheck {
	return &HealthCheck{
		Name:      name,
		CheckFunc: checkFunc,
		Timeout:   10 * time.Second,
		Critical:  false,
	}
}
