package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

func TestValidatePolicy(t *testing.T) {
	tests := []struct {
		name        string
		policy      *v1alpha1.GarbageCollectionPolicy
		expectError bool
	}{
		{
			name: "valid policy with fixed TTL",
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
						SecondsAfterCreation: int64Ptr(3600),
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
		{
			name: "missing target resource",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: int64Ptr(3600),
					},
				},
			},
			expectError: true,
		},
		{
			name: "missing TTL",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid behavior - negative rate",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: int64Ptr(3600),
					},
					Behavior: v1alpha1.BehaviorSpec{
						MaxDeletionsPerSecond: -1,
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid propagation policy",
			policy: &v1alpha1.GarbageCollectionPolicy{
				Spec: v1alpha1.GarbageCollectionPolicySpec{
					TargetResource: v1alpha1.TargetResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					TTL: v1alpha1.TTLSpec{
						SecondsAfterCreation: int64Ptr(3600),
					},
					Behavior: v1alpha1.BehaviorSpec{
						PropagationPolicy: "Invalid",
					},
				},
			},
			expectError: true,
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

func int64Ptr(i int64) *int64 {
	return &i
}
