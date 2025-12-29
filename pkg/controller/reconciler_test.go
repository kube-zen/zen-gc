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
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

// setupTestReconciler creates a test reconciler with fake clients.
func setupTestReconciler(t *testing.T) (*GCPolicyReconciler, client.Client) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add v1alpha1 to scheme: %v", err)
	}

	// Create fake controller-runtime client
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()

	// Create fake dynamic client
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create status updater and event recorder
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil) // nil is OK for tests

	// Create reconciler
	reconciler := NewGCPolicyReconciler(
		fakeClient,
		scheme,
		dynamicClient,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	return reconciler, fakeClient
}

func TestNewGCPolicyReconciler(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	if reconciler == nil {
		t.Fatal("NewGCPolicyReconciler() returned nil reconciler")
	}

	if reconciler.Client == nil {
		t.Error("NewGCPolicyReconciler() did not set Client")
	}

	if reconciler.dynamicClient == nil {
		t.Error("NewGCPolicyReconciler() did not set dynamicClient")
	}

	if reconciler.resourceInformers == nil {
		t.Error("NewGCPolicyReconciler() did not initialize resourceInformers map")
	}

	if reconciler.rateLimiters == nil {
		t.Error("NewGCPolicyReconciler() did not initialize rateLimiters map")
	}

	if reconciler.policyUIDs == nil {
		t.Error("NewGCPolicyReconciler() did not initialize policyUIDs map")
	}

	if reconciler.policySpecs == nil {
		t.Error("NewGCPolicyReconciler() did not initialize policySpecs map")
	}
}

func TestGCPolicyReconciler_Reconcile_NotFound(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler(t)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Errorf("Reconcile() should not error on not found, got: %v", err)
	}

	if result.Requeue {
		t.Error("Reconcile() should not requeue on not found")
	}

	// Verify policy was not created
	policy := &v1alpha1.GarbageCollectionPolicy{}
	err = fakeClient.Get(ctx, req.NamespacedName, policy)
	if err == nil {
		t.Error("Policy should not exist")
	}
}

func TestGCPolicyReconciler_Reconcile_PausedPolicy(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler(t)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Paused: true,
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600),
			},
		},
	}

	if err := fakeClient.Create(context.Background(), policy); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-policy",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Errorf("Reconcile() should not error on paused policy, got: %v", err)
	}

	// Should requeue with interval
	if result.RequeueAfter == 0 {
		t.Error("Reconcile() should requeue paused policy with interval")
	}
}

func TestGCPolicyReconciler_Reconcile_PolicyDeletion(t *testing.T) {
	reconciler, fakeClient := setupTestReconciler(t)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600),
			},
		},
	}

	// Create policy
	if err := fakeClient.Create(context.Background(), policy); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Track UID manually for test
	nn := types.NamespacedName{Name: "test-policy", Namespace: "default"}
	reconciler.trackPolicyUID(nn, policy.UID)

	// Create a rate limiter to test cleanup
	reconciler.rateLimitersMu.Lock()
	reconciler.rateLimiters[policy.UID] = NewRateLimiter(10)
	reconciler.rateLimitersMu.Unlock()

	// Delete policy
	if err := fakeClient.Delete(context.Background(), policy); err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	// Reconcile should handle deletion
	req := reconcile.Request{NamespacedName: nn}
	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Errorf("Reconcile() should not error on deletion, got: %v", err)
	}

	if result.Requeue {
		t.Error("Reconcile() should not requeue on deletion")
	}

	// Verify rate limiter was cleaned up
	reconciler.rateLimitersMu.RLock()
	_, exists := reconciler.rateLimiters[policy.UID]
	reconciler.rateLimitersMu.RUnlock()

	if exists {
		t.Error("Rate limiter should be cleaned up on policy deletion")
	}

	// Verify UID tracking was cleaned up
	reconciler.policyUIDsMu.RLock()
	_, exists = reconciler.policyUIDs[nn]
	reconciler.policyUIDsMu.RUnlock()

	if exists {
		t.Error("Policy UID tracking should be cleaned up on deletion")
	}
}

