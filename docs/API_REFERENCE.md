# API Reference

Complete API reference for the GarbageCollectionPolicy CRD.

## GarbageCollectionPolicy

`GarbageCollectionPolicy` is a namespaced resource that defines a garbage collection policy for Kubernetes resources.

### API Version

- **Group**: `gc.kube-zen.io`
- **Version**: `v1alpha1`
- **Kind**: `GarbageCollectionPolicy`
- **Plural**: `garbagecollectionpolicies`
- **Short Names**: `gcp`, `gcpolicy`

### Schema

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: string
  namespace: string
spec:
  targetResource: TargetResourceSpec
  ttl: TTLSpec
  conditions: ConditionsSpec (optional)
  behavior: BehaviorSpec (optional)
status:
  phase: string
  resourcesMatched: int64
  resourcesDeleted: int64
  resourcesPending: int64
  lastGCRun: string (RFC3339)
  nextGCRun: string (RFC3339)
  conditions: []Condition
```

---

## TargetResourceSpec

Defines which resources the GC policy applies to.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiVersion` | string | Yes | API version of target resource (e.g., "v1", "apps/v1", "batch/v1") |
| `kind` | string | Yes | Kind of target resource (e.g., "Pod", "ConfigMap", "Job", "Secret") |
| `namespace` | string | No | Namespace scope. Use "*" for all namespaces, or specific namespace |
| `labelSelector` | LabelSelector | No | Label selector to filter resources (pushed down to API server) |
| `fieldSelector` | FieldSelectorSpec | No | Field selector to filter resources (evaluated in-memory only) |

**Performance Note**: `labelSelector` is pushed down to the Kubernetes API server, reducing network traffic and API server load. `fieldSelector` is evaluated in-memory after resources are fetched, so it does not reduce API server load. For better performance, prefer `labelSelector` when possible.

### Example

```yaml
targetResource:
  apiVersion: v1
  kind: ConfigMap
  namespace: default
  labelSelector:
    matchLabels:
      temporary: "true"
```

---

## FieldSelectorSpec

Defines field-based selection for resources.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `matchFields` | map[string]string | Map of field paths to values (e.g., `metadata.namespace: "zen-system"`) |

### Important Performance Consideration

**Field selectors are evaluated in-memory only** and are **not** pushed down to the Kubernetes API server. This means:

- ✅ **Label selectors** (`labelSelector`): Filtered at the API server, reducing network traffic and API load
- ⚠️ **Field selectors** (`fieldSelector`): Evaluated after resources are fetched, **does not reduce API server load**

**Impact:**
- All resources matching the GVR, namespace, and label selector are fetched and cached
- Field selector filtering happens in the controller's memory
- For large resource sets, this can increase memory usage and API server load

**Recommendation:** Prefer `labelSelector` when possible for better performance. Use `fieldSelector` only when label-based filtering is not feasible.

### Example

```yaml
targetResource:
  apiVersion: v1
  kind: ConfigMap
  fieldSelector:
    matchFields:
      metadata.namespace: "zen-system"
      metadata.name: "temp-*"  # Note: wildcards not supported, exact match only
```

**Note:** Field selectors support exact value matching only. Complex operators (like wildcards, regex) are not supported. For complex matching, use `conditions.and` with field conditions instead.

---

## TTLSpec

Defines time-to-live configuration.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secondsAfterCreation` | int64 | No* | Fixed TTL in seconds after creation |
| `fieldPath` | string | No* | JSONPath to TTL field in resource |
| `mappings` | map[string]int64 | No | Map field values to TTL seconds |
| `default` | int64 | No | Default TTL for mappings when no match |
| `relativeTo` | string | No* | JSONPath to timestamp field for relative TTL |
| `secondsAfter` | int64 | No* | Seconds after relativeTo timestamp |

\* At least one TTL option must be specified.

### Examples

**Fixed TTL:**
```yaml
ttl:
  secondsAfterCreation: 604800  # 7 days
```

**Mapped TTL:**
```yaml
ttl:
  fieldPath: "spec.severity"
  mappings:
    CRITICAL: 1814400
    HIGH: 1209600
  default: 604800
```

**Relative TTL:**
```yaml
ttl:
  relativeTo: "status.lastProcessedAt"
  secondsAfter: 86400  # 1 day after
