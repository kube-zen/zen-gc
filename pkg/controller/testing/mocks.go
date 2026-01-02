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

package testing

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/controller"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
)

// MockResourceInformer is a mock implementation of ResourceInformer for testing.
type MockResourceInformer struct {
	store   cache.Store
	synced  bool
	handler cache.ResourceEventHandler
	mu      sync.RWMutex
}

// NewMockResourceInformer creates a new MockResourceInformer with the given resources.
func NewMockResourceInformer(resources []*unstructured.Unstructured) *MockResourceInformer {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	for _, r := range resources {
		_ = store.Add(r)
	}
	return &MockResourceInformer{
		store:  store,
		synced: true,
	}
}

// GetStore returns the mock store.
func (m *MockResourceInformer) GetStore() cache.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

// HasSynced returns whether the mock informer has synced.
func (m *MockResourceInformer) HasSynced() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.synced
}

// SetSynced sets the synced status.
func (m *MockResourceInformer) SetSynced(synced bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synced = synced
}

// AddEventHandler adds an event handler (no-op for mock).
func (m *MockResourceInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
	return nil, nil
}

// MockResourceInformerFactory is a mock implementation of ResourceInformerFactory for testing.
type MockResourceInformerFactory struct {
	informers map[schema.GroupVersionResource]*MockResourceInformer
	mu        sync.RWMutex
}

// NewMockResourceInformerFactory creates a new MockResourceInformerFactory.
func NewMockResourceInformerFactory() *MockResourceInformerFactory {
	return &MockResourceInformerFactory{
		informers: make(map[schema.GroupVersionResource]*MockResourceInformer),
	}
}

// SetInformer sets an informer for a given GVR.
func (f *MockResourceInformerFactory) SetInformer(gvr schema.GroupVersionResource, informer *MockResourceInformer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.informers[gvr] = informer
}

// ForResource returns the mock informer for the given GVR.
func (f *MockResourceInformerFactory) ForResource(gvr schema.GroupVersionResource) controller.ResourceInformer {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if informer, exists := f.informers[gvr]; exists {
		return informer
	}
	// Return empty informer if not set
	return NewMockResourceInformer(nil)
}

// Start is a no-op for mock.
func (f *MockResourceInformerFactory) Start(stopCh <-chan struct{}) {
	// No-op
}

// MockResourceLister is a mock implementation of ResourceLister for testing.
type MockResourceLister struct {
	resources map[string]map[string][]*unstructured.Unstructured // gvr -> namespace -> resources
	err       error                                              // error to return
	mu        sync.RWMutex
}

// NewMockResourceLister creates a new MockResourceLister.
func NewMockResourceLister() *MockResourceLister {
	return &MockResourceLister{
		resources: make(map[string]map[string][]*unstructured.Unstructured),
	}
}

// SetResources sets resources for a given GVR and namespace.
func (l *MockResourceLister) SetResources(gvr schema.GroupVersionResource, namespace string, resources []*unstructured.Unstructured) {
	l.mu.Lock()
	defer l.mu.Unlock()
	gvrKey := gvr.String()
	if l.resources[gvrKey] == nil {
		l.resources[gvrKey] = make(map[string][]*unstructured.Unstructured)
	}
	l.resources[gvrKey][namespace] = resources
}

// SetError sets an error to return from ListResources.
func (l *MockResourceLister) SetError(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
}

// ListResources lists resources for the given GVR and namespace.
func (l *MockResourceLister) ListResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return error if set
	if l.err != nil {
		return nil, l.err
	}

	gvrKey := gvr.String()
	if nsMap, exists := l.resources[gvrKey]; exists {
		if resources, exists := nsMap[namespace]; exists {
			// Return a copy to avoid mutation
			result := make([]*unstructured.Unstructured, len(resources))
			copy(result, resources)
			return result, nil
		}
	}
	return []*unstructured.Unstructured{}, nil
}

// MockSelectorMatcher is a mock implementation of SelectorMatcher for testing.
type MockSelectorMatcher struct {
	matches map[string]bool // resource key -> matches
	mu      sync.RWMutex
}

// NewMockSelectorMatcher creates a new MockSelectorMatcher.
func NewMockSelectorMatcher() *MockSelectorMatcher {
	return &MockSelectorMatcher{
		matches: make(map[string]bool),
	}
}

