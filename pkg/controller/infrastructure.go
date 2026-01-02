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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/config"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
)

// DefaultResourceInformer wraps a cache.SharedInformer to implement ResourceInformer interface.
type DefaultResourceInformer struct {
	informer cache.SharedInformer
}

// NewDefaultResourceInformer creates a new DefaultResourceInformer.
func NewDefaultResourceInformer(informer cache.SharedInformer) ResourceInformer {
	return &DefaultResourceInformer{informer: informer}
}

// GetStore returns the informer's store.
func (d *DefaultResourceInformer) GetStore() cache.Store {
	return d.informer.GetStore()
}

// HasSynced returns whether the informer has synced.
func (d *DefaultResourceInformer) HasSynced() bool {
	return d.informer.HasSynced()
}

// AddEventHandler adds an event handler to the informer.
func (d *DefaultResourceInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return d.informer.AddEventHandler(handler)
}

// DefaultResourceInformerFactory wraps a DynamicSharedInformerFactory to implement ResourceInformerFactory interface.
type DefaultResourceInformerFactory struct {
	factory dynamicinformer.DynamicSharedInformerFactory
}

// NewDefaultResourceInformerFactory creates a new DefaultResourceInformerFactory.
func NewDefaultResourceInformerFactory(factory dynamicinformer.DynamicSharedInformerFactory) ResourceInformerFactory {
	return &DefaultResourceInformerFactory{factory: factory}
}

// ForResource returns an informer for the given GVR.
func (f *DefaultResourceInformerFactory) ForResource(gvr schema.GroupVersionResource) ResourceInformer {
	return NewDefaultResourceInformer(f.factory.ForResource(gvr).Informer())
}

// Start starts all informers created by this factory.
func (f *DefaultResourceInformerFactory) Start(stopCh <-chan struct{}) {
	f.factory.Start(stopCh)
}

// DefaultResourceLister implements ResourceLister using a dynamic client.
type DefaultResourceLister struct {
	client dynamic.Interface
}

// NewDefaultResourceLister creates a new DefaultResourceLister.
func NewDefaultResourceLister(client dynamic.Interface) ResourceLister {
	return &DefaultResourceLister{client: client}
}

// ListResources lists all resources of the given GVR in the namespace.
func (l *DefaultResourceLister) ListResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	var resourceInterface dynamic.ResourceInterface
	if namespace == "" {
		resourceInterface = l.client.Resource(gvr)
	} else {
		resourceInterface = l.client.Resource(gvr).Namespace(namespace)
	}

	list, err := resourceInterface.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		resources = append(resources, &list.Items[i])
	}

	return resources, nil
}

// DefaultSelectorMatcher implements SelectorMatcher using shared logic.
type DefaultSelectorMatcher struct{}

// NewDefaultSelectorMatcher creates a new DefaultSelectorMatcher.
func NewDefaultSelectorMatcher() SelectorMatcher {
	return &DefaultSelectorMatcher{}
}

// MatchesSelectors checks if a resource matches the given selectors.
func (m *DefaultSelectorMatcher) MatchesSelectors(resource *unstructured.Unstructured, spec *v1alpha1.TargetResourceSpec) bool {
	return matchesSelectorsShared(resource, spec)
}

// DefaultConditionMatcher implements ConditionMatcher using shared logic.
type DefaultConditionMatcher struct{}

// NewDefaultConditionMatcher creates a new DefaultConditionMatcher.
func NewDefaultConditionMatcher() ConditionMatcher {
	return &DefaultConditionMatcher{}
}

// MeetsConditions checks if a resource meets the given conditions.
func (m *DefaultConditionMatcher) MeetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	return meetsConditionsShared(resource, conditions)
}

// DefaultRateLimiterProvider implements RateLimiterProvider.
type DefaultRateLimiterProvider struct {
	rateLimiters map[types.UID]*ratelimiter.RateLimiter
	config       *config.ControllerConfig
	mu           sync.RWMutex
}

// NewDefaultRateLimiterProvider creates a new DefaultRateLimiterProvider.
func NewDefaultRateLimiterProvider(cfg *config.ControllerConfig) RateLimiterProvider {
	return &DefaultRateLimiterProvider{
		rateLimiters: make(map[types.UID]*ratelimiter.RateLimiter),
		config:       cfg,
	}
}

// GetOrCreateRateLimiter returns a rate limiter for the given policy.
func (p *DefaultRateLimiterProvider) GetOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter {
	p.mu.Lock()
	defer p.mu.Unlock()

	if limiter, exists := p.rateLimiters[policy.UID]; exists {
		return limiter
	}

	maxDeletionsPerSecond := 10 // DefaultMaxDeletionsPerSecond
	if p.config != nil && p.config.MaxDeletionsPerSecond > 0 {
		maxDeletionsPerSecond = p.config.MaxDeletionsPerSecond
	}
	if policy.Spec.Behavior.MaxDeletionsPerSecond > 0 {
		maxDeletionsPerSecond = policy.Spec.Behavior.MaxDeletionsPerSecond
	}

	limiter := ratelimiter.NewRateLimiter(maxDeletionsPerSecond)
	p.rateLimiters[policy.UID] = limiter
	return limiter
}

