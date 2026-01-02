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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
	"github.com/kube-zen/zen-sdk/pkg/gc/backoff"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
	sdkttl "github.com/kube-zen/zen-sdk/pkg/gc/ttl"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// Constants for deletion reasons and error types.
const (
	// ReasonTTLExpired indicates that a resource's TTL has expired.
	ReasonTTLExpired = "ttl_expired"

	// ReasonNotExpired indicates that a resource's TTL has not expired.
	ReasonNotExpired = "not_expired"

	// ReasonConditionNotMet indicates that a resource does not meet the deletion conditions.
	ReasonConditionNotMet = "condition_not_met"

	// ErrorTypeEvaluationFailed indicates that policy evaluation failed.
	ErrorTypeEvaluationFailed = "evaluation_failed"
)

// Constants for deletion propagation policies.
const (
	// PropagationPolicyForeground indicates foreground deletion propagation.
	PropagationPolicyForeground = "Foreground"

	// PropagationPolicyBackground indicates background deletion propagation.
	PropagationPolicyBackground = "Background"

	// PropagationPolicyOrphan indicates orphan deletion propagation.
	PropagationPolicyOrphan = "Orphan"
)

// Constants for field condition operators.
const (
	// OperatorNotIn indicates a "NotIn" operator for field conditions.
	OperatorNotIn = "NotIn"
)

// Constants for policy phases.
const (
	// PolicyPhaseActive indicates the policy is active and processing resources.
	PolicyPhaseActive = "Active"

	// PolicyPhasePaused indicates the policy is paused.
	PolicyPhasePaused = "Paused"

	// PolicyPhaseError indicates the policy encountered errors during evaluation.
	PolicyPhaseError = "Error"
)

// RateLimiterManager manages rate limiters for policies.
// This interface allows both GCController and GCPolicyReconciler to use shared rate limiter logic.
type RateLimiterManager interface {
	getRateLimiters() map[types.UID]*ratelimiter.RateLimiter
	getRateLimitersMu() *sync.RWMutex
	getConfig() *config.ControllerConfig
}