// SetMatch sets whether a resource should match.
func (m *MockSelectorMatcher) SetMatch(resource *unstructured.Unstructured, matches bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := resource.GetNamespace() + "/" + resource.GetName()
	m.matches[key] = matches
}

// MatchesSelectors returns the mock match result.
func (m *MockSelectorMatcher) MatchesSelectors(resource *unstructured.Unstructured, spec *v1alpha1.TargetResourceSpec) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := resource.GetNamespace() + "/" + resource.GetName()
	if match, exists := m.matches[key]; exists {
		return match
	}
	// Default to true if not explicitly set
	return true
}

// MockConditionMatcher is a mock implementation of ConditionMatcher for testing.
type MockConditionMatcher struct {
	meets map[string]bool // resource key -> meets conditions
	mu    sync.RWMutex
}

// NewMockConditionMatcher creates a new MockConditionMatcher.
func NewMockConditionMatcher() *MockConditionMatcher {
	return &MockConditionMatcher{
		meets: make(map[string]bool),
	}
}

// SetMeetsConditions sets whether a resource should meet conditions.
func (m *MockConditionMatcher) SetMeetsConditions(resource *unstructured.Unstructured, meets bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := resource.GetNamespace() + "/" + resource.GetName()
	m.meets[key] = meets
}

// MeetsConditions returns the mock result.
func (m *MockConditionMatcher) MeetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionsSpec) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := resource.GetNamespace() + "/" + resource.GetName()
	if meets, exists := m.meets[key]; exists {
		return meets
	}
	// Default to true if not explicitly set
	return true
}

// MockRateLimiterProvider is a mock implementation of RateLimiterProvider for testing.
type MockRateLimiterProvider struct {
	limiters map[types.UID]*ratelimiter.RateLimiter
	mu       sync.RWMutex
}

// NewMockRateLimiterProvider creates a new MockRateLimiterProvider.
func NewMockRateLimiterProvider() *MockRateLimiterProvider {
	return &MockRateLimiterProvider{
		limiters: make(map[types.UID]*ratelimiter.RateLimiter),
	}
}

// GetOrCreateRateLimiter returns a rate limiter for the given policy.
func (p *MockRateLimiterProvider) GetOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter {
	p.mu.Lock()
	defer p.mu.Unlock()
	if limiter, exists := p.limiters[policy.UID]; exists {
		return limiter
	}
	limiter := ratelimiter.NewRateLimiter(100) // Default to 100/sec for tests
	p.limiters[policy.UID] = limiter
	return limiter
}

// MockBatchDeleterCore is a mock implementation of BatchDeleterCore for testing.
type MockBatchDeleterCore struct {
	deleteResults map[string]error // resource key -> error (nil if success)
	mu            sync.RWMutex
}

// NewMockBatchDeleterCore creates a new MockBatchDeleterCore.
func NewMockBatchDeleterCore() *MockBatchDeleterCore {
	return &MockBatchDeleterCore{
		deleteResults: make(map[string]error),
	}
}

// SetDeleteResult sets the result for deleting a resource.
func (d *MockBatchDeleterCore) SetDeleteResult(resource *unstructured.Unstructured, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := resource.GetNamespace() + "/" + resource.GetName()
	d.deleteResults[key] = err
}

// DeleteBatch deletes a batch of resources and returns the number of successful deletions.
func (d *MockBatchDeleterCore) DeleteBatch(ctx context.Context, batch []*unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter, reasons map[string]string) (int64, []error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var deleted int64
	var errors []error
	for _, resource := range batch {
		key := resource.GetNamespace() + "/" + resource.GetName()
		if err, exists := d.deleteResults[key]; exists {
			if err != nil {
				errors = append(errors, err)
			} else {
				deleted++
			}
		} else {
			// Default to success if not explicitly set
			deleted++
		}
	}
	return deleted, errors
}

// MockStatusUpdater is a mock implementation of StatusUpdater for testing.
type MockStatusUpdater struct {
	err error
	mu  sync.RWMutex
}

// NewMockStatusUpdater creates a new MockStatusUpdater.
func NewMockStatusUpdater() *MockStatusUpdater {
	return &MockStatusUpdater{}
}

// SetError sets an error to return from UpdateStatus.
func (m *MockStatusUpdater) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// UpdateStatus updates the policy status (mock implementation).
func (m *MockStatusUpdater) UpdateStatus(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy, matchedCount, deletedCount, pendingCount int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.err
}
