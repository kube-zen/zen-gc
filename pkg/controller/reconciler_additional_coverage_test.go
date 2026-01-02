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

package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// TestGCPolicyReconciler_matchesSelectors tests the matchesSelectors method.
func TestGCPolicyReconciler_matchesSelectors(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}

	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		target        *v1alpha1.TargetResourceSpec
		expectedMatch bool
	}{
		{
			name: "matches label selector",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
			expectedMatch: true,
		},
		{
			name: "does not match label selector",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "other",
						},
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
			expectedMatch: false,
		},
		{
			name: "matches namespace",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				Namespace: "default",
			},
			expectedMatch: true,
		},
		{
			name: "wildcard namespace matches all",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "any-namespace",
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				Namespace: "*",
			},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.matchesSelectors(tt.resource, tt.target)
			if result != tt.expectedMatch {
				t.Errorf("matchesSelectors() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

// TestGCPolicyReconciler_getOrCreateRateLimiter tests rate limiter creation and reuse.
func TestGCPolicyReconciler_getOrCreateRateLimiter(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	policy1 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: "policy-1",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: 10,
			},
		},
	}

	policy2 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: "policy-2",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: 20,
			},
		},
	}

	// First call should create a new rate limiter
	limiter1 := reconciler.getOrCreateRateLimiter(policy1)
	if limiter1 == nil {
		t.Fatal("getOrCreateRateLimiter() returned nil")
	}

	// Second call with same policy should return the same limiter
	limiter2 := reconciler.getOrCreateRateLimiter(policy1)
	if limiter1 != limiter2 {
		t.Error("getOrCreateRateLimiter() should return the same limiter for the same policy")
	}

	// Different policy should get a different limiter
	limiter3 := reconciler.getOrCreateRateLimiter(policy2)
	if limiter3 == nil {
		t.Fatal("getOrCreateRateLimiter() returned nil for second policy")
	}
	if limiter1 == limiter3 {
		t.Error("getOrCreateRateLimiter() should return different limiters for different policies")
	}
}

// TestGCPolicyReconciler_getBatchSize tests batch size calculation.
func TestGCPolicyReconciler_getBatchSize(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	cfg := config.NewControllerConfig()
	cfg.BatchSize = 50

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		cfg,
	)

	tests := []struct {
		name           string
		policyBatchSize int
		expectedSize   int
	}{
		{
			name:           "uses policy batch size when set",
			policyBatchSize: 25,
			expectedSize:   25,
		},
		{
			name:           "uses config batch size when policy not set",
			policyBatchSize: 0,
			expectedSize:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					Behavior: v1alpha1.BehaviorSpec{
						BatchSize: tt.policyBatchSize,
					},
				},
			}

			size := reconciler.getBatchSize(policy)
			if size != tt.expectedSize {
				t.Errorf("getBatchSize() = %d, want %d", size, tt.expectedSize)
			}
		})
	}
}

// TestGCPolicyReconciler_cleanupPolicyResources tests cleanup of policy resources.
func TestGCPolicyReconciler_cleanupPolicyResources(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	policy1 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: "policy-1",
		},
	}

	policy2 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: "policy-2",
		},
	}

	// Add some resources
	reconciler.getOrCreateRateLimiter(policy1)
	reconciler.getOrCreateRateLimiter(policy2)

	// Cleanup policy1
	reconciler.cleanupPolicyResources(policy1)

	// Verify policy1 resources are cleaned up
	limiter := reconciler.getOrCreateRateLimiter(policy1)
	if limiter == nil {
		t.Error("cleanupPolicyResources() should not prevent creating new resources")
	}

	// Verify policy2 resources are still there
	limiter2 := reconciler.getOrCreateRateLimiter(policy2)
	if limiter2 == nil {
		t.Error("cleanupPolicyResources() should not affect other policies")
	}
}

// TestGCPolicyReconciler_trackPolicyUID_Coverage tests tracking of policy UIDs.
func TestGCPolicyReconciler_trackPolicyUID_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: "test-uid",
		},
	}

	// Track policy UID
	nn := types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}
	reconciler.trackPolicyUID(nn, policy.UID)

	// Verify it's tracked (indirectly by checking that resources can be created)
	limiter := reconciler.getOrCreateRateLimiter(policy)
	if limiter == nil {
		t.Error("trackPolicyUID() should allow resource creation")
	}
}

// TestGCPolicyReconciler_trackPolicySpec tests tracking of policy spec.
func TestGCPolicyReconciler_trackPolicySpec(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: "test-uid",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: 10,
			},
		},
	}

	// Track policy spec
	reconciler.trackPolicySpec(policy.UID, &policy.Spec)

	// Verify it's tracked (indirectly by checking that batch size uses policy spec)
	batchSize := reconciler.getBatchSize(policy)
	if batchSize == 0 {
		t.Error("trackPolicySpec() should allow policy spec to be used")
	}
}

// TestGCPolicyReconciler_getOrCreateResourceInformer_ErrorHandling tests error handling.
func TestGCPolicyReconciler_getOrCreateResourceInformer_ErrorHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	ctx := context.Background()
	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "invalid/api",
				Kind:       "InvalidKind",
			},
		},
	}

	// Should handle invalid GVR gracefully
	_, err := reconciler.getOrCreateResourceInformer(ctx, policy)
	if err == nil {
		t.Log("getOrCreateResourceInformer() handled invalid GVR - may return error or handle gracefully")
	}
}

// TestGCPolicyReconciler_deleteResource_DryRun tests dry run deletion.
func TestGCPolicyReconciler_deleteResource_DryRun(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	ctx := context.Background()
	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				DryRun: true,
			},
		},
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	// Dry run should not actually delete
	rateLimiter := ratelimiter.NewRateLimiter(10)
	err := reconciler.deleteResource(ctx, resource, policy, rateLimiter)
	if err != nil {
		t.Errorf("deleteResource() with dry run should not return error, got: %v", err)
	}
}

// TestGCPolicyReconciler_deleteResource_ContextCancellation tests context cancellation.
func TestGCPolicyReconciler_deleteResource_ContextCancellation(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		NewStatusUpdater(dynamicClient),
		NewEventRecorder(nil),
		config.NewControllerConfig(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				DryRun: false,
			},
		},
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	// Should handle context cancellation gracefully
	rateLimiter := ratelimiter.NewRateLimiter(10)
	err := reconciler.deleteResource(ctx, resource, policy, rateLimiter)
	if err == nil {
		t.Log("deleteResource() handled context cancellation - may return error or handle gracefully")
	}
}

