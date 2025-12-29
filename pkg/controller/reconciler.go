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
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
	"github.com/kube-zen/zen-gc/pkg/logging"
	"github.com/kube-zen/zen-gc/pkg/validation"
)

// GCPolicyReconciler reconciles GarbageCollectionPolicy resources.
// It implements the controller-runtime Reconciler interface.
type GCPolicyReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	dynamicClient dynamic.Interface

	// Controller configuration.
	config *config.ControllerConfig

	// Resource informers (one per policy).
	// Protected by resourceInformersMu mutex.
	resourceInformers map[types.UID]cache.SharedInformer

	// Resource informer factories (one per policy).
	// Protected by resourceInformersMu mutex.
	resourceInformerFactories map[types.UID]dynamicinformer.DynamicSharedInformerFactory

	// Mutex to protect resourceInformers and resourceInformerFactories maps.
	resourceInformersMu sync.RWMutex

	// Per-policy rate limiters (one per policy).
	// Protected by rateLimitersMu mutex.
	rateLimiters map[types.UID]*RateLimiter

	// Mutex to protect rateLimiters map.
	rateLimitersMu sync.RWMutex

	// Track policy UIDs by NamespacedName for cleanup on deletion.
	// Protected by policyUIDsMu mutex.
	policyUIDs map[types.NamespacedName]types.UID

	// Mutex to protect policyUIDs map.
	policyUIDsMu sync.RWMutex

	// Track last known policy spec for update detection.
	// Protected by policySpecsMu mutex.
	policySpecs map[types.UID]*v1alpha1.GarbageCollectionPolicySpec

	// Mutex to protect policySpecs map.
	policySpecsMu sync.RWMutex

	// Status updater.
	statusUpdater *StatusUpdater

	// Event recorder.
	eventRecorder *EventRecorder
}

// NewGCPolicyReconciler creates a new GC policy reconciler.
func NewGCPolicyReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	dynamicClient dynamic.Interface,
	statusUpdater *StatusUpdater,
	eventRecorder *EventRecorder,
	cfg *config.ControllerConfig,
) *GCPolicyReconciler {
	// Use default config if nil
	if cfg == nil {
		cfg = config.NewControllerConfig()
	}

	return &GCPolicyReconciler{
		Client:                    client,
		Scheme:                    scheme,
		dynamicClient:             dynamicClient,
		config:                    cfg,
		resourceInformers:         make(map[types.UID]cache.SharedInformer),
		resourceInformerFactories: make(map[types.UID]dynamicinformer.DynamicSharedInformerFactory),
		rateLimiters:              make(map[types.UID]*RateLimiter),
		policyUIDs:                make(map[types.NamespacedName]types.UID),
		policySpecs:               make(map[types.UID]*v1alpha1.GarbageCollectionPolicySpec),
		statusUpdater:             statusUpdater,
		eventRecorder:             eventRecorder,
	}
}

