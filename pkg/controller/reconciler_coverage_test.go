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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

// TestGCPolicyReconciler_getRequeueInterval_Coverage tests getRequeueInterval for coverage.
func TestGCPolicyReconciler_getRequeueInterval_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	// Test with default config
	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	interval := reconciler.getRequeueInterval()
	if interval <= 0 {
		t.Errorf("getRequeueInterval() returned invalid interval: %v", interval)
	}

	// Test with custom config
	customConfig := config.NewControllerConfig().WithGCInterval(2 * time.Minute)
	reconciler2 := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		customConfig,
	)

	interval2 := reconciler2.getRequeueInterval()
	if interval2 != 2*time.Minute {
		t.Errorf("getRequeueInterval() = %v, want %v", interval2, 2*time.Minute)
	}
}

// TestGCPolicyReconciler_getRequeueIntervalForPolicy tests getRequeueIntervalForPolicy.
func TestGCPolicyReconciler_getRequeueIntervalForPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	// Test with policy-specific interval
	policyInterval := metav1.Duration{Duration: 5 * time.Minute}
	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			EvaluationInterval: &policyInterval,
		},
	}

	interval := reconciler.getRequeueIntervalForPolicy(policy)
	if interval != 5*time.Minute {
		t.Errorf("getRequeueIntervalForPolicy() = %v, want %v", interval, 5*time.Minute)
	}

	// Test without policy-specific interval (should use default)
	policy2 := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{},
	}

	interval2 := reconciler.getRequeueIntervalForPolicy(policy2)
	if interval2 <= 0 {
		t.Errorf("getRequeueIntervalForPolicy() returned invalid interval: %v", interval2)
	}
}

// TestGCPolicyReconciler_trackPolicyUID_Coverage tests trackPolicyUID for coverage.
func TestGCPolicyReconciler_trackPolicyUID_Coverage(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	nn := types.NamespacedName{Name: "test-policy", Namespace: "default"}
	uid := types.UID("test-uid")

	reconciler.trackPolicyUID(nn, uid)

	reconciler.policyUIDsMu.RLock()
	trackedUID, exists := reconciler.policyUIDs[nn]
	reconciler.policyUIDsMu.RUnlock()

	if !exists {
		t.Error("trackPolicyUID() did not track UID")
	}
	if trackedUID != uid {
		t.Errorf("trackPolicyUID() tracked UID = %v, want %v", trackedUID, uid)
	}
}

// TestGCPolicyReconciler_trackPolicySpec tests trackPolicySpec.
func TestGCPolicyReconciler_trackPolicySpec(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	uid := types.UID("test-uid")
	spec := &v1alpha1.GarbageCollectionPolicySpec{
		TargetResource: v1alpha1.TargetResourceSpec{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
	}

	reconciler.trackPolicySpec(uid, spec)

	reconciler.policySpecsMu.RLock()
	trackedSpec, exists := reconciler.policySpecs[uid]
	reconciler.policySpecsMu.RUnlock()

	if !exists {
		t.Error("trackPolicySpec() did not track spec")
	}
	if trackedSpec.TargetResource.Kind != spec.TargetResource.Kind {
		t.Errorf("trackPolicySpec() tracked spec = %v, want %v", trackedSpec, spec)
	}
}

// TestGCPolicyReconciler_getOrCreateRateLimiter tests getOrCreateRateLimiter.
func TestGCPolicyReconciler_getOrCreateRateLimiter(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("test-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: 10,
			},
		},
	}

	// First call should create rate limiter
	limiter1 := reconciler.getOrCreateRateLimiter(policy)
	if limiter1 == nil {
		t.Error("getOrCreateRateLimiter() returned nil")
	}

	// Second call should return same rate limiter
	limiter2 := reconciler.getOrCreateRateLimiter(policy)
	if limiter1 != limiter2 {
		t.Error("getOrCreateRateLimiter() returned different rate limiter on second call")
	}
}

// TestGCPolicyReconciler_getBatchSize tests getBatchSize.
func TestGCPolicyReconciler_getBatchSize(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	// Test with policy-specific batch size
	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				BatchSize: 100,
			},
		},
	}

	batchSize := reconciler.getBatchSize(policy)
	if batchSize != 100 {
		t.Errorf("getBatchSize() = %d, want 100", batchSize)
	}

	// Test without policy-specific batch size (should use default)
	policy2 := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{},
	}

	batchSize2 := reconciler.getBatchSize(policy2)
	if batchSize2 <= 0 {
		t.Errorf("getBatchSize() returned invalid batch size: %d", batchSize2)
	}
}

// TestGCPolicyReconciler_cleanupPolicyResources tests cleanupPolicyResources.
func TestGCPolicyReconciler_cleanupPolicyResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(fake.NewSimpleClientset())

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	nn := types.NamespacedName{Name: "test-policy", Namespace: "default"}
	uid := types.UID("test-uid")

	// Track policy UID
	reconciler.trackPolicyUID(nn, uid)

	// Create a rate limiter for this policy
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: uid,
		},
	}
	_ = reconciler.getOrCreateRateLimiter(policy)

	// Cleanup should remove tracked UID and rate limiter
	reconciler.cleanupPolicyResources(nn)

	reconciler.policyUIDsMu.RLock()
	_, exists := reconciler.policyUIDs[nn]
	reconciler.policyUIDsMu.RUnlock()

	if exists {
		t.Error("cleanupPolicyResources() did not remove tracked UID")
	}

	reconciler.rateLimitersMu.RLock()
	_, exists = reconciler.rateLimiters[uid]
	reconciler.rateLimitersMu.RUnlock()

	if exists {
		t.Error("cleanupPolicyResources() did not remove rate limiter")
	}
}


