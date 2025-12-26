# Example Garbage Collection Policies

This directory contains example `GarbageCollectionPolicy` resources demonstrating various use cases and configurations.

## Quick Start

Apply any example policy:

```bash
kubectl apply -f examples/<example-name>.yaml
```

View policy status:

```bash
kubectl get garbagecollectionpolicies
kubectl describe garbagecollectionpolicies <policy-name>
```

## Examples

### Basic Examples

#### [temp-configmap-cleanup.yaml](temp-configmap-cleanup.yaml)
**Use Case**: Clean up temporary ConfigMaps after 1 hour

**Features**:
- Fixed TTL (1 hour)
- Label selector filtering
- Rate limiting (20 deletions/second)

**Apply**:
```bash
kubectl apply -f examples/temp-configmap-cleanup.yaml
```

#### [test-pod-cleanup.yaml](test-pod-cleanup.yaml)
**Use Case**: Remove test Pods after completion

**Features**:
- Fixed TTL (30 minutes)
- Phase condition (only Succeeded/Failed)
- Label selector for test pods

**Apply**:
```bash
kubectl apply -f examples/test-pod-cleanup.yaml
```

### Job Cleanup Examples

#### [completed-jobs-cleanup.yaml](completed-jobs-cleanup.yaml)
**Use Case**: Clean up completed Kubernetes Jobs after 24 hours

**Features**:
- Fixed TTL (24 hours)
- Phase condition (Succeeded/Failed only)
- Batch processing

**Apply**:
```bash
kubectl apply -f examples/completed-jobs-cleanup.yaml
```

**Why**: Jobs often leave behind completed pods. This policy ensures they're cleaned up automatically.

### Pod Cleanup Examples

#### [failed-pods-cleanup.yaml](failed-pods-cleanup.yaml)
**Use Case**: Quickly remove failed Pods

**Features**:
- Short TTL (5 minutes)
- Phase condition (Failed only)
- Fast deletion rate

**Apply**:
```bash
kubectl apply -f examples/failed-pods-cleanup.yaml
```

#### [evicted-pods-cleanup.yaml](evicted-pods-cleanup.yaml)
**Use Case**: Remove evicted Pods immediately

**Features**:
- Immediate deletion (0 seconds)
- Phase condition (Failed with reason Evicted)
- High deletion rate

**Apply**:
```bash
kubectl apply -f examples/evicted-pods-cleanup.yaml
```

**Why**: Evicted pods consume resources but serve no purpose. Clean them up quickly.

### Deployment Cleanup Examples

#### [old-deployments-cleanup.yaml](old-deployments-cleanup.yaml)
**Use Case**: Remove old Deployments that are no longer active

**Features**:
- Long TTL (30 days)
- Label selector for temporary deployments
- Foreground deletion (waits for dependents)

**Apply**:
```bash
kubectl apply -f examples/old-deployments-cleanup.yaml
```

### ReplicaSet Cleanup Examples

#### [orphaned-replicaset-cleanup.yaml](orphaned-replicaset-cleanup.yaml)
**Use Case**: Clean up ReplicaSets not owned by Deployments

**Features**:
- Fixed TTL (7 days)
- Label selector for orphaned ReplicaSets
- Background deletion

**Apply**:
```bash
kubectl apply -f examples/orphaned-replicaset-cleanup.yaml
```

**Why**: Orphaned ReplicaSets can accumulate over time. This policy cleans them up.

### Storage Cleanup Examples

#### [pvc-cleanup.yaml](pvc-cleanup.yaml)
**Use Case**: Remove Released PersistentVolumeClaims

**Features**:
- Fixed TTL (24 hours)
- Phase condition (Released only)
- Label selector for temporary PVCs

**Apply**:
```bash
kubectl apply -f examples/pvc-cleanup.yaml
```

**Why**: Released PVCs indicate the PV is no longer bound. Clean them up to free storage.

### Secret Cleanup Examples

#### [old-secrets-cleanup.yaml](old-secrets-cleanup.yaml)
**Use Case**: Remove old Secrets that are no longer used

**Features**:
- Long TTL (90 days)
- Label selector for temporary secrets
- Dry-run mode (for safety)

