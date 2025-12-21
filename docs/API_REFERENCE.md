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
| `labelSelector` | LabelSelector | No | Label selector to filter resources |
| `fieldSelector` | FieldSelectorSpec | No | Field selector to filter resources |

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

See [examples/](../examples/) directory for complete examples.

---

## Validation Rules

1. **Target Resource**: `apiVersion` and `kind` are required
2. **TTL**: At least one TTL option must be specified
3. **Behavior**: `maxDeletionsPerSecond` and `batchSize` must be non-negative
4. **Propagation Policy**: Must be "Foreground", "Background", or "Orphan"

---

## See Also

- [User Guide](USER_GUIDE.md) - How to use the API
- [Operator Guide](OPERATOR_GUIDE.md) - Installation and configuration

