package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/validation"
)

const (
	// DefaultGCInterval is the default interval for GC runs
	DefaultGCInterval = 1 * time.Minute

	// DefaultMaxDeletionsPerSecond is the default rate limit
	DefaultMaxDeletionsPerSecond = 10

	// DefaultBatchSize is the default batch size for deletions
	DefaultBatchSize = 50
)

// GCController manages garbage collection policies
type GCController struct {
	dynamicClient dynamic.Interface

	// Policy informer factory
	policyInformerFactory dynamicinformer.DynamicSharedInformerFactory

	// Policy informer
	policyInformer cache.SharedInformer

	// Resource informers (one per policy)
	resourceInformers map[types.UID]cache.SharedInformer

	// Rate limiter
	rateLimiter *RateLimiter

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewGCController creates a new GC controller
func NewGCController(dynamicClient dynamic.Interface) (*GCController, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create policy GVR
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.k8s.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}

	// Create informer factory for policies
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, DefaultGCInterval)

	// Create policy informer
	policyInformer := factory.ForResource(policyGVR).Informer()

	return &GCController{
		dynamicClient:         dynamicClient,
		policyInformerFactory: factory,
		policyInformer:        policyInformer,
		resourceInformers:     make(map[types.UID]cache.SharedInformer),
		rateLimiter:           NewRateLimiter(DefaultMaxDeletionsPerSecond),
		ctx:                   ctx,
		cancel:                cancel,
	}, nil
}

// Start starts the GC controller
func (gc *GCController) Start() error {
	klog.Info("Starting GC controller")

	// Start policy informer factory
	gc.policyInformerFactory.Start(gc.ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(gc.ctx.Done(), gc.policyInformer.HasSynced) {
		return fmt.Errorf("failed to sync policy informer cache")
	}

	// Start GC loop
	go gc.runGCLoop()

	klog.Info("GC controller started")
	return nil
}

// Stop stops the GC controller
func (gc *GCController) Stop() {
	klog.Info("Stopping GC controller")
	gc.cancel()
}

// runGCLoop runs the main GC evaluation loop
func (gc *GCController) runGCLoop() {
	ticker := time.NewTicker(DefaultGCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-gc.ctx.Done():
			return
		case <-ticker.C:
			if err := gc.evaluatePolicies(); err != nil {
				klog.Errorf("Error evaluating policies: %v", err)
			}
		}
	}
}

// evaluatePolicies evaluates all policies and performs GC
func (gc *GCController) evaluatePolicies() error {
	// Get all policies from cache
	policies := gc.policyInformer.GetStore().List()

	for _, obj := range policies {
		// Convert unstructured to GarbageCollectionPolicy
		unstructuredObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			klog.Warningf("Unexpected object type in policy informer: %T", obj)
			continue
		}

		// Convert to typed object
		policy := &v1alpha1.GarbageCollectionPolicy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, policy); err != nil {
			klog.Errorf("Error converting unstructured to GarbageCollectionPolicy: %v", err)
			continue
		}

		// Skip paused or error policies
		if policy.Status.Phase == "Paused" || policy.Status.Phase == "Error" {
			continue
		}

		if err := gc.evaluatePolicy(policy); err != nil {
			klog.Errorf("Error evaluating policy %s/%s: %v", policy.Namespace, policy.Name, err)
		}
	}

	return nil
}

// evaluatePolicy evaluates a single policy
func (gc *GCController) evaluatePolicy(policy *v1alpha1.GarbageCollectionPolicy) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		recordEvaluationDuration(policy.Namespace, policy.Name, duration)
	}()

	klog.V(4).Infof("Evaluating policy %s/%s", policy.Namespace, policy.Name)

	// Record policy phase
	phase := policy.Status.Phase
	if phase == "" {
		phase = "Active"
	}
	recordPolicyPhase(policy.Namespace, policy.Name, phase)

	// Get or create resource informer for this policy
	informer, err := gc.getOrCreateResourceInformer(policy)
	if err != nil {
		recordError(policy.Namespace, policy.Name, "informer_creation_failed")
		return fmt.Errorf("failed to get resource informer: %w", err)
	}

	// Get all resources from cache
	resources := informer.GetStore().List()

	matchedCount := int64(0)
	deletedCount := int64(0)
	pendingCount := int64(0)

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	for _, obj := range resources {
		resource, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Check if resource matches selectors
		if !gc.matchesSelectors(resource, &policy.Spec.TargetResource) {
			continue
		}

		matchedCount++
		recordResourceMatched(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind)

		// Check if resource should be deleted
		shouldDelete, reason := gc.shouldDelete(resource, policy)
		if !shouldDelete {
			pendingCount++
			continue
		}

		// Delete the resource
		deleteStart := time.Now()
		if err := gc.deleteResource(resource, policy); err != nil {
			recordError(policy.Namespace, policy.Name, "deletion_failed")
			klog.Errorf("Error deleting resource %s/%s: %v", resource.GetNamespace(), resource.GetName(), err)
			continue
		}

		deletedCount++
		duration := time.Since(deleteStart).Seconds()
		recordResourceDeleted(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind, reason, duration)
		klog.V(2).Infof("Deleted resource %s/%s (reason: %s)", resource.GetNamespace(), resource.GetName(), reason)
	}

	// Update policy status
	if err := gc.updatePolicyStatus(policy, matchedCount, deletedCount, pendingCount); err != nil {
		recordError(policy.Namespace, policy.Name, "status_update_failed")
		klog.Errorf("Error updating policy status: %v", err)
	}

	return nil
}

