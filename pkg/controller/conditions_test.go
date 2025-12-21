package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestGCController_meetsConditions_Phase(t *testing.T) {
	gc := &GCController{}

	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		conditions    *v1alpha1.ConditionsSpec
		expectedMatch bool
	}{
		{
			name: "matches phase condition",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Processed",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				Phase: []string{"Processed"},
			},
			expectedMatch: true,
		},
		{
			name: "does not match phase condition",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Pending",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				Phase: []string{"Processed"},
			},
			expectedMatch: false,
		},
		{
			name: "matches one of multiple phases",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Succeeded",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				Phase: []string{"Succeeded", "Failed"},
			},
			expectedMatch: true,
		},
		{
			name: "no phase condition matches all",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			conditions: &v1alpha1.ConditionsSpec{},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gc.meetsConditions(tt.resource, tt.conditions)
			if result != tt.expectedMatch {
				t.Errorf("meetsConditions() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

func TestGCController_meetsConditions_Labels(t *testing.T) {
	gc := &GCController{}

	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		conditions    *v1alpha1.ConditionsSpec
		expectedMatch bool
	}{
		{
			name: "matches label condition",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"processed": "true",
						},
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				HasLabels: []v1alpha1.LabelCondition{
					{Key: "processed", Value: "true"},
				},
			},
			expectedMatch: true,
		},
		{
			name: "does not match label condition",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"processed": "false",
						},
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				HasLabels: []v1alpha1.LabelCondition{
					{Key: "processed", Value: "true"},
				},
			},
			expectedMatch: false,
		},
		{
			name: "matches Exists operator",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"processed": "any-value",
						},
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				HasLabels: []v1alpha1.LabelCondition{
					{Key: "processed", Operator: "Exists"},
				},
			},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gc.meetsConditions(tt.resource, tt.conditions)
			if result != tt.expectedMatch {
				t.Errorf("meetsConditions() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

func TestGCController_meetsConditions_Annotations(t *testing.T) {
	gc := &GCController{}

	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		conditions    *v1alpha1.ConditionsSpec
		expectedMatch bool
	}{
		{
			name: "matches annotation condition",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"cleanup-allowed": "true",
						},
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				HasAnnotations: []v1alpha1.AnnotationCondition{
					{Key: "cleanup-allowed", Value: "true"},
				},
			},
			expectedMatch: true,
		},
		{
			name: "does not match annotation condition",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"cleanup-allowed": "false",
						},
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				HasAnnotations: []v1alpha1.AnnotationCondition{
					{Key: "cleanup-allowed", Value: "true"},
				},
			},
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gc.meetsConditions(tt.resource, tt.conditions)
			if result != tt.expectedMatch {
				t.Errorf("meetsConditions() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

func TestGCController_meetsConditions_FieldConditions(t *testing.T) {
	gc := &GCController{}

	tests := []struct {
		name          string
		resource      *unstructured.Unstructured
		conditions    *v1alpha1.ConditionsSpec
		expectedMatch bool
	}{
		{
			name: "matches Equals operator",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"severity": "LOW",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				And: []v1alpha1.FieldCondition{
					{FieldPath: "spec.severity", Operator: "Equals", Value: "LOW"},
				},
			},
			expectedMatch: true,
		},
		{
			name: "matches In operator",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"severity": "LOW",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				And: []v1alpha1.FieldCondition{
					{FieldPath: "spec.severity", Operator: "In", Values: []string{"LOW", "INFO"}},
				},
			},
			expectedMatch: true,
		},
		{
			name: "does not match NotEquals operator",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"severity": "CRITICAL",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				And: []v1alpha1.FieldCondition{
					{FieldPath: "spec.severity", Operator: "NotEquals", Value: "CRITICAL"},
				},
			},
			expectedMatch: false,
		},
		{
			name: "matches NotIn operator",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"severity": "CRITICAL",
					},
				},
			},
			conditions: &v1alpha1.ConditionsSpec{
				And: []v1alpha1.FieldCondition{
					{FieldPath: "spec.severity", Operator: "NotIn", Values: []string{"LOW", "INFO"}},
				},
			},
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gc.meetsConditions(tt.resource, tt.conditions)
			if result != tt.expectedMatch {
				t.Errorf("meetsConditions() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

