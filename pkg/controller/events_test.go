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
	if recorder.recorder == nil {
		t.Fatal("EventRecorder.recorder is nil")
	}
}

func TestNewEventRecorder_NilClient(t *testing.T) {
	recorder := NewEventRecorder(nil)
	if recorder == nil {
		t.Fatal("NewEventRecorder returned nil")
	}
	if recorder.recorder == nil {
		t.Fatal("EventRecorder.recorder is nil")
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
	recorder.RecordResourceDeleted(policy, resource, "ttl_expired")
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
	name := getResourceName(resource)
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
	name := getResourceName(obj)
	// Unstructured.GetName() returns empty string if metadata.name is not set
	if name != "" {
		t.Errorf("Expected empty string for object without name, got '%s'", name)
	}
}

func TestEventSinkWrapper_Create(t *testing.T) {
	client := fake.NewSimpleClientset()
	wrapper := &eventSinkWrapper{
		events: client.CoreV1().Events("default"),
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
		},
		Reason:  "TestReason",
		Message: "Test message",
	}

	created, err := wrapper.Create(event)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if created == nil {
		t.Fatal("Create() returned nil event")
	}

	if created.Name != "test-event" {
		t.Errorf("Create() event name = %v, want %v", created.Name, "test-event")
	}
}

func TestEventSinkWrapper_Update(t *testing.T) {
	client := fake.NewSimpleClientset()
	wrapper := &eventSinkWrapper{
		events: client.CoreV1().Events("default"),
	}

	// Create event first
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
		},
		Reason:  "TestReason",
		Message: "Test message",
	}

	created, err := client.CoreV1().Events("default").Create(context.Background(), event, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Update event
	created.Message = "Updated message"
	updated, err := wrapper.Update(created)
	if err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}

	if updated == nil {
		t.Fatal("Update() returned nil event")
	}

	if updated.Message != "Updated message" {
		t.Errorf("Update() event message = %v, want %v", updated.Message, "Updated message")
	}
}

func TestEventSinkWrapper_Patch(t *testing.T) {
	client := fake.NewSimpleClientset()
	wrapper := &eventSinkWrapper{
		events: client.CoreV1().Events("default"),
	}

	// Create event first
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-event",
			Namespace: "default",
		},
		Reason:  "TestReason",
		Message: "Test message",
	}

	created, err := client.CoreV1().Events("default").Create(context.Background(), event, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Patch event
	patch := []byte(`{"message":"Patched message"}`)
	patched, err := wrapper.Patch(created, patch)
	if err != nil {
		t.Fatalf("Patch() returned error: %v", err)
	}

	if patched == nil {
		t.Fatal("Patch() returned nil event")
	}

	if patched.Message != "Patched message" {
		t.Errorf("Patch() event message = %v, want %v", patched.Message, "Patched message")
	}
}
