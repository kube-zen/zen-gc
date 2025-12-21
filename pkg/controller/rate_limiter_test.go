package controller

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name           string
		maxPerSecond   int
		expectedLimit  int
		expectedBurst  int
	}{
		{
			name:          "valid rate",
			maxPerSecond:  10,
			expectedLimit: 10,
			expectedBurst: 10,
		},
		{
			name:          "zero rate uses default",
			maxPerSecond:  0,
			expectedLimit: DefaultMaxDeletionsPerSecond,
			expectedBurst: DefaultMaxDeletionsPerSecond,
		},
		{
			name:          "negative rate uses default",
			maxPerSecond:  -1,
			expectedLimit: DefaultMaxDeletionsPerSecond,
			expectedBurst: DefaultMaxDeletionsPerSecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.maxPerSecond)
			if rl == nil {
				t.Fatal("NewRateLimiter returned nil")
			}
			if rl.limiter == nil {
				t.Fatal("RateLimiter.limiter is nil")
			}
		})
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(10)
	ctx := context.Background()

	// Test that Wait doesn't block immediately (token bucket should have tokens)
	start := time.Now()
	err := rl.Wait(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Wait() returned error: %v", err)
	}

	// Should return quickly (within 100ms)
	if duration > 100*time.Millisecond {
		t.Errorf("Wait() took too long: %v", duration)
	}
}

func TestRateLimiter_Wait_ContextCancelled(t *testing.T) {
	rl := NewRateLimiter(1) // Very low rate to ensure we hit the limit
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Wait should respect context cancellation
	err := rl.Wait(ctx)
	if err == nil {
		t.Error("Wait() should return error when context is cancelled")
	}
}

func TestRateLimiter_SetRate(t *testing.T) {
	rl := NewRateLimiter(10)

	// Change rate
	rl.SetRate(20)
	if rl.limiter == nil {
		t.Fatal("limiter is nil after SetRate")
	}

	// Set to zero should use default
	rl.SetRate(0)
	if rl.limiter == nil {
		t.Fatal("limiter is nil after SetRate(0)")
	}
}