// getOrCreateRateLimiterShared is a shared implementation for getting or creating a rate limiter.
func getOrCreateRateLimiterShared(mgr RateLimiterManager, policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter {
	// Determine rate limit for this policy
	maxDeletionsPerSecond := DefaultMaxDeletionsPerSecond
	if policy.Spec.Behavior.MaxDeletionsPerSecond > 0 {
		maxDeletionsPerSecond = policy.Spec.Behavior.MaxDeletionsPerSecond
	}

	rateLimiters := mgr.getRateLimiters()
	rateLimitersMu := mgr.getRateLimitersMu()

	// Check if rate limiter already exists (with read lock)
	rateLimitersMu.RLock()
	if limiter, ok := rateLimiters[policy.UID]; ok {
		// Update rate if it changed
		if limiter != nil {
			// Update rate to match policy configuration
			limiter.SetRate(maxDeletionsPerSecond)
		}
		rateLimitersMu.RUnlock()
		return limiter
	}
	rateLimitersMu.RUnlock()

	// Acquire write lock for creating new rate limiter
	rateLimitersMu.Lock()
	defer rateLimitersMu.Unlock()

	// Double-check after acquiring write lock
	if limiter, ok := rateLimiters[policy.UID]; ok {
		limiter.SetRate(maxDeletionsPerSecond)
		return limiter
	}

	// Create new rate limiter using zen-sdk
	limiter := ratelimiter.NewRateLimiter(maxDeletionsPerSecond)
	rateLimiters[policy.UID] = limiter

	// Update metrics
	recordRateLimiterCount(len(rateLimiters))

	logger := sdklog.NewLogger("zen-gc")
	logger.Debug("Created rate limiter for policy", sdklog.Operation("get_or_create_rate_limiter"), sdklog.String("policy", policy.Namespace+"/"+policy.Name), sdklog.String("uid", string(policy.UID)), sdklog.Int("rate_per_sec", maxDeletionsPerSecond))
	return limiter
}

// BatchDeleter provides methods needed for batch deletion.
type BatchDeleter interface {
	DeleteResourceWithBackoff(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter) error
	GetEventRecorder() *EventRecorder
}

// deleteBatchShared is a shared implementation for deleting a batch of resources.
func deleteBatchShared(
	ctx context.Context,
	batch []*unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	rateLimiter *ratelimiter.RateLimiter,
	reasons map[string]string,
	deleter BatchDeleter,
) (int64, []error) {
	deletedCount := int64(0)
	// Pre-allocate errors slice with batch size (worst case: all deletions fail)
	errors := make([]error, 0, len(batch))

	resourceAPIVersion := policy.Spec.TargetResource.APIVersion
	resourceKind := policy.Spec.TargetResource.Kind

	const contextCheckInterval = 50 // Check context every 50 iterations
	for i, resource := range batch {
		// Check context cancellation periodically to reduce overhead
		if i%contextCheckInterval == 0 {
			select {
			case <-ctx.Done():
				return deletedCount, errors
			default:
			}
		}

		// Rate limiting (per resource)
		if err := rateLimiter.Wait(ctx); err != nil {
			errors = append(errors, fmt.Errorf("rate limiter error: %w", err))
			continue
		}

		// Delete the resource with exponential backoff
		deleteStart := time.Now()
		if err := deleter.DeleteResourceWithBackoff(ctx, resource, policy, rateLimiter); err != nil {
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
		if eventRecorder := deleter.GetEventRecorder(); eventRecorder != nil {
			eventRecorder.RecordResourceDeleted(policy, resource, reason)
		}
		// Logger creation here is acceptable as deletion logging is infrequent
		// Future optimization: pass logger as parameter to avoid allocations
		logger := sdklog.NewLogger("zen-gc")
		logger.Info("Deleted resource", sdklog.Operation("delete_batch"), sdklog.String("resource", fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName())), sdklog.String("reason", reason))
	}

	return deletedCount, errors
}

// TTLCalculator provides methods needed for TTL calculation.
type TTLCalculator interface{}

// calculateExpirationTimeShared is a shared implementation for calculating expiration time.
// This now delegates to zen-sdk/pkg/gc/ttl for the actual evaluation.
func calculateExpirationTimeShared(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (time.Time, error) {
	// Convert v1alpha1.TTLSpec to zen-sdk ttl.Spec
	sdkSpec := convertToSDKTTLSpec(ttlSpec)
	return sdkttl.CalculateExpirationTime(resource, sdkSpec)
}

// convertToSDKTTLSpec converts zen-gc's TTLSpec to zen-sdk's ttl.Spec.
func convertToSDKTTLSpec(gcSpec *v1alpha1.TTLSpec) *sdkttl.Spec {
	return &sdkttl.Spec{
		SecondsAfterCreation: gcSpec.SecondsAfterCreation,
		FieldPath:            gcSpec.FieldPath,
		Mappings:             gcSpec.Mappings,
		Default:              gcSpec.Default,
		RelativeTo:           gcSpec.RelativeTo,
		SecondsAfter:         gcSpec.SecondsAfter,
	}
}

// getDeletionPropagationPolicy converts a string policy to metav1.DeletionPropagation.
func getDeletionPropagationPolicy(policyStr string) metav1.DeletionPropagation {
	switch policyStr {
	case PropagationPolicyForeground:
		return PropagationPolicyForeground
	case PropagationPolicyOrphan:
		return PropagationPolicyOrphan
	default:
		return PropagationPolicyBackground
	}
}

// ResourceDeleterWithContext provides the deleteResource method needed for backoff retry (with context).
type ResourceDeleterWithContext interface {
	DeleteResourceWithContext(ctx context.Context, resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter) error
}

// ResourceDeleterWithoutContext provides the deleteResource method needed for backoff retry (without context).
type ResourceDeleterWithoutContext interface {
	DeleteResourceWithoutContext(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter) error
}

// deleteResourceWithBackoffShared deletes a resource with exponential backoff retry.
func deleteResourceWithBackoffShared(
	ctx context.Context,
	resource *unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	rateLimiter *ratelimiter.RateLimiter,
	deleterWithCtx ResourceDeleterWithContext,
	deleterWithoutCtx ResourceDeleterWithoutContext,
) error {
	var lastErr error

	// Use zen-sdk backoff
	backoffConfig := backoff.DefaultConfig()
	b := backoff.NewBackoff(backoffConfig)

	for !b.IsExhausted() {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var err error
		if deleterWithCtx != nil {
			err = deleterWithCtx.DeleteResourceWithContext(ctx, resource, policy, rateLimiter)
		} else if deleterWithoutCtx != nil {
			err = deleterWithoutCtx.DeleteResourceWithoutContext(resource, policy, rateLimiter)
		} else {
			return fmt.Errorf("%w", ErrNoDeleter)
		}

		if err == nil {
			return nil // success
		}

		// Check if error is retryable
		if k8serrors.IsTimeout(err) || k8serrors.IsServerTimeout(err) ||
			k8serrors.IsTooManyRequests(err) || k8serrors.IsServiceUnavailable(err) {
			lastErr = err
			// Wait for backoff duration before retry
			duration := b.Next()
			if duration == 0 {
				break // exhausted
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(duration):
				// Continue to retry
			}
			continue
		}

		// For NotFound errors, consider it success (already deleted)
		if k8serrors.IsNotFound(err) {
			return nil // success
		}

		// Non-retryable error
		return err
	}

	// Backoff exhausted
	return fmt.Errorf("deletion failed after retries: %w", lastErr)
}

// meetsConditionsShared checks if a resource meets the deletion conditions.
func meetsConditionsShared(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	if !meetsPhaseConditionsShared(resource, conditions.Phase) {
		return false
	}
	if !meetsLabelConditionsShared(resource, conditions.HasLabels) {
		return false
	}
	if !meetsAnnotationConditionsShared(resource, conditions.HasAnnotations) {
		return false
	}
	if !meetsFieldConditionsShared(resource, conditions.And) {
		return false
	}
	return true
}

// meetsPhaseConditionsShared checks if resource phase matches any of the required phases.
func meetsPhaseConditionsShared(resource *unstructured.Unstructured, phases []string) bool {
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

// meetsLabelConditionsShared checks if resource labels match the required conditions.
func meetsLabelConditionsShared(resource *unstructured.Unstructured, labelConds []v1alpha1.LabelCondition) bool {
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
		case OperatorNotIn:
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
			logger := sdklog.NewLogger("zen-gc")
			logger.Warn("Unknown label condition operator, rejecting match", sdklog.Operation("meets_label_conditions"), sdklog.String("operator", labelCond.Operator))
			return false
		}
	}
	return true
}

// meetsAnnotationConditionsShared checks if resource annotations match the required conditions.
func meetsAnnotationConditionsShared(resource *unstructured.Unstructured, annConds []v1alpha1.AnnotationCondition) bool {
	resourceAnnotations := resource.GetAnnotations()
	for _, annCond := range annConds {
		value, exists := resourceAnnotations[annCond.Key]
		if !exists || value != annCond.Value {
			return false
		}
	}
	return true
}

// meetsFieldConditionsShared checks if resource fields match the required conditions.
func meetsFieldConditionsShared(resource *unstructured.Unstructured, fieldConds []v1alpha1.FieldCondition) bool {
	for _, fieldCond := range fieldConds {
		fieldPath := parseFieldPath(fieldCond.FieldPath)
		fieldValue, found, _ := unstructured.NestedString(resource.Object, fieldPath...)
		if !found {
			return false
		}
		if !matchesFieldOperatorShared(fieldValue, fieldCond) {
			return false
		}
	}
	return true
}

// matchesFieldOperatorShared checks if field value matches the operator condition.
func matchesFieldOperatorShared(fieldValue string, fieldCond v1alpha1.FieldCondition) bool {
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
	case OperatorNotIn:
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

// matchesSelectorsShared checks if a resource matches the target resource selectors.
func matchesSelectorsShared(resource *unstructured.Unstructured, target *v1alpha1.TargetResourceSpec) bool {
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
			logger := sdklog.NewLogger("zen-gc")
			logger.Error(gcErr, "Invalid label selector", sdklog.Operation("matches_selectors"), sdklog.ErrorCode("INVALID_LABEL_SELECTOR"))
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
