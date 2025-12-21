# KEP: Generic Garbage Collection Controller for Kubernetes

**Status**: Draft  
**Last Updated**: 2025-12-21  
**Authors**: Kube-ZEN Community  
**SIG**: sig-apps (primary), sig-architecture (secondary)  
**KEP Number**: TBD

---

## Summary

This proposal introduces a generic, declarative garbage collection mechanism for Kubernetes resources via a new `GarbageCollectionPolicy` CRD. This enables automatic, time-based cleanup of any Kubernetes resource (CRDs, ConfigMaps, Secrets, Pods, etc.) without requiring custom controllers or external dependencies.

**Note**: This is a **standalone project** (`zen-gc`) separate from `zen-watcher`. zen-watcher has its own built-in GC and will continue to work independently. This KEP proposes a generic solution for the entire Kubernetes ecosystem.

**Key Value Proposition**:
- **Declarative**: Define GC policies as Kubernetes resources (CRDs)
- **Generic**: Works with any Kubernetes resource type (CRDs, core resources, etc.)
- **Kubernetes-Native**: Uses standard Kubernetes patterns (spec fields, not annotations)
- **Zero Dependencies**: No external controllers or policy engines required
- **Production-Ready**: Built-in rate limiting, metrics, and observability

---

## Motivation

### Problem Statement

Kubernetes currently lacks a generic mechanism for automatic, time-based resource cleanup:

1. **Limited Native Support**: Only Jobs have built-in TTL (`spec.ttlSecondsAfterFinished`), introduced in v1.23
2. **No Generic Solution**: CRDs, ConfigMaps, Secrets, and other resources require custom controllers
3. **Fragmented Ecosystem**: Multiple third-party solutions (k8s-ttl-controller, Kyverno cleanup policies) with different approaches
4. **Operational Overhead**: Operators must build and maintain custom GC logic for each resource type
5. **Inconsistent Patterns**: Some use annotations, others use spec fields, creating confusion

### Use Cases

1. **Observability Data**: Auto-cleanup of ConfigMaps, metrics, logs after retention period
2. **Temporary Resources**: Auto-delete ConfigMaps, Secrets created for temporary operations
3. **Test Artifacts**: Automatic cleanup of test resources (Pods, Services) after test completion
4. **Audit Trails**: Time-based retention policies for audit logs stored as CRDs
5. **Multi-Tenant Isolation**: Per-tenant GC policies for namespace-scoped resources
6. **Cost Optimization**: Automatic cleanup of expensive resources (PVCs, LoadBalancers) after TTL

### Current Workarounds and Their Limitations

| Approach | Limitations |
|----------|-------------|
| **Custom Controllers** | Requires development, maintenance, and deployment for each resource type |
| **k8s-ttl-controller** | Annotation-based (not spec fields), external dependency, no declarative policies |
| **Kyverno Cleanup Policies** | Requires Kyverno installation, policy engine overhead, not suitable for all clusters |
| **CronJobs + kubectl** | Manual setup, no declarative policies, difficult to manage at scale |
| **Finalizers** | Only works for owner-based cleanup, not time-based |

---

## Goals

### Primary Goals

1. **Generic Resource Support**: Work with any Kubernetes resource type (CRDs, core resources, namespaced, cluster-scoped)
2. **Declarative Policies**: Define GC policies as Kubernetes CRDs (declarative, versioned, auditable)
3. **Kubernetes-Native Patterns**: Use spec fields (like Jobs) rather than annotations or external config
4. **Production-Grade**: Built-in rate limiting, metrics, observability, and error handling
5. **Zero External Dependencies**: No requirement for policy engines, external controllers, or additional infrastructure

### Non-Goals

1. **Not a Replacement for Owner References**: This KEP does not replace Kubernetes' built-in owner reference GC
2. **Not a Policy Engine**: This is a GC mechanism, not a general-purpose policy engine (like OPA/Gatekeeper)
3. **Not a Backup/Restore Solution**: GC policies are for cleanup, not data retention/backup
4. **Not a Replacement for Resource Quotas**: GC policies don't enforce resource limits

---

## Proposal

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Kubernetes API Server                       │
│  ┌──────────────────────────────────────────────────┐  │
│  │  GarbageCollectionPolicy CRD                      │  │
│  │  - spec.targetResource                           │  │
│  │  - spec.ttlSecondsAfterCreation                  │  │
│  │  - spec.conditions                               │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                        │
                        │ watches
                        ▼
