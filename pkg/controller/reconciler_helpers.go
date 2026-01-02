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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	gcerrors "github.com/kube-zen/zen-gc/pkg/errors"
	"github.com/kube-zen/zen-gc/pkg/validation"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// handlePolicyDeletion handles cleanup when a policy is deleted.
func (r *GCPolicyReconciler) handlePolicyDeletion(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Debug("Policy not found, cleaning up resources", sdklog.Operation("reconcile"))
	r.cleanupPolicyResources(req.NamespacedName)
	return ctrl.Result{}, nil
}

// handlePolicyFetchError handles errors when fetching a policy.
func (r *GCPolicyReconciler) handlePolicyFetchError(err error) (ctrl.Result, error) {
	r.logger.Error(err, "Failed to fetch GarbageCollectionPolicy", sdklog.Operation("fetch_policy"), sdklog.ErrorCode("FETCH_POLICY_FAILED"))
	return ctrl.Result{}, err
}

// handleInformerRecreation handles informer recreation when policy spec changes.
func (r *GCPolicyReconciler) handleInformerRecreation(policy *v1alpha1.GarbageCollectionPolicy) {
	if !r.shouldRecreateInformer(policy) {
		return
	}

	r.logger.Debug("Policy spec changed, recreating informer", sdklog.Operation("update_informer"))
	r.cleanupResourceInformer(policy.UID)
	// Clear old spec to allow new one to be tracked
	r.policySpecsMu.Lock()
	delete(r.policySpecs, policy.UID)
	r.policySpecsMu.Unlock()
}

// handlePausedPolicy handles paused policies.
func (r *GCPolicyReconciler) handlePausedPolicy() (ctrl.Result, error) {
	r.logger.Debug("Policy is paused, skipping evaluation", sdklog.Operation("reconcile"))
	return ctrl.Result{RequeueAfter: r.getRequeueInterval()}, nil
}

// handleEvaluationError handles errors during policy evaluation.
func (r *GCPolicyReconciler) handleEvaluationError(err error, policy *v1alpha1.GarbageCollectionPolicy) (ctrl.Result, error) {
	gcErr := gcerrors.WithPolicy(err, policy.Namespace, policy.Name)
	if gcErr.Type == "" {
		gcErr.Type = ErrorTypeEvaluationFailed
	}
	r.logger.Error(gcErr, "Error evaluating policy", sdklog.Operation("evaluate_policy"), sdklog.ErrorCode("EVALUATE_POLICY_FAILED"))
	// Requeue with backoff on error
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// resolveGVRForDeletion resolves the GVR for a resource deletion.
func (r *GCPolicyReconciler) resolveGVRForDeletion(resource *unstructured.Unstructured) schema.GroupVersionResource {
	if r.gvrResolver != nil {
		resolvedGVR, gvrErr := r.gvrResolver.ResolveGVR(resource)
		if gvrErr == nil {
			return resolvedGVR
		}
		// Fall back to pluralization if GVRResolver fails
		r.logger.Debug("GVRResolver failed, falling back to pluralization", sdklog.Operation("delete_resource"), sdklog.String("resource", fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName())), sdklog.Error(gvrErr))
	}

	// Use pluralization fallback
	return schema.GroupVersionResource{
		Group:    resource.GroupVersionKind().Group,
		Version:  resource.GroupVersionKind().Version,
		Resource: validation.PluralizeKind(resource.GetKind()),
	}
}

// buildDeleteOptions builds delete options from policy behavior.
func buildDeleteOptions(policy *v1alpha1.GarbageCollectionPolicy) *metav1.DeleteOptions {
	deleteOptions := &metav1.DeleteOptions{}
	if policy.Spec.Behavior.GracePeriodSeconds != nil {
		deleteOptions.GracePeriodSeconds = policy.Spec.Behavior.GracePeriodSeconds
	}

	propagationPolicy := getDeletionPropagationPolicy(policy.Spec.Behavior.PropagationPolicy)
	deleteOptions.PropagationPolicy = &propagationPolicy

	return deleteOptions
}

// performResourceDeletion performs the actual resource deletion.
func (r *GCPolicyReconciler) performResourceDeletion(ctx context.Context, resource *unstructured.Unstructured, gvr schema.GroupVersionResource, deleteOptions *metav1.DeleteOptions) error {
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

// normalizeNamespace normalizes namespace for informer creation.
func normalizeNamespace(namespace string) string {
	// Normalize: empty defaults to "*" (cluster-wide) to match webhook behavior
	if namespace == "" {
		namespace = "*"
	}
	// Translate "*" to NamespaceAll (empty string) for cluster-wide watching
	if namespace == "*" {
		namespace = metav1.NamespaceAll
	}
	return namespace
}

// buildLabelSelectorFilter builds a label selector filter function for informer factory.
func buildLabelSelectorFilter(policy *v1alpha1.GarbageCollectionPolicy) func(options *metav1.ListOptions) {
	return func(options *metav1.ListOptions) {
		if policy.Spec.TargetResource.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.Spec.TargetResource.LabelSelector)
			if err == nil {
				options.LabelSelector = selector.String()
			}
		}
	}
}
