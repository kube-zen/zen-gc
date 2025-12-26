# GC Controller Helm Chart

This Helm chart deploys the Generic Garbage Collection Controller for Kubernetes.

## Prerequisites

- Kubernetes 1.23+
- Helm 3.0+

## Installation

### Install from Helm Repository

Add the Helm repository and install:

```bash
# Add the Helm repository
helm repo add zen-gc https://kube-zen.github.io/zen-gc
helm repo update

# Install the chart
helm install gc-controller zen-gc/gc-controller --namespace gc-system --create-namespace
```

### Alternative Installation Methods

**Using install script:**

```bash
curl -sSL https://raw.githubusercontent.com/kube-zen/zen-gc/main/install.sh | bash -s -- --method helm
```

**Manual clone (for customization):**

```bash
git clone https://github.com/kube-zen/zen-gc.git
cd zen-gc
helm install gc-controller ./charts/gc-controller --namespace gc-system --create-namespace
```

## Configuration

The following table lists the configurable parameters and their default values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `2` |
| `image.repository` | Image repository | `docker.io/kube-zen/gc-controller` |
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

### Resource Recommendations

Resource requirements vary based on workload scale. See [OPERATOR_GUIDE.md](../../docs/OPERATOR_GUIDE.md#resource-limits) for detailed recommendations:

- **Small Scale** (< 10 policies, < 1,000 resources): Default values are sufficient
- **Medium Scale** (10-50 policies, 1,000-10,000 resources): Increase to 200m CPU / 256Mi memory requests, 1 CPU / 1Gi limits
- **Large Scale** (50-100 policies, 10,000-100,000 resources): Increase to 500m CPU / 512Mi memory requests, 2 CPU / 2Gi limits
- **Very Large Scale** (> 100 policies, > 100,000 resources): Increase to 1 CPU / 1Gi memory requests, 4 CPU / 4Gi limits

### Vertical Pod Autoscaler (VPA)

For automatic resource adjustment, you can use Kubernetes VPA. A sample VPA manifest is available in `deploy/manifests/vpa.yaml`:

```bash
# Apply VPA (recommendations only - safest for production)
kubectl apply -f deploy/manifests/vpa.yaml

# View VPA recommendations
kubectl describe vpa gc-controller-vpa -n gc-system
```

See [OPERATOR_GUIDE.md](../../docs/OPERATOR_GUIDE.md#vertical-pod-autoscaler-vpa-support) for VPA configuration details.

## Values

See [values.yaml](values.yaml) for all available configuration options.

## Uninstallation

```bash
helm uninstall gc-controller --namespace gc-system
```

## License

Apache License 2.0

