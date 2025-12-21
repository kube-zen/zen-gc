# zen-gc: Generic Garbage Collection for Kubernetes

**Status**: KEP Draft  
**Purpose**: Design and propose a generic garbage collection mechanism for Kubernetes

## Overview

This repository contains the design and proposal for a generic garbage collection (GC) controller for Kubernetes. The goal is to provide a Kubernetes-native, declarative way to automatically clean up resources based on time-to-live (TTL) policies.

## Problem Statement

Kubernetes currently only provides built-in TTL support for Jobs (`spec.ttlSecondsAfterFinished`). For all other resources (CRDs, ConfigMaps, Secrets, Pods, etc.), operators must:

1. Build custom controllers
2. Use external tools (k8s-ttl-controller, Kyverno)
3. Manually manage cleanup via CronJobs

This creates operational overhead and inconsistency across the ecosystem.

## Proposed Solution

A new `GarbageCollectionPolicy` CRD that enables declarative, time-based cleanup of any Kubernetes resource:

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
    secondsAfterCreation: 604800  # 7 days
  behavior:
    maxDeletionsPerSecond: 10
```

## Key Features

- ✅ **Generic**: Works with any Kubernetes resource (CRDs, core resources)
- ✅ **Declarative**: Policies defined as Kubernetes CRDs
- ✅ **Kubernetes-Native**: Uses spec fields (like Jobs), not annotations
- ✅ **Zero Dependencies**: Built into Kubernetes, no external controllers
- ✅ **Production-Ready**: Rate limiting, metrics, observability

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

## Status

**Current Status**: KEP Draft - Implementation in progress

The goal is to:

1. ✅ **Design**: Create a strong KEP candidate for Kubernetes
2. ✅ **PoC**: Implement a working prototype with >80% test coverage
3. ⏳ **Open Source**: Release as OSS for community testing
4. ⏳ **Validate**: Test in real-world scenarios
5. ⏳ **Submit**: Submit KEP to Kubernetes Enhancement Proposals after validation

### Current Implementation Status

- ✅ GC controller implementation
- ✅ `GarbageCollectionPolicy` CRD
- ✅ Fixed and dynamic TTL support
- ✅ Selectors (label, field, namespace)
- ✅ Conditions (phase, labels, annotations, field conditions)
- ✅ Rate limiting and batching
- ✅ Dry-run mode
- ✅ Prometheus metrics
- ✅ Unit tests (>80% coverage)
- ✅ Documentation (API reference, user guide, operator guide)

## Contributing

This is an early-stage proposal. Feedback and contributions are welcome!

## References

- [Kubernetes TTL-after-finished](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/)
- [Kubernetes Enhancement Proposals](https://github.com/kubernetes/enhancements)

