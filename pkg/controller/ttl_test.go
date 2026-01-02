package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestGCPolicyReconciler_calculateExpirationTime_FixedTTL(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": metav1.Now().Format(time.RFC3339),
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		SecondsAfterCreation: int64Ptr(3600), // 1 hour
	}

	ttl, err := gc.calculateTTL(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateTTL() returned error: %v", err)
	}

	if ttl != 3600 {
		t.Errorf("calculateTTL() = %d, want 3600", ttl)
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_MappedTTL(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"severity": "CRITICAL",
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		FieldPath: "spec.severity",
		Mappings: map[string]int64{
			"CRITICAL": 1814400, // 3 weeks
			"HIGH":     1209600, // 2 weeks
			"MEDIUM":   604800,  // 1 week
			"LOW":      259200,  // 3 days
		},
		Default: int64Ptr(604800), // 1 week default
	}

	ttl, err := gc.calculateTTL(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateTTL() returned error: %v", err)
	}

	if ttl != 1814400 {
		t.Errorf("calculateTTL() = %d, want 1814400", ttl)
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_MappedTTL_Default(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"severity": "UNKNOWN",
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		FieldPath: "spec.severity",
		Mappings: map[string]int64{
			"CRITICAL": 1814400,
		},
		Default: int64Ptr(604800), // 1 week default
	}

	ttl, err := gc.calculateTTL(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateTTL() returned error: %v", err)
	}

	if ttl != 604800 {
		t.Errorf("calculateTTL() = %d, want 604800 (default)", ttl)
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_RelativeTTL(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"lastProcessedAt": oneHourAgo.Format(time.RFC3339),
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		RelativeTo:   "status.lastProcessedAt",
		SecondsAfter: int64Ptr(7200), // 2 hours after
	}

	ttl, err := gc.calculateTTL(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateTTL() returned error: %v", err)
	}

	// TTL should be approximately 1 hour (2 hours after - 1 hour ago = 1 hour remaining)
	expectedTTL := int64(3600) // 1 hour in seconds
	tolerance := int64(60)     // 1 minute tolerance

	if ttl < expectedTTL-tolerance || ttl > expectedTTL+tolerance {
		t.Errorf("calculateTTL() = %d, want approximately %d (within %d seconds)", ttl, expectedTTL, tolerance)
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_NoTTL(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	resource := &unstructured.Unstructured{}

	ttlSpec := &v1alpha1.TTLSpec{}

	_, err := gc.calculateTTL(resource, ttlSpec)
	if err == nil {
		t.Error("calculateTTL() should return error when no TTL is configured")
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_FieldPathNotFound(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		FieldPath: "spec.nonexistent",
		Mappings: map[string]int64{
			"VALUE": 3600,
		},
	}

	_, err := gc.calculateTTL(resource, ttlSpec)
	if err == nil {
		t.Error("calculateTTL() should return error when field path is not found")
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
