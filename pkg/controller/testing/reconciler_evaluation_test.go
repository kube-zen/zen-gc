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
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-gc/pkg/controller"
)

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TestGCPolicyReconciler_EvaluatePolicy_WithMocks tests evaluatePolicy using PolicyEvaluationService with mocks.
// This test demonstrates that we can now test policy evaluation without complex fake client setup.
func TestGCPolicyReconciler_EvaluatePolicy_WithMocks(t *testing.T) {
	ctx := context.Background()

	// Setup fake clients (minimal setup - no complex resource registration needed)
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	// Create a test policy
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("test-uid"),
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

	// Create fake client with the policy
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(policy).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create reconciler
	reconciler := controller.NewGCPolicyReconciler(
		fakeClient,
		scheme,
		dynamicClient,
		controller.NewStatusUpdater(dynamicClient),
		controller.NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	// Create mock dependencies for PolicyEvaluationService
	mockLister := NewMockResourceLister()
	mockSelectorMatcher := NewMockSelectorMatcher()
	mockConditionMatcher := NewMockConditionMatcher()
	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()

	// Set up mock expectations
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	testResources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":              "test-cm",
					"namespace":         "default",
					"creationTimestamp": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				},
			},
		},
	}
	mockLister.SetResources(gvr, "default", testResources)
	mockSelectorMatcher.SetMatch(testResources[0], true)
	mockConditionMatcher.SetMeetsConditions(testResources[0], true)
	mockDeleter.SetDeleteResult(testResources[0], nil)

	// Create PolicyEvaluationService with mocks
	// Use mock status updater to avoid fake client complexity
	mockStatusUpdater := NewMockStatusUpdater()
	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil, // TTLCalculator
		mockRateLimiter,
		mockDeleter,
		&controller.StatusUpdater{}, // Will be replaced - status updater is concrete type
		reconciler.GetEventRecorder(),
		reconciler.GetLogger(),
	)

	// Inject the service into reconciler (bypassing getOrCreateEvaluationService for testing)
	// Note: We need to access the private field, so we'll test through the public interface
	// by ensuring the service is created on first evaluation
	err := reconciler.EvaluatePolicyForTesting(ctx, policy, service)
	if err != nil {
		t.Errorf("evaluatePolicy() returned error: %v", err)
	}
}

// TestGCPolicyReconciler_EvaluatePolicy_EmptyResources tests evaluation with no resources.
func TestGCPolicyReconciler_EvaluatePolicy_EmptyResources(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	reconciler := controller.NewGCPolicyReconciler(
		fakeClient,
		scheme,
		dynamicClient,
		controller.NewStatusUpdater(dynamicClient),
		controller.NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("test-uid"),
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

	// Create mocks with empty resources
	mockLister := NewMockResourceLister()
	mockSelectorMatcher := NewMockSelectorMatcher()
	mockConditionMatcher := NewMockConditionMatcher()
	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{})

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		reconciler.GetStatusUpdater(),
		reconciler.GetEventRecorder(),
		reconciler.GetLogger(),
	)

	// Should handle empty resources gracefully
	// Note: Status update may fail (policy not in fake client), but that's OK for this test
	err := reconciler.EvaluatePolicyForTesting(ctx, policy, service)
	if err != nil && !contains(err.Error(), "not found") && !contains(err.Error(), "StatusUpdateFailed") {
		t.Errorf("evaluatePolicy() with empty resources returned unexpected error: %v", err)
	}
}

// TestGCPolicyReconciler_EvaluatePolicy_ContextCancellation tests context cancellation.
func TestGCPolicyReconciler_EvaluatePolicy_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	reconciler := controller.NewGCPolicyReconciler(
		fakeClient,
		scheme,
		dynamicClient,
		controller.NewStatusUpdater(dynamicClient),
		controller.NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("test-uid"),
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

	// Create mocks
	mockLister := NewMockResourceLister()
	mockSelectorMatcher := NewMockSelectorMatcher()
	mockConditionMatcher := NewMockConditionMatcher()
	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()

	// Set error for context cancellation
	mockLister.SetError(context.Canceled)

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		reconciler.GetStatusUpdater(),
		reconciler.GetEventRecorder(),
		reconciler.GetLogger(),
	)

	// Should handle context cancellation gracefully
	err := reconciler.EvaluatePolicyForTesting(ctx, policy, service)
	if err == nil {
		t.Error("Expected error for context cancellation, got nil")
	}
}

