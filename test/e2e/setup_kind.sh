#!/bin/bash
# Copyright 2025 Kube-ZEN Contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

CLUSTER_NAME="${CLUSTER_NAME:-zen-gc-e2e}"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-${HOME}/.kube/${CLUSTER_NAME}-config}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kind &> /dev/null; then
        log_error "kind is not installed. Install from https://kind.sigs.k8s.io/"
        exit 1
    fi
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

create_cluster() {
    log_info "Creating kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_warn "Cluster $CLUSTER_NAME already exists"
        read -p "Delete existing cluster? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kind delete cluster --name "$CLUSTER_NAME"
        else
            log_info "Using existing cluster"
            return
        fi
    fi
    
    # Create cluster config with port mappings for metrics/webhook
    cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
  - containerPort: 9443
    hostPort: 9443
    protocol: TCP
EOF
    
    log_info "Cluster created successfully"
}

setup_kubeconfig() {
    log_info "Setting up kubeconfig..."
    export KUBECONFIG="$KUBECONFIG_PATH"
    kind get kubeconfig --name "$CLUSTER_NAME" > "$KUBECONFIG_PATH"
    chmod 600 "$KUBECONFIG_PATH"
    log_info "Kubeconfig saved to $KUBECONFIG_PATH"
    log_info "Export KUBECONFIG=$KUBECONFIG_PATH to use this cluster"
}

install_crds() {
    log_info "Installing CRDs..."
    kubectl apply -f ../../deploy/crds/
    log_info "Waiting for CRDs to be established..."
    kubectl wait --for condition=established --timeout=60s crd/garbagecollectionpolicies.gc.kube-zen.io || true
}

deploy_controller() {
    log_info "Deploying GC Controller..."
    
    # Create namespace
    kubectl create namespace gc-system --dry-run=client -o yaml | kubectl apply -f -
    
    # Apply RBAC
    kubectl apply -f ../../deploy/manifests/rbac.yaml
    
    # Build and load image
    log_info "Building controller image..."
    docker build -t kube-zen/gc-controller:test ../../.
    kind load docker-image kube-zen/gc-controller:test --name "$CLUSTER_NAME"
    
    # Apply deployment (modify image tag)
    kubectl apply -f ../../deploy/manifests/deployment.yaml
    kubectl set image deployment/gc-controller gc-controller=kube-zen/gc-controller:test -n gc-system
    
    log_info "Waiting for controller to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/gc-controller -n gc-system
}

cleanup_cluster() {
    log_info "Cleaning up cluster..."
    kind delete cluster --name "$CLUSTER_NAME" || true
    rm -f "$KUBECONFIG_PATH"
    log_info "Cleanup complete"
}

print_usage() {
    cat << EOF
Usage: $0 [COMMAND]

Commands:
    create      Create kind cluster and deploy controller
    delete      Delete kind cluster
    kubeconfig  Show kubeconfig export command
    help        Show this help message

Environment Variables:
    CLUSTER_NAME       Name of the kind cluster (default: zen-gc-e2e)
    KUBECONFIG_PATH    Path to kubeconfig file (default: ~/.kube/zen-gc-e2e-config)

Examples:
    # Create cluster and deploy controller
    $0 create

    # Delete cluster
    $0 delete

    # Export kubeconfig
    export KUBECONFIG=\$($0 kubeconfig)
EOF
}

main() {
    case "${1:-help}" in
        create)
            check_prerequisites
            create_cluster
            setup_kubeconfig
            install_crds
            deploy_controller
            log_info "✅ E2E test cluster is ready!"
            log_info "Export KUBECONFIG=$KUBECONFIG_PATH to use this cluster"
            ;;
        delete)
            cleanup_cluster
            ;;
        kubeconfig)
            echo "$KUBECONFIG_PATH"
            ;;
        help|--help|-h)
            print_usage
            ;;
        *)
            log_error "Unknown command: $1"
            print_usage
            exit 1
            ;;
    esac
}

main "$@"

