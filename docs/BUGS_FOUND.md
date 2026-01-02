# Bugs Found and Fixed

## Critical Bugs

### 1. Race Condition in `getOrCreateEvaluationService` ⚠️ **CRITICAL**

**Location:** `pkg/controller/gc_controller.go:518-546`

**Issue:** The check `if gc.evaluationService != nil` and assignment `gc.evaluationService = NewPolicyEvaluationService(...)` are not protected by a mutex. If multiple goroutines call this function concurrently (which can happen in parallel policy evaluation), multiple services could be created.

**Impact:** 
- Memory leak (multiple services created)
- Inconsistent behavior (different policies might use different service instances)
- Potential nil pointer dereferences

**Fix:** Add mutex protection using double-checked locking pattern.

### 2. Missing Nil Check for `informer.GetStore()` ⚠️ **HIGH**

**Location:** `pkg/controller/adapters.go:85`

**Issue:** `informer.GetStore()` could potentially return nil if the informer hasn't been properly initialized, leading to a nil pointer dereference in `NewInformerStoreResourceLister`.

**Impact:** Panic when trying to list resources from an uninitialized informer.

**Fix:** Add nil check before using the store.

### 3. Missing Context Check at Start of `evaluateResources` ⚠️ **MEDIUM**

**Location:** `pkg/controller/evaluate_policy_refactored.go:150-203`

**Issue:** The function checks context cancellation periodically (every 100 iterations) but not at the start. If the context is already canceled when the function is called, it will still iterate through all resources before checking.

**Impact:** Unnecessary work when context is already canceled, especially for large resource lists.

**Fix:** Add context check at the beginning of the function.

### 4. Potential Nil Rate Limiter ⚠️ **MEDIUM**

**Location:** `pkg/controller/evaluate_policy_refactored.go:213`

**Issue:** `GetOrCreateRateLimiter` could potentially return nil (though unlikely), and we pass it directly to `DeleteBatch` without checking.

**Impact:** Potential nil pointer dereference in batch deletion.

**Fix:** Add nil check and handle gracefully.

### 5. Missing Nil Check for Informer Store ⚠️ **MEDIUM**

**Location:** `pkg/controller/gc_controller.go:590`

**Issue:** `informer.GetStore()` is called without checking if the store is nil. While the informer should be valid (checked by error return), defensive programming suggests we should verify.

**Impact:** Potential panic if informer is in an invalid state.

**Fix:** Add defensive nil check.

## Medium Priority Issues

### 6. Missing Context Check in `deleteResourcesInBatches` ⚠️ **LOW**

**Location:** `pkg/controller/evaluate_policy_refactored.go:207-251`

**Issue:** Similar to `evaluateResources`, context is checked between batches but not at the start.

**Impact:** Minor - less critical since batches are typically smaller.

**Fix:** Add context check at the beginning.

### 7. Potential Resource Leak in Rate Limiter ⚠️ **LOW**

**Location:** `pkg/controller/shared.go:108-113`

**Issue:** If `limiter` is nil in the map, we still return it. This could lead to nil pointer dereferences downstream.

**Impact:** Potential panic if a nil limiter is stored in the map.

**Fix:** Check for nil before returning.

## Summary

- **Critical:** 1 bug (race condition)
- **High:** 1 bug (nil pointer)
- **Medium:** 3 bugs (context checks, nil checks)
- **Low:** 2 bugs (defensive programming)

Total: 7 potential bugs identified

