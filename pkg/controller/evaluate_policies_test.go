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

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// TestGCController_evaluatePolicies_ContextCancellation tests context cancellation handling.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePolicies_ContextCancellation(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePolicies_CacheNotSynced_New tests cache not synced scenario.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePolicies_CacheNotSynced_New(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePolicies_EmptyPolicies tests empty policies list.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePolicies_EmptyPolicies(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePolicies_WithMaxConcurrent_New tests different maxConcurrent settings.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePolicies_WithMaxConcurrent_New(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePoliciesSequential_ErrorHandling tests error handling in sequential evaluation.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePoliciesSequential_ErrorHandling(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePoliciesParallel_WorkerPool tests worker pool behavior.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePoliciesParallel_WorkerPool(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}

// TestGCController_evaluatePoliciesParallel_ContextCancellation tests context cancellation in parallel evaluation.
// GCController is deprecated - use GCPolicyReconciler tests with mocks instead.
func TestGCController_evaluatePoliciesParallel_ContextCancellation(t *testing.T) {
	t.Skip("GCController is deprecated - use GCPolicyReconciler tests with mocks instead")
}
