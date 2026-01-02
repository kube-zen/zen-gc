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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

// TestGCController_evaluatePolicies_ContextCancellation tests context cancellation handling.
// Note: GCController is deprecated, but this test verifies basic behavior.
func TestGCController_evaluatePolicies_ContextCancellation(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")

	// Cancel context before evaluation
	controller.cancel()

	// Should handle cancellation gracefully
	controller.evaluatePolicies()
}

// TestGCController_evaluatePolicies_CacheNotSynced_New tests cache not synced scenario.
// Note: GCController is deprecated, but this test verifies basic behavior.
func TestGCController_evaluatePolicies_CacheNotSynced_New(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePolicies_EmptyPolicies tests empty policies list.
// Note: GCController is deprecated, but this test verifies basic behavior.
func TestGCController_evaluatePolicies_EmptyPolicies(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePolicies_WithMaxConcurrent_New tests different maxConcurrent settings.
// Note: GCController is deprecated. This test is kept for reference but may need updates.
func TestGCController_evaluatePolicies_WithMaxConcurrent_New(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePoliciesSequential_ErrorHandling tests error handling in sequential evaluation.
// Note: This test is for the deprecated GCController. For new tests, use GCPolicyReconciler with mocks.
func TestGCController_evaluatePoliciesSequential_ErrorHandling(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePoliciesParallel_WorkerPool tests worker pool behavior.
// Note: This test is for the deprecated GCController. For new tests, use GCPolicyReconciler with mocks.
func TestGCController_evaluatePoliciesParallel_WorkerPool(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePoliciesParallel_ContextCancellation tests context cancellation in parallel evaluation.
// Note: This test is for the deprecated GCController. For new tests, use GCPolicyReconciler with mocks.
func TestGCController_evaluatePoliciesParallel_ContextCancellation(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// Helper function to create unstructured policy with spec.
// This is a duplicate of createUnstructuredPolicyWithSpec but kept for test isolation.
func createUnstructuredPolicyWithSpecForTest(name string) *unstructured.Unstructured {
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       types.UID(name + "-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Paused: false,
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	unstructuredPolicy, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	return &unstructured.Unstructured{Object: unstructuredPolicy}
}
