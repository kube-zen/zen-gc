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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// labelSelectorsEqual compares two label selectors for equality.
func labelSelectorsEqual(a, b *metav1.LabelSelector) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if !matchLabelsEqual(a.MatchLabels, b.MatchLabels) {
		return false
	}

	return matchExpressionsEqual(a.MatchExpressions, b.MatchExpressions)
}

// matchLabelsEqual compares two matchLabels maps for equality.
func matchLabelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// matchExpressionsEqual compares two matchExpressions slices for equality.
func matchExpressionsEqual(a, b []metav1.LabelSelectorRequirement) bool {
	if len(a) != len(b) {
		return false
	}
	for i, exprA := range a {
		exprB := b[i]
		if !labelSelectorRequirementEqual(exprA, exprB) {
			return false
		}
	}
	return true
}

// labelSelectorRequirementEqual compares two label selector requirements for equality.
func labelSelectorRequirementEqual(a, b metav1.LabelSelectorRequirement) bool {
	if a.Key != b.Key || a.Operator != b.Operator || len(a.Values) != len(b.Values) {
		return false
	}
	for i, valA := range a.Values {
		if valA != b.Values[i] {
			return false
		}
	}
	return true
}
