package controllers

import (
	"context"
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	quaycontext "github.com/quay/quay-operator/pkg/context"
)

// checkTLSSecurityProfile reads the cluster-wide TLS security profile from the
// OpenShift APIServer resource and populates the QuayRegistryContext with the
// corresponding SSL_PROTOCOLS and SSL_CIPHERS values. If the user has already
// set these values in config.yaml, this function is a no-op.
func (r *QuayRegistryReconciler) checkTLSSecurityProfile(
	ctx context.Context,
	qctx *quaycontext.QuayRegistryContext,
	bundle *corev1.Secret,
) error {
	// Parse config.yaml from the bundle to check for user overrides.
	var config map[string]interface{}
	if err := yaml.Unmarshal(bundle.Data["config.yaml"], &config); err != nil {
		return fmt.Errorf("parsing config.yaml: %w", err)
	}

	// If the user already specified SSL_PROTOCOLS or SSL_CIPHERS, respect
	// their override and do not inherit from the cluster profile.
	if _, ok := config["SSL_PROTOCOLS"]; ok {
		return nil
	}
	if _, ok := config["SSL_CIPHERS"]; ok {
		return nil
	}
	if _, ok := config["SSL_CIPHERSUITES"]; ok {
		return nil
	}

	// Try to read the APIServer "cluster" resource.
	var apiServer configv1.APIServer
	err := r.Get(ctx, types.NamespacedName{Name: "cluster"}, &apiServer)
	if err != nil {
		// On vanilla Kubernetes the CRD does not exist at all.
		if errors.IsNotFound(err) {
			return nil
		}
		// The config.openshift.io API group may not be registered.
		if meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err) {
			return nil
		}
		return fmt.Errorf("fetching APIServer: %w", err)
	}

	// Translate the profile into nginx/OpenSSL formatted strings.
	protocols, ciphers, ciphersuites := translateTLSProfile(apiServer.Spec.TLSSecurityProfile)
	qctx.SSLProtocols = protocols
	qctx.SSLCiphers = ciphers
	qctx.SSLCiphersuites = ciphersuites
	return nil
}

// translateTLSProfile converts an OpenShift TLSSecurityProfile into
// space-separated protocol versions (nginx format) and colon-separated cipher
// names (OpenSSL format).
func translateTLSProfile(profile *configv1.TLSSecurityProfile) (protocols, ciphers, ciphersuites string) {
	translate := func(spec *configv1.TLSProfileSpec) (string, string, string) {
		tls12, tls13 := splitCiphers(spec.Ciphers)
		return tlsVersionToProtocols(spec.MinTLSVersion), joinCiphers(tls12), joinCiphers(tls13)
	}

	if profile == nil {
		return translate(configv1.TLSProfiles[configv1.TLSProfileIntermediateType])
	}

	switch profile.Type {
	case configv1.TLSProfileCustomType:
		if profile.Custom != nil {
			return translate(&profile.Custom.TLSProfileSpec)
		}
		return translate(configv1.TLSProfiles[configv1.TLSProfileIntermediateType])
	default:
		spec, ok := configv1.TLSProfiles[profile.Type]
		if !ok {
			spec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
		}
		return translate(spec)
	}
}

// tlsVersionToProtocols converts an OpenShift TLSProtocolVersion to the
// space-separated list of TLS protocol names that nginx expects in the
// ssl_protocols directive.
func tlsVersionToProtocols(minVersion configv1.TLSProtocolVersion) string {
	switch minVersion {
	case configv1.VersionTLS10:
		return "TLSv1 TLSv1.1 TLSv1.2 TLSv1.3"
	case configv1.VersionTLS11:
		return "TLSv1.1 TLSv1.2 TLSv1.3"
	case configv1.VersionTLS12:
		return "TLSv1.2 TLSv1.3"
	case configv1.VersionTLS13:
		return "TLSv1.3"
	default:
		// Empty or unknown — safe default per OCPBUGS-24226.
		return "TLSv1.2 TLSv1.3"
	}
}

func splitCiphers(ciphers []string) (tls12, tls13 []string) {
	for _, c := range ciphers {
		if strings.HasPrefix(c, "TLS_") {
			tls13 = append(tls13, c)
		} else {
			tls12 = append(tls12, c)
		}
	}
	return
}

func joinCiphers(ciphers []string) string {
	return strings.Join(ciphers, ":")
}
