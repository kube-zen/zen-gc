# Refactoring Plan: Eliminate Testing Complexity

## Problem

Tests require `NewSimpleDynamicClientWithCustomListKinds` to register resource types (like ConfigMaps) before creating informers. This is complex and error-prone.

**Root Cause**: Both `GCController` and `GCPolicyReconciler` directly create dynamic informers via `getOrCreateResourceInformer()`, which requires the fake client to know about resource types ahead of time.

## Solution

The `PolicyEvaluationService` already solves this problem using dependency injection with `ResourceLister` interface that can be easily mocked.

## Refactoring Steps

### Phase 1: Enable PolicyEvaluationService in GCPolicyReconciler ✅ HIGH PRIORITY

**Current State**: `GCPolicyReconciler.evaluatePolicy()` directly calls `getOrCreateResourceInformer()`.

**Target State**: Use `PolicyEvaluationService` with adapters (like `GCController` already has).

**Benefits**:
- ✅ Simple mock-based tests (no fake client setup needed)
- ✅ Better separation of concerns
- ✅ Consistent with refactoring already done
- ✅ All existing tests can use mocks

**Implementation**:
1. Add `PolicyEvaluationService` field to `GCPolicyReconciler`
2. Create `getOrCreateEvaluationService()` method (similar to `GCController`)
3. Modify `evaluatePolicy()` to use the service
4. Use adapters to bridge existing methods to interfaces

### Phase 2: Remove Deprecated GCController (Optional)

**Current State**: `GCController` is marked deprecated but still used in tests.

**Target State**: Remove `GCController` entirely or move to `internal/` package.

**Benefits**:
- ✅ Cleaner codebase
- ✅ Single implementation path
- ✅ Less confusion

**Note**: Only do this after Phase 1 is complete and all tests pass.

### Phase 3: Update Tests to Use Mocks

**Current State**: Many tests skip due to fake client complexity.

**Target State**: All tests use `PolicyEvaluationService` with mocks.

**Example**:
```go
func TestGCPolicyReconciler_EvaluatePolicy(t *testing.T) {
    // Simple mock setup - no fake client needed!
    mockLister := testing.NewMockResourceLister()
    mockLister.SetResources(gvr, "default", resources)
    
    mockSelectorMatcher := testing.NewMockSelectorMatcher()
    mockSelectorMatcher.SetMatch(resource, true)
    
    service := NewPolicyEvaluationService(mockLister, mockSelectorMatcher, ...)
    
    reconciler := &GCPolicyReconciler{
        evaluationService: service,
        // ... other fields
    }
    
    err := reconciler.evaluatePolicy(ctx, policy)
    // Test assertions...
}
```

## Impact Assessment

### Files to Modify

1. **`pkg/controller/reconciler.go`**:
   - Add `evaluationService *PolicyEvaluationService` field
   - Add `evaluationServiceMu sync.RWMutex` for thread safety
   - Add `getOrCreateEvaluationService()` method
   - Modify `evaluatePolicy()` to use service

2. **`pkg/controller/adapters.go`**:
   - Already has `GCControllerAdapter` - may need `GCPolicyReconcilerAdapter` or reuse

3. **Test Files**:
   - `pkg/controller/reconciler_test.go` - Update to use mocks
   - `pkg/controller/evaluate_policies_test.go` - Re-enable skipped tests with mocks

### Estimated Effort

- **Phase 1**: 2-3 hours (moderate complexity)
- **Phase 2**: 1-2 hours (low complexity, but requires careful testing)
- **Phase 3**: 3-4 hours (update all tests)

**Total**: ~6-9 hours

## Benefits

1. **✅ Eliminates Complex Test Setup**: No more `NewSimpleDynamicClientWithCustomListKinds`
2. **✅ Faster Tests**: Mocks are faster than fake clients
3. **✅ Better Test Coverage**: Can test edge cases easily with mocks
4. **✅ Consistent Architecture**: All evaluation goes through `PolicyEvaluationService`
5. **✅ Easier Maintenance**: Clear interfaces and separation of concerns

## Risks

1. **Low Risk**: The `PolicyEvaluationService` already exists and is tested
2. **Low Risk**: Adapters already exist for `GCController` - can reuse pattern
3. **Medium Risk**: Need to ensure backward compatibility during migration

## Recommendation

**✅ PROCEED with Phase 1** - This is a high-value, low-risk refactoring that will significantly improve testability.

The refactoring infrastructure is already in place; we just need to wire it up in `GCPolicyReconciler`.

