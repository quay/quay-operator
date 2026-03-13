package controllers

import (
	"context"
	"errors"
	"strings"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

func TestParseServerHostname(t *testing.T) {
	for _, tt := range []struct {
		name             string
		configYAML       string
		expectedHostname string
		expectErr        bool
	}{
		{
			name:             "SERVER_HOSTNAME set",
			configYAML:       "SERVER_HOSTNAME: registry.example.com",
			expectedHostname: "registry.example.com",
		},
		{
			name:             "SERVER_HOSTNAME not set",
			configYAML:       "FEATURE_USER_INITIALIZE: true",
			expectedHostname: "",
		},
		{
			name:       "invalid YAML",
			configYAML: "{{invalid",
			expectErr:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			qctx := quaycontext.NewQuayRegistryContext()
			bundle := &corev1.Secret{
				Data: map[string][]byte{
					"config.yaml": []byte(tt.configYAML),
				},
			}

			err := parseServerHostname(qctx, bundle)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if qctx.ServerHostname != tt.expectedHostname {
				t.Errorf("ServerHostname = %q, want %q", qctx.ServerHostname, tt.expectedHostname)
			}
		})
	}
}

func TestFillServerHostname(t *testing.T) {
	for _, tt := range []struct {
		name             string
		serverHostname   string
		clusterHostname  string
		quayName         string
		quayNamespace    string
		expectedHostname string
	}{
		{
			name:             "already set",
			serverHostname:   "custom.example.com",
			clusterHostname:  "apps.cluster.example.com",
			quayName:         "test",
			quayNamespace:    "ns",
			expectedHostname: "custom.example.com",
		},
		{
			name:             "derived from cluster hostname",
			serverHostname:   "",
			clusterHostname:  "apps.cluster.example.com",
			quayName:         "test",
			quayNamespace:    "ns",
			expectedHostname: "test-quay-ns.apps.cluster.example.com",
		},
		{
			name:             "both empty",
			serverHostname:   "",
			clusterHostname:  "",
			quayName:         "test",
			quayNamespace:    "ns",
			expectedHostname: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			qctx := quaycontext.NewQuayRegistryContext()
			qctx.ServerHostname = tt.serverHostname
			qctx.ClusterHostname = tt.clusterHostname

			quay := &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.quayName,
					Namespace: tt.quayNamespace,
				},
			}

			fillServerHostname(qctx, quay)

			if qctx.ServerHostname != tt.expectedHostname {
				t.Errorf("ServerHostname = %q, want %q", qctx.ServerHostname, tt.expectedHostname)
			}
		})
	}
}

func TestEnsureRouteDiscovery(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = routev1.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			UID:       "test-uid",
		},
	}

	t.Run("fast path: hostname cached", func(t *testing.T) {
		r := &QuayRegistryReconciler{
			Log: zap.New(zap.UseDevMode(true)),
		}
		hostname := "apps.cached.example.com"
		r.clusterHostname.Store(&hostname)
		cert := []byte("cached-cert")
		r.clusterWildcardCert.Store(&cert)

		qctx := quaycontext.NewQuayRegistryContext()
		err := r.ensureRouteDiscovery(context.Background(), qctx, quay)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !qctx.SupportsRoutes {
			t.Error("expected SupportsRoutes=true")
		}
		if qctx.ClusterHostname != "apps.cached.example.com" {
			t.Errorf("ClusterHostname = %q, want %q", qctx.ClusterHostname, "apps.cached.example.com")
		}
		if string(qctx.ClusterWildcardCert) != "cached-cert" {
			t.Errorf("ClusterWildcardCert = %q, want %q", string(qctx.ClusterWildcardCert), "cached-cert")
		}
	})

	t.Run("probe in progress: route exists but no ingress", func(t *testing.T) {
		existingRoute := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-test-route",
				Namespace: "ns",
			},
		}
		client := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(existingRoute).
			Build()

		r := &QuayRegistryReconciler{
			Client: client,
			Log:    zap.New(zap.UseDevMode(true)),
		}

		qctx := quaycontext.NewQuayRegistryContext()
		err := r.ensureRouteDiscovery(context.Background(), qctx, quay)
		if !errors.Is(err, errRouteProbeInProgress) {
			t.Fatalf("expected errRouteProbeInProgress, got: %v", err)
		}
	})

	t.Run("successful discovery: route with ingress", func(t *testing.T) {
		existingRoute := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-test-route",
				Namespace: "ns",
			},
			Spec: routev1.RouteSpec{
				Host: "test-test-route-ns.apps.cluster.example.com",
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "none",
				},
			},
			Status: routev1.RouteStatus{
				Ingress: []routev1.RouteIngress{
					{
						RouterName:              "default",
						RouterCanonicalHostname: "router-default.apps.cluster.example.com",
					},
				},
			},
		}
		client := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(existingRoute).
			Build()

		r := &QuayRegistryReconciler{
			Client: client,
			Log:    zap.New(zap.UseDevMode(true)),
		}

		qctx := quaycontext.NewQuayRegistryContext()
		err := r.ensureRouteDiscovery(context.Background(), qctx, quay)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !qctx.SupportsRoutes {
			t.Error("expected SupportsRoutes=true")
		}
		if qctx.ClusterHostname != "apps.cluster.example.com" {
			t.Errorf("ClusterHostname = %q, want %q", qctx.ClusterHostname, "apps.cluster.example.com")
		}

		// Verify hostname was cached.
		cached := r.clusterHostname.Load()
		if cached == nil || *cached != "apps.cluster.example.com" {
			t.Error("hostname was not cached")
		}
	})
}

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
