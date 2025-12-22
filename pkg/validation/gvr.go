package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ParseGVR parses a GVR from API version and kind
func ParseGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to parse API version: %w", err)
	}

	resource := PluralizeKind(kind)

	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}, nil
}

// PluralizeKind converts a kind to a resource name (plural, lowercase)
func PluralizeKind(kind string) string {
	// Simple conversion: lowercase and add 's' or 'es'
	// This is a simplified version; in production, you'd want to use discovery
	lower := strings.ToLower(kind)

	// Handle common pluralization rules
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") ||
		strings.HasSuffix(lower, "z") || strings.HasSuffix(lower, "ch") ||
		strings.HasSuffix(lower, "sh") {
		return lower + "es"
	}

	if strings.HasSuffix(lower, "y") && len(lower) > 1 {
		// Change 'y' to 'ies'
		return lower[:len(lower)-1] + "ies"
	}

	return lower + "s"
}
