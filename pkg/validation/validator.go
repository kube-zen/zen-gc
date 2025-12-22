package validation

import (
	"fmt"

	gcapi "github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

// ValidatePolicy validates a GarbageCollectionPolicy
func ValidatePolicy(policy *gcapi.GarbageCollectionPolicy) error {
	// Validate target resource
	if err := validateTargetResource(&policy.Spec.TargetResource); err != nil {
		return fmt.Errorf("invalid targetResource: %w", err)
	}

	// Validate TTL
	if err := validateTTL(&policy.Spec.TTL); err != nil {
		return fmt.Errorf("invalid ttl: %w", err)
	}

	// Validate behavior
	if err := validateBehavior(&policy.Spec.Behavior); err != nil {
		return fmt.Errorf("invalid behavior: %w", err)
	}

	return nil
}

// validateTargetResource validates the target resource specification
func validateTargetResource(target *gcapi.TargetResourceSpec) error {
	if target.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}

	if target.Kind == "" {
		return fmt.Errorf("kind is required")
	}

	return nil
}

// validateTTL validates the TTL specification
func validateTTL(ttl *gcapi.TTLSpec) error {
	// At least one TTL option must be specified
	hasTTL := false

	if ttl.SecondsAfterCreation != nil && *ttl.SecondsAfterCreation > 0 {
		hasTTL = true
	}

	if ttl.FieldPath != "" {
		hasTTL = true
	}

	if ttl.RelativeTo != "" && ttl.SecondsAfter != nil && *ttl.SecondsAfter > 0 {
		hasTTL = true
	}

	if !hasTTL {
		return fmt.Errorf("at least one TTL option must be specified")
	}

	// Validate mappings if fieldPath is specified
	if ttl.FieldPath != "" && len(ttl.Mappings) > 0 {
		// Mappings are optional, but if specified, they should be valid
		for key, value := range ttl.Mappings {
			if value <= 0 {
				return fmt.Errorf("invalid TTL mapping for key %s: value must be positive", key)
			}
		}
	}

	return nil
}

// validateBehavior validates the behavior specification
func validateBehavior(behavior *gcapi.BehaviorSpec) error {
	if behavior.MaxDeletionsPerSecond < 0 {
		return fmt.Errorf("maxDeletionsPerSecond must be non-negative")
	}

	if behavior.BatchSize < 0 {
		return fmt.Errorf("batchSize must be non-negative")
	}

	if behavior.PropagationPolicy != "" {
		validPolicies := map[string]bool{
			"Foreground": true,
			"Background": true,
			"Orphan":     true,
		}
		if !validPolicies[behavior.PropagationPolicy] {
			return fmt.Errorf("invalid propagationPolicy: %s (must be Foreground, Background, or Orphan)", behavior.PropagationPolicy)
		}
	}

	if behavior.GracePeriodSeconds != nil && *behavior.GracePeriodSeconds < 0 {
		return fmt.Errorf("gracePeriodSeconds must be non-negative")
	}

	return nil
}
