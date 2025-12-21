package integration

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/controller"
)

func TestGCController_Integration(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create controller
	gcController, err := controller.NewGCController(dynamicClient)
	if err != nil {
		t.Fatalf("Failed to create GC controller: %v", err)
	}

	// Test that controller can be started
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start controller in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- gcController.Start()
	}()

	// Wait a bit for initialization
	time.Sleep(100 * time.Millisecond)

	// Stop controller
	gcController.Stop()

	// Check for errors
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("Controller start returned error: %v", err)
		}
	case <-ctx.Done():
		// Timeout is OK, controller is running
	}
}

func TestGCController_PolicyCRUD(t *testing.T) {
	// This test would require a more sophisticated fake client
	// For now, we'll test the basic structure
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	gcController, err := controller.NewGCController(dynamicClient)
	if err != nil {
		t.Fatalf("Failed to create GC controller: %v", err)
	}

	// Verify controller was created
	if gcController == nil {
		t.Fatal("GCController is nil")
	}
}

// Helper function to create a test policy
func createTestPolicy(namespace, name string) *v1alpha1.GarbageCollectionPolicy {
	return &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
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
}

// Helper function to create a test resource
func createTestResource(namespace, name string, creationTime time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": creationTime.Format(time.RFC3339),
			},
		},
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
