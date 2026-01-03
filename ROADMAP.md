# zen-gc Roadmap

This document outlines the planned features and improvements for zen-gc. The roadmap is organized by theme and prioritized for upcoming releases.

**Last Updated**: 2025-12-26

---

## 🎯 Current Status

zen-gc is currently in **0.0.1-alpha** with core functionality complete:
- ✅ Generic garbage collection for any Kubernetes resource
- ✅ Multiple TTL modes (fixed, dynamic, field-based, mapped, relative)
- ✅ Per-policy rate limiting and batching
- ✅ Leader election for HA deployments
- ✅ Comprehensive metrics and observability
- ✅ Validating and mutating admission webhooks
- ✅ Structured logging with correlation IDs
- ✅ Parallel policy evaluation
- ✅ Graceful shutdown and error handling
- ✅ RESTMapper integration for reliable GVR resolution
- ✅ PolicyEvaluationService for improved testability
- ✅ Refactored architecture with reduced complexity

---

## 📅 Planned Releases

### 0.0.2-alpha - Enhanced Observability & Health Checks

**Target**: Q1 2026

**Focus**: Improve operational readiness and observability

#### Health Check Enhancements
- Enhanced readiness probe that verifies informer sync status
- Liveness probe that verifies controller is actively processing policies
- Startup probe for slow-starting controllers in large clusters
- Health check metrics and dashboards

**Benefits**:
- Better Kubernetes deployment reliability
- Faster detection of controller issues
- Improved operator confidence

**Related Documentation**:
- `docs/OPERATOR_GUIDE.md` - Current health check configuration
- `docs/ARCHITECTURE.md` - Controller lifecycle

---

### 0.1.0 - Advanced Policy Features

**Target**: Q2 2026

**Focus**: More powerful and flexible policy capabilities

#### Policy Conditions Enhancement
- Regex support for field matching
- Date/time comparisons (e.g., "delete if older than X days")
- Numeric comparisons (e.g., "delete if count > threshold")
- Complex boolean logic (AND/OR/NOT operators)
- Nested condition groups

**Benefits**:
- More expressive policy conditions
- Support for complex deletion criteria
- Better integration with business logic

**Related Documentation**:
- `docs/API_REFERENCE.md` - Current condition syntax
- `examples/README.md` - Condition examples

#### Dry-Run Mode Enhancement
- Dry-run metrics (what would be deleted, impact analysis)
- Dry-run report generation (JSON/YAML/HTML formats)
- Dry-run API endpoint for programmatic access
- Dry-run dashboard/UI integration

**Benefits**:
- Better visibility into policy impact before activation
- Safer policy testing and validation
- Improved change management workflows

**Related Documentation**:
- `docs/USER_GUIDE.md` - Current dry-run usage
- `docs/API_REFERENCE.md` - Dry-run API details

---

### 0.2.0 - Policy Management & Templates

**Target**: Q3 2026

**Focus**: Simplify policy creation and management

#### Policy Templates
- Policy template CRD for reusable policy definitions
- Policy composition/inheritance (extend base templates)
- Policy presets for common scenarios (e.g., "cleanup-old-pods", "delete-completed-jobs")
- Template parameterization and variable substitution

**Benefits**:
- Faster policy creation
- Consistent policy patterns across teams
- Reduced configuration errors
- Easier policy maintenance

**Related Documentation**:
- `examples/README.md` - Example policies
- `docs/API_REFERENCE.md` - Policy structure

---

### 0.3.0 - Performance & Production Hardening

**Target**: Q4 2026

**Focus**: Performance optimizations and production readiness

#### Performance Optimizations

**Shared Informer Architecture (P1.1 - Critical for Scale)**
- Refactor from per-policy informers to shared informers per (GVR, namespace) combination
- Multiple policies targeting the same resource type share a single informer
- Apply selectors in-memory after fetching resources
- Automatic cleanup of unused informers when no policies reference them
- Handle policy updates that change GVR/namespace gracefully

**Current Limitation**:
- Each policy creates its own informer factory and watch, even if multiple policies target the same GVR
- 100 policies against the same GVR can create ~100 watches + caches, multiplying API server load and memory
- This does not scale well beyond ~50-100 policies

**Benefits**:
- Dramatically reduced API server load (one watch per GVR/namespace instead of per-policy)
- Lower memory consumption (shared cache across policies)
- Better scalability (1000+ policies feasible)
- Reduced network traffic

