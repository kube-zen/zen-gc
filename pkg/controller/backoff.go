package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

var (
	// DefaultBackoff is the default exponential backoff configuration.
	DefaultBackoff = wait.Backoff{
		Steps:    5,
		Duration: 100 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Cap:      30 * time.Second,
	}
)

// deleteResourceWithBackoff deletes a resource with exponential backoff retry
func (gc *GCController) deleteResourceWithBackoff(
	ctx context.Context,
	resource *unstructured.Unstructured,
	policy *v1alpha1.GarbageCollectionPolicy,
) error {
	var lastErr error

	err := wait.ExponentialBackoff(DefaultBackoff, func() (bool, error) {
		err := gc.deleteResource(resource, policy)
		if err != nil {
			// Check if error is retryable
			if k8serrors.IsTimeout(err) || k8serrors.IsServerTimeout(err) ||
				k8serrors.IsTooManyRequests(err) || k8serrors.IsServiceUnavailable(err) {
				lastErr = err
				return false, nil // retry
			}
			// For NotFound errors, consider it success (already deleted)
			if k8serrors.IsNotFound(err) {
				return true, nil // success
			}
			return false, err // don't retry
		}
		return true, nil // success
	})

	if errors.Is(err, wait.ErrWaitTimeout) {
		return fmt.Errorf("deletion failed after retries: %w", lastErr)
	}

	return err
}
