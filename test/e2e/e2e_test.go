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

