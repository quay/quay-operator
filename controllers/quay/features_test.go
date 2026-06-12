package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: false, SecretRef: &corev1.LocalObjectReference{Name: "my-tls"}},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{}},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tls", Namespace: "ns"},
					Type:       corev1.SecretTypeTLS,
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
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: false, SecretRef: &corev1.LocalObjectReference{Name: "missing"}},
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
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: false, SecretRef: &corev1.LocalObjectReference{Name: "my-tls"}},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{}},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tls", Namespace: "ns"},
					Type:       corev1.SecretTypeTLS,
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
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: false, SecretRef: &corev1.LocalObjectReference{Name: "my-tls"}},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{}},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tls", Namespace: "ns"},
					Type:       corev1.SecretTypeTLS,
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
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: false, SecretRef: &corev1.LocalObjectReference{Name: "my-tls"}},
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
					Components: []v1.Component{
						{Kind: v1.ComponentTLS, Managed: false, SecretRef: &corev1.LocalObjectReference{Name: "my-tls"}},
					},
				},
			},
			bundle: &corev1.Secret{Data: map[string][]byte{
				"ssl.key": []byte("key"),
			}},
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

				var updated corev1.Secret
				if err := cli.Get(ctx, types.NamespacedName{
					Name: v1.GetTLSSecretRef(tt.quay.Spec.Components).Name, Namespace: tt.quay.Namespace,
				}, &updated); err != nil {
					t.Fatalf("failed to refetch secret: %s", err)
				}
				if updated.Labels[v1.TLSSecretLabel] != "true" {
					t.Error("expected TLSSecretLabel to be applied to the secret")
				}
			}
		})
	}
}

func TestCheckManagedKeys_TLSFields(t *testing.T) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-ns",
		},
	}

	secretName := fmt.Sprintf("%s-%s", quay.Name, v1.ManagedKeysName)

	for _, tt := range []struct {
		name                  string
		secretData            map[string][]byte
		expectPostgresTLSCA   string
		expectPostgresTLSCert string
		expectPostgresTLSKey  string
		expectClairPgTLSCA    string
		expectClairPgTLSCert  string
		expectClairPgTLSKey   string
	}{
		{
			name: "reads all TLS fields from managed keys",
			secretData: map[string][]byte{
				"DATABASE_SECRET_KEY":     []byte("dbsecret"),
				"SECRET_KEY":              []byte("secret"),
				"DB_URI":                  []byte("postgresql://user:pass@host:5432/db"),
				"DB_ROOT_PW":              []byte("rootpw"),
				"SECURITY_SCANNER_V4_PSK": []byte("psk"),
				"CLAIR_DB_USER":           []byte("clair"),
				"CLAIR_DB_PASSWORD":       []byte("clairpw"),
				"CLAIR_DB_ROOT_PW":        []byte("clairrootpw"),
				"CLAIR_DB_NAME":           []byte("clair"),
				"POSTGRES_TLS_CA":         []byte("-----BEGIN CERTIFICATE-----\nfakeca\n-----END CERTIFICATE-----\n"),
				"POSTGRES_TLS_CERT":       []byte("-----BEGIN CERTIFICATE-----\nfakecert\n-----END CERTIFICATE-----\n"),
				"POSTGRES_TLS_KEY":        []byte("fake-postgres-key"),
				"CLAIRPOSTGRES_TLS_CA":    []byte("-----BEGIN CERTIFICATE-----\nclairca\n-----END CERTIFICATE-----\n"),
				"CLAIRPOSTGRES_TLS_CERT":  []byte("-----BEGIN CERTIFICATE-----\nclaircert\n-----END CERTIFICATE-----\n"),
				"CLAIRPOSTGRES_TLS_KEY":   []byte("fake-clairpostgres-key"),
			},
			expectPostgresTLSCA:   "-----BEGIN CERTIFICATE-----\nfakeca\n-----END CERTIFICATE-----\n",
			expectPostgresTLSCert: "-----BEGIN CERTIFICATE-----\nfakecert\n-----END CERTIFICATE-----\n",
			expectPostgresTLSKey:  "fake-postgres-key",
			expectClairPgTLSCA:    "-----BEGIN CERTIFICATE-----\nclairca\n-----END CERTIFICATE-----\n",
			expectClairPgTLSCert:  "-----BEGIN CERTIFICATE-----\nclaircert\n-----END CERTIFICATE-----\n",
			expectClairPgTLSKey:   "fake-clairpostgres-key",
		},
		{
			name: "empty TLS fields when not present in secret",
			secretData: map[string][]byte{
				"DATABASE_SECRET_KEY":     []byte("dbsecret"),
				"SECRET_KEY":              []byte("secret"),
				"DB_URI":                  []byte("postgresql://user:pass@host:5432/db"),
				"DB_ROOT_PW":              []byte("rootpw"),
				"SECURITY_SCANNER_V4_PSK": []byte("psk"),
				"CLAIR_DB_USER":           []byte("clair"),
				"CLAIR_DB_PASSWORD":       []byte("clairpw"),
				"CLAIR_DB_ROOT_PW":        []byte("clairrootpw"),
				"CLAIR_DB_NAME":           []byte("clair"),
			},
			expectPostgresTLSCA:   "",
			expectPostgresTLSCert: "",
			expectPostgresTLSKey:  "",
			expectClairPgTLSCA:    "",
			expectClairPgTLSCert:  "",
			expectClairPgTLSKey:   "",
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

			cli := fake.NewClientBuilder().WithObjects(secret).Build()
			r := newReconcilerWithClient(cli)
			qctx := quaycontext.NewQuayRegistryContext()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := r.checkManagedKeys(ctx, qctx, quay)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if qctx.PostgresTLSCA != tt.expectPostgresTLSCA {
				t.Errorf("PostgresTLSCA = %q, want %q", qctx.PostgresTLSCA, tt.expectPostgresTLSCA)
			}
			if qctx.PostgresTLSCert != tt.expectPostgresTLSCert {
				t.Errorf("PostgresTLSCert = %q, want %q", qctx.PostgresTLSCert, tt.expectPostgresTLSCert)
			}
			if qctx.PostgresTLSKey != tt.expectPostgresTLSKey {
				t.Errorf("PostgresTLSKey = %q, want %q", qctx.PostgresTLSKey, tt.expectPostgresTLSKey)
			}
			if qctx.ClairPostgresTLSCA != tt.expectClairPgTLSCA {
				t.Errorf("ClairPostgresTLSCA = %q, want %q", qctx.ClairPostgresTLSCA, tt.expectClairPgTLSCA)
			}
			if qctx.ClairPostgresTLSCert != tt.expectClairPgTLSCert {
				t.Errorf("ClairPostgresTLSCert = %q, want %q", qctx.ClairPostgresTLSCert, tt.expectClairPgTLSCert)
			}
			if qctx.ClairPostgresTLSKey != tt.expectClairPgTLSKey {
				t.Errorf("ClairPostgresTLSKey = %q, want %q", qctx.ClairPostgresTLSKey, tt.expectClairPgTLSKey)
			}
		})
	}
}

