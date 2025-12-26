# Version Compatibility Matrix

This document provides version compatibility information for zen-gc, including supported Kubernetes versions, CRD version migration guides, and migration from other TTL solutions.

## Table of Contents

- [Supported Kubernetes Versions](#supported-kubernetes-versions)
- [CRD Version Migration Guide](#crd-version-migration-guide)
- [Migration from Other Solutions](#migration-from-other-solutions)

---

## Supported Kubernetes Versions

### Compatibility Matrix

| zen-gc Version | Kubernetes 1.23 | Kubernetes 1.24 | Kubernetes 1.25 | Kubernetes 1.26 | Kubernetes 1.27 | Kubernetes 1.28 | Kubernetes 1.29+ |
|----------------|-----------------|-----------------|-----------------|-----------------|-----------------|-----------------|------------------|
| 0.1.x          | ✅              | ✅              | ✅              | ✅              | ✅              | ✅              | ✅               |
| 0.2.x          | ❌              | ✅              | ✅              | ✅              | ✅              | ✅              | ✅               |
| 1.0.x          | ❌              | ❌              | ✅              | ✅              | ✅              | ✅              | ✅               |

**Legend:**
- ✅ Fully supported
- ⚠️ Supported with limitations
- ❌ Not supported

### Minimum Requirements

- **Kubernetes**: 1.23+ (required for TTL-after-finished feature)
- **CRD API**: v1 (apiextensions.k8s.io/v1)
- **RBAC**: v1 (rbac.authorization.k8s.io/v1)

### Tested Versions

The following Kubernetes versions are regularly tested:

- 1.23.x (EKS, GKE, AKS)
- 1.24.x (EKS, GKE, AKS)
- 1.25.x (EKS, GKE, AKS)
- 1.26.x (EKS, GKE, AKS)
- 1.27.x (EKS, GKE, AKS)
- 1.28.x (EKS, GKE, AKS)
- 1.29.x (EKS, GKE, AKS)

### Version Support Policy

- **Current Version**: Fully supported
- **Previous Minor Version**: Supported with bug fixes
- **Older Versions**: Best effort support

### Deprecated Features

| Feature | Deprecated In | Removed In | Replacement |
|---------|--------------|------------|-------------|
| - | - | - | - |

---

## CRD Version Migration Guide

### CRD Version History

| Version | Status | Kubernetes Version | Notes |
|---------|--------|-------------------|-------|
| v1alpha1 | ✅ Current | 1.23+ | Initial release, may have breaking changes |
| v1beta1 | 🔜 Planned | TBD | API stabilized, no breaking changes |
| v1 | 🔜 Planned | TBD | Stable API, long-term support |

### Migration Path: v1alpha1 → v1beta1

**When**: After API stabilization and community feedback

**Breaking Changes**: None expected

**Migration Steps**:

1. **Backup Current Policies**:
   ```bash
   kubectl get garbagecollectionpolicies --all-namespaces -o yaml > policies-backup.yaml
   ```

2. **Update CRD**:
   ```bash
   kubectl apply -f deploy/crds/gc.kube-zen.io_garbagecollectionpolicies-v1beta1.yaml
   ```

3. **Verify Policies**:
   ```bash
   kubectl get garbagecollectionpolicies --all-namespaces
   ```

4. **Update Policy Definitions** (if needed):
   ```bash
   # Update apiVersion in policy YAML files
   sed -i 's/apiVersion: gc.kube-zen.io\/v1alpha1/apiVersion: gc.kube-zen.io\/v1beta1/g' policies/*.yaml
   kubectl apply -f policies/
   ```

5. **Test Functionality**:
   ```bash
   # Verify controller still works
   kubectl logs -n gc-system deployment/gc-controller
   ```

### Migration Path: v1beta1 → v1

**When**: After production validation and API stability guarantee

**Breaking Changes**: None (guaranteed)

**Migration Steps**:

Same as v1alpha1 → v1beta1, but with v1 CRD and API version.

### Rollback Procedure

If migration causes issues:

```bash
# 1. Scale down controller
kubectl scale deployment gc-controller -n gc-system --replicas=0

# 2. Restore previous CRD version
kubectl apply -f deploy/crds/gc.kube-zen.io_garbagecollectionpolicies-v1alpha1.yaml

# 3. Restore policies from backup
kubectl apply -f policies-backup.yaml

# 4. Scale controller back up
kubectl scale deployment gc-controller -n gc-system --replicas=2
```

---

## Migration from Other Solutions

### Migration from k8s-ttl-controller

#### Overview

k8s-ttl-controller uses annotations for TTL configuration, while zen-gc uses declarative CRDs.

#### Key Differences

| Feature | k8s-ttl-controller | zen-gc |
|---------|-------------------|--------|
| Configuration | Annotations | CRDs |
| Policy Management | Per-resource | Centralized |
| Selectors | Limited | Full Kubernetes selectors |
| Conditions | No | Yes |
| Rate Limiting | No | Yes |
| Metrics | Limited | Comprehensive |

#### Migration Steps

1. **Audit Current Usage**:
   ```bash
   # Find all resources with TTL annotations
   kubectl get all --all-namespaces -o json | \
     jq '.items[] | select(.metadata.annotations["ttl-controller.k8s.io/ttl"]) | 
         {namespace: .metadata.namespace, name: .metadata.name, 
          kind: .kind, ttl: .metadata.annotations["ttl-controller.k8s.io/ttl"]}'
   ```

2. **Create Equivalent Policies**:
   ```yaml
   # Example: Convert annotation-based TTL to policy
   # Before (k8s-ttl-controller):
   # metadata:
   #   annotations:
   #     ttl-controller.k8s.io/ttl: "3600"
   
   # After (zen-gc):
   apiVersion: gc.kube-zen.io/v1alpha1
   kind: GarbageCollectionPolicy
   metadata:
     name: configmap-ttl-policy
   spec:
     targetResource:
       apiVersion: v1
       kind: ConfigMap
       labelSelector:
         matchLabels:
           ttl-enabled: "true"
     ttl:
       secondsAfterCreation: 3600
   ```

3. **Migrate Resources**:
   ```bash
   # Add labels to resources that should be managed by zen-gc
   kubectl label configmap <name> ttl-enabled=true
   
   # Remove old annotations (optional, after verification)
   kubectl annotate configmap <name> ttl-controller.k8s.io/ttl-
   ```

4. **Test and Verify**:
   ```bash
   # Enable dry-run mode initially
   # Verify policies match expected behavior
   # Disable k8s-ttl-controller
   # Enable zen-gc policies
   ```

5. **Remove k8s-ttl-controller**:
   ```bash
   kubectl delete deployment k8s-ttl-controller -n <namespace>
   ```

#### Migration Script

```bash
#!/bin/bash
# migrate-from-k8s-ttl-controller.sh

# Find resources with TTL annotations
RESOURCES=$(kubectl get all --all-namespaces -o json | \
  jq -r '.items[] | select(.metadata.annotations["ttl-controller.k8s.io/ttl"]) | 
         "\(.metadata.namespace)|\(.metadata.name)|\(.kind)|\(.metadata.annotations["ttl-controller.k8s.io/ttl"])"')

# Generate policies for each resource type
echo "$RESOURCES" | while IFS='|' read -r ns name kind ttl; do
  # Create policy YAML
  cat <<EOF
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: ${kind}-ttl-migrated-$(echo $name | tr '[:upper:]' '[:lower:]')
  namespace: $ns
spec:
  targetResource:
    apiVersion: v1
    kind: $kind
    namespace: $ns
    labelSelector:
      matchLabels:
        migrated-from-k8s-ttl-controller: "true"
  ttl:
    secondsAfterCreation: $ttl
EOF
done
```

---

### Migration from Kyverno Cleanup Policies

#### Overview

Kyverno cleanup policies are similar to zen-gc but require Kyverno installation.

#### Key Differences

| Feature | Kyverno Cleanup | zen-gc |
|---------|----------------|--------|
| Dependencies | Requires Kyverno | Zero dependencies |
| Policy Language | Kyverno policy | Kubernetes CRD |
| Conditions | Rego-based | Kubernetes-native |
| Performance | Policy engine overhead | Direct controller |

#### Migration Steps

1. **Export Kyverno Policies**:
   ```bash
   kubectl get clusterpolicies -o yaml > kyverno-policies.yaml
   ```

2. **Convert to zen-gc Policies**:
   ```yaml
   # Before (Kyverno):
   apiVersion: kyverno.io/v1
   kind: ClusterPolicy
   metadata:
     name: cleanup-configmaps
   spec:
     rules:
     - name: cleanup-old-configmaps
       match:
         resources:
           kinds:
           - ConfigMap
       exclude:
         resources:
           namespaces:
           - kube-system
       validate:
         message: "ConfigMap will be deleted after 1 hour"
         pattern:
           metadata:
             annotations:
               cleanup.kyverno.io/ttl: "1h"
   
   # After (zen-gc):
   apiVersion: gc.kube-zen.io/v1alpha1
   kind: GarbageCollectionPolicy
   metadata:
     name: cleanup-configmaps
   spec:
     targetResource:
       apiVersion: v1
       kind: ConfigMap
       labelSelector:
         matchLabels:
           cleanup-enabled: "true"
     ttl:
       secondsAfterCreation: 3600
     conditions:
       hasLabels:
       - key: cleanup-enabled
         value: "true"
   ```

3. **Migrate Resources**:
   ```bash
   # Add labels to resources
   kubectl label configmap <name> cleanup-enabled=true
   ```

4. **Test and Remove Kyverno**:
   ```bash
   # Test zen-gc policies
   # Remove Kyverno cleanup policies
   kubectl delete clusterpolicy cleanup-configmaps
   ```

---

### Migration from Custom Controllers

#### Overview

Many organizations have built custom GC controllers. Migration to zen-gc provides standardization.

#### Migration Strategy

1. **Document Current Behavior**: Understand what your custom controller does
2. **Map to zen-gc Features**: Identify equivalent zen-gc policies
3. **Create Policies**: Convert custom logic to zen-gc policies
4. **Test Side-by-Side**: Run both controllers temporarily
5. **Migrate Gradually**: Migrate one resource type at a time
6. **Remove Custom Controller**: After full migration

#### Example Migration

```yaml
# Custom controller logic (pseudo-code):
# if resource.age > 7 days AND resource.status == "completed":
#   delete resource

# zen-gc equivalent:
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: completed-resource-cleanup
spec:
  targetResource:
    apiVersion: v1
    kind: Pod
  ttl:
    secondsAfterCreation: 604800  # 7 days
  conditions:
    phase: ["Succeeded", "Failed"]
```

---

## Compatibility Testing

### Testing Matrix

We test compatibility with:

- **Cloud Providers**: AWS EKS, GKE, Azure AKS
- **Distributions**: k3s, k0s, Rancher
- **CNI Plugins**: Calico, Cilium, Flannel
- **Storage**: Local, NFS, CSI drivers

### Reporting Compatibility Issues

If you encounter compatibility issues:

1. Open a GitHub issue
2. Include:
   - Kubernetes version
   - Cloud provider/distribution
   - Error messages
   - Steps to reproduce

---

## Version Support Timeline

### Current Support

- **v1alpha1**: Supported until v1beta1 release + 6 months
- **v1beta1**: Will be supported until v1 release + 12 months
- **v1**: Long-term support (LTS)

### End of Life Policy

- **Deprecation Notice**: 6 months before removal
- **Security Fixes**: Provided for 12 months after deprecation
- **Bug Fixes**: Provided for 6 months after deprecation

---

## Summary

- **Kubernetes**: 1.23+ required, 1.25+ recommended
- **CRD Migration**: Straightforward, no data loss
- **Solution Migration**: Well-documented paths from common alternatives
- **Testing**: Regular compatibility testing across platforms

For specific migration help, open a GitHub issue or contact maintainers.