**Implementation Notes**:
- Requires refactoring informer management to key by (GVR, namespace) instead of policy UID
- Need reference counting to track which policies use which informers
- Cleanup logic needed when no policies reference an informer
- Policy update handling must recreate informers if GVR/namespace changes

**Related Documentation**:
- `docs/ARCHITECTURE.md` - Current informer architecture
- `docs/OPERATOR_GUIDE.md` - Resource limits and monitoring
- `pkg/controller/gc_controller.go` - Current per-policy informer implementation

**Other Optimizations**:
- More efficient filtered informer usage
- Memory usage optimization for large-scale deployments
- Performance benchmarking and tuning

#### GVR Resolution with RESTMapper
- Replace naive pluralization with discovery-based RESTMapper resolution
- Properly handle irregular Kinds and CRDs that don't follow standard pluralization rules
- Cache GVR mappings for performance
- Requires architectural change to pass discovery client through constructor

**Benefits**:
- Reliable GVR resolution for all resource types
- Support for CRDs with irregular plural forms
- Prevents deletion failures due to incorrect resource paths

**Known Limitation**:
- Current implementation uses simple pluralization which may fail for irregular Kinds/CRDs
- Workaround: Ensure CRD resource names follow standard pluralization rules

**Related Documentation**:
- `pkg/controller/gc_controller.go` - Current GVR resolution implementation
- `pkg/validation/gvr.go` - Pluralization logic

#### Distributed Tracing
- OpenTelemetry integration
- Trace context propagation across operations
- Integration with popular tracing backends (Jaeger, Zipkin, etc.)
- Tracing documentation and setup guides

**Benefits**:
- Better request tracing across components
- Easier debugging of complex deletion flows
- Integration with existing observability stacks

**Related Documentation**:
- `docs/METRICS.md` - Current observability features
- `docs/ARCHITECTURE.md` - Component interactions

---

### 1.0.0 - Stable Release

**Target**: 2027

**Focus**: Production-ready stable release

**Requirements**:
- All critical features implemented and tested
- Comprehensive documentation complete
- Production deployments validated
- API stability guaranteed
- Migration guides available

---

## 🔮 Future Considerations

### Beyond v1.0.0

These items are under consideration but not yet planned for a specific release:

- **Multi-cluster support**: Manage policies across multiple Kubernetes clusters
- **Policy versioning**: Track and manage policy changes over time
- **Policy analytics**: Insights into policy effectiveness and resource patterns
- **Webhook enhancements**: More sophisticated validation and mutation rules
- **Custom resource support**: Native support for custom resource types
- **Policy scheduling**: Time-based policy activation/deactivation
- **Resource dependencies**: Handle resource dependencies during deletion

---

## 📋 Prioritization

### High Priority (Next Release - 0.0.2-alpha)
1. **Health Check Enhancement** - Critical for production reliability

### Medium Priority (0.1.0)
2. **Policy Conditions Enhancement** - High user demand for more flexibility
3. **Dry-Run Mode Enhancement** - Important for safety and change management

### Medium Priority (0.2.0)
4. **Policy Templates** - Improves developer experience significantly

### Lower Priority (0.3.0)
5. **Shared Informer Architecture (P1.1)** - Critical for scaling beyond 100 policies, reduces API server load significantly
6. **Informer Factory Optimization** - Important for large-scale deployments
7. **Distributed Tracing** - Nice to have, depends on user observability stack

---

## 🤝 Contributing

We welcome contributions! If you're interested in working on any roadmap items:

1. Check existing issues and pull requests
2. Discuss your approach in an issue before starting
3. Follow our [Contributing Guide](CONTRIBUTING.md)
4. Ensure tests and documentation are updated

---

## 📝 Notes

- This roadmap is subject to change based on user feedback and priorities
- Breaking changes will be planned for major version releases (v1.0.0, v2.0.0)
- Timeline estimates are approximate and may shift based on resources and priorities
- Features may be released earlier or later than indicated based on community needs

---

**See Also**:
- [CHANGELOG.md](CHANGELOG.md) - Past releases and changes
- [RELEASING.md](RELEASING.md) - Release process
- [docs/VERSION_COMPATIBILITY.md](docs/VERSION_COMPATIBILITY.md) - Version compatibility matrix

