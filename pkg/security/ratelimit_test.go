package security

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test Rate Limit Enforcement
func TestRateLimiter_BasicEnforcement(t *testing.T) {
	limiter := NewRateLimiter(2.0, 2) // 2 requests per second, burst of 2

	clientID := "client1"

	// First two requests should succeed (burst)
	if !limiter.Allow(clientID) {
		t.Error("first request should be allowed")
	}
	if !limiter.Allow(clientID) {
		t.Error("second request should be allowed")
	}

	// Third request should fail (rate limited)
	if limiter.Allow(clientID) {
		t.Error("third request should be rate limited")
	}
}

// Test Rate Limit Reset
func TestRateLimiter_RateReset(t *testing.T) {
	limiter := NewRateLimiter(2.0, 2) // 2 requests per second, burst of 2

	clientID := "client1"

	// Consume burst
	limiter.Allow(clientID)
	limiter.Allow(clientID)

	// Should be rate limited
	if limiter.Allow(clientID) {
		t.Error("request should be rate limited")
	}

	// Wait for rate to refill
	time.Sleep(600 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow(clientID) {
		t.Error("request should be allowed after waiting")
	}
}

// Test Multiple Clients
func TestRateLimiter_MultipleClients(t *testing.T) {
	// Use higher limits to accommodate both global and per-client limits
	limiter := NewRateLimiter(10.0, 10)

	client1 := "client1"
	client2 := "client2"

	// Both clients should have independent per-client rate limits
	// but share the global rate limit
	if !limiter.Allow(client1) {
		t.Error("client1 first request should be allowed")
	}
	if !limiter.Allow(client1) {
		t.Error("client1 second request should be allowed")
	}

	if !limiter.Allow(client2) {
		t.Error("client2 first request should be allowed")
	}
	if !limiter.Allow(client2) {
		t.Error("client2 second request should be allowed")
	}

	// Exhaust both clients' burst capacity
	for i := 0; i < 8; i++ {
		if i%2 == 0 {
			limiter.Allow(client1)
		} else {
			limiter.Allow(client2)
		}
	}

	// Both should be rate limited now (either by global or per-client limit)
	if limiter.Allow(client1) {
		t.Error("client1 should be rate limited after exhausting capacity")
	}
	if limiter.Allow(client2) {
		t.Error("client2 should be rate limited after exhausting capacity")
	}
}

// Test Global Rate Limit
func TestRateLimiter_GlobalLimit(t *testing.T) {
	limiter := NewRateLimiter(5.0, 5) // 5 requests per second globally

	// Create multiple clients trying to exceed global limit
	clients := []string{"client1", "client2", "client3"}
	allowed := 0
	denied := 0

	for i := 0; i < 20; i++ {
		clientID := clients[i%len(clients)]
		if limiter.Allow(clientID) {
			allowed++
		} else {
			denied++
		}
	}

	// Global limit should have kicked in
	if denied == 0 {
		t.Error("expected some requests to be denied by global rate limit")
	}

	t.Logf("allowed=%d, denied=%d", allowed, denied)
}

// Test Wait Functionality
func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(2.0, 1) // 2 requests per second, burst of 1

	clientID := "client1"
	ctx := context.Background()

	// First request should succeed immediately
	if err := limiter.Wait(ctx, clientID); err != nil {
		t.Errorf("first wait should succeed: %v", err)
	}

	// Second request should wait
	start := time.Now()
	if err := limiter.Wait(ctx, clientID); err != nil {
		t.Errorf("second wait should succeed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have waited approximately 500ms (half second for 2 req/sec)
	if elapsed < 400*time.Millisecond {
		t.Errorf("wait duration too short: %v", elapsed)
	}
}

// Test Wait with Context Cancellation
func TestRateLimiter_WaitContextCancel(t *testing.T) {
	limiter := NewRateLimiter(1.0, 1) // 1 request per second

	clientID := "client1"

	// Consume the burst
	limiter.Allow(clientID)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should fail due to context cancellation
	err := limiter.Wait(ctx, clientID)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

// Test Concurrent Access
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(10.0, 10) // 10 requests per second

	var wg sync.WaitGroup
	var allowed, denied int32

	// Simulate 100 concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := "client1"
			if limiter.Allow(clientID) {
				atomic.AddInt32(&allowed, 1)
			} else {
				atomic.AddInt32(&denied, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("allowed=%d, denied=%d", allowed, denied)

	// Should have some allowed and some denied
	if allowed == 0 {
		t.Error("expected some requests to be allowed")
	}
	if denied == 0 {
		t.Error("expected some requests to be denied")
	}
}

// Test Tool Rate Limiter
func TestToolRateLimiter_BasicEnforcement(t *testing.T) {
	toolLimiter := NewToolRateLimiter()

	toolName := "dangerous_tool"
	toolLimiter.SetToolLimit(toolName, 1.0, 1) // 1 request per second

	// First request should succeed
	if !toolLimiter.Allow(toolName) {
		t.Error("first request should be allowed")
	}

	// Second request should fail
	if toolLimiter.Allow(toolName) {
		t.Error("second request should be rate limited")
	}

	// Wait and try again
	time.Sleep(1100 * time.Millisecond)

	if !toolLimiter.Allow(toolName) {
		t.Error("request should be allowed after waiting")
	}
}

// Test Tool Rate Limiter - No Limit Set
func TestToolRateLimiter_NoLimit(t *testing.T) {
	toolLimiter := NewToolRateLimiter()

	toolName := "safe_tool"

	// Should allow unlimited requests if no limit is set
	for i := 0; i < 100; i++ {
		if !toolLimiter.Allow(toolName) {
			t.Errorf("request %d should be allowed (no limit set)", i)
		}
	}
}

// Test Tool Rate Limiter - Multiple Tools
func TestToolRateLimiter_MultipleTools(t *testing.T) {
	toolLimiter := NewToolRateLimiter()

	tool1 := "tool1"
	tool2 := "tool2"

	toolLimiter.SetToolLimit(tool1, 2.0, 2)
	toolLimiter.SetToolLimit(tool2, 5.0, 5)

	// Tool1: consume burst
	if !toolLimiter.Allow(tool1) {
		t.Error("tool1 first request should be allowed")
	}
	if !toolLimiter.Allow(tool1) {
		t.Error("tool1 second request should be allowed")
	}
	if toolLimiter.Allow(tool1) {
		t.Error("tool1 should be rate limited")
	}

	// Tool2: should still have capacity
	for i := 0; i < 5; i++ {
		if !toolLimiter.Allow(tool2) {
			t.Errorf("tool2 request %d should be allowed", i)
		}
	}
}

// Test Tool Rate Limiter Wait
func TestToolRateLimiter_Wait(t *testing.T) {
	toolLimiter := NewToolRateLimiter()

	toolName := "slow_tool"
	toolLimiter.SetToolLimit(toolName, 2.0, 1)

	ctx := context.Background()

	// First request immediate
	if err := toolLimiter.Wait(ctx, toolName); err != nil {
		t.Errorf("first wait should succeed: %v", err)
	}

	// Second request should wait
	start := time.Now()
	if err := toolLimiter.Wait(ctx, toolName); err != nil {
		t.Errorf("second wait should succeed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 400*time.Millisecond {
		t.Errorf("wait duration too short: %v", elapsed)
	}
}

// Test Circuit Breaker - Basic Operation
func TestCircuitBreaker_BasicOperation(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	// Should start closed
	if cb.GetState() != CircuitClosed {
		t.Error("circuit breaker should start closed")
	}

	// Successful executions
	for i := 0; i < 5; i++ {
		err := cb.Execute(func() error {
			return nil
		})
		if err != nil {
			t.Errorf("successful execution failed: %v", err)
		}
	}

	if cb.GetState() != CircuitClosed {
		t.Error("circuit breaker should remain closed on success")
	}
}

// Test Circuit Breaker - Opens on Failures
func TestCircuitBreaker_OpensOnFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	failErr := errors.New("operation failed")

	// Fail 3 times to open circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			return failErr
		})
		if err != failErr {
			t.Errorf("expected failure error, got: %v", err)
		}
	}

	// Circuit should now be open
	if cb.GetState() != CircuitOpen {
		t.Errorf("circuit breaker should be open, got state: %v", cb.GetState())
	}

	// Next execution should fail immediately
	err := cb.Execute(func() error {
		return nil
	})
	if err == nil {
		t.Error("execution should fail when circuit is open")
	}
	if err.Error() != "circuit breaker is open" {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test Circuit Breaker - Half-Open State
func TestCircuitBreaker_HalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 500*time.Millisecond)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	if cb.GetState() != CircuitOpen {
		t.Error("circuit should be open")
	}

	// Wait for reset timeout
	time.Sleep(600 * time.Millisecond)

	// Next execution should transition to half-open
	err := cb.Execute(func() error {
		return nil // succeed
	})

	if err != nil {
		t.Errorf("execution in half-open should succeed: %v", err)
	}

	// Should be closed again after success
	if cb.GetState() != CircuitClosed {
		t.Errorf("circuit should be closed after successful half-open, got: %v", cb.GetState())
	}
}

// Test Circuit Breaker - Reset
func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2, 1*time.Second)

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})
	}

	if cb.GetState() != CircuitOpen {
		t.Error("circuit should be open")
	}

	// Manual reset
	cb.Reset()

	if cb.GetState() != CircuitClosed {
		t.Error("circuit should be closed after reset")
	}

	// Should work normally
	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("execution should succeed after reset: %v", err)
	}
}

