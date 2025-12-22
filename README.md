# zen-gc: Generic Garbage Collection for Kubernetes

**Automatically clean up any Kubernetes resource based on time-to-live policies**

## Overview

`zen-gc` is a Kubernetes controller that provides declarative, automatic garbage collection for any Kubernetes resource. Define cleanup policies once, and let zen-gc handle the rest—no custom controllers or manual cleanup scripts needed.

**Why zen-gc?**

Kubernetes only provides built-in TTL support for Jobs. For everything else (ConfigMaps, Secrets, Pods, CRDs, etc.), you're on your own. zen-gc fills this gap with a simple, Kubernetes-native solution.

## Key Benefits

- 🎯 **Works with Everything**: Clean up ConfigMaps, Secrets, Pods, Jobs, CRDs, or any Kubernetes resource
- ⚡ **Zero Configuration**: Define policies as Kubernetes resources—no external tools or complex setup
- 🔒 **Production-Ready**: Built-in rate limiting, metrics, and observability out of the box
- 🎨 **Flexible**: Support for complex conditions, selectors, and custom TTL calculations
- 🚀 **Easy to Use**: Simple YAML policies—no coding required
- 📊 **Observable**: Prometheus metrics and Kubernetes events for full visibility

## Powerful TTL Options

zen-gc's flexible TTL system is what makes it shine. Choose from four powerful options:

### 1. Fixed TTL
Simple time-based cleanup—delete resources after a fixed period:
```yaml
ttl:
  secondsAfterCreation: 604800  # 7 days
```

### 2. Field-Based TTL
Extract TTL directly from resource fields—let resources define their own lifetime:
```yaml
ttl:
  fieldPath: "spec.ttlSeconds"  # Resource controls its own TTL
```

### 3. Mapped TTL
Different TTLs based on resource values—perfect for severity-based retention:
```yaml
ttl:
  fieldPath: "spec.severity"
  mappings:
    CRITICAL: 1814400  # 3 weeks
    HIGH: 1209600      # 2 weeks
    MEDIUM: 604800     # 1 week
    LOW: 259200        # 3 days
  default: 604800
```

### 4. Relative TTL
TTL relative to another timestamp—clean up after last activity:
```yaml
ttl:
  relativeTo: "status.lastProcessedAt"
  secondsAfter: 86400  # 1 day after last activity
```

**This flexibility means zen-gc adapts to your needs, not the other way around.**

## Quick Start

Install zen-gc and create your first cleanup policy:

```bash
# Install zen-gc
kubectl apply -f deploy/crds/gc.kube-zen.io_garbagecollectionpolicies.yaml
kubectl apply -f deploy/manifests/

# Create a cleanup policy
kubectl apply -f examples/temp-configmap-cleanup.yaml
```

**Example Policy**: Clean up temporary ConfigMaps after 1 hour

```yaml
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: cleanup-temp-configmaps
spec:
  targetResource:
    apiVersion: v1
    kind: ConfigMap
    labelSelector:
      matchLabels:
        temporary: "true"
  ttl:
    secondsAfterCreation: 3600  # 1 hour
  behavior:
    maxDeletionsPerSecond: 10
```

## Use Cases

- **Clean up completed Jobs**: Automatically remove finished Jobs after 24 hours
- **Remove old ConfigMaps/Secrets**: Delete temporary resources created during CI/CD
- **Evicted Pod cleanup**: Quickly remove pods evicted due to resource pressure
- **Orphaned ReplicaSet cleanup**: Remove ReplicaSets not owned by Deployments
- **PVC cleanup**: Delete Released PersistentVolumeClaims
- **Test resource cleanup**: Automatically remove test Pods, Services after completion
- **Multi-tenant isolation**: Per-tenant cleanup policies for namespace-scoped resources

## Documentation

- **[KEP Document](docs/KEP_GENERIC_GARBAGE_COLLECTION.md)**: Complete Kubernetes Enhancement Proposal
- **[API Reference](docs/API_REFERENCE.md)**: Complete API documentation
- **[User Guide](docs/USER_GUIDE.md)**: How to create and use GC policies
- **[Operator Guide](docs/OPERATOR_GUIDE.md)**: Installation, configuration, and maintenance
- **[Metrics Documentation](docs/METRICS.md)**: Prometheus metrics reference
- **[Security Documentation](docs/SECURITY.md)**: Security best practices and guidelines
- **[Disaster Recovery](docs/DISASTER_RECOVERY.md)**: Recovery procedures and backup strategies
- **[Version Compatibility](docs/VERSION_COMPATIBILITY.md)**: Kubernetes versions and migration guides
- **[Architecture](docs/ARCHITECTURE.md)**: System architecture and design diagrams
- **[Examples](examples/)**: Example GC policies
- **[Contributing](CONTRIBUTING.md)**: Development guidelines and contribution process
- **[Governance](GOVERNANCE.md)**: Project governance model
- **[Maintainers](MAINTAINERS.md)**: List of project maintainers
- **[Releasing](RELEASING.md)**: Release process documentation
- **[Adopters](ADOPTERS.md)**: Organizations using zen-gc

## Features

- ✅ **Generic Resource Support**: Works with any Kubernetes resource (CRDs, core resources)
- ✅ **Four TTL Modes**: Fixed, field-based, mapped, or relative TTL—choose what fits your use case
- ✅ **Powerful Selectors**: Label selectors, field selectors, and namespace scoping
- ✅ **Condition Matching**: Match resources by phase, labels, annotations, or custom field conditions
- ✅ **Rate Limiting**: Configurable deletion rate per policy to prevent API server overload
- ✅ **Dry-Run Mode**: Test policies safely before enabling actual deletion
- ✅ **Production Features**: Prometheus metrics, Kubernetes events, leader election for HA
- ✅ **Well Tested**: >80% test coverage with comprehensive unit and integration tests

## Status

zen-gc is **production-ready** and actively maintained. The project is open source and welcomes contributions.

**Note**: This project may eventually be proposed as a Kubernetes Enhancement Proposal (KEP) based on community adoption and feedback, but the primary focus is on providing a useful, production-ready solution for Kubernetes operators.

## Contributing

This is an early-stage proposal. Feedback and contributions are welcome!

## References

- [Kubernetes TTL-after-finished](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/)
- [Kubernetes Enhancement Proposals](https://github.com/kubernetes/enhancements)

