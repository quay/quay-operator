package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
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

func TestCheckManagedKeys_UpgradeFallback(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-ns",
		},
	}
	secretName := fmt.Sprintf("%s-%s", quay.Name, v1.ManagedKeysName)

	for _, tt := range []struct {
		name                      string
		secretData                map[string][]byte
		expectedClairPassword     string
		expectedClairRootPassword string
	}{
		{
			name: "upgrade - secret exists without clair postgres keys",
			secretData: map[string][]byte{
				"DATABASE_SECRET_KEY": []byte("some-key"),
				"SECRET_KEY":          []byte("another-key"),
			},
			expectedClairPassword:     "postgres",
			expectedClairRootPassword: "postgres",
		},
		{
			name: "normal reconcile - secret has clair postgres keys",
			secretData: map[string][]byte{
				"DATABASE_SECRET_KEY":          []byte("some-key"),
				"CLAIR_POSTGRES_PASSWORD":      []byte("random-pw"),
				"CLAIR_POSTGRES_ROOT_PASSWORD": []byte("random-root-pw"),
			},
			expectedClairPassword:     "random-pw",
			expectedClairRootPassword: "random-root-pw",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: quay.Namespace,
				},
				Data: tt.secretData,
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(secret).
				Build()

			reconciler := QuayRegistryReconciler{
				Client: fakeClient,
				Log:    testLogger,
			}

			qctx := &quaycontext.QuayRegistryContext{}
			if err := reconciler.checkManagedKeys(context.Background(), qctx, quay); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if qctx.ClairPostgresPassword != tt.expectedClairPassword {
				t.Errorf("ClairPostgresPassword = %q, want %q",
					qctx.ClairPostgresPassword, tt.expectedClairPassword)
			}
			if qctx.ClairPostgresRootPassword != tt.expectedClairRootPassword {
				t.Errorf("ClairPostgresRootPassword = %q, want %q",
					qctx.ClairPostgresRootPassword, tt.expectedClairRootPassword)
			}
		})
	}
}

func TestCheckManagedKeys_FreshInstall(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)

	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-ns",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := QuayRegistryReconciler{
		Client: fakeClient,
		Log:    testLogger,
	}

	qctx := &quaycontext.QuayRegistryContext{}
	if err := reconciler.checkManagedKeys(context.Background(), qctx, quay); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fresh install: no secret exists, fields should remain empty
	// so KustomizationFor() generates random passwords
	if qctx.ClairPostgresPassword != "" {
		t.Errorf("ClairPostgresPassword = %q, want empty", qctx.ClairPostgresPassword)
	}
	if qctx.ClairPostgresRootPassword != "" {
		t.Errorf("ClairPostgresRootPassword = %q, want empty", qctx.ClairPostgresRootPassword)
	}
}
