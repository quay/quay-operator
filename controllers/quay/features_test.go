package controllers

import (
	"testing"
)

func TestExtractImageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "image with digest",
			input:    "quay.io/redhat/quay@sha256:abcdef1234567890",
			expected: "quay.io/redhat/quay",
		},
		{
			name:     "image with tag",
			input:    "quay.io/redhat/quay:v3.7.0",
			expected: "quay.io/redhat/quay",
		},
		{
			name:     "image without digest or tag",
			input:    "quay.io/redhat/quay",
			expected: "quay.io/redhat/quay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageName(tt.input)
			if result != tt.expected {
				t.Errorf("extractImageName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
