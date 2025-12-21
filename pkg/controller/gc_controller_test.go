package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestNewGCController(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil) // nil is OK for tests

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	if controller == nil {
		t.Fatal("NewGCController() returned nil controller")
	}

	if controller.dynamicClient == nil {
		t.Error("NewGCController() did not set dynamicClient")
	}

	if controller.policyInformer == nil {
		t.Error("NewGCController() did not set policyInformer")
	}

	if controller.rateLimiter == nil {
		t.Error("NewGCController() did not set rateLimiter")
	}

	if controller.resourceInformers == nil {
		t.Error("NewGCController() did not initialize resourceInformers map")
	}
}

func TestGCController_Stop(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Stop should not panic
	controller.Stop()

	// Verify context is cancelled
	select {
	case <-controller.ctx.Done():
		// Expected
	default:
		t.Error("Stop() did not cancel context")
	}
}

func TestGCController_evaluatePolicies_Empty(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Create a mock informer with empty store
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute)
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.k8s.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	controller.policyInformer = factory.ForResource(policyGVR).Informer()

	// Should not error with empty policies
	err = controller.evaluatePolicies()
	if err != nil {
		t.Errorf("evaluatePolicies() returned error: %v", err)
	}
}

func TestGCController_evaluatePolicies_WithPausedPolicy(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Create a paused policy
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600),
			},
		},
		Status: v1alpha1.GarbageCollectionPolicyStatus{
			Phase: "Paused",
		},
	}

	// Convert to unstructured
	unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy to unstructured: %v", err)
	}

	// Create mock informer with paused policy
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute)
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.k8s.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	controller.policyInformer = factory.ForResource(policyGVR).Informer()

	// Add policy to store
	unstructuredObj := &unstructured.Unstructured{Object: unstructuredPolicy}
	controller.policyInformer.GetStore().Add(unstructuredObj)

	// Should skip paused policy
	err = controller.evaluatePolicies()
	if err != nil {
		t.Errorf("evaluatePolicies() returned error: %v", err)
	}
}

func TestGCController_deleteResource_DryRun(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-configmap",
				"namespace": "default",
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				DryRun: true,
			},
		},
	}

	// Dry run should not actually delete
	err = controller.deleteResource(resource, policy)
	if err != nil {
		t.Errorf("deleteResource() returned error: %v", err)
	}
}

func TestGCController_deleteResource_WithGracePeriod(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-configmap",
				"namespace": "default",
			},
		},
	}

	gracePeriod := int64(30)
	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				DryRun:             false,
				GracePeriodSeconds: &gracePeriod,
				PropagationPolicy:  "Foreground",
			},
		},
	}

	// Create resource in fake client
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	_, err = dynamicClient.Resource(gvr).Namespace("default").Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	// Delete should succeed
	err = controller.deleteResource(resource, policy)
	if err != nil {
		t.Errorf("deleteResource() returned error: %v", err)
	}
}

func TestGCController_deleteResource_ClusterScoped(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": "test-namespace",
			},
		},
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				DryRun: false,
			},
		},
	}

	// Create cluster-scoped resource
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	_, err = dynamicClient.Resource(gvr).Create(context.Background(), resource, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	// Delete should succeed
	err = controller.deleteResource(resource, policy)
	if err != nil {
		t.Errorf("deleteResource() returned error: %v", err)
	}
}

func TestGCController_updatePolicyStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
	}

	// Should not error (currently just logs)
	err = controller.updatePolicyStatus(policy, 10, 5, 5)
	if err != nil {
		t.Errorf("updatePolicyStatus() returned error: %v", err)
	}
}

func TestGCController_calculateTTL_FieldPathInt64(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"ttlSeconds": int64(7200),
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		FieldPath: "spec.ttlSeconds",
	}

	ttl, err := gc.calculateTTL(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateTTL() returned error: %v", err)
	}

	if ttl != 7200 {
		t.Errorf("calculateTTL() = %d, want 7200", ttl)
	}
}

func TestGCController_calculateTTL_RelativeToExpired(t *testing.T) {
	gc := &GCController{}
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"lastProcessedAt": twoHoursAgo.Format(time.RFC3339),
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		RelativeTo:   "status.lastProcessedAt",
		SecondsAfter: int64Ptr(3600), // 1 hour after
	}

	// Should return error because TTL already expired
	_, err := gc.calculateTTL(resource, ttlSpec)
	if err == nil {
		t.Error("calculateTTL() should return error when relative TTL already expired")
	}
}