┌─────────────────────────────────────────────────────────┐
│         GC Controller (Built-in Kubernetes)             │
│  ┌──────────────────────────────────────────────────┐  │
│  │  1. Watch GarbageCollectionPolicy CRDs           │  │
│  │  2. Watch target resources (via informers)       │  │
│  │  3. Evaluate TTL conditions                     │  │
│  │  4. Delete expired resources                     │  │
│  │  5. Emit metrics and events                      │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### Core API: GarbageCollectionPolicy CRD

**Group**: `gc.kube-zen.io`  
**Version**: `v1alpha1` (initial), `v1beta1` (after validation), `v1` (stable)  
**Kind**: `GarbageCollectionPolicy`  
**Scope**: `Namespaced` (for namespace-scoped resources) or `Cluster` (for cluster-scoped resources)

#### Basic Schema

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: cleanup-temp-configmaps-policy
  namespace: zen-system  # Optional: for namespaced policies
spec:
  # Target resource to apply GC policy
  targetResource:
    apiVersion: gc.kube-zen.io/v1alpha1
    kind: ConfigMap
    # Optional: label selector to filter resources
    labelSelector:
      matchLabels:
        temporary: "true"
  
  # TTL configuration
  ttl:
    # Option 1: Fixed TTL for all matching resources
    secondsAfterCreation: 604800  # 7 days
    
    # Option 2: Dynamic TTL based on resource fields
    # fieldPath: spec.severity  # Extract TTL from resource spec
    # mappings:
    #   CRITICAL: 1814400  # 3 weeks
    #   HIGH: 1209600      # 2 weeks
    #   MEDIUM: 604800     # 1 week
    #   LOW: 259200        # 3 days
  
  # Optional: Additional conditions for deletion
  conditions:
    # Only delete if resource is in specific state
    phase: ["Succeeded", "Failed"]  # Only delete completed resources
    
    # Only delete if resource has specific labels/annotations
    hasLabels:
      - key: processed
        value: "true"
  
  # GC behavior configuration
  behavior:
    # Rate limiting: max deletions per second
    maxDeletionsPerSecond: 10
    
    # Batch size: delete resources in batches
    batchSize: 50
    
    # Dry run: don't actually delete, just log
    dryRun: false
    
    # Finalizer: add finalizer before deletion (for graceful cleanup)
    finalizer: "gc.kube-zen.io/cleanup"

status:
  # Policy status
  phase: Active  # Active, Paused, Error
  
  # Statistics
  resourcesMatched: 1250
  resourcesDeleted: 1200
  resourcesPending: 50
  
  # Last GC run
  lastGCRun: "2025-12-21T10:30:00Z"
  nextGCRun: "2025-12-21T11:30:00Z"
  
  # Conditions
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2025-12-21T10:00:00Z"
    - type: Error
      status: "False"
      message: ""
```

### Detailed Field Specifications

#### `spec.targetResource`

Defines which resources the GC policy applies to:

```yaml
targetResource:
  # Required: API version and kind
  apiVersion: string  # e.g., "v1", "apps/v1", "batch/v1"
  kind: string        # e.g., "ConfigMap", "Pod", "Job", "Secret"
  
  # Optional: Namespace scope (for namespaced resources)
  namespace: string   # Specific namespace, or "*" for all namespaces
  
  # Optional: Label selector
  labelSelector:
    matchLabels:
      temporary: "true"
    matchExpressions:
      - key: environment
        operator: In
        values: [dev, staging]
  
  # Optional: Field selector (for resources that support it)
  fieldSelector:
    metadata.namespace: zen-system
```

#### `spec.ttl`

Time-to-live configuration:

```yaml
ttl:
  # Option 1: Fixed TTL (seconds after creation)
  secondsAfterCreation: int64  # e.g., 604800 (7 days)
  
  # Option 2: Dynamic TTL from resource fields
  fieldPath: string  # JSONPath to TTL field, e.g., "spec.ttlSecondsAfterCreation"
  
  # Option 3: Mapped TTL based on resource field values
  fieldPath: "spec.severity"
  mappings:
    CRITICAL: 1814400  # 3 weeks in seconds
    HIGH: 1209600      # 2 weeks
    MEDIUM: 604800     # 1 week
    LOW: 259200        # 3 days
    default: 604800    # Default if no match
  
  # Option 4: Relative to another timestamp field
  relativeTo: "status.lastProcessedAt"  # TTL relative to this field
  secondsAfter: 86400  # 1 day after lastProcessedAt
