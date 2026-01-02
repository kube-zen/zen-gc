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
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

// TestNormalizeNamespace tests namespace normalization.
func TestNormalizeNamespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty namespace becomes NamespaceAll",
			input:    "",
			expected: metav1.NamespaceAll,
		},
		{
			name:     "wildcard namespace becomes NamespaceAll",
			input:    "*",
			expected: metav1.NamespaceAll,
		},
		{
			name:     "specific namespace is preserved",
			input:    "default",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeNamespace(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeNamespace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBuildDeleteOptions tests delete options building.
func TestBuildDeleteOptions(t *testing.T) {
	gracePeriod := int64(30)
	tests := []struct {
		name   string
		policy *v1alpha1.GarbageCollectionPolicy
		check  func(*metav1.DeleteOptions) bool
	}{
		{
			name: "with grace period",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					Behavior: v1alpha1.BehaviorSpec{
						GracePeriodSeconds: &gracePeriod,
						PropagationPolicy:  PropagationPolicyForeground,
					},
				},
			},
			check: func(opts *metav1.DeleteOptions) bool {
				return opts.GracePeriodSeconds != nil && *opts.GracePeriodSeconds == gracePeriod &&
					opts.PropagationPolicy != nil && *opts.PropagationPolicy == metav1.DeletePropagationForeground
			},
		},
		{
			name: "without grace period",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					Behavior: v1alpha1.BehaviorSpec{
						PropagationPolicy: PropagationPolicyBackground,
					},
				},
			},
			check: func(opts *metav1.DeleteOptions) bool {
				return opts.GracePeriodSeconds == nil &&
					opts.PropagationPolicy != nil && *opts.PropagationPolicy == metav1.DeletePropagationBackground
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildDeleteOptions(tt.policy)
			if !tt.check(opts) {
				t.Errorf("buildDeleteOptions() did not produce expected options")
			}
		})
	}
}

// TestResolveGVRForDeletion tests GVR resolution for deletion.
func TestResolveGVRForDeletion(t *testing.T) {
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

	gvr := reconciler.resolveGVRForDeletion(resource)
	if gvr.Group != "" || gvr.Version != "v1" || gvr.Resource != "configmaps" {
		t.Errorf("resolveGVRForDeletion() = %v, want GroupVersionResource with v1/configmaps", gvr)
	}
}

// TestBuildLabelSelectorFilter tests label selector filter building.
func TestBuildLabelSelectorFilter(t *testing.T) {
	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
			},
		},
	}

	filter := buildLabelSelectorFilter(policy)
	options := &metav1.ListOptions{}
	filter(options)

	if options.LabelSelector == "" {
		t.Error("buildLabelSelectorFilter() should set LabelSelector")
	}
}