func TestCheckPostgresTLSSecrets(t *testing.T) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// Generate valid test certificates
	certPEM, keyPEM, err := cert.GenerateSelfSignedCertKey("test-db", nil, nil)
	if err != nil {
		t.Fatalf("failed to generate test certs: %v", err)
	}
	caPEM := certPEM // self-signed, CA is the same cert

	quayWithSecretRef := func(secretName string) *v1.QuayRegistry {
		return &v1.QuayRegistry{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
			Spec: v1.QuayRegistrySpec{
				Components: []v1.Component{
					{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
						Enabled:   true,
						SecretRef: &corev1.LocalObjectReference{Name: secretName},
					}}},
				},
			},
		}
	}

	for _, tt := range []struct {
		name          string
		quay          *v1.QuayRegistry
		objs          []client.Object
		expectErr     string
		expectCA      bool
		expectClairCA bool
	}{
		{
			name: "valid secret populates context",
			quay: quayWithSecretRef("pg-tls"),
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.crt": certPEM,
						"tls.key": keyPEM,
					},
				},
			},
			expectCA: true,
		},
		{
			name:      "secret not found",
			quay:      quayWithSecretRef("missing-secret"),
			objs:      []client.Object{},
			expectErr: "not found",
		},
		{
			name: "missing ca.crt key",
			quay: quayWithSecretRef("pg-tls"),
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"tls.crt": certPEM,
						"tls.key": keyPEM,
					},
				},
			},
			expectErr: "missing required key \"ca.crt\"",
		},
		{
			name: "missing tls.crt key",
			quay: quayWithSecretRef("pg-tls"),
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.key": keyPEM,
					},
				},
			},
			expectErr: "missing required key \"tls.crt\"",
		},
		{
			name: "missing tls.key key",
			quay: quayWithSecretRef("pg-tls"),
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.crt": certPEM,
					},
				},
			},
			expectErr: "missing required key \"tls.key\"",
		},
		{
			name: "cert/key mismatch",
			quay: quayWithSecretRef("pg-tls"),
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.crt": certPEM,
						"tls.key": []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALRiMLAH\n-----END RSA PRIVATE KEY-----\n"),
					},
				},
			},
			expectErr: "do not match",
		},
		{
			name: "no secretRef configured is a no-op",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
					},
				},
			},
			objs: []client.Object{},
		},
		{
			name: "TLS not enabled is a no-op",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: "postgres", Managed: true},
					},
				},
			},
			objs: []client.Object{},
		},
		{
			name: "clairpostgres valid secret populates clair context fields",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
							Enabled:   true,
							SecretRef: &corev1.LocalObjectReference{Name: "clair-pg-tls"},
						}}},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "clair-pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.crt": certPEM,
						"tls.key": keyPEM,
					},
				},
			},
			expectClairCA: true,
		},
		{
			name: "both postgres and clairpostgres configured simultaneously",
			quay: &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				Spec: v1.QuayRegistrySpec{
					Components: []v1.Component{
						{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
							Enabled:   true,
							SecretRef: &corev1.LocalObjectReference{Name: "pg-tls"},
						}}},
						{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
							Enabled:   true,
							SecretRef: &corev1.LocalObjectReference{Name: "clair-pg-tls"},
						}}},
					},
				},
			},
			objs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.crt": certPEM,
						"tls.key": keyPEM,
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "clair-pg-tls", Namespace: "test-ns"},
					Data: map[string][]byte{
						"ca.crt":  caPEM,
						"tls.crt": certPEM,
						"tls.key": keyPEM,
					},
				},
			},
			expectCA:      true,
			expectClairCA: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().WithObjects(tt.objs...).Build()
			r := newReconcilerWithClient(cli)
			qctx := quaycontext.NewQuayRegistryContext()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := r.checkPostgresTLSSecrets(ctx, qctx, tt.quay)

			if tt.expectErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectErr)
				}
				if !strings.Contains(err.Error(), tt.expectErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.expectErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expectCA {
				if qctx.PostgresTLSCA == "" {
					t.Error("expected PostgresTLSCA to be populated")
				}
				if qctx.PostgresTLSCert == "" {
					t.Error("expected PostgresTLSCert to be populated")
				}
				if qctx.PostgresTLSKey == "" {
					t.Error("expected PostgresTLSKey to be populated")
				}
			}
			if tt.expectClairCA {
				if qctx.ClairPostgresTLSCA == "" {
					t.Error("expected ClairPostgresTLSCA to be populated")
				}
				if qctx.ClairPostgresTLSCert == "" {
					t.Error("expected ClairPostgresTLSCert to be populated")
				}
				if qctx.ClairPostgresTLSKey == "" {
					t.Error("expected ClairPostgresTLSKey to be populated")
				}
			}
		})
	}
}

