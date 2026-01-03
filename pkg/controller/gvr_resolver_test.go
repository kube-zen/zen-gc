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
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kube-zen/zen-gc/pkg/validation"
)

// TestGVRResolver_WithRESTMapper tests GVRResolver with RESTMapper.
func TestGVRResolver_WithRESTMapper(t *testing.T) {
	// Create a test RESTMapper using standard Kubernetes resources
	restMapper := testrestmapper.TestOnlyStaticRESTMapper(runtime.NewScheme())

	// Create resolver with RESTMapper
	resolver := NewGVRResolver(restMapper)

	// Test with a standard resource (ConfigMap)
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	gvr, err := resolver.ResolveGVR(resource)
	if err != nil {
		t.Fatalf("ResolveGVR() returned error: %v", err)
	}

	expectedGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	if gvr != expectedGVR {
		t.Errorf("ResolveGVR() = %v, want %v", gvr, expectedGVR)
	}

	// Test caching - second call should use cache
	gvr2, err := resolver.ResolveGVR(resource)
	if err != nil {
		t.Fatalf("ResolveGVR() (cached) returned error: %v", err)
	}
	if gvr2 != expectedGVR {
		t.Errorf("ResolveGVR() (cached) = %v, want %v", gvr2, expectedGVR)
	}
}

// TestGVRResolver_WithoutRESTMapper tests GVRResolver without RESTMapper (fallback).
func TestGVRResolver_WithoutRESTMapper(t *testing.T) {
	// Create resolver without RESTMapper (nil)
	resolver := NewGVRResolver(nil)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	gvr, err := resolver.ResolveGVR(resource)
	if err != nil {
		t.Fatalf("ResolveGVR() returned error: %v", err)
	}

	// Should use pluralization fallback
	expectedGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: validation.PluralizeKind("ConfigMap"),
	}

	if gvr != expectedGVR {
		t.Errorf("ResolveGVR() = %v, want %v", gvr, expectedGVR)
	}
}

// TestGVRResolver_RESTMapperFailure tests fallback when RESTMapper fails.
func TestGVRResolver_RESTMapperFailure(t *testing.T) {
	// Create a RESTMapper that will fail for unknown resources
	restMapper := &failingRESTMapper{}

	resolver := NewGVRResolver(restMapper)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "custom.example.com/v1",
			"kind":       "CustomResource",
			"metadata": map[string]interface{}{
				"name": "test-resource",
			},
		},
	}

	gvr, err := resolver.ResolveGVR(resource)
	if err != nil {
		t.Fatalf("ResolveGVR() should fall back to pluralization, got error: %v", err)
	}

	// Should fall back to pluralization
	expectedGVR := schema.GroupVersionResource{
		Group:    "custom.example.com",
		Version:  "v1",
		Resource: validation.PluralizeKind("CustomResource"),
	}

	if gvr != expectedGVR {
		t.Errorf("ResolveGVR() = %v, want %v", gvr, expectedGVR)
	}
}

// failingRESTMapper is a RESTMapper that always fails (for testing fallback).
type failingRESTMapper struct{}

func (f *failingRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, &meta.NoResourceMatchError{}
}

func (f *failingRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, &meta.NoResourceMatchError{}
}

func (f *failingRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, &meta.NoResourceMatchError{}
}

func (f *failingRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, &meta.NoResourceMatchError{}
}

func (f *failingRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return nil, &meta.NoResourceMatchError{}
}

func (f *failingRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return nil, &meta.NoResourceMatchError{}
}

func (f *failingRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	return "", &meta.NoResourceMatchError{}
}
