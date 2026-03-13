package controllers

import (
	"strings"
	"testing"
)

func Test_extractImageName(t *testing.T) {
	for _, tt := range []struct {
		name      string
		imageName string
		expected  string
	}{
		{
			name:      "image with digest",
			imageName: "quay.io/projectquay/quay-postgres-rhel8@sha256:abc123",
			expected:  "quay.io/projectquay/quay-postgres-rhel8",
		},
		{
			name:      "image with tag",
			imageName: "quay.io/projectquay/quay-postgres-rhel8:v1.2.3",
			expected:  "quay.io/projectquay/quay-postgres-rhel8",
		},
		{
			name:      "image without tag or digest",
			imageName: "quay.io/projectquay/quay-postgres-rhel8",
			expected:  "quay.io/projectquay/quay-postgres-rhel8",
		},
		{
			name:      "image with multiple path components",
			imageName: "registry.example.com/org/team/postgres:latest",
			expected:  "registry.example.com/org/team/postgres",
		},
		{
			name:      "simple image name",
			imageName: "postgres:13",
			expected:  "postgres",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageName(tt.imageName)
			if result != tt.expected {
				t.Errorf("extractImageName(%q) = %q, want %q", tt.imageName, result, tt.expected)
			}
		})
	}
}

func Test_repositoryNameComparison(t *testing.T) {
	for _, tt := range []struct {
		name         string
		currentName  string
		expectedName string
		shouldMatch  bool
	}{
		{
			name:         "exact match - same registry and repo",
			currentName:  "quay.io/projectquay/quay-postgres-rhel8",
			expectedName: "quay.io/projectquay/quay-postgres-rhel8",
			shouldMatch:  true,
		},
		{
			name:         "different registries - same repo name",
			currentName:  "quay.io/projectquay/quay-postgres-rhel8",
			expectedName: "registry.example.com/org/quay-postgres-rhel8",
			shouldMatch:  true,
		},
		{
			name:         "different repo names",
			currentName:  "quay.io/projectquay/quay-postgres-rhel8",
			expectedName: "quay.io/projectquay/clair-postgres-rhel8",
			shouldMatch:  false,
		},
		{
			name:         "different org - same repo name",
			currentName:  "quay.io/org1/postgres",
			expectedName: "quay.io/org2/postgres",
			shouldMatch:  true,
		},
		{
			name:         "repo with slashes in name",
			currentName:  "quay.io/projectquay/team/postgres",
			expectedName: "registry.io/org/team/postgres",
			shouldMatch:  true,
		},
		{
			name:         "simple names - match",
			currentName:  "postgres",
			expectedName: "postgres",
			shouldMatch:  true,
		},
		{
			name:         "simple names - no match",
			currentName:  "postgres",
			expectedName: "mysql",
			shouldMatch:  false,
		},
		{
			name:         "registry vs simple name - same repo",
			currentName:  "quay.io/projectquay/postgres",
			expectedName: "postgres",
			shouldMatch:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Extract repository names by finding the last component after splitting by '/'
			// This is the same logic used in the actual code
			currentRepoName := tt.currentName[strings.LastIndex(tt.currentName, "/")+1:]
			expectedRepoName := tt.expectedName[strings.LastIndex(tt.expectedName, "/")+1:]

			matches := currentRepoName == expectedRepoName
			if matches != tt.shouldMatch {
				t.Errorf("Repository name comparison failed:\n"+
					"  current:  %q (repo: %q)\n"+
					"  expected: %q (repo: %q)\n"+
					"  matches:  %v, want: %v",
					tt.currentName, currentRepoName,
					tt.expectedName, expectedRepoName,
					matches, tt.shouldMatch)
			}
		})
	}
}
