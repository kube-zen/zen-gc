// Package controller implements the garbage collection controller.
package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

// DefaultBackoff is the default exponential backoff configuration.
var DefaultBackoff = wait.Backoff{
	Steps:    5,
	Duration: 100 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
	Cap:      30 * time.Second,
}

// deleteResourceWithBackoff deletes a resource with exponential backoff retry.
func (gc *GCController) deleteResourceWithBackoff(
	ctx context.Context,
	resource *unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
	rateLimiter *RateLimiter,
) error {
	return deleteResourceWithBackoffShared(ctx, resource, policy, rateLimiter, nil, gc)
}

// DeleteResourceWithoutContext deletes a resource without context (implements ResourceDeleterWithoutContext).
func (gc *GCController) DeleteResourceWithoutContext(resource *unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *RateLimiter) error {
	return gc.deleteResource(resource, policy, rateLimiter)
}
