# User Guide

This guide explains how to use the Generic Garbage Collection (GC) controller to automatically clean up Kubernetes resources based on time-to-live (TTL) policies.

## Table of Contents

- [Quick Start](#quick-start)
- [Creating GC Policies](#creating-gc-policies)
- [TTL Configuration](#ttl-configuration)
- [Selectors](#selectors)
- [Conditions](#conditions)
- [Behavior Configuration](#behavior-configuration)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

### 1. Install the GC Controller

```bash
# Install CRD
kubectl apply -f deploy/crds/

# Install controller
kubectl apply -f deploy/manifests/
```

### 2. Create Your First Policy

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: temp-configmap-cleanup
  namespace: default
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
    maxDeletionsPerSecond: 10
```

### 3. Verify Policy Status

```bash
kubectl get garbagecollectionpolicies
kubectl describe garbagecollectionpolicy temp-configmap-cleanup
```

---

## Creating GC Policies

A `GarbageCollectionPolicy` defines:
- **What** resources to clean up (targetResource)
- **When** to clean them up (TTL)
- **Under what conditions** (conditions)
- **How** to clean them up (behavior)

### Basic Policy Structure

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: <policy-name>
  namespace: <namespace>
spec:
  targetResource:
    apiVersion: <api-version>
    kind: <kind>
  ttl:
    # TTL configuration (see below)
  conditions:
    # Optional conditions (see below)
  behavior:
    # Optional behavior configuration (see below)
```

---

## TTL Configuration

The `ttl` field defines when resources should be deleted. There are four options:

### Option 1: Fixed TTL

Delete all matching resources after a fixed time period:

```yaml
ttl:
  secondsAfterCreation: 604800  # 7 days
```

### Option 2: Field-Based TTL

Extract TTL from a resource field:

```yaml
ttl:
  fieldPath: "spec.ttlSeconds"  # Path to TTL field in resource
```

### Option 3: Mapped TTL

Map field values to different TTLs:

```yaml
ttl:
  fieldPath: "spec.severity"
  mappings:
    CRITICAL: 1814400  # 3 weeks
    HIGH: 1209600      # 2 weeks
    MEDIUM: 604800     # 1 week
    LOW: 259200        # 3 days
  default: 604800      # Default if no match
```

### Option 4: Relative TTL

TTL relative to another timestamp field:

```yaml
ttl:
  relativeTo: "status.lastProcessedAt"
  secondsAfter: 86400  # 1 day after lastProcessedAt
```

---

## Selectors

Use selectors to filter which resources a policy applies to.

### Label Selector

```yaml
targetResource:
  apiVersion: v1
  kind: Pod
  labelSelector:
    matchLabels:
      app: my-app
      environment: staging
    matchExpressions:
      - key: version
        operator: In
        values: ["v1", "v2"]
```

### Field Selector

```yaml
targetResource:
  apiVersion: v1
  kind: ConfigMap
  fieldSelector:
    matchFields:
      metadata.namespace: "zen-system"
```

### Namespace Scope

```yaml
targetResource:
  apiVersion: v1
  kind: ConfigMap
  namespace: "zen-system"  # Specific namespace
  # OR
  namespace: "*"           # All namespaces
```

---

## Conditions

Conditions define additional requirements that must be met before deletion.

### Phase Condition

Only delete resources in specific phases:

```yaml
conditions:
  phase: ["Succeeded", "Failed"]  # Only delete completed pods
```

### Label Condition

Only delete if resource has specific labels:

```yaml
conditions:
  hasLabels:
    - key: processed
      value: "true"
    - key: archived
      operator: Exists  # Label exists (any value)
```

### Annotation Condition

Only delete if resource has specific annotations:

```yaml
conditions:
  hasAnnotations:
    - key: cleanup-allowed
      value: "true"
```

### Field Conditions (AND Logic)

Complex conditions using field values:

```yaml
conditions:
  and:
    - fieldPath: "status.processed"
      operator: Equals
      value: "true"
    - fieldPath: "spec.severity"
      operator: In
      values: ["LOW", "INFO"]
```

**Supported Operators:**
- `Equals` - Field equals value
- `NotEquals` - Field does not equal value
- `In` - Field value is in list
- `NotIn` - Field value is not in list

---

## Behavior Configuration

Control how the GC controller executes deletions.

### Rate Limiting

```yaml
behavior:
  maxDeletionsPerSecond: 10  # Max deletions per second
```

### Batch Processing

```yaml
behavior:
  batchSize: 50  # Process resources in batches
```

### Dry Run

Test policies without actually deleting:

```yaml
behavior:
  dryRun: true  # Log deletions but don't delete
```

### Deletion Options

```yaml
behavior:
  gracePeriodSeconds: 30  # Grace period before force deletion
  propagationPolicy: Foreground  # Foreground, Background, or Orphan
```

---

## Examples

### Example 1: Temporary ConfigMap Cleanup

Delete temporary ConfigMaps after 1 hour:

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
```

### Example 3: Test Pod Cleanup

Delete completed test pods after 30 minutes:

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
    phase: ["Succeeded", "Failed"]
  behavior:
    maxDeletionsPerSecond: 5
    gracePeriodSeconds: 10
```

---

## Troubleshooting

### Policy Not Working

1. **Check policy status:**
   ```bash
   kubectl get garbagecollectionpolicy <name> -o yaml
   kubectl describe garbagecollectionpolicy <name>
   ```

2. **Check controller logs:**
   ```bash
   kubectl logs -n gc-system -l app=gc-controller
   ```

3. **Verify resources match selectors:**
   ```bash
   kubectl get <resource-kind> --selector=<label-selector>
   ```

### Resources Not Being Deleted

- **Check TTL:** Verify resources are actually expired
- **Check conditions:** Ensure resources meet all conditions
- **Check dry-run:** Make sure `dryRun: false` (or not set)
- **Check RBAC:** Ensure controller has delete permissions

### Too Many Deletions

- **Reduce rate:** Lower `maxDeletionsPerSecond`
- **Increase batch size:** Adjust `batchSize`
- **Add conditions:** Be more selective with conditions

### Performance Issues

- **Monitor metrics:** Check `/metrics` endpoint
- **Reduce GC interval:** Adjust controller sync interval
- **Optimize selectors:** Use more specific label/field selectors

---

## Best Practices

1. **Start with dry-run:** Always test policies with `dryRun: true` first
2. **Use specific selectors:** Narrow down resources with label/field selectors
3. **Set appropriate TTLs:** Balance retention needs with storage costs
4. **Monitor metrics:** Watch deletion rates and errors
5. **Use conditions:** Add conditions to prevent accidental deletions
6. **Test in staging:** Validate policies in non-production first

---

## See Also

- [Operator Guide](OPERATOR_GUIDE.md) - Installation and configuration
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Metrics Documentation](METRICS.md) - Prometheus metrics
- [Examples](../examples/) - More example policies

