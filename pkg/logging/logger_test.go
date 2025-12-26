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

package logging

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
)

const testCorrelationID = "test-correlation-id"

var errRegularError = errors.New("regular error")

var errTest = errors.New("test error")

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	if logger.fields == nil {
		t.Fatal("Logger fields map is nil")
	}
}

func TestLogger_WithField(t *testing.T) {
	logger := NewLogger().WithField("key", "value")
	if logger.fields["key"] != "value" {
		t.Errorf("Expected field 'key' to be 'value', got %v", logger.fields["key"])
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger := NewLogger().WithFields(
		Field{Key: "key1", Value: "value1"},
		Field{Key: "key2", Value: "value2"},
	)
	if logger.fields["key1"] != "value1" {
		t.Errorf("Expected field 'key1' to be 'value1', got %v", logger.fields["key1"])
	}
	if logger.fields["key2"] != "value2" {
		t.Errorf("Expected field 'key2' to be 'value2', got %v", logger.fields["key2"])
	}
}

func TestLogger_WithPolicy(t *testing.T) {
	logger := NewLogger().WithPolicy("default", "test-policy")
	if logger.fields["policy_namespace"] != "default" {
		t.Errorf("Expected policy_namespace to be 'default', got %v", logger.fields["policy_namespace"])
	}
	if logger.fields["policy_name"] != "test-policy" {
		t.Errorf("Expected policy_name to be 'test-policy', got %v", logger.fields["policy_name"])
	}
}

func TestLogger_WithResource(t *testing.T) {
	logger := NewLogger().WithResource("default", "test-resource", "v1", "ConfigMap")
	if logger.fields["resource_namespace"] != "default" {
		t.Errorf("Expected resource_namespace to be 'default', got %v", logger.fields["resource_namespace"])
	}
	if logger.fields["resource_name"] != "test-resource" {
		t.Errorf("Expected resource_name to be 'test-resource', got %v", logger.fields["resource_name"])
	}
	if logger.fields["resource_api_version"] != "v1" {
		t.Errorf("Expected resource_api_version to be 'v1', got %v", logger.fields["resource_api_version"])
	}
	if logger.fields["resource_kind"] != "ConfigMap" {
		t.Errorf("Expected resource_kind to be 'ConfigMap', got %v", logger.fields["resource_kind"])
	}
}

func TestLogger_WithCorrelationID(t *testing.T) {
	logger := NewLogger().WithCorrelationID(testCorrelationID)
	if logger.fields["correlation_id"] != testCorrelationID {
		t.Errorf("Expected correlation_id to be '%s', got %v", testCorrelationID, logger.fields["correlation_id"])
	}
}

func TestLogger_WithError(t *testing.T) {
	logger := NewLogger().WithError(errTest)
	if logger.fields["error"] != errTest.Error() {
		t.Errorf("Expected error field to contain error message, got %v", logger.fields["error"])
	}
	if logger.fields["error_type"] == "" {
		t.Error("Expected error_type field to be set")
	}
}

func TestLogger_WithError_Nil(t *testing.T) {
	logger := NewLogger().WithError(nil)
	// Should not panic and should not add error fields
	if _, ok := logger.fields["error"]; ok {
		t.Error("Expected no error field when error is nil")
	}
}

func TestLogger_Chaining(t *testing.T) {
	logger := NewLogger().
		WithPolicy("default", "test-policy").
		WithResource("default", "test-resource", "v1", "ConfigMap").
		WithCorrelationID("test-id")

	if logger.fields["policy_name"] != "test-policy" {
		t.Error("Policy field not set correctly")
	}
	if logger.fields["resource_name"] != "test-resource" {
		t.Error("Resource field not set correctly")
	}
	if logger.fields["correlation_id"] != "test-id" {
		t.Error("Correlation ID not set correctly")
	}
}

func TestFromContext(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), testCorrelationID)
	logger := FromContext(ctx)
	if logger.fields["correlation_id"] != testCorrelationID {
		t.Errorf("Expected correlation_id from context to be '%s', got %v", testCorrelationID, logger.fields["correlation_id"])
	}
}

func TestFromContext_NoCorrelationID(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	if _, ok := logger.fields["correlation_id"]; ok {
		t.Error("Expected no correlation_id when not set in context")
	}
}

func TestWithCorrelationID(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), testCorrelationID)
	retrieved := GetCorrelationID(ctx)
	if retrieved != testCorrelationID {
		t.Errorf("Expected correlation ID '%s', got '%s'", testCorrelationID, retrieved)
	}
}

