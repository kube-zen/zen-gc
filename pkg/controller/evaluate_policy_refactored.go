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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
	"github.com/kube-zen/zen-gc/pkg/validation"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// PolicyEvaluationService provides policy evaluation using injected dependencies.
// This is the refactored version that uses interfaces for better testability.
type PolicyEvaluationService struct {
	resourceLister      ResourceLister
	selectorMatcher     SelectorMatcher
	conditionMatcher    ConditionMatcher
	ttlCalculator       TTLCalculator
	rateLimiterProvider RateLimiterProvider
	batchDeleter        BatchDeleterCore
	statusUpdater       *StatusUpdater
	eventRecorder       *EventRecorder
	logger              *sdklog.Logger
}

// NewPolicyEvaluationService creates a new PolicyEvaluationService with injected dependencies.
func NewPolicyEvaluationService(
	resourceLister ResourceLister,
	selectorMatcher SelectorMatcher,
	conditionMatcher ConditionMatcher,
	ttlCalculator TTLCalculator,
	rateLimiterProvider RateLimiterProvider,
	batchDeleter BatchDeleterCore,
	statusUpdater *StatusUpdater,
	eventRecorder *EventRecorder,
	logger *sdklog.Logger,
) *PolicyEvaluationService {
	if logger == nil {
		logger = sdklog.NewLogger("zen-gc")
	}
	return &PolicyEvaluationService{
		resourceLister:      resourceLister,
		selectorMatcher:     selectorMatcher,
		conditionMatcher:    conditionMatcher,
		ttlCalculator:       ttlCalculator,
		rateLimiterProvider: rateLimiterProvider,
		batchDeleter:        batchDeleter,
		statusUpdater:       statusUpdater,
		eventRecorder:       eventRecorder,
		logger:              logger,
	}
}

// EvaluatePolicy evaluates a policy using the injected dependencies.
// This is the refactored version that's much easier to test.
func (s *PolicyEvaluationService) EvaluatePolicy(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) error {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Seconds()
		recordEvaluationDuration(policy.Namespace, policy.Name, duration)
	}()

	s.logger.Debug("Evaluating policy", sdklog.Operation("evaluate_policy"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)))

	// Parse GVR from policy
	gvr, err := parseGVR(policy.Spec.TargetResource.APIVersion, policy.Spec.TargetResource.Kind)
	if err != nil {
		gcErr := gcerrors.Wrap(err, "invalid_gvr", "failed to parse GVR")
		gcErr = gcErr.WithContext("policy_namespace", policy.Namespace)
		gcErr = gcErr.WithContext("policy_name", policy.Name)
		recordError(policy.Namespace, policy.Name, "invalid_gvr")
		s.logger.Error(gcErr, "Invalid GVR in policy", sdklog.Operation("evaluate_policy"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)), sdklog.ErrorCode("INVALID_GVR"))
		return gcErr
	}

	// Get namespace (use "*" for all namespaces if empty)
	namespace := policy.Spec.TargetResource.Namespace
	if namespace == "" {
		namespace = "*"
	}

	// List resources using ResourceLister interface
	resources, err := s.resourceLister.ListResources(ctx, gvr, namespace)
	if err != nil {
		gcErr := gcerrors.Wrap(err, "list_resources_failed", "failed to list resources")
		gcErr = gcErr.WithContext("policy_namespace", policy.Namespace)
		gcErr = gcErr.WithContext("policy_name", policy.Name)
		recordError(policy.Namespace, policy.Name, "list_resources_failed")
		s.logger.Error(gcErr, "Error listing resources", sdklog.Operation("evaluate_policy"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)), sdklog.ErrorCode("LIST_RESOURCES_FAILED"))
		return gcErr
	}

	var matchedCount, deletedCount, pendingCount int64

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	// Pre-allocate with estimated capacity
	estimatedDeletions := len(resources) / 10
	if estimatedDeletions < 10 {
		estimatedDeletions = 10
	}
	resourcesToDelete := make([]*unstructured.Unstructured, 0, estimatedDeletions)
	resourcesToDeleteReasons := make(map[string]string, estimatedDeletions)

	// Evaluate each resource
	matchedCount, pendingCount = s.evaluateResources(ctx, resources, policy, &resourcesToDelete, resourcesToDeleteReasons, resourceAPIVersion, resourceKind)

	// Delete resources in batches using BatchDeleterCore interface
	if len(resourcesToDelete) > 0 {
		deletedCount = s.deleteResourcesInBatches(ctx, policy, resourcesToDelete, resourcesToDeleteReasons)
	}

	// Record pending resources metric
	if pendingCount > 0 {
		recordResourcesPending(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind, pendingCount)
	}

	// Update policy status
	if err := s.updatePolicyStatus(ctx, policy, matchedCount, deletedCount, pendingCount); err != nil {
		return err
	}

	// Record policy evaluation event
	if s.eventRecorder != nil {
		s.eventRecorder.RecordPolicyEvaluated(policy, matchedCount, deletedCount, pendingCount)
	}

	return nil
}

