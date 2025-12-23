package controller

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimiter implements rate limiting for deletions using token bucket algorithm.
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a new rate limiter.
// MaxPerSecond specifies the maximum number of deletions allowed per second.
func NewRateLimiter(maxPerSecond int) *RateLimiter {
	if maxPerSecond <= 0 {
		maxPerSecond = DefaultMaxDeletionsPerSecond
	}

	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(maxPerSecond), maxPerSecond),
	}
}

// Wait waits until the next deletion is allowed, respecting the rate limit.
// It returns an error if the context is canceled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

// SetRate updates the rate limit dynamically.
func (rl *RateLimiter) SetRate(maxPerSecond int) {
	if maxPerSecond <= 0 {
		maxPerSecond = DefaultMaxDeletionsPerSecond
	}
	rl.limiter.SetLimit(rate.Limit(maxPerSecond))
	rl.limiter.SetBurst(maxPerSecond)
}
