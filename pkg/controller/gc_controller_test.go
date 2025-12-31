package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
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

	if controller.rateLimiters == nil {
		t.Error("NewGCController() did not initialize rateLimiters map")
	}

	if controller.resourceInformers == nil {
		t.Error("NewGCController() did not initialize resourceInformers map")
	}

	if controller.resourceInformerFactories == nil {
		t.Error("NewGCController() did not initialize resourceInformerFactories map")
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

	// Verify context is canceled.
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
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	controller.policyInformer = factory.ForResource(policyGVR).Informer()

	// Should not error with empty policies
	controller.evaluatePolicies()
}

func TestGCController_evaluatePolicies_ContextCanceled(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Cancel context before evaluation
	controller.Stop()

	// Create a mock informer
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute)
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	controller.policyInformer = factory.ForResource(policyGVR).Informer()

	// Should return early when context is canceled
	controller.evaluatePolicies()
}

func TestGCController_evaluatePolicies_CacheNotSynced(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Create a mock informer that hasn't synced
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute)
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	controller.policyInformer = factory.ForResource(policyGVR).Informer()

	// Should return early when cache is not synced
	controller.evaluatePolicies()
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
			Phase: PolicyPhasePaused,
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
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	controller.policyInformer = factory.ForResource(policyGVR).Informer()

	// Add policy to store
	unstructuredObj := &unstructured.Unstructured{Object: unstructuredPolicy}
	controller.policyInformer.GetStore().Add(unstructuredObj)

	// Should skip paused policy
	controller.evaluatePolicies()
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

	// Get or create rate limiter for the policy
	rateLimiter := controller.getOrCreateRateLimiter(policy)

	// Dry run should not actually delete
	err = controller.deleteResource(resource, policy, rateLimiter)
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
				PropagationPolicy:  PropagationPolicyForeground,
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

	// Get or create rate limiter for the policy
	rateLimiter := controller.getOrCreateRateLimiter(policy)

	// Delete should succeed
	err = controller.deleteResource(resource, policy, rateLimiter)
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

	// Get or create rate limiter for the policy
	rateLimiter := controller.getOrCreateRateLimiter(policy)

	// Delete should succeed
	err = controller.deleteResource(resource, policy, rateLimiter)
	if err != nil {
		t.Errorf("deleteResource() returned error: %v", err)
	}
}

func TestGCController_StatusUpdater(t *testing.T) {
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
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			TargetResource: v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			TTL: v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600),
			},
		},
	}

	// Create policy in fake client first
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}
	unstructuredPolicy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		t.Fatalf("Failed to convert policy to unstructured: %v", err)
	}
	_, err = dynamicClient.Resource(policyGVR).Namespace("default").Create(context.Background(), &unstructured.Unstructured{Object: unstructuredPolicy}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Test status updater directly (replaces deprecated updatePolicyStatus)
	err = controller.statusUpdater.UpdateStatus(context.Background(), policy, 10, 5, 5)
	if err != nil {
		t.Errorf("StatusUpdater.UpdateStatus() returned error: %v", err)
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

func TestGCController_cleanupResourceInformer(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Create a test policy UID
	policyUID := types.UID("test-policy-uid")

	// Initially, no informer should exist
	controller.resourceInformersMu.RLock()
	_, exists := controller.resourceInformers[policyUID]
	controller.resourceInformersMu.RUnlock()
	if exists {
		t.Error("Resource informer should not exist initially")
	}

	// Cleanup should not panic on non-existent informer
	controller.cleanupResourceInformer(policyUID)

	// Add a mock informer
	controller.resourceInformersMu.Lock()
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute)
	gvr := schema.GroupVersionResource{
		Group:    "v1",
		Version:  "v1",
		Resource: "configmaps",
	}
	informer := factory.ForResource(gvr).Informer()
	controller.resourceInformers[policyUID] = informer
	controller.resourceInformerFactories[policyUID] = factory
	controller.resourceInformersMu.Unlock()

	// Verify informer exists
	controller.resourceInformersMu.RLock()
	_, exists = controller.resourceInformers[policyUID]
	factoryExists := controller.resourceInformerFactories[policyUID] != nil
	controller.resourceInformersMu.RUnlock()
	if !exists {
		t.Error("Resource informer should exist after creation")
	}
	if !factoryExists {
		t.Error("Resource informer factory should exist after creation")
	}

	// Cleanup the informer
	controller.cleanupResourceInformer(policyUID)

	// Verify informer is cleaned up
	controller.resourceInformersMu.RLock()
	_, exists = controller.resourceInformers[policyUID]
	factoryExists = controller.resourceInformerFactories[policyUID] != nil
	controller.resourceInformersMu.RUnlock()
	if exists {
		t.Error("Resource informer should be cleaned up")
	}
	if factoryExists {
		t.Error("Resource informer factory should be cleaned up")
	}
}

func TestGCController_cleanupAllResourceInformers(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Add multiple mock informers
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, time.Minute)
	gvr := schema.GroupVersionResource{
		Group:    "v1",
		Version:  "v1",
		Resource: "configmaps",
	}
	informer := factory.ForResource(gvr).Informer()

	controller.resourceInformersMu.Lock()
	controller.resourceInformers[types.UID("uid1")] = informer
	controller.resourceInformers[types.UID("uid2")] = informer
	controller.resourceInformerFactories[types.UID("uid1")] = factory
	controller.resourceInformerFactories[types.UID("uid2")] = factory
	controller.resourceInformersMu.Unlock()

	// Verify informers exist
	controller.resourceInformersMu.RLock()
	count := len(controller.resourceInformers)
	controller.resourceInformersMu.RUnlock()
	if count != 2 {
		t.Errorf("Expected 2 informers, got %d", count)
	}

	// Cleanup all
	controller.cleanupAllResourceInformers()

	// Verify all are cleaned up
	controller.resourceInformersMu.RLock()
	count = len(controller.resourceInformers)
	factoryCount := len(controller.resourceInformerFactories)
	controller.resourceInformersMu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 informers after cleanup, got %d", count)
	}
	if factoryCount != 0 {
		t.Errorf("Expected 0 factories after cleanup, got %d", factoryCount)
	}
}

