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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
)

// Static errors for adapters.
var (
	errInformerStoreNil = errors.New("informer store is nil")
)

// InformerStoreResourceLister adapts a cache.Store to ResourceLister interface.
// This allows us to use existing informer stores with the new ResourceLister interface.
type InformerStoreResourceLister struct {
	store cache.Store
}

// NewInformerStoreResourceLister creates a new InformerStoreResourceLister.
func NewInformerStoreResourceLister(store cache.Store) ResourceLister {
	return &InformerStoreResourceLister{store: store}
}

// ListResources lists all resources from the store.
func (l *InformerStoreResourceLister) ListResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	items := l.store.List()
	resources := make([]*unstructured.Unstructured, 0, len(items))

	for _, obj := range items {
		resource, ok := obj.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		// Filter by namespace if specified
		if namespace != "" && namespace != "*" && resource.GetNamespace() != namespace {
			continue
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// GCPolicyReconcilerAdapter adapts GCPolicyReconciler to provide interfaces for PolicyEvaluationService.
// This allows GCPolicyReconciler to use PolicyEvaluationService internally while maintaining backward compatibility.
type GCPolicyReconcilerAdapter struct {
	reconciler *GCPolicyReconciler
}

// NewGCPolicyReconcilerAdapter creates a new GCPolicyReconcilerAdapter.
func NewGCPolicyReconcilerAdapter(reconciler *GCPolicyReconciler) *GCPolicyReconcilerAdapter {
	return &GCPolicyReconcilerAdapter{reconciler: reconciler}
}

// GetResourceListerForPolicy creates a ResourceLister from the policy's informer.
func (a *GCPolicyReconcilerAdapter) GetResourceListerForPolicy(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) (ResourceLister, error) {
	informer, err := a.reconciler.getOrCreateResourceInformer(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource informer: %w", err)
	}
	store := informer.GetStore()
	if store == nil {
		return nil, fmt.Errorf("%w for policy %s/%s", errInformerStoreNil, policy.Namespace, policy.Name)
	}
	return NewInformerStoreResourceLister(store), nil
}

// GetSelectorMatcher returns a SelectorMatcher using GCPolicyReconciler's implementation.
func (a *GCPolicyReconcilerAdapter) GetSelectorMatcher() SelectorMatcher {
	return &GCPolicyReconcilerSelectorMatcher{reconciler: a.reconciler}
}

// GetConditionMatcher returns a ConditionMatcher using GCPolicyReconciler's implementation.
func (a *GCPolicyReconcilerAdapter) GetConditionMatcher() ConditionMatcher {
	return &GCPolicyReconcilerConditionMatcher{reconciler: a.reconciler}
}

// GetRateLimiterProvider returns a RateLimiterProvider using GCPolicyReconciler's implementation.
func (a *GCPolicyReconcilerAdapter) GetRateLimiterProvider() RateLimiterProvider {
	return &GCPolicyReconcilerRateLimiterProvider{reconciler: a.reconciler}
}

// GetBatchDeleter returns a BatchDeleterCore using GCPolicyReconciler's implementation.
func (a *GCPolicyReconcilerAdapter) GetBatchDeleter() BatchDeleterCore {
	return &GCPolicyReconcilerBatchDeleter{reconciler: a.reconciler}
}

// GCPolicyReconcilerSelectorMatcher adapts GCPolicyReconciler to SelectorMatcher interface.
type GCPolicyReconcilerSelectorMatcher struct {
	reconciler *GCPolicyReconciler
}

// MatchesSelectors checks if a resource matches selectors.
func (m *GCPolicyReconcilerSelectorMatcher) MatchesSelectors(resource *unstructured.Unstructured, spec *v1alpha1.TargetResourceSpec) bool {
	return m.reconciler.matchesSelectors(resource, spec)
}

// GCPolicyReconcilerConditionMatcher adapts GCPolicyReconciler to ConditionMatcher interface.
type GCPolicyReconcilerConditionMatcher struct {
	reconciler *GCPolicyReconciler
}

// MeetsConditions checks if a resource meets conditions.
func (m *GCPolicyReconcilerConditionMatcher) MeetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	return m.reconciler.meetsConditions(resource, conditions)
}

// GCPolicyReconcilerRateLimiterProvider adapts GCPolicyReconciler to RateLimiterProvider interface.
type GCPolicyReconcilerRateLimiterProvider struct {
	reconciler *GCPolicyReconciler
}

// GetOrCreateRateLimiter returns a rate limiter for the policy.
func (p *GCPolicyReconcilerRateLimiterProvider) GetOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter {
	return p.reconciler.getOrCreateRateLimiter(policy)
}

// GCPolicyReconcilerBatchDeleter adapts GCPolicyReconciler to BatchDeleterCore interface.
type GCPolicyReconcilerBatchDeleter struct {
	reconciler *GCPolicyReconciler
}

// DeleteBatch deletes a batch of resources.
func (d *GCPolicyReconcilerBatchDeleter) DeleteBatch(ctx context.Context, batch []*unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter, reasons map[string]string) (int64, []error) {
	return d.reconciler.deleteBatch(ctx, batch, policy, rateLimiter, reasons)
}