```

---

## ConditionsSpec

Defines additional conditions that must be met before deletion.

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | []string | Only delete resources in these phases |
| `hasLabels` | []LabelCondition | Only delete if resource has these labels |
| `hasAnnotations` | []AnnotationCondition | Only delete if resource has these annotations |
| `and` | []FieldCondition | All field conditions must be met (AND logic) |

### LabelCondition

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Label key |
| `value` | string | Label value (for Equals operator) |
| `operator` | string | Operator: "Exists", "Equals" (default) |

### AnnotationCondition

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Annotation key |
| `value` | string | Annotation value |

### FieldCondition

| Field | Type | Description |
|-------|------|-------------|
| `fieldPath` | string | JSONPath to field |
| `operator` | string | Operator: "Equals", "NotEquals", "In", "NotIn" |
| `value` | string | Value for Equals/NotEquals |
| `values` | []string | Values for In/NotIn |

---

## BehaviorSpec

Defines GC execution behavior.

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `maxDeletionsPerSecond` | int | 10 | Maximum deletions per second |
| `batchSize` | int | 50 | Process resources in batches |
| `dryRun` | bool | false | If true, log but don't delete |
| `finalizer` | string | "" | Finalizer to add before deletion |
| `propagationPolicy` | string | "Background" | "Foreground", "Background", or "Orphan" |
| `gracePeriodSeconds` | int64 | nil | Grace period before force deletion |

---

## Status Fields

### Phase

- `Active` - Policy is active and processing resources
- `Paused` - Policy is paused (skipped during evaluation)
- `Error` - Policy has errors

### Statistics

- `resourcesMatched` - Total resources matched by selectors
- `resourcesDeleted` - Total resources deleted
- `resourcesPending` - Resources matched but not yet expired

### Timestamps

- `lastGCRun` - Last time policy was evaluated
- `nextGCRun` - Next scheduled evaluation time

### Conditions

Standard Kubernetes conditions:
- `Ready` - Policy is ready and working
- `Error` - Policy has errors

---

## Field Path Syntax

Field paths use dot notation for nested fields:

- `spec` - Top-level spec field
- `spec.severity` - Nested field
- `status.lastProcessedAt` - Deeply nested field
- `metadata.namespace` - Metadata field

---

## Examples

### Example 1: Fixed TTL (Simple)

Delete all ConfigMaps with `temp: "true"` label after 1 hour:

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
```

### Example 2: Field-Based TTL

Delete resources based on TTL field in the resource itself:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: resource-controlled-ttl
spec:
  targetResource:
    apiVersion: v1
    kind: ConfigMap
  ttl:
    fieldPath: "spec.ttlSeconds"  # Resource defines its own TTL
```

### Example 3: Mapped TTL

Different TTLs based on resource severity:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: severity-based-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: ConfigMap
  ttl:
    fieldPath: "spec.severity"
    mappings:
      CRITICAL: 1814400  # 3 weeks
      HIGH: 1209600      # 2 weeks
      MEDIUM: 604800     # 1 week
      LOW: 259200        # 3 days
    default: 604800      # Default: 1 week
```

### Example 4: Relative TTL

Delete resources relative to last activity timestamp:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: activity-based-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: ConfigMap
  ttl:
    relativeTo: "status.lastActivityAt"
    secondsAfter: 604800  # 1 week after last activity
```

### Example 5: With Conditions

Only delete completed Jobs:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: completed-jobs-cleanup
spec:
  targetResource:
    apiVersion: batch/v1
    kind: Job
  ttl:
    secondsAfterCreation: 86400  # 24 hours
  conditions:
    phase: ["Succeeded", "Failed"]
```

### Example 6: Dry-Run Mode

Test policy without actually deleting:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: test-policy
spec:
  targetResource:
    apiVersion: v1
    kind: ConfigMap
  ttl:
    secondsAfterCreation: 3600
  behavior:
    dryRun: true  # Log but don't delete
```

### Example 7: High-Rate Deletion

Delete resources quickly with high rate limit:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: fast-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: Pod
    labelSelector:
      matchLabels:
        temp: "true"
  ttl:
    secondsAfterCreation: 300  # 5 minutes
  behavior:
    maxDeletionsPerSecond: 100
    batchSize: 100
```

### Example 8: Foreground Deletion

