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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
				gcErr.Type = ErrorTypeEvaluationFailed
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

	// Evaluate resources and collect those to delete
	matchedCount, _, pendingCount, resourcesToDelete, resourcesToDeleteReasons, err := evaluatePolicyResourcesShared(ctx, r, policy, informer)
	if err != nil {
		return err
	}

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	// Delete resources in batches
	deletedCount, err := deleteResourcesInBatchesShared(ctx, r, policy, resourcesToDelete, resourcesToDeleteReasons)
	if err != nil {
		return err
	}

	// Record pending resources metric
	if pendingCount > 0 {
		recordResourcesPending(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind, pendingCount)
	}

	// Update policy status
	if err := updatePolicyStatusShared(ctx, r, policy, matchedCount, deletedCount, pendingCount); err != nil {
		return err
	}

	// Record policy evaluation event
	if r.eventRecorder != nil {
		r.eventRecorder.RecordPolicyEvaluated(policy, matchedCount, deletedCount, pendingCount)
	}

	return nil
}

// getStatusUpdater returns the status updater (implements PolicyEvaluator).
func (r *GCPolicyReconciler) getStatusUpdater() *StatusUpdater {
	return r.statusUpdater
}

// matchesSelectors checks if a resource matches the target resource selectors.
func (r *GCPolicyReconciler) matchesSelectors(resource *unstructured.Unstructured, target *v1alpha1.TargetResourceSpec) bool {
	return matchesSelectorsShared(resource, target)
}

// shouldDelete determines if a resource should be deleted based on TTL and conditions.
func (r *GCPolicyReconciler) shouldDelete(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) (shouldDelete bool, reason string) {
	// Check conditions first
	if policy.Spec.Conditions != nil {
		if !r.meetsConditions(resource, policy.Spec.Conditions) {
			return false, ReasonConditionNotMet
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
		return true, ReasonTTLExpired
	}

	return false, ReasonNotExpired
}

// calculateExpirationTime calculates the absolute expiration time for a resource based on policy.
// Returns zero time if TTL cannot be calculated or is invalid.
func (r *GCPolicyReconciler) calculateExpirationTime(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (time.Time, error) {
	return calculateExpirationTimeShared(resource, ttlSpec)
}

// meetsConditions checks if a resource meets the deletion conditions.
func (r *GCPolicyReconciler) meetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	return meetsConditionsShared(resource, conditions)
}



// matchesFieldOperator checks if field value matches the operator condition.
func (r *GCPolicyReconciler) matchesFieldOperator(fieldValue string, fieldCond v1alpha1.FieldCondition) bool {
	return matchesFieldOperatorShared(fieldValue, fieldCond)
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

	propagationPolicy := getDeletionPropagationPolicy(policy.Spec.Behavior.PropagationPolicy)
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
	return getOrCreateRateLimiterShared(r, policy)
}

// getRateLimiters returns the rate limiters map (implements RateLimiterManager).
func (r *GCPolicyReconciler) getRateLimiters() map[types.UID]*RateLimiter {
	return r.rateLimiters
}

// getRateLimitersMu returns the rate limiters mutex (implements RateLimiterManager).
func (r *GCPolicyReconciler) getRateLimitersMu() *sync.RWMutex {
	return &r.rateLimitersMu
}

// getConfig returns the controller config (implements RateLimiterManager).
func (r *GCPolicyReconciler) getConfig() *config.ControllerConfig {
	return r.config
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
	return deleteBatchShared(ctx, batch, policy, rateLimiter, reasons, r)
}

// DeleteResourceWithBackoff deletes a resource with exponential backoff (implements BatchDeleter).
func (r *GCPolicyReconciler) DeleteResourceWithBackoff(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *RateLimiter) error {
	return r.deleteResourceWithBackoff(ctx, resource, policy, rateLimiter)
}

// GetEventRecorder returns the event recorder (implements BatchDeleter).
func (r *GCPolicyReconciler) GetEventRecorder() *EventRecorder {
	return r.eventRecorder
}

// deleteResourceWithBackoff deletes a resource with exponential backoff retry logic.
func (r *GCPolicyReconciler) deleteResourceWithBackoff(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *RateLimiter) error {
	return deleteResourceWithBackoffShared(ctx, resource, policy, rateLimiter, r, nil)
}

// DeleteResourceWithContext deletes a resource with context (implements ResourceDeleterWithContext).
func (r *GCPolicyReconciler) DeleteResourceWithContext(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *RateLimiter) error {
	return r.deleteResource(ctx, resource, policy, rateLimiter)
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
				phase = PolicyPhasePaused
			} else {
				phase = PolicyPhaseActive
			}
		}
		phaseCounts[phase]++
	}

	// Update metrics for each phase
	for phase, count := range phaseCounts {
		recordPolicyPhase(phase, count)
	}

	// Reset phases that are no longer present
	knownPhases := []string{PolicyPhaseActive, PolicyPhasePaused, PolicyPhaseError}
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

