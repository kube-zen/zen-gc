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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

// TestGCPolicyReconciler_recordPolicyPhaseMetrics tests the recordPolicyPhaseMetrics function.
func TestGCPolicyReconciler_recordPolicyPhaseMetrics(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("Failed to add scheme: %v", err)
	}

	fakeClient := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := NewEventRecorder(kubeClient)

	reconciler := NewGCPolicyReconcilerWithRESTMapper(
		fakeClient,
		scheme,
		dynamicClient,
		nil,
		statusUpdater,
		eventRecorder,
		config.NewControllerConfig(),
	)

	ctx := context.Background()

	// Test with no policies
	reconciler.recordPolicyPhaseMetrics(ctx)

	// Test with active policy
	policy1 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy1",
			Namespace: "default",
			UID:       types.UID("uid-1"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: PolicyPhaseActive,
		},
	}
	if err := fakeClient.Create(ctx, policy1); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
	reconciler.recordPolicyPhaseMetrics(ctx)

	// Test with paused policy
	policy2 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy2",
			Namespace: "default",
			UID:       types.UID("uid-2"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Paused: true,
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: PolicyPhasePaused,
		},
	}
	if err := fakeClient.Create(ctx, policy2); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
	reconciler.recordPolicyPhaseMetrics(ctx)

	// Test with error policy
	policy3 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy3",
			Namespace: "default",
			UID:       types.UID("uid-3"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: PolicyPhaseError,
		},
	}
	if err := fakeClient.Create(ctx, policy3); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
	reconciler.recordPolicyPhaseMetrics(ctx)

	// Test with multiple policies of same phase
	policy4 := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy4",
			Namespace: "default",
			UID:       types.UID("uid-4"),
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: PolicyPhaseActive,
		},
	}
	if err := fakeClient.Create(ctx, policy4); err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
	reconciler.recordPolicyPhaseMetrics(ctx)

	// Test passes if no panic occurs
}
