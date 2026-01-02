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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	sdklog "github.com/kube-zen/zen-sdk/pkg/logging"
)

// BenchmarkLoggerReuse benchmarks logger reuse vs creating new loggers
func BenchmarkLoggerReuse(b *testing.B) {
	// Benchmark: Creating new logger each time (old way)
	b.Run("NewLoggerEachTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger := sdklog.NewLogger("zen-gc")
			_ = logger
		}
	})

	// Benchmark: Reusing logger (new way)
	logger := sdklog.NewLogger("zen-gc")
	b.Run("ReuseLogger", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger
		}
	})
}

// BenchmarkStringConcatenation benchmarks string concatenation methods
func BenchmarkStringConcatenation(b *testing.B) {
	namespace := "test-namespace"
	name := "test-policy"

	// Benchmark: String concatenation (old way)
	b.Run("StringConcatenation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = namespace + "/" + name
		}
	})

	// Benchmark: fmt.Sprintf (new way)
	b.Run("FmtSprintf", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = fmt.Sprintf("%s/%s", namespace, name)
		}
	})
}

// BenchmarkSlicePreAllocation benchmarks slice allocation strategies
func BenchmarkSlicePreAllocation(b *testing.B) {
	const resourceCount = 1000
	const estimatedMatchRate = 10 // 10% match rate

	// Benchmark: No pre-allocation (old way)
	b.Run("NoPreAllocation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := make([]*unstructured.Unstructured, 0)
			for j := 0; j < resourceCount/estimatedMatchRate; j++ {
				slice = append(slice, &unstructured.Unstructured{})
			}
		}
	})

	// Benchmark: Pre-allocated (new way)
	b.Run("PreAllocated", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			estimatedSize := resourceCount / estimatedMatchRate
			if estimatedSize < 10 {
				estimatedSize = 10
			}
			slice := make([]*unstructured.Unstructured, 0, estimatedSize)
			for j := 0; j < resourceCount/estimatedMatchRate; j++ {
				slice = append(slice, &unstructured.Unstructured{})
			}
		}
	})
}

// BenchmarkMapPreSizing benchmarks map allocation strategies
func BenchmarkMapPreSizing(b *testing.B) {
	const expectedPhases = 3

	// Benchmark: No pre-sizing (old way)
	b.Run("NoPreSizing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m := make(map[string]float64)
			m["Active"] = 1.0
			m["Paused"] = 2.0
			m["Error"] = 3.0
		}
	})

	// Benchmark: Pre-sized (new way)
	b.Run("PreSized", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m := make(map[string]float64, expectedPhases)
			m["Active"] = 1.0
			m["Paused"] = 2.0
			m["Error"] = 3.0
		}
	})
}

// BenchmarkContextCheckFrequency benchmarks context check strategies
func BenchmarkContextCheckFrequency(b *testing.B) {
	ctx := context.Background()
	const iterations = 10000

	// Benchmark: Check every iteration (old way)
	b.Run("CheckEveryIteration", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < iterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	})

	// Benchmark: Check every 100 iterations (new way)
	b.Run("CheckEvery100Iterations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < iterations; j++ {
				if j%100 == 0 {
					select {
					case <-ctx.Done():
						return
					default:
					}
				}
			}
		}
	})
}

// BenchmarkRecordPolicyPhaseMetrics benchmarks with and without duplicate cache calls
func BenchmarkRecordPolicyPhaseMetrics(b *testing.B) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		b.Fatalf("Failed to create controller: %v", err)
	}

	// Create test policies
	policies := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		policy := &v1alpha1.GarbageCollectionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "default",
				UID:       types.UID("test-uid"),
			},
			Status: v1alpha1.GarbageCollectionPolicyStatus{
				Phase: PolicyPhaseActive,
			},
		}
		policies[i] = policy
	}

	// Benchmark: With duplicate cache call (old way - simulated)
	b.Run("WithDuplicateCacheCall", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate old way: call List() twice
			_ = controller.policyInformer.GetStore().List()
			_ = controller.policyInformer.GetStore().List()
		}
	})

	// Benchmark: Without duplicate cache call (new way)
	b.Run("WithoutDuplicateCacheCall", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// New way: call List() once and pass result
			policiesList := controller.policyInformer.GetStore().List()
			_ = policiesList
		}
	})
}

// BenchmarkEvaluatePolicyResources benchmarks resource evaluation with optimizations
func BenchmarkEvaluatePolicyResources(b *testing.B) {
	scheme := runtime.NewScheme()
	dynamicClient := fake.NewSimpleDynamicClient(scheme)
	statusUpdater := NewStatusUpdater(dynamicClient)
	eventRecorder := NewEventRecorder(nil)

	controller, err := NewGCController(dynamicClient, statusUpdater, eventRecorder)
	if err != nil {
		b.Fatalf("Failed to create controller: %v", err)
	}

	// Create test resources (policy not needed for this benchmark)

	// Create test resources
	resources := make([]*unstructured.Unstructured, 1000)
	for i := 0; i < 1000; i++ {
		resources[i] = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":              "test-configmap",
					"namespace":         "default",
					"uid":               "test-uid",
					"creationTimestamp": metav1.Now().Format(time.RFC3339),
				},
			},
		}
	}

	b.ResetTimer()
	b.Run("EvaluateResources", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// This would normally use the informer, but for benchmark we'll simulate
			// the resource iteration logic with optimizations
			estimatedDeletions := len(resources) / 10
			if estimatedDeletions < 10 {
				estimatedDeletions = 10
			}
			resourcesToDelete := make([]*unstructured.Unstructured, 0, estimatedDeletions)
			resourcesToDeleteReasons := make(map[string]string, estimatedDeletions)

			const contextCheckInterval = 100
			for j, resource := range resources {
				if j%contextCheckInterval == 0 {
					select {
					case <-controller.ctx.Done():
						return
					default:
					}
				}

				// Simulate matching logic
				if j%10 == 0 { // 10% match rate
					resourcesToDelete = append(resourcesToDelete, resource)
					resourcesToDeleteReasons[string(resource.GetUID())] = "ttl_expired"
				}
			}
		}
	})
}