func TestResolvePostgresTLSSource(t *testing.T) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	for _, tt := range []struct {
		name                       string
		supportsRoutes             bool
		components                 []v1.Component
		expectPostgresUseServiceCA bool
		expectClairUseServiceCA    bool
		expectPostgresSSLRoot      string
		expectClairSSLRoot         string
	}{
		{
			name:           "OpenShift + TLS enabled no secretRef uses service CA",
			supportsRoutes: true,
			components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			},
			expectPostgresUseServiceCA: true,
			expectPostgresSSLRoot:      "/conf/stack/extra_ca_certs/service-ca.crt",
		},
		{
			name:           "OpenShift + TLS enabled with secretRef does not use service CA",
			supportsRoutes: true,
			components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
					Enabled:   true,
					SecretRef: &corev1.LocalObjectReference{Name: "my-certs"},
				}}},
			},
			expectPostgresUseServiceCA: false,
			expectPostgresSSLRoot:      "/run/secrets/postgresql/ca.crt",
		},
		{
			name:           "non-OpenShift + TLS enabled does not use service CA",
			supportsRoutes: false,
			components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			},
			expectPostgresUseServiceCA: false,
			expectPostgresSSLRoot:      "/run/secrets/postgresql/ca.crt",
		},
		{
			name:           "TLS not enabled sets no service CA or sslrootcert",
			supportsRoutes: true,
			components: []v1.Component{
				{Kind: "postgres", Managed: true},
			},
			expectPostgresUseServiceCA: false,
			expectPostgresSSLRoot:      "",
		},
		{
			name:           "OpenShift + both components TLS enabled uses service CA for both",
			supportsRoutes: true,
			components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
				{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			},
			expectPostgresUseServiceCA: true,
			expectClairUseServiceCA:    true,
			expectPostgresSSLRoot:      "/conf/stack/extra_ca_certs/service-ca.crt",
			expectClairSSLRoot:         "/var/run/certs/service-ca.crt",
		},
		{
			name:           "OpenShift + mixed: postgres secretRef, clair no secretRef",
			supportsRoutes: true,
			components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{
					Enabled:   true,
					SecretRef: &corev1.LocalObjectReference{Name: "my-certs"},
				}}},
				{Kind: "clairpostgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			},
			expectPostgresUseServiceCA: false,
			expectClairUseServiceCA:    true,
			expectPostgresSSLRoot:      "/run/secrets/postgresql/ca.crt",
			expectClairSSLRoot:         "/var/run/certs/service-ca.crt",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quay := &v1.QuayRegistry{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
				Spec:       v1.QuayRegistrySpec{Components: tt.components},
			}
			qctx := quaycontext.NewQuayRegistryContext()
			qctx.SupportsRoutes = tt.supportsRoutes

			resolvePostgresTLSSource(qctx, quay)

			if qctx.PostgresUseServiceCA != tt.expectPostgresUseServiceCA {
				t.Errorf("PostgresUseServiceCA = %v, want %v", qctx.PostgresUseServiceCA, tt.expectPostgresUseServiceCA)
			}
			if qctx.ClairPostgresUseServiceCA != tt.expectClairUseServiceCA {
				t.Errorf("ClairPostgresUseServiceCA = %v, want %v", qctx.ClairPostgresUseServiceCA, tt.expectClairUseServiceCA)
			}
			if qctx.PostgresSSLRootCert != tt.expectPostgresSSLRoot {
				t.Errorf("PostgresSSLRootCert = %q, want %q", qctx.PostgresSSLRootCert, tt.expectPostgresSSLRoot)
			}
			if qctx.ClairPostgresSSLRootCert != tt.expectClairSSLRoot {
				t.Errorf("ClairPostgresSSLRootCert = %q, want %q", qctx.ClairPostgresSSLRootCert, tt.expectClairSSLRoot)
			}
		})
	}
}