**Apply**:
```bash
kubectl apply -f examples/old-secrets-cleanup.yaml
```

**Note**: This example uses `dryRun: true` for safety. Remove this to enable actual deletion.

## Advanced Examples

### Field-Based TTL

Delete resources based on TTL field in the resource:

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

### Mapped TTL

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

### Relative TTL

Delete resources relative to last activity:

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

### Multi-Condition Cleanup

Only delete resources that meet multiple conditions:

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: multi-condition-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: Pod
  ttl:
    secondsAfterCreation: 3600
  conditions:
    phase: ["Succeeded", "Failed"]
    hasLabels:
      - key: "temp"
        operator: "Equals"
        value: "true"
    and:
      - fieldPath: "status.containerStatuses[0].restartCount"
        operator: "GreaterThan"
        value: "3"
```

## Best Practices

### 1. Start with Dry-Run

Always test policies in dry-run mode first:

```yaml
behavior:
  dryRun: true
```

### 2. Use Label Selectors

Be specific about which resources to clean up:

```yaml
targetResource:
  labelSelector:
    matchLabels:
      cleanup: "enabled"
      environment: "test"
```

### 3. Set Appropriate Rate Limits

Don't overwhelm the API server:

```yaml
behavior:
  maxDeletionsPerSecond: 10  # Conservative default
  batchSize: 50
```

### 4. Use Conditions for Safety

Only delete resources in safe states:

```yaml
conditions:
  phase: ["Succeeded", "Failed"]  # Only completed resources
```

### 5. Monitor Policy Status

Check policy status regularly:

```bash
kubectl get garbagecollectionpolicies -o wide
kubectl describe garbagecollectionpolicies <policy-name>
```

## Testing Examples

### Test in Dry-Run Mode

1. Apply policy with `dryRun: true`:
   ```bash
   kubectl apply -f examples/temp-configmap-cleanup.yaml
   ```

2. Check controller logs:
   ```bash
   kubectl logs -n gc-system -l app=gc-controller | grep "DRY RUN"
   ```

3. Verify resources would be deleted (but aren't)

4. Remove `dryRun: true` and reapply

### Test with Short TTL

For testing, use very short TTLs:

```yaml
ttl:
  secondsAfterCreation: 60  # 1 minute for testing
```

Then verify deletion happens quickly.

## Customizing Examples

### Change TTL

```yaml
ttl:
  secondsAfterCreation: 86400  # 1 day (in seconds)
```

Common TTL values:
- 1 hour: `3600`
- 1 day: `86400`
- 1 week: `604800`
- 1 month: `2592000`
- 3 months: `7776000`

### Change Rate Limits

```yaml
behavior:
  maxDeletionsPerSecond: 50  # Higher rate
  batchSize: 100             # Larger batches
```

### Change Propagation Policy

```yaml
behavior:
  propagationPolicy: "Foreground"  # Wait for dependents
  # Options: "Foreground", "Background", "Orphan"
```

## Troubleshooting

### Policy Not Working

1. Check policy status:
   ```bash
   kubectl get garbagecollectionpolicies -o wide
   kubectl describe garbagecollectionpolicies <policy-name>
   ```

2. Verify resources match selectors:
   ```bash
   kubectl get <resource-kind> -l <label-selector>
   ```

3. Check controller logs:
   ```bash
   kubectl logs -n gc-system -l app=gc-controller
   ```

### Resources Not Being Deleted

1. Verify TTL has expired:
   ```bash
   kubectl get <resource> -o jsonpath='{.metadata.creationTimestamp}'
   ```

2. Check conditions are met:
   ```bash
   kubectl get <resource> -o jsonpath='{.status.phase}'
   ```

3. Verify policy is Active:
   ```bash
   kubectl get garbagecollectionpolicies <policy-name> -o jsonpath='{.status.phase}'
   ```

## See Also

- [API Reference](../docs/API_REFERENCE.md) - Complete API documentation
- [User Guide](../docs/USER_GUIDE.md) - How to create and use policies
- [Operator Guide](../docs/OPERATOR_GUIDE.md) - Installation and configuration