func TestGCController_getOrCreateRateLimiter(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Create a test policy with custom rate limit
	customRate := int(20)
	policy := &v1alpha1.GarbageCollectionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("test-policy-uid"),
			Namespace: "default",
			Name:      "test-policy",
		},
		Spec: v1alpha1.GarbageCollectionPolicySpec{
			Behavior: v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: customRate,
			},
		},
	}

	// Get rate limiter - should create new one
	rateLimiter1 := controller.getOrCreateRateLimiter(policy)
	if rateLimiter1 == nil {
		t.Fatal("getOrCreateRateLimiter() returned nil")
	}

	// Get again - should return same instance
	rateLimiter2 := controller.getOrCreateRateLimiter(policy)
	if rateLimiter1 != rateLimiter2 {
		t.Error("getOrCreateRateLimiter() should return same instance for same policy")
	}

	// Verify rate limiter exists in map
	controller.rateLimitersMu.RLock()
	_, exists := controller.rateLimiters[policy.UID]
	controller.rateLimitersMu.RUnlock()
	if !exists {
		t.Error("Rate limiter should exist in map")
	}
}

func TestGCController_cleanupRateLimiter(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Create a test policy UID
	policyUID := types.UID("test-policy-uid")

	// Initially, no rate limiter should exist
	controller.rateLimitersMu.RLock()
	_, exists := controller.rateLimiters[policyUID]
	controller.rateLimitersMu.RUnlock()
	if exists {
		t.Error("Rate limiter should not exist initially")
	}

	// Cleanup should not panic on non-existent rate limiter
	controller.cleanupRateLimiter(policyUID)

	// Add a rate limiter
	controller.rateLimitersMu.Lock()
	controller.rateLimiters[policyUID] = ratelimiter.NewRateLimiter(10)
	controller.rateLimitersMu.Unlock()

	// Verify rate limiter exists
	controller.rateLimitersMu.RLock()
	_, exists = controller.rateLimiters[policyUID]
	controller.rateLimitersMu.RUnlock()
	if !exists {
		t.Error("Rate limiter should exist after creation")
	}

	// Cleanup the rate limiter
	controller.cleanupRateLimiter(policyUID)

	// Verify rate limiter is cleaned up
	controller.rateLimitersMu.RLock()
	_, exists = controller.rateLimiters[policyUID]
	controller.rateLimitersMu.RUnlock()
	if exists {
		t.Error("Rate limiter should be cleaned up")
	}
}

func TestGCController_cleanupAllRateLimiters(t *testing.T) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		t.Fatalf("NewGCController() returned error: %v", err)
	}

	// Add multiple rate limiters
	controller.rateLimitersMu.Lock()
	controller.rateLimiters[types.UID("uid1")] = ratelimiter.NewRateLimiter(10)
	controller.rateLimiters[types.UID("uid2")] = ratelimiter.NewRateLimiter(20)
	controller.rateLimitersMu.Unlock()

	// Verify rate limiters exist
	controller.rateLimitersMu.RLock()
	count := len(controller.rateLimiters)
	controller.rateLimitersMu.RUnlock()
	if count != 2 {
		t.Errorf("Expected 2 rate limiters, got %d", count)
	}

	// Cleanup all
	controller.cleanupAllRateLimiters()

	// Verify all are cleaned up
	controller.rateLimitersMu.RLock()
	count = len(controller.rateLimiters)
	controller.rateLimitersMu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 rate limiters after cleanup, got %d", count)
	}
}
