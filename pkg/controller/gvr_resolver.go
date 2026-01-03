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
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kube-zen/zen-gc/pkg/validation"
)

// GVRResolver provides GroupVersionResource resolution with caching.
// This replaces naive pluralization with discovery-based RESTMapper resolution
// to properly handle irregular Kinds and CRDs.
type GVRResolver struct {
	restMapper meta.RESTMapper
	cache      map[schema.GroupVersionKind]schema.GroupVersionResource
	mu         sync.RWMutex
}

// NewGVRResolver creates a new GVRResolver with RESTMapper.
// If restMapper is nil, falls back to pluralization-based resolution.
func NewGVRResolver(restMapper meta.RESTMapper) *GVRResolver {
	return &GVRResolver{
		restMapper: restMapper,
		cache:      make(map[schema.GroupVersionKind]schema.GroupVersionResource),
	}
}

// ResolveGVR resolves a GroupVersionResource from a resource's GroupVersionKind.
// Uses RESTMapper if available, otherwise falls back to pluralization.
func (r *GVRResolver) ResolveGVR(resource *unstructured.Unstructured) (schema.GroupVersionResource, error) {
	gvk := resource.GroupVersionKind()

	// Check cache first
	r.mu.RLock()
	if gvr, found := r.cache[gvk]; found {
		r.mu.RUnlock()
		return gvr, nil
	}
	r.mu.RUnlock()

	var gvr schema.GroupVersionResource

	// Use RESTMapper if available
	if r.restMapper != nil {
		mapping, err := r.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err == nil {
			gvr = mapping.Resource
		} else {
			// RESTMapper failed, fall back to pluralization
			gvr = r.resolveGVRWithPluralization(gvk)
		}
	} else {
		// No RESTMapper, use pluralization
		gvr = r.resolveGVRWithPluralization(gvk)
	}

	// Cache the result
	r.mu.Lock()
	r.cache[gvk] = gvr
	r.mu.Unlock()

	return gvr, nil
}

// resolveGVRWithPluralization resolves GVR using pluralization (fallback).
func (r *GVRResolver) resolveGVRWithPluralization(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	resource := validation.PluralizeKind(gvk.Kind)
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}
}