Delete resources with foreground propagation (wait for dependents):

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: foreground-deletion
spec:
  targetResource:
    apiVersion: apps/v1
    kind: Deployment
  ttl:
    secondsAfterCreation: 604800  # 7 days
  behavior:
    propagationPolicy: "Foreground"  # Wait for dependents
```

See [examples/](../examples/) directory for more complete examples.

---

## OpenAPI Specification

The CRD includes OpenAPI v3 schema validation. You can extract the OpenAPI spec:

```bash
# Get OpenAPI spec from CRD
kubectl get crd garbagecollectionpolicies.gc.kube-zen.io -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema}' | jq .

# Or view the full CRD
kubectl get crd garbagecollectionpolicies.gc.kube-zen.io -o yaml
```

The OpenAPI schema is also available in `deploy/crds/gc.kube-zen.io_garbagecollectionpolicies.yaml`.

---

## Validation Rules

1. **Target Resource**: `apiVersion` and `kind` are required
2. **TTL**: At least one TTL option must be specified:
   - `secondsAfterCreation` (fixed TTL)
   - `fieldPath` (field-based TTL)
   - `relativeTo` + `secondsAfter` (relative TTL)
3. **Behavior**: 
   - `maxDeletionsPerSecond` must be > 0
   - `batchSize` must be > 0
   - `propagationPolicy` must be "Foreground", "Background", or "Orphan"
4. **Namespace**: Must be valid DNS-1123 label or "*" for cluster-wide
5. **Label Selector**: Keys and values must be valid Kubernetes label names/values

---

## Troubleshooting

### Policy Not Working

**Symptoms**: Policy exists but no resources are deleted

**Check**:
1. Verify policy is Active:
   ```bash
   kubectl get garbagecollectionpolicies -o wide
   kubectl describe garbagecollectionpolicies <policy-name>
   ```

2. Check policy status:
   ```bash
   kubectl get garbagecollectionpolicies <policy-name> -o jsonpath='{.status}'
   ```

3. Verify resources match selectors:
   ```bash
   kubectl get <resource-kind> -l <label-selector>
   ```

4. Check controller logs:
   ```bash
   kubectl logs -n gc-system -l app=gc-controller
   ```

### Policy Shows Error Phase

**Symptoms**: Policy status shows `phase: Error`

**Check**:
1. View policy status:
   ```bash
   kubectl get garbagecollectionpolicies <policy-name> -o yaml
   ```

2. Check status conditions:
   ```bash
   kubectl get garbagecollectionpolicies <policy-name> -o jsonpath='{.status.conditions}'
   ```

3. Common errors:
   - Invalid `apiVersion` or `kind`
   - Invalid field path in TTL
   - Invalid label selector syntax

### Resources Not Matching

**Symptoms**: `resourcesMatched` is 0

**Check**:
1. Verify label selectors:
   ```bash
   kubectl get <resource-kind> --show-labels
   ```

2. Test label selector manually:
   ```bash
   kubectl get <resource-kind> -l <label-selector>
   ```

3. Check namespace scope:
   - Policy namespace must match resource namespace (or use "*")

### TTL Not Expiring

**Symptoms**: Resources exist longer than TTL

**Check**:
1. Verify resource creation time:
   ```bash
   kubectl get <resource-kind> <resource-name> -o jsonpath='{.metadata.creationTimestamp}'
   ```

2. Check if conditions are preventing deletion:
   ```bash
   kubectl get garbagecollectionpolicies <policy-name> -o jsonpath='{.spec.conditions}'
   ```

3. Verify TTL calculation:
   - For fixed TTL: Check `secondsAfterCreation` value
   - For field-based: Verify field exists and has valid value
   - For mapped: Verify field value matches mapping key

---

## API Versioning

### Current Version: v1alpha1

- **Status**: Alpha (may have breaking changes)
- **Stability**: Not guaranteed
- **Breaking Changes**: Possible in future releases

### Future Versions

- **v1beta1**: Planned after API stabilization
- **v1**: Stable API with long-term support

See [VERSION_COMPATIBILITY.md](VERSION_COMPATIBILITY.md) for migration guides.

---

## See Also

- [User Guide](USER_GUIDE.md) - How to use the API
- [Operator Guide](OPERATOR_GUIDE.md) - Installation and configuration
- [Examples](../examples/) - Complete example policies
- [Version Compatibility](VERSION_COMPATIBILITY.md) - Version migration guides

