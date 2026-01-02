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

// TestGCController_recordPolicyPhaseMetrics tests the recordPolicyPhaseMetrics function.
func TestGCController_recordPolicyPhaseMetrics(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	tests := []struct {
		name     string
		policies []interface{}
	}{
		{
			name:     "empty policies list",
			policies: []interface{}{},
		},
		{
			name: "policies with different phases",
			policies: []interface{}{
				createUnstructuredPolicy("policy1", "default", PolicyPhaseActive),
				createUnstructuredPolicy("policy2", "default", PolicyPhasePaused),
				createUnstructuredPolicy("policy3", "default", PolicyPhaseError),
			},
		},
		{
			name: "policies with empty phase (defaults to Active)",
			policies: []interface{}{
				createUnstructuredPolicy("policy1", "default", ""),
				createUnstructuredPolicy("policy2", "default", PolicyPhaseActive),
			},
		},
		{
			name: "policies with nil conversion (should be skipped)",
			policies: []interface{}{
				createUnstructuredPolicy("policy1", "default", PolicyPhaseActive),
				"invalid-policy-object", // Will be skipped in convertToPolicy
			},
		},
		{
			name: "multiple policies with same phase",
			policies: []interface{}{
				createUnstructuredPolicy("policy1", "default", PolicyPhaseActive),
				createUnstructuredPolicy("policy2", "default", PolicyPhaseActive),
				createUnstructuredPolicy("policy3", "default", PolicyPhaseActive),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic and should record metrics
			controller.recordPolicyPhaseMetrics(tt.policies)
		})
	}
}

// TestGCController_evaluatePoliciesParallel_Coverage tests parallel policy evaluation for coverage.
// Full evaluation requires complex informer setup, so we test the basic structure.
func TestGCController_evaluatePoliciesParallel_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	tests := []struct {
		name          string
		policies      []interface{}
		maxConcurrent int
		setupFunc     func(*GCController)
	}{
		{
			name:          "empty policies list",
			policies:      []interface{}{},
			maxConcurrent: 5,
		},
		{
			name: "policies with paused policy (should be skipped)",
			policies: []interface{}{
				createUnstructuredPolicyWithSpec("policy1", "default", false),
				createUnstructuredPolicyWithSpec("policy2", "default", true), // Paused
				createUnstructuredPolicyWithSpec("policy3", "default", false),
			},
			maxConcurrent: 2,
		},
		{
			name: "policies with invalid objects (should be skipped)",
			policies: []interface{}{
				createUnstructuredPolicyWithSpec("policy1", "default", false),
				"invalid-object", // Should be skipped
			},
			maxConcurrent: 1,
		},
		{
			name: "context cancellation during evaluation",
			policies: []interface{}{
				createUnstructuredPolicyWithSpec("policy1", "default", false),
				createUnstructuredPolicyWithSpec("policy2", "default", false),
			},
			maxConcurrent: 1,
			setupFunc: func(gc *GCController) {
				// Cancel context to test cancellation path
				gc.cancel()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh controller for each test
			testController, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			if tt.setupFunc != nil {
				tt.setupFunc(testController)
			}

			// This should not panic - even if evaluation fails due to missing informers
			// The function should handle errors gracefully
			testController.evaluatePoliciesParallel(tt.policies, tt.maxConcurrent)
		})
	}
}

// TestGCController_evaluatePoliciesSequential_Coverage tests sequential evaluation for coverage.
func TestGCController_evaluatePoliciesSequential_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	tests := []struct {
		name     string
		policies []interface{}
		setupFunc func(*GCController)
	}{
		{
			name:     "empty policies list",
			policies: []interface{}{},
		},
		{
			name: "policies with paused policy (should be skipped)",
			policies: []interface{}{
				createUnstructuredPolicyWithSpec("policy1", "default", true), // Paused
				createUnstructuredPolicyWithSpec("policy2", "default", false),
			},
		},
		{
			name: "policies with invalid objects (should be skipped)",
			policies: []interface{}{
				"invalid-object", // Should be skipped
				createUnstructuredPolicyWithSpec("policy1", "default", false),
			},
		},
		{
			name: "context cancellation during evaluation",
			policies: []interface{}{
				createUnstructuredPolicyWithSpec("policy1", "default", false),
			},
			setupFunc: func(gc *GCController) {
				// Cancel context to test cancellation path
				gc.cancel()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh controller for each test
			testController, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
			if err != nil {
				t.Fatalf("Failed to create controller: %v", err)
			}

			if tt.setupFunc != nil {
				tt.setupFunc(testController)
			}

			// This should not panic - even if evaluation fails due to missing informers
			// The function should handle errors gracefully
			testController.evaluatePoliciesSequential(tt.policies)
		})
	}
}

// TestGCController_evaluatePolicies_WithMaxConcurrent tests evaluatePolicies with different maxConcurrent settings.
func TestGCController_evaluatePolicies_WithMaxConcurrent(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	// Test with custom maxConcurrent setting
	cfg := config.NewControllerConfig()
	cfg.MaxConcurrentEvaluations = 3

	controller, err := NewGCControllerWithConfig(dynamicClient, statusUpdater, eventRecorder, cfg)
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Test that evaluatePolicies handles the config correctly
	// Even without starting, we can test the config path
	if controller.config == nil || controller.config.MaxConcurrentEvaluations != 3 {
		t.Error("Controller config not set correctly")
	}
}

// Helper function to create unstructured policy with phase.
func createUnstructuredPolicy(name, namespace, phase string) *unstructured.Unstructured {
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: phase,
		},
	}

	unstructuredPolicy, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	return &unstructured.Unstructured{Object: unstructuredPolicy}
}

// Helper function to create unstructured policy with spec (including paused flag).
func createUnstructuredPolicyWithSpec(name, namespace string, paused bool) *unstructured.Unstructured {
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Paused: paused,
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
