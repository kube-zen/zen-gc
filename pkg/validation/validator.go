package validation

import (
	"errors"
	"fmt"

	gcapi "github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
)

var (
	// ErrAPIVersionRequired indicates apiVersion is required.
	ErrAPIVersionRequired = errors.New("apiVersion is required")

	// ErrKindRequired indicates kind is required.
	ErrKindRequired = errors.New("kind is required")

	// ErrNoTTLOptionSpecified indicates at least one TTL option must be specified.
	ErrNoTTLOptionSpecified = errors.New("at least one TTL option must be specified")

	// ErrInvalidTTLMapping indicates invalid TTL mapping value.
	ErrInvalidTTLMapping = errors.New("invalid TTL mapping: value must be positive")

	// ErrMaxDeletionsPerSecondNegative indicates maxDeletionsPerSecond must be non-negative.
	ErrMaxDeletionsPerSecondNegative = errors.New("maxDeletionsPerSecond must be non-negative")

	// ErrBatchSizeNegative indicates batchSize must be non-negative.
	ErrBatchSizeNegative = errors.New("batchSize must be non-negative")

	// ErrInvalidPropagationPolicy indicates invalid propagationPolicy value.
	ErrInvalidPropagationPolicy = errors.New("invalid propagationPolicy")

	// ErrGracePeriodSecondsNegative indicates gracePeriodSeconds must be non-negative.
	ErrGracePeriodSecondsNegative = errors.New("gracePeriodSeconds must be non-negative")
)

// ValidatePolicy validates a GarbageCollectionPolicy.
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

// validateTargetResource validates the target resource specification.
func validateTargetResource(target *gcapi.TargetResourceSpec) error {
	if target.APIVersion == "" {
		return fmt.Errorf("%w", ErrAPIVersionRequired)
	}

	if target.Kind == "" {
		return fmt.Errorf("%w", ErrKindRequired)
	}

	return nil
}

// validateTTL validates the TTL specification.
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
		return fmt.Errorf("%w", ErrNoTTLOptionSpecified)
	}

	// Validate mappings if fieldPath is specified
	if ttl.FieldPath != "" && len(ttl.Mappings) > 0 {
		// Mappings are optional, but if specified, they should be valid
		for key, value := range ttl.Mappings {
			if value <= 0 {
				return fmt.Errorf("%w for key %s", ErrInvalidTTLMapping, key)
			}
		}
	}

	return nil
}

// validateBehavior validates the behavior specification.
func validateBehavior(behavior *gcapi.BehaviorSpec) error {
	if behavior.MaxDeletionsPerSecond < 0 {
		return fmt.Errorf("%w", ErrMaxDeletionsPerSecondNegative)
	}

	if behavior.BatchSize < 0 {
		return fmt.Errorf("%w", ErrBatchSizeNegative)
	}

	if behavior.PropagationPolicy != "" {
		validPolicies := map[string]bool{
			"Foreground": true,
			"Background": true,
			"Orphan":     true,
		}
		if !validPolicies[behavior.PropagationPolicy] {
			return fmt.Errorf("%w: %s (must be Foreground, Background, or Orphan)", ErrInvalidPropagationPolicy, behavior.PropagationPolicy)
		}
	}

	if behavior.GracePeriodSeconds != nil && *behavior.GracePeriodSeconds < 0 {
		return fmt.Errorf("%w", ErrGracePeriodSecondsNegative)
	}

	return nil
}
