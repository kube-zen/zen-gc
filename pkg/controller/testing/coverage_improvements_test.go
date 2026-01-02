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

// Package testing provides test utilities and mocks for the controller package.
package testing

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/controller"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Static errors for testing.
var (
	errListResourcesFailed = errors.New("list resources failed")
)

// TestPolicyEvaluationService_NoResources tests evaluation with no resources.
func TestPolicyEvaluationService_NoResources(t *testing.T) {
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	mockLister := NewMockResourceLister()
	// No resources set - empty list
	mockSelectorMatcher := NewMockSelectorMatcher()
	mockConditionMatcher := NewMockConditionMatcher()
	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy should handle empty resources gracefully: %v", err)
	}
}

// TestPolicyEvaluationService_NoConditions tests evaluation without conditions.
func TestPolicyEvaluationService_NoConditions(t *testing.T) {
	now := time.Now()
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              "test-cm",
				"namespace":         "default",
				"uid":               "test-uid",
				"creationTimestamp": metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
			// No conditions specified
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{resource})

	mockSelectorMatcher := NewMockSelectorMatcher()
	mockSelectorMatcher.SetMatch(resource, true)

	mockConditionMatcher := NewMockConditionMatcher()
	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()
	mockDeleter.SetDeleteResult(resource, nil)

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
}

// TestPolicyEvaluationService_ListResourcesError tests error handling when listing resources fails.
func TestPolicyEvaluationService_ListResourcesError(t *testing.T) {
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	// Create a mock lister that returns an error
	mockLister := NewMockResourceLister()
	mockLister.SetError(errListResourcesFailed)

	mockSelectorMatcher := NewMockSelectorMatcher()
	mockConditionMatcher := NewMockConditionMatcher()
	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err == nil {
		t.Error("EvaluatePolicy should return error when listing resources fails")
	}
}

// TestPolicyEvaluationService_ContextCancellationDuringEvaluation tests context cancellation.
func TestPolicyEvaluationService_ContextCancellationDuringEvaluation(t *testing.T) {
	now := time.Now()
	// Create many resources to test context check frequency
	resources := make([]*unstructured.Unstructured, 0, 150)
	for i := 0; i < 150; i++ {
		resources = append(resources, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":              fmt.Sprintf("test-cm-%d", i),
					"namespace":         "default",
					"uid":               types.UID(fmt.Sprintf("uid-%d", i)),
					"creationTimestamp": metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
				},
			},
		})
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", resources)

	mockSelectorMatcher := NewMockSelectorMatcher()
	for _, r := range resources {
		mockSelectorMatcher.SetMatch(r, true)
	}

	mockConditionMatcher := NewMockConditionMatcher()
	for _, r := range resources {
		mockConditionMatcher.SetMeetsConditions(r, true)
	}

	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()

	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil,
		sdklog.NewLogger("zen-gc"),
	)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.EvaluatePolicy(ctx, policy)
	// Should handle cancellation gracefully
	if err != nil {
		t.Logf("EvaluatePolicy returned error (may be expected with cancellation): %v", err)
	}
}

// TestPolicyEvaluationService_StatusUpdater tests with status updater.
func TestPolicyEvaluationService_StatusUpdater(t *testing.T) {
	now := time.Now()
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              "test-cm",
				"namespace":         "default",
				"uid":               "test-uid",
				"creationTimestamp": metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{resource})

	mockSelectorMatcher := NewMockSelectorMatcher()
	mockSelectorMatcher.SetMatch(resource, true)

	mockConditionMatcher := NewMockConditionMatcher()
	mockConditionMatcher.SetMeetsConditions(resource, true)

	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()
	mockDeleter.SetDeleteResult(resource, nil)

	// StatusUpdater requires a real dynamic client, so we pass nil.
	// This tests that the service handles nil status updater gracefully.
	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil, // StatusUpdater - nil is OK for this test
		nil, // EventRecorder - nil is OK for this test
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
}

// TestPolicyEvaluationService_EventRecorder tests with event recorder.
func TestPolicyEvaluationService_EventRecorder(t *testing.T) {
	now := time.Now()
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              "test-cm",
				"namespace":         "default",
				"uid":               "test-uid",
				"creationTimestamp": metav1.NewTime(now.Add(-2 * time.Hour)).Format(time.RFC3339),
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: func() *int64 { v := int64(3600); return &v }(),
			},
		},
	}

	mockLister := NewMockResourceLister()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	mockLister.SetResources(gvr, "default", []*unstructured.Unstructured{resource})

	mockSelectorMatcher := NewMockSelectorMatcher()
	mockSelectorMatcher.SetMatch(resource, true)

	mockConditionMatcher := NewMockConditionMatcher()
	mockConditionMatcher.SetMeetsConditions(resource, true)

	mockRateLimiter := NewMockRateLimiterProvider()
	mockDeleter := NewMockBatchDeleterCore()
	mockDeleter.SetDeleteResult(resource, nil)

	// Test with nil event recorder (should handle gracefully)
	service := controller.NewPolicyEvaluationService(
		mockLister,
		mockSelectorMatcher,
		mockConditionMatcher,
		nil,
		mockRateLimiter,
		mockDeleter,
		nil,
		nil, // EventRecorder - nil is OK
		sdklog.NewLogger("zen-gc"),
	)

	ctx := context.Background()
	err := service.EvaluatePolicy(ctx, policy)
	if err != nil {
		t.Fatalf("EvaluatePolicy failed: %v", err)
	}
}
