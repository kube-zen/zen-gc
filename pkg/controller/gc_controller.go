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
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
	"github.com/kube-zen/zen-gc/pkg/validation"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

const (
	// ReasonNoTTL indicates that TTL could not be calculated.
	ReasonNoTTL = "no_ttl"

	// DefaultGCInterval is the default interval for GC runs.
	DefaultGCInterval = 1 * time.Minute

	// DefaultMaxDeletionsPerSecond is the default rate limit.
	DefaultMaxDeletionsPerSecond = 10

	// DefaultBatchSize is the default batch size for deletions.
	DefaultBatchSize = 50

	// DefaultMaxConcurrentEvaluations is the default number of concurrent policy evaluations.
	DefaultMaxConcurrentEvaluations = 5

	// DefaultCacheSyncTimeout is the default timeout for cache synchronization.
	DefaultCacheSyncTimeout = 30 * time.Second

	// DefaultShutdownTimeout is the default timeout for graceful shutdown.
	DefaultShutdownTimeout = 30 * time.Second
)

var (
	// ErrPolicyInformerCacheSyncFailed indicates policy informer cache sync failed.
	ErrPolicyInformerCacheSyncFailed = errors.New("failed to sync policy informer cache")

	// ErrResourceInformerCacheSyncFailed indicates resource informer cache sync failed.
	ErrResourceInformerCacheSyncFailed = errors.New("failed to sync resource informer cache")

	// ErrNoMappingForFieldValue indicates no mapping found for field value.
	ErrNoMappingForFieldValue = errors.New("no mapping for field value")

	// ErrFieldPathNotFound indicates field path not found.
	ErrFieldPathNotFound = errors.New("field path not found")

	// ErrRelativeTimestampFieldNotFound indicates relative timestamp field not found.
	ErrRelativeTimestampFieldNotFound = errors.New("relative timestamp field not found")

	// ErrInvalidTimestampFormat indicates invalid timestamp format.
	ErrInvalidTimestampFormat = errors.New("invalid timestamp format")

	// ErrRelativeTTLExpired indicates relative TTL already expired.
	ErrRelativeTTLExpired = errors.New("relative TTL already expired")

	// ErrNoValidTTLConfiguration indicates no valid TTL configuration.
	ErrNoValidTTLConfiguration = errors.New("no valid TTL configuration")

	// ErrNoDeleter indicates no deleter was provided.
	ErrNoDeleter = errors.New("no deleter provided")

	// ErrCacheSyncFailed indicates that the cache sync failed.
	ErrCacheSyncFailed = errors.New("cache sync failed")
)

// GCController manages garbage collection policies.
type GCController struct {
	dynamicClient dynamic.Interface

	// Controller configuration.
	config *config.ControllerConfig

	// Policy informer factory.
	policyInformerFactory dynamicinformer.DynamicSharedInformerFactory

	// Policy informer.
	policyInformer cache.SharedInformer

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
	rateLimiters map[types.UID]*ratelimiter.RateLimiter

	// Mutex to protect rateLimiters map.
	rateLimitersMu sync.RWMutex

	// Status updater.
	statusUpdater *StatusUpdater

	// Event recorder.
	eventRecorder *EventRecorder

	// Context for cancellation.
	ctx    context.Context
	cancel context.CancelFunc

	// Shutdown synchronization
	shutdownComplete chan struct{}
	shutdownOnce     sync.Once
}

// NewGCController creates a new GC controller with default configuration.
// Deprecated: This implementation has been replaced by GCPolicyReconciler using controller-runtime.
// Use NewGCPolicyReconciler instead. This function is kept for backward compatibility with tests.
func NewGCController(dynamicClient dynamic.Interface, statusUpdater *StatusUpdater, eventRecorder *EventRecorder) (*GCController, error) {
	return NewGCControllerWithConfig(dynamicClient, statusUpdater, eventRecorder, config.NewControllerConfig())
}

