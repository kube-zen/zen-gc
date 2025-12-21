# Implementation Roadmap

This document outlines the implementation plan for the Generic Garbage Collection KEP.

**Important**: This is a **separate project** from zen-watcher. zen-gc will be developed independently as a standalone Kubernetes controller.

## Phase 1: KEP Preparation (Current)

**Goal**: Create a strong KEP candidate for Kubernetes

- [x] Draft KEP document
- [ ] Gather community feedback
- [ ] Refine API design based on feedback
- [ ] Create prototype/mockup
- [ ] Submit to Kubernetes Enhancement Proposals

## Phase 2: Prototype Implementation (If KEP Accepted)

**Goal**: Build a working prototype to validate the design

### 2.1 Core Controller

- [ ] GC controller skeleton
- [ ] Policy CRD definition
- [ ] Policy informer/watcher
- [ ] Resource informer factory
- [ ] Basic TTL evaluation logic

### 2.2 Basic Features

- [ ] Fixed TTL (`secondsAfterCreation`)
- [ ] Label selector support
- [ ] Field selector support
- [ ] Basic deletion logic
- [ ] Error handling

### 2.3 Testing

- [ ] Unit tests
- [ ] Integration tests
- [ ] E2E tests with kind/minikube

## Phase 3: Advanced Features

### 3.1 Dynamic TTL

- [ ] Field-based TTL (`fieldPath`)
- [ ] TTL mappings (severity-based, etc.)
- [ ] Relative TTL (relative to another timestamp)

### 3.2 Conditions

- [ ] Phase conditions
- [ ] Label conditions
- [ ] Annotation conditions
- [ ] Complex condition logic (AND/OR)

### 3.3 Rate Limiting & Batching

- [ ] Per-policy rate limiting
- [ ] Global rate limiting
- [ ] Batch processing
- [ ] Exponential backoff

### 3.4 Observability

- [ ] Prometheus metrics
- [ ] Kubernetes events
- [ ] Structured logging
- [ ] Status updates

## Phase 4: Production Readiness

### 4.1 Security

- [ ] RBAC implementation
- [ ] Admission webhooks
- [ ] Security audit
- [ ] Finalizer support

### 4.2 Performance

- [ ] Performance benchmarks
- [ ] Scale testing (10k+ resources)
- [ ] Memory optimization
- [ ] CPU optimization

### 4.3 Documentation

- [ ] User guide
- [ ] API reference
- [ ] Operator guide
- [ ] Migration guide

## Phase 5: Integration

### 5.1 Kubernetes Integration

- [ ] Integration with kube-controller-manager (or separate component)
- [ ] Default GC policies (optional)
- [ ] Policy priority system

### 5.2 Ecosystem

- [ ] Helm charts
- [ ] Operator support
- [ ] kubectl plugin
- [ ] Dashboard integration

## Timeline Estimate

- **Phase 1**: 2-4 weeks (KEP review process)
- **Phase 2**: 4-6 weeks (prototype)
- **Phase 3**: 6-8 weeks (advanced features)
- **Phase 4**: 4-6 weeks (production readiness)
- **Phase 5**: 4-8 weeks (integration)

**Total**: ~20-32 weeks (5-8 months) from KEP acceptance to stable release

## Success Criteria

1. **KEP Acceptance**: KEP approved by SIG-apps
2. **Prototype Working**: Basic GC functionality working in kind/minikube
3. **Community Adoption**: Used by at least 3 production clusters
4. **Performance Validated**: Handles 10k+ resources efficiently
5. **API Stable**: v1 API released and stable

