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

	// Compare matchLabels
	if len(a.MatchLabels) != len(b.MatchLabels) {
		return false
	}
	for k, v := range a.MatchLabels {
		if b.MatchLabels[k] != v {
			return false
		}
	}

	// Compare matchExpressions
	if len(a.MatchExpressions) != len(b.MatchExpressions) {
		return false
	}
	for i, exprA := range a.MatchExpressions {
		if i >= len(b.MatchExpressions) {
			return false
		}
		exprB := b.MatchExpressions[i]
		if exprA.Key != exprB.Key ||
			exprA.Operator != exprB.Operator ||
			len(exprA.Values) != len(exprB.Values) {
			return false
		}
		for j, valA := range exprA.Values {
			if j >= len(exprB.Values) || valA != exprB.Values[j] {
				return false
			}
		}
	}

	return true
}

