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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
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

// GCControllerAdapter adapts GCController to provide interfaces for PolicyEvaluationService.
// This allows GCController to use PolicyEvaluationService internally while maintaining backward compatibility.
type GCControllerAdapter struct {
	gc *GCController
}

// NewGCControllerAdapter creates a new GCControllerAdapter.
func NewGCControllerAdapter(gc *GCController) *GCControllerAdapter {
	return &GCControllerAdapter{gc: gc}
}

// GetResourceListerForPolicy creates a ResourceLister from the policy's informer.
func (a *GCControllerAdapter) GetResourceListerForPolicy(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) (ResourceLister, error) {
	// Use context from GCController if ctx is not provided
	if ctx == nil {
		ctx = a.gc.ctx
	}
	informer, err := a.gc.getOrCreateResourceInformer(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource informer: %w", err)
	}
	return NewInformerStoreResourceLister(informer.GetStore()), nil
}

// GetSelectorMatcher returns a SelectorMatcher using GCController's implementation.
func (a *GCControllerAdapter) GetSelectorMatcher() SelectorMatcher {
	return &GCControllerSelectorMatcher{gc: a.gc}
}

// GetConditionMatcher returns a ConditionMatcher using GCController's implementation.
func (a *GCControllerAdapter) GetConditionMatcher() ConditionMatcher {
	return &GCControllerConditionMatcher{gc: a.gc}
}

// GetRateLimiterProvider returns a RateLimiterProvider using GCController's implementation.
func (a *GCControllerAdapter) GetRateLimiterProvider() RateLimiterProvider {
	return &GCControllerRateLimiterProvider{gc: a.gc}
}

// GetBatchDeleter returns a BatchDeleterCore using GCController's implementation.
func (a *GCControllerAdapter) GetBatchDeleter() BatchDeleterCore {
	return &GCControllerBatchDeleter{gc: a.gc}
}

// GCControllerSelectorMatcher adapts GCController to SelectorMatcher interface.
type GCControllerSelectorMatcher struct {
	gc *GCController
}

// MatchesSelectors checks if a resource matches selectors.
func (m *GCControllerSelectorMatcher) MatchesSelectors(resource *unstructured.Unstructured, spec *v1alpha1.TargetResourceSpec) bool {
	return m.gc.matchesSelectors(resource, spec)
}

// GCControllerConditionMatcher adapts GCController to ConditionMatcher interface.
type GCControllerConditionMatcher struct {
	gc *GCController
}

// MeetsConditions checks if a resource meets conditions.
func (m *GCControllerConditionMatcher) MeetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	return m.gc.meetsConditions(resource, conditions)
}

// GCControllerRateLimiterProvider adapts GCController to RateLimiterProvider interface.
type GCControllerRateLimiterProvider struct {
	gc *GCController
}

// GetOrCreateRateLimiter returns a rate limiter for the policy.
func (p *GCControllerRateLimiterProvider) GetOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter {
	return p.gc.getOrCreateRateLimiter(policy)
}

// GCControllerBatchDeleter adapts GCController to BatchDeleterCore interface.
type GCControllerBatchDeleter struct {
	gc *GCController
}

// DeleteBatch deletes a batch of resources.
func (d *GCControllerBatchDeleter) DeleteBatch(ctx context.Context, batch []*unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter, reasons map[string]string) (int64, []error) {
	return d.gc.deleteBatch(ctx, batch, policy, rateLimiter, reasons)
}

