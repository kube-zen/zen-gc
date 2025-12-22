package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestValidateTargetResource(t *testing.T) {
	tests := []struct {
		name        string
		target      *v1alpha1.TargetResourceSpec
		expectError bool
	}{
		{
			name: "valid target resource",
			target: &v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			expectError: false,
		},
		{
			name: "missing apiVersion",
			target: &v1alpha1.TargetResourceSpec{
				Kind: "ConfigMap",
			},
			expectError: true,
		},
		{
			name: "missing kind",
			target: &v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
			},
			expectError: true,
		},
		{
			name: "empty apiVersion",
			target: &v1alpha1.TargetResourceSpec{
				APIVersion: "",
				Kind:       "ConfigMap",
			},
			expectError: true,
		},
		{
			name: "empty kind",
			target: &v1alpha1.TargetResourceSpec{
				APIVersion: "v1",
				Kind:       "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTargetResource(tt.target)
			if tt.expectError {
				if err == nil {
					t.Errorf("validateTargetResource() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateTargetResource() returned error: %v", err)
				}
			}
		})
	}
}

func TestValidateTTL(t *testing.T) {
	tests := []struct {
		name        string
		ttl         *v1alpha1.TTLSpec
		expectError bool
	}{
		{
			name: "valid fixed TTL",
			ttl: &v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(3600),
			},
			expectError: false,
		},
		{
			name: "valid fieldPath TTL",
			ttl: &v1alpha1.TTLSpec{
				FieldPath: "spec.ttlSeconds",
			},
			expectError: false,
		},
		{
			name: "valid relative TTL",
			ttl: &v1alpha1.TTLSpec{
				RelativeTo:   "status.lastProcessedAt",
				SecondsAfter: int64Ptr(3600),
			},
			expectError: false,
		},
		{
			name:        "no TTL configured",
			ttl:         &v1alpha1.TTLSpec{},
			expectError: true,
		},
		{
			name: "zero secondsAfterCreation",
			ttl: &v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(0),
			},
			expectError: true,
		},
		{
			name: "negative secondsAfterCreation",
			ttl: &v1alpha1.TTLSpec{
				SecondsAfterCreation: int64Ptr(-1),
			},
			expectError: true,
		},
		{
			name: "fieldPath with invalid mapping value",
			ttl: &v1alpha1.TTLSpec{
				FieldPath: "spec.severity",
				Mappings: map[string]int64{
					"CRITICAL": 0, // Invalid: zero value
				},
			},
			expectError: true,
		},
		{
			name: "fieldPath with negative mapping value",
			ttl: &v1alpha1.TTLSpec{
				FieldPath: "spec.severity",
				Mappings: map[string]int64{
					"CRITICAL": -1, // Invalid: negative value
				},
			},
			expectError: true,
		},
		{
			name: "relativeTo without secondsAfter",
			ttl: &v1alpha1.TTLSpec{
				RelativeTo: "status.lastProcessedAt",
			},
			expectError: true,
		},
		{
			name: "secondsAfter without relativeTo",
			ttl: &v1alpha1.TTLSpec{
				SecondsAfter: int64Ptr(3600),
			},
			expectError: true,
		},
		{
			name: "zero secondsAfter",
			ttl: &v1alpha1.TTLSpec{
				RelativeTo:   "status.lastProcessedAt",
				SecondsAfter: int64Ptr(0),
			},
			expectError: true,
		},
		{
			name: "negative secondsAfter",
			ttl: &v1alpha1.TTLSpec{
				RelativeTo:   "status.lastProcessedAt",
				SecondsAfter: int64Ptr(-1),
			},
			expectError: true,
		},
		{
			name: "fieldPath with valid mappings",
			ttl: &v1alpha1.TTLSpec{
				FieldPath: "spec.severity",
				Mappings: map[string]int64{
					"CRITICAL": 1814400,
					"HIGH":     1209600,
				},
				Default: int64Ptr(604800),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTTL(tt.ttl)
			if tt.expectError {
				if err == nil {
					t.Errorf("validateTTL() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateTTL() returned error: %v", err)
				}
			}
		})
	}
}

