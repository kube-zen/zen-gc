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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestMatchesFieldOperatorShared(t *testing.T) {

	tests := []struct {
		name       string
		fieldValue string
		condition  v1alpha1.FieldCondition
		want       bool
	}{
		{
			name:       "Equals - match",
			fieldValue: "test-value",
			condition: v1alpha1.FieldCondition{
				Operator: "Equals",
				Value:    "test-value",
			},
			want: true,
		},
		{
			name:       "Equals - no match",
			fieldValue: "test-value",
			condition: v1alpha1.FieldCondition{
				Operator: "Equals",
				Value:    "other-value",
			},
			want: false,
		},
		{
			name:       "NotEquals - match",
			fieldValue: "test-value",
			condition: v1alpha1.FieldCondition{
				Operator: "NotEquals",
				Value:    "other-value",
			},
			want: true,
		},
		{
			name:       "NotEquals - no match",
			fieldValue: "test-value",
			condition: v1alpha1.FieldCondition{
				Operator: "NotEquals",
				Value:    "test-value",
			},
			want: false,
		},
		{
			name:       "In - match",
			fieldValue: "value1",
			condition: v1alpha1.FieldCondition{
				Operator: "In",
				Values:   []string{"value1", "value2", "value3"},
			},
			want: true,
		},
		{
			name:       "In - no match",
			fieldValue: "value4",
			condition: v1alpha1.FieldCondition{
				Operator: "In",
				Values:   []string{"value1", "value2", "value3"},
			},
			want: false,
		},
		{
			name:       "NotIn - match",
			fieldValue: "value4",
			condition: v1alpha1.FieldCondition{
				Operator: OperatorNotIn,
				Values:   []string{"value1", "value2", "value3"},
			},
			want: true,
		},
		{
			name:       "NotIn - no match",
			fieldValue: "value1",
			condition: v1alpha1.FieldCondition{
				Operator: OperatorNotIn,
				Values:   []string{"value1", "value2", "value3"},
			},
			want: false,
		},
		{
			name:       "Unknown operator",
			fieldValue: "test-value",
			condition: v1alpha1.FieldCondition{
				Operator: "Unknown",
				Value:    "test-value",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := controller.matchesFieldOperator(tt.fieldValue, tt.condition)
			if got != tt.want {
				t.Errorf("matchesFieldOperator() = %v, want %v", got, tt.want)
			}
		})
	}
}
