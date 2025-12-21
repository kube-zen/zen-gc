# GC Controller Helm Chart

This Helm chart deploys the Generic Garbage Collection Controller for Kubernetes.

## Prerequisites

- Kubernetes 1.23+
- Helm 3.0+

## Installation

```bash
# Add the repository (if applicable)
helm repo add zen-gc https://charts.kube-zen.io
helm repo update

# Install the chart
helm install gc-controller zen-gc/gc-controller --namespace gc-system --create-namespace
```

## Configuration

The following table lists the configurable parameters and their default values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `2` |
| `image.repository` | Image repository | `kube-zen/gc-controller` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `serviceAccount.create` | Create service account | `true` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `leaderElection.enabled` | Enable leader election | `true` |
| `prometheus.prometheusRule.enabled` | Enable PrometheusRule | `true` |

## Values

See [values.yaml](values.yaml) for all available configuration options.

## Uninstallation

```bash
helm uninstall gc-controller --namespace gc-system
```

## License

Apache License 2.0

