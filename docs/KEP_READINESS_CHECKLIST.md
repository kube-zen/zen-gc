# KEP Readiness Checklist - Making zen-gc Production-Grade

This document tracks progress toward KEP approval and production readiness.

## 🎯 Goal: 9.5/10 Quality Score for KEP Approval

---

## Phase 1: Code Quality & Cleanup (CRITICAL - Do First)

### 1.1 Fix Code Issues
- [ ] **Clean up duplicate code** in `pkg/controller/gc_controller.go` (has 2500+ lines with duplicates)
- [ ] **Fix compilation errors** - ensure `go build` works
- [ ] **Fix imports** - remove unused imports, add missing ones
- [ ] **Standardize on controller-runtime** - use proper manager pattern
- [ ] **Fix rate limiter** - remove duplicate implementation, use `golang.org/x/time/rate`

### 1.2 Code Standards
- [ ] **Follow Kubernetes code style** - run `gofmt`, `golint`, `govet`
- [ ] **Add proper error wrapping** - use `fmt.Errorf` with `%w`
- [ ] **Add context propagation** - all API calls should use context
- [ ] **Remove TODOs** - implement or document deferred features

---

## Phase 2: Alpha Requirements (Must Have for KEP)

### 2.1 Core Functionality ✅ (Mostly Done)
- [x] GC controller implementation
- [x] Basic `GarbageCollectionPolicy` CRD
- [x] Fixed TTL support (`secondsAfterCreation`)
- [x] Label/field selector support
- [ ] **Basic metrics** ⚠️ (Missing - CRITICAL)
- [ ] **Unit tests** ⚠️ (Missing - CRITICAL)
- [ ] **Documentation** ⚠️ (Partial - needs improvement)

### 2.2 Metrics (CRITICAL - Missing)
- [ ] **Prometheus metrics**:
  - `gc_policies_total` (gauge by phase)
  - `gc_resources_matched_total` (counter by policy, resource)
  - `gc_resources_deleted_total` (counter by policy, resource, reason)
  - `gc_deletion_duration_seconds` (histogram)
  - `gc_errors_total` (counter by policy, error_type)
- [ ] **Metrics endpoint** - `/metrics` on port 8080
- [ ] **Metrics documentation** - document all metrics

### 2.3 Testing (CRITICAL - Missing)
- [ ] **Unit tests** - at least 60% coverage:
  - TTL calculation logic
  - Selector matching
  - Condition evaluation
  - Rate limiting
- [ ] **Integration tests** - test with fake client
- [ ] **E2E tests** - test with kind/minikube
- [ ] **Test fixtures** - sample policies and resources

### 2.4 Documentation
- [ ] **API documentation** - godoc comments on all public APIs
- [ ] **User guide** - how to create and use policies
- [ ] **Operator guide** - deployment and configuration
- [ ] **Troubleshooting guide** - common issues and solutions

---

## Phase 3: Production Readiness

### 3.1 Deployment Manifests
- [ ] **RBAC manifests**:
  - ServiceAccount
  - ClusterRole (with proper permissions)
  - ClusterRoleBinding
- [ ] **Deployment manifest**:
  - Deployment with proper resource limits
  - Service for metrics
  - ConfigMap for configuration
- [ ] **Helm chart** (optional but recommended):
  - Values file
  - Templates
  - README

### 3.2 Observability
- [ ] **Structured logging** - use `klog` with proper levels
- [ ] **Kubernetes events** - emit events for policy changes
- [ ] **Health checks** - `/healthz` and `/readyz` endpoints
- [ ] **Leader election** - for HA deployments

### 3.3 Security
- [ ] **RBAC review** - minimal required permissions
- [ ] **Security context** - non-root, read-only filesystem
- [ ] **Admission webhook** (optional) - validate policies
- [ ] **Finalizer support** - graceful cleanup

### 3.4 Performance
- [ ] **Benchmarks** - measure deletion throughput
- [ ] **Memory profiling** - ensure no leaks
- [ ] **CPU profiling** - optimize hot paths
- [ ] **Scale testing** - test with 10k+ resources

---

## Phase 4: KEP Submission Requirements

### 4.1 KEP Document
- [ ] **Complete KEP** - all sections filled
- [ ] **API design** - finalized and documented
- [ ] **Migration guide** - from custom controllers
- [ ] **Examples** - comprehensive use cases

### 4.2 Prototype Quality
- [ ] **Working prototype** - runs in kind/minikube
- [ ] **Demo video** - 5-minute walkthrough
- [ ] **Test results** - performance benchmarks
- [ ] **Known limitations** - documented

### 4.3 Community Engagement
- [ ] **GitHub issues** - template for bug reports
- [ ] **Contributing guide** - CONTRIBUTING.md
- [ ] **Code of conduct** - if needed
- [ ] **Release notes** - CHANGELOG.md

---

## Phase 5: Advanced Features (Beta Requirements)

### 5.1 Dynamic TTL ✅ (Implemented)
- [x] Field-based TTL (`fieldPath`)
- [x] TTL mappings (severity-based)
- [x] Relative TTL

### 5.2 Conditions ✅ (Implemented)
- [x] Phase conditions
- [x] Label conditions
- [x] Annotation conditions
- [x] Field conditions (AND logic)

### 5.3 Rate Limiting ✅ (Implemented)
- [x] Per-policy rate limiting
- [x] Batch processing
- [ ] **Exponential backoff** ⚠️ (Missing)

### 5.4 Dry Run ✅ (Implemented)
- [x] Dry-run mode

---

## Priority Order for Implementation

### 🔴 CRITICAL (Do First - Blocks KEP)
1. Clean up duplicate code in controller
2. Add Prometheus metrics
3. Add unit tests (minimum 60% coverage)
4. Fix compilation errors
5. Add RBAC manifests

### 🟡 HIGH (Needed for Alpha)
6. Add integration tests
7. Add E2E tests
8. Improve documentation
9. Add deployment manifests
10. Add health checks

### 🟢 MEDIUM (Nice to Have)
11. Add Helm chart
12. Add admission webhook
13. Add exponential backoff
14. Performance benchmarks
15. Demo video

---

## Success Metrics

- ✅ Code compiles without errors
- ✅ All tests pass
- ✅ Code coverage > 60%
- ✅ No critical security issues
- ✅ Works in kind/minikube
- ✅ Metrics exposed and documented
- ✅ Documentation complete
- ✅ KEP document ready for submission

---

## Estimated Timeline

- **Week 1-2**: Code cleanup + metrics + basic tests
- **Week 3**: Integration tests + E2E tests
- **Week 4**: Documentation + deployment manifests
- **Week 5**: Final polish + KEP submission prep

**Total: ~5 weeks to KEP-ready state**