// NewGCControllerWithConfig creates a new GC controller with the given configuration.
// Deprecated: This implementation has been replaced by GCPolicyReconciler using controller-runtime.
// Use NewGCPolicyReconciler instead. This function is kept for backward compatibility with tests.
func NewGCControllerWithConfig(dynamicClient dynamic.Interface, statusUpdater *StatusUpdater, eventRecorder *EventRecorder, cfg *config.ControllerConfig) (*GCController, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Use default config if nil
	if cfg == nil {
		cfg = config.NewControllerConfig()
	}

	// Create policy GVR
	policyGVR := schema.GroupVersionResource{
		Group:    "gc.kube-zen.io",
		Version:  "v1alpha1",
		Resource: "garbagecollectionpolicies",
	}

	// Create informer factory for policies using configured interval
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, cfg.GCInterval)

	// Create policy informer
	policyInformer := factory.ForResource(policyGVR).Informer()

	return &GCController{
		dynamicClient:             dynamicClient,
		config:                    cfg,
		policyInformerFactory:     factory,
		policyInformer:            policyInformer,
		resourceInformers:         make(map[types.UID]cache.SharedInformer),
		resourceInformerFactories: make(map[types.UID]dynamicinformer.DynamicSharedInformerFactory),
		rateLimiters:              make(map[types.UID]*ratelimiter.RateLimiter),
		statusUpdater:             statusUpdater,
		eventRecorder:             eventRecorder,
		ctx:                       ctx,
		cancel:                    cancel,
		shutdownComplete:          make(chan struct{}),
	}, nil
}

// Start starts the GC controller.
// Cache sync is performed asynchronously to avoid blocking startup.
func (gc *GCController) Start() error {
	logger := sdklog.NewLogger("zen-gc")
	logger.Info("Starting GC controller", sdklog.Operation("start_controller"))

	// Add event handlers to policy informer to handle policy deletions and updates
	_, err := gc.policyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			gc.handlePolicyDelete(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			gc.handlePolicyUpdate(oldObj, newObj)
		},
	})
	if err != nil {
		logger.Error(err, "Failed to add event handlers to policy informer", sdklog.Operation("setup_informer"), sdklog.ErrorCode("SETUP_INFORMER_FAILED"))
		return err
	}

	// Start policy informer factory (non-blocking)
	gc.policyInformerFactory.Start(gc.ctx.Done())

	// Start cache sync in background and GC loop
	// The GC loop will wait for cache sync before evaluating policies
	go gc.waitForCacheSyncAndStart()

	logger.Info("GC controller started (cache sync in progress)", sdklog.Operation("start_controller"))
	return nil
}

// waitForCacheSyncAndStart waits for cache sync and then starts the GC loop.
// This allows the controller to start quickly while cache sync happens in background.
func (gc *GCController) waitForCacheSyncAndStart() {
	// Wait for cache sync with timeout
	syncCtx, syncCancel := context.WithTimeout(gc.ctx, DefaultCacheSyncTimeout)
	defer syncCancel()

	// Create logger
	logger := sdklog.NewLogger("zen-gc")

	logger.Debug("Waiting for policy informer cache to sync", sdklog.Operation("cache_sync"))
	startTime := time.Now()

	if !cache.WaitForCacheSync(syncCtx.Done(), gc.policyInformer.HasSynced) {
		syncDuration := time.Since(startTime)
		if syncCtx.Err() != nil {
			logger.Error(syncCtx.Err(), "Policy informer cache sync timed out",
				sdklog.Duration("duration", syncDuration))
			return
		}
		logger.Error(ErrCacheSyncFailed, "Policy informer cache sync failed", sdklog.Operation("cache_sync"), sdklog.ErrorCode("CACHE_SYNC_FAILED"),
			sdklog.Duration("duration", syncDuration))
		return
	}

	syncDuration := time.Since(startTime)
	logger.Info("Policy informer cache synced successfully", sdklog.Operation("cache_sync"),
		sdklog.Duration("duration", syncDuration))

	// Start GC loop once cache is synced
	go gc.runGCLoop()
}

// Stop stops the GC controller gracefully.
// It waits for in-flight operations to complete (with timeout) before cleaning up.
func (gc *GCController) Stop() {
	gc.shutdownOnce.Do(func() {
		gc.stop()
	})
}

// stop performs the actual shutdown logic.
func (gc *GCController) stop() {
	logger := sdklog.NewLogger("zen-gc")
	logger.Info("Stopping GC controller gracefully")
	shutdownStart := time.Now()

	// Cancel context to signal shutdown to all goroutines
	gc.cancel()

	// Wait for GC loop to finish current evaluation (with timeout)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer shutdownCancel()

	// Wait for shutdown completion or timeout
	select {
	case <-gc.shutdownComplete:
		logger.Info("GC controller stopped gracefully", sdklog.Operation("shutdown"))
	case <-shutdownCtx.Done():
		logger.Warn("GC controller shutdown timed out, forcing cleanup", sdklog.Operation("shutdown"), sdklog.Duration("timeout", DefaultShutdownTimeout))
	}

	// Clean up all resource informers and rate limiters
	gc.cleanupAllResourceInformers()
	gc.cleanupAllRateLimiters()

	shutdownDuration := time.Since(shutdownStart)
	logger.Info("GC controller shutdown completed", sdklog.Operation("shutdown"), sdklog.Duration("duration", shutdownDuration))
}

