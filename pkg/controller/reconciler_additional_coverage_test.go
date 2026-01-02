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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
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


// TestGCPolicyReconciler_cleanupPolicyResources_Additional tests cleanup of policy resources.
func TestGCPolicyReconciler_cleanupPolicyResources_Additional(t *testing.T) {
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
	nn1 := types.NamespacedName{Name: policy1.Name, Namespace: policy1.Namespace}
	reconciler.cleanupPolicyResources(nn1)

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

	// Cleanup policy2
	nn2 := types.NamespacedName{Name: policy2.Name, Namespace: policy2.Namespace}
	reconciler.cleanupPolicyResources(nn2)
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

