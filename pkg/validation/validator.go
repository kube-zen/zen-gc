package validation

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

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

	// ErrInvalidNamespace indicates invalid namespace format.
	ErrInvalidNamespace = errors.New("invalid namespace: must be a valid DNS-1123 label, '*' for all namespaces, or empty")

	// ErrInvalidAPIVersion indicates invalid API version format.
	ErrInvalidAPIVersion = errors.New("invalid apiVersion format")

	// ErrInvalidKind indicates invalid kind format.
	ErrInvalidKind = errors.New("invalid kind: must be non-empty and not contain leading/trailing whitespace")

	// ErrInvalidLabelKey indicates invalid label key format.
	ErrInvalidLabelKey = errors.New("invalid label key")

	// ErrInvalidLabelValue indicates invalid label value format.
	ErrInvalidLabelValue = errors.New("invalid label value")

	// ErrInvalidLabelExpressionKey indicates invalid label expression key format.
	ErrInvalidLabelExpressionKey = errors.New("invalid label expression key")

	// ErrLabelExpressionOperatorRequired indicates label expression operator is required.
	ErrLabelExpressionOperatorRequired = errors.New("label expression operator is required")

	// ErrInvalidLabelExpressionOperator indicates invalid label expression operator.
	ErrInvalidLabelExpressionOperator = errors.New("invalid label expression operator")

	// ErrLabelExpressionValuesRequired indicates label expression values are required.
	ErrLabelExpressionValuesRequired = errors.New("label expression values are required")

	// ErrInvalidLabelExpressionValue indicates invalid label expression value format.
	ErrInvalidLabelExpressionValue = errors.New("invalid label expression value")
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
	// Validate APIVersion
	if target.APIVersion == "" {
		return fmt.Errorf("%w", ErrAPIVersionRequired)
	}
	// APIVersion should not contain leading/trailing whitespace
	if strings.TrimSpace(target.APIVersion) != target.APIVersion {
		return fmt.Errorf("%w: contains leading or trailing whitespace", ErrInvalidAPIVersion)
	}
	// Basic format check: should contain at least one '/' or be a valid version
	if !strings.Contains(target.APIVersion, "/") && !isValidVersion(target.APIVersion) {
		// Allow simple versions like "v1" but validate format
		if errs := validation.IsDNS1123Subdomain(target.APIVersion); len(errs) > 0 {
			return fmt.Errorf("%w: %v", ErrInvalidAPIVersion, errs)
		}
	}

	// Validate Kind
	if target.Kind == "" {
		return fmt.Errorf("%w", ErrKindRequired)
	}
	// Kind should be non-empty and not contain leading/trailing whitespace
	// Kubernetes kinds are typically PascalCase (e.g., ConfigMap, Pod, Deployment)
	if strings.TrimSpace(target.Kind) != target.Kind {
		return fmt.Errorf("%w: contains leading or trailing whitespace", ErrInvalidKind)
	}
	// Basic validation: must start with a letter and contain only alphanumeric characters
	if target.Kind == "" {
		return fmt.Errorf("%w: cannot be empty", ErrKindRequired)
	}
	// Allow PascalCase, camelCase, and lowercase (Kubernetes allows various formats)
	// Just ensure it's not empty and doesn't have whitespace

	// Validate Namespace
	if err := validateNamespace(target.Namespace); err != nil {
		return fmt.Errorf("invalid namespace: %w", err)
	}

	// Validate LabelSelector if provided
	if target.LabelSelector != nil {
		if err := validateLabelSelector(target.LabelSelector); err != nil {
			return fmt.Errorf("invalid labelSelector: %w", err)
		}
	}

	return nil
}

// validateNamespace validates a namespace string.
// Valid values: empty string, "*" for all namespaces, or a valid DNS-1123 label.
// Kubernetes namespaces must start with a letter or number, but cannot start with a number.
func validateNamespace(namespace string) error {
	// Empty namespace is valid (will default to policy namespace)
	if namespace == "" {
		return nil
	}

	// "*" is valid for cluster-wide watching
	if namespace == "*" {
		return nil
	}

	// Kubernetes namespaces cannot start with a number
	if namespace != "" && namespace[0] >= '0' && namespace[0] <= '9' {
		return fmt.Errorf("%w: cannot start with a number", ErrInvalidNamespace)
	}

	// Otherwise, must be a valid DNS-1123 label
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		return fmt.Errorf("%w: %v", ErrInvalidNamespace, errs)
	}

	return nil
}

// validateLabelSelector validates a label selector.
func validateLabelSelector(selector *metav1.LabelSelector) error {
	// Nil selector is valid (means no selector)
	if selector == nil {
		return nil
	}

	// Validate match labels
	if err := validateMatchLabels(selector.MatchLabels); err != nil {
		return err
	}

	// Validate match expressions
	if err := validateMatchExpressions(selector.MatchExpressions); err != nil {
		return err
	}

	return nil
}

// validateMatchLabels validates match labels.
func validateMatchLabels(matchLabels map[string]string) error {
	if matchLabels == nil {
		return nil
	}

	for key, value := range matchLabels {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return fmt.Errorf("%w %q: %v", ErrInvalidLabelKey, key, errs)
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("%w %q: %v", ErrInvalidLabelValue, value, errs)
		}
	}

	return nil
}

// validateMatchExpressions validates match expressions.
func validateMatchExpressions(expressions []metav1.LabelSelectorRequirement) error {
	if expressions == nil {
		return nil
	}

	for i, expr := range expressions {
		if err := validateLabelExpression(&expr, i); err != nil {
			return err
		}
	}

	return nil
}

// validateLabelExpression validates a single label expression.
func validateLabelExpression(expr *metav1.LabelSelectorRequirement, index int) error {
	if errs := validation.IsQualifiedName(expr.Key); len(errs) > 0 {
		return fmt.Errorf("%w at index %d: %v", ErrInvalidLabelExpressionKey, index, errs)
	}

	if expr.Operator == "" {
		return fmt.Errorf("%w at index %d", ErrLabelExpressionOperatorRequired, index)
	}

	validOperators := map[metav1.LabelSelectorOperator]bool{
		metav1.LabelSelectorOpIn:           true,
		metav1.LabelSelectorOpNotIn:        true,
		metav1.LabelSelectorOpExists:       true,
		metav1.LabelSelectorOpDoesNotExist: true,
	}
	if !validOperators[expr.Operator] {
		return fmt.Errorf("%w %q at index %d (must be In, NotIn, Exists, or DoesNotExist)", ErrInvalidLabelExpressionOperator, expr.Operator, index)
	}

	// Validate values for In/NotIn operators
	if (expr.Operator == metav1.LabelSelectorOpIn || expr.Operator == metav1.LabelSelectorOpNotIn) && len(expr.Values) == 0 {
		return fmt.Errorf("%w for operator %q at index %d", ErrLabelExpressionValuesRequired, expr.Operator, index)
	}

	for j, value := range expr.Values {
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("%w %q at index %d, value %d: %v", ErrInvalidLabelExpressionValue, value, index, j, errs)
		}
	}

	return nil
}

// isValidVersion checks if a string is a valid version format.
func isValidVersion(version string) bool {
	// Basic check: should start with 'v' followed by digits, or just digits
	if version == "" {
		return false
	}
	// Allow formats like "v1", "v1beta1", "v2alpha1", etc.
	if version[0] == 'v' && len(version) > 1 {
		return true
	}
	// Allow pure numeric versions
	for _, r := range version {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
