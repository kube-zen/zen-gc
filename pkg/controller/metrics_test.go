package controller

import (
	"testing"
)

func TestRecordPolicyPhase(t *testing.T) {
	// Test that metric recording doesn't panic
	recordPolicyPhase("default", "test-policy", "Active")
	recordPolicyPhase("default", "test-policy", "Paused")
	recordPolicyPhase("default", "test-policy", "Error")

	// Verify metric was recorded (we can't easily check exact values without exposing internals,
	// but we can verify it doesn't panic)
}

func TestRecordResourceMatched(t *testing.T) {
	recordResourceMatched("default", "test-policy", "v1", "ConfigMap")
	recordResourceMatched("default", "test-policy", "v1", "Pod")

	// Verify metric was recorded
}

func TestRecordResourceDeleted(t *testing.T) {
	recordResourceDeleted("default", "test-policy", "v1", "ConfigMap", "ttl_expired", 0.5)
	recordResourceDeleted("default", "test-policy", "v1", "Pod", "condition_not_met", 0.3)

	// Verify metric was recorded
}

func TestRecordError(t *testing.T) {
	recordError("default", "test-policy", "deletion_failed")
	recordError("default", "test-policy", "informer_creation_failed")

	// Verify metric was recorded
}

func TestRecordEvaluationDuration(t *testing.T) {
	recordEvaluationDuration("default", "test-policy", 0.1)
	recordEvaluationDuration("default", "test-policy", 0.5)

	// Verify metric was recorded
}

func TestMetrics_AllFunctions(t *testing.T) {
	// Test all metric recording functions don't panic
	t.Run("recordPolicyPhase", func(t *testing.T) {
		recordPolicyPhase("ns1", "policy1", "Active")
		recordPolicyPhase("ns1", "policy1", "Paused")
		recordPolicyPhase("ns1", "policy1", "Error")
	})

	t.Run("recordResourceMatched", func(t *testing.T) {
		recordResourceMatched("ns1", "policy1", "v1", "ConfigMap")
		recordResourceMatched("ns1", "policy1", "apps/v1", "Deployment")
	})

	t.Run("recordResourceDeleted", func(t *testing.T) {
		recordResourceDeleted("ns1", "policy1", "v1", "ConfigMap", "ttl_expired", 0.1)
		recordResourceDeleted("ns1", "policy1", "v1", "Pod", "condition_not_met", 0.2)
	})

	t.Run("recordError", func(t *testing.T) {
		recordError("ns1", "policy1", "deletion_failed")
		recordError("ns1", "policy1", "informer_creation_failed")
		recordError("ns1", "policy1", "status_update_failed")
	})

	t.Run("recordEvaluationDuration", func(t *testing.T) {
		recordEvaluationDuration("ns1", "policy1", 0.05)
		recordEvaluationDuration("ns1", "policy1", 0.1)
		recordEvaluationDuration("ns1", "policy1", 1.0)
	})
}
