package controller

import (
	"time"
)

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	tokens chan struct{}
	ticker *time.Ticker
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerSecond int) *RateLimiter {
	rl := &RateLimiter{
		tokens: make(chan struct{}, maxPerSecond),
		ticker: time.NewTicker(time.Second / time.Duration(maxPerSecond)),
	}

	// Fill the bucket initially
	for i := 0; i < maxPerSecond; i++ {
		rl.tokens <- struct{}{}
	}

	// Start refilling tokens
	go rl.refill()

	return rl
}

// refill continuously refills tokens
func (rl *RateLimiter) refill() {
	for range rl.ticker.C {
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Bucket is full, skip
		}
	}
}

// Wait waits for a token to become available
func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	if rl.ticker != nil {
		rl.ticker.Stop()
	}
}

