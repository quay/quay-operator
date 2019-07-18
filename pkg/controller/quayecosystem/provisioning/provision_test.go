package provisioning

import (
	"testing"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	corev1 "k8s.io/api/core/v1"
)

func TestAllowedNamespaces(t *testing.T) {

	cases := []struct {
		secret   *corev1.Secret
		expected bool
	}{
		{
			secret:   &corev1.Secret{},
			expected: false,
		},
		{
			secret: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte(""),
				},
			},
			expected: false,
		},
		{
			secret: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte(""),
					constants.QuayAppConfigSSLPrivateKeySecretKey:  []byte(""),
				},
			},
			expected: true,
		},
	}

	for i, c := range cases {
		result := isQuayCertificatesConfigured(c.secret)

		if c.expected != result {
			t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
		}
	}
}
