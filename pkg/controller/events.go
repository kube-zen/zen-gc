package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

// eventSinkWrapper wraps EventInterface to implement record.EventSink.
type eventSinkWrapper struct {
	events v1.EventInterface
}

func (e *eventSinkWrapper) Create(event *corev1.Event) (*corev1.Event, error) {
	return e.events.Create(context.Background(), event, metav1.CreateOptions{})
}

func (e *eventSinkWrapper) Update(event *corev1.Event) (*corev1.Event, error) {
	return e.events.Update(context.Background(), event, metav1.UpdateOptions{})
}

func (e *eventSinkWrapper) Patch(oldEvent *corev1.Event, data []byte) (*corev1.Event, error) {
	return e.events.Patch(context.Background(), oldEvent.Name, types.MergePatchType, data, metav1.PatchOptions{})
}

// EventRecorder wraps Kubernetes event recorder for GC controller.
type EventRecorder struct {
	recorder record.EventRecorder
}

// NewEventRecorder creates a new event recorder.
func NewEventRecorder(client kubernetes.Interface) *EventRecorder {
	// Create event broadcaster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	if client != nil {
		eventBroadcaster.StartRecordingToSink(&eventSinkWrapper{
			events: client.CoreV1().Events(""),
		})
	}

	// Create event recorder
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: "gc-controller",
	})

	return &EventRecorder{
		recorder: eventRecorder,
	}
}

// RecordPolicyEvaluated records that a policy was evaluated.
func (er *EventRecorder) RecordPolicyEvaluated(
	policy *v1alpha1.GarbageCollectionPolicy,
	matched, deleted, pending int64,
) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyEvaluated",
		"Evaluated policy: matched=%d, deleted=%d, pending=%d",
		matched, deleted, pending,
	)
}

// RecordResourceDeleted records that a resource was deleted.
func (er *EventRecorder) RecordResourceDeleted(
	policy *v1alpha1.GarbageCollectionPolicy,
	resource runtime.Object,
	reason string,
) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeNormal,
		"ResourceDeleted",
		"Deleted resource %s (reason: %s)",
		getResourceName(resource), reason,
	)
}

// RecordEvaluationFailed records that policy evaluation failed.
func (er *EventRecorder) RecordEvaluationFailed(
	policy *v1alpha1.GarbageCollectionPolicy,
	err error,
) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeWarning,
		"EvaluationFailed",
		"Failed to evaluate policy: %v",
		err,
	)
}

// RecordStatusUpdateFailed records that status update failed.
func (er *EventRecorder) RecordStatusUpdateFailed(
	policy *v1alpha1.GarbageCollectionPolicy,
	err error,
) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeWarning,
		"StatusUpdateFailed",
		"Failed to update policy status: %v",
		err,
	)
}

// getResourceName extracts resource name from runtime.Object.
func getResourceName(obj runtime.Object) string {
	if metaObj, ok := obj.(interface{ GetName() string }); ok {
		return metaObj.GetName()
	}
	return "unknown"
}

// RecordPolicyCreated records that a policy was created.
func (er *EventRecorder) RecordPolicyCreated(policy *v1alpha1.GarbageCollectionPolicy) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyCreated",
		"GarbageCollectionPolicy created",
	)
}

// RecordPolicyUpdated records that a policy was updated.
func (er *EventRecorder) RecordPolicyUpdated(policy *v1alpha1.GarbageCollectionPolicy) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyUpdated",
		"GarbageCollectionPolicy updated",
	)
}

// RecordPolicyDeleted records that a policy was deleted.
func (er *EventRecorder) RecordPolicyDeleted(policy *v1alpha1.GarbageCollectionPolicy) {
	er.recorder.Eventf(
		policy,
		corev1.EventTypeNormal,
		"PolicyDeleted",
		"GarbageCollectionPolicy deleted",
	)
}
