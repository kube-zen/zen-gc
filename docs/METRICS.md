# Metrics Documentation

This document describes all Prometheus metrics exposed by the GC controller.

## Metrics Endpoint

The GC controller exposes metrics on the `/metrics` endpoint, defaulting to port `8080`.

## Available Metrics

### `gc_policies_total`
**Type**: Gauge  
**Description**: Total number of GC policies  
**Labels**:
- `phase`: Policy phase (Active, Paused, Error)

**Example**:
```
gc_policies_total{phase="Active"} 5
gc_policies_total{phase="Paused"} 1
```

---

### `gc_resources_matched_total`
**Type**: Counter  
**Description**: Total number of resources matched by GC policies  
**Labels**:
- `policy_namespace`: Namespace of the GC policy
- `policy_name`: Name of the GC policy
- `resource_api_version`: API version of the matched resource
- `resource_kind`: Kind of the matched resource

**Example**:
```
gc_resources_matched_total{policy_namespace="zen-system",policy_name="observation-retention",resource_api_version="zen.kube-zen.io/v1",resource_kind="Observation"} 1250
```

---

### `gc_resources_deleted_total`
**Type**: Counter  
**Description**: Total number of resources deleted by GC  
**Labels**:
- `policy_namespace`: Namespace of the GC policy
- `policy_name`: Name of the GC policy
- `resource_api_version`: API version of the deleted resource
- `resource_kind`: Kind of the deleted resource
- `reason`: Reason for deletion (ttl_expired, condition_not_met, etc.)

**Example**:
```
gc_resources_deleted_total{policy_namespace="zen-system",policy_name="observation-retention",resource_api_version="zen.kube-zen.io/v1",resource_kind="Observation",reason="ttl_expired"} 1200
```

---

### `gc_deletion_duration_seconds`
**Type**: Histogram  
**Description**: Time taken to delete resources  
**Labels**:
- `policy_namespace`: Namespace of the GC policy
- `policy_name`: Name of the GC policy
- `resource_api_version`: API version of the deleted resource
- `resource_kind`: Kind of the deleted resource

**Buckets**: Default Prometheus buckets (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)

**Example**:
```
gc_deletion_duration_seconds_bucket{policy_namespace="zen-system",policy_name="observation-retention",resource_api_version="zen.kube-zen.io/v1",resource_kind="Observation",le="0.1"} 1150
```

---

### `gc_errors_total`
**Type**: Counter  
**Description**: Total number of GC errors  
**Labels**:
- `policy_namespace`: Namespace of the GC policy
- `policy_name`: Name of the GC policy
- `error_type`: Type of error (informer_creation_failed, deletion_failed, status_update_failed, etc.)

**Example**:
```
gc_errors_total{policy_namespace="zen-system",policy_name="observation-retention",error_type="deletion_failed"} 5
```

---

### `gc_evaluation_duration_seconds`
**Type**: Histogram  
**Description**: Time taken to evaluate GC policies  
**Labels**:
- `policy_namespace`: Namespace of the GC policy
- `policy_name`: Name of the GC policy

**Buckets**: [0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0]

**Example**:
```
gc_evaluation_duration_seconds_bucket{policy_namespace="zen-system",policy_name="observation-retention",le="0.1"} 1
```

---

## Health Check Endpoints

### `/healthz`
**Description**: Health check endpoint  
**Returns**: `200 OK` if the controller is running

### `/readyz`
**Description**: Readiness check endpoint  
**Returns**: `200 OK` if the controller is ready to serve requests

---

## Example Prometheus Queries

### Total resources deleted per policy
```promql
sum by (policy_namespace, policy_name) (gc_resources_deleted_total)
```

### Deletion rate per policy
```promql
rate(gc_resources_deleted_total[5m])
```

### Average deletion duration
```promql
histogram_quantile(0.95, gc_deletion_duration_seconds)
```

### Error rate
```promql
rate(gc_errors_total[5m])
```

### Policies by phase
```promql
gc_policies_total
```

---

## Grafana Dashboard

A sample Grafana dashboard JSON is available in `config/dashboards/gc-controller.json` (to be created).

