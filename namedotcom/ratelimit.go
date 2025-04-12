package namedotcom

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"golang.org/x/time/rate"
)

var (
	// Global rate limiters
	perSecondLimiter *rate.Limiter
	perHourLimiter   *rate.Limiter
	
	// Use mutex to ensure thread-safe initialization
	limiterMutex sync.Mutex
	initialized  bool
)

// InitRateLimiters initializes the rate limiters with specified limits
func InitRateLimiters(perSecond, perHour int) {
	limiterMutex.Lock()
	defer limiterMutex.Unlock()
	
	perSecondLimiter = rate.NewLimiter(rate.Limit(perSecond), perSecond)
	perHourLimiter = rate.NewLimiter(rate.Limit(float64(perHour)/3600), perHour/10)
	initialized = true
}

// RespectRateLimits waits until rate limiters allow the request to proceed
func RespectRateLimits(ctx context.Context) error {
	limiterMutex.Lock()
	if !initialized {
		// Default values if not explicitly initialized
		perSecondLimiter = rate.NewLimiter(rate.Limit(20), 20)
		perHourLimiter = rate.NewLimiter(rate.Limit(float64(3000)/3600), 300)
		initialized = true
	}
	limiterMutex.Unlock()
	
	// Wait for both limiters
	if err := perSecondLimiter.Wait(ctx); err != nil {
		return errors.Wrap(err, "per-second rate limiter error")
	}
	
	if err := perHourLimiter.Wait(ctx); err != nil {
		return errors.Wrap(err, "per-hour rate limiter error")
	}
	
	return nil
}
