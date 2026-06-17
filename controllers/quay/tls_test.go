package controllers

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	quaycontext "github.com/quay/quay-operator/pkg/context"
)

func TestTranslateTLSProfile(t *testing.T) {
	for _, tt := range []struct {
		name              string
		profile           *configv1.TLSSecurityProfile
		expectedProtocols string
		expectedCiphers   string
	}{
		{
			name:              "nil profile defaults to Intermediate",
			profile:           nil,
			expectedProtocols: "TLSv1.2 TLSv1.3",
		},
		{
			name: "Old profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileOldType,
			},
			expectedProtocols: "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3",
		},
		{
			name: "Intermediate profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
			expectedProtocols: "TLSv1.2 TLSv1.3",
		},
		{
			name: "Modern profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			expectedProtocols: "TLSv1.3",
		},
		{
			name: "Custom profile with TLS 1.2",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256", "ECDHE-RSA-AES256-GCM-SHA384"},
					},
				},
			},
			expectedProtocols: "TLSv1.2 TLSv1.3",
			expectedCiphers:   "ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384",
		},
		{
			name: "Custom profile with empty minTLSVersion defaults to TLS 1.2",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: "",
						Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
					},
				},
			},
			expectedProtocols: "TLSv1.2 TLSv1.3",
			expectedCiphers:   "ECDHE-RSA-AES128-GCM-SHA256",
		},
		{
			name: "Custom type with nil Custom spec defaults to Intermediate",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
			},
			expectedProtocols: "TLSv1.2 TLSv1.3",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			protocols, ciphers := translateTLSProfile(tt.profile)
			if protocols != tt.expectedProtocols {
				t.Errorf("protocols = %q, want %q", protocols, tt.expectedProtocols)
			}
			if tt.expectedCiphers != "" && ciphers != tt.expectedCiphers {
				t.Errorf("ciphers = %q, want %q", ciphers, tt.expectedCiphers)
			}
			// For named profiles, just verify ciphers are non-empty.
			if tt.expectedCiphers == "" && tt.profile != nil &&
				tt.profile.Type != configv1.TLSProfileCustomType {
				if ciphers == "" {
					t.Error("expected non-empty ciphers for named profile")
				}
			}
		})
	}
}

func TestTlsVersionToProtocols(t *testing.T) {
	for _, tt := range []struct {
		version  configv1.TLSProtocolVersion
		expected string
	}{
		{configv1.VersionTLS10, "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3"},
		{configv1.VersionTLS11, "TLSv1.1 TLSv1.2 TLSv1.3"},
		{configv1.VersionTLS12, "TLSv1.2 TLSv1.3"},
		{configv1.VersionTLS13, "TLSv1.3"},
		{"", "TLSv1.2 TLSv1.3"},
	} {
		t.Run(string(tt.version), func(t *testing.T) {
			result := tlsVersionToProtocols(tt.version)
			if result != tt.expected {
				t.Errorf("tlsVersionToProtocols(%q) = %q, want %q", tt.version, result, tt.expected)
			}
		})
	}
}

func TestTlsCiphersToString(t *testing.T) {
	result := tlsCiphersToString([]string{"A", "B", "C"})
	if result != "A:B:C" {
		t.Errorf("tlsCiphersToString = %q, want %q", result, "A:B:C")
	}

	result = tlsCiphersToString(nil)
	if result != "" {
		t.Errorf("tlsCiphersToString(nil) = %q, want empty", result)
	}
}

func TestCheckTLSSecurityProfile_UserOverride(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = configv1.Install(scheme)
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &QuayRegistryReconciler{
		Client: client,
		Log:    zap.New(zap.UseDevMode(true)),
	}

	for _, tt := range []struct {
		name       string
		configYAML string
	}{
		{
			name:       "user set SSL_PROTOCOLS",
			configYAML: "SSL_PROTOCOLS:\n- TLSv1.3",
		},
		{
			name:       "user set SSL_CIPHERS",
			configYAML: "SSL_CIPHERS:\n- ECDHE-RSA-AES128-GCM-SHA256",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			qctx := quaycontext.NewQuayRegistryContext()
			bundle := &corev1.Secret{
				Data: map[string][]byte{
					"config.yaml": []byte(tt.configYAML),
				},
			}

			err := r.checkTLSSecurityProfile(context.Background(), qctx, bundle)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Context should NOT be populated — user override takes precedence.
			if qctx.SSLProtocols != "" {
				t.Errorf("SSLProtocols should be empty, got %q", qctx.SSLProtocols)
			}
			if qctx.SSLCiphers != "" {
				t.Errorf("SSLCiphers should be empty, got %q", qctx.SSLCiphers)
			}
		})
	}
}

func TestCheckTLSSecurityProfile_VanillaK8s(t *testing.T) {
	// Scheme without configv1 registered — simulates vanilla Kubernetes.
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &QuayRegistryReconciler{
		Client: client,
		Log:    zap.New(zap.UseDevMode(true)),
	}

	qctx := quaycontext.NewQuayRegistryContext()
	bundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": []byte("FEATURE_USER_INITIALIZE: true"),
		},
	}

	err := r.checkTLSSecurityProfile(context.Background(), qctx, bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qctx.SSLProtocols != "" {
		t.Errorf("SSLProtocols should be empty on vanilla K8s, got %q", qctx.SSLProtocols)
	}
}

func TestCheckTLSSecurityProfile_WithAPIServer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = configv1.Install(scheme)
	_ = corev1.AddToScheme(scheme)

	apiServer := &configv1.APIServer{}
	apiServer.Name = "cluster"
	apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileModernType,
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(apiServer).
		Build()

	r := &QuayRegistryReconciler{
		Client: client,
		Log:    zap.New(zap.UseDevMode(true)),
	}

	qctx := quaycontext.NewQuayRegistryContext()
	bundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": []byte("FEATURE_USER_INITIALIZE: true"),
		},
	}

	err := r.checkTLSSecurityProfile(context.Background(), qctx, bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qctx.SSLProtocols != "TLSv1.3" {
		t.Errorf("SSLProtocols = %q, want %q", qctx.SSLProtocols, "TLSv1.3")
	}
	if qctx.SSLCiphers == "" {
		t.Error("expected non-empty SSLCiphers for Modern profile")
	}
}

func TestCheckTLSSecurityProfile_NilProfile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = configv1.Install(scheme)
	_ = corev1.AddToScheme(scheme)

	apiServer := &configv1.APIServer{}
	apiServer.Name = "cluster"
	// TLSSecurityProfile is nil — should default to Intermediate.

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(apiServer).
		Build()

	r := &QuayRegistryReconciler{
		Client: client,
		Log:    zap.New(zap.UseDevMode(true)),
	}

	qctx := quaycontext.NewQuayRegistryContext()
	bundle := &corev1.Secret{
		Data: map[string][]byte{
			"config.yaml": []byte("FEATURE_USER_INITIALIZE: true"),
		},
	}

	err := r.checkTLSSecurityProfile(context.Background(), qctx, bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qctx.SSLProtocols != "TLSv1.2 TLSv1.3" {
		t.Errorf("SSLProtocols = %q, want %q", qctx.SSLProtocols, "TLSv1.2 TLSv1.3")
	}
}