// matchesSelectors checks if a resource matches the target resource selectors
func (gc *GCController) matchesSelectors(resource *unstructured.Unstructured, target *v1alpha1.TargetResourceSpec) bool {
	// Check namespace
	if target.Namespace != "" && target.Namespace != "*" {
		if resource.GetNamespace() != target.Namespace {
			return false
		}
	}

	// Check label selector
	if target.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(target.LabelSelector)
		if err != nil {
			klog.Errorf("Invalid label selector: %v", err)
			return false
		}

		resourceLabels := labels.Set(resource.GetLabels())
		if !selector.Matches(resourceLabels) {
			return false
		}
	}

	// Check field selector
	if target.FieldSelector != nil {
		for key, value := range target.FieldSelector.MatchFields {
			fieldPath := parseFieldPath(key)
			fieldValue, found, err := unstructured.NestedString(resource.Object, fieldPath...)
			if err != nil || !found || fieldValue != value {
				return false
			}
		}
	}

	return true
}

// shouldDelete determines if a resource should be deleted based on TTL and conditions
func (gc *GCController) shouldDelete(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) (bool, string) {
	// Check conditions first
	if policy.Spec.Conditions != nil {
		if !gc.meetsConditions(resource, policy.Spec.Conditions) {
			return false, "condition_not_met"
		}
	}

	// Calculate TTL
	ttlSeconds, err := gc.calculateTTL(resource, &policy.Spec.TTL)
	if err != nil {
		klog.V(4).Infof("Could not calculate TTL for resource %s/%s: %v", resource.GetNamespace(), resource.GetName(), err)
		return false, "no_ttl"
	}

	if ttlSeconds <= 0 {
		return false, "no_ttl"
	}

	// Check if expired
	creationTime := resource.GetCreationTimestamp().Time
	expirationTime := creationTime.Add(time.Duration(ttlSeconds) * time.Second)

	if time.Now().After(expirationTime) {
		return true, "ttl_expired"
	}

	return false, "not_expired"
}