```

#### `spec.conditions`

Additional conditions that must be met before deletion:

```yaml
conditions:
  # Only delete resources in specific phases/states
  phase: ["Succeeded", "Failed"]  # Array of allowed phases
  
  # Only delete if resource has specific labels
  hasLabels:
    - key: processed
      value: "true"
    - key: archived
      operator: Exists  # Label exists (any value)
  
  # Only delete if resource has specific annotations
  hasAnnotations:
    - key: cleanup-allowed
      value: "true"
  
  # Only delete if resource age exceeds TTL AND condition is met
  and:
    - fieldPath: "status.processed"
      operator: Equals
      value: true
    - fieldPath: "spec.severity"
      operator: In
      values: ["LOW", "INFO"]
```

#### `spec.behavior`

GC execution behavior:

```yaml
behavior:
  # Rate limiting
  maxDeletionsPerSecond: 10  # Max deletions per second (prevents API server overload)
  
  # Batch processing
  batchSize: 50  # Process resources in batches
  
  # Dry run mode
  dryRun: false  # If true, log deletions but don't actually delete
  
  # Finalizer for graceful cleanup
  finalizer: "gc.kube-zen.io/cleanup"  # Add finalizer, wait for removal before deletion
  
  # Deletion propagation
  propagationPolicy: Foreground  # Foreground, Background, Orphan
  
  # Grace period
  gracePeriodSeconds: 30  # Grace period before force deletion
```

---

## Design Details

### Controller Implementation

The GC controller is a built-in Kubernetes controller (similar to Job controller, Deployment controller):

1. **Policy Watcher**: Watches `GarbageCollectionPolicy` CRDs via informer
2. **Resource Watchers**: For each policy, creates informers for target resources
3. **Evaluation Loop**: Periodically evaluates resources against TTL and conditions
4. **Deletion Queue**: Queues deletions with rate limiting
5. **Metrics & Events**: Emits Prometheus metrics and Kubernetes events

### TTL Evaluation Logic

```go
func (gc *GCController) shouldDelete(resource *unstructured.Unstructured, policy *GarbageCollectionPolicy) (bool, string) {
    // 1. Check label/field selectors
    if !matchesSelectors(resource, policy.Spec.TargetResource) {
        return false, "selector_mismatch"
    }
    
    // 2. Check conditions
    if !meetsConditions(resource, policy.Spec.Conditions) {
        return false, "condition_not_met"
    }
    
    // 3. Calculate TTL
    ttlSeconds := calculateTTL(resource, policy.Spec.TTL)
    if ttlSeconds <= 0 {
        return false, "no_ttl"
    }
    
    // 4. Check if expired
    creationTime := resource.GetCreationTimestamp().Time
    expirationTime := creationTime.Add(time.Duration(ttlSeconds) * time.Second)
    
    if time.Now().After(expirationTime) {
        return true, "ttl_expired"
    }
    
    return false, "not_expired"
}
```

### Rate Limiting and Batching

To prevent API server overload:

1. **Per-Policy Rate Limiting**: Each policy has `maxDeletionsPerSecond`
2. **Global Rate Limiting**: Global limit across all policies (configurable)
3. **Batch Processing**: Process resources in batches (`batchSize`)
4. **Exponential Backoff**: Back off on API server errors

### Metrics and Observability

Prometheus metrics:

```go
var (
    gcPoliciesTotal = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "gc_policies_total",
            Help: "Total number of GC policies",
        },
        []string{"phase"},
    )
    
    gcResourcesMatched = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gc_resources_matched_total",
            Help: "Total number of resources matched by GC policies",
        },
        []string{"policy", "resource"},
    )
    
    gcResourcesDeleted = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gc_resources_deleted_total",
            Help: "Total number of resources deleted by GC",
        },
        []string{"policy", "resource", "reason"},
    )
    
    gcDeletionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "gc_deletion_duration_seconds",
            Help: "Time taken to delete resources",
        },
        []string{"policy", "resource"},
    )
    
    gcErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gc_errors_total",
            Help: "Total number of GC errors",
        },
        []string{"policy", "resource", "error_type"},
    )
)
```

### Security Considerations

1. **RBAC**: GC controller requires `delete` permissions on target resources
2. **Admission Webhooks**: Validate GC policies before creation
3. **Resource Quotas**: Respect resource quotas when deleting
4. **Finalizers**: Support finalizers for graceful cleanup
5. **Dry Run**: Support dry-run mode for testing

### Performance Considerations

1. **Informer Caching**: Use shared informers for efficient resource watching
2. **Batch Processing**: Process deletions in batches to reduce API calls
3. **Rate Limiting**: Built-in rate limiting prevents API server overload
4. **Lazy Evaluation**: Only evaluate resources when GC run is triggered
5. **Indexing**: Index resources by creation timestamp for efficient TTL evaluation

---

## Examples

### Example 1: ConfigMap Cleanup

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: cleanup-temp-configmaps
  namespace: zen-system
spec:
  targetResource:
    apiVersion: gc.kube-zen.io/v1alpha1
    kind: ConfigMap
    labelSelector:
      matchLabels:
        temporary: "true"
  ttl:
    secondsAfterCreation: 3600  # 1 hour
  behavior:
    maxDeletionsPerSecond: 10
    batchSize: 50
```

