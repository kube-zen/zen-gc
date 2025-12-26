package validation

import (
	"testing"
)

func TestPluralizeKind_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected string
	}{
		{
			name:     "empty string",
			kind:     "",
			expected: "s",
		},
		{
			name:     "single character",
			kind:     "A",
			expected: "as",
		},
		{
			name:     "ends with s",
			kind:     "Service",
			expected: "services",
		},
		{
			name:     "ends with x",
			kind:     "Index",
			expected: "indexes",
		},
		{
			name:     "ends with z",
			kind:     "Buzz",
			expected: "buzzes",
		},
		{
			name:     "ends with ch",
			kind:     "Watch",
			expected: "watches",
		},
		{
			name:     "ends with sh",
			kind:     "Dash",
			expected: "dashes",
		},
		{
			name:     "ends with y - single letter before",
			kind:     "Ay",
			expected: "aies",
		},
		{
			name:     "ends with y - multiple letters",
			kind:     "Policy",
			expected: "policies",
		},
		{
			name:     "mixed case",
			kind:     "ConfigMap",
			expected: "configmaps",
		},
		{
			name:     "all uppercase",
			kind:     "POD",
			expected: "pods",
		},
		{
			name:     "all lowercase",
			kind:     "pod",
			expected: "pods",
		},
		{
			name:     "camelCase",
			kind:     "CustomResource",
			expected: "customresources",
		},
		{
			name:     "with numbers",
			kind:     "Resource1",
			expected: "resource1s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PluralizeKind(tt.kind)
			if result != tt.expected {
				t.Errorf("PluralizeKind(%q) = %q, want %q", tt.kind, result, tt.expected)
			}
		})
	}
}

func TestParseGVR_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		apiVersion  string
		kind        string
		expectError bool
	}{
		{
			name:        "empty apiVersion",
			apiVersion:  "",
			kind:        "Resource",
			expectError: true,
		},
		{
			name:        "empty kind",
			apiVersion:  "v1",
			kind:        "",
			expectError: true, // Empty kind is invalid
		},
		{
			name:        "invalid format - no version",
			apiVersion:  "group",
			kind:        "Resource",
			expectError: true,
		},
		{
			name:        "invalid format - multiple slashes",
			apiVersion:  "group/v1/v2",
			kind:        "Resource",
			expectError: true,
		},
		{
			name:        "core API with empty group",
			apiVersion:  "v1",
			kind:        "Pod",
			expectError: false,
		},
		{
			name:        "API group with subdomain",
			apiVersion:  "apps/v1",
			kind:        "Deployment",
			expectError: false,
		},
		{
			name:        "API group with multiple subdomains",
			apiVersion:  "apps.kubernetes.io/v1",
			kind:        "Deployment",
			expectError: false,
		},
		{
			name:        "version with alpha",
			apiVersion:  "v1alpha1",
			kind:        "Resource",
			expectError: false,
		},
		{
			name:        "version with beta",
			apiVersion:  "v1beta1",
			kind:        "Resource",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGVR(tt.apiVersion, tt.kind)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseGVR() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("ParseGVR() returned error: %v", err)
				}
			}
		})
	}
}