func TestValidateBehavior(t *testing.T) {
	tests := []struct {
		name        string
		behavior    *v1alpha1.BehaviorSpec
		expectError bool
	}{
		{
			name: "valid behavior",
			behavior: &v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: 10,
				BatchSize:             50,
				DryRun:                false,
				PropagationPolicy:     "Background",
			},
			expectError: false,
		},
		{
			name: "negative maxDeletionsPerSecond",
			behavior: &v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: -1,
			},
			expectError: true,
		},
		{
			name: "negative batchSize",
			behavior: &v1alpha1.BehaviorSpec{
				BatchSize: -1,
			},
			expectError: true,
		},
		{
			name: "zero maxDeletionsPerSecond (valid)",
			behavior: &v1alpha1.BehaviorSpec{
				MaxDeletionsPerSecond: 0,
			},
			expectError: false,
		},
		{
			name: "zero batchSize (valid)",
			behavior: &v1alpha1.BehaviorSpec{
				BatchSize: 0,
			},
			expectError: false,
		},
		{
			name: "invalid propagationPolicy",
			behavior: &v1alpha1.BehaviorSpec{
				PropagationPolicy: "Invalid",
			},
			expectError: true,
		},
		{
			name: "valid Foreground propagationPolicy",
			behavior: &v1alpha1.BehaviorSpec{
				PropagationPolicy: "Foreground",
			},
			expectError: false,
		},
		{
			name: "valid Background propagationPolicy",
			behavior: &v1alpha1.BehaviorSpec{
				PropagationPolicy: "Background",
			},
			expectError: false,
		},
		{
			name: "valid Orphan propagationPolicy",
			behavior: &v1alpha1.BehaviorSpec{
				PropagationPolicy: "Orphan",
			},
			expectError: false,
		},
		{
			name: "empty propagationPolicy (valid)",
			behavior: &v1alpha1.BehaviorSpec{
				PropagationPolicy: "",
			},
			expectError: false,
		},
		{
			name: "negative gracePeriodSeconds",
			behavior: &v1alpha1.BehaviorSpec{
				GracePeriodSeconds: int64Ptr(-1),
			},
			expectError: true,
		},
		{
			name: "zero gracePeriodSeconds (valid)",
			behavior: &v1alpha1.BehaviorSpec{
				GracePeriodSeconds: int64Ptr(0),
			},
			expectError: false,
		},
		{
			name: "positive gracePeriodSeconds (valid)",
			behavior: &v1alpha1.BehaviorSpec{
				GracePeriodSeconds: int64Ptr(30),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBehavior(tt.behavior)
			if tt.expectError {
				if err == nil {
					t.Errorf("validateBehavior() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateBehavior() returned error: %v", err)
				}
			}
		})
	}
}

func TestValidatePolicy_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		policy      *v1alpha1.GarbageCollectionPolicy
		expectError bool
	}{
		{
			name: "valid policy with all fields",
			policy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "default",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: int64Ptr(3600),
					},
					Conditions: &v1alpha1.ConditionsSpec{
						Phase: []string{"Succeeded", "Failed"},
					},
					Behavior: v1alpha1.BehaviorSpec{
						MaxDeletionsPerSecond: 10,
						BatchSize:             50,
						DryRun:                false,
						PropagationPolicy:     "Background",
						GracePeriodSeconds:    int64Ptr(30),
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid policy with relative TTL",
			policy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					TTL: v1alpha1.TTLSpec{
						RelativeTo:   "status.lastProcessedAt",
						SecondsAfter: int64Ptr(86400),
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid policy with mapped TTL",
			policy: &v1alpha1.GarbageCollectionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-policy",
					Namespace: "default",
				},
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					TTL: v1alpha1.TTLSpec{
						FieldPath: "spec.severity",
						Mappings: map[string]int64{
							"CRITICAL": 1814400,
							"HIGH":     1209600,
						},
						Default: int64Ptr(604800),
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicy(tt.policy)
			if tt.expectError {
				if err == nil {
					t.Errorf("ValidatePolicy() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePolicy() returned error: %v", err)
				}
			}
		})
	}
}

// int64Ptr helper - shared with validator_test.go
func int64Ptr(i int64) *int64 {
	return &i
}
