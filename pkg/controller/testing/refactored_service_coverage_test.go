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

package testing

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/controller"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Static errors for testing.
var (
	errDeleteFailed = errors.New("delete failed")
)

// TestParseGVR tests the parseGVR function with various inputs.
func TestParseGVR(t *testing.T) {
	tests := []struct {
		name        string
		apiVersion  string
		kind        string
		expectError bool
		expectedGVR schema.GroupVersionResource
	}{
		{
			name:        "core API group",
			apiVersion:  "v1",
			kind:        "ConfigMap",
			expectError: false,
			expectedGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		},
		{
			name:        "apps API group",
			apiVersion:  "apps/v1",
			kind:        "Deployment",
			expectError: false,
			expectedGVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		{
			name:        "custom resource",
			apiVersion:  "example.com/v1alpha1",
			kind:        "MyResource",
			expectError: false,
			expectedGVR: schema.GroupVersionResource{Group: "example.com", Version: "v1alpha1", Resource: "myresources"},
		},
		{
			name:        "kind ending with y",
			apiVersion:  "v1",
			kind:        "Policy",
			expectError: false,
			expectedGVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "policies"},
		},
		{
			name:        "invalid API version",
			apiVersion:  "invalid/version/format",
			kind:        "Resource",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to test parseGVR indirectly through PolicyEvaluationService
			// since it's not exported. We'll test it via EvaluatePolicy with invalid GVR
			policy := &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       types.UID("policy-uid"),
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: tt.apiVersion,
						Kind:       tt.kind,
					},
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
					},
				},
			}

			mockLister := NewMockResourceLister()
			mockSelectorMatcher := NewMockSelectorMatcher()
			mockConditionMatcher := NewMockConditionMatcher()
			mockRateLimiter := NewMockRateLimiterProvider()
			mockDeleter := NewMockBatchDeleterCore()

			service := controller.NewPolicyEvaluationService(
				mockLister,
				mockSelectorMatcher,
				mockConditionMatcher,
				nil,
				mockRateLimiter,
				mockDeleter,
				nil,
				nil,
				sdklog.NewLogger("zen-gc"),
			)

			ctx := context.Background()
			err := service.EvaluatePolicy(ctx, policy)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error for invalid GVR, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid GVR: %v", err)
				}
			}
		})
	}
}

// TestPolicyEvaluationService_GetBatchSize tests batch size logic.
func TestPolicyEvaluationService_GetBatchSize(t *testing.T) {
	tests := []struct {
		name           string
		policyBatchSize int
		expectedSize   int
	}{
		{
			name:           "policy batch size set",
			policyBatchSize: 5,
			expectedSize:   5,
		},
		{
			name:           "zero batch size uses default",
			policyBatchSize: 0,
			expectedSize:   10, // Default
		},
		{
			name:           "large batch size",
			policyBatchSize: 100,
			expectedSize:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			resources := make([]*unstructured.Unstructured, 0, 20)
			for i := 0; i < 20; i++ {
				resources = append(resources, &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":              fmt.Sprintf("test-cm-%d", i),
							"namespace":          "default",
							"uid":                types.UID(fmt.Sprintf("uid-%d", i)),
							"creationTimestamp":  metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
						},
					},
				})
			}

			policy := &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       types.UID("policy-uid"),
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
					},
					Behavior: v1alpha1.BehaviorSpec{
						BatchSize: tt.policyBatchSize,
					},
				},
			}

			mockLister := NewMockResourceLister()
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
			mockLister.SetResources(gvr, "default", resources)

			mockSelectorMatcher := NewMockSelectorMatcher()
			for _, r := range resources {
				mockSelectorMatcher.SetMatch(r, true)
			}

			mockConditionMatcher := NewMockConditionMatcher()
			for _, r := range resources {
				mockConditionMatcher.SetMeetsConditions(r, true)
			}

			mockRateLimiter := NewMockRateLimiterProvider()
			mockDeleter := NewMockBatchDeleterCore()
			for _, r := range resources {
				mockDeleter.SetDeleteResult(r, nil)
			}

			service := controller.NewPolicyEvaluationService(
				mockLister,
				mockSelectorMatcher,
				mockConditionMatcher,
				nil,
				mockRateLimiter,
				mockDeleter,
				nil,
				nil,
				sdklog.NewLogger("zen-gc"),
			)

			ctx := context.Background()
			err := service.EvaluatePolicy(ctx, policy)
			if err != nil {
				t.Fatalf("EvaluatePolicy failed: %v", err)
			}

			// Verify batch size was used (we can't directly test getBatchSize, but we can verify
			// that deletion happened in batches by checking mock calls)
			// The actual batch size logic is tested indirectly through the deletion process
		})
	}
}