func TestGetCorrelationID_NotSet(t *testing.T) {
	ctx := context.Background()
	id := GetCorrelationID(ctx)
	if id != "" {
		t.Errorf("Expected empty correlation ID, got '%s'", id)
	}
}

func TestGetCorrelationID_NilContext(t *testing.T) {
	// Test with context.TODO() instead of nil to avoid SA1012
	ctx := context.TODO()
	id := GetCorrelationID(ctx)
	// context.TODO() doesn't have a correlation ID, so it should be empty
	if id != "" {
		t.Errorf("Expected empty correlation ID for context.TODO(), got '%s'", id)
	}
}

func TestGenerateCorrelationID(t *testing.T) {
	id1 := GenerateCorrelationID()
	id2 := GenerateCorrelationID()
	if id1 == id2 {
		t.Error("Generated correlation IDs should be unique")
	}
	if id1 == "" {
		t.Error("Generated correlation ID should not be empty")
	}
}

func TestVerboseLogger(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	verboseLogger := logger.V(2)
	if verboseLogger.logger != logger {
		t.Error("VerboseLogger should reference the original logger")
	}
	if verboseLogger.level != 2 {
		t.Errorf("Expected verbosity level 2, got %d", verboseLogger.level)
	}
}

func TestLogger_WithDuration(t *testing.T) {
	logger := NewLogger().WithDuration(1500 * time.Millisecond)
	if logger.fields["duration_ms"] != int64(1500) {
		t.Errorf("Expected duration_ms to be 1500, got %v", logger.fields["duration_ms"])
	}
}

func TestLogger_Info(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Should not panic
	logger.Info("test message")
}

func TestLogger_Infof(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Should not panic
	logger.Infof("test message: %s", "value")
}

func TestLogger_Error(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Should not panic
	logger.Error("test error message")
}

func TestLogger_Errorf(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Should not panic
	logger.Errorf("test error message: %s", "value")
}

func TestLogger_Warning(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Should not panic
	logger.Warning("test warning message")
}

func TestLogger_Warningf(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Should not panic
	logger.Warningf("test warning message: %s", "value")
}

func TestLogger_fieldsToKlogArgs(t *testing.T) {
	logger := NewLogger().
		WithField("key1", "value1").
		WithField("key2", "value2")
	args := logger.fieldsToKlogArgs()
	// Should have 4 elements (2 keys + 2 values)
	if len(args) != 4 {
		t.Errorf("Expected 4 args, got %d", len(args))
	}
	// Check that args are key-value pairs
	argMap := make(map[string]interface{})
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			argMap[args[i].(string)] = args[i+1]
		}
	}
	if argMap["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %v", argMap["key1"])
	}
	if argMap["key2"] != "value2" {
		t.Errorf("Expected key2=value2, got %v", argMap["key2"])
	}
}

func TestVerboseLogger_Info(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	verboseLogger := logger.V(1)
	// Should not panic
	verboseLogger.Info("test message")
}

func TestVerboseLogger_Infof(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	verboseLogger := logger.V(1)
	// Should not panic
	verboseLogger.Infof("test message: %s", "value")
}

func TestSafeKlogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int // Expected clamped value
	}{
		{"negative", -5, 0},
		{"zero", 0, 0},
		{"valid", 5, 5},
		{"max", 10, 10},
		{"over_max", 15, 10},
		{"very_large", 100, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeKlogLevel(tt.input)
			if int(result) != tt.expected {
				t.Errorf("safeKlogLevel(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetErrorType_GCError(t *testing.T) {
	gcErr := &gcerrors.GCError{
		Type: "test_error_type",
	}
	result := getErrorType(gcErr)
	if result != "test_error_type" {
		t.Errorf("Expected 'test_error_type', got '%s'", result)
	}
}

func TestGetErrorType_WrappedGCError(t *testing.T) {
	gcErr := &gcerrors.GCError{
		Type: "wrapped_error_type",
	}
	wrappedErr := fmt.Errorf("wrapped: %w", gcErr)
	result := getErrorType(wrappedErr)
	if result != "wrapped_error_type" {
		t.Errorf("Expected 'wrapped_error_type', got '%s'", result)
	}
}

func TestGetErrorType_RegularError(t *testing.T) {
	regularErr := fmt.Errorf("wrapped: %w", errRegularError)
	result := getErrorType(regularErr)
	if result != "unknown" {
		t.Errorf("Expected 'unknown', got '%s'", result)
	}
}

func TestGetErrorType_Nil(t *testing.T) {
	result := getErrorType(nil)
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}
