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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-sdk/pkg/gc/ratelimiter"
)

// ResourceInformer provides access to Kubernetes resource informers.
// This interface abstracts away the concrete informer implementation,
// making it easier to test and mock.
type ResourceInformer interface {
	GetStore() cache.Store
	HasSynced() bool
	AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error)
}

// ResourceInformerFactory creates resource informers for given GVRs.
// This interface allows us to mock informer creation in tests.
type ResourceInformerFactory interface {
	// ForResource returns an informer for the given GVR.
	ForResource(gvr schema.GroupVersionResource) ResourceInformer
	
	// Start starts all informers created by this factory.
	Start(stopCh <-chan struct{})
}

// ResourceLister provides a simple interface for listing Kubernetes resources.
// This abstracts away the informer complexity and makes testing easier.
type ResourceLister interface {
	// ListResources lists all resources of the given GVR in the namespace.
	// If namespace is empty, lists cluster-scoped resources.
	ListResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error)
}

// SelectorMatcher checks if a resource matches the given selectors.
// This interface allows us to test selector logic independently.
type SelectorMatcher interface {
	// MatchesSelectors returns true if the resource matches all selectors in the spec.
	MatchesSelectors(resource *unstructured.Unstructured, spec *v1alpha1.TargetResourceSpec) bool
}

// TTLCalculator calculates TTL and expiration times for resources.
// This interface allows us to test TTL logic independently.
type TTLCalculator interface {
	// CalculateExpirationTime calculates when a resource should expire based on policy TTL.
	CalculateExpirationTime(resource *unstructured.Unstructured, ttlSpec *v1alpha1.TTLSpec) (int64, error)
}

// ConditionMatcher checks if a resource meets the given conditions.
// This interface allows us to test condition logic independently.
type ConditionMatcher interface {
	// MeetsConditions returns true if the resource meets all conditions in the policy.
	MeetsConditions(resource *unstructured.Unstructured, conditions *v1alpha1.ConditionSpec) bool
}

// RateLimiterProvider provides rate limiters for policies.
// This interface allows us to mock rate limiting in tests.
type RateLimiterProvider interface {
	// GetOrCreateRateLimiter returns a rate limiter for the given policy.
	GetOrCreateRateLimiter(policy *v1alpha1.GarbageCollectionPolicy) *ratelimiter.RateLimiter
}

// BatchDeleter handles batch deletion of resources.
// This interface allows us to test deletion logic independently.
type BatchDeleter interface {
	// DeleteBatch deletes a batch of resources and returns the number of successful deletions.
	DeleteBatch(ctx context.Context, batch []*unstructured.Unstructured, policy *v1alpha1.GarbageCollectionPolicy, rateLimiter *ratelimiter.RateLimiter, reasons map[string]string) (int64, []error)
}

// PolicyEvaluatorCore provides the core methods needed for policy evaluation.
// This is a composition of the above interfaces for convenience.
type PolicyEvaluatorCore interface {
	ResourceLister
	SelectorMatcher
	TTLCalculator
	ConditionMatcher
	RateLimiterProvider
	BatchDeleter
}