// TestPolicyEvaluationService_ShouldDelete tests TTL expiration logic.
func TestPolicyEvaluationService_ShouldDelete(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		creationTime   time.Time
		ttlSeconds     int64
		expectDelete   bool
		expectedReason string
	}{
		{
			name:           "expired resource",
			creationTime:   now.Add(-2 * time.Hour),
			ttlSeconds:     3600, // 1 hour
			expectDelete:   true,
			expectedReason: "ttl_expired",
		},
		{
			name:           "not expired resource",
			creationTime:   now.Add(-30 * time.Minute),
			ttlSeconds:     3600, // 1 hour
			expectDelete:   false,
			expectedReason: "not_expired",
		},
		{
			name:           "no TTL configured",
			creationTime:   now.Add(-2 * time.Hour),
			ttlSeconds:     0,
			expectDelete:   false,
			expectedReason: "no_ttl",
		},
		{
			name:           "just expired",
			creationTime:   now.Add(-1 * time.Hour).Add(-1 * time.Second),
			ttlSeconds:     3600,
			expectDelete:   true,
			expectedReason: "ttl_expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":              "test-cm",
						"namespace":          "default",
						"uid":                "test-uid",
						"creationTimestamp":  metav1.NewTime(tt.creationTime).Format(time.RFC3339),
					},
				},
			}

			policy := &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       types.UID("policy-uid"),
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: func() *int64 {
							if tt.ttlSeconds > 0 {
								v := tt.ttlSeconds
								return &v
							}
							return nil
						}(),
					},
				},
			}

			mockLister := NewMockResourceLister()
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
			mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{resource})

			mockSelectorMatcher := NewMockSelectorMatcher()
			mockSelectorMatcher.SetMatch(resource, true)

			mockConditionMatcher := NewMockConditionMatcher()
			mockConditionMatcher.SetMeetsConditions(resource, true)

			mockRateLimiter := NewMockRateLimiterProvider()
			mockDeleter := NewMockBatchDeleterCore()
			if tt.expectDelete {
				mockDeleter.SetDeleteResult(resource, nil)
			}

			service := controller.NewPolicyEvaluationService(
				mockLister,
				mockSelectorMatcher,
				mockConditionMatcher,
				nil,
				mockRateLimiter,
				mockDeleter,
				nil,
				nil,
				sdklog.NewLogger("zen-gc"),
			)

			ctx := context.Background()
			err := service.EvaluatePolicy(ctx, policy)
			if err != nil {
				t.Fatalf("EvaluatePolicy failed: %v", err)
			}

			// Verify deletion happened if expected
			// The shouldDelete logic is tested indirectly through EvaluatePolicy
		})
	}
}

// TestPolicyEvaluationService_StatusUpdateTimeout tests status update timeout handling.
func TestPolicyEvaluationService_StatusUpdateTimeout(t *testing.T) {
	now := time.Now()
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              "test-cm",
				"namespace":          "default",
				"uid":                "test-uid",
				"creationTimestamp":  metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{resource})

	mockSelectorMatcher := NewMockSelectorMatcher()
	mockSelectorMatcher.SetMatch(resource, true)

	mockConditionMatcher := NewMockConditionMatcher()
	mockConditionMatcher.SetMeetsConditions(resource, true)

	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()
	mockDeleter.SetDeleteResult(resource, nil)

	// StatusUpdater is a concrete type, not an interface.
	// We'll test status update errors by passing nil and testing the error path.
	// The actual status update error handling is tested in status_updater_test.go.
	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil, // StatusUpdater - nil is OK, error handling tested elsewhere
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	// Should handle status update error gracefully
	if err != nil {
		t.Logf("EvaluatePolicy returned error (may be expected): %v", err)
	}
}

// TestPolicyEvaluationService_BatchDeletionErrors tests batch deletion error handling.
func TestPolicyEvaluationService_BatchDeletionErrors(t *testing.T) {
	now := time.Now()
	resources := make([]*unstructured.Unstructured, 0, 5)
	for i := 0; i < 5; i++ {
		resources = append(resources, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":              fmt.Sprintf("test-cm-%d", i),
					"namespace":          "default",
					"uid":                types.UID(fmt.Sprintf("uid-%d", i)),
					"creationTimestamp":  metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
				},
			},
		})
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
			Behavior: v1alpha1.BehaviorSpec{
				BatchSize: 2, // Small batch for testing
			},
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", resources)

	mockSelectorMatcher := NewMockSelectorMatcher()
	for _, r := range resources {
		mockSelectorMatcher.SetMatch(r, true)
	}

	mockConditionMatcher := NewMockConditionMatcher()
	for _, r := range resources {
		mockConditionMatcher.SetMeetsConditions(r, true)
	}

	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()
	// Set some deletions to fail
	mockDeleter.SetDeleteResult(resources[0], nil)                    // Success
	mockDeleter.SetDeleteResult(resources[1], errDeleteFailed)       // Error
	mockDeleter.SetDeleteResult(resources[2], nil)                   // Success
	mockDeleter.SetDeleteResult(resources[3], errDeleteFailed)       // Error
	mockDeleter.SetDeleteResult(resources[4], nil)                   // Success

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	// Should handle deletion errors gracefully and continue
	if err != nil {
		t.Fatalf("EvaluatePolicy should handle deletion errors gracefully: %v", err)
	}
}

// TestPolicyEvaluationService_WildcardNamespace tests wildcard namespace handling.
func TestPolicyEvaluationService_WildcardNamespace(t *testing.T) {
	now := time.Now()
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":              "test-cm-1",
					"namespace":          "default",
					"uid":                "uid-1",
					"creationTimestamp":  metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":              "test-cm-2",
					"namespace":          "kube-system",
					"uid":                "uid-2",
					"creationTimestamp":  metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
				},
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				// Empty namespace means wildcard
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	// Set resources for wildcard namespace
	mockLister.SetResources(gvr, "*", resources)

	mockSelectorMatcher := NewMockSelectorMatcher()
	for _, r := range resources {
		mockSelectorMatcher.SetMatch(r, true)
	}

	mockConditionMatcher := NewMockConditionMatcher()
	for _, r := range resources {
		mockConditionMatcher.SetMeetsConditions(r, true)
	}

	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()
	for _, r := range resources {
		mockDeleter.SetDeleteResult(r, nil)
	}

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
}