// Test Timeout Manager - Default Timeout
func TestTimeoutManager_DefaultTimeout(t *testing.T) {
	tm := NewTimeoutManager(5 * time.Second)

	timeout := tm.GetTimeout("unknown_tool")
	if timeout != 5*time.Second {
		t.Errorf("default timeout = %v, want 5s", timeout)
	}
}

// Test Timeout Manager - Custom Tool Timeout
func TestTimeoutManager_CustomToolTimeout(t *testing.T) {
	tm := NewTimeoutManager(5 * time.Second)

	toolName := "slow_tool"
	tm.SetToolTimeout(toolName, 10*time.Second)

	timeout := tm.GetTimeout(toolName)
	if timeout != 10*time.Second {
		t.Errorf("tool timeout = %v, want 10s", timeout)
	}

	// Other tools should still use default
	defaultTimeout := tm.GetTimeout("other_tool")
	if defaultTimeout != 5*time.Second {
		t.Errorf("default timeout = %v, want 5s", defaultTimeout)
	}
}

// Test Timeout Manager - Context Creation
func TestTimeoutManager_WithTimeout(t *testing.T) {
	tm := NewTimeoutManager(1 * time.Second)

	toolName := "fast_tool"
	tm.SetToolTimeout(toolName, 100*time.Millisecond)

	ctx := context.Background()
	timeoutCtx, cancel := tm.WithTimeout(ctx, toolName)
	defer cancel()

	// Context should have deadline
	deadline, ok := timeoutCtx.Deadline()
	if !ok {
		t.Error("context should have deadline")
	}

	// Deadline should be approximately 100ms from now
	expectedDeadline := time.Now().Add(100 * time.Millisecond)
	if deadline.Before(expectedDeadline.Add(-50*time.Millisecond)) ||
		deadline.After(expectedDeadline.Add(50*time.Millisecond)) {
		t.Errorf("deadline = %v, want approximately %v", deadline, expectedDeadline)
	}
}

