// Package controller implements the garbage collection controller.
package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
)

// deleteResourceWithBackoff deletes a resource with exponential backoff retry.
// Backoff logic is now handled in shared.go using zen-sdk/pkg/gc/backoff.
func (gc *GCController) deleteResourceWithBackoff(
	ctx context.Context,
	resource *unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	rateLimiter *ratelimiter.RateLimiter,
) error {
	return deleteResourceWithBackoffShared(ctx, resource, policy, rateLimiter, nil, gc)
}

// DeleteResourceWithoutContext deletes a resource without context (implements ResourceDeleterWithoutContext).
func (gc *GCController) DeleteResourceWithoutContext(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter) error {
	return gc.deleteResource(resource, policy, rateLimiter)
}