// Reconcile is the main reconciliation function called by controller-runtime.
// It is triggered by changes to GarbageCollectionPolicy resources.
func (r *GCPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logging.FromContext(ctx)
	logger = logger.WithField("policy", fmt.Sprintf("%s/%s", req.Namespace, req.Name))

	// Fetch the GarbageCollectionPolicy instance
	policy := &v1alpha1.GarbageCollectionPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			// Policy was deleted - clean up associated resources
			logger.V(2).Info("Policy not found, cleaning up resources")
			r.cleanupPolicyResources(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		logger.WithError(err).Error("Failed to fetch GarbageCollectionPolicy")
		return ctrl.Result{}, err
	}

	// Track policy UID for cleanup on deletion
	r.trackPolicyUID(req.NamespacedName, policy.UID)

	// Check if policy spec changed and requires informer recreation
	if r.shouldRecreateInformer(policy) {
		logger.V(2).Info("Policy spec changed, recreating informer")
		r.cleanupResourceInformer(policy.UID)
		// Clear old spec to allow new one to be tracked
		r.policySpecsMu.Lock()
		delete(r.policySpecs, policy.UID)
		r.policySpecsMu.Unlock()
	}

	// Store current spec for future comparison
	r.trackPolicySpec(policy.UID, &policy.Spec)

	// Skip paused policies
	if policy.Spec.Paused {
		logger.V(4).Info("Policy is paused, skipping evaluation")
		return ctrl.Result{RequeueAfter: r.getRequeueInterval(policy)}, nil
	}

	// Evaluate the policy
	if err := r.evaluatePolicy(ctx, policy); err != nil {
		gcErr := gcerrors.WithPolicy(err, policy.Namespace, policy.Name)
		if gcErr.Type == "" {
			gcErr.Type = "evaluation_failed"
		}
		logger.WithError(gcErr).Error("Error evaluating policy")
		// Requeue with backoff on error
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Record policy phase metrics periodically
	// Use a simple counter to avoid calling too frequently
	r.recordPolicyPhaseMetrics(ctx)

	// Determine requeue interval based on policy evaluation interval or default
	requeueAfter := r.getRequeueInterval(policy)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// getRequeueInterval returns the requeue interval for a policy.
// Uses policy-specific evaluation interval if configured, otherwise uses default.
func (r *GCPolicyReconciler) getRequeueInterval(policy *v1alpha1.GarbageCollectionPolicy) time.Duration {
	// TODO: Add EvaluationInterval field to GarbageCollectionPolicySpec if needed
	// For now, use the default GC interval from config
	interval := DefaultGCInterval
	if r.config != nil {
		interval = r.config.GCInterval
	}
	return interval
}

// evaluatePolicy evaluates a single policy.
// This is adapted from the original GCController.evaluatePolicy method.
func (r *GCPolicyReconciler) evaluatePolicy(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		recordEvaluationDuration(policy.Namespace, policy.Name, duration)
	}()

	klog.V(4).Infof("Evaluating policy %s/%s", policy.Namespace, policy.Name)

	// Get or create resource informer for this policy
	informer, err := r.getOrCreateResourceInformer(ctx, policy)
	if err != nil {
		gcErr := gcerrors.Wrap(err, "informer_creation_failed", "failed to get resource informer")
		gcErr.PolicyNamespace = policy.Namespace
		gcErr.PolicyName = policy.Name
		recordError(policy.Namespace, policy.Name, "informer_creation_failed")
		klog.Errorf("Error creating resource informer for policy %s/%s: %v", policy.Namespace, policy.Name, gcErr)
		return gcErr
	}

	// Get all resources from cache
	resources := informer.GetStore().List()

	matchedCount := int64(0)
	deletedCount := int64(0)
	pendingCount := int64(0)

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	// Collect resources to delete
	resourcesToDelete := make([]*unstructured.Unstructured, 0)
	resourcesToDeleteReasons := make(map[string]string) // resource UID -> reason

	for _, obj := range resources {
		// Check context cancellation during resource iteration
		select {
		case <-ctx.Done():
			klog.V(4).Infof("Stopping policy evaluation for %s/%s: context canceled", policy.Namespace, policy.Name)
			return nil
		default:
		}

		resource, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Check if resource matches selectors
		if !r.matchesSelectors(resource, &policy.Spec.TargetResource) {
			continue
		}

		matchedCount++
		recordResourceMatched(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind)

		// Check if resource should be deleted
		shouldDelete, reason := r.shouldDelete(resource, policy)
		if !shouldDelete {
			pendingCount++
			continue
		}

		// Add to deletion list
		resourcesToDelete = append(resourcesToDelete, resource)
		resourcesToDeleteReasons[string(resource.GetUID())] = reason
	}

	// Delete resources in batches
	if len(resourcesToDelete) > 0 {
		rateLimiter := r.getOrCreateRateLimiter(policy)
		batchSize := r.getBatchSize(policy)

		// Process deletions in batches
		for i := 0; i < len(resourcesToDelete); i += batchSize {
			// Check context cancellation between batches
			select {
			case <-ctx.Done():
				klog.V(4).Infof("Stopping batch deletion for %s/%s: context canceled", policy.Namespace, policy.Name)
				return nil
			default:
			}

			end := i + batchSize
			if end > len(resourcesToDelete) {
				end = len(resourcesToDelete)
			}
			batch := resourcesToDelete[i:end]

			// Delete batch
			// Track deletion attempts (total resources in batch)
			deletionAttempts := int64(len(batch))
			batchDeleted, batchErrors := r.deleteBatch(ctx, batch, policy, rateLimiter, resourcesToDeleteReasons)
			deletedCount += batchDeleted

			// Track deletion failures
			if len(batchErrors) > 0 {
				recordError(policy.Namespace, policy.Name, "deletion_failed")
			}

			// Log errors
			for _, err := range batchErrors {
				if r.eventRecorder != nil {
					r.eventRecorder.RecordEvaluationFailed(policy, err)
				}
				klog.Errorf("Error deleting batch for policy %s/%s: %v", policy.Namespace, policy.Name, err)
			}

			// Log deletion attempt metrics
			klog.V(4).Infof("Policy %s/%s: attempted %d deletions, succeeded %d, failed %d",
				policy.Namespace, policy.Name, deletionAttempts, batchDeleted, int64(len(batchErrors)))
		}
	}

	// Record pending resources metric
	if pendingCount > 0 {
		recordResourcesPending(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind, pendingCount)
	}

	// Update policy status with timeout context
	if r.statusUpdater != nil {
		// Use timeout context for status updates to prevent hanging
		statusCtx, statusCancel := context.WithTimeout(ctx, 10*time.Second)
		defer statusCancel()

		if err := r.statusUpdater.UpdateStatus(statusCtx, policy, matchedCount, deletedCount, pendingCount); err != nil {
			// Check if error is due to context cancellation/timeout
			if statusCtx.Err() != nil {
				klog.V(4).Infof("Status update canceled or timed out for policy %s/%s: %v", policy.Namespace, policy.Name, statusCtx.Err())
				return nil // Don't treat cancellation as error
			}
			gcErr := gcerrors.Wrap(err, "status_update_failed", "failed to update policy status")
			gcErr.PolicyNamespace = policy.Namespace
			gcErr.PolicyName = policy.Name
			recordError(policy.Namespace, policy.Name, "status_update_failed")
			if r.eventRecorder != nil {
				r.eventRecorder.RecordStatusUpdateFailed(policy, gcErr)
			}
			klog.Errorf("Error updating policy status for %s/%s: %v", policy.Namespace, policy.Name, gcErr)
		}
	}

	// Record policy evaluation event
	if r.eventRecorder != nil {
		r.eventRecorder.RecordPolicyEvaluated(policy, matchedCount, deletedCount, pendingCount)
	}

	return nil
}

// matchesSelectors checks if a resource matches the target resource selectors.
func (r *GCPolicyReconciler) matchesSelectors(resource *unstructured.Unstructured, target *v1alpha1.TargetResourceSpec) bool {
	// Normalize namespace: empty defaults to "*" (cluster-wide) to match webhook behavior
	namespace := target.Namespace
	if namespace == "" {
		namespace = "*"
	}

	// Check namespace
	if namespace != "*" {
		if resource.GetNamespace() != namespace {
			return false
		}
	}

	// Check label selector
	if target.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(target.LabelSelector)
		if err != nil {
			gcErr := gcerrors.Wrap(err, "invalid_label_selector", "invalid label selector")
			klog.Errorf("Invalid label selector: %v", gcErr)
			return false
		}

		resourceLabels := labels.Set(resource.GetLabels())
		if !selector.Matches(resourceLabels) {
			return false
		}
	}

	// Check field selector
	// Field selectors are evaluated in-memory only (not pushed down to API server).
	// Unlike label selectors which are sent to the API server to reduce watch/list volume,
	// field selectors are evaluated after resources are fetched. This means:
	// - Field selectors do NOT reduce API server load or network traffic
	// - All resources matching the GVR/namespace/labelSelector are fetched and cached
	// - Field selector filtering happens in the controller's memory
	// For better performance, prefer label selectors when possible.
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

// shouldDelete determines if a resource should be deleted based on TTL and conditions.
func (r *GCPolicyReconciler) shouldDelete(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) (shouldDelete bool, reason string) {
	// Check conditions first
	if policy.Spec.Conditions != nil {
		if !r.meetsConditions(resource, policy.Spec.Conditions) {
			return false, "condition_not_met"
		}
	}

	// Calculate expiration time
	expirationTime, err := r.calculateExpirationTime(resource, &policy.Spec.TTL)
	if err != nil {
		klog.V(4).Infof("Could not calculate expiration time for resource %s/%s: %v", resource.GetNamespace(), resource.GetName(), err)
		return false, ReasonNoTTL
	}

	if expirationTime.IsZero() {
		return false, ReasonNoTTL
	}

	// Check if expired
	if time.Now().After(expirationTime) {
		return true, "ttl_expired"
	}

	return false, "not_expired"
}

// calculateExpirationTime calculates the absolute expiration time for a resource based on policy.
// Returns zero time if TTL cannot be calculated or is invalid.
func (r *GCPolicyReconciler) calculateExpirationTime(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (time.Time, error) {
	// Option 1: Fixed TTL (seconds after creation)
	if ttlSpec.SecondsAfterCreation != nil {
		creationTime := resource.GetCreationTimestamp().Time
		return creationTime.Add(time.Duration(*ttlSpec.SecondsAfterCreation) * time.Second), nil
	}

	// Option 2: Dynamic TTL from field
	if ttlSpec.FieldPath != "" {
		fieldPath := parseFieldPath(ttlSpec.FieldPath)

		// Try to get as int64 first
		value, found, _ := unstructured.NestedInt64(resource.Object, fieldPath...)
		if found {
			creationTime := resource.GetCreationTimestamp().Time
			return creationTime.Add(time.Duration(value) * time.Second), nil
		}

		// Try as string for mappings
		strValue, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if found {
			// Option 3: Mapped TTL
			if len(ttlSpec.Mappings) > 0 {
				var ttl int64
				if mappedTTL, ok := ttlSpec.Mappings[strValue]; ok {
					ttl = mappedTTL
				} else if ttlSpec.Default != nil {
					ttl = *ttlSpec.Default
				} else {
					return time.Time{}, fmt.Errorf("%w: %s", ErrNoMappingForFieldValue, strValue)
				}
				creationTime := resource.GetCreationTimestamp().Time
				return creationTime.Add(time.Duration(ttl) * time.Second), nil
			}
		}

		if !found && ttlSpec.Default != nil {
			creationTime := resource.GetCreationTimestamp().Time
			return creationTime.Add(time.Duration(*ttlSpec.Default) * time.Second), nil
		}
		return time.Time{}, fmt.Errorf("%w: %s", ErrFieldPathNotFound, ttlSpec.FieldPath)
	}

	// Option 4: Relative TTL (relative to another timestamp field)
	if ttlSpec.RelativeTo != "" && ttlSpec.SecondsAfter != nil {
		fieldPath := parseFieldPath(ttlSpec.RelativeTo)
		timestampStr, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if !found {
			return time.Time{}, fmt.Errorf("%w: %s", ErrRelativeTimestampFieldNotFound, ttlSpec.RelativeTo)
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %w", ErrInvalidTimestampFormat, err)
		}

		// Calculate absolute expiration time from the relative timestamp
		expirationTime := timestamp.Add(time.Duration(*ttlSpec.SecondsAfter) * time.Second)
		if time.Now().After(expirationTime) {
			return time.Time{}, fmt.Errorf("%w", ErrRelativeTTLExpired)
		}
		return expirationTime, nil
	}

	return time.Time{}, fmt.Errorf("%w", ErrNoValidTTLConfiguration)
}

// meetsConditions checks if a resource meets the deletion conditions.
func (r *GCPolicyReconciler) meetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	if !r.meetsPhaseConditions(resource, conditions.Phase) {
		return false
	}
	if !r.meetsLabelConditions(resource, conditions.HasLabels) {
		return false
	}
	if !r.meetsAnnotationConditions(resource, conditions.HasAnnotations) {
		return false
	}
	if !r.meetsFieldConditions(resource, conditions.And) {
		return false
	}
	return true
}

// meetsPhaseConditions checks if resource phase matches any of the required phases.
func (r *GCPolicyReconciler) meetsPhaseConditions(resource *unstructured.Unstructured, phases []string) bool {
	if len(phases) == 0 {
		return true
	}
	phase, found, _ := unstructured.NestedString(resource.Object, "status", "phase")
	if !found {
		return false
	}
	for _, p := range phases {
		if phase == p {
			return true
		}
	}
	return false
}

// meetsLabelConditions checks if resource labels match the required conditions.
func (r *GCPolicyReconciler) meetsLabelConditions(resource *unstructured.Unstructured, labelConds []v1alpha1.LabelCondition) bool {
	resourceLabels := resource.GetLabels()
	for _, labelCond := range labelConds {
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
		case "In":
			if !exists {
				return false
			}
			// Check if value is in the Values list (if Values is set) or matches Value
			found := false
			if labelCond.Value != "" {
				found = value == labelCond.Value
			}
			// LabelCondition doesn't have Values field, so In operator checks against Value only
			// This matches the documented behavior where In checks if label value equals the specified value
			if !found {
				return false
			}
		case "NotIn":
			if !exists {
				// Label doesn't exist, so it's "not in" any value - condition satisfied
				continue
			}
			// Check if value is NOT in the Values list (if Values is set) or doesn't match Value
			if value == labelCond.Value {
				return false
			}
		default:
			// Unknown operator - fail safe by rejecting
			klog.Warningf("Unknown label condition operator: %s, rejecting match", labelCond.Operator)
			return false
		}
	}
	return true
}

