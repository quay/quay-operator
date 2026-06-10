package kustomize

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePostgresTLSCerts(t *testing.T) {
	caCertPEM, serverCertPEM, serverKeyPEM, err := generatePostgresTLSCerts("test-quay-database", "test-ns")
	require.NoError(t, err)

	assert.NotEmpty(t, caCertPEM)
	assert.NotEmpty(t, serverCertPEM)
	assert.NotEmpty(t, serverKeyPEM)

	caCert := parseCert(t, caCertPEM)
	serverCert := parseCert(t, serverCertPEM)
	serverKey := parseECKey(t, serverKeyPEM)

	t.Run("CA is self-signed CA certificate", func(t *testing.T) {
		assert.True(t, caCert.IsCA)
		assert.Equal(t, "test-quay-database-ca", caCert.Subject.CommonName)
		assert.Equal(t, caCert.Issuer.CommonName, caCert.Subject.CommonName)
	})

	t.Run("server cert is signed by CA", func(t *testing.T) {
		roots := x509.NewCertPool()
		roots.AddCert(caCert)
		_, err := serverCert.Verify(x509.VerifyOptions{
			Roots: roots,
			KeyUsages: []x509.ExtKeyUsage{
				x509.ExtKeyUsageServerAuth,
			},
		})
		assert.NoError(t, err)
	})

	t.Run("server cert has correct SANs", func(t *testing.T) {
		expectedSANs := []string{
			"test-quay-database",
			"test-quay-database.test-ns",
			"test-quay-database.test-ns.svc",
			"test-quay-database.test-ns.svc.cluster.local",
			"localhost",
		}
		assert.Equal(t, expectedSANs, serverCert.DNSNames)
	})

	t.Run("uses ECDSA P-256 keys", func(t *testing.T) {
		caKey, ok := caCert.PublicKey.(*ecdsa.PublicKey)
		require.True(t, ok, "CA cert should use ECDSA key")
		assert.Equal(t, elliptic.P256(), caKey.Curve)

		serverPubKey, ok := serverCert.PublicKey.(*ecdsa.PublicKey)
		require.True(t, ok, "server cert should use ECDSA key")
		assert.Equal(t, elliptic.P256(), serverPubKey.Curve)

		assert.Equal(t, elliptic.P256(), serverKey.Curve)
	})

	t.Run("10-year validity", func(t *testing.T) {
		expectedExpiry := time.Now().AddDate(certValidityYears, 0, 0)
		assert.WithinDuration(t, expectedExpiry, caCert.NotAfter, 2*time.Hour)
		assert.WithinDuration(t, expectedExpiry, serverCert.NotAfter, 2*time.Hour)
	})

	t.Run("server cert has ServerAuth extended key usage", func(t *testing.T) {
		assert.Contains(t, serverCert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
	})

	t.Run("server key matches server cert", func(t *testing.T) {
		certPubKey, ok := serverCert.PublicKey.(*ecdsa.PublicKey)
		require.True(t, ok)
		assert.True(t, serverKey.PublicKey.Equal(certPubKey))
	})
}

func TestGeneratePostgresTLSCerts_UniqueSerials(t *testing.T) {
	_, cert1PEM, _, err := generatePostgresTLSCerts("svc1", "ns1")
	require.NoError(t, err)
	_, cert2PEM, _, err := generatePostgresTLSCerts("svc1", "ns1")
	require.NoError(t, err)

	cert1 := parseCert(t, cert1PEM)
	cert2 := parseCert(t, cert2PEM)
	assert.NotEqual(t, cert1.SerialNumber, cert2.SerialNumber)
}

func TestPostgresSANs(t *testing.T) {
	sans := postgresSANs("myquay-quay-database", "quay-enterprise")
	expected := []string{
		"myquay-quay-database",
		"myquay-quay-database.quay-enterprise",
		"myquay-quay-database.quay-enterprise.svc",
		"myquay-quay-database.quay-enterprise.svc.cluster.local",
		"localhost",
	}
	assert.Equal(t, expected, sans)
}

func parseCert(t *testing.T, pemData string) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode([]byte(pemData))
	require.NotNil(t, block, "failed to decode PEM block")
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}

func parseECKey(t *testing.T, pemData string) *ecdsa.PrivateKey {
	t.Helper()
	block, _ := pem.Decode([]byte(pemData))
	require.NotNil(t, block, "failed to decode PEM block")
	key, err := x509.ParseECPrivateKey(block.Bytes)
	require.NoError(t, err)
	return key
}