// Test Timeout Manager - Context Expires
func TestTimeoutManager_ContextExpires(t *testing.T) {
	tm := NewTimeoutManager(100 * time.Millisecond)

	ctx := context.Background()
	timeoutCtx, cancel := tm.WithTimeout(ctx, "test_tool")
	defer cancel()

	// Wait for context to expire
	<-timeoutCtx.Done()

	if timeoutCtx.Err() != context.DeadlineExceeded {
		t.Errorf("context error = %v, want DeadlineExceeded", timeoutCtx.Err())
	}
}

// Test Rate Limit Burst Handling
func TestRateLimiter_BurstHandling(t *testing.T) {
	limiter := NewRateLimiter(1.0, 5) // 1 request per second, burst of 5

	clientID := "client1"

	// Should allow burst of 5 immediately
	for i := 0; i < 5; i++ {
		if !limiter.Allow(clientID) {
			t.Errorf("burst request %d should be allowed", i)
		}
	}

	// Next request should be denied
	if limiter.Allow(clientID) {
		t.Error("request beyond burst should be denied")
	}

	// Wait for one request to refill
	time.Sleep(1100 * time.Millisecond)

	// Should allow one more request
	if !limiter.Allow(clientID) {
		t.Error("request after waiting should be allowed")
	}
}

// Test Configuration Changes
func TestToolRateLimiter_ConfigurationChanges(t *testing.T) {
	toolLimiter := NewToolRateLimiter()

	toolName := "dynamic_tool"

	// Set initial limit
	toolLimiter.SetToolLimit(toolName, 1.0, 1)

	// Consume the limit
	if !toolLimiter.Allow(toolName) {
		t.Error("first request should be allowed")
	}
	if toolLimiter.Allow(toolName) {
		t.Error("second request should be denied")
	}

	// Change the limit
	toolLimiter.SetToolLimit(toolName, 10.0, 10)

	// Should now allow more requests
	allowed := 0
	for i := 0; i < 10; i++ {
		if toolLimiter.Allow(toolName) {
			allowed++
		}
	}

	if allowed < 5 {
		t.Errorf("expected at least 5 allowed requests after config change, got %d", allowed)
	}
}

// Test Concurrent Circuit Breaker Access
func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(10, 1*time.Second)

	var wg sync.WaitGroup
	var successCount, failCount int32

	// Concurrent executions
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			err := cb.Execute(func() error {
				if id%10 == 0 {
					return errors.New("fail")
				}
				return nil
			})

			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("success=%d, fail=%d", successCount, failCount)

	if successCount == 0 {
		t.Error("expected some successful executions")
	}
}

// Benchmark tests
func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000.0, 1000)
	clientID := "client1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(clientID)
	}
}

func BenchmarkToolRateLimiter_Allow(b *testing.B) {
	toolLimiter := NewToolRateLimiter()
	toolLimiter.SetToolLimit("test_tool", 1000.0, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toolLimiter.Allow("test_tool")
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	cb := NewCircuitBreaker(1000, 1*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.Execute(func() error {
			return nil
		})
	}
}

func BenchmarkTimeoutManager_GetTimeout(b *testing.B) {
	tm := NewTimeoutManager(5 * time.Second)
	tm.SetToolTimeout("test_tool", 10*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tm.GetTimeout("test_tool")
	}
}