// meetsAnnotationConditions checks if resource annotations match the required conditions.
func (r *GCPolicyReconciler) meetsAnnotationConditions(resource *unstructured.Unstructured, annConds []v1alpha1.AnnotationCondition) bool {
	resourceAnnotations := resource.GetAnnotations()
	for _, annCond := range annConds {
		value, exists := resourceAnnotations[annCond.Key]
		if !exists || value != annCond.Value {
			return false
		}
	}
	return true
}

// meetsFieldConditions checks if resource fields match the required conditions.
func (r *GCPolicyReconciler) meetsFieldConditions(resource *unstructured.Unstructured, fieldConds []v1alpha1.FieldCondition) bool {
	for _, fieldCond := range fieldConds {
		fieldPath := parseFieldPath(fieldCond.FieldPath)
		fieldValue, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if !found {
			return false
		}
		if !r.matchesFieldOperator(fieldValue, fieldCond) {
			return false
		}
	}
	return true
}

// matchesFieldOperator checks if field value matches the operator condition.
func (r *GCPolicyReconciler) matchesFieldOperator(fieldValue string, fieldCond v1alpha1.FieldCondition) bool {
	switch fieldCond.Operator {
	case "Equals":
		return fieldValue == fieldCond.Value
	case "NotEquals":
		return fieldValue != fieldCond.Value
	case "In":
		for _, v := range fieldCond.Values {
			if fieldValue == v {
				return true
			}
		}
		return false
	case "NotIn":
		for _, v := range fieldCond.Values {
			if fieldValue == v {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// deleteResource deletes a resource based on policy behavior.
func (r *GCPolicyReconciler) deleteResource(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *RateLimiter) error {
	// Rate limiting
	if err := rateLimiter.Wait(ctx); err != nil {
		return err
	}

	// Dry run check
	if policy.Spec.Behavior.DryRun {
		klog.Infof("[DRY RUN] Would delete resource %s/%s", resource.GetNamespace(), resource.GetName())
		return nil
	}

	// Get GVR using pluralization
	// TODO: Replace with RESTMapper-based resolution (see ROADMAP.md)
	// Current pluralization may fail for irregular Kinds/CRDs, but maintains backward compatibility
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

	var propagationPolicy metav1.DeletionPropagation
	switch policy.Spec.Behavior.PropagationPolicy {
	case "Foreground":
		propagationPolicy = "Foreground"
	case "Orphan":
		propagationPolicy = "Orphan"
	default:
		propagationPolicy = "Background"
	}
	deleteOptions.PropagationPolicy = &propagationPolicy

	// Delete the resource
	namespace := resource.GetNamespace()
	var err error
	if namespace == "" {
		err = r.dynamicClient.Resource(gvr).Delete(ctx, resource.GetName(), *deleteOptions)
	} else {
		err = r.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, resource.GetName(), *deleteOptions)
	}

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

// getOrCreateResourceInformer gets or creates a resource informer for a policy.
func (r *GCPolicyReconciler) getOrCreateResourceInformer(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) (cache.SharedInformer, error) {
	// Check if informer already exists (with read lock)
	r.resourceInformersMu.RLock()
	if informer, ok := r.resourceInformers[policy.UID]; ok {
		r.resourceInformersMu.RUnlock()
		return informer, nil
	}
	r.resourceInformersMu.RUnlock()

	// Acquire write lock for creating new informer
	r.resourceInformersMu.Lock()
	defer r.resourceInformersMu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have created it)
	if informer, ok := r.resourceInformers[policy.UID]; ok {
		return informer, nil
	}

	// Create GVR
	gvr, err := validation.ParseGVR(policy.Spec.TargetResource.APIVersion, policy.Spec.TargetResource.Kind)
	if err != nil {
		return nil, fmt.Errorf("invalid target resource: %w", err)
	}

	// Determine namespace
	// Normalize: empty defaults to "*" (cluster-wide) to match webhook behavior
	namespace := policy.Spec.TargetResource.Namespace
	if namespace == "" {
		namespace = "*"
	}
	// Translate "*" to NamespaceAll (empty string) for cluster-wide watching
	if namespace == "*" {
		namespace = metav1.NamespaceAll
	}

	// Create informer factory using configured interval
	interval := DefaultGCInterval
	if r.config != nil {
		interval = r.config.GCInterval
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		r.dynamicClient,
		interval,
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

	// Store informer and factory
	r.resourceInformers[policy.UID] = informer
	r.resourceInformerFactories[policy.UID] = factory

	// Update metrics
	recordInformerCount(len(r.resourceInformers))

	// Start informer factory
	factory.Start(ctx.Done())

	// Wait for cache sync with timeout
	syncCtx, syncCancel := context.WithTimeout(ctx, DefaultCacheSyncTimeout)
	defer syncCancel()

	if !cache.WaitForCacheSync(syncCtx.Done(), informer.HasSynced) {
		// Clean up on failure
		delete(r.resourceInformers, policy.UID)
		delete(r.resourceInformerFactories, policy.UID)
		if syncCtx.Err() != nil {
			return nil, fmt.Errorf("resource informer cache sync timed out: %w", syncCtx.Err())
		}
		return nil, fmt.Errorf("%w", ErrResourceInformerCacheSyncFailed)
	}

	klog.V(4).Infof("Created resource informer for policy %s/%s (UID: %s)", policy.Namespace, policy.Name, policy.UID)
	return informer, nil
}

// getOrCreateRateLimiter gets or creates a rate limiter for a policy.
func (r *GCPolicyReconciler) getOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *RateLimiter {
	// Determine rate limit for this policy
	maxDeletionsPerSecond := DefaultMaxDeletionsPerSecond
	if policy.Spec.Behavior.MaxDeletionsPerSecond > 0 {
		maxDeletionsPerSecond = policy.Spec.Behavior.MaxDeletionsPerSecond
	}

	// Check if rate limiter already exists (with read lock)
	r.rateLimitersMu.RLock()
	if limiter, ok := r.rateLimiters[policy.UID]; ok {
		// Update rate if it changed
		if limiter != nil {
			// Update rate to match policy configuration
			limiter.SetRate(maxDeletionsPerSecond)
		}
		r.rateLimitersMu.RUnlock()
		return limiter
	}
	r.rateLimitersMu.RUnlock()

	// Acquire write lock for creating new rate limiter
	r.rateLimitersMu.Lock()
	defer r.rateLimitersMu.Unlock()

	// Double-check after acquiring write lock
	if limiter, ok := r.rateLimiters[policy.UID]; ok {
		limiter.SetRate(maxDeletionsPerSecond)
		return limiter
	}

	// Create new rate limiter
	limiter := NewRateLimiter(maxDeletionsPerSecond)
	r.rateLimiters[policy.UID] = limiter

	// Update metrics
	recordRateLimiterCount(len(r.rateLimiters))

	klog.V(4).Infof("Created rate limiter for policy %s/%s (UID: %s, rate: %d/sec)", policy.Namespace, policy.Name, policy.UID, maxDeletionsPerSecond)
	return limiter
}

// getBatchSize returns the batch size for a policy.
func (r *GCPolicyReconciler) getBatchSize(policy *v1alpha1.GarbageCollectionPolicy) int {
	batchSize := DefaultBatchSize
	if r.config != nil {
		batchSize = r.config.BatchSize
	}
	if policy.Spec.Behavior.BatchSize > 0 {
		batchSize = policy.Spec.Behavior.BatchSize
	}
	return batchSize
}

// deleteBatch deletes a batch of resources.
// Returns the number of successfully deleted resources and any errors encountered.
func (r *GCPolicyReconciler) deleteBatch(
	ctx context.Context,
	batch []*unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	rateLimiter *RateLimiter,
	reasons map[string]string,
) (int64, []error) {
	deletedCount := int64(0)
	errors := make([]error, 0)

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	for _, resource := range batch {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return deletedCount, errors
		default:
		}

		// Rate limiting (per resource)
		if err := rateLimiter.Wait(ctx); err != nil {
			errors = append(errors, fmt.Errorf("rate limiter error: %w", err))
			continue
		}

		// Delete the resource with exponential backoff
		deleteStart := time.Now()
		if err := r.deleteResourceWithBackoff(ctx, resource, policy, rateLimiter); err != nil {
			gcErr := gcerrors.WithResource(
				gcerrors.WithPolicy(err, policy.Namespace, policy.Name),
				resource.GetNamespace(),
				resource.GetName(),
			)
			gcErr.Type = "deletion_failed"
			recordError(policy.Namespace, policy.Name, "deletion_failed")
			errors = append(errors, gcErr)
			continue
		}

		deletedCount++
		duration := time.Since(deleteStart).Seconds()
		reason := reasons[string(resource.GetUID())]
		recordResourceDeleted(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind, reason, duration)
		if r.eventRecorder != nil {
			r.eventRecorder.RecordResourceDeleted(policy, resource, reason)
		}
		klog.V(2).Infof("Deleted resource %s/%s (reason: %s)", resource.GetNamespace(), resource.GetName(), reason)
	}

	return deletedCount, errors
}

// deleteResourceWithBackoff deletes a resource with exponential backoff retry logic.
// This method uses the same backoff logic as the original GCController implementation.
func (r *GCPolicyReconciler) deleteResourceWithBackoff(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *RateLimiter) error {
	var lastErr error

	err := wait.ExponentialBackoff(DefaultBackoff, func() (bool, error) {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}
		err := r.deleteResource(ctx, resource, policy, rateLimiter)
		if err != nil {
			// Check if error is retryable
			if errors.IsTimeout(err) || errors.IsServerTimeout(err) ||
				errors.IsTooManyRequests(err) || errors.IsServiceUnavailable(err) {
				lastErr = err
				return false, nil // retry
			}
			// For NotFound errors, consider it success (already deleted)
			if errors.IsNotFound(err) {
				return true, nil // success
			}
			return false, err // don't retry
		}
		return true, nil // success
	})

	if wait.Interrupted(err) {
		return fmt.Errorf("deletion failed after retries: %w", lastErr)
	}

	return err
}

// trackPolicyUID tracks a policy UID by NamespacedName for cleanup on deletion.
func (r *GCPolicyReconciler) trackPolicyUID(nn types.NamespacedName, uid types.UID) {
	r.policyUIDsMu.Lock()
	defer r.policyUIDsMu.Unlock()
	r.policyUIDs[nn] = uid
}

// trackPolicySpec tracks a policy spec for change detection.
func (r *GCPolicyReconciler) trackPolicySpec(uid types.UID, spec *v1alpha1.GarbageCollectionPolicySpec) {
	r.policySpecsMu.Lock()
	defer r.policySpecsMu.Unlock()
	// Deep copy the spec to avoid reference issues
	specCopy := spec.DeepCopy()
	r.policySpecs[uid] = specCopy
}

// shouldRecreateInformer checks if policy spec changed in a way that requires informer recreation.
func (r *GCPolicyReconciler) shouldRecreateInformer(policy *v1alpha1.GarbageCollectionPolicy) bool {
	r.policySpecsMu.RLock()
	defer r.policySpecsMu.RUnlock()

	oldSpec, exists := r.policySpecs[policy.UID]
	if !exists {
		// First time seeing this policy, no need to recreate
		return false
	}

	// Compare key fields that affect informer creation
	newSpec := policy.Spec.TargetResource
	oldTarget := oldSpec.TargetResource

	if oldTarget.APIVersion != newSpec.APIVersion ||
		oldTarget.Kind != newSpec.Kind ||
		oldTarget.Namespace != newSpec.Namespace ||
		!labelSelectorsEqual(oldTarget.LabelSelector, newSpec.LabelSelector) {
		return true
	}

	return false
}

// cleanupPolicyResources cleans up all resources associated with a policy by NamespacedName.
func (r *GCPolicyReconciler) cleanupPolicyResources(nn types.NamespacedName) {
	r.policyUIDsMu.Lock()
	uid, exists := r.policyUIDs[nn]
	if exists {
		delete(r.policyUIDs, nn)
	}
	r.policyUIDsMu.Unlock()

	if !exists {
		// Policy UID not tracked, nothing to clean up
		return
	}

	klog.V(2).Infof("Cleaning up resources for policy %s/%s (UID: %s)", nn.Namespace, nn.Name, uid)

	// Clean up resource informer
	r.cleanupResourceInformer(uid)

	// Clean up rate limiter
	r.cleanupRateLimiter(uid)

	// Clean up tracked spec
	r.policySpecsMu.Lock()
	delete(r.policySpecs, uid)
	r.policySpecsMu.Unlock()
}

// cleanupResourceInformer cleans up a resource informer for a given policy UID.
func (r *GCPolicyReconciler) cleanupResourceInformer(policyUID types.UID) {
	r.resourceInformersMu.Lock()
	defer r.resourceInformersMu.Unlock()

	_, informerExists := r.resourceInformers[policyUID]
	_, factoryExists := r.resourceInformerFactories[policyUID]

	if !informerExists && !factoryExists {
		// Already cleaned up or never existed
		return
	}

	// Stop the informer factory (which will stop all informers created by it)
	if factoryExists {
		// DynamicSharedInformerFactory doesn't have a Stop method,
		// but stopping is handled by context cancellation.
		// We just need to remove it from our tracking.
		delete(r.resourceInformerFactories, policyUID)
	}

	// Remove informer from map
	if informerExists {
		delete(r.resourceInformers, policyUID)
		klog.V(4).Infof("Cleaned up resource informer for policy UID: %s", policyUID)
	}

	// Update metrics
	recordInformerCount(len(r.resourceInformers))
}

// cleanupRateLimiter cleans up a rate limiter for a given policy UID.
func (r *GCPolicyReconciler) cleanupRateLimiter(policyUID types.UID) {
	r.rateLimitersMu.Lock()
	defer r.rateLimitersMu.Unlock()

	if _, exists := r.rateLimiters[policyUID]; exists {
		delete(r.rateLimiters, policyUID)
		klog.V(4).Infof("Cleaned up rate limiter for policy UID: %s", policyUID)
	}

	// Update metrics
	recordRateLimiterCount(len(r.rateLimiters))
}

// recordPolicyPhaseMetrics records metrics for policy phases.
// Uses controller-runtime cache to list all policies.
func (r *GCPolicyReconciler) recordPolicyPhaseMetrics(ctx context.Context) {
	// List all policies using the client cache
	policyList := &v1alpha1.GarbageCollectionPolicyList{}
	if err := r.List(ctx, policyList); err != nil {
		klog.V(4).Infof("Failed to list policies for metrics: %v", err)
		return
	}

	phaseCounts := make(map[string]float64)

	for _, policy := range policyList.Items {
		phase := policy.Status.Phase
		if phase == "" {
			// Determine phase from spec
			if policy.Spec.Paused {
				phase = "Paused"
			} else {
				phase = "Active"
			}
		}
		phaseCounts[phase]++
	}

	// Update metrics for each phase
	for phase, count := range phaseCounts {
		recordPolicyPhase(phase, count)
	}

	// Reset phases that are no longer present
	knownPhases := []string{"Active", "Paused", "Error"}
	for _, phase := range knownPhases {
		if _, exists := phaseCounts[phase]; !exists {
			recordPolicyPhase(phase, 0)
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GCPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GarbageCollectionPolicy{}).
		Complete(r)
}

