package validation

import (
	"strings"
	"testing"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestParseGVR(t *testing.T) {
	tests := []struct {
		name        string
		apiVersion  string
		kind        string
		expectedGVR schema.GroupVersionResource
		expectError bool
	}{
		{
			name:       "core API group",
			apiVersion: "v1",
			kind:       "Pod",
			expectedGVR: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			expectError: false,
		},
		{
			name:       "apps API group",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			expectError: false,
		},
		{
			name:       "apps API group",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			expectedGVR: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			expectError: false,
		},
		{
			name:        "invalid API version",
			apiVersion:  "invalid",
			kind:        "Resource",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvr, err := ParseGVR(tt.apiVersion, tt.kind)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseGVR() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseGVR() returned error: %v", err)
			}

			if gvr.Group != tt.expectedGVR.Group {
				t.Errorf("ParseGVR() Group = %q, want %q", gvr.Group, tt.expectedGVR.Group)
			}
			if gvr.Version != tt.expectedGVR.Version {
				t.Errorf("ParseGVR() Version = %q, want %q", gvr.Version, tt.expectedGVR.Version)
			}
			if gvr.Resource != tt.expectedGVR.Resource {
				t.Errorf("ParseGVR() Resource = %q, want %q", gvr.Resource, tt.expectedGVR.Resource)
			}
		})
	}
}

func TestPluralizeKind(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected string
	}{
		{
			name:     "simple plural",
			kind:     "Pod",
			expected: "pods",
		},
		{
			name:     "ends with s",
			kind:     "Service",
			expected: "services",
		},
		{
			name:     "ends with y",
			kind:     "Policy",
			expected: "policies",
		},
		{
			name:     "ends with x",
			kind:     "Index",
			expected: "indexes",
		},
		{
			name:     "ends with ch",
			kind:     "Watch",
			expected: "watches",
		},
		{
			name:     "custom resource",
			kind:     "Deployment",
			expected: "deployments",
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

func TestValidateGVR(t *testing.T) {
	tests := []struct {
		name        string
		gvr         schema.GroupVersionResource
		expectError bool
	}{
		{
			name: "valid GVR",
			gvr: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			expectError: false,
		},
		{
			name: "empty version",
			gvr: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "",
				Resource: "deployments",
			},
			expectError: true,
		},
		{
			name: "empty resource",
			gvr: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "",
			},
			expectError: true,
		},
		{
			name: "empty group is valid (core API)",
			gvr: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert GVR to API version and kind for testing ParseGVR
			apiVersion := tt.gvr.Version
			if tt.gvr.Group != "" {
				apiVersion = tt.gvr.Group + "/" + tt.gvr.Version
			}
			// Convert resource name back to kind (singularize and capitalize)
			kind := strings.TrimSuffix(tt.gvr.Resource, "s")
			if strings.HasSuffix(tt.gvr.Resource, "ies") {
				kind = strings.TrimSuffix(tt.gvr.Resource, "ies") + "y"
			}
			kind = cases.Title(language.English).String(kind)

			_, err := ParseGVR(apiVersion, kind)
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
