# Grafana Dashboard for zen-gc

This directory contains a pre-built Grafana dashboard for monitoring zen-gc controller metrics.

## Installation

### Option 1: Import via Grafana UI

1. Open Grafana and go to **Dashboards** → **Import**
2. Upload `dashboard.json` or paste its contents
3. Select your Prometheus data source
4. Click **Import**

### Option 2: Apply via ConfigMap (Grafana Operator)

```bash
kubectl create configmap zen-gc-dashboard \
  --from-file=dashboard.json \
  -n grafana \
  --dry-run=client -o yaml | \
  kubectl apply -f -
```

### Option 3: Apply via ConfigMap (Standard Grafana)

```bash
kubectl create configmap zen-gc-dashboard \
  --from-file=dashboard.json \
  -n <grafana-namespace> \
  --dry-run=client -o yaml | \
  kubectl apply -f -
```

## Dashboard Panels

The dashboard includes the following panels:

1. **GC Policies by Phase** - Active, Paused, Error counts
2. **Total Resources Deleted** - 5-minute deletion rate
3. **Deletion Rate** - Deletions per second
4. **Error Rate** - Errors per second
5. **Resources Deleted Over Time** - Time series by policy and resource kind
6. **Deletion Duration (P95)** - 95th percentile deletion latency
7. **Policy Evaluation Duration** - Time taken to evaluate policies
8. **Errors by Type** - Pie chart of error types
9. **Resources Matched vs Deleted** - Comparison graph
10. **Deletions by Reason** - Bar gauge showing deletion reasons
11. **Deletions by Resource Kind** - Bar gauge showing resource types

## Prerequisites

- Grafana installed and configured
- Prometheus scraping zen-gc metrics endpoint (`:8080/metrics`)
- ServiceMonitor or Prometheus scrape config pointing to `gc-controller-metrics` service

## Metrics Endpoint

The controller exposes metrics on port `8080` at `/metrics`. Ensure Prometheus is configured to scrape:

```yaml
- job_name: 'gc-controller'
  kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
          - gc-system
  relabel_configs:
    - source_labels: [__meta_kubernetes_service_name]
      action: keep
      regex: gc-controller-metrics
```

