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

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// PolicyEvaluationResult contains the results of evaluating a policy.
type PolicyEvaluationResult struct {
	MatchedCount            int64
	DeletedCount            int64
	PendingCount            int64
	ResourcesToDelete       []*unstructured.Unstructured
	ResourcesToDeleteReasons map[string]string
}

