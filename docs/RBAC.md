# RBAC Permissions Documentation

This document explains all RBAC permissions required by the GC controller and why each permission is necessary.

## Overview

The GC controller requires cluster-wide permissions to:
1. Manage `GarbageCollectionPolicy` CRDs
2. Read and delete arbitrary Kubernetes resources (for garbage collection)
3. Perform leader election
4. Emit Kubernetes events

## Required Permissions

### 1. GarbageCollectionPolicy CRD Permissions

```yaml
- apiGroups:
    - gc.kube-zen.io
  resources:
    - garbagecollectionpolicies
    - garbagecollectionpolicies/status
  verbs:
    - get
    - list
    - watch
    - create
    - update
    - patch
    - delete
```

**Why Required:**
- **get, list, watch**: Controller needs to discover and monitor GC policies
- **create, update, patch**: Controller may create or update policies (if needed for future features)
- **delete**: Controller may delete policies (cleanup scenarios)
- **garbagecollectionpolicies/status**: Controller updates policy status to reflect evaluation results

**Security Considerations:**
- ✅ Scoped to specific API group (`gc.kube-zen.io`)
- ✅ Only affects GC policy resources, not user workloads
- ⚠️ **Note**: `create`, `update`, `patch`, `delete` on policies may not be strictly necessary for basic operation, but are included for completeness and future features

**Recommendation**: If you want to restrict policy management to administrators only, you can remove `create`, `update`, `patch`, `delete` verbs and use a separate Role/RoleBinding for policy management.

---

### 2. Resource Deletion Permissions

```yaml
- apiGroups:
    - "*"
  resources:
    - "*"
  verbs:
    - get
    - list
    - watch
    - delete
```

**Why Required:**
- **get, list, watch**: Controller needs to discover resources that match GC policies
- **delete**: Controller deletes resources that meet GC criteria (expired TTL, conditions not met)

**Security Considerations:**
- ⚠️ **Broad Permissions**: This is the most sensitive permission set
- ⚠️ **Cluster-Wide**: Applies to all resources in all namespaces
- ✅ **Read-Only Operations**: Only `get`, `list`, `watch` are used for discovery
- ✅ **Controlled Deletion**: Deletion only happens based on explicit policies

**Why This Is Necessary:**
The GC controller is designed to be **generic** - it must work with any Kubernetes resource type (ConfigMaps, Secrets, Pods, Deployments, etc.). This requires broad permissions.

**Mitigation Strategies:**

1. **Policy-Level Restrictions**: Use label selectors and conditions in policies to limit what can be deleted:
   ```yaml
   apiVersion: gc.kube-zen.io/v1alpha1
   kind: GarbageCollectionPolicy
   spec:
     targetResource:
       labelSelector:
         matchLabels:
           gc-allowed: "true"  # Only delete resources with this label
   ```

2. **Namespace Scoping**: Deploy separate controllers per namespace/tenant with namespace-scoped RBAC (see below)

3. **Admission Webhooks**: Use validating webhooks to restrict which policies can be created

4. **Audit Logging**: Enable Kubernetes audit logging to track all deletions

5. **Dry-Run Mode**: Test policies in dry-run mode before enabling deletion

**Alternative: Namespace-Scoped RBAC**

For multi-tenant environments, you can use namespace-scoped permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: gc-controller
  namespace: tenant-a
