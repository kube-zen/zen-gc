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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// PolicyEvaluator provides methods needed for policy evaluation.
type PolicyEvaluator interface {
	getOrCreateResourceInformer(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) (cache.SharedInformer, error)
	matchesSelectors(resource *unstructured.Unstructured, target *v1alpha1.TargetResourceSpec) bool
	shouldDelete(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy) (bool, string)
	getOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter
	getBatchSize(policy *v1alpha1.GarbageCollectionPolicy) int
	deleteBatch(ctx context.Context, batch []*unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter, reasons map[string]string) (int64, []error)
	getStatusUpdater() *StatusUpdater
	GetEventRecorder() *EventRecorder
}

// evaluatePolicyResourcesShared evaluates resources for a policy and collects those to delete.
func evaluatePolicyResourcesShared(
	ctx context.Context,
	evaluator PolicyEvaluator,
	policy *v1alpha1.GarbageCollectionPolicy,
	informer cache.SharedInformer,
) *PolicyEvaluationResult {
	// Get all resources from cache
	resources := informer.GetStore().List()

	result := &PolicyEvaluationResult{
		MatchedCount:             int64(0),
		DeletedCount:             int64(0),
		PendingCount:             int64(0),
		ResourcesToDelete:        make([]*unstructured.Unstructured, 0),
		ResourcesToDeleteReasons: make(map[string]string),
	}

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	logger := sdklog.NewLogger("zen-gc")
	for _, obj := range resources {
		// Check context cancellation during resource iteration
		select {
		case <-ctx.Done():
			logger.Debug("Stopping policy evaluation: context canceled", sdklog.Operation("evaluate_policy"), sdklog.String("policy", policy.Namespace+"/"+policy.Name))
			return result
		default:
		}

		resource, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Check if resource matches selectors
		if !evaluator.matchesSelectors(resource, &policy.Spec.TargetResource) {
			continue
		}

		result.MatchedCount++
		recordResourceMatched(policy.Namespace, policy.Name, resourceAPIVersion, resourceKind)

		// Check if resource should be deleted
		shouldDelete, reason := evaluator.shouldDelete(resource, policy)
		if !shouldDelete {
			result.PendingCount++
			continue
		}

		// Add to deletion list
		result.ResourcesToDelete = append(result.ResourcesToDelete, resource)
		result.ResourcesToDeleteReasons[string(resource.GetUID())] = reason
	}

	return result
}

// deleteResourcesInBatchesShared deletes resources in batches.
func deleteResourcesInBatchesShared(
	ctx context.Context,
	evaluator PolicyEvaluator,
	policy *v1alpha1.GarbageCollectionPolicy,
	resourcesToDelete []*unstructured.Unstructured,
	resourcesToDeleteReasons map[string]string,
) int64 {
	if len(resourcesToDelete) == 0 {
		return 0
	}

	rateLimiter := evaluator.getOrCreateRateLimiter(policy)
	batchSize := evaluator.getBatchSize(policy)
	deletedCount := int64(0)

	logger := sdklog.NewLogger("zen-gc")
	// Process deletions in batches
	for i := 0; i < len(resourcesToDelete); i += batchSize {
		// Check context cancellation between batches
		select {
		case <-ctx.Done():
			logger.Debug("Stopping batch deletion: context canceled", sdklog.Operation("delete_batch"), sdklog.String("policy", policy.Namespace+"/"+policy.Name))
			return deletedCount
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
		batchDeleted, batchErrors := evaluator.deleteBatch(ctx, batch, policy, rateLimiter, resourcesToDeleteReasons)
		deletedCount += batchDeleted

		// Track deletion failures
		if len(batchErrors) > 0 {
			recordError(policy.Namespace, policy.Name, "deletion_failed")
		}

		// Log errors
		eventRecorder := evaluator.GetEventRecorder()
		for _, err := range batchErrors {
			if eventRecorder != nil {
				eventRecorder.RecordEvaluationFailed(policy, err)
			}
			logger.Error(err, "Error deleting batch for policy", sdklog.Operation("delete_batch"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.ErrorCode("DELETE_BATCH_FAILED"))
		}

		// Log deletion attempt metrics
		logger.Debug("Policy deletion batch completed", sdklog.Operation("delete_batch"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.Int64("attempted", deletionAttempts), sdklog.Int64("succeeded", batchDeleted), sdklog.Int64("failed", int64(len(batchErrors))))
	}

	return deletedCount
}

// updatePolicyStatusShared updates the policy status.
func updatePolicyStatusShared(
	ctx context.Context,
	evaluator PolicyEvaluator,
	policy *v1alpha1.GarbageCollectionPolicy,
	matchedCount, deletedCount, pendingCount int64,
) error {
	statusUpdater := evaluator.getStatusUpdater()
	if statusUpdater == nil {
		return nil
	}

	// Use timeout context for status updates to prevent hanging
	statusCtx, statusCancel := context.WithTimeout(ctx, 10*time.Second)
	defer statusCancel()

	logger := sdklog.NewLogger("zen-gc")
	if err := statusUpdater.UpdateStatus(statusCtx, policy, matchedCount, deletedCount, pendingCount); err != nil {
		// Check if error is due to context cancellation/timeout
		if statusCtx.Err() != nil {
			logger.Debug("Status update canceled or timed out", sdklog.Operation("update_status"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.Error(statusCtx.Err()))
			return nil // Don't treat cancellation as error
		}
		gcErr := gcerrors.Wrap(err, "status_update_failed", "failed to update policy status")
		gcErr.PolicyNamespace = policy.Namespace
		gcErr.PolicyName = policy.Name
		recordError(policy.Namespace, policy.Name, "status_update_failed")
		eventRecorder := evaluator.GetEventRecorder()
		if eventRecorder != nil {
			eventRecorder.RecordStatusUpdateFailed(policy, gcErr)
		}
		logger.Error(gcErr, "Error updating policy status", sdklog.Operation("update_status"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.ErrorCode("UPDATE_STATUS_FAILED"))
		return gcErr
	}

	return nil
}
