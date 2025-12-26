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
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

func TestGCController_getBatchSize(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCControllerWithConfig(
		dynamicClient,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig().WithBatchSize(100),
	)
	if err != nil {
		t.Fatalf("NewGCControllerWithConfig() returned error: %v", err)
	}

	tests := []struct {
		name         string
		policy       *v1alpha1.GarbageCollectionPolicy
		expectedSize int
	}{
		{
			name: "default batch size",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					Behavior: v1alpha1.BehaviorSpec{},
				},
			},
			expectedSize: 100, // From config
		},
		{
			name: "policy batch size overrides config",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					Behavior: v1alpha1.BehaviorSpec{
						BatchSize: 200,
					},
				},
			},
			expectedSize: 200,
		},
		{
			name: "zero batch size uses config default",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					Behavior: v1alpha1.BehaviorSpec{
						BatchSize: 0,
					},
				},
			},
			expectedSize: 100, // From config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := controller.getBatchSize(tt.policy)
			if size != tt.expectedSize {
				t.Errorf("getBatchSize() = %d, want %d", size, tt.expectedSize)
			}
		})
	}
}

func TestLabelSelectorsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        *metav1.LabelSelector
		b        *metav1.LabelSelector
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil",
			a:        nil,
			b:        &metav1.LabelSelector{},
			expected: false,
		},
		{
			name: "equal selectors",
			a: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			b: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			expected: true,
		},
		{
			name: "different selectors",
			a: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			b: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "other",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := labelSelectorsEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("labelSelectorsEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGCController_handlePolicyDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCControllerWithConfig(
		dynamicClient,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)
	if err != nil {
		t.Fatalf("NewGCControllerWithConfig() returned error: %v", err)
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
	}

	// Create a rate limiter for this policy
	controller.getOrCreateRateLimiter(policy)

	// Convert to unstructured
	unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy to unstructured: %v", err)
	}

	// Should not panic
	controller.handlePolicyDelete(&unstructured.Unstructured{Object: unstructuredPolicy})
}

func TestGCController_handlePolicyUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCControllerWithConfig(
		dynamicClient,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)
	if err != nil {
		t.Fatalf("NewGCControllerWithConfig() returned error: %v", err)
	}

	tests := []struct {
		name      string
		oldPolicy *v1alpha1.GarbageCollectionPolicy
		newPolicy *v1alpha1.GarbageCollectionPolicy
	}{
		{
			name: "kind changed",
			oldPolicy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
				},
			},
			newPolicy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "Secret", // Changed
					},
				},
			},
		},
		{
			name: "no change",
			oldPolicy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
				},
			},
			newPolicy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
				},
			},
		},
		{
			name: "namespace changed",
			oldPolicy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "default",
					},
				},
			},
			newPolicy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "kube-system", // Changed
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to unstructured
			oldUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.oldPolicy)
			if err != nil {
				t.Fatalf("Failed to convert old policy: %v", err)
			}
			newUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.newPolicy)
			if err != nil {
				t.Fatalf("Failed to convert new policy: %v", err)
			}

			// Should not panic
			controller.handlePolicyUpdate(
				&unstructured.Unstructured{Object: oldUnstructured},
				&unstructured.Unstructured{Object: newUnstructured},
			)
		})
	}
}

func TestGCController_handlePolicyUpdate_InvalidTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCControllerWithConfig(
		dynamicClient,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)
	if err != nil {
		t.Fatalf("NewGCControllerWithConfig() returned error: %v", err)
	}

	// Test with invalid types (should not panic)
	controller.handlePolicyUpdate("invalid", "invalid")
	controller.handlePolicyUpdate(nil, nil)
}
