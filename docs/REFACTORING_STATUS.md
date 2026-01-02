# Refactoring Status: Reducing Complexity for Better Testability

## Overview

We're refactoring the controller to reduce complexity and improve testability by:
1. Extracting interfaces for key dependencies
2. Using dependency injection
3. Separating business logic from infrastructure
4. Making functions easier to test without complex Kubernetes client setup

## Current Status

### ✅ Phase 1: Extract Interfaces (IN PROGRESS)

**Completed:**
- ✅ Created `pkg/controller/interfaces.go` with core interfaces:
  - `ResourceInformer` - abstracts informer implementation
  - `ResourceInformerFactory` - abstracts informer creation
  - `ResourceLister` - simple interface for listing resources
  - `SelectorMatcher` - abstracts selector matching logic
  - `ConditionMatcher` - abstracts condition matching logic
  - `RateLimiterProvider` - abstracts rate limiter creation
  - `BatchDeleterCore` - higher-level batch deletion interface
  - `PolicyEvaluatorCore` - composition interface

**In Progress:**
- ⏳ Create default implementations of interfaces
- ⏳ Refactor GCController to use interfaces
- ⏳ Create test helpers with mocks

**Notes:**
- Existing `BatchDeleter` interface in `shared.go` is kept for backward compatibility
- `TTLCalculator` interface exists but is currently empty (TTL logic is in shared functions)
- Method naming: existing methods use lowercase (e.g., `deleteBatch`), interfaces use uppercase (e.g., `DeleteBatch`) - will be aligned during refactoring

## Next Steps

### Immediate (Week 1)
1. Create default implementations for all interfaces
2. Create mock implementations for testing
3. Update one function (`evaluatePolicy`) to use interfaces as proof of concept

### Short-term (Week 2-3)
1. Refactor remaining functions to use interfaces
2. Update tests to use mocks instead of complex client setup
3. Measure coverage improvement

### Long-term (Week 4+)
1. Achieve 70%+ coverage
2. Document patterns for future development
3. Update contributing guide

## Benefits Realized So Far

1. **Clearer Architecture**: Interfaces document what dependencies are needed
2. **Better Documentation**: Interface contracts are explicit
3. **Foundation for Testing**: Can now create mocks easily

## Challenges

1. **Backward Compatibility**: Need to keep existing code working during refactoring
2. **Method Naming**: Existing methods use lowercase, Go conventions use uppercase for exported methods
3. **Gradual Migration**: Refactoring must be done incrementally to avoid breaking changes

## Success Metrics

- **Coverage**: Target 70%+ (currently 56%)
- **Test Setup Time**: Reduce from complex setup to simple mocks
- **Test Execution Time**: Faster tests (no real informers)
- **Code Maintainability**: Clear separation of concerns

## References

- [Refactoring Plan](./REFACTORING_PLAN.md) - Detailed plan
- [Coverage Status](./COVERAGE_STATUS.md) - Current coverage metrics

