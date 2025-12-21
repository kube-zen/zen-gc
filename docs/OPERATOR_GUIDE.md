# Operator Guide

This guide is for operators who need to install, configure, and maintain the GC controller.

## Table of Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Upgrading](#upgrading)
- [Security](#security)

---

## Installation

### Prerequisites

- Kubernetes cluster 1.23+
- kubectl configured
- Cluster admin permissions (for CRD installation)

### Step 1: Install CRD

```bash
kubectl apply -f deploy/crds/gc.k8s.io_garbagecollectionpolicies.yaml
```

Verify installation:

```bash
kubectl get crd garbagecollectionpolicies.gc.k8s.io
```

### Step 2: Install Controller

```bash
# Create namespace
kubectl apply -f deploy/manifests/namespace.yaml

# Install RBAC (includes leader election and event permissions)
kubectl apply -f deploy/manifests/rbac.yaml

# Install Deployment (configured for HA with 2 replicas)
kubectl apply -f deploy/manifests/deployment.yaml

# Install Service
kubectl apply -f deploy/manifests/service.yaml
```

**Note**: The deployment is configured with 2 replicas and leader election for high availability. Only the leader instance will actively process policies.

Or use kustomize:

```bash
kubectl apply -k deploy/manifests/
```

### Step 3: Verify Installation

```bash
# Check controller is running (should see 2 pods)
kubectl get pods -n gc-system

# Check leader election lease (only one leader)
kubectl get lease gc-controller-leader-election -n gc-system

# Check logs (only leader will show active processing)
kubectl logs -n gc-system -l app=gc-controller

# Check metrics endpoint
kubectl port-forward -n gc-system svc/gc-controller-metrics 8080:8080
curl http://localhost:8080/metrics

# Check events (after creating a policy)
kubectl get events -n gc-system --field-selector involvedObject.kind=GarbageCollectionPolicy
```

---

## Configuration

### Environment Variables

The controller supports the following environment variables:

- `METRICS_ADDR` - Metrics server address (default: `:8080`)
- `KUBECONFIG` - Path to kubeconfig file (for local development)
- `POD_NAMESPACE` - Namespace for leader election (auto-detected from service account)
- `POD_NAME` - Pod name for leader election identity (auto-detected)

### Command Line Flags

```bash
--kubeconfig=""                    # Path to kubeconfig file
--master=""                        # Kubernetes API server address
--metrics-addr=":8080"             # Metrics server address
--enable-leader-election=true      # Enable leader election for HA (default: true)
--leader-election-namespace=""     # Namespace for leader election lease (default: POD_NAMESPACE)
```

### Resource Limits

Default resource limits in deployment:

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

Adjust based on:
- Number of policies
- Number of resources being watched
- Deletion rate

---

## Monitoring

### Metrics

The controller exposes Prometheus metrics on port 8080:

```bash
# Port forward to access metrics
kubectl port-forward -n gc-system svc/gc-controller-metrics 8080:8080

# View metrics
curl http://localhost:8080/metrics
```

Key metrics to monitor:
- `gc_policies_total` - Number of policies
- `gc_resources_deleted_total` - Deletion rate
- `gc_errors_total` - Error rate
- `gc_deletion_duration_seconds` - Deletion performance

### Health Checks

- `/healthz` - Liveness probe
- `/readyz` - Readiness probe

### Logging

Controller logs include:
- Policy evaluation events
- Resource deletion events
- Leader election events
- Errors and warnings

View logs:

```bash
# View all pods
kubectl logs -n gc-system -l app=gc-controller -f

# View leader only
kubectl logs -n gc-system -l app=gc-controller | grep "leader"
```

### Events

The controller emits Kubernetes events for:
- Policy lifecycle (created, updated, deleted)
- Policy evaluation results
- Resource deletions
- Errors

View events:

```bash
# View all GC events
kubectl get events -n gc-system --field-selector involvedObject.kind=GarbageCollectionPolicy

# View events for specific policy
kubectl describe garbagecollectionpolicy <policy-name> -n <namespace>
```

---

## Troubleshooting

### Controller Not Starting

1. **Check CRD installation:**
   ```bash
   kubectl get crd garbagecollectionpolicies.gc.k8s.io
   ```

2. **Check RBAC:**
   ```bash
   kubectl get clusterrole gc-controller
   kubectl get clusterrolebinding gc-controller
   ```

3. **Check pod status:**
   ```bash
   kubectl describe pod -n gc-system -l app=gc-controller
   ```

### Policies Not Working

1. **Check policy status:**
   ```bash
   kubectl get garbagecollectionpolicies --all-namespaces
   kubectl describe garbagecollectionpolicy <name> -n <namespace>
   ```

2. **Check controller logs:**
   ```bash
   kubectl logs -n gc-system -l app=gc-controller | grep <policy-name>
   ```

3. **Verify resources match:**
   ```bash
   # Check if resources match selectors
   kubectl get <resource-kind> --selector=<label-selector>
   ```

### High Resource Usage

1. **Reduce number of policies**
2. **Optimize selectors** - Use more specific label/field selectors
3. **Increase resource limits** if needed
4. **Scale horizontally** (if supported)

### Deletion Issues

1. **Check RBAC permissions:**
   ```bash
   kubectl auth can-i delete <resource-kind> --as=system:serviceaccount:gc-system:gc-controller
   ```

2. **Check for finalizers** blocking deletion
3. **Check resource quotas**
4. **Review error logs**

---

## Upgrading

### Backup Policies

Before upgrading, backup existing policies:

```bash
kubectl get garbagecollectionpolicies --all-namespaces -o yaml > policies-backup.yaml
```

### Upgrade Steps

1. **Backup policies** (see above)
2. **Update CRD** (if changed):
   ```bash
   kubectl apply -f deploy/crds/
   ```
3. **Update controller:**
   ```bash
   kubectl set image deployment/gc-controller gc-controller=kube-zen/gc-controller:<new-version> -n gc-system
   ```
4. **Verify upgrade:**
   ```bash
   kubectl rollout status deployment/gc-controller -n gc-system
   kubectl logs -n gc-system -l app=gc-controller
   ```

### Rollback

If upgrade fails:

```bash
kubectl rollout undo deployment/gc-controller -n gc-system
```

---

## Security

### RBAC

The controller requires:
- **Read/Write** access to `GarbageCollectionPolicy` CRDs
- **Read/Delete** access to all resources (for GC operations)
- **Read** access to namespaces

### Service Account

The controller runs as a non-root user (UID 65534) with minimal permissions.

### Network Policies

If using network policies, allow:
- Controller → API server (all ports)
- Prometheus → Controller metrics (port 8080)

### Admission Webhooks

Consider using admission webhooks to:
- Validate policy syntax
- Prevent dangerous policies
- Audit policy changes

---

## Performance Tuning

### For Large Clusters

1. **Increase informer resync interval**
2. **Use more specific selectors**
3. **Batch deletions** (already built-in)
4. **Monitor API server load**

### For High Deletion Rates

1. **Increase `maxDeletionsPerSecond`**
2. **Increase `batchSize`**
3. **Monitor API server rate limits**
4. **Consider API server scaling**

---

## Backup and Recovery

### Backup Policies

```bash
# Backup all policies
kubectl get garbagecollectionpolicies --all-namespaces -o yaml > policies-backup.yaml

# Backup specific namespace
kubectl get garbagecollectionpolicies -n <namespace> -o yaml > policies-<namespace>.yaml
```

### Restore Policies

```bash
kubectl apply -f policies-backup.yaml
```

---

## Uninstallation

### Remove Policies

```bash
kubectl delete garbagecollectionpolicies --all-namespaces --all
```

### Remove Controller

```bash
kubectl delete -f deploy/manifests/
```

### Remove CRD

```bash
kubectl delete crd garbagecollectionpolicies.gc.k8s.io
```

**Warning:** Removing the CRD will delete all policies!

---

## See Also

- [User Guide](USER_GUIDE.md) - How to use GC policies
- [Metrics Documentation](METRICS.md) - Monitoring and metrics
- [API Reference](API_REFERENCE.md) - Complete API documentation

