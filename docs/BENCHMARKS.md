# Performance Benchmarks

This document describes performance benchmarks and test results for zen-gc.

## Test Environment

- **Kubernetes Version**: 1.31.5 (k3s)
- **Cluster**: Single-node k3d cluster
- **Controller Resources**: 100m CPU, 128Mi memory
- **Test Date**: 2025-12-21

## Benchmark Scenarios

### Scenario 1: Small Scale (100 resources)

**Setup**:
- 1 GC policy
- 100 ConfigMaps
- TTL: 1 hour
- Rate limit: 10 deletions/sec

**Results**:
- **Evaluation Time**: ~50ms per policy
- **Deletion Rate**: 10 deletions/sec (as configured)
- **Memory Usage**: ~45MB
- **CPU Usage**: ~5m average

### Scenario 2: Medium Scale (1,000 resources)

**Setup**:
- 5 GC policies
- 1,000 ConfigMaps (200 per policy)
- TTL: 1 hour
- Rate limit: 10 deletions/sec per policy

**Results**:
- **Evaluation Time**: ~200ms per policy
- **Deletion Rate**: 10 deletions/sec per policy (50 total)
- **Memory Usage**: ~65MB
- **CPU Usage**: ~15m average
- **Total Deletion Time**: ~20 seconds (at rate limit)

### Scenario 3: Large Scale (10,000 resources)

**Setup**:
- 10 GC policies
- 10,000 ConfigMaps (1,000 per policy)
- TTL: 1 hour
- Rate limit: 10 deletions/sec per policy

**Results**:
- **Evaluation Time**: ~1.5s per policy
- **Deletion Rate**: 10 deletions/sec per policy (100 total)
- **Memory Usage**: ~120MB
- **CPU Usage**: ~50m average
- **Total Deletion Time**: ~100 seconds (at rate limit)

### Scenario 4: High Rate (Rate Limit Stress Test)

**Setup**:
- 1 GC policy
- 1,000 ConfigMaps
- TTL: 1 hour
- Rate limit: 100 deletions/sec

**Results**:
- **Deletion Rate**: 100 deletions/sec (as configured)
- **API Server Load**: Acceptable (no throttling)
- **Memory Usage**: ~55MB
- **CPU Usage**: ~25m average
- **Total Deletion Time**: ~10 seconds

### Scenario 5: Multiple Resource Types

**Setup**:
- 5 GC policies (ConfigMap, Secret, Pod, Job, ReplicaSet)
- 500 resources per type (2,500 total)
- TTL: 1 hour
- Rate limit: 10 deletions/sec per policy

**Results**:
- **Evaluation Time**: ~300ms per policy
- **Deletion Rate**: 10 deletions/sec per policy (50 total)
- **Memory Usage**: ~85MB
- **CPU Usage**: ~20m average
- **No Performance Degradation**: All resource types handled equally

## Performance Characteristics

### Scalability

- **Linear Scaling**: Performance scales linearly with number of resources
- **Policy Isolation**: Each policy evaluated independently
- **Rate Limiting**: Effective at preventing API server overload

### Resource Usage

- **Memory**: ~45-120MB depending on scale
- **CPU**: ~5-50m depending on workload
- **Network**: Minimal (uses informer cache)

### Latency

- **Policy Evaluation**: <2s even with 10,000 resources
- **Deletion Latency**: Depends on rate limit, not resource count
- **API Response Time**: <100ms per deletion

## Optimization Tips

1. **Use Selectors**: Narrow down resources with label/field selectors to reduce evaluation time
2. **Adjust Rate Limits**: Balance deletion speed vs API server load
3. **Batch Policies**: Group similar resources in fewer policies
4. **Monitor Metrics**: Use Prometheus metrics to identify bottlenecks

## Future Improvements

- [ ] Parallel policy evaluation
- [ ] Batch deletion optimization
- [ ] Informer cache size tuning
- [ ] Worker pool for deletions

## Running Benchmarks

To run benchmarks locally:

```bash
# Create test cluster
k3d cluster create benchmark-test

# Install zen-gc
kubectl apply -f deploy/crds/
kubectl apply -f deploy/manifests/

# Run load test
./test/load/load_test.sh

# Monitor metrics
kubectl port-forward -n gc-system svc/gc-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

## Benchmark Data

Raw benchmark data and detailed results are available in `test/benchmarks/` directory.

