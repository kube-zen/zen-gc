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

package errors

import (
	"errors"
	"testing"
)

var (
	testErrorType = "test_error"
	errUnderlying = errors.New("underlying error")
	testNS        = "test-ns"
)

func TestGCError_Error(t *testing.T) {
	tests := []struct {
		name    string
		gcErr   *GCError
		wantErr string
	}{
		{
			name: "error with message only",
			gcErr: &GCError{
				Type:    testErrorType,
				Message: "test message",
			},
			wantErr: "test message",
		},
		{
			name: "error with underlying error",
			gcErr: &GCError{
				Type:    testErrorType,
				Message: "test message",
				Err:     errUnderlying,
			},
			wantErr: "test message: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gcErr.Error(); got != tt.wantErr {
				t.Errorf("GCError.Error() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}

func TestGCError_Unwrap(t *testing.T) {
	gcErr := &GCError{
		Type:    testErrorType,
		Message: "test message",
		Err:     errUnderlying,
	}

	if got := gcErr.Unwrap(); !errors.Is(got, errUnderlying) {
		t.Errorf("GCError.Unwrap() = %v, want %v", got, errUnderlying)
	}
}

func TestWithPolicy(t *testing.T) {
	gcErr := WithPolicy(errUnderlying, testNS, "test-policy")

	if gcErr.GetContext("policy_namespace") != testNS {
		t.Errorf("Expected policy_namespace=%s, got %s", testNS, gcErr.GetContext("policy_namespace"))
	}
	if gcErr.GetContext("policy_name") != "test-policy" {
		t.Errorf("Expected policy_name=test-policy, got %s", gcErr.GetContext("policy_name"))
	}
}

func TestWithPolicy_AlreadyGCError(t *testing.T) {
	existingGCErr := New(testErrorType, "existing error")
	gcErr := WithPolicy(existingGCErr, testNS, "test-policy")

	if gcErr.GetContext("policy_namespace") != testNS {
		t.Errorf("Expected policy_namespace=%s, got %s", testNS, gcErr.GetContext("policy_namespace"))
	}
	if gcErr.GetContext("policy_name") != "test-policy" {
		t.Errorf("Expected policy_name=test-policy, got %s", gcErr.GetContext("policy_name"))
	}
}

func TestWithResource(t *testing.T) {
	gcErr := WithResource(errUnderlying, testNS, "test-resource")

	if gcErr.GetContext("resource_namespace") != testNS {
		t.Errorf("Expected resource_namespace=%s, got %s", testNS, gcErr.GetContext("resource_namespace"))
	}
	if gcErr.GetContext("resource_name") != "test-resource" {
		t.Errorf("Expected resource_name=test-resource, got %s", gcErr.GetContext("resource_name"))
	}
}

func TestWithResource_AlreadyGCError(t *testing.T) {
	existingGCErr := New(testErrorType, "existing error")
	gcErr := WithResource(existingGCErr, testNS, "test-resource")

	if gcErr.GetContext("resource_namespace") != testNS {
		t.Errorf("Expected resource_namespace=%s, got %s", testNS, gcErr.GetContext("resource_namespace"))
	}
	if gcErr.GetContext("resource_name") != "test-resource" {
		t.Errorf("Expected resource_name=test-resource, got %s", gcErr.GetContext("resource_name"))
	}
}

func TestNew(t *testing.T) {
	gcErr := New(testErrorType, "test message")

	if gcErr.Type != testErrorType {
		t.Errorf("Expected Type=%s, got %s", testErrorType, gcErr.Type)
	}
	if gcErr.Message != "test message" {
		t.Errorf("Expected Message=test message, got %s", gcErr.Message)
	}
}

func TestWrap(t *testing.T) {
	gcErr := Wrap(errUnderlying, testErrorType, "test message")

	if gcErr.Type != testErrorType {
		t.Errorf("Expected Type=%s, got %s", testErrorType, gcErr.Type)
	}
	if gcErr.Message != "test message" {
		t.Errorf("Expected Message=test message, got %s", gcErr.Message)
	}
	if !errors.Is(gcErr.Err, errUnderlying) {
		t.Errorf("Expected Err to wrap underlying error, got %v", gcErr.Err)
	}
}

func TestWrapf(t *testing.T) {
	gcErr := Wrapf(errUnderlying, testErrorType, "test message: %s", "formatted")

	if gcErr.Type != testErrorType {
		t.Errorf("Expected Type=%s, got %s", testErrorType, gcErr.Type)
	}
	if gcErr.Message != "test message: formatted" {
		t.Errorf("Expected Message=test message: formatted, got %s", gcErr.Message)
	}
	if !errors.Is(gcErr.Err, errUnderlying) {
		t.Errorf("Expected Err to wrap underlying error, got %v", gcErr.Err)
	}
}
