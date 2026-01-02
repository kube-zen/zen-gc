package controller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	sdkevents "github.com/kube-zen/zen-sdk/pkg/events"
)

// EventRecorder wraps Kubernetes event recorder for GC controller.
// This now uses zen-sdk/pkg/events as the base implementation.
type EventRecorder struct {
	*sdkevents.Recorder
}

// NewEventRecorder creates a new event recorder.
func NewEventRecorder(client kubernetes.Interface) *EventRecorder {
	return &EventRecorder{
		Recorder: sdkevents.NewRecorder(client, "gc-controller"),
	}
}

// RecordPolicyEvaluated records that a policy was evaluated.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordPolicyEvaluated(
	policy *v1alpha1.GarbageCollectionPolicy,
	matched, deleted, pending int64,
) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyEvaluated",
		"Evaluated policy: matched=%d, deleted=%d, pending=%d",
		matched, deleted, pending,
	)
}

// RecordResourceDeleted records that a resource was deleted.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordResourceDeleted(
	policy *v1alpha1.GarbageCollectionPolicy,
	resource runtime.Object,
	reason string,
) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeNormal,
		"ResourceDeleted",
		"Deleted resource %s (reason: %s)",
		sdkevents.GetResourceName(resource), reason,
	)
}

// RecordEvaluationFailed records that policy evaluation failed.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordEvaluationFailed(
	policy *v1alpha1.GarbageCollectionPolicy,
	err error,
) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeWarning,
		"EvaluationFailed",
		"Failed to evaluate policy: %v",
		err,
	)
}

// RecordStatusUpdateFailed records that status update failed.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordStatusUpdateFailed(
	policy *v1alpha1.GarbageCollectionPolicy,
	err error,
) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeWarning,
		"StatusUpdateFailed",
		"Failed to update policy status: %v",
		err,
	)
}

// RecordPolicyCreated records that a policy was created.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordPolicyCreated(policy *v1alpha1.GarbageCollectionPolicy) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyCreated",
		"GarbageCollectionPolicy created",
	)
}

// RecordPolicyUpdated records that a policy was updated.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordPolicyUpdated(policy *v1alpha1.GarbageCollectionPolicy) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyUpdated",
		"GarbageCollectionPolicy updated",
	)
}

// RecordPolicyDeleted records that a policy was deleted.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (er *EventRecorder) RecordPolicyDeleted(policy *v1alpha1.GarbageCollectionPolicy) {
	if er == nil || er.Recorder == nil {
		return
	}
	// Event recording for CRDs may fail - log but don't fail
	er.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyDeleted",
		"GarbageCollectionPolicy deleted",
	)
}
