# Security Documentation

This document provides comprehensive security guidance for deploying and operating zen-gc.

## Table of Contents

- [Pod Security Standards](#pod-security-standards)
- [Network Policies](#network-policies)
- [Audit Logging](#audit-logging)
- [RBAC Scoping Best Practices](#rbac-scoping-best-practices)
- [Security Checklist](#security-checklist)

---

## Pod Security Standards

zen-gc follows Kubernetes Pod Security Standards to ensure secure deployments.

### Recommended Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534  # nobody
  fsGroup: 65534
  seccompProfile:
    type: RuntimeDefault
  capabilities:
    drop:
      - ALL
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
```

### Pod Security Standards Compliance

zen-gc is compliant with **Restricted** Pod Security Standard:

- ✅ Runs as non-root user
- ✅ No privileged containers
- ✅ No host network access
- ✅ No host PID/IPC namespaces
- ✅ Read-only root filesystem
- ✅ No capabilities granted
- ✅ Seccomp profile enforced

### Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gc-controller
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: gc-controller
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
              - ALL
          readOnlyRootFilesystem: true
```

---

## Network Policies

Network policies restrict network access to the GC controller, following the principle of least privilege.

### Recommended Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: gc-controller-netpol
  namespace: gc-system
spec:
  podSelector:
    matchLabels:
      app: gc-controller
  policyTypes:
  - Ingress
  - Egress
  
  # Allow ingress from Prometheus (metrics scraping)
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 8080
  
  # Allow egress to Kubernetes API server
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: TCP
      port: 443
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
```

### Network Policy Best Practices

1. **Restrict Ingress**: Only allow necessary traffic (metrics scraping)
2. **Limit Egress**: Only allow access to Kubernetes API server and DNS
3. **Namespace Isolation**: Use namespace selectors to restrict cross-namespace communication
4. **Port Specificity**: Specify exact ports rather than allowing all ports

---

## Audit Logging

Enable audit logging to track all GC operations for security and compliance.

### Kubernetes Audit Policy

Create an audit policy to log GC controller operations:

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
# Log all delete operations by GC controller
- level: RequestResponse
  users: ["system:serviceaccount:gc-system:gc-controller"]
  verbs: ["delete"]
  resources:
  - group: "*"
    resources: ["*"]

# Log all policy modifications
- level: RequestResponse
  users: ["*"]
  verbs: ["create", "update", "patch", "delete"]
  resources:
  - group: "gc.k8s.io"
    resources: ["garbagecollectionpolicies"]
```

### Controller Logging

The GC controller logs all deletion operations:

```go
// Example log output
klog.InfoS("Deleting resource",
    "policy", policyName,
    "resource", resourceName,
    "namespace", namespace,
    "reason", "ttl_expired",
    "age", age)
```

### Log Aggregation

Recommended log aggregation setup:

1. **Fluentd/Fluent Bit**: Collect logs from controller pods
2. **Loki**: Store and query logs
3. **Grafana**: Visualize log data
4. **Alerting**: Alert on suspicious deletion patterns

### Audit Log Retention

- **Minimum**: 30 days for production
- **Recommended**: 90 days for compliance
- **Compliance**: Follow your organization's retention policies

---

## RBAC Scoping Best Practices

### Principle of Least Privilege

The GC controller requires broad permissions to delete resources. Follow these best practices:

#### 1. Namespace Scoping

For multi-tenant environments, consider namespace-scoped deployments:

```yaml
# ClusterRole with namespace restriction
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gc-controller-namespaced
rules:
- apiGroups: ["gc.k8s.io"]
  resources: ["garbagecollectionpolicies"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
```

#### 2. Resource Filtering

Use label selectors to limit which resources can be deleted:

```yaml
apiVersion: gc.k8s.io/v1alpha1
kind: GarbageCollectionPolicy
spec:
  targetResource:
    labelSelector:
      matchLabels:
        # Only delete resources with this label
        gc-allowed: "true"
```

#### 3. Multi-Tenant Considerations

**Option A: Separate Controller per Tenant**

Deploy separate GC controllers per tenant namespace:

```yaml
# Tenant-specific ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gc-controller
  namespace: tenant-a
---
# Tenant-specific RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: gc-controller
  namespace: tenant-a
rules:
- apiGroups: ["gc.k8s.io"]
  resources: ["garbagecollectionpolicies"]
  verbs: ["*"]
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["get", "list", "watch", "delete"]
```

**Option B: Policy-Level Restrictions**

Use policy conditions to prevent cross-tenant deletions:

```yaml
apiVersion: gc.k8s.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: tenant-a-policy
  namespace: tenant-a
spec:
  targetResource:
    namespace: tenant-a  # Explicitly restrict to tenant namespace
    labelSelector:
      matchLabels:
        tenant: tenant-a
```

#### 4. Resource Quota Considerations

Be aware that GC operations count against resource quotas:

- Deletion operations consume API server quota
- Monitor API server rate limits
- Use `behavior.maxDeletionsPerSecond` to control rate

---

## Security Checklist

Use this checklist when deploying zen-gc:

### Pre-Deployment

- [ ] Review and understand RBAC permissions
- [ ] Verify Pod Security Standards compliance
- [ ] Configure network policies
- [ ] Enable audit logging
- [ ] Review policy definitions for dangerous patterns
- [ ] Test policies in non-production environment
- [ ] Set up monitoring and alerting

### Policy Definition

- [ ] Use label selectors to limit scope
- [ ] Specify explicit namespaces (avoid wildcards)
- [ ] Set reasonable TTL values
- [ ] Enable dry-run mode for testing
- [ ] Use conditions to prevent accidental deletions
- [ ] Document policy purpose and scope
- [ ] Review policy with security team

### Runtime Security

- [ ] Monitor deletion rates
- [ ] Alert on abnormal deletion patterns
- [ ] Review audit logs regularly
- [ ] Keep controller updated
- [ ] Rotate service account tokens
- [ ] Monitor for privilege escalation attempts
- [ ] Review RBAC permissions periodically

### Multi-Tenant Security

- [ ] Isolate tenants using namespaces
- [ ] Use namespace-scoped policies
- [ ] Implement tenant-specific RBAC
- [ ] Monitor cross-tenant access attempts
- [ ] Use resource quotas to limit impact
- [ ] Document tenant isolation strategy

### Incident Response

- [ ] Have emergency stop procedure documented
- [ ] Know how to disable policies quickly
- [ ] Have rollback plan ready
- [ ] Document recovery procedures
- [ ] Test disaster recovery procedures

---

## Security Best Practices Summary

1. **Least Privilege**: Grant only necessary permissions
2. **Defense in Depth**: Use multiple security layers (RBAC, network policies, pod security)
3. **Monitoring**: Monitor all deletion operations
4. **Audit**: Log all security-relevant operations
5. **Isolation**: Isolate tenants and environments
6. **Validation**: Validate policies before deployment
7. **Testing**: Test security configurations regularly
8. **Documentation**: Document security decisions and procedures

---

## Reporting Security Issues

If you discover a security vulnerability, please report it to: security@kube-zen.io

**Do not** open a public GitHub issue for security vulnerabilities.

See [SECURITY.md](../SECURITY.md) in the root directory for more information.

