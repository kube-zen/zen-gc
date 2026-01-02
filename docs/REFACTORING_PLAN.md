# Refactoring Plan: Reduce Complexity for Better Testability

## Problem Statement

Many controller functions require complex Kubernetes client setup for testing because:
1. **Tight coupling** to concrete Kubernetes clients (`dynamic.Interface`, informers)
2. **No dependency injection** - functions directly create/manage resources
3. **Mixed concerns** - business logic intertwined with infrastructure code
4. **Hard to mock** - no interfaces for key dependencies

## Current Issues

### 1. Direct Dependencies on Concrete Types
```go
// Current: Hard to test
func (gc *GCController) evaluatePolicy(policy *v1alpha1.GarbageCollectionPolicy) error {
    informer := gc.getOrCreateResourceInformer(policy) // Creates real informer
    resources := informer.GetStore().List() // Direct access
    // ...
}
```

### 2. No Interfaces for Key Operations
- Resource informer creation
- Resource listing
- Policy evaluation
- Batch deletion

### 3. Complex Setup Required
Tests need:
- Registered list kinds for all resources
- Proper scheme setup
- Fake informers that sync
- Complex mock factories

## Refactoring Strategy

### Phase 1: Extract Interfaces (High Priority)

#### 1.1 Resource Informer Interface
```go
// pkg/controller/interfaces.go
type ResourceInformer interface {
    GetStore() cache.Store
    HasSynced() bool
    AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error)
}

type ResourceInformerFactory interface {
    ForResource(gvr schema.GroupVersionResource) ResourceInformer
    Start(stopCh <-chan struct{})
}
```

#### 1.2 Resource Lister Interface
```go
type ResourceLister interface {
    ListResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error)
}
```

#### 1.3 Policy Evaluator Interface
```go
type PolicyEvaluator interface {
    EvaluatePolicy(ctx context.Context, policy *v1alpha1.GarbageCollectionPolicy) (*PolicyEvaluationResult, error)
}
```

### Phase 2: Dependency Injection

#### 2.1 Refactor GCController Constructor
```go
type GCController struct {
    // ... existing fields ...
    
    // New: Injected dependencies
    resourceInformerFactory ResourceInformerFactory
    resourceLister         ResourceLister
    policyEvaluator        PolicyEvaluator
}

func NewGCControllerWithDependencies(
    dynamicClient dynamic.Interface,
    statusUpdater *StatusUpdater,
    eventRecorder *EventRecorder,
    cfg *config.ControllerConfig,
    // New: Injected dependencies
    resourceInformerFactory ResourceInformerFactory,
    resourceLister ResourceLister,
    policyEvaluator PolicyEvaluator,
) (*GCController, error) {
    // Use injected dependencies instead of creating them
}
```

#### 2.2 Create Default Implementations
```go
// pkg/controller/infrastructure.go
type DefaultResourceInformerFactory struct {
    factory dynamicinformer.DynamicSharedInformerFactory
}

func (f *DefaultResourceInformerFactory) ForResource(gvr schema.GroupVersionResource) ResourceInformer {
    return f.factory.ForResource(gvr).Informer()
}

// Similar for other default implementations
```

### Phase 3: Separate Business Logic from Infrastructure

#### 3.1 Extract Policy Evaluation Logic
```go
// pkg/controller/policy_evaluator.go
type DefaultPolicyEvaluator struct {
    resourceLister ResourceLister
    selectorMatcher SelectorMatcher
    ttlCalculator TTLCalculator
}

func (e *DefaultPolicyEvaluator) EvaluatePolicy(
    ctx context.Context,
    policy *v1alpha1.GarbageCollectionPolicy,
) (*PolicyEvaluationResult, error) {
    // Pure business logic - easy to test
    resources, err := e.resourceLister.ListResources(ctx, ...)
    // ...
}
```

#### 3.2 Extract Resource Matching Logic
```go
// pkg/controller/selector_matcher.go
type SelectorMatcher interface {
    MatchesSelectors(resource *unstructured.Unstructured, spec *v1alpha1.TargetResourceSpec) bool
}

type DefaultSelectorMatcher struct {
    // Implementation
}
```

### Phase 4: Simplify Test Setup

#### 4.1 Create Test Helpers
```go
// pkg/controller/testing/helpers.go
func NewMockResourceInformer(resources []*unstructured.Unstructured) ResourceInformer {
    store := cache.NewStore(cache.MetaNamespaceKeyFunc)
    for _, r := range resources {
        store.Add(r)
    }
    return &mockInformer{store: store, synced: true}
}

func NewMockResourceLister(resources map[string][]*unstructured.Unstructured) ResourceLister {
    return &mockResourceLister{resources: resources}
}
```

#### 4.2 Example Test After Refactoring
```go
func TestGCController_evaluatePolicy(t *testing.T) {
    // Simple setup - no complex client needed
    mockInformer := testing.NewMockResourceInformer(testResources)
    mockLister := testing.NewMockResourceLister(testResourcesMap)
    mockEvaluator := testing.NewMockPolicyEvaluator()
    
    controller := NewGCControllerWithDependencies(
        nil, // Not needed anymore
        statusUpdater,
        eventRecorder,
        config,
        mockInformer,
        mockLister,
        mockEvaluator,
    )
    
    // Test is now simple and fast
    err := controller.evaluatePolicy(testPolicy)
    // ...
}
```

## Benefits

1. **Easier Testing**: No complex Kubernetes client setup needed
2. **Better Maintainability**: Clear separation of concerns
3. **More Testable**: Business logic isolated from infrastructure
4. **Community Adoption**: Standard patterns (dependency injection, interfaces)
5. **Higher Coverage**: Can easily test all code paths

## Implementation Plan

### Week 1: Extract Interfaces
- [ ] Create `pkg/controller/interfaces.go` with all interfaces
- [ ] Document interface contracts
- [ ] Create default implementations

### Week 2: Refactor Core Functions
- [ ] Refactor `evaluatePolicy` to use interfaces
- [ ] Refactor `getOrCreateResourceInformer` to use factory interface
- [ ] Refactor `deleteBatch` to use interfaces

### Week 3: Update Tests
- [ ] Create test helpers with mocks
- [ ] Update existing tests to use new interfaces
- [ ] Add tests for previously untestable functions

### Week 4: Validation
- [ ] Run full test suite
- [ ] Measure coverage improvement
- [ ] Update documentation

## Migration Strategy

1. **Backward Compatible**: Keep existing constructors, add new ones
2. **Gradual Migration**: Refactor one function at a time
3. **Test Coverage**: Ensure coverage doesn't decrease during migration
4. **Documentation**: Update docs as we go

## Success Metrics

- **Coverage**: Increase from 56% to 70%+
- **Test Setup Time**: Reduce from complex setup to simple mocks
- **Test Execution Time**: Faster tests (no real informers)
- **Code Maintainability**: Clear separation of concerns

## References

- [Go Testing Best Practices](https://golang.org/doc/effective_go#testing)
- [Dependency Injection in Go](https://blog.drewolson.org/dependency-injection-in-go)
- [Kubernetes Controller Patterns](https://kubernetes.io/docs/concepts/architecture/controller/)