// runGCLoop runs the main GC evaluation loop.
func (gc *GCController) runGCLoop() {
	defer close(gc.shutdownComplete)

	interval := DefaultGCInterval
	if gc.config != nil {
		interval = gc.config.GCInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger := sdklog.NewLogger("zen-gc")
	for {
		select {
		case <-gc.ctx.Done():
			logger.Info("GC loop stopping: context canceled", sdklog.Operation("gc_loop"))
			// Allow current evaluation to complete if in progress
			// The evaluatePolicies function checks context cancellation internally
			return
		case <-ticker.C:
			gc.evaluatePolicies()
		}
	}
}

// evaluatePolicies evaluates all policies and performs GC.
// Policies are evaluated in parallel using a worker pool pattern.
func (gc *GCController) evaluatePolicies() {
	logger := sdklog.NewLogger("zen-gc")
	// Check if context is canceled before starting
	select {
	case <-gc.ctx.Done():
		logger.Debug("Skipping policy evaluation: context canceled", sdklog.Operation("evaluate_policies"))
		return
	default:
	}

	// Ensure cache is synced before evaluating policies
	if !gc.policyInformer.HasSynced() {
		logger.Debug("Skipping policy evaluation: cache not yet synced", sdklog.Operation("evaluate_policies"))
		return
	}

	// Get all policies from cache
	policies := gc.policyInformer.GetStore().List()

	// Determine concurrency limit
	maxConcurrent := DefaultMaxConcurrentEvaluations
	if gc.config != nil && gc.config.MaxConcurrentEvaluations > 0 {
		maxConcurrent = gc.config.MaxConcurrentEvaluations
	}

	// If we have fewer policies than the concurrency limit, use sequential evaluation
	if len(policies) <= maxConcurrent {
		gc.evaluatePoliciesSequential(policies)
		return
	}

	// Use worker pool for parallel evaluation
	gc.evaluatePoliciesParallel(policies, maxConcurrent)

	// Record policy phase metrics after evaluation
	gc.recordPolicyPhaseMetrics()
}

// recordPolicyPhaseMetrics records metrics for policy phases.
func (gc *GCController) recordPolicyPhaseMetrics() {
	if !gc.policyInformer.HasSynced() {
		return
	}

	policies := gc.policyInformer.GetStore().List()
	phaseCounts := make(map[string]float64)

	for _, obj := range policies {
		policy := gc.convertToPolicy(obj)
		if policy == nil {
			continue
		}

		phase := policy.Status.Phase
		if phase == "" {
			phase = PolicyPhaseActive
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

// evaluatePoliciesSequential evaluates policies sequentially (for small numbers).
func (gc *GCController) evaluatePoliciesSequential(policies []interface{}) {
	logger := sdklog.NewLogger("zen-gc")
	for _, obj := range policies {
		// Check context cancellation between policy evaluations
		select {
		case <-gc.ctx.Done():
			logger.Debug("Stopping policy evaluation: context canceled", sdklog.Operation("evaluate_policies_sequential"))
			return
		default:
		}

		policy := gc.convertToPolicy(obj)
		if policy == nil {
			continue
		}

		// Skip paused policies (check spec.paused, not status.phase)
		if policy.Spec.Paused {
			continue
		}

		if err := gc.evaluatePolicy(policy); err != nil {
			gcErr := gcerrors.WithPolicy(err, policy.Namespace, policy.Name)
			if gcErr.Type == "" {
				gcErr.Type = ErrorTypeEvaluationFailed
			}
			logger.Error(gcErr, "Error evaluating policy", sdklog.Operation("evaluate_policy"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.ErrorCode("EVALUATE_POLICY_FAILED"))
		}
	}
}

// evaluatePoliciesParallel evaluates policies in parallel using a worker pool.
func (gc *GCController) evaluatePoliciesParallel(policies []interface{}, maxConcurrent int) {
	// Create a channel for policies to evaluate
	policyChan := make(chan *v1alpha1.GarbageCollectionPolicy, len(policies))

	// Convert policies and send to channel
	for _, obj := range policies {
		policy := gc.convertToPolicy(obj)
		if policy == nil {
			continue
		}
		// Skip paused policies (check spec.paused, not status.phase)
		if policy.Spec.Paused {
			continue
		}
		policyChan <- policy
	}
	close(policyChan)

	// Create worker pool
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrent)

	logger := sdklog.NewLogger("zen-gc")
	// Process policies with worker pool
	for policy := range policyChan {
		// Check context cancellation
		select {
		case <-gc.ctx.Done():
			logger.Debug("Stopping policy evaluation: context canceled", sdklog.Operation("evaluate_policies_parallel"))
			// Drain remaining policies from channel
			for range policyChan {
			}
			wg.Wait()
			return
		default:
		}

		// Acquire semaphore
		semaphore <- struct{}{}
		wg.Add(1)

		// Evaluate policy in goroutine
		go func(p *v1alpha1.GarbageCollectionPolicy) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			if err := gc.evaluatePolicy(p); err != nil {
				gcErr := gcerrors.WithPolicy(err, p.Namespace, p.Name)
				if gcErr.Type == "" {
					gcErr.Type = ErrorTypeEvaluationFailed
				}
				logger.Error(gcErr, "Error evaluating policy", sdklog.Operation("evaluate_policy"), sdklog.String("policy", p.Namespace+"/"+p.Name), sdklog.ErrorCode("EVALUATE_POLICY_FAILED"))
			}
		}(policy)
	}

	// Wait for all workers to complete
	wg.Wait()
}

// convertToPolicy converts an unstructured object to a GarbageCollectionPolicy.
// Returns nil if conversion fails or object type is unexpected.
func (gc *GCController) convertToPolicy(obj interface{}) *v1alpha1.GarbageCollectionPolicy {
	logger := sdklog.NewLogger("zen-gc")
	// Convert unstructured to GarbageCollectionPolicy
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		logger.Warn("Unexpected object type in policy informer", sdklog.Operation("convert_to_policy"), sdklog.String("type", fmt.Sprintf("%T", obj)))
		return nil
	}

	// Convert to typed object
	policy := &v1alpha1.GarbageCollectionPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, policy); err != nil {
		gcErr := gcerrors.Wrap(err, "conversion_failed", "failed to convert unstructured to GarbageCollectionPolicy")
		gcErr.PolicyNamespace = unstructuredObj.GetNamespace()
		gcErr.PolicyName = unstructuredObj.GetName()
		logger.Error(gcErr, "Error converting unstructured to GarbageCollectionPolicy", sdklog.Operation("convert_to_policy"), sdklog.ErrorCode("CONVERSION_FAILED"))
		return nil
	}

	return policy
}

// evaluatePolicy evaluates a single policy.
//
//nolint:gocyclo // Policy evaluation logic is inherently complex
func (gc *GCController) evaluatePolicy(policy *v1alpha1.GarbageCollectionPolicy) error {
	logger := sdklog.NewLogger("zen-gc")
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		recordEvaluationDuration(policy.Namespace, policy.Name, duration)
	}()

	logger.Debug("Evaluating policy", sdklog.Operation("evaluate_policy"), sdklog.String("policy", policy.Namespace+"/"+policy.Name))

	// Get or create resource informer for this policy
	informer, err := gc.getOrCreateResourceInformer(policy)
	if err != nil {
		gcErr := gcerrors.Wrap(err, "informer_creation_failed", "failed to get resource informer")
		gcErr.PolicyNamespace = policy.Namespace
		gcErr.PolicyName = policy.Name
		recordError(policy.Namespace, policy.Name, "informer_creation_failed")
		logger.Error(gcErr, "Error creating resource informer for policy", sdklog.Operation("evaluate_policy"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.ErrorCode("INFORMER_CREATION_FAILED"))
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
		case <-gc.ctx.Done():
			logger.Debug("Stopping policy evaluation: context canceled", sdklog.Operation("evaluate_policy"), sdklog.String("policy", policy.Namespace+"/"+policy.Name))
			return nil
		default:
		}

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

		// Add to deletion list
		resourcesToDelete = append(resourcesToDelete, resource)
		resourcesToDeleteReasons[string(resource.GetUID())] = reason
	}

	// Delete resources in batches
	if len(resourcesToDelete) > 0 {
		rateLimiter := gc.getOrCreateRateLimiter(policy)
		batchSize := gc.getBatchSize(policy)

		// Process deletions in batches
		for i := 0; i < len(resourcesToDelete); i += batchSize {
			// Check context cancellation between batches
			select {
			case <-gc.ctx.Done():
				logger.Debug("Stopping batch deletion: context canceled", sdklog.Operation("delete_batch"), sdklog.String("policy", policy.Namespace+"/"+policy.Name))
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
			batchDeleted, batchErrors := gc.deleteBatch(gc.ctx, batch, policy, rateLimiter, resourcesToDeleteReasons)
			deletedCount += batchDeleted

			// Track deletion failures
			if len(batchErrors) > 0 {
				recordError(policy.Namespace, policy.Name, "deletion_failed")
			}

			// Log errors
			for _, err := range batchErrors {
				if gc.eventRecorder != nil {
					gc.eventRecorder.RecordEvaluationFailed(policy, err)
				}
				logger.Error(err, "Error deleting batch for policy", sdklog.Operation("delete_batch"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.ErrorCode("DELETE_BATCH_FAILED"))
			}

			// Log deletion attempt metrics
			logger.Debug("Policy deletion batch completed", sdklog.Operation("delete_batch"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.Int64("attempted", deletionAttempts), sdklog.Int64("succeeded", batchDeleted), sdklog.Int64("failed", int64(len(batchErrors))))
		}
	}

	// Record pending resources metric
	if pendingCount > 0 {
		recordResourcesPending(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind, pendingCount)
	}

	// Update policy status with timeout context
	if gc.statusUpdater != nil {
		// Use timeout context for status updates to prevent hanging
		statusCtx, statusCancel := context.WithTimeout(gc.ctx, 10*time.Second)
		defer statusCancel()

		if err := gc.statusUpdater.UpdateStatus(statusCtx, policy, matchedCount, deletedCount, pendingCount); err != nil {
			// Check if error is due to context cancellation/timeout
			if statusCtx.Err() != nil {
				logger.Debug("Status update canceled or timed out", sdklog.Operation("update_status"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.Error(statusCtx.Err()))
				return nil // Don't treat cancellation as error
			}
			gcErr := gcerrors.Wrap(err, "status_update_failed", "failed to update policy status")
			gcErr.PolicyNamespace = policy.Namespace
			gcErr.PolicyName = policy.Name
			recordError(policy.Namespace, policy.Name, "status_update_failed")
			if gc.eventRecorder != nil {
				gc.eventRecorder.RecordStatusUpdateFailed(policy, gcErr)
			}
			logger.Error(gcErr, "Error updating policy status", sdklog.Operation("update_status"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.ErrorCode("UPDATE_STATUS_FAILED"))
		}
	}

	// Record policy evaluation event
	if gc.eventRecorder != nil {
		gc.eventRecorder.RecordPolicyEvaluated(policy, matchedCount, deletedCount, pendingCount)
	}

	return nil
}

// matchesSelectors checks if a resource matches the target resource selectors.
func (gc *GCController) matchesSelectors(resource *unstructured.Unstructured, target *v1alpha1.TargetResourceSpec) bool {
	return matchesSelectorsShared(resource, target)
}

// shouldDelete determines if a resource should be deleted based on TTL and conditions.
func (gc *GCController) shouldDelete(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) (shouldDelete bool, reason string) {
	// Check conditions first
	if policy.Spec.Conditions != nil {
		if !gc.meetsConditions(resource, policy.Spec.Conditions) {
			return false, ReasonConditionNotMet
		}
	}

	// Calculate expiration time
	expirationTime, err := gc.calculateExpirationTime(resource, &policy.Spec.TTL)
	if err != nil {
		logger := sdklog.NewLogger("zen-gc")
		logger.Debug("Could not calculate expiration time for resource", sdklog.Operation("should_delete"), sdklog.String("resource", resource.GetNamespace()+"/"+resource.GetName()), sdklog.Error(err))
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
func (gc *GCController) calculateExpirationTime(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (time.Time, error) {
	return calculateExpirationTimeShared(resource, ttlSpec)
}

// calculateTTL calculates the TTL in seconds for a resource based on policy.
// Deprecated: Use calculateExpirationTime instead for accurate relative TTL handling.
// This function is kept for backward compatibility and testing.
func (gc *GCController) calculateTTL(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (int64, error) {
	expirationTime, err := gc.calculateExpirationTime(resource, ttlSpec)
	if err != nil {
		return 0, err
	}

	// For relative TTL, return seconds remaining
	if ttlSpec.RelativeTo != "" && ttlSpec.SecondsAfter != nil {
		ttlSeconds := int64(time.Until(expirationTime).Seconds())
		if ttlSeconds <= 0 {
			return 0, fmt.Errorf("%w", ErrRelativeTTLExpired)
		}
		return ttlSeconds, nil
	}

	// For other TTL modes, return seconds after creation
	creationTime := resource.GetCreationTimestamp().Time
	return int64(expirationTime.Sub(creationTime).Seconds()), nil
}

// meetsConditions checks if a resource meets the deletion conditions.
func (gc *GCController) meetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	return meetsConditionsShared(resource, conditions)
}

// matchesFieldOperator checks if field value matches the operator condition.
func (gc *GCController) matchesFieldOperator(fieldValue string, fieldCond v1alpha1.FieldCondition) bool {
	return matchesFieldOperatorShared(fieldValue, fieldCond)
}

// deleteResource deletes a resource based on policy behavior.
func (gc *GCController) deleteResource(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter) error {
	// Rate limiting
	if err := rateLimiter.Wait(gc.ctx); err != nil {
		return err
	}

	// Dry run check
	if policy.Spec.Behavior.DryRun {
		logger := sdklog.NewLogger("zen-gc")
		logger.Info("[DRY RUN] Would delete resource", sdklog.Operation("delete_resource"), sdklog.String("resource", resource.GetNamespace()+"/"+resource.GetName()))
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
		err = gc.dynamicClient.Resource(gvr).Delete(gc.ctx, resource.GetName(), *deleteOptions)
	} else {
		err = gc.dynamicClient.Resource(gvr).Namespace(namespace).Delete(gc.ctx, resource.GetName(), *deleteOptions)
	}

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

// getOrCreateResourceInformer gets or creates a resource informer for a policy.
func (gc *GCController) getOrCreateResourceInformer(policy *v1alpha1.GarbageCollectionPolicy) (cache.SharedInformer, error) {
	// Check if informer already exists (with read lock)
	gc.resourceInformersMu.RLock()
	if informer, ok := gc.resourceInformers[policy.UID]; ok {
		gc.resourceInformersMu.RUnlock()
		return informer, nil
	}
	gc.resourceInformersMu.RUnlock()

	// Acquire write lock for creating new informer
	gc.resourceInformersMu.Lock()
	defer gc.resourceInformersMu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have created it)
	if informer, ok := gc.resourceInformers[policy.UID]; ok {
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
	if gc.config != nil {
		interval = gc.config.GCInterval
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		gc.dynamicClient,
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
	gc.resourceInformers[policy.UID] = informer
	gc.resourceInformerFactories[policy.UID] = factory

	// Update metrics
	recordInformerCount(len(gc.resourceInformers))

	// Start informer factory
	factory.Start(gc.ctx.Done())

	// Wait for cache sync with timeout
	syncCtx, syncCancel := context.WithTimeout(gc.ctx, DefaultCacheSyncTimeout)
	defer syncCancel()

	if !cache.WaitForCacheSync(syncCtx.Done(), informer.HasSynced) {
		// Clean up on failure
		delete(gc.resourceInformers, policy.UID)
		delete(gc.resourceInformerFactories, policy.UID)
		if syncCtx.Err() != nil {
			return nil, fmt.Errorf("resource informer cache sync timed out: %w", syncCtx.Err())
		}
		return nil, fmt.Errorf("%w", ErrResourceInformerCacheSyncFailed)
	}

	logger := sdklog.NewLogger("zen-gc")
	logger.Debug("Created resource informer for policy", sdklog.Operation("get_or_create_informer"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.String("uid", string(policy.UID)))
	return informer, nil
}

// handlePolicyDelete handles policy deletion events and cleans up associated resource informers.
func (gc *GCController) handlePolicyDelete(obj interface{}) {
	logger := sdklog.NewLogger("zen-gc")
	var policy *v1alpha1.GarbageCollectionPolicy

	// Handle different object types (unstructured or typed)
	switch o := obj.(type) {
	case *unstructured.Unstructured:
		policy = &v1alpha1.GarbageCollectionPolicy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, policy); err != nil {
			logger.Error(err, "Error converting unstructured to GarbageCollectionPolicy in delete handler", sdklog.Operation("handle_policy_delete"), sdklog.ErrorCode("CONVERSION_FAILED"))
			return
		}
	case *v1alpha1.GarbageCollectionPolicy:
		policy = o
	case cache.DeletedFinalStateUnknown:
		// Handle deleted final state unknown
		if unstructuredObj, ok := o.Obj.(*unstructured.Unstructured); ok {
			policy = &v1alpha1.GarbageCollectionPolicy{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, policy); err != nil {
				gcErr := gcerrors.Wrap(err, "conversion_failed", "failed to convert unstructured to GarbageCollectionPolicy in delete handler")
				logger.Error(gcErr, "Error converting unstructured to GarbageCollectionPolicy in delete handler", sdklog.Operation("handle_policy_delete"), sdklog.ErrorCode("CONVERSION_FAILED"))
				return
			}
		} else {
			logger.Warn("Unexpected object type in delete handler", sdklog.Operation("handle_policy_delete"), sdklog.String("type", fmt.Sprintf("%T", o.Obj)))
			return
		}
	default:
		logger.Warn("Unexpected object type in delete handler", sdklog.Operation("handle_policy_delete"), sdklog.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	if policy == nil {
		return
	}

	logger.Info("Policy deleted, cleaning up resource informer and rate limiter", sdklog.Operation("handle_policy_delete"), sdklog.String("policy", policy.Namespace+"/"+policy.Name))
	gc.cleanupResourceInformer(policy.UID)
	gc.cleanupRateLimiter(policy.UID)
}

// handlePolicyUpdate handles policy update events and recreates informer if needed.
func (gc *GCController) handlePolicyUpdate(oldObj, newObj interface{}) {
	logger := sdklog.NewLogger("zen-gc")
	var oldPolicy, newPolicy *v1alpha1.GarbageCollectionPolicy

	// Convert old object
	switch o := oldObj.(type) {
	case *unstructured.Unstructured:
		oldPolicy = &v1alpha1.GarbageCollectionPolicy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, oldPolicy); err != nil {
			gcErr := gcerrors.Wrap(err, "conversion_failed", "failed to convert old unstructured to GarbageCollectionPolicy")
			logger.Error(gcErr, "Error converting old unstructured to GarbageCollectionPolicy", sdklog.Operation("handle_policy_update"), sdklog.ErrorCode("CONVERSION_FAILED"))
			return
		}
	case *v1alpha1.GarbageCollectionPolicy:
		oldPolicy = o
	default:
		return
	}

	// Convert new object
	switch o := newObj.(type) {
	case *unstructured.Unstructured:
		newPolicy = &v1alpha1.GarbageCollectionPolicy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, newPolicy); err != nil {
			gcErr := gcerrors.Wrap(err, "conversion_failed", "failed to convert new unstructured to GarbageCollectionPolicy")
			logger.Error(gcErr, "Error converting new unstructured to GarbageCollectionPolicy", sdklog.Operation("handle_policy_update"), sdklog.ErrorCode("CONVERSION_FAILED"))
			return
		}
	case *v1alpha1.GarbageCollectionPolicy:
		newPolicy = o
	default:
		return
	}

	if oldPolicy == nil || newPolicy == nil {
		return
	}

	// Check if target resource spec changed (which would require informer recreation)
	oldSpec := oldPolicy.Spec.TargetResource
	newSpec := newPolicy.Spec.TargetResource

	// Compare key fields that affect informer creation
	if oldSpec.APIVersion != newSpec.APIVersion ||
		oldSpec.Kind != newSpec.Kind ||
		oldSpec.Namespace != newSpec.Namespace ||
		!labelSelectorsEqual(oldSpec.LabelSelector, newSpec.LabelSelector) {
		logger.Info("Policy target resource spec changed, recreating informer", sdklog.Operation("handle_policy_update"), sdklog.String("policy", newPolicy.Namespace+"/"+newPolicy.Name))
		// Clean up old informer
		gc.cleanupResourceInformer(oldPolicy.UID)
		// New informer will be created on next evaluation
	}
}

// labelSelectorsEqual compares two label selectors for equality.
func labelSelectorsEqual(a, b *metav1.LabelSelector) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to selectors and compare their string representations
	selectorA, errA := metav1.LabelSelectorAsSelector(a)
	selectorB, errB := metav1.LabelSelectorAsSelector(b)

	// If either conversion fails, consider them different
	if errA != nil || errB != nil {
		return false
	}

	// Compare selector string representations
	return selectorA.String() == selectorB.String()
}

// cleanupResourceInformer cleans up a resource informer for a given policy UID.
func (gc *GCController) cleanupResourceInformer(policyUID types.UID) {
	gc.resourceInformersMu.Lock()
	defer gc.resourceInformersMu.Unlock()

	_, informerExists := gc.resourceInformers[policyUID]
	_, factoryExists := gc.resourceInformerFactories[policyUID]

	if !informerExists && !factoryExists {
		// Already cleaned up or never existed
		return
	}

	// Stop the informer factory (which will stop all informers created by it)
	if factoryExists {
		// DynamicSharedInformerFactory doesn't have a Stop method,
		// but stopping is handled by context cancellation.
		// We just need to remove it from our tracking.
		delete(gc.resourceInformerFactories, policyUID)
	}

	// Remove informer from map
	if informerExists {
		delete(gc.resourceInformers, policyUID)
		logger := sdklog.NewLogger("zen-gc")
		logger.Debug("Cleaned up resource informer for policy", sdklog.Operation("cleanup_informer"), sdklog.String("uid", string(policyUID)))
	}

	// Update metrics
	recordInformerCount(len(gc.resourceInformers))
}

// cleanupAllResourceInformers cleans up all resource informers (used during shutdown).
func (gc *GCController) cleanupAllResourceInformers() {
	gc.resourceInformersMu.Lock()
	defer gc.resourceInformersMu.Unlock()

	count := len(gc.resourceInformers)
	if count > 0 {
		logger := sdklog.NewLogger("zen-gc")
		logger.Info("Cleaning up resource informers during shutdown", sdklog.Operation("cleanup_all_informers"), sdklog.Int("count", count))
	}

	// Clear all informers and factories
	gc.resourceInformers = make(map[types.UID]cache.SharedInformer)
	gc.resourceInformerFactories = make(map[types.UID]dynamicinformer.DynamicSharedInformerFactory)
}

// getOrCreateRateLimiter gets or creates a rate limiter for a policy.
func (gc *GCController) getOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter {
	return getOrCreateRateLimiterShared(gc, policy)
}

// getRateLimiters returns the rate limiters map (implements RateLimiterManager).
func (gc *GCController) getRateLimiters() map[types.UID]*ratelimiter.RateLimiter {
	return gc.rateLimiters
}

// getRateLimitersMu returns the rate limiters mutex (implements RateLimiterManager).
func (gc *GCController) getRateLimitersMu() *sync.RWMutex {
	return &gc.rateLimitersMu
}

// getConfig returns the controller config (implements RateLimiterManager).
func (gc *GCController) getConfig() *config.ControllerConfig {
	return gc.config
}

// cleanupRateLimiter cleans up a rate limiter for a given policy UID.
func (gc *GCController) cleanupRateLimiter(policyUID types.UID) {
	gc.rateLimitersMu.Lock()
	defer gc.rateLimitersMu.Unlock()

	if _, exists := gc.rateLimiters[policyUID]; exists {
		delete(gc.rateLimiters, policyUID)
		logger := sdklog.NewLogger("zen-gc")
		logger.Debug("Cleaned up rate limiter for policy", sdklog.Operation("cleanup_rate_limiter"), sdklog.String("uid", string(policyUID)))
	}

	// Update metrics
	recordRateLimiterCount(len(gc.rateLimiters))
}

// cleanupAllRateLimiters cleans up all rate limiters (used during shutdown).
func (gc *GCController) cleanupAllRateLimiters() {
	gc.rateLimitersMu.Lock()
	defer gc.rateLimitersMu.Unlock()

	count := len(gc.rateLimiters)
	if count > 0 {
		logger := sdklog.NewLogger("zen-gc")
		logger.Info("Cleaning up rate limiters during shutdown", sdklog.Operation("cleanup_all_rate_limiters"), sdklog.Int("count", count))
	}

	gc.rateLimiters = make(map[types.UID]*ratelimiter.RateLimiter)
}

// getBatchSize returns the batch size for a policy.
func (gc *GCController) getBatchSize(policy *v1alpha1.GarbageCollectionPolicy) int {
	batchSize := DefaultBatchSize
	if gc.config != nil {
		batchSize = gc.config.BatchSize
	}
	if policy.Spec.Behavior.BatchSize > 0 {
		batchSize = policy.Spec.Behavior.BatchSize
	}
	return batchSize
}

// deleteBatch deletes a batch of resources.
// Returns the number of successfully deleted resources and any errors encountered.
func (gc *GCController) deleteBatch(
	ctx context.Context,
	batch []*unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	rateLimiter *ratelimiter.RateLimiter,
	reasons map[string]string,
) (int64, []error) {
	return deleteBatchShared(ctx, batch, policy, rateLimiter, reasons, gc)
}

// DeleteResourceWithBackoff deletes a resource with exponential backoff (implements BatchDeleter).
// This wraps the existing deleteResourceWithBackoff method.
func (gc *GCController) DeleteResourceWithBackoff(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter) error {
	return gc.deleteResourceWithBackoff(ctx, resource, policy, rateLimiter)
}

// GetEventRecorder returns the event recorder (implements BatchDeleter).
func (gc *GCController) GetEventRecorder() *EventRecorder {
	return gc.eventRecorder
}
