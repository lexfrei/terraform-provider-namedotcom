package namedotcom

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"golang.org/x/time/rate"
)

const (
	// Default rate limits.
	defaultPerSecondLimit = 20
	defaultPerHourLimit   = 3000
	// Default burst size for hourly limiter (10% of hourly limit).
	defaultHourlyBurst = 300
	// Burst divisor for calculating hourly burst from limit.
	burstDivisor = 10
	// Seconds per hour for rate calculation.
	secondsPerHour = 3600
)

var (
	// Global rate limiters.
	perSecondLimiter *rate.Limiter
	perHourLimiter   *rate.Limiter

	// Use mutex to ensure thread-safe initialization.
	limiterMutex sync.Mutex
	initialized  bool
)

// InitRateLimiters initializes the rate limiters with specified limits.
func InitRateLimiters(perSecond, perHour int) {
	limiterMutex.Lock()
	defer limiterMutex.Unlock()

	perSecondLimiter = rate.NewLimiter(rate.Limit(perSecond), perSecond)
	perHourLimiter = rate.NewLimiter(rate.Limit(float64(perHour)/secondsPerHour), perHour/burstDivisor)
	initialized = true
}

// RespectRateLimits waits until rate limiters allow the request to proceed.
func RespectRateLimits(ctx context.Context) error {
	limiterMutex.Lock()
	if !initialized {
		// Default values if not explicitly initialized
		perSecondLimiter = rate.NewLimiter(rate.Limit(defaultPerSecondLimit), defaultPerSecondLimit)
		perHourLimiter = rate.NewLimiter(rate.Limit(float64(defaultPerHourLimit)/secondsPerHour), defaultHourlyBurst)
		initialized = true
	}
	limiterMutex.Unlock()

	// Wait for both limiters
	err := perSecondLimiter.Wait(ctx)
	if err != nil {
		return errors.Wrap(err, "per-second rate limiter error")
	}

	err = perHourLimiter.Wait(ctx)
	if err != nil {
		return errors.Wrap(err, "per-hour rate limiter error")
	}

	return nil
}
