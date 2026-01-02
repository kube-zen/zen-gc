package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

func TestGCPolicyReconciler_shouldDelete_TTLExpired(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}

	// Create a resource that was created 2 hours ago
	creationTime := metav1.NewTime(time.Now().Add(-2 * time.Hour))
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": creationTime.Format(time.RFC3339),
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600), // 1 hour TTL
			},
		},
	}

	shouldDelete, reason := reconciler.shouldDelete(resource, policy)
	if !shouldDelete {
		t.Errorf("shouldDelete() = false, want true (resource is expired)")
	}
	if reason != ReasonTTLExpired {
		t.Errorf("shouldDelete() reason = %q, want %q", reason, ReasonTTLExpired)
	}
}

func TestGCPolicyReconciler_shouldDelete_TTLNotExpired(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}

	// Create a resource that was created 30 minutes ago
	creationTime := metav1.NewTime(time.Now().Add(-30 * time.Minute))
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": creationTime.Format(time.RFC3339),
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600), // 1 hour TTL
			},
		},
	}

	shouldDelete, reason := reconciler.shouldDelete(resource, policy)
	if shouldDelete {
		t.Errorf("shouldDelete() = true, want false (resource is not expired)")
	}
	if reason != "not_expired" {
		t.Errorf("shouldDelete() reason = %q, want 'not_expired'", reason)
	}
}

func TestGCPolicyReconciler_shouldDelete_ConditionNotMet(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}

	// Create an expired resource
	creationTime := metav1.NewTime(time.Now().Add(-2 * time.Hour))
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": creationTime.Format(time.RFC3339),
			},
			"status": map[string]interface{}{
				"phase": "Pending", // Not Processed
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600), // 1 hour TTL
			},
			Conditions: &v1alpha1.ConditionsSpec{
				Phase: []string{"Processed"}, // Only delete Processed resources
			},
		},
	}

	shouldDelete, reason := reconciler.shouldDelete(resource, policy)
	if shouldDelete {
		t.Errorf("shouldDelete() = true, want false (condition not met)")
	}
	if reason != ReasonConditionNotMet {
		t.Errorf("shouldDelete() reason = %q, want %q", reason, ReasonConditionNotMet)
	}
}

func TestGCPolicyReconciler_shouldDelete_NoTTL(t *testing.T) {
	reconciler := &GCPolicyReconciler{
		logger: sdklog.NewLogger("zen-gc"),
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TTL: v1alpha1.TTLSpec{}, // No TTL configured
		},
	}

	shouldDelete, reason := reconciler.shouldDelete(resource, policy)
	if shouldDelete {
		t.Errorf("shouldDelete() = true, want false (no TTL)")
	}
	if reason != ReasonNoTTL {
		t.Errorf("shouldDelete() reason = %q, want %q", reason, ReasonNoTTL)
	}
}