func TestGCController_calculateTTL_RelativeToInvalidTimestamp(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"lastProcessedAt": "invalid-timestamp",
			},
		},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		RelativeTo:   "status.lastProcessedAt",
		SecondsAfter: int64Ptr(3600),
	}

	_, err := gc.calculateTTL(resource, ttlSpec)
	if err == nil {
		t.Error("calculateTTL() should return error for invalid timestamp format")
	}
}

func TestGCController_calculateTTL_MappedTTL_NoMatchNoDefault(t *testing.T) {
	gc := &GCController{}
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
		// No default
	}

	_, err := gc.calculateTTL(resource, ttlSpec)
	if err == nil {
		t.Error("calculateTTL() should return error when no mapping matches and no default")
	}
}

func TestGCController_calculateTTL_FieldPathNotFoundNoDefault(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		FieldPath: "spec.nonexistent",
		// No default
	}

	_, err := gc.calculateTTL(resource, ttlSpec)
	if err == nil {
		t.Error("calculateTTL() should return error when field path not found and no default")
	}
}

func TestGCController_calculateTTL_FieldPathWithDefault(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	ttlSpec := &v1alpha1.TTLSpec{
		FieldPath: "spec.nonexistent",
		Default:   int64Ptr(3600),
	}

	ttl, err := gc.calculateTTL(resource, ttlSpec)
	if err != nil {
		t.Fatalf("calculateTTL() returned error: %v", err)
	}

	if ttl != 3600 {
		t.Errorf("calculateTTL() = %d, want 3600 (default)", ttl)
	}
}

func TestGCController_meetsConditions_PhaseNotFound(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	conditions := &v1alpha1.ConditionsSpec{
		Phase: []string{"Processed"},
	}

	result := gc.meetsConditions(resource, conditions)
	if result {
		t.Error("meetsConditions() should return false when phase field not found")
	}
}

func TestGCController_meetsConditions_LabelNotExists(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{},
			},
		},
	}

	conditions := &v1alpha1.ConditionsSpec{
		HasLabels: []v1alpha1.LabelCondition{
			{Key: "processed", Value: "true"},
		},
	}

	result := gc.meetsConditions(resource, conditions)
	if result {
		t.Error("meetsConditions() should return false when label does not exist")
	}
}

func TestGCController_meetsConditions_AnnotationNotExists(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{},
			},
		},
	}

	conditions := &v1alpha1.ConditionsSpec{
		HasAnnotations: []v1alpha1.AnnotationCondition{
			{Key: "cleanup-allowed", Value: "true"},
		},
	}

	result := gc.meetsConditions(resource, conditions)
	if result {
		t.Error("meetsConditions() should return false when annotation does not exist")
	}
}

func TestGCController_meetsConditions_FieldConditionNotFound(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	conditions := &v1alpha1.ConditionsSpec{
		And: []v1alpha1.FieldCondition{
			{FieldPath: "spec.nonexistent", Operator: "Equals", Value: "value"},
		},
	}

	result := gc.meetsConditions(resource, conditions)
	if result {
		t.Error("meetsConditions() should return false when field not found")
	}
}

func TestGCController_matchesSelectors_InvalidLabelSelector(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{},
			},
		},
	}

	// Create an invalid label selector (this would normally be caught by validation)
	// But we test the error handling in matchesSelectors
	target := &v1alpha1.TargetResourceSpec{
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: "InvalidOperator",
					Values:   []string{"test"},
				},
			},
		},
	}

	// Should return false due to invalid selector
	result := gc.matchesSelectors(resource, target)
	if result {
		t.Error("matchesSelectors() should return false for invalid label selector")
	}
}

func TestGCController_matchesSelectors_FieldSelectorNotFound(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	target := &v1alpha1.TargetResourceSpec{
		FieldSelector: &v1alpha1.FieldSelectorSpec{
			MatchFields: map[string]string{
				"metadata.nonexistent": "value",
			},
		},
	}

	result := gc.matchesSelectors(resource, target)
	if result {
		t.Error("matchesSelectors() should return false when field not found")
	}
}

func TestGCController_matchesSelectors_FieldSelectorValueMismatch(t *testing.T) {
	gc := &GCController{}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"namespace": "default",
			},
		},
	}

	target := &v1alpha1.TargetResourceSpec{
		FieldSelector: &v1alpha1.FieldSelectorSpec{
			MatchFields: map[string]string{
				"metadata.namespace": "zen-system",
			},
		},
	}

	result := gc.matchesSelectors(resource, target)
	if result {
		t.Error("matchesSelectors() should return false when field value doesn't match")
	}
}
