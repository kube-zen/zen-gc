# Zen-SDK Migration Analysis for zen-gc

**Date:** 2025-01-02  
**Status:** Analysis Complete

## Executive Summary

This document analyzes local implementations in `zen-gc` that could be migrated to `zen-sdk` to reduce code duplication and improve maintainability.

## Current zen-sdk Usage ✅

`zen-gc` already uses the following zen-sdk packages:

1. **`zen-sdk/pkg/logging`** - Structured logging ✅
2. **`zen-sdk/pkg/gc/ratelimiter`** - Rate limiting ✅
3. **`zen-sdk/pkg/gc/backoff`** - Exponential backoff ✅
4. **`zen-sdk/pkg/gc/ttl`** - TTL calculation ✅
5. **`zen-sdk/pkg/lifecycle`** - Graceful shutdown ✅
6. **`zen-sdk/pkg/leader`** - Leader election ✅

## Migration Candidates

### 1. Error Handling (`pkg/errors/errors.go`) ⚠️ **HIGH PRIORITY**

**Current Implementation:**
- Custom `GCError` struct with policy/resource context
- Helper functions: `WithPolicy`, `WithResource`, `New`, `Wrap`, `Wrapf`
- ~120 lines of code

**zen-sdk Status:**
- `zen-sdk/pkg/logging/errors.go` exists but provides different functionality (error logging, not structured errors)
- No equivalent structured error type with context

**Recommendation:**
- **Option A (Recommended):** Create `zen-sdk/pkg/errors` with generic structured error types
  - Generic `ContextError` struct that can be extended
  - Support for arbitrary context fields (policy, resource, etc.)
  - Used by zen-gc, zen-lock, zen-watcher, and other components
- **Option B:** Keep local implementation if it's too GC-specific
  - However, error context patterns are likely needed elsewhere

**Migration Effort:** Medium  
**Impact:** High (reduces duplication across components)

---

### 2. Health Checks (`pkg/controller/health.go`) ⚠️ **MEDIUM PRIORITY**

**Current Implementation:**
- `HealthChecker` struct with `ReadinessCheck`, `LivenessCheck`, `StartupCheck`
- Tracks informer sync status and evaluation time
- ~150 lines of code

**zen-sdk Status:**
- No health check package in zen-sdk
- Health check patterns exist in `zen-platform/src/shared/health` but not in zen-sdk

**Recommendation:**
- **Create `zen-sdk/pkg/health`** with generic health check interfaces
  - `HealthChecker` interface
  - Standard patterns for readiness/liveness/startup
  - Can be used by all Kubernetes controllers
  - zen-gc, zen-lock, zen-watcher, zen-flow would benefit

**Migration Effort:** Medium  
**Impact:** Medium (useful for multiple components)

---

### 3. Event Recording (`pkg/controller/events.go`) ⚠️ **LOW PRIORITY**

**Current Implementation:**
- `EventRecorder` struct for Kubernetes events
- Records policy evaluation, resource deletion, errors
- ~200 lines of code

**zen-sdk Status:**
- No event recording package in zen-sdk
- Event recording is controller-runtime specific

**Recommendation:**
- **Option A:** Create `zen-sdk/pkg/events` if pattern is common
  - Check if zen-lock, zen-watcher, zen-flow also need event recording
  - If yes, create generic event recorder
- **Option B:** Keep local if GC-specific
  - Event recording may be too specific to each controller

**Migration Effort:** Low  
**Impact:** Low (may be too component-specific)

---

### 4. Status Updater (`pkg/controller/status_updater.go`) ❌ **NOT RECOMMENDED**

**Current Implementation:**
- Updates GarbageCollectionPolicy CRD status
- GC-specific logic (phase calculation, conditions)
- ~170 lines of code

**zen-sdk Status:**
- No generic status updater (would be too abstract)

**Recommendation:**
- **Keep local** - This is GC-specific business logic
- Status update patterns are too different across components
- Not worth abstracting

**Migration Effort:** N/A  
**Impact:** N/A

---

### 5. Configuration (`pkg/config/config.go`) ⚠️ **LOW PRIORITY**

**Current Implementation:**
- `ControllerConfig` struct with GC-specific settings
- Environment variable loading
- ~125 lines of code

**zen-sdk Status:**
- `zen-sdk/pkg/config` exists with `Validator` for environment validation
- Different purpose (validation vs. configuration struct)

**Recommendation:**
- **Option A:** Keep local if GC-specific
  - Configuration structs are typically component-specific
- **Option B:** Use `zen-sdk/pkg/config/validator` for env var validation
  - Could replace manual `os.Getenv` calls
  - But current implementation is simple and works

**Migration Effort:** Low  
**Impact:** Low (current implementation is fine)

---

## Summary Table

| Component | Current Lines | Priority | Recommendation | Effort |
|-----------|--------------|----------|----------------|--------|
| Error Handling | ~120 | **HIGH** | Create `zen-sdk/pkg/errors` | Medium |
| Health Checks | ~150 | **MEDIUM** | Create `zen-sdk/pkg/health` | Medium |
| Event Recording | ~200 | **LOW** | Evaluate if common pattern | Low |
| Status Updater | ~170 | **N/A** | Keep local (GC-specific) | N/A |
| Configuration | ~125 | **LOW** | Keep local (works fine) | Low |

## Recommended Action Plan

### Phase 1: High Priority (Error Handling)
1. **Create `zen-sdk/pkg/errors`**
   - Generic `ContextError` struct
   - Support for arbitrary context fields
   - Helper functions: `WithContext`, `Wrap`, `Wrapf`
   - Used by zen-gc, zen-lock, zen-watcher

2. **Migrate zen-gc**
   - Replace `pkg/errors/errors.go` with zen-sdk
   - Update all error creation sites
   - Test thoroughly

### Phase 2: Medium Priority (Health Checks)
1. **Create `zen-sdk/pkg/health`**
   - Generic `HealthChecker` interface
   - Standard patterns for Kubernetes controllers
   - Informer sync checking utilities

2. **Migrate zen-gc**
   - Replace `pkg/controller/health.go` with zen-sdk
   - Update health check registration

### Phase 3: Low Priority (Evaluate)
1. **Evaluate Event Recording**
   - Check if zen-lock, zen-watcher, zen-flow need similar patterns
   - If yes, create `zen-sdk/pkg/events`
   - If no, keep local

## Benefits of Migration

1. **Code Reuse:** Reduce duplication across components
2. **Consistency:** Standardized patterns across all Zen tools
3. **Maintainability:** Fix bugs once, benefit everywhere
4. **Testing:** Shared test coverage
5. **Documentation:** Centralized documentation

## Risks and Considerations

1. **Over-abstraction:** Don't abstract too early
2. **Breaking Changes:** SDK changes affect all components
3. **Versioning:** Need careful version management
4. **Component-Specific Logic:** Some code should stay local

## Next Steps

1. ✅ Complete analysis (this document)
2. ⏳ Review with team
3. ⏳ Prioritize based on impact
4. ⏳ Create zen-sdk packages for high-priority items
5. ⏳ Migrate zen-gc implementations
6. ⏳ Update other components (zen-lock, zen-watcher, etc.)