rules:
- apiGroups: ["gc.kube-zen.io"]
  resources: ["garbagecollectionpolicies"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list", "watch", "delete"]
```

**Limitation**: This requires deploying a separate controller per namespace, which may not be practical for all environments.

---

### 3. Namespace Read Permissions

```yaml
- apiGroups:
    - ""
  resources:
    - namespaces
  verbs:
    - get
    - list
    - watch
```

**Why Required:**
- Controller needs to read namespaces to:
  - Validate namespace-scoped policies
  - Filter resources by namespace
  - Check namespace existence before processing policies

**Security Considerations:**
- ✅ Read-only operations
- ✅ No ability to modify namespaces
- ✅ Standard permission for most controllers

---

### 4. Leader Election Permissions

```yaml
- apiGroups:
    - coordination.k8s.io
  resources:
    - leases
  verbs:
    - get
    - list
    - watch
    - create
    - update
    - patch
```

**Why Required:**
- **get, list, watch**: Monitor leader election lease
- **create**: Create lease resource on first startup
- **update, patch**: Renew lease to maintain leadership

**Security Considerations:**
- ✅ Scoped to specific resource type (leases)
- ✅ Only affects leader election, not user workloads
- ✅ Standard permission for HA controllers

**Note**: If leader election is disabled (`--enable-leader-election=false`), this permission is not required.

---

### 5. Event Permissions

```yaml
- apiGroups:
    - ""
  resources:
    - events
  verbs:
    - create
    - patch
```

**Why Required:**
- Controller emits Kubernetes events for:
  - Policy lifecycle (created, updated, deleted)
  - Resource deletions
  - Errors and warnings

**Security Considerations:**
- ✅ Events are informational only
- ✅ Cannot read or delete events (only create/patch)
- ✅ Standard permission for controllers

---

## Permission Summary

| Permission | Scope | Risk Level | Required? |
|-----------|-------|------------|-----------|
| GC Policy CRUD | `gc.kube-zen.io` | Low | Yes (core functionality) |
| Resource Deletion | `*/*` | **High** | Yes (core functionality) |
| Namespace Read | `""/namespaces` | Low | Yes (namespace filtering) |
| Leader Election | `coordination.k8s.io/leases` | Low | Yes (if HA enabled) |
| Events | `""/events` | Low | Yes (observability) |

## Least-Privilege Recommendations

### 1. Remove Unnecessary Policy Verbs

If policy management is handled separately (e.g., by administrators), you can remove:

```yaml
# Remove these verbs if policies are managed externally
- create
- update
- patch
- delete
```

**Minimal Policy Permissions:**
```yaml
- apiGroups: ["gc.kube-zen.io"]
  resources:
    - garbagecollectionpolicies
    - garbagecollectionpolicies/status
  verbs:
    - get
    - list
    - watch
    - patch  # Still needed for status updates
```

### 2. Use Resource Names (Future Enhancement)

Kubernetes RBAC supports resource name restrictions, but this is not practical for GC controller since it needs to delete arbitrary resources based on policies.

### 3. Multi-Tenant Deployment

For multi-tenant environments:

**Option A: Separate Controller per Tenant**
- Deploy controller per namespace/tenant
- Use namespace-scoped Role instead of ClusterRole
- Limits controller to its namespace only

**Option B: Policy-Level Restrictions**
- Use label selectors in policies
- Use admission webhooks to enforce restrictions
- Controller still has broad permissions but policies limit scope

### 4. Audit and Monitor

Enable Kubernetes audit logging to track all operations:

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: RequestResponse
  resources:
  - group: "*"
    resources: ["*"]
  verbs: ["delete"]
```

## Security Best Practices

1. **Review Policies Regularly**: Audit GC policies to ensure they're appropriate
2. **Use Dry-Run**: Test policies in dry-run mode before enabling deletion
3. **Enable Audit Logging**: Track all deletions for security auditing
4. **Use Label Selectors**: Restrict policies to specific labels
5. **Monitor Metrics**: Watch `gc_resources_deleted_total` for unexpected deletions
6. **Limit Policy Creation**: Use RBAC to restrict who can create policies
7. **Network Policies**: Restrict network access to controller pods
8. **Pod Security**: Use restricted Pod Security Standards

## Verification

### Check Current Permissions

```bash
# View ClusterRole
kubectl get clusterrole gc-controller -o yaml

# Check what permissions the service account has
kubectl auth can-i --list --as=system:serviceaccount:gc-system:gc-controller

# Test specific permission
kubectl auth can-i delete pods --as=system:serviceaccount:gc-system:gc-controller --all-namespaces
```

### Verify Least-Privilege

```bash
# Check if controller can access resources it shouldn't
kubectl auth can-i create pods --as=system:serviceaccount:gc-system:gc-controller
# Should return: no (controller only needs delete, not create)

kubectl auth can-i update secrets --as=system:serviceaccount:gc-system:gc-controller
# Should return: no (controller only needs delete, not update)
```

## Troubleshooting

### Issue: Controller Cannot Delete Resources

**Symptoms:**
- Policies are created but no deletions occur
- Logs show "forbidden" errors

**Check:**
```bash
# Verify RBAC is applied
kubectl get clusterrolebinding gc-controller

# Check service account
kubectl get serviceaccount gc-controller -n gc-system

# Test permissions
kubectl auth can-i delete configmaps --as=system:serviceaccount:gc-system:gc-controller --all-namespaces
```

### Issue: Controller Cannot Update Policy Status

**Symptoms:**
- Policies show no status updates
- Logs show "forbidden" errors on status updates

**Check:**
```bash
# Verify status subresource permission
kubectl auth can-i patch garbagecollectionpolicies/status --as=system:serviceaccount:gc-system:gc-controller
```

## References

- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Least Privilege Principle](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [Audit Logging](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/)