### Example 2: Temporary ConfigMap Cleanup

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: temp-configmap-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: ConfigMap
    labelSelector:
      matchLabels:
        temp: "true"
  ttl:
    secondsAfterCreation: 3600  # 1 hour
  behavior:
    maxDeletionsPerSecond: 20
    dryRun: false
```

### Example 3: Test Pod Cleanup

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: test-pod-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: Pod
    labelSelector:
      matchLabels:
        test: "true"
  ttl:
    secondsAfterCreation: 1800  # 30 minutes
  conditions:
    phase: ["Succeeded", "Failed"]  # Only delete completed pods
  behavior:
    maxDeletionsPerSecond: 5
    gracePeriodSeconds: 10
```

### Example 4: Cluster-Scoped Resource Cleanup

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: cluster-namespace-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: Namespace
    labelSelector:
      matchLabels:
        temporary: "true"
    # No namespace (cluster-scoped)
  ttl:
    secondsAfterCreation: 2592000  # 30 days
  behavior:
    maxDeletionsPerSecond: 5
    batchSize: 100
```

---

## Alternatives Considered

### 1. Annotation-Based Approach (k8s-ttl-controller)

**Pros**:
- Simple annotation-based TTL
- Works with existing resources

**Cons**:
- Not declarative (annotations on each resource)
- No policy management
- External dependency
- Doesn't follow Kubernetes spec field patterns

**Decision**: Rejected - We want declarative policies and spec fields (like Jobs)

### 2. Policy Engine Integration (Kyverno/OPA)

**Pros**:
- Leverages existing policy engines
- More flexible for complex policies

**Cons**:
- Requires external dependencies (Kyverno/OPA)
- Not suitable for all clusters
- Policy engines are heavy-weight for simple GC

**Decision**: Rejected - We want zero external dependencies

### 3. Extend Existing Controllers

**Pros**:
- No new controller needed
- Leverages existing patterns

**Cons**:
- Each resource type needs custom logic
- Not generic
- Maintenance burden

**Decision**: Rejected - We want a generic solution

### 4. Finalizers Only

**Pros**:
- Uses existing Kubernetes mechanisms

**Cons**:
- No time-based cleanup
- Requires custom controllers for each resource type
- Not declarative

**Decision**: Rejected - Doesn't solve time-based cleanup

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **API Server Overload** | High | Rate limiting, batching, exponential backoff |
| **Accidental Deletion** | High | Dry-run mode, admission webhooks, finalizers |
| **Performance Impact** | Medium | Efficient informers, lazy evaluation, indexing |
| **RBAC Complexity** | Medium | Clear documentation, RBAC examples |
| **Policy Conflicts** | Low | Policy priority, conflict detection |

---

## Graduation Criteria

### Alpha (v1alpha1)

- [x] GC controller implementation
- [x] Basic `GarbageCollectionPolicy` CRD
- [x] Fixed TTL support (`secondsAfterCreation`)
- [x] Label/field selector support
- [x] Basic metrics (Prometheus)
- [x] Unit tests (>80% coverage)
- [x] Documentation (API reference, user guide, operator guide)
- [x] Dynamic TTL (field-based, mappings, relative)
- [x] Conditions support (phase, labels, annotations, field conditions)
- [x] Rate limiting and batching
- [x] Dry-run mode

### Beta (v1beta1)

- [ ] E2E tests with kind/minikube
- [ ] Performance benchmarks
- [ ] Migration guide from custom controllers
- [ ] Admission webhook for policy validation
- [ ] Leader election for HA deployments
- [ ] Enhanced observability (events, structured logging)

### Stable (v1)

- [ ] Production deployments in multiple clusters (3+)
- [ ] Performance validated at scale (10k+ resources)
- [ ] Security audit completed
- [ ] API stability guaranteed (no breaking changes)
- [ ] Comprehensive documentation (all sections complete)
- [ ] Operator guides (deployment, troubleshooting)

---

## Implementation Plan

### Phase 1: Core Implementation (Alpha)

1. **GC Controller**: Basic controller implementation
2. **CRD Definition**: `GarbageCollectionPolicy` CRD
3. **Basic TTL**: Fixed TTL support
4. **Selectors**: Label and field selector support
5. **Metrics**: Basic Prometheus metrics

### Phase 2: Advanced Features (Beta)

1. **Dynamic TTL**: Field-based TTL, mappings
2. **Conditions**: Phase, label, annotation conditions
3. **Rate Limiting**: Per-policy and global rate limiting
4. **Batching**: Batch processing support
5. **Dry Run**: Dry-run mode

### Phase 3: Production Readiness (Stable)

1. **Performance**: Optimize for scale
2. **Security**: Security audit, RBAC hardening
3. **Observability**: Enhanced metrics, events, logging
4. **Documentation**: Comprehensive guides
5. **Migration**: Tools for migrating from custom controllers

---

## Open Questions

1. **Controller Location**: Should GC controller be built into kube-controller-manager or a separate component?
   - **Proposed Answer**: Initially as a separate component (like other controllers), with potential integration into kube-controller-manager in future releases
   
2. **Default Policies**: Should Kubernetes ship with default GC policies for common resources?
   - **Proposed Answer**: No default policies in initial release. Users create policies as needed. Future consideration for optional default policies.

3. **Policy Priority**: How to handle multiple policies matching the same resource?
   - **Proposed Answer**: All matching policies are evaluated. If any policy determines a resource should be deleted, it will be deleted. This allows multiple policies to work together (e.g., one for TTL, another for conditions).

4. **Cross-Namespace Policies**: Should cluster-scoped policies be able to target namespaced resources?
   - **Proposed Answer**: Yes, cluster-scoped policies can target namespaced resources using namespace selectors or wildcard namespace (`*`).

5. **TTL Field Standardization**: Should we standardize `ttlSecondsAfterCreation` field across all resources?
   - **Proposed Answer**: No. This KEP provides a generic mechanism. Individual resource types may choose to add native TTL fields, but this controller works with any resource without requiring changes to resource definitions.

---

## Test Plan

### Unit Tests

- **Coverage Target**: >80% code coverage
- **Test Areas**:
  - TTL calculation logic (fixed, field-based, mapped, relative)
  - Selector matching (label, field, namespace)
  - Condition evaluation (phase, labels, annotations, field conditions)
  - Rate limiting and batching
  - Policy validation
  - Error handling

### Integration Tests

- **Test Environment**: Fake Kubernetes client
- **Test Scenarios**:
  - Policy creation and updates
  - Resource matching and deletion
  - Multiple policies on same resource
  - Error scenarios (API failures, invalid policies)

### E2E Tests

- **Test Environment**: kind or minikube cluster
- **Test Scenarios**:
  - End-to-end policy lifecycle
  - Resource cleanup workflows
  - Rate limiting behavior
  - Dry-run mode
  - Metrics collection

### Performance Tests

- **Scale Targets**:
  - 1000+ policies
  - 10,000+ resources per policy
  - 100+ deletions per second
- **Metrics**:
  - API server load
  - Memory usage
  - CPU usage
  - Deletion throughput

---

## References

- [Kubernetes TTL-after-finished Controller](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/)
- [Kubernetes Garbage Collection](https://kubernetes.io/docs/concepts/architecture/garbage-collection/)
- [KEP Template](https://github.com/kubernetes/enhancements/tree/master/keps)
- [k8s-ttl-controller](https://github.com/TwiN/k8s-ttl-controller)
- [Kyverno Cleanup Policies](https://kyverno.io/docs/policy-types/cleanup-policy/)

---

## Appendix

### Comparison with Existing Solutions

| Feature | This KEP | k8s-ttl-controller | Kyverno | Custom Controllers |
|---------|----------|-------------------|---------|-------------------|
| **Declarative Policies** | ✅ | ❌ | ✅ | ❌ |
| **Spec Fields** | ✅ | ❌ | ❌ | ✅ |
| **Zero Dependencies** | ✅ | ❌ | ❌ | ✅ |
| **Generic** | ✅ | ✅ | ✅ | ❌ |
| **Built-in Kubernetes** | ✅ | ❌ | ❌ | ❌ |
| **Rate Limiting** | ✅ | ❌ | ❌ | ❌ |
| **Metrics** | ✅ | ❌ | ✅ | ❌ |
| **Conditions** | ✅ | ❌ | ✅ | ✅ |

### API Versioning Strategy

- **v1alpha1**: Initial implementation, may have breaking changes
- **v1beta1**: API stabilized, no breaking changes, but may add fields
- **v1**: Stable API, long-term support, deprecation policy applies

---

**Next Steps**:
1. Review and refine this KEP with SIG-apps
2. Create prototype implementation
3. Gather community feedback
4. Submit to Kubernetes Enhancement Proposals repository