// calculateTTL calculates the TTL in seconds for a resource based on policy
func (gc *GCController) calculateTTL(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (int64, error) {
	// Option 1: Fixed TTL
	if ttlSpec.SecondsAfterCreation != nil {
		return *ttlSpec.SecondsAfterCreation, nil
	}

	// Option 2: Dynamic TTL from field
	if ttlSpec.FieldPath != "" {
		fieldPath := parseFieldPath(ttlSpec.FieldPath)
		
		// Try to get as int64 first
		value, found, _ := unstructured.NestedInt64(resource.Object, fieldPath...)
		if found {
			return value, nil
		}

		// Try as string for mappings
		strValue, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if found {
			// Option 3: Mapped TTL
			if len(ttlSpec.Mappings) > 0 {
				if ttl, ok := ttlSpec.Mappings[strValue]; ok {
					return ttl, nil
				}
				if ttlSpec.Default != nil {
					return *ttlSpec.Default, nil
				}
				return 0, fmt.Errorf("no mapping for field value %s", strValue)
			}
		}

		if !found && ttlSpec.Default != nil {
			return *ttlSpec.Default, nil
		}
		return 0, fmt.Errorf("field path %s not found", ttlSpec.FieldPath)
	}

	// Option 4: Relative TTL
	if ttlSpec.RelativeTo != "" && ttlSpec.SecondsAfter != nil {
		fieldPath := parseFieldPath(ttlSpec.RelativeTo)
		timestampStr, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if !found {
			return 0, fmt.Errorf("relative timestamp field not found: %s", ttlSpec.RelativeTo)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp format: %v", err)
		}

		expirationTime := timestamp.Add(time.Duration(*ttlSpec.SecondsAfter) * time.Second)
		ttlSeconds := int64(time.Until(expirationTime).Seconds())
		if ttlSeconds > 0 {
			return ttlSeconds, nil
		}
		return 0, fmt.Errorf("relative TTL already expired")
	}

	return 0, fmt.Errorf("no valid TTL configuration")
}

// meetsConditions checks if a resource meets the deletion conditions
func (gc *GCController) meetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	// Check phase conditions
	if len(conditions.Phase) > 0 {
		phase, found, _ := unstructured.NestedString(resource.Object, "status", "phase")
		if !found {
			return false
		}

		phaseMatch := false
		for _, p := range conditions.Phase {
			if phase == p {
				phaseMatch = true
				break
			}
		}
		if !phaseMatch {
			return false
		}
	}

	// Check label conditions
	for _, labelCond := range conditions.HasLabels {
		resourceLabels := resource.GetLabels()
		value, exists := resourceLabels[labelCond.Key]

		switch labelCond.Operator {
		case "Exists":
			if !exists {
				return false
			}
		case "Equals", "":
			if !exists || value != labelCond.Value {
				return false
			}
		}
	}

	// Check annotation conditions
	for _, annCond := range conditions.HasAnnotations {
		resourceAnnotations := resource.GetAnnotations()
		value, exists := resourceAnnotations[annCond.Key]
		if !exists || value != annCond.Value {
			return false
		}
	}

	// Check field conditions (AND logic)
	for _, fieldCond := range conditions.And {
		fieldPath := parseFieldPath(fieldCond.FieldPath)
		fieldValue, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if !found {
			return false
		}

		switch fieldCond.Operator {
		case "Equals":
			if fieldValue != fieldCond.Value {
				return false
			}
		case "NotEquals":
			if fieldValue == fieldCond.Value {
				return false
			}
		case "In":
			found := false
			for _, v := range fieldCond.Values {
				if fieldValue == v {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case "NotIn":
			for _, v := range fieldCond.Values {
				if fieldValue == v {
					return false
				}
			}
		}
	}

	return true
}

// deleteResource deletes a resource based on policy behavior
func (gc *GCController) deleteResource(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) error {
	// Rate limiting
	if err := gc.rateLimiter.Wait(gc.ctx); err != nil {
		return err
	}

	// Dry run check
	if policy.Spec.Behavior.DryRun {
		klog.Infof("[DRY RUN] Would delete resource %s/%s", resource.GetNamespace(), resource.GetName())
		return nil
	}

	// Get GVR
	gvr := schema.GroupVersionResource{
		Group:    resource.GroupVersionKind().Group,
		Version:  resource.GroupVersionKind().Version,
		Resource: validation.PluralizeKind(resource.GetKind()),
	}

	// Delete options
	deleteOptions := &metav1.DeleteOptions{}
	if policy.Spec.Behavior.GracePeriodSeconds != nil {
		deleteOptions.GracePeriodSeconds = policy.Spec.Behavior.GracePeriodSeconds
	}

	propagationPolicy := metav1.DeletionPropagationBackground
	if policy.Spec.Behavior.PropagationPolicy != "" {
		switch policy.Spec.Behavior.PropagationPolicy {
		case "Foreground":
			propagationPolicy = metav1.DeletionPropagationForeground
		case "Orphan":
			propagationPolicy = metav1.DeletionPropagationOrphan
		}
	}
	deleteOptions.PropagationPolicy = &propagationPolicy

	// Delete the resource
	namespace := resource.GetNamespace()
	var err error
	if namespace == "" {
		err = gc.dynamicClient.Resource(gvr).Delete(gc.ctx, resource.GetName(), *deleteOptions)
	} else {
		err = gc.dynamicClient.Resource(gvr).Namespace(namespace).Delete(gc.ctx, resource.GetName(), *deleteOptions)
	}

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

// getOrCreateResourceInformer gets or creates a resource informer for a policy
func (gc *GCController) getOrCreateResourceInformer(policy *v1alpha1.GarbageCollectionPolicy) (cache.SharedInformer, error) {
	// Check if informer already exists
	if informer, ok := gc.resourceInformers[policy.UID]; ok {
		return informer, nil
	}

	// Create GVR
	gvr, err := validation.ParseGVR(policy.Spec.TargetResource.APIVersion, policy.Spec.TargetResource.Kind)
	if err != nil {
		return nil, fmt.Errorf("invalid target resource: %w", err)
	}

	// Determine namespace
	namespace := policy.Spec.TargetResource.Namespace
	if namespace == "" {
		namespace = policy.Namespace
	}

	// Create informer factory
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		gc.dynamicClient,
		DefaultGCInterval,
		namespace,
		func(options *metav1.ListOptions) {
			if policy.Spec.TargetResource.LabelSelector != nil {
				selector, err := metav1.LabelSelectorAsSelector(policy.Spec.TargetResource.LabelSelector)
				if err == nil {
					options.LabelSelector = selector.String()
				}
			}
		},
	)

	// Create informer
	informer := factory.ForResource(gvr).Informer()

	// Store informer
	gc.resourceInformers[policy.UID] = informer

	// Start informer
	go informer.Run(gc.ctx.Done())

	// Wait for cache sync
	if !cache.WaitForCacheSync(gc.ctx.Done(), informer.HasSynced) {
		return nil, fmt.Errorf("failed to sync resource informer cache")
	}

	return informer, nil
}

// updatePolicyStatus updates the status of a policy
func (gc *GCController) updatePolicyStatus(policy *v1alpha1.GarbageCollectionPolicy, matched, deleted, pending int64) error {
	// This would require a client for the GarbageCollectionPolicy CRD
	// For now, we'll just log the status update
	klog.V(2).Infof("Policy %s/%s: matched=%d, deleted=%d, pending=%d",
		policy.Namespace, policy.Name, matched, deleted, pending)

	// TODO: Implement actual status update via client
	return nil
}
