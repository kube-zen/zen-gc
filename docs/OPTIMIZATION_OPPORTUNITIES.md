# Optimization Opportunities for zen-gc

This document identifies performance optimization opportunities in the zen-gc controller.

## Summary

After analyzing the codebase, the following optimization opportunities have been identified:

1. **Logger Reuse** (High Impact) - Creating new loggers in hot paths
2. **Slice Pre-allocation** (Medium Impact) - Slices created without capacity hints
3. **Duplicate Cache Calls** (Medium Impact) - Redundant `List()` calls
4. **String Concatenation** (Low Impact) - Using `+` instead of `strings.Builder`
5. **Context Check Optimization** (Low Impact) - Context checks in tight loops

## 1. Logger Reuse (High Impact)

### Problem
New loggers are created in hot paths (loops, frequent functions), causing unnecessary allocations.

### Locations
- `gc_controller.go`: Lines 195, 231, 266, 304, 321, 396, 449, 490, 515, 681, 746, 867, 874, 916, 1015, 1030, 1066, 1081
- `shared.go`: Lines 134, 195, 387, 468
- `reconciler.go`: Lines 168, 246, 316, 353, 474, 598, 637, 652, 666
- `evaluate_policy_shared.go`: Lines 65, 119, 179

### Solution
Create a single logger instance per struct and reuse it:

```go
// In GCController struct
type GCController struct {
    // ... existing fields ...
    logger sdklog.Logger // Add this
}

// In constructor
func NewGCController(...) *GCController {
    return &GCController{
        // ... existing fields ...
        logger: sdklog.NewLogger("zen-gc"), // Initialize once
    }
}

// Then use gc.logger instead of creating new ones
```

### Impact
- Reduces allocations in hot paths
- Improves performance for high-frequency operations
- Estimated: 5-10% reduction in GC pressure

## 2. Slice Pre-allocation (Medium Impact)

### Problem
Slices are created without capacity hints, causing multiple reallocations as they grow.

### Locations

#### `gc_controller.go:546`
```go
resourcesToDelete := make([]*unstructured.Unstructured, 0)
```
Should be:
```go
resourcesToDelete := make([]*unstructured.Unstructured, 0, len(resources)/10) // Estimate 10% match
```

#### `shared.go:155`
```go
errors := make([]error, 0)
```
Should be:
```go
errors := make([]error, 0, len(batch)) // Pre-allocate for worst case
```

#### `evaluate_policy_shared.go:58`
```go
ResourcesToDelete: make([]*unstructured.Unstructured, 0),
```
Should be:
```go
ResourcesToDelete: make([]*unstructured.Unstructured, 0, len(resources)/10),
```

### Impact
- Reduces slice reallocations
- Improves memory locality
- Estimated: 2-5% performance improvement

## 3. Duplicate Cache Calls (Medium Impact)

### Problem
`recordPolicyPhaseMetrics()` calls `List()` again immediately after `evaluatePolicies()` already called it.

### Location
`gc_controller.go:318-356`

```go
func (gc *GCController) evaluatePolicies() {
    // ...
    policies := gc.policyInformer.GetStore().List() // First call
    
    // ... evaluation ...
    
    gc.recordPolicyPhaseMetrics() // Calls List() again!
}

func (gc *GCController) recordPolicyPhaseMetrics() {
    policies := gc.policyInformer.GetStore().List() // Duplicate call
    // ...
}
```

### Solution
Pass the policies list to `recordPolicyPhaseMetrics()`:

```go
func (gc *GCController) evaluatePolicies() {
    // ...
    policies := gc.policyInformer.GetStore().List()
    
    // ... evaluation ...
    
    gc.recordPolicyPhaseMetrics(policies) // Pass the list
}

func (gc *GCController) recordPolicyPhaseMetrics(policies []interface{}) {
    // Use passed policies instead of calling List() again
    // ...
}
```

### Impact
- Eliminates redundant cache access
- Reduces lock contention
- Estimated: 1-3% performance improvement

## 4. String Concatenation (Low Impact)

### Problem
String concatenation using `+` in hot paths creates temporary allocations.

### Locations
- `gc_controller.go:522`: `policy.Namespace+"/"+policy.Name`
- `gc_controller.go:569`: Similar patterns
- Multiple locations with resource name formatting

### Solution
Use `fmt.Sprintf()` or `strings.Builder` for multiple concatenations:

```go
// Instead of:
policyKey := policy.Namespace + "/" + policy.Name

// Use:
policyKey := fmt.Sprintf("%s/%s", policy.Namespace, policy.Name)
// Or for many concatenations:
var b strings.Builder
b.WriteString(policy.Namespace)
b.WriteString("/")
b.WriteString(policy.Name)
policyKey := b.String()
```

### Impact
- Slightly better memory efficiency
- Estimated: <1% performance improvement

## 5. Context Check Optimization (Low Impact)

### Problem
Context cancellation checks in tight loops add overhead.

### Locations
- `gc_controller.go:549-556` - Context check in resource iteration loop
- `shared.go:160-166` - Context check in batch deletion loop

### Solution
Check context less frequently (e.g., every N iterations):

```go
const contextCheckInterval = 100

for i, obj := range resources {
    // Check context every 100 iterations
    if i%contextCheckInterval == 0 {
        select {
        case <-gc.ctx.Done():
            return nil
        default:
        }
    }
    // ... process resource ...
}
```

### Impact
- Reduces overhead in tight loops
- Still maintains responsiveness
- Estimated: <1% performance improvement

## 6. Map Pre-sizing (Low Impact)

### Problem
Maps are created without size hints when approximate size is known.

### Locations
- `gc_controller.go:547`: `make(map[string]string)`
- `gc_controller.go:365`: `make(map[string]float64)`

### Solution
Pre-size maps when approximate size is known:

```go
// If we expect ~10 policies
phaseCounts := make(map[string]float64, 10)

// If we expect ~len(resources)/10 deletions
resourcesToDeleteReasons := make(map[string]string, len(resources)/10)
```

### Impact
- Reduces map rehashing
- Estimated: <1% performance improvement

## 7. Batch Channel Pre-sizing (Low Impact)

### Problem
Channel created with exact size but could be optimized.

### Location
`gc_controller.go:429`
```go
policyChan := make(chan *v1alpha1.GarbageCollectionPolicy, len(policies))
```

### Current Status
Already optimized - channel is pre-sized correctly.

## Implementation Priority

1. **High Priority**: Logger reuse (#1)
2. **Medium Priority**: Slice pre-allocation (#2), Duplicate cache calls (#3)
3. **Low Priority**: String concatenation (#4), Context check optimization (#5), Map pre-sizing (#6)

## Testing Recommendations

After implementing optimizations:

1. Run benchmarks to measure improvement
2. Monitor memory allocations with `go test -benchmem`
3. Use pprof to verify reduced allocations
4. Test with realistic workloads (100+ policies, 1000+ resources)

## Example Benchmark

```go
func BenchmarkEvaluatePolicies(b *testing.B) {
    // Setup test data
    // ...
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        gc.evaluatePolicies()
    }
}
```

## Notes

- All optimizations should maintain existing functionality
- Performance improvements are estimates based on typical workloads
- Actual impact may vary based on cluster size and policy count
- Consider profiling before and after to measure real impact