func TestGCPolicyReconciler_shouldRecreateInformer(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("test-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Namespace:  "default",
			},
		},
	}

	// First time - should not recreate
	if reconciler.shouldRecreateInformer(policy) {
		t.Error("shouldRecreateInformer() should return false for new policy")
	}

	// Track the spec
	reconciler.trackPolicySpec(policy.UID, &policy.Spec)

	// Same spec - should not recreate
	if reconciler.shouldRecreateInformer(policy) {
		t.Error("shouldRecreateInformer() should return false for unchanged spec")
	}

	// Change APIVersion - should recreate
	policy.Spec.TargetResource.APIVersion = "apps/v1"
	if !reconciler.shouldRecreateInformer(policy) {
		t.Error("shouldRecreateInformer() should return true when APIVersion changes")
	}

	// Reset and change Kind
	policy.Spec.TargetResource.APIVersion = "v1"
	reconciler.trackPolicySpec(policy.UID, &policy.Spec)
	policy.Spec.TargetResource.Kind = "Deployment"
	if !reconciler.shouldRecreateInformer(policy) {
		t.Error("shouldRecreateInformer() should return true when Kind changes")
	}

	// Reset and change Namespace
	policy.Spec.TargetResource.Kind = "ConfigMap"
	reconciler.trackPolicySpec(policy.UID, &policy.Spec)
	policy.Spec.TargetResource.Namespace = "other"
	if !reconciler.shouldRecreateInformer(policy) {
		t.Error("shouldRecreateInformer() should return true when Namespace changes")
	}
}

func TestGCPolicyReconciler_trackPolicyUID(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	nn := types.NamespacedName{Name: "test", Namespace: "default"}
	uid := types.UID("test-uid")

	reconciler.trackPolicyUID(nn, uid)

	reconciler.policyUIDsMu.RLock()
	trackedUID, exists := reconciler.policyUIDs[nn]
	reconciler.policyUIDsMu.RUnlock()

	if !exists {
		t.Error("Policy UID should be tracked")
	}

	if trackedUID != uid {
		t.Errorf("Tracked UID = %s, want %s", trackedUID, uid)
	}
}

func TestGCPolicyReconciler_SetupWithManager(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	// Verify reconciler has SetupWithManager method
	// This test just ensures the method exists and can be called
	// Full integration test would require envtest setup
	if reconciler == nil {
		t.Fatal("Reconciler should not be nil")
	}

	// The method exists if we can reference it without compilation error
	_ = reconciler.SetupWithManager
}

func TestGCPolicyReconciler_cleanupResourceInformer(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	uid := types.UID("test-uid")

	// Create a mock informer entry
	reconciler.resourceInformersMu.Lock()
	reconciler.resourceInformers[uid] = nil // nil is OK for this test
	reconciler.resourceInformerFactories[uid] = nil
	initialCount := len(reconciler.resourceInformers)
	reconciler.resourceInformersMu.Unlock()

	// Cleanup
	reconciler.cleanupResourceInformer(uid)

	// Verify cleanup
	reconciler.resourceInformersMu.RLock()
	finalCount := len(reconciler.resourceInformers)
	_, exists := reconciler.resourceInformers[uid]
	reconciler.resourceInformersMu.RUnlock()

	if exists {
		t.Error("Resource informer should be cleaned up")
	}

	if finalCount != initialCount-1 {
		t.Errorf("Expected informer count to decrease by 1, got %d -> %d", initialCount, finalCount)
	}
}

func TestGCPolicyReconciler_cleanupRateLimiter(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	uid := types.UID("test-uid")

	// Create a rate limiter
	reconciler.rateLimitersMu.Lock()
	reconciler.rateLimiters[uid] = NewRateLimiter(10)
	initialCount := len(reconciler.rateLimiters)
	reconciler.rateLimitersMu.Unlock()

	// Cleanup
	reconciler.cleanupRateLimiter(uid)

	// Verify cleanup
	reconciler.rateLimitersMu.RLock()
	finalCount := len(reconciler.rateLimiters)
	_, exists := reconciler.rateLimiters[uid]
	reconciler.rateLimitersMu.RUnlock()

	if exists {
		t.Error("Rate limiter should be cleaned up")
	}

	if finalCount != initialCount-1 {
		t.Errorf("Expected rate limiter count to decrease by 1, got %d -> %d", initialCount, finalCount)
	}
}

func TestGCPolicyReconciler_getRequeueInterval(t *testing.T) {
	reconciler, _ := setupTestReconciler(t)

	interval := reconciler.getRequeueInterval()

	if interval <= 0 {
		t.Errorf("Requeue interval should be positive, got: %v", interval)
	}

	if interval != DefaultGCInterval {
		t.Errorf("Expected default interval %v, got %v", DefaultGCInterval, interval)
	}
}
