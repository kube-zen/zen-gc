//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

// TestE2E_GCController requires a running Kubernetes cluster
// Run with: go test -tags=e2e ./test/e2e
// Or use: make test-e2e
func TestE2E_GCController(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Get Kubernetes config
	config, err := getKubeConfig()
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create dynamic client: %v", err)
	}

	ctx := context.Background()
	namespace := "default" // Use default namespace for E2E tests

	// Test policy creation
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: namespace,
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(60), // 1 minute for testing
			},
			Behavior: v1alpha1.BehaviorSpec{
				DryRun: true, // Use dry-run for safety
			},
		},
	}

	// Convert to unstructured
	policyObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy: %v", err)
	}

	unstructuredPolicy := &unstructured.Unstructured{Object: policyObj}
	unstructuredPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gc.kube-zen.io",
		Version: "v1alpha1",
		Kind:    "GarbageCollectionPolicy",
	})

	// Create policy
	_, err = dynamicClient.Resource(policyGVR).Namespace(namespace).Create(ctx, unstructuredPolicy, metav1.CreateOptions{})
	if err != nil {
		t.Logf("Note: Policy creation failed (may need CRD installed): %v", err)
		t.Skip("Skipping E2E test - CRD not installed")
	}

	// Cleanup
	defer func() {
		dynamicClient.Resource(policyGVR).Namespace(namespace).Delete(ctx, "test-policy", metav1.DeleteOptions{})
	}()

	// Wait a bit for controller to process
	time.Sleep(5 * time.Second)

	// Verify policy exists
	_, err = dynamicClient.Resource(policyGVR).Namespace(namespace).Get(ctx, "test-policy", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get policy: %v", err)
	}
}

// TestE2E_PolicyDeletion tests that policies can be deleted and controller handles it gracefully.
func TestE2E_PolicyDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config, err := getKubeConfig()
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create dynamic client: %v", err)
	}

	ctx := context.Background()
	namespace := "default"

	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy-delete",
			Namespace: namespace,
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

	policyObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy: %v", err)
	}

	unstructuredPolicy := &unstructured.Unstructured{Object: policyObj}
	unstructuredPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gc.kube-zen.io",
		Version: "v1alpha1",
		Kind:    "GarbageCollectionPolicy",
	})

	// Create policy
	_, err = dynamicClient.Resource(policyGVR).Namespace(namespace).Create(ctx, unstructuredPolicy, metav1.CreateOptions{})
	if err != nil {
		t.Skipf("Skipping E2E test - CRD not installed: %v", err)
	}

	// Wait for controller to process
	time.Sleep(2 * time.Second)

	// Delete policy
	err = dynamicClient.Resource(policyGVR).Namespace(namespace).Delete(ctx, "test-policy-delete", metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(2 * time.Second)

	// Verify policy is deleted
	_, err = dynamicClient.Resource(policyGVR).Namespace(namespace).Get(ctx, "test-policy-delete", metav1.GetOptions{})
	if err == nil {
		t.Error("Policy should have been deleted")
	}
}

// TestE2E_ResourceDeletion tests actual resource deletion (with dry-run).
func TestE2E_ResourceDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config, err := getKubeConfig()
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Fatalf("Failed to create dynamic client: %v", err)
	}

	ctx := context.Background()
	testNamespace := "gc-e2e-test"

	// Create test namespace
	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": testNamespace,
			},
		},
	}
	_, err = dynamicClient.Resource(nsGVR).Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		t.Logf("Namespace may already exist: %v", err)
	}

	defer func() {
		dynamicClient.Resource(nsGVR).Delete(ctx, testNamespace, metav1.DeleteOptions{})
	}()

	// Create a test ConfigMap
	cmGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": testNamespace,
				"labels": map[string]interface{}{
					"test": "e2e",
				},
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}
	_, err = dynamicClient.Resource(cmGVR).Namespace(testNamespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Create GC policy with short TTL and dry-run
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deletion-policy",
			Namespace: testNamespace,
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Namespace:  testNamespace,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test": "e2e",
					},
				},
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(10), // Very short TTL for testing
			},
			Behavior: v1alpha1.BehaviorSpec{
				DryRun: true, // Use dry-run for safety
			},
		},
	}

	policyObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy: %v", err)
	}

	unstructuredPolicy := &unstructured.Unstructured{Object: policyObj}
	unstructuredPolicy.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gc.kube-zen.io",
		Version: "v1alpha1",
		Kind:    "GarbageCollectionPolicy",
	})

	_, err = dynamicClient.Resource(policyGVR).Namespace(testNamespace).Create(ctx, unstructuredPolicy, metav1.CreateOptions{})
	if err != nil {
		t.Skipf("Skipping E2E test - CRD not installed: %v", err)
	}

	defer func() {
		dynamicClient.Resource(policyGVR).Namespace(testNamespace).Delete(ctx, "test-deletion-policy", metav1.DeleteOptions{})
	}()

	// Wait for TTL to expire and controller to evaluate
	time.Sleep(15 * time.Second)

	// Verify ConfigMap still exists (dry-run mode)
	_, err = dynamicClient.Resource(cmGVR).Namespace(testNamespace).Get(ctx, "test-cm", metav1.GetOptions{})
	if err != nil {
		t.Errorf("ConfigMap should still exist in dry-run mode: %v", err)
	}
}

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

func int64Ptr(i int64) *int64 {
	return &i
}
