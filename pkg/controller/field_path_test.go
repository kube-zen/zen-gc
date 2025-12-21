package controller

import (
	"testing"
)

func TestParseFieldPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple field",
			input:    "spec",
			expected: []string{"spec"},
		},
		{
			name:     "nested field",
			input:    "spec.severity",
			expected: []string{"spec", "severity"},
		},
		{
			name:     "deeply nested field",
			input:    "status.lastProcessedAt",
			expected: []string{"status", "lastProcessedAt"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "triple nested",
			input:    "spec.metadata.annotations",
			expected: []string{"spec", "metadata", "annotations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFieldPath(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseFieldPath(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("parseFieldPath(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

