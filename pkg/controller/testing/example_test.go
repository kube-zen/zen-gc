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

package testing

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

// TestMockUsage demonstrates how to use mocks for testing.
func TestMockUsage(t *testing.T) {
	// Create mock informer with test resources
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "test-cm",
					"namespace": "default",
					"uid":       "test-uid",
				},
			},
		},
	}

	mockInformer := NewMockResourceInformer(resources)
	if !mockInformer.HasSynced() {
		t.Error("Mock informer should be synced")
	}

	store := mockInformer.GetStore()
	if store == nil {
		t.Error("Mock informer should have a store")
	}

	// Create mock resource lister
	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", resources)

	listed, err := mockLister.ListResources(context.Background(), gvr, "default")
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(listed) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(listed))
	}

	// Create mock selector matcher
	mockMatcher := NewMockSelectorMatcher()
	resource := resources[0]
	mockMatcher.SetMatch(resource, true)

	spec := &v1alpha1.TargetResourceSpec{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}
	if !mockMatcher.MatchesSelectors(resource, spec) {
		t.Error("Resource should match selectors")
	}

	// Create mock rate limiter provider
	mockRateLimiter := NewMockRateLimiterProvider()
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
	}
	limiter := mockRateLimiter.GetOrCreateRateLimiter(policy)
	if limiter == nil {
		t.Error("Rate limiter should not be nil")
	}

	// Create mock batch deleter
	mockDeleter := NewMockBatchDeleterCore()
	mockDeleter.SetDeleteResult(resource, nil) // Success

	batch := []*unstructured.Unstructured{resource}
	deleted, errors := mockDeleter.DeleteBatch(context.Background(), batch, policy, limiter, nil)
	if deleted != 1 {
		t.Errorf("Expected 1 deletion, got %d", deleted)
	}
	if len(errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(errors))
	}
}

// TestMockResourceInformer tests the mock resource informer.
func TestMockResourceInformer(t *testing.T) {
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "test-cm",
					"namespace": "default",
				},
			},
		},
	}

	mock := NewMockResourceInformer(resources)
	if !mock.HasSynced() {
		t.Error("Should be synced by default")
	}

	store := mock.GetStore()
	if store == nil {
		t.Fatal("Store should not be nil")
	}

	items := store.List()
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}

	mock.SetSynced(false)
	if mock.HasSynced() {
		t.Error("Should not be synced after SetSynced(false)")
	}
}

// TestMockResourceLister tests the mock resource lister.
func TestMockResourceLister(t *testing.T) {
	mock := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	// Test empty list
	resources, err := mock.ListResources(context.Background(), gvr, "default")
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("Expected empty list, got %d resources", len(resources))
	}

	// Set resources
	testResources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "test-cm",
					"namespace": "default",
				},
			},
		},
	}
	mock.SetResources(gvr, "default", testResources)

	// List resources
	resources, err = mock.ListResources(context.Background(), gvr, "default")
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}
}