func TestEnsurePostgresServiceCAAnnotation(t *testing.T) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: "postgres", Managed: true, Overrides: &v1.Override{TLS: &v1.TLSOverride{Enabled: true}}},
			},
		},
	}

	postgresService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-quay-database",
			Namespace: "test-ns",
			Labels:    map[string]string{"quay-component": "postgres"},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 5432}},
		},
	}

	for _, tt := range []struct {
		name                string
		useServiceCA        bool
		existingAnnotation  string
		servingSecretExists bool
		expectAnnotation    string
		expectErr           string
	}{
		{
			name:                "adds annotation when useServiceCA is true",
			useServiceCA:        true,
			servingSecretExists: true,
			expectAnnotation:    "test-postgres-tls",
		},
		{
			name:                "returns error when serving cert secret not found",
			useServiceCA:        true,
			servingSecretExists: false,
			expectErr:           "serving certificate secret",
		},
		{
			name:               "removes annotation when useServiceCA is false",
			useServiceCA:       false,
			existingAnnotation: "test-postgres-tls",
			expectAnnotation:   "",
		},
		{
			name:             "no-op when useServiceCA is false and no annotation exists",
			useServiceCA:     false,
			expectAnnotation: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := postgresService.DeepCopy()
			if tt.existingAnnotation != "" {
				svc.Annotations = map[string]string{
					"service.beta.openshift.io/serving-cert-secret-name": tt.existingAnnotation,
				}
			}

			objs := []client.Object{svc}
			if tt.servingSecretExists {
				objs = append(objs, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-postgres-tls",
						Namespace: "test-ns",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
					},
				})
			}

			cli := fake.NewClientBuilder().WithObjects(objs...).Build()
			r := newReconcilerWithClient(cli)

			qctx := quaycontext.NewQuayRegistryContext()
			qctx.PostgresUseServiceCA = tt.useServiceCA

			ctx := context.Background()
			err := r.ensurePostgresServiceCAAnnotation(ctx, qctx, quay, v1.ComponentPostgres)

			if tt.expectErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectErr)
				}
				if !strings.Contains(err.Error(), tt.expectErr) {
					t.Fatalf("expected error containing %q, got %q", tt.expectErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var updatedSvc corev1.Service
			if e := cli.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &updatedSvc); e != nil {
				t.Fatalf("failed to get updated service: %v", e)
			}

			got := updatedSvc.Annotations["service.beta.openshift.io/serving-cert-secret-name"]
			if got != tt.expectAnnotation {
				t.Errorf("annotation = %q, want %q", got, tt.expectAnnotation)
			}
		})
	}
}
