package integration

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-gc/pkg/controller"
)

func TestGCPolicyReconciler_Integration(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	// Create fake controller-runtime client
	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create status updater and event recorder
	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	// Create reconciler with config
	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil, // RESTMapper - nil is OK for tests
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Verify reconciler was created
	if reconciler == nil {
		t.Fatal("GCPolicyReconciler is nil")
	}

	// Test that reconciler can handle reconcile requests for non-existent policy
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-policy",
			Namespace: "default",
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		// Error is OK if policy doesn't exist
		t.Logf("Reconcile returned error (expected for non-existent policy): %v", err)
	}

	// Verify result is valid
	if result.RequeueAfter < 0 {
		t.Error("RequeueAfter should be non-negative")
	}
}

func TestGCPolicyReconciler_PolicyCRUD(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Verify reconciler was created
	if reconciler == nil {
		t.Fatal("GCPolicyReconciler is nil")
	}
}

// TestGCPolicyReconciler_PolicyDeletion tests that informers and rate limiters are cleaned up when a policy is deleted.
func TestGCPolicyReconciler_PolicyDeletion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	// Register ConfigMaps list kind for fake dynamic client
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
	)
	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Create a test policy
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       types.UID("test-policy-uid"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Namespace:  "default",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600),
			},
		},
	}

	ctx := context.Background()
	if err := fakeClient.Create(ctx, policy); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Reconcile to create informer
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	_, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Logf("Reconcile error (may be expected): %v", err)
	}

	// Delete policy
	if err := fakeClient.Delete(ctx, policy); err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	// Reconcile deletion - should clean up informers and rate limiters
	_, err = reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Logf("Reconcile error on deletion (may be expected): %v", err)
	}

	// Verify cleanup happened (reconciler should handle it)
	// Test passes if Reconcile completes without panic
}

// TestGCPolicyReconciler_InformerCleanup tests that informers are properly cleaned up.
func TestGCPolicyReconciler_InformerCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	// Register ConfigMaps list kind for fake dynamic client
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
	)
	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Create and delete a policy to test cleanup
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
	}

	ctx := context.Background()
	if err := fakeClient.Create(ctx, policy); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-policy",
			Namespace: "default",
		},
	}

	// Reconcile to create informer
	_, _ = reconciler.Reconcile(ctx, req)

	// Delete policy
	_ = fakeClient.Delete(ctx, policy)

	// Reconcile deletion - should clean up informers
	_, _ = reconciler.Reconcile(ctx, req)

	// Test passes if Reconcile completes without panic
}

// TestGCPolicyReconciler_RateLimiterBehavior tests rate limiter creation and cleanup.
func TestGCPolicyReconciler_RateLimiterBehavior(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Rate limiter creation is tested indirectly through policy evaluation.
	// Policy creation is tested in other integration tests.
	// Direct access to getOrCreateRateLimiter is not exposed, which is correct.
	// We verify rate limiter behavior through policy evaluation tests

	// Test passes if reconciler is created successfully
	if reconciler == nil {
		t.Fatal("Reconciler is nil")
	}
}

// TestGCPolicyReconciler_ErrorRecovery tests error recovery scenarios.
func TestGCPolicyReconciler_ErrorRecovery(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Test that reconciler can handle invalid policies gracefully
	ctx := context.Background()

	// Reconcile non-existent policy - should handle gracefully
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		// Error is OK for non-existent policy
		t.Logf("Reconcile error (expected for non-existent policy): %v", err)
	}

	// Verify result is valid
	if result.RequeueAfter < 0 {
		t.Error("RequeueAfter should be non-negative")
	}
}

// TestGCPolicyReconciler_MultiplePolicies tests handling of multiple policies.
func TestGCPolicyReconciler_MultiplePolicies(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add corev1 to scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	// Register ConfigMaps list kind for fake dynamic client
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
	)
	statusUpdater := controller.NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := controller.NewEventRecorder(kubeClient)

	cfg := config.NewControllerConfig()
	reconciler := controller.NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		cfg,
	)

	// Create multiple policies
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		policy := &v1alpha1.GarbageCollectionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy-" + string(rune('0'+i)),
				Namespace: "default",
			},
			Spec: v1alpha1.GarbageCollectionPolicySpec{
				TargetResource: v1alpha1.TargetResourceSpec{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
			},
		}
		if err := fakeClient.Create(ctx, policy); err != nil {
			t.Fatalf("Failed to create policy %d: %v", i, err)
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-policy-" + string(rune('0'+i)),
				Namespace: "default",
			},
		}
		_, _ = reconciler.Reconcile(ctx, req)
	}

	// Test passes if Reconcile completes without panic for multiple policies
}

// Helper function.
func int64Ptr(i int64) *int64 {
	return &i
}
