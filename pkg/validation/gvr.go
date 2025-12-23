// Package validation provides GVR parsing and validation utilities.
package validation

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ParseGVR parses a GVR from API version and kind.
func ParseGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	if apiVersion == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion cannot be empty")
	}
	if kind == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("kind cannot be empty")
	}

	// Validate format: must be "version" (core API) or "group/version" (grouped API)
	// Core API versions must start with "v"
	if !strings.Contains(apiVersion, "/") {
		if !strings.HasPrefix(apiVersion, "v") {
			return schema.GroupVersionResource{}, fmt.Errorf("apiVersion must be in format 'v1' (core API) or 'group/v1' (grouped API)")
		}
	} else {
		// Grouped API: must have both group and version
		parts := strings.Split(apiVersion, "/")
		if len(parts) != 2 {
			return schema.GroupVersionResource{}, fmt.Errorf("apiVersion must have exactly one '/' separator")
		}
		if parts[0] == "" {
			return schema.GroupVersionResource{}, fmt.Errorf("apiVersion group cannot be empty")
		}
		if parts[1] == "" {
			return schema.GroupVersionResource{}, fmt.Errorf("apiVersion version cannot be empty")
		}
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to parse API version: %w", err)
	}

	// Validate that version is not empty
	if gv.Version == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("apiVersion must include a version (e.g., 'v1' or 'apps/v1')")
	}

	resource := PluralizeKind(kind)

	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}, nil
}

// PluralizeKind converts a kind to a resource name (plural, lowercase).
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
