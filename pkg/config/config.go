/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package config provides centralized configuration management for the GC controller.
package config

import (
	"os"
	"strconv"
	"time"
)

// Default values for controller configuration.
const (
	// DefaultGCInterval is the default interval for GC runs.
	DefaultGCInterval = 1 * time.Minute

	// DefaultMaxDeletionsPerSecond is the default rate limit for deletions.
	DefaultMaxDeletionsPerSecond = 10

	// DefaultBatchSize is the default batch size for deletions.
	DefaultBatchSize = 50

	// DefaultMaxConcurrentEvaluations is the default number of concurrent policy evaluations.
	DefaultMaxConcurrentEvaluations = 5
)

// ControllerConfig holds configuration for the GC controller.
type ControllerConfig struct {
	// GCInterval is the interval between GC evaluation runs.
	GCInterval time.Duration

	// MaxDeletionsPerSecond is the default maximum deletions per second.
	// Individual policies can override this.
	MaxDeletionsPerSecond int

	// BatchSize is the default batch size for deletions.
	// Individual policies can override this.
	BatchSize int

	// MaxConcurrentEvaluations is the maximum number of policies to evaluate concurrently.
	// Defaults to 5 if not set.
	MaxConcurrentEvaluations int
}

// NewControllerConfig creates a new controller config with defaults.
func NewControllerConfig() *ControllerConfig {
	return &ControllerConfig{
		GCInterval:               DefaultGCInterval,
		MaxDeletionsPerSecond:    DefaultMaxDeletionsPerSecond,
		BatchSize:                DefaultBatchSize,
		MaxConcurrentEvaluations: DefaultMaxConcurrentEvaluations,
	}
}

// LoadFromEnv loads configuration from environment variables.
// Environment variables override defaults if set.
// This implementation uses direct os.Getenv for simplicity, but could be enhanced
// to use zen-sdk/pkg/config/validator for validation if needed.
func (c *ControllerConfig) LoadFromEnv() {
	// GC_INTERVAL - duration string (e.g., "1m", "30s", "2h")
	if val := os.Getenv("GC_INTERVAL"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			c.GCInterval = d
		}
	}

	// GC_MAX_DELETIONS_PER_SECOND - integer
	if val := os.Getenv("GC_MAX_DELETIONS_PER_SECOND"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			c.MaxDeletionsPerSecond = i
		}
	}

	// GC_BATCH_SIZE - integer
	if val := os.Getenv("GC_BATCH_SIZE"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			c.BatchSize = i
		}
	}

	// GC_MAX_CONCURRENT_EVALUATIONS - integer
	if val := os.Getenv("GC_MAX_CONCURRENT_EVALUATIONS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			c.MaxConcurrentEvaluations = i
		}
	}
}

// WithGCInterval sets the GC interval.
func (c *ControllerConfig) WithGCInterval(interval time.Duration) *ControllerConfig {
	c.GCInterval = interval
	return c
}

// WithMaxDeletionsPerSecond sets the max deletions per second.
func (c *ControllerConfig) WithMaxDeletionsPerSecond(rate int) *ControllerConfig {
	c.MaxDeletionsPerSecond = rate
	return c
}

// WithBatchSize sets the batch size.
func (c *ControllerConfig) WithBatchSize(size int) *ControllerConfig {
	c.BatchSize = size
	return c
}

// WithMaxConcurrentEvaluations sets the maximum concurrent evaluations.
func (c *ControllerConfig) WithMaxConcurrentEvaluations(maxConcurrent int) *ControllerConfig {
	c.MaxConcurrentEvaluations = maxConcurrent
	return c
}
