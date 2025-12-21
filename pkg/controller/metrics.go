package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// gcPoliciesTotal is a gauge that tracks the total number of GC policies by phase
	gcPoliciesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gc_policies_total",
			Help: "Total number of GC policies",
		},
		[]string{"phase"},
	)

	// gcResourcesMatchedTotal is a counter that tracks the total number of resources matched by GC policies
	gcResourcesMatchedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gc_resources_matched_total",
			Help: "Total number of resources matched by GC policies",
		},
		[]string{"policy_namespace", "policy_name", "resource_api_version", "resource_kind"},
	)

	// gcResourcesDeletedTotal is a counter that tracks the total number of resources deleted by GC
	gcResourcesDeletedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gc_resources_deleted_total",
			Help: "Total number of resources deleted by GC",
		},
		[]string{"policy_namespace", "policy_name", "resource_api_version", "resource_kind", "reason"},
	)

	// gcDeletionDurationSeconds is a histogram that tracks the time taken to delete resources
	gcDeletionDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gc_deletion_duration_seconds",
			Help:    "Time taken to delete resources",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"policy_namespace", "policy_name", "resource_api_version", "resource_kind"},
	)

	// gcErrorsTotal is a counter that tracks the total number of GC errors
	gcErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gc_errors_total",
			Help: "Total number of GC errors",
		},
		[]string{"policy_namespace", "policy_name", "error_type"},
	)

	// gcEvaluationDurationSeconds is a histogram that tracks the time taken to evaluate policies
	gcEvaluationDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gc_evaluation_duration_seconds",
			Help:    "Time taken to evaluate GC policies",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		},
		[]string{"policy_namespace", "policy_name"},
	)
)

// recordPolicyPhase records the current phase of a policy
func recordPolicyPhase(namespace, name, phase string) {
	gcPoliciesTotal.WithLabelValues(phase).Inc()
}

// recordResourceMatched records that a resource was matched by a policy
func recordResourceMatched(policyNamespace, policyName, resourceAPIVersion, resourceKind string) {
	gcResourcesMatchedTotal.WithLabelValues(policyNamespace, policyName, resourceAPIVersion, resourceKind).Inc()
}

// recordResourceDeleted records that a resource was deleted
func recordResourceDeleted(policyNamespace, policyName, resourceAPIVersion, resourceKind, reason string, duration float64) {
	gcResourcesDeletedTotal.WithLabelValues(policyNamespace, policyName, resourceAPIVersion, resourceKind, reason).Inc()
	gcDeletionDurationSeconds.WithLabelValues(policyNamespace, policyName, resourceAPIVersion, resourceKind).Observe(duration)
}

// recordError records an error that occurred during GC
func recordError(policyNamespace, policyName, errorType string) {
	gcErrorsTotal.WithLabelValues(policyNamespace, policyName, errorType).Inc()
}

// recordEvaluationDuration records the time taken to evaluate a policy
func recordEvaluationDuration(policyNamespace, policyName string, duration float64) {
	gcEvaluationDurationSeconds.WithLabelValues(policyNamespace, policyName).Observe(duration)
}

