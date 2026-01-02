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

package config

import (
	"os"
	"testing"
	"time"
)

func TestNewControllerConfig(t *testing.T) {
	cfg := NewControllerConfig()

	if cfg.GCInterval != DefaultGCInterval {
		t.Errorf("Expected GCInterval=%v, got %v", DefaultGCInterval, cfg.GCInterval)
	}

	if cfg.MaxDeletionsPerSecond != DefaultMaxDeletionsPerSecond {
		t.Errorf("Expected MaxDeletionsPerSecond=%d, got %d", DefaultMaxDeletionsPerSecond, cfg.MaxDeletionsPerSecond)
	}

	if cfg.BatchSize != DefaultBatchSize {
		t.Errorf("Expected BatchSize=%d, got %d", DefaultBatchSize, cfg.BatchSize)
	}
}

func TestControllerConfig_LoadFromEnv(t *testing.T) {
	// Save original values
	originalGCInterval := os.Getenv("GC_INTERVAL")
	originalMaxDeletions := os.Getenv("GC_MAX_DELETIONS_PER_SECOND")
	originalBatchSize := os.Getenv("GC_BATCH_SIZE")

	// Clean up after test
	defer func() {
		if originalGCInterval != "" {
			os.Setenv("GC_INTERVAL", originalGCInterval)
		} else {
			os.Unsetenv("GC_INTERVAL")
		}
		if originalMaxDeletions != "" {
			os.Setenv("GC_MAX_DELETIONS_PER_SECOND", originalMaxDeletions)
		} else {
			os.Unsetenv("GC_MAX_DELETIONS_PER_SECOND")
		}
		if originalBatchSize != "" {
			os.Setenv("GC_BATCH_SIZE", originalBatchSize)
		} else {
			os.Unsetenv("GC_BATCH_SIZE")
		}
	}()

	// Test GC_INTERVAL
	os.Setenv("GC_INTERVAL", "2m")
	cfg := NewControllerConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv() returned error: %v", err)
	}
	if cfg.GCInterval != 2*time.Minute {
		t.Errorf("Expected GCInterval=2m, got %v", cfg.GCInterval)
	}

	// Test GC_MAX_DELETIONS_PER_SECOND
	os.Setenv("GC_MAX_DELETIONS_PER_SECOND", "20")
	cfg = NewControllerConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv() returned error: %v", err)
	}
	if cfg.MaxDeletionsPerSecond != 20 {
		t.Errorf("Expected MaxDeletionsPerSecond=20, got %d", cfg.MaxDeletionsPerSecond)
	}

	// Test GC_BATCH_SIZE
	os.Setenv("GC_BATCH_SIZE", "100")
	cfg = NewControllerConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("LoadFromEnv() returned error: %v", err)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("Expected BatchSize=100, got %d", cfg.BatchSize)
	}

	// Test invalid values (should keep defaults and return validation error)
	os.Setenv("GC_MAX_DELETIONS_PER_SECOND", "invalid")
	cfg = NewControllerConfig()
	err := cfg.LoadFromEnv()
	if err == nil {
		t.Error("Expected LoadFromEnv() to return error for invalid GC_MAX_DELETIONS_PER_SECOND")
	}
	if cfg.MaxDeletionsPerSecond != DefaultMaxDeletionsPerSecond {
		t.Errorf("Expected MaxDeletionsPerSecond=%d (default), got %d", DefaultMaxDeletionsPerSecond, cfg.MaxDeletionsPerSecond)
	}

	os.Unsetenv("GC_MAX_DELETIONS_PER_SECOND")
	os.Setenv("GC_BATCH_SIZE", "not-a-number")
	cfg = NewControllerConfig()
	err = cfg.LoadFromEnv()
	if err == nil {
		t.Error("Expected LoadFromEnv() to return error for invalid GC_BATCH_SIZE")
	}
	if cfg.BatchSize != DefaultBatchSize {
		t.Errorf("Expected BatchSize=%d (default), got %d", DefaultBatchSize, cfg.BatchSize)
	}
}

func TestControllerConfig_WithGCInterval(t *testing.T) {
	cfg := NewControllerConfig()
	newInterval := 5 * time.Minute
	cfg.WithGCInterval(newInterval)

	if cfg.GCInterval != newInterval {
		t.Errorf("Expected GCInterval=%v, got %v", newInterval, cfg.GCInterval)
	}
}

func TestControllerConfig_WithMaxDeletionsPerSecond(t *testing.T) {
	cfg := NewControllerConfig()
	newRate := 25
	cfg.WithMaxDeletionsPerSecond(newRate)

	if cfg.MaxDeletionsPerSecond != newRate {
		t.Errorf("Expected MaxDeletionsPerSecond=%d, got %d", newRate, cfg.MaxDeletionsPerSecond)
	}
}

func TestControllerConfig_WithBatchSize(t *testing.T) {
	cfg := NewControllerConfig()
	newSize := 75
	cfg.WithBatchSize(newSize)

	if cfg.BatchSize != newSize {
		t.Errorf("Expected BatchSize=%d, got %d", newSize, cfg.BatchSize)
	}
}

func TestControllerConfig_WithMaxConcurrentEvaluations(t *testing.T) {
	cfg := NewControllerConfig()
	maxConcurrent := 10
	cfg.WithMaxConcurrentEvaluations(maxConcurrent)

	if cfg.MaxConcurrentEvaluations != maxConcurrent {
		t.Errorf("Expected MaxConcurrentEvaluations=%d, got %d", maxConcurrent, cfg.MaxConcurrentEvaluations)
	}
}
