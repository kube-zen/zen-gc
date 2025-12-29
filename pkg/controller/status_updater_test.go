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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
)

func TestNewStatusUpdaterWithConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	cfg := config.NewControllerConfig()

	updater := NewStatusUpdaterWithConfig(dynamicClient, cfg)
	if updater == nil {
		t.Fatal("NewStatusUpdaterWithConfig returned nil")
	}
	if updater.dynClient != dynamicClient {
		t.Error("StatusUpdater.dynClient not set correctly")
	}
	if updater.config != cfg {
		t.Error("StatusUpdater.config not set correctly")
	}
}

func TestNewStatusUpdaterWithConfig_NilConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)

	updater := NewStatusUpdaterWithConfig(dynamicClient, nil)
	if updater == nil {
		t.Fatal("NewStatusUpdaterWithConfig returned nil")
	}
	if updater.config == nil {
		t.Error("StatusUpdater.config should be initialized when nil is passed")
	}
}

func TestStatusUpdater_UpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	updater := NewStatusUpdater(dynamicClient)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
	}

	// Create the policy in the fake client
	gvr := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy to unstructured: %v", err)
	}
	_, err = dynamicClient.Resource(gvr).Namespace("default").Create(
		context.Background(),
		&unstructured.Unstructured{Object: unstructuredPolicy},
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	ctx := context.Background()
	err = updater.UpdateStatus(ctx, policy, 10, 5, 3)
	if err != nil {
		t.Errorf("UpdateStatus() returned error: %v", err)
	}
}

func TestStatusUpdater_UpdateStatus_WithExistingStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	updater := NewStatusUpdater(dynamicClient)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: PolicyPhasePaused, // Existing phase
		},
	}

	// Create the policy in the fake client with existing status
	gvr := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy to unstructured: %v", err)
	}
	unstructuredPolicy["status"] = map[string]interface{}{
		"phase":         PolicyPhasePaused,
		"existingField": "should be preserved",
	}
	_, err = dynamicClient.Resource(gvr).Namespace("default").Create(
		context.Background(),
		&unstructured.Unstructured{Object: unstructuredPolicy},
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	ctx := context.Background()
	err = updater.UpdateStatus(ctx, policy, 10, 5, 3)
	if err != nil {
		t.Errorf("UpdateStatus() returned error: %v", err)
	}
}

func TestStatusUpdater_UpdateStatus_WithConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	cfg := config.NewControllerConfig().WithGCInterval(2 * time.Minute)
	updater := NewStatusUpdaterWithConfig(dynamicClient, cfg)

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
		},
	}

	// Create the policy in the fake client
	gvr := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy to unstructured: %v", err)
	}
	_, err = dynamicClient.Resource(gvr).Namespace("default").Create(
		context.Background(),
		&unstructured.Unstructured{Object: unstructuredPolicy},
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	ctx := context.Background()
	err = updater.UpdateStatus(ctx, policy, 10, 5, 3)
	if err != nil {
		t.Errorf("UpdateStatus() returned error: %v", err)
	}
}
