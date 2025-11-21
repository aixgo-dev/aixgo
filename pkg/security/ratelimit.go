package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	globalLimiter  *rate.Limiter
	clientLimiters map[string]*rate.Limiter
	mu             sync.RWMutex

	// Configuration
	requestsPerSecond float64
	burst             int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		globalLimiter:     rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
		clientLimiters:    make(map[string]*rate.Limiter),
		requestsPerSecond: requestsPerSecond,
		burst:             burst,
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(clientID string) bool {
	// Check global rate limit
	if !rl.globalLimiter.Allow() {
		return false
	}

	// Check per-client rate limit
	limiter := rl.getClientLimiter(clientID)
	return limiter.Allow()
}

// Wait blocks until a request can be made
func (rl *RateLimiter) Wait(ctx context.Context, clientID string) error {
	// Wait for global rate limit
	if err := rl.globalLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("global rate limit: %w", err)
	}

	// Wait for per-client rate limit
	limiter := rl.getClientLimiter(clientID)
	if err := limiter.Wait(ctx); err != nil {
		return fmt.Errorf("client rate limit: %w", err)
	}

	return nil
}

// getClientLimiter gets or creates a rate limiter for a specific client
func (rl *RateLimiter) getClientLimiter(clientID string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.clientLimiters[clientID]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter for client
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists := rl.clientLimiters[clientID]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rate.Limit(rl.requestsPerSecond), rl.burst)
	rl.clientLimiters[clientID] = limiter
	return limiter
}

// ToolRateLimiter provides per-tool rate limiting
type ToolRateLimiter struct {
	toolLimiters map[string]*rate.Limiter
	mu           sync.RWMutex
}

// NewToolRateLimiter creates a new tool-specific rate limiter
func NewToolRateLimiter() *ToolRateLimiter {
	return &ToolRateLimiter{
		toolLimiters: make(map[string]*rate.Limiter),
	}
}

// SetToolLimit configures rate limit for a specific tool
func (trl *ToolRateLimiter) SetToolLimit(toolName string, requestsPerSecond float64, burst int) {
	trl.mu.Lock()
	defer trl.mu.Unlock()
	trl.toolLimiters[toolName] = rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
}

// Allow checks if a tool execution should be allowed
func (trl *ToolRateLimiter) Allow(toolName string) bool {
	trl.mu.RLock()
	limiter, exists := trl.toolLimiters[toolName]
	trl.mu.RUnlock()

	if !exists {
		return true // No limit set for this tool
	}

	return limiter.Allow()
}

// Wait blocks until a tool execution can proceed
func (trl *ToolRateLimiter) Wait(ctx context.Context, toolName string) error {
	trl.mu.RLock()
	limiter, exists := trl.toolLimiters[toolName]
	trl.mu.RUnlock()

	if !exists {
		return nil // No limit set for this tool
	}

	return limiter.Wait(ctx)
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration

	mu              sync.RWMutex
	failures        int
	lastFailureTime time.Time
	state           CircuitState
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
	}
}

// Execute runs a function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if circuit should transition to half-open
	if cb.state == CircuitOpen && time.Since(cb.lastFailureTime) > cb.resetTimeout {
		cb.state = CircuitHalfOpen
		cb.failures = 0
	}

	// Don't execute if circuit is open
	if cb.state == CircuitOpen {
		return fmt.Errorf("circuit breaker is open")
	}

	// Execute the function
	err := fn()

	if err != nil {
		cb.failures++
		cb.lastFailureTime = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.state = CircuitOpen
		}

		return err
	}

	// Success - reset circuit
	cb.failures = 0
	cb.state = CircuitClosed

	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = CircuitClosed
}

// TimeoutManager manages execution timeouts
type TimeoutManager struct {
	defaultTimeout time.Duration
	toolTimeouts   map[string]time.Duration
	mu             sync.RWMutex
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(defaultTimeout time.Duration) *TimeoutManager {
	return &TimeoutManager{
		defaultTimeout: defaultTimeout,
		toolTimeouts:   make(map[string]time.Duration),
	}
}

// SetToolTimeout sets a specific timeout for a tool
func (tm *TimeoutManager) SetToolTimeout(toolName string, timeout time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.toolTimeouts[toolName] = timeout
}

// GetTimeout returns the timeout for a specific tool
func (tm *TimeoutManager) GetTimeout(toolName string) time.Duration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if timeout, exists := tm.toolTimeouts[toolName]; exists {
		return timeout
	}

	return tm.defaultTimeout
}

// WithTimeout creates a context with the appropriate timeout for a tool
func (tm *TimeoutManager) WithTimeout(ctx context.Context, toolName string) (context.Context, context.CancelFunc) {
	timeout := tm.GetTimeout(toolName)
	return context.WithTimeout(ctx, timeout)
}
