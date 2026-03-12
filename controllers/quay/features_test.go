package controllers

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

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

func newReconcilerWithClient(cli client.Client) *QuayRegistryReconciler {
	return &QuayRegistryReconciler{
		Client: cli,
		Log:    logf.Log.WithName("test"),
	}
}

func Test_checkExternalTLSSecret(t *testing.T) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	for _, tt := range []struct {
		name      string
		quay      *v1.QuayRegistry
		bundle    *corev1.Secret
		objs      []client.Object
		expectErr bool
		expectTLS bool
	}{
		{
			name: "no secretRef configured",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
			},
			bundle:    &corev1.Secret{Data: map[string][]byte{}},
			expectErr: false,
			expectTLS: false,
		},
		{
			name: "valid external TLS secret",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{}},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tls", Namespace: "ns"},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
					},
				},
			},
			expectErr: false,
			expectTLS: true,
		},
		{
			name: "external secret not found",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "missing"},
					},
				},
			},
			bundle:    &corev1.Secret{Data: map[string][]byte{}},
			expectErr: true,
		},
		{
			name: "external secret missing tls.crt",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{}},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tls", Namespace: "ns"},
					Data: map[string][]byte{
						"tls.key": []byte("key-data"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "external secret missing tls.key",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{}},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tls", Namespace: "ns"},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
					},
				},
			},
			expectErr: true,
		},
		{
			name: "conflict with ssl.cert in bundle",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{
				"ssl.cert": []byte("cert"),
			}},
			expectErr: true,
		},
		{
			name: "conflict with ssl.key in bundle",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{
				"ssl.key": []byte("key"),
			}},
			expectErr: true,
		},
		{
			name: "conflict with managed TLS component",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: v1.QuayRegistrySpec{
					TLS: &v1.TLSConfig{
						SecretRef: &corev1.LocalObjectReference{Name: "my-tls"},
					},
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: true},
					},
				},
			},
			bundle:    &corev1.Secret{Data: map[string][]byte{}},
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			cli := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			r := newReconcilerWithClient(cli)
			qctx := quaycontext.NewQuayRegistryContext()

			err := r.checkExternalTLSSecret(ctx, qctx, tt.quay, tt.bundle)

			if tt.expectErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if tt.expectTLS {
				if len(qctx.TLSCert) == 0 {
					t.Error("expected TLSCert to be populated")
				}
				if len(qctx.TLSKey) == 0 {
					t.Error("expected TLSKey to be populated")
				}
				if qctx.TLSSecretHash == "" {
					t.Error("expected TLSSecretHash to be populated")
				}
				if len(qctx.TLSSecretHash) != 8 {
					t.Errorf("expected TLSSecretHash length 8, got %d", len(qctx.TLSSecretHash))
				}
			}
		})
	}
}
