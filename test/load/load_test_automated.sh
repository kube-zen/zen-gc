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

# Automated load test script with performance metrics collection
# This script runs load tests and collects performance metrics for regression testing

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
NAMESPACE="gc-system"
TEST_NAMESPACE="gc-load-test"
RESULTS_DIR="${RESULTS_DIR:-./results}"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
RESULTS_FILE="${RESULTS_DIR}/load-test-${TIMESTAMP}.json"

# Test scenarios
declare -a SCENARIOS=(
	"100:ConfigMap:60"
	"1000:ConfigMap:60"
	"5000:ConfigMap:60"
	"100:Pod:60"
	"1000:Pod:60"
)

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_metric() {
    echo -e "${BLUE}[METRIC]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_warn "jq is not installed. JSON results will not be formatted."
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    if ! kubectl get deployment gc-controller -n "$NAMESPACE" &> /dev/null; then
        log_error "GC Controller not found in namespace $NAMESPACE"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

create_results_dir() {
    mkdir -p "$RESULTS_DIR"
}

run_load_test() {
    local num_resources=$1
    local resource_type=$2
    local ttl_seconds=$3
    
    log_info "Running load test: $num_resources $resource_type resources, TTL: ${ttl_seconds}s"
    
    local start_time=$(date +%s)
    local test_namespace="${TEST_NAMESPACE}-${num_resources}-${resource_type}"
    
    # Create test namespace
    kubectl create namespace "$test_namespace" --dry-run=client -o yaml | kubectl apply -f - || true
    
    # Create GC policy
    cat <<EOF | kubectl apply -f -
apiVersion: gc.kube-zen.io/v1alpha1
kind: GarbageCollectionPolicy
metadata:
  name: load-test-policy
  namespace: $test_namespace
spec:
  targetResource:
    apiVersion: v1
    kind: $resource_type
    namespace: $test_namespace
    labelSelector:
      matchLabels:
        load-test: "true"
  ttl:
    secondsAfterCreation: $ttl_seconds
  behavior:
    maxDeletionsPerSecond: 100
    batchSize: 50
EOF
    
    # Create resources
    log_info "Creating $num_resources $resource_type resources..."
    local create_start=$(date +%s)
    
    local batch_size=100
    local count=0
    
    while [ $count -lt $num_resources ]; do
        local batch_end=$((count + batch_size))
        if [ $batch_end -gt $num_resources ]; then
            batch_end=$num_resources
        fi
        
        for i in $(seq $count $((batch_end - 1))); do
            if [ "$resource_type" = "ConfigMap" ]; then
                kubectl create configmap "load-test-cm-$i" \
                    -n "$test_namespace" \
                    --from-literal=index="$i" \
                    --dry-run=client -o yaml | \
                kubectl label --local -f - load-test=true -o yaml | \
                kubectl apply -f - >/dev/null 2>&1 &
            elif [ "$resource_type" = "Pod" ]; then
                kubectl run "load-test-pod-$i" \
                    -n "$test_namespace" \
                    --image=busybox \
                    --restart=Never \
                    --labels=load-test=true \
                    --command -- sleep 3600 >/dev/null 2>&1 &
            fi
        done
        
        wait
        count=$batch_end
        sleep 0.5
    done
    
    local create_end=$(date +%s)
    local create_duration=$((create_end - create_start))
    
    log_info "Resources created in ${create_duration}s"
    
    # Wait for TTL to expire
    log_info "Waiting for TTL to expire (${ttl_seconds}s)..."
    sleep $ttl_seconds
    
    # Wait for deletion
    local deletion_start=$(date +%s)
    local timeout=$((ttl_seconds + 300))  # 5 minute buffer
    
    while true; do
        local remaining=$(kubectl get "$resource_type" -n "$test_namespace" -l load-test=true --no-headers 2>/dev/null | wc -l || echo "0")
        
        if [ "$remaining" -eq 0 ]; then
            local deletion_end=$(date +%s)
            local deletion_duration=$((deletion_end - deletion_start))
            log_info "All resources deleted in ${deletion_duration}s"
            break
        fi
        
        local elapsed=$(($(date +%s) - deletion_start))
        if [ $elapsed -gt $timeout ]; then
            log_warn "Timeout waiting for deletion. $remaining resources still exist"
            break
        fi
        
        sleep 5
    done
    
    local end_time=$(date +%s)
    local total_duration=$((end_time - start_time))
    
    # Collect metrics
    local metrics=""
    if kubectl port-forward -n "$NAMESPACE" service/gc-controller-metrics 8080:8080 >/dev/null 2>&1 & then
        local pf_pid=$!
        sleep 2
        
        if command -v curl &> /dev/null; then
            metrics=$(curl -s http://localhost:8080/metrics 2>/dev/null || echo "")
            kill $pf_pid 2>/dev/null || true
        fi
    fi
    
    # Cleanup
    kubectl delete namespace "$test_namespace" --ignore-not-found=true >/dev/null 2>&1
    
    # Return results as JSON
    cat <<EOF
{
  "scenario": {
    "resources": $num_resources,
    "type": "$resource_type",
    "ttl_seconds": $ttl_seconds
  },
  "timing": {
    "create_duration_seconds": $create_duration,
    "deletion_duration_seconds": $deletion_duration,
    "total_duration_seconds": $total_duration
  },
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
}

collect_baseline_metrics() {
    log_info "Collecting baseline metrics..."
    
    # Get controller pod resource usage
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l app=gc-controller -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    if [ -n "$pod_name" ]; then
        kubectl top pod "$pod_name" -n "$NAMESPACE" 2>/dev/null || log_warn "Could not get pod metrics"
    fi
}

run_all_scenarios() {
    log_info "Running all load test scenarios..."
    
    local results="["
    local first=true
    
    for scenario in "${SCENARIOS[@]}"; do
        IFS=':' read -r num_resources resource_type ttl_seconds <<< "$scenario"
        
        if [ "$first" = true ]; then
            first=false
        else
            results+=","
        fi
        
        local result=$(run_load_test "$num_resources" "$resource_type" "$ttl_seconds")
        results+="$result"
        
        # Wait between scenarios
        sleep 10
    done
    
    results+="]"
    
    # Save results
    echo "$results" > "$RESULTS_FILE"
    if command -v jq &> /dev/null; then
        echo "$results" | jq '.' > "${RESULTS_FILE}.formatted"
        log_info "Results saved to ${RESULTS_FILE}.formatted"
    else
        log_info "Results saved to $RESULTS_FILE"
    fi
}

compare_with_baseline() {
    local baseline_file="${RESULTS_DIR}/baseline.json"
    
    if [ ! -f "$baseline_file" ]; then
        log_warn "No baseline file found at $baseline_file"
        log_info "To create a baseline, run: cp $RESULTS_FILE $baseline_file"
        return
    fi
    
    log_info "Comparing results with baseline..."
    
    if command -v jq &> /dev/null; then
        # Compare key metrics
        local baseline_deletion=$(jq -r '.[0].timing.deletion_duration_seconds' "$baseline_file" 2>/dev/null || echo "0")
        local current_deletion=$(jq -r '.[0].timing.deletion_duration_seconds' "$RESULTS_FILE" 2>/dev/null || echo "0")
        
        if [ "$baseline_deletion" != "0" ] && [ "$current_deletion" != "0" ]; then
            local diff_percent=$(echo "scale=2; (($current_deletion - $baseline_deletion) / $baseline_deletion) * 100" | bc)
            log_metric "Deletion duration change: ${diff_percent}%"
            
            # Alert if significant regression (>20% slower)
            if (( $(echo "$diff_percent > 20" | bc -l) )); then
                log_error "Performance regression detected: deletion is ${diff_percent}% slower than baseline"
                return 1
            fi
        fi
    fi
}

print_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Automated load testing with performance regression detection

OPTIONS:
    --scenario COUNT:TYPE:TTL  Run specific scenario (e.g., 1000:ConfigMap:60)
    --all                       Run all scenarios (default)
    --baseline                  Create baseline from current results
    --compare                   Compare with baseline
    --results-dir DIR           Results directory (default: ./results)
    -h, --help                  Show this help message

EXAMPLES:
    # Run all scenarios
    $0 --all

    # Run specific scenario
    $0 --scenario 1000:ConfigMap:60

    # Create baseline
    $0 --all --baseline

    # Compare with baseline
    $0 --all --compare
EOF
}

main() {
    local run_all=true
    local create_baseline=false
    local compare_baseline=false
    local scenario=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --scenario)
                scenario="$2"
                run_all=false
                shift 2
                ;;
            --all)
                run_all=true
                shift
                ;;
            --baseline)
                create_baseline=true
                shift
                ;;
            --compare)
                compare_baseline=true
                shift
                ;;
            --results-dir)
                RESULTS_DIR="$2"
                shift 2
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
    
    check_prerequisites
    create_results_dir
    collect_baseline_metrics
    
    if [ "$run_all" = true ]; then
        run_all_scenarios
    elif [ -n "$scenario" ]; then
        IFS=':' read -r num_resources resource_type ttl_seconds <<< "$scenario"
        run_load_test "$num_resources" "$resource_type" "$ttl_seconds" > "$RESULTS_FILE"
    fi
    
    if [ "$create_baseline" = true ]; then
        cp "$RESULTS_FILE" "${RESULTS_DIR}/baseline.json"
        log_info "Baseline created at ${RESULTS_DIR}/baseline.json"
    fi
    
    if [ "$compare_baseline" = true ]; then
        compare_with_baseline
    fi
    
    log_info "Load testing completed!"
}

main "$@"

