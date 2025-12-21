package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

// PolicyGVR is the GroupVersionResource for GarbageCollectionPolicy CRDs
var PolicyGVR = schema.GroupVersionResource{
	Group:    "gc.k8s.io",
	Version:  "v1alpha1",
	Resource: "garbagecollectionpolicies",
}

// StatusUpdater updates GarbageCollectionPolicy CRD status subresource
type StatusUpdater struct {
	dynClient dynamic.Interface
}

// NewStatusUpdater creates a new status updater
func NewStatusUpdater(dynClient dynamic.Interface) *StatusUpdater {
	return &StatusUpdater{
		dynClient: dynClient,
	}
}

// UpdateStatus updates the GarbageCollectionPolicy CRD status subresource
func (s *StatusUpdater) UpdateStatus(
	ctx context.Context,
	policy *v1alpha1.GarbageCollectionPolicy,
	matched, deleted, pending int64,
) error {
	// Get the current policy CRD
	unstructuredPolicy, err := s.dynClient.Resource(PolicyGVR).
		Namespace(policy.Namespace).
		Get(ctx, policy.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get GarbageCollectionPolicy CRD: %w", err)
	}

	// Build status object
	now := metav1.Now()
	nextRun := metav1.NewTime(now.Add(DefaultGCInterval))

	statusObj := map[string]interface{}{
		"resourcesMatched": matched,
		"resourcesDeleted": deleted,
		"resourcesPending": pending,
		"lastGCRun":        now.Format(time.RFC3339),
		"nextGCRun":        nextRun.Format(time.RFC3339),
	}

	// Set phase if not set
	phase := policy.Status.Phase
	if phase == "" {
		phase = "Active"
	}
	statusObj["phase"] = phase

	// Merge status (preserve existing fields, update only provided fields)
	if existingStatus, ok := unstructuredPolicy.Object["status"].(map[string]interface{}); ok {
		// Merge: update provided fields, keep others
		for k, v := range statusObj {
			existingStatus[k] = v
		}
		unstructuredPolicy.Object["status"] = existingStatus
	} else {
		// No existing status, set new status
		unstructuredPolicy.Object["status"] = statusObj
	}

	// Update status subresource
	_, err = s.dynClient.Resource(PolicyGVR).
		Namespace(policy.Namespace).
		UpdateStatus(ctx, unstructuredPolicy, metav1.UpdateOptions{})
	if err != nil {
		klog.Warningf("Failed to update GarbageCollectionPolicy status: %v", err)
		return fmt.Errorf("failed to update GarbageCollectionPolicy status: %w", err)
	}

	klog.V(4).Infof("Updated GarbageCollectionPolicy status: %s/%s (matched=%d, deleted=%d, pending=%d)",
		policy.Namespace, policy.Name, matched, deleted, pending)

	return nil
}
