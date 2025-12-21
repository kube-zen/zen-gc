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

# Default values
NAMESPACE="gc-system"
INSTALL_METHOD="kubectl"  # kubectl or helm
HELM_RELEASE_NAME="gc-controller"
IMAGE_TAG="latest"
DRY_RUN=false

# Functions
print_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Install the GC Controller for Kubernetes

OPTIONS:
    -n, --namespace NAME       Namespace to install into (default: gc-system)
    -m, --method METHOD        Installation method: kubectl or helm (default: kubectl)
    -r, --release NAME         Helm release name (default: gc-controller)
    -t, --tag TAG              Docker image tag (default: latest)
    -d, --dry-run              Show what would be installed without actually installing
    -h, --help                 Show this help message

EXAMPLES:
    # Install using kubectl
    $0

    # Install using Helm
    $0 --method helm

    # Install with custom namespace
    $0 --namespace my-gc-system

    # Dry run
    $0 --dry-run
EOF
}

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
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check kubectl connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check Helm if using Helm method
    if [ "$INSTALL_METHOD" = "helm" ]; then
        if ! command -v helm &> /dev/null; then
            log_error "helm is not installed or not in PATH"
            exit 1
        fi
    fi
    
    log_info "Prerequisites check passed"
}

install_crds() {
    log_info "Installing CRDs..."
    if [ "$DRY_RUN" = true ]; then
        kubectl apply --dry-run=client -f deploy/crds/
    else
        kubectl apply -f deploy/crds/
        log_info "Waiting for CRDs to be established..."
        kubectl wait --for=condition=established --timeout=60s crd/garbagecollectionpolicies.gc.kube-zen.io || true
    fi
}

install_kubectl() {
    log_info "Installing using kubectl..."
    
    # Create namespace
    if [ "$DRY_RUN" = true ]; then
        kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml
    else
        kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - || true
    fi
    
    # Update namespace in manifests if needed
    if [ "$NAMESPACE" != "gc-system" ]; then
        log_warn "Note: Manifests use namespace 'gc-system'. You may need to update them manually."
    fi
    
    # Install manifests (skip kustomization.yaml)
    if [ "$DRY_RUN" = true ]; then
        kubectl apply --dry-run=client -f deploy/manifests/namespace.yaml
        kubectl apply --dry-run=client -f deploy/manifests/rbac.yaml
        kubectl apply --dry-run=client -f deploy/manifests/deployment.yaml
        kubectl apply --dry-run=client -f deploy/manifests/service.yaml
    else
        kubectl apply -f deploy/manifests/namespace.yaml
        kubectl apply -f deploy/manifests/rbac.yaml
        kubectl apply -f deploy/manifests/deployment.yaml
        kubectl apply -f deploy/manifests/service.yaml
    fi
    
    # Install PrometheusRules if available
    if [ -f "deploy/prometheus/prometheus-rules.yaml" ]; then
        if [ "$DRY_RUN" = true ]; then
            kubectl apply --dry-run=client -f deploy/prometheus/prometheus-rules.yaml
        else
            kubectl apply -f deploy/prometheus/prometheus-rules.yaml
        fi
    fi
    
    log_info "Installation complete!"
    
    if [ "$DRY_RUN" != true ]; then
        log_info "Waiting for deployment to be ready..."
        kubectl wait --for=condition=available --timeout=300s deployment/gc-controller -n "$NAMESPACE" || true
        log_info "GC Controller is ready!"
    fi
}

install_helm() {
    log_info "Installing using Helm..."
    
    if [ ! -d "charts/gc-controller" ]; then
        log_error "Helm chart not found at charts/gc-controller"
        exit 1
    fi
    
    # Create namespace
    if [ "$DRY_RUN" != true ]; then
        kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - || true
    fi
    
    # Install CRDs first
    install_crds
    
    # Install Helm chart
    if [ "$DRY_RUN" = true ]; then
        helm install "$HELM_RELEASE_NAME" charts/gc-controller \
            --namespace "$NAMESPACE" \
            --set image.tag="$IMAGE_TAG" \
            --dry-run --debug
    else
        helm upgrade --install "$HELM_RELEASE_NAME" charts/gc-controller \
            --namespace "$NAMESPACE" \
            --set image.tag="$IMAGE_TAG" \
            --wait \
            --timeout 5m
        
        log_info "Installation complete!"
        log_info "GC Controller is ready!"
    fi
}

verify_installation() {
    if [ "$DRY_RUN" = true ]; then
        return
    fi
    
    log_info "Verifying installation..."
    
    # Check deployment
    if kubectl get deployment gc-controller -n "$NAMESPACE" &> /dev/null; then
        log_info "Deployment exists"
    else
        log_error "Deployment not found"
        return 1
    fi
    
    # Check pods
    if kubectl get pods -n "$NAMESPACE" -l app=gc-controller &> /dev/null; then
        log_info "Pods are running"
        kubectl get pods -n "$NAMESPACE" -l app=gc-controller
    else
        log_error "Pods not found"
        return 1
    fi
    
    # Check service
    if kubectl get service -n "$NAMESPACE" -l app=gc-controller &> /dev/null; then
        log_info "Service exists"
    else
        log_warn "Service not found"
    fi
    
    log_info "Verification complete"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -m|--method)
            INSTALL_METHOD="$2"
            shift 2
            ;;
        -r|--release)
            HELM_RELEASE_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            print_usage
            exit 1
            ;;
    esac
done

# Validate install method
if [ "$INSTALL_METHOD" != "kubectl" ] && [ "$INSTALL_METHOD" != "helm" ]; then
    log_error "Invalid install method: $INSTALL_METHOD (must be kubectl or helm)"
    exit 1
fi

# Main installation flow
log_info "Starting GC Controller installation..."
log_info "Namespace: $NAMESPACE"
log_info "Method: $INSTALL_METHOD"
if [ "$DRY_RUN" = true ]; then
    log_warn "DRY RUN MODE - No changes will be made"
fi

check_prerequisites

if [ "$INSTALL_METHOD" = "helm" ]; then
    install_helm
else
    install_crds
    install_kubectl
fi

if [ "$DRY_RUN" != true ]; then
    verify_installation
    log_info ""
    log_info "Installation completed successfully!"
    log_info "You can now create GarbageCollectionPolicy resources."
    log_info "See examples/ directory for example policies."
fi

