package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestMatchesSelectorsShared_LabelSelector(t *testing.T) {
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
							"temporary": "true",
						},
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"temporary": "true",
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
							"app": "other-app",
						},
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"temporary": "true",
					},
				},
			},
			expectedMatch: false,
		},
		{
			name: "no label selector matches all",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			target:        &v1alpha1.TargetResourceSpec{},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesSelectorsShared(tt.resource, tt.target)
			if result != tt.expectedMatch {
				t.Errorf("matchesSelectorsShared() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

func TestMatchesSelectorsShared_Namespace(t *testing.T) {
	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		target        *v1alpha1.TargetResourceSpec
		expectedMatch bool
	}{
		{
			name: "matches namespace",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "zen-system",
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				Namespace: "zen-system",
			},
			expectedMatch: true,
		},
		{
			name: "does not match namespace",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				Namespace: "zen-system",
			},
			expectedMatch: false,
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
		{
			name: "empty namespace matches all",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "any-namespace",
					},
				},
			},
			target:        &v1alpha1.TargetResourceSpec{},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesSelectorsShared(tt.resource, tt.target)
			if result != tt.expectedMatch {
				t.Errorf("matchesSelectors() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

func TestGCController_matchesSelectors_FieldSelector(t *testing.T) {
	// Test matchesSelectorsShared directly

	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		target        *v1alpha1.TargetResourceSpec
		expectedMatch bool
	}{
		{
			name: "matches field selector",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "zen-system",
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				FieldSelector: &v1alpha1.FieldSelectorSpec{
					MatchFields: map[string]string{
						"metadata.namespace": "zen-system",
					},
				},
			},
			expectedMatch: true,
		},
		{
			name: "does not match field selector",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			target: &v1alpha1.TargetResourceSpec{
				FieldSelector: &v1alpha1.FieldSelectorSpec{
					MatchFields: map[string]string{
						"metadata.namespace": "zen-system",
					},
				},
			},
			expectedMatch: false,
		},
		{
			name: "no field selector matches all",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			target:        &v1alpha1.TargetResourceSpec{},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesSelectorsShared(tt.resource, tt.target)
			if result != tt.expectedMatch {
				t.Errorf("matchesSelectors() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}
