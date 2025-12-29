# Leader Election Guide

This document describes leader election behavior, configuration, and troubleshooting for the GC controller.

## Overview

The GC controller supports leader election for High Availability (HA) deployments. When leader election is enabled, multiple controller replicas can run simultaneously, but only one (the leader) actively processes GC policies. This ensures:

- **High Availability**: If the leader fails, another replica automatically takes over
- **No Duplicate Work**: Only one controller processes policies at a time
- **Zero Downtime**: Seamless failover when leadership changes

## How It Works

### Leader Election Mechanism

The controller uses [controller-runtime's built-in leader election](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/manager#Manager), which uses the Kubernetes Lease API:

1. **Lease Resource**: A `Lease` object is created in the controller namespace
2. **Lease Duration**: 15 seconds (how long a leader holds the lease) - configurable via Manager
3. **Renew Deadline**: 10 seconds (time to renew before losing leadership) - configurable via Manager
4. **Retry Period**: 2 seconds (how often to retry acquiring leadership) - configurable via Manager

**Note**: As of v0.0.2-alpha, leader election is handled automatically by controller-runtime Manager. The custom `LeaderElection` implementation has been replaced.

### Leader Responsibilities

When a controller instance becomes the leader (via controller-runtime Manager):

1. Manager starts the reconciler and begins processing policy reconciliations
2. Only the leader's reconciler processes `GarbageCollectionPolicy` resources
3. Metrics and health endpoints are available on all replicas
4. Leader election status is managed by controller-runtime

### Follower Behavior

When a controller instance is not the leader:

1. Manager does not start reconciliation (only leader processes policies)
2. `/readyz` endpoint returns `503 Service Unavailable` (managed by Manager)
3. `/healthz` endpoint returns `200 OK` on all replicas
4. Metrics server runs on all replicas
5. Manager monitors for leadership opportunities automatically

### Leadership Loss

When a leader loses leadership:

1. Manager stops reconciliation gracefully
2. Manager attempts to reacquire leadership automatically
3. Another replica becomes leader and starts processing
4. Zero downtime failover (seamless transition)

## Configuration

### Enabling Leader Election

Leader election is enabled by default. To disable:

```bash
--enable-leader-election=false
```

### Leader Election Namespace

The lease is created in the controller's namespace (detected from service account):

```bash
--leader-election-namespace=gc-system
```

### Environment Variables

- `POD_NAME`: Used as leader election identity (auto-detected from Kubernetes)
- `POD_NAMESPACE`: Used as leader election namespace (auto-detected from service account)

## Monitoring

### Metrics

#### `gc_leader_election_status`
- **Type**: Gauge (0 or 1)
- **Description**: Current leader election status
- **Value**: `1` if leader, `0` if follower

#### `gc_leader_election_transitions_total`
- **Type**: Counter
- **Description**: Total number of leadership transitions
- **Use**: Monitor for frequent leadership changes (may indicate issues)

### Prometheus Queries

#### Check current leader
```promql
gc_leader_election_status
```

#### Count leadership transitions per minute
```promql
rate(gc_leader_election_transitions_total[1m])
```

#### Alert on frequent leadership changes
```promql
rate(gc_leader_election_transitions_total[5m]) > 0.1
```

### Logs

Leader election events are logged at `INFO` level:

```
I1224 10:00:00.000000       1 leader_election.go:85] Became leader (identity: gc-controller-abc123, namespace: gc-system, name: gc-controller-leader-election)
I1224 10:00:15.000000       1 leader_election.go:97] Lost leadership (identity: gc-controller-abc123)
I1224 10:00:15.000000       1 leader_election.go:104] New leader elected: gc-controller-xyz789 (current identity: gc-controller-abc123)
```

## Troubleshooting

### Issue: No Leader Elected

**Symptoms**:
- No policies are being processed
- All replicas show `gc_leader_election_status 0`
- Logs show "Waiting for leadership..." but no "Became leader" message

**Possible Causes**:
1. **RBAC Issues**: Controller lacks permissions to create/update Lease resources
2. **API Server Issues**: Cannot communicate with Kubernetes API server
3. **Namespace Issues**: Controller namespace doesn't exist or is inaccessible

**Solutions**:
```bash
# Check RBAC permissions
kubectl auth can-i create leases --namespace=gc-system --as=system:serviceaccount:gc-system:gc-controller

# Check Lease resource
kubectl get lease gc-controller-leader-election -n gc-system

# Check controller logs
kubectl logs -n gc-system -l app=gc-controller | grep -i leader

# Verify API server connectivity
kubectl get nodes
```

### Issue: Frequent Leadership Changes

**Symptoms**:
- High `gc_leader_election_transitions_total` counter
- Logs show frequent "Became leader" / "Lost leadership" messages
- Policies may not be processed consistently

**Possible Causes**:
1. **Network Latency**: High latency to API server causes lease renewal failures
2. **API Server Overload**: API server is slow to respond to lease updates
3. **Resource Constraints**: Controller pods are being throttled or OOMKilled

**Solutions**:
```bash
# Check API server latency
kubectl get lease gc-controller-leader-election -n gc-system -v=9

# Check pod resource usage
kubectl top pods -n gc-system -l app=gc-controller

# Check for OOMKills
kubectl describe pod -n gc-system -l app=gc-controller | grep -i oom

# Check API server metrics (if available)
# Look for high latency or error rates
```

### Issue: Multiple Leaders

**Symptoms**:
- Multiple replicas show `gc_leader_election_status 1` simultaneously
- Duplicate resource deletions
- Logs show multiple "Became leader" messages

**Possible Causes**:
1. **Clock Skew**: System clocks are out of sync between nodes
2. **Lease API Bug**: Rare Kubernetes API server bug
3. **Network Partition**: Network split causing isolated leaders

**Solutions**:
```bash
# Check system time on all nodes
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.nodeInfo.systemUUID}{"\n"}{end}'

# Check Lease resource for multiple holders (should not happen)
kubectl get lease gc-controller-leader-election -n gc-system -o yaml

# Verify network connectivity between pods
kubectl exec -n gc-system <pod-name> -- ping <other-pod-ip>
```

### Issue: Leader Not Processing Policies

**Symptoms**:
- `gc_leader_election_status 1` (is leader)
- But no policy evaluations or deletions
- Logs show "Became leader" but no "Evaluating policies"

**Possible Causes**:
1. **Controller Startup Failure**: Controller failed to start after becoming leader
2. **Policy Informer Sync Issue**: Policy informer cache not synced
3. **Context Cancellation**: Context was canceled before processing started

**Solutions**:
```bash
# Check controller logs for errors
kubectl logs -n gc-system -l app=gc-controller --tail=100 | grep -i error

# Check if policies exist
kubectl get garbagecollectionpolicies --all-namespaces

# Check controller status
kubectl get pods -n gc-system -l app=gc-controller
kubectl describe pod -n gc-system -l app=gc-controller
```

### Issue: Readiness Probe Failing

**Symptoms**:
- `/readyz` endpoint returns `503 Service Unavailable`
- Pod shows `NotReady` status
- But controller is running

**Possible Causes**:
1. **Not Leader**: This is expected behavior - only the leader should be ready
2. **Leader Election Disabled**: Readiness check still checks leader status even if disabled (bug)

**Solutions**:
```bash
# Check if this is expected (not leader)
kubectl get pods -n gc-system -l app=gc-controller -o wide
# Only the leader pod should be Ready

# Check leader election status
curl http://<pod-ip>:8080/metrics | grep gc_leader_election_status

# If leader election is disabled, verify readiness check logic
```

## Best Practices

1. **Deploy Multiple Replicas**: Always deploy at least 2 replicas for HA
2. **Monitor Transitions**: Alert on frequent leadership changes
3. **Resource Limits**: Ensure pods have adequate CPU/memory to renew leases
4. **Network Policies**: Allow communication to API server for lease operations
5. **Clock Synchronization**: Use NTP to keep system clocks synchronized
6. **Readiness Probes**: Use `/readyz` endpoint for readiness probes (only leader is ready)

## Example Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gc-controller
  namespace: gc-system
spec:
  replicas: 2  # At least 2 for HA
  template:
    spec:
      containers:
      - name: gc-controller
        args:
        - --enable-leader-election=true
        - --leader-election-namespace=gc-system
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
```

## References

- [Kubernetes Leader Election](https://kubernetes.io/docs/reference/access-authn-authz/lease/)
- [client-go Leader Election](https://github.com/kubernetes/client-go/tree/master/tools/leaderelection)

