# Leader Election for zen-gc

zen-gc uses **zen-sdk/pkg/leader** for mandatory leader election (controller-runtime Lease-based).

## Overview

zen-gc uses **zen-sdk/pkg/leader** (controller-runtime Manager) for leader election:
- ✅ Consistent approach across all Zen tools
- ✅ Uses controller-runtime Manager (only for leader election, not reconciliation)
- ✅ Standard Kubernetes Lease API
- ✅ **Mandatory** - leader election is always enabled (no off switch)

## Architecture

**Leader Responsibilities:**
- Runs all GarbageCollectionPolicy reconcilers
- Processes policy evaluations
- Manages resource deletions

**Follower Pods:**
- ❌ Do NOT run reconcilers (waits for leader election)
- ✅ Serve webhooks (if enabled) - load-balanced across pods

## Setup

### Step 1: Configure zen-gc Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gc-controller
spec:
  replicas: 2  # Default: 2 replicas for HA (leader election mandatory)
  template:
    spec:
      containers:
      - name: gc-controller
        image: kubezen/gc-controller:latest
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        # Leader election is mandatory and always enabled (via zen-sdk/pkg/leader)
        # No --ha-mode or --enable-leader-election flags needed
```

### Step 2: Verify Leader Status

```bash
# Check Lease resource (created automatically by controller-runtime)
kubectl get lease gc-controller-leader-election -n <namespace> -o yaml

# Check which pod holds the lease
kubectl get lease gc-controller-leader-election -n <namespace> -o jsonpath='{.spec.holderIdentity}'
```

## Behavior

**Leader election is mandatory and always enabled** (via zen-sdk/pkg/leader).

**Leader Pod:**
- ✅ Runs all GarbageCollectionPolicy reconcilers
- ✅ Processes policy evaluations
- ✅ Manages resource deletions
- ✅ Serves webhooks (if enabled)

**Follower Pods:**
- ❌ Do NOT run reconcilers (waits for leader election)
- ✅ Serve webhooks (if enabled) - load-balanced (run immediately)

**Note:** For single-replica deployments, set `replicas: 1`. Leader election is still enabled but only one pod exists.

## Benefits

1. **Prevents Duplicate Processing**
   - Only leader processes policies
   - Prevents duplicate deletions

2. **Resource Efficiency**
   - Followers don't run reconcilers
   - Reduces CPU/memory usage per pod

3. **Automatic Failover**
   - If leader crashes, new leader elected in seconds
   - Reconcilers automatically start on new leader

## Configuration

### Environment Variables

- `POD_NAMESPACE`: Namespace of the pod (required for leader election, set via Downward API)
- **Note:** Leader election is mandatory and always enabled. No flags or env vars to disable it.

### Leader Election Configuration

Leader election uses controller-runtime Manager with zen-sdk/pkg/leader:
- **Lease Duration**: 15 seconds (default)
- **Renew Deadline**: 10 seconds (default)
- **Retry Period**: 2 seconds (default)
- **Lease Name**: `gc-controller-leader-election`

These are configured via `zen-sdk/pkg/leader` and match zen-lead, zen-flow, zen-lock, and zen-watcher.

## Troubleshooting

### Pod Not Becoming Leader

1. **Check Lease resource:**
   ```bash
   kubectl get lease gc-controller-leader-election -n <namespace> -o yaml
   ```
   Verify `spec.holderIdentity` matches your pod name

2. **Check leader election manager logs:**
   ```bash
   kubectl logs <pod-name> | grep -i leader
   ```

3. **Verify environment variables:**
   ```bash
   kubectl exec <pod-name> -- env | grep -E "POD_NAMESPACE"
   ```

### Components Not Starting

1. **Check leader status:**
   ```bash
   kubectl logs <pod-name> | grep "leader"
   ```

2. **Verify POD_NAMESPACE is set:**
   ```bash
   kubectl exec <pod-name> -- env | grep -E "POD_NAMESPACE"
   ```

## Migration from Single-Replica

If you're currently running zen-gc as a single replica:

1. **Update Deployment:**
   - Add `POD_NAMESPACE` environment variable (via Downward API)
   - Increase replicas to 2 (or more) for HA
   - Leader election is automatically enabled (mandatory)
2. **Verify:** Check that only one pod is leader and reconcilers are running correctly

## Implementation Details

zen-gc uses **controller-runtime Manager** for leader election:
- Manager is created with leader election enabled via `zen-sdk/pkg/leader.ApplyRequiredLeaderElection()`
- Manager only calls Reconcile on the leader pod
- This ensures only one replica processes policies at a time

**Architecture:**
```
zen-gc (controller-runtime)
  ├── Manager (leader election via zen-sdk/pkg/leader)
  │   └── Uses Lease API (coordination.k8s.io)
  ├── GCPolicyReconciler (only runs on leader)
  └── Webhook server (runs on all pods)
```

---

**See also:**
- [zen-sdk Documentation](https://github.com/kube-zen/zen-sdk)
- [zen-lead Documentation](https://github.com/kube-zen/zen-lead) - For workload leader routing (not controller HA)
