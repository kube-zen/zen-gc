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

// TestPolicyEvaluationService_EvaluatePolicy demonstrates how much simpler testing becomes
// with the refactored version using interfaces and mocks.
func TestPolicyEvaluationService_EvaluatePolicy(t *testing.T) {
	// Create test resources
	now := time.Now()
	expiredResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              "expired-cm",
				"namespace":          "default",
				"uid":                "expired-uid",
				"creationTimestamp":  metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
			},
		},
	}

	validResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              "valid-cm",
				"namespace":          "default",
				"uid":                "valid-uid",
				"creationTimestamp":  metav1.NewTime(now.Add(-30 * time.Minute)).Format(time.RFC3339),
			},
		},
	}

	// Create test policy
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
				Namespace:  "default",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(), // 1 hour
			},
		},
	}

	// Create mocks - this is MUCH simpler than setting up real Kubernetes clients!
	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{expiredResource, validResource})

	mockSelectorMatcher := NewMockSelectorMatcher()
	mockSelectorMatcher.SetMatch(expiredResource, true)
	mockSelectorMatcher.SetMatch(validResource, true)

	mockConditionMatcher := NewMockConditionMatcher()
	mockConditionMatcher.SetMeetsConditions(expiredResource, true)
	mockConditionMatcher.SetMeetsConditions(validResource, true)

	mockRateLimiter := NewMockRateLimiterProvider()

	mockDeleter := NewMockBatchDeleterCore()
	mockDeleter.SetDeleteResult(expiredResource, nil) // Success

	// Create service with mocks
	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil, // TTLCalculator (using shared function for now)
		mockRateLimiter,
		mockDeleter,
		nil, // StatusUpdater (not needed for this test)
		nil, // EventRecorder (not needed for this test)
		sdklog.NewLogger("zen-gc"),
	)

	// Evaluate policy - this is now testable without complex setup!
	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
}

// TestPolicyEvaluationService_EvaluatePolicy_EmptyResources tests with no resources.
func TestPolicyEvaluationService_EvaluatePolicy_EmptyResources(t *testing.T) {
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
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
}

// TestPolicyEvaluationService_EvaluatePolicy_ContextCanceled tests context cancellation.
func TestPolicyEvaluationService_EvaluatePolicy_ContextCanceled(t *testing.T) {
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

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy should handle canceled context gracefully: %v", err)
	}
}

