package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
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

	expirationTime, err := reconciler.calculateExpirationTime(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateExpirationTime() returned error: %v", err)
	}

	// Check that expiration time is approximately 1 hour from now
	expectedExpiration := time.Now().Add(3600 * time.Second)
	tolerance := 5 * time.Second
	if expirationTime.Before(expectedExpiration.Add(-tolerance)) || expirationTime.After(expectedExpiration.Add(tolerance)) {
		t.Errorf("calculateExpirationTime() = %v, want approximately %v", expirationTime, expectedExpiration)
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

	expirationTime, err := reconciler.calculateExpirationTime(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateExpirationTime() returned error: %v", err)
	}

	// Check that expiration time is approximately 3 weeks from now
	expectedExpiration := time.Now().Add(1814400 * time.Second)
	tolerance := 5 * time.Second
	if expirationTime.Before(expectedExpiration.Add(-tolerance)) || expirationTime.After(expectedExpiration.Add(tolerance)) {
		t.Errorf("calculateExpirationTime() = %v, want approximately %v", expirationTime, expectedExpiration)
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_MappedTTL_Default(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	// Add creation timestamp for proper TTL calculation
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": metav1.Now().Format(time.RFC3339),
			},
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

	expirationTime, err := reconciler.calculateExpirationTime(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateExpirationTime() returned error: %v", err)
	}

	// Check that expiration time is approximately 1 week from now (default)
	expectedExpiration := time.Now().Add(604800 * time.Second)
	tolerance := 5 * time.Second
	if expirationTime.Before(expectedExpiration.Add(-tolerance)) || expirationTime.After(expectedExpiration.Add(tolerance)) {
		t.Errorf("calculateExpirationTime() = %v, want approximately %v (default)", expirationTime, expectedExpiration)
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

	expirationTime, err := reconciler.calculateExpirationTime(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateExpirationTime() returned error: %v", err)
	}

	// Expiration time should be approximately 1 hour from now (2 hours after - 1 hour ago = 1 hour remaining)
	expectedExpiration := now.Add(3600 * time.Second)
	tolerance := 60 * time.Second // 1 minute tolerance

	if expirationTime.Before(expectedExpiration.Add(-tolerance)) || expirationTime.After(expectedExpiration.Add(tolerance)) {
		t.Errorf("calculateExpirationTime() = %v, want approximately %v (within %v)", expirationTime, expectedExpiration, tolerance)
	}
}

func TestGCPolicyReconciler_calculateExpirationTime_NoTTL(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}
	resource := &unstructured.Unstructured{}

	ttlSpec := &v1alpha1.TTLSpec{}

	_, err := reconciler.calculateExpirationTime(resource, ttlSpec)
	if err == nil {
		t.Error("calculateExpirationTime() should return error when no TTL is configured")
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

	_, err := reconciler.calculateExpirationTime(resource, ttlSpec)
	if err == nil {
		t.Error("calculateExpirationTime() should return error when field path is not found")
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
