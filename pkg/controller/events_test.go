/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	sdkevents "github.com/kube-zen/zen-sdk/pkg/events"
)

var (
	errTestError         = errors.New("test error")
	errStatusUpdateError = errors.New("status update error")
)

func TestNewEventRecorder(t *testing.T) {
	client := fake.NewSimpleClientset()
	recorder := NewEventRecorder(client)
	if recorder == nil {
		t.Fatal("NewEventRecorder returned nil")
	}
	if recorder.Recorder == nil {
		t.Fatal("EventRecorder.Recorder is nil")
	}
}

func TestNewEventRecorder_NilClient(t *testing.T) {
	recorder := NewEventRecorder(nil)
	if recorder == nil {
		t.Fatal("NewEventRecorder returned nil")
	}
	if recorder.Recorder == nil {
		t.Fatal("EventRecorder.Recorder is nil")
	}
}

func TestEventRecorder_RecordPolicyEvaluated(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.RecordPolicyEvaluated(policy, 10, 5, 3)
}

func TestEventRecorder_RecordResourceDeleted(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
		},
	}
	// Should not panic
	recorder.RecordResourceDeleted(policy, resource, ReasonTTLExpired)
}

func TestEventRecorder_RecordEvaluationFailed(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.RecordEvaluationFailed(policy, fmt.Errorf("wrapped: %w", errTestError))
}

func TestEventRecorder_RecordStatusUpdateFailed(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.RecordStatusUpdateFailed(policy, fmt.Errorf("wrapped: %w", errStatusUpdateError))
}

func TestEventRecorder_RecordPolicyCreated(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.RecordPolicyCreated(policy)
}

func TestEventRecorder_RecordPolicyUpdated(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.RecordPolicyUpdated(policy)
}

func TestEventRecorder_RecordPolicyDeleted(t *testing.T) {
	recorder := NewEventRecorder(nil)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.RecordPolicyDeleted(policy)
}

func TestGetResourceName(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-resource",
			},
		},
	}
	name := sdkevents.GetResourceName(resource)
	if name != "test-resource" {
		t.Errorf("Expected 'test-resource', got '%s'", name)
	}
}

func TestGetResourceName_Unknown(t *testing.T) {
	// Create a mock object that implements runtime.Object but not GetName()
	// We'll use a custom type that implements runtime.Object but not the GetName() method
	// Since unstructured.Unstructured does implement GetName(), we need to test differently
	// The function checks for GetName() method, so if it doesn't exist, it returns "unknown"
	// However, since we can't easily create a runtime.Object without GetName() in tests,
	// we'll test with an object that has an empty name (which is the practical case)
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{
				// No name field - GetName() will return empty string
			},
		},
	}
	name := sdkevents.GetResourceName(obj)
	// Unstructured.GetName() returns empty string if metadata.name is not set
	if name != "" {
		t.Errorf("Expected empty string for object without name, got '%s'", name)
	}
}

func TestEventSinkWrapper_Create(t *testing.T) {
	// eventSinkWrapper is now internal to zen-sdk/pkg/events
	// Test through the recorder instead
	client := fake.NewSimpleClientset()
	recorder := NewEventRecorder(client)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.Eventf(policy, corev1.EventTypeNormal, "TestReason", "Test message")
}

func TestEventSinkWrapper_Update(t *testing.T) {
	// eventSinkWrapper is now internal to zen-sdk/pkg/events
	// Test through the recorder instead
	client := fake.NewSimpleClientset()
	recorder := NewEventRecorder(client)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.Event(policy, corev1.EventTypeNormal, "TestReason", "Test message")
}

func TestEventSinkWrapper_Patch(t *testing.T) {
	// eventSinkWrapper is now internal to zen-sdk/pkg/events
	// Test through the recorder instead
	client := fake.NewSimpleClientset()
	recorder := NewEventRecorder(client)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}
	// Should not panic
	recorder.Eventf(policy, corev1.EventTypeNormal, "TestReason", "Test message with format: %s", "value")
}
