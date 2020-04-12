package provisioning

import (
	"testing"

	"reflect"

	redhatcopv1alpha1 "github.com/redhat-cop/quay-operator/pkg/apis/redhatcop/v1alpha1"

	"github.com/redhat-cop/quay-operator/pkg/controller/quayecosystem/constants"
	corev1 "k8s.io/api/core/v1"
)

func TestQuayCertificatesConfigured(t *testing.T) {

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
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("sslcertificate"),
				},
			},
			expected: false,
		},
		{
			secret: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("sslcertificate"),
					constants.QuayAppConfigSSLPrivateKeySecretKey:  []byte("privatekey"),
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

func TestCopySecretContent(t *testing.T) {

	cases := []struct {
		source *corev1.Secret
		dest   *corev1.Secret
		prefix string
		output *corev1.Secret
	}{
		{
			source: &corev1.Secret{},
			dest:   &corev1.Secret{},
			output: &corev1.Secret{},
		},
		{
			source: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("sslcertificate"),
				},
			},
			dest: &corev1.Secret{},
			output: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("sslcertificate"),
				},
			},
		},
		{
			prefix: "extra_ca_certs_",
			source: &corev1.Secret{
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			},
			dest: &corev1.Secret{},
			output: &corev1.Secret{
				Data: map[string][]byte{
					"extra_ca_certs_foo": []byte("bar"),
				},
			},
		},
	}

	for i, c := range cases {
		result := copySecretContent(c.source, c.dest, c.prefix)

		if !reflect.DeepEqual(c.output, result) {
			t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.output, result)
		}
	}
}

func TestConfigFileExtraCaCertToConfigSecret(t *testing.T) {

	cases := []struct {
		configFiles  []redhatcopv1alpha1.ConfigFiles
		configSecret *corev1.Secret
		output       *corev1.Secret
		changed      bool
	}{
		{
			configFiles: []redhatcopv1alpha1.ConfigFiles{},
			configSecret: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("quaycert"),
				}},
			output: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("quaycert"),
				}},
			changed: false,
		},
		{
			configFiles: []redhatcopv1alpha1.ConfigFiles{
				redhatcopv1alpha1.ConfigFiles{
					Type: redhatcopv1alpha1.ConfigConfigFileType,
					Files: []redhatcopv1alpha1.ConfigFile{
						redhatcopv1alpha1.ConfigFile{
							Type:          redhatcopv1alpha1.ConfigConfigFileType,
							Filename:      "some_config_file.txt",
							Key:           "some_config_file.txt",
							SecretContent: []byte("contentoffile"),
						},
					},
				},
			},
			configSecret: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("quaycert"),
				}},
			output: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("quaycert"),
				}},
			changed: false,
		},
		{
			configFiles: []redhatcopv1alpha1.ConfigFiles{
				redhatcopv1alpha1.ConfigFiles{
					Type: redhatcopv1alpha1.ExtraCaCertConfigFileType,
					Files: []redhatcopv1alpha1.ConfigFile{
						redhatcopv1alpha1.ConfigFile{
							Type:          redhatcopv1alpha1.ExtraCaCertConfigFileType,
							Filename:      "my_cert.crt",
							Key:           "my_cert.crt",
							SecretContent: []byte("certcontent"),
						},
					},
				},
			},
			configSecret: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("quaycert"),
				}},
			output: &corev1.Secret{
				Data: map[string][]byte{
					constants.QuayAppConfigSSLCertificateSecretKey: []byte("quaycert"),
					"extra_ca_certs_my_cert.crt":                   []byte("certcontent"),
				}},
			changed: true,
		},
	}

	for i, c := range cases {
		result, changed := copyConfigFileExtraCaCertToConfigSecret(c.configFiles, c.configSecret)

		if !reflect.DeepEqual(c.output, result) || changed != c.changed {
			t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v\nExpected Change Found: %#v\nActual Change Found: %#v", i, c.output, result, c.changed, changed)
		}
	}
}
