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
TEST_NAMESPACE="gc-load-test"
NUM_RESOURCES=1000
RESOURCE_TYPE="ConfigMap"
TTL_SECONDS=60
CLEANUP=true

print_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Simple load test for GC Controller

OPTIONS:
    -n, --namespace NAME       GC Controller namespace (default: gc-system)
    -t, --test-ns NAME         Test namespace (default: gc-load-test)
    -c, --count COUNT          Number of resources to create (default: 1000)
    -r, --resource TYPE        Resource type (default: ConfigMap)
    -s, --ttl SECONDS          TTL in seconds (default: 60)
    --no-cleanup               Don't cleanup test resources
    -h, --help                 Show this help message

EXAMPLES:
    # Basic load test
    $0

    # Test with 5000 ConfigMaps
    $0 --count 5000

    # Test with Pods
    $0 --resource Pod --count 500
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
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if GC Controller is running
    if ! kubectl get deployment gc-controller -n "$NAMESPACE" &> /dev/null; then
        log_error "GC Controller not found in namespace $NAMESPACE"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

create_test_namespace() {
    log_info "Creating test namespace: $TEST_NAMESPACE"
    kubectl create namespace "$TEST_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - || true
}

create_gc_policy() {
    log_info "Creating GC policy for load test..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: gc.k8s.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: load-test-policy
  namespace: $TEST_NAMESPACE
spec:
  targetResource:
    apiVersion: v1
    kind: $RESOURCE_TYPE
    namespace: $TEST_NAMESPACE
    labelSelector:
      matchLabels:
        load-test: "true"
  ttl:
    secondsAfterCreation: $TTL_SECONDS
  behavior:
    maxDeletionsPerSecond: 100
    batchSize: 50
EOF
    
    log_info "GC policy created"
}

create_test_resources() {
    log_info "Creating $NUM_RESOURCES $RESOURCE_TYPE resources..."
    
    local count=0
    local batch_size=100
    
    while [ $count -lt $NUM_RESOURCES ]; do
        local batch_end=$((count + batch_size))
        if [ $batch_end -gt $NUM_RESOURCES ]; then
            batch_end=$NUM_RESOURCES
        fi
        
        log_info "Creating resources $count to $((batch_end - 1))..."
        
        for i in $(seq $count $((batch_end - 1))); do
            if [ "$RESOURCE_TYPE" = "ConfigMap" ]; then
                kubectl create configmap "load-test-cm-$i" \
                    -n "$TEST_NAMESPACE" \
                    --from-literal=index="$i" \
                    --from-literal=created="$(date +%s)" \
                    --dry-run=client -o yaml | \
                kubectl label --local -f - load-test=true -o yaml | \
                kubectl apply -f - &
            elif [ "$RESOURCE_TYPE" = "Pod" ]; then
                kubectl run "load-test-pod-$i" \
                    -n "$TEST_NAMESPACE" \
                    --image=busybox \
                    --restart=Never \
                    --labels=load-test=true \
                    --command -- sleep 3600 &
            else
                log_error "Unsupported resource type: $RESOURCE_TYPE"
                exit 1
            fi
        done
        
        wait
        count=$batch_end
        
        # Small delay to avoid overwhelming the API server
        sleep 1
    done
    
    log_info "All resources created"
}

wait_for_deletion() {
    log_info "Waiting for resources to be deleted (TTL: ${TTL_SECONDS}s)..."
    
    local start_time=$(date +%s)
    local timeout=$((TTL_SECONDS + 120))  # Add 2 minutes buffer
    
    while true; do
        local remaining=$(kubectl get "$RESOURCE_TYPE" -n "$TEST_NAMESPACE" -l load-test=true --no-headers 2>/dev/null | wc -l || echo "0")
        
        if [ "$remaining" -eq 0 ]; then
            local elapsed=$(($(date +%s) - start_time))
            log_info "All resources deleted in ${elapsed}s"
            break
        fi
        
        local elapsed=$(($(date +%s) - start_time))
        if [ $elapsed -gt $timeout ]; then
            log_warn "Timeout waiting for deletion. $remaining resources still exist"
            break
        fi
        
        echo -ne "\rRemaining: $remaining resources (elapsed: ${elapsed}s)"
        sleep 5
    done
    echo ""
}

check_metrics() {
    log_info "Checking GC Controller metrics..."
    
    # Port-forward to metrics service
    log_info "Port-forwarding to metrics service..."
    kubectl port-forward -n "$NAMESPACE" service/gc-controller-metrics 8080:8080 &
    local pf_pid=$!
    sleep 2
    
    # Fetch metrics
    if command -v curl &> /dev/null; then
        log_info "Fetching metrics..."
        curl -s http://localhost:8080/metrics | grep -E "^gc_" | head -20 || true
    else
        log_warn "curl not available, skipping metrics check"
    fi
    
    # Kill port-forward
    kill $pf_pid 2>/dev/null || true
}

cleanup() {
    if [ "$CLEANUP" = true ]; then
        log_info "Cleaning up test resources..."
        kubectl delete namespace "$TEST_NAMESPACE" --ignore-not-found=true
        kubectl delete garbagecollectionpolicy load-test-policy -n "$TEST_NAMESPACE" --ignore-not-found=true
        log_info "Cleanup complete"
    else
        log_warn "Skipping cleanup (--no-cleanup specified)"
    fi
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -t|--test-ns)
            TEST_NAMESPACE="$2"
            shift 2
            ;;
        -c|--count)
            NUM_RESOURCES="$2"
            shift 2
            ;;
        -r|--resource)
            RESOURCE_TYPE="$2"
            shift 2
            ;;
        -s|--ttl)
            TTL_SECONDS="$2"
            shift 2
            ;;
        --no-cleanup)
            CLEANUP=false
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

# Main execution
log_info "Starting GC Controller load test"
log_info "Test namespace: $TEST_NAMESPACE"
log_info "Resource type: $RESOURCE_TYPE"
log_info "Number of resources: $NUM_RESOURCES"
log_info "TTL: ${TTL_SECONDS}s"

trap cleanup EXIT

check_prerequisites
create_test_namespace
create_gc_policy
create_test_resources

log_info "Waiting for TTL to expire..."
sleep $TTL_SECONDS

wait_for_deletion
check_metrics

log_info "Load test completed!"

