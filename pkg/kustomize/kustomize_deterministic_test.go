package kustomize

import (
	"strings"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/quay/quay-operator/apis/quay/v1"
	quaycontext "github.com/quay/quay-operator/pkg/context"
)

// TestKustomizationDeterministicSecretGeneration verifies that KustomizationFor
// generates the same secret names across multiple invocations when the input
// configuration is identical. This addresses the bug where non-deterministic
// map iteration caused Kustomize to compute different content hashes on each
// reconcile, resulting in unnecessary secret recreation.
func TestKustomizationDeterministicSecretGeneration(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-registry",
			Namespace: "test-ns",
		},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: true},
				{Kind: v1.ComponentRedis, Managed: true},
			},
		},
	}

	ctx := &quaycontext.QuayRegistryContext{
		ServerHostname: "quay.example.com",
		DbUri:          "postgresql://user:pass@host:5432/db",
	}

	// Config bundle with multiple files to ensure map iteration matters
	configFiles := map[string][]byte{
		"config.yaml":       []byte("FEATURE_MAILING: false"),
		"extra_ca_cert_rh":  []byte("-----BEGIN CERTIFICATE-----\nRH_CERT\n-----END CERTIFICATE-----"),
		"extra_ca_cert_foo": []byte("-----BEGIN CERTIFICATE-----\nFOO_CERT\n-----END CERTIFICATE-----"),
		"extra_ca_cert_bar": []byte("-----BEGIN CERTIFICATE-----\nBAR_CERT\n-----END CERTIFICATE-----"),
		"custom.cert":       []byte("custom cert content"),
	}

	// Generate kustomization multiple times
	results := make([]string, 5)
	for i := 0; i < 5; i++ {
		kustomization, err := KustomizationFor(testlogr.NewTestLogger(t), ctx, quay, configFiles, overlayDir())
		assert.NoError(t, err, "KustomizationFor should not error")
		assert.NotNil(t, kustomization, "Kustomization should not be nil")

		// Extract the config secret generator
		var configSecretFileSources []string
		for _, secret := range kustomization.SecretGenerator {
			if secret.Name == configSecretPrefix {
				configSecretFileSources = secret.FileSources
				break
			}
		}

		// Convert to string for comparison
		results[i] = strings.Join(configSecretFileSources, "|")
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i],
			"File source ordering must be deterministic across multiple invocations (iteration %d)", i)
	}
}

// TestKustomizationDeterministicCACerts verifies that CA certificates
// are processed in a deterministic order regardless of map iteration order.
func TestKustomizationDeterministicCACerts(t *testing.T) {
	quay := &v1.QuayRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-registry",
			Namespace: "test-ns",
		},
		Spec: v1.QuayRegistrySpec{
			Components: []v1.Component{
				{Kind: v1.ComponentPostgres, Managed: true},
			},
		},
	}

	ctx := &quaycontext.QuayRegistryContext{
		ServerHostname: "quay.example.com",
		DbUri:          "postgresql://user:pass@host:5432/db",
	}

	// Config bundle with multiple CA certs
	configFiles := map[string][]byte{
		"config.yaml":         []byte("FEATURE_MAILING: false"),
		"extra_ca_cert_zebra": []byte("ZEBRA_CERT"),
		"extra_ca_cert_apple": []byte("APPLE_CERT"),
		"extra_ca_cert_mango": []byte("MANGO_CERT"),
	}

	// Generate kustomization multiple times
	results := make([]string, 5)
	for i := 0; i < 5; i++ {
		kustomization, err := KustomizationFor(testlogr.NewTestLogger(t), ctx, quay, configFiles, overlayDir())
		assert.NoError(t, err, "KustomizationFor should not error")

		// Extract the extra-ca-certs secret generator
		var caCertSources []string
		for _, secret := range kustomization.SecretGenerator {
			if secret.Name == "extra-ca-certs" {
				caCertSources = secret.LiteralSources
				break
			}
		}

		results[i] = strings.Join(caCertSources, "|")
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		assert.Equal(t, results[0], results[i],
			"CA cert ordering must be deterministic across multiple invocations (iteration %d)", i)
	}

	// Verify alphabetical ordering is maintained
	kustomization, err := KustomizationFor(testlogr.NewTestLogger(t), ctx, quay, configFiles, overlayDir())
	assert.NoError(t, err, "KustomizationFor should not error")
	assert.NotNil(t, kustomization, "Kustomization should not be nil")
	var caCertSources []string
	for _, secret := range kustomization.SecretGenerator {
		if secret.Name == "extra-ca-certs" {
			caCertSources = secret.LiteralSources
			break
		}
	}
	assert.GreaterOrEqual(t, len(caCertSources), 3, "expected at least 3 CA cert sources")

	// Should be in alphabetical order: apple, mango, zebra
	assert.Contains(t, caCertSources[0], "apple", "First CA cert should be apple (alphabetically first)")
	assert.Contains(t, caCertSources[1], "mango", "Second CA cert should be mango")
	assert.Contains(t, caCertSources[2], "zebra", "Third CA cert should be zebra (alphabetically last)")
}
