//nolint:paralleltest // Can't run rate limiter tests in parallel due to global state
package namedotcom_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

func TestInitRateLimiters(t *testing.T) {
	// Reset state before test
	resetRateLimiters()

	perSecond := 10
	perHour := 500

	namedotcom.InitRateLimiters(perSecond, perHour)

	// Verify initialization
	if !getRateLimiterState() {
		t.Error("Rate limiters should be initialized")
	}
}

func TestInitRateLimiters_ThreadSafety(t *testing.T) {
	// Reset state before test
	resetRateLimiters()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Start multiple goroutines to test thread safety
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			namedotcom.InitRateLimiters(20, 1000)
		}()
	}

	wg.Wait()

	// Verify that rate limiters are properly initialized
	if !getRateLimiterState() {
		t.Error("Rate limiters should be initialized after concurrent access")
	}
}

func TestRespectRateLimits_WithInitializedLimiters(t *testing.T) {

	// Initialize rate limiters first
	namedotcom.InitRateLimiters(100, 10000) // High limits to avoid blocking in test

	ctx := context.Background()
	err := namedotcom.RespectRateLimits(ctx)

	if err != nil {
		t.Errorf("RespectRateLimits should not return error with initialized limiters: %v", err)
	}
}

func TestRespectRateLimits_BasicFunctionality(t *testing.T) {

	// Initialize with reasonable limits
	namedotcom.InitRateLimiters(100, 10000)

	ctx := context.Background()
	err := namedotcom.RespectRateLimits(ctx)

	if err != nil {
		t.Errorf("RespectRateLimits should not return error: %v", err)
	}
}

func TestRespectRateLimits_ContextCancellation(t *testing.T) {

	// Initialize with reasonable limits
	namedotcom.InitRateLimiters(5, 100)

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	// Try to use cancelled context
	err := namedotcom.RespectRateLimits(ctx)

	if err == nil {
		t.Error("RespectRateLimits should return error when context is cancelled")
	}

	if err.Error() != "per-second rate limiter error: context canceled" {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestRespectRateLimits_Timeout(t *testing.T) {

	// Initialize with reasonable limits
	namedotcom.InitRateLimiters(10, 1000)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Should timeout immediately
	err := namedotcom.RespectRateLimits(ctx)

	if err == nil {
		t.Error("RespectRateLimits should return error when context times out")
	}
}

func TestRespectRateLimits_RateLimiting(t *testing.T) {
	// Initialize with strict limits
	perSecond := 2
	namedotcom.InitRateLimiters(perSecond, 100)

	ctx := context.Background()
	start := time.Now()

	// Make multiple rapid calls
	for i := range 4 {
		err := namedotcom.RespectRateLimits(ctx)
		if err != nil {
			t.Errorf("Call %d failed: %v", i, err)
		}
	}

	elapsed := time.Since(start)

	// Should take at least 1 second due to rate limiting
	// (first 2 calls are immediate, next 2 require waiting)
	expectedMinDuration := 1 * time.Second
	if elapsed < expectedMinDuration {
		t.Errorf("Expected rate limiting to take at least %v, but took %v", expectedMinDuration, elapsed)
	}
}

// Helper functions for testing.

// resetRateLimiters resets the global rate limiters for testing.
// This function accesses package-level variables which is not ideal in production code
// but necessary for testing the rate limiting functionality.
func resetRateLimiters() {
	// We need to call a function that resets the internal state
	// Since we can't access private variables, we'll reinitialize with defaults
	namedotcom.InitRateLimiters(20, 3000)
}

// getRateLimiterState returns whether rate limiters are initialized.
// This is a simplified check by attempting to use the rate limiters.
func getRateLimiterState() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := namedotcom.RespectRateLimits(ctx)

	return err == nil
}