// evaluateResources evaluates all resources and builds the deletion list.
func (s *PolicyEvaluationService) evaluateResources(
	ctx context.Context,
	resources []*unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	resourcesToDelete *[]*unstructured.Unstructured,
	resourcesToDeleteReasons map[string]string,
	resourceAPIVersion, resourceKind string,
) (matchedCount, pendingCount int64) {
	// Check context cancellation at start to avoid unnecessary work
	select {
	case <-ctx.Done():
		s.logger.Debug("Stopping policy evaluation: context canceled", sdklog.Operation("evaluate_policy"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)))
		return int64(0), int64(0)
	default:
	}

	const contextCheckInterval = 100
	for i, resource := range resources {
		// Check context cancellation periodically
		if i%contextCheckInterval == 0 {
			select {
			case <-ctx.Done():
				s.logger.Debug("Stopping policy evaluation: context canceled", sdklog.Operation("evaluate_policy"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)))
				return matchedCount, pendingCount
			default:
			}
		}

		// Check if resource matches selectors using SelectorMatcher interface
		if !s.selectorMatcher.MatchesSelectors(resource, &policy.Spec.TargetResource) {
			continue
		}

		matchedCount++
		recordResourceMatched(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind)

		// Check conditions using ConditionMatcher interface
		if policy.Spec.Conditions != nil {
			if !s.conditionMatcher.MeetsConditions(resource, policy.Spec.Conditions) {
				pendingCount++
				continue
			}
		}

		// Check TTL using shared function (TTLCalculator interface is for future use)
		shouldDelete, reason := s.shouldDelete(resource, policy)
		if !shouldDelete {
			pendingCount++
			continue
		}

		// Add to deletion list
		*resourcesToDelete = append(*resourcesToDelete, resource)
		resourcesToDeleteReasons[string(resource.GetUID())] = reason
	}
	return matchedCount, pendingCount
}

// deleteResourcesInBatches deletes resources in batches.
func (s *PolicyEvaluationService) deleteResourcesInBatches(
	ctx context.Context,
	policy *v1alpha1.GarbageCollectionPolicy,
	resourcesToDelete []*unstructured.Unstructured,
	resourcesToDeleteReasons map[string]string,
) int64 {
	// Check context cancellation at start
	select {
	case <-ctx.Done():
		s.logger.Debug("Stopping batch deletion: context canceled", sdklog.Operation("delete_batch"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)))
		return 0
	default:
	}

	rateLimiter := s.rateLimiterProvider.GetOrCreateRateLimiter(policy)
	if rateLimiter == nil {
		s.logger.Error(nil, "Rate limiter is nil, cannot proceed with deletions", sdklog.Operation("delete_batch"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)), sdklog.ErrorCode("RATE_LIMITER_NIL"))
		return 0
	}
	batchSize := s.getBatchSize(policy)
	deletedCount := int64(0)

	// Process deletions in batches
	for i := 0; i < len(resourcesToDelete); i += batchSize {
		// Check context cancellation between batches
		select {
		case <-ctx.Done():
			s.logger.Debug("Stopping batch deletion: context canceled", sdklog.Operation("delete_batch"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)))
			return deletedCount
		default:
		}

		end := i + batchSize
		if end > len(resourcesToDelete) {
			end = len(resourcesToDelete)
		}
		batch := resourcesToDelete[i:end]

		// Delete batch using BatchDeleterCore interface
		batchDeleted, batchErrors := s.batchDeleter.DeleteBatch(ctx, batch, policy, rateLimiter, resourcesToDeleteReasons)
		deletedCount += batchDeleted

		// Track deletion failures
		if len(batchErrors) > 0 {
			recordError(policy.Namespace, policy.Name, "deletion_failed")
		}

		// Log errors
		for _, err := range batchErrors {
			if s.eventRecorder != nil {
				s.eventRecorder.RecordEvaluationFailed(policy, err)
			}
			s.logger.Error(err, "Error deleting batch for policy", sdklog.Operation("delete_batch"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)), sdklog.ErrorCode("DELETE_BATCH_FAILED"))
		}
	}
	return deletedCount
}

// updatePolicyStatus updates the policy status.
func (s *PolicyEvaluationService) updatePolicyStatus(
	ctx context.Context,
	policy *v1alpha1.GarbageCollectionPolicy,
	matchedCount, deletedCount, pendingCount int64,
) error {
	if s.statusUpdater == nil {
		return nil
	}

	statusCtx, statusCancel := context.WithTimeout(ctx, 10*time.Second)
	defer statusCancel()

	if err := s.statusUpdater.UpdateStatus(statusCtx, policy, matchedCount, deletedCount, pendingCount); err != nil {
		if statusCtx.Err() != nil {
			s.logger.Debug("Status update canceled or timed out", sdklog.Operation("update_status"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)), sdklog.Error(statusCtx.Err()))
			return nil
		}
		gcErr := gcerrors.Wrap(err, "status_update_failed", "failed to update policy status")
		gcErr = gcErr.WithContext("policy_namespace", policy.Namespace)
		gcErr = gcErr.WithContext("policy_name", policy.Name)
		recordError(policy.Namespace, policy.Name, "status_update_failed")
		if s.eventRecorder != nil {
			s.eventRecorder.RecordStatusUpdateFailed(policy, gcErr)
		}
		s.logger.Error(gcErr, "Error updating policy status", sdklog.Operation("update_status"), sdklog.String("policy", fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)), sdklog.ErrorCode("UPDATE_STATUS_FAILED"))
		return gcErr
	}
	return nil
}

// shouldDelete determines if a resource should be deleted based on TTL.
func (s *PolicyEvaluationService) shouldDelete(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) (shouldDelete bool, reason string) {
	// Calculate expiration time using shared function
	expirationTime, err := calculateExpirationTimeShared(resource, &policy.Spec.TTL)
	if err != nil {
		s.logger.Debug("Could not calculate expiration time for resource", sdklog.Operation("should_delete"), sdklog.String("resource", fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName())), sdklog.Error(err))
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

// getBatchSize returns the batch size for deletions.
func (s *PolicyEvaluationService) getBatchSize(policy *v1alpha1.GarbageCollectionPolicy) int {
	if policy.Spec.Behavior.BatchSize > 0 {
		return policy.Spec.Behavior.BatchSize
	}
	return 10 // Default batch size
}

// parseGVR parses a GVR from API version and kind.
func parseGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid API version: %w", err)
	}

	resource := validation.PluralizeKind(kind)
	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}, nil
}
