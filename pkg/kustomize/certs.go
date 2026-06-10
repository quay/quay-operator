package kustomize

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

const (
	certValidityYears = 10
)

// generatePostgresTLSCerts generates a self-signed CA and server certificate for
// a PostgreSQL service. The server certificate includes SANs for all in-cluster
// DNS names. Returns PEM-encoded CA cert, server cert, and server key.
func generatePostgresTLSCerts(serviceName, namespace string) (caCert, serverCert, serverKey string, err error) {
	caKeyPair, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", fmt.Errorf("generating CA key: %w", err)
	}

	caSerial, err := randomSerial()
	if err != nil {
		return "", "", "", err
	}

	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			CommonName: serviceName + "-ca",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(certValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKeyPair.PublicKey, caKeyPair)
	if err != nil {
		return "", "", "", fmt.Errorf("creating CA certificate: %w", err)
	}

	caParsed, err := x509.ParseCertificate(caDER)
	if err != nil {
		return "", "", "", fmt.Errorf("parsing CA certificate: %w", err)
	}

	serverKeyPair, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", "", fmt.Errorf("generating server key: %w", err)
	}

	serverSerial, err := randomSerial()
	if err != nil {
		return "", "", "", err
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			CommonName: serviceName,
		},
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.AddDate(certValidityYears, 0, 0),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: postgresSANs(serviceName, namespace),
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caParsed, &serverKeyPair.PublicKey, caKeyPair)
	if err != nil {
		return "", "", "", fmt.Errorf("creating server certificate: %w", err)
	}

	caCertPEM, err := encodeCertPEM(caDER)
	if err != nil {
		return "", "", "", err
	}

	serverCertPEM, err := encodeCertPEM(serverDER)
	if err != nil {
		return "", "", "", err
	}

	serverKeyPEM, err := encodeECKeyPEM(serverKeyPair)
	if err != nil {
		return "", "", "", err
	}

	return caCertPEM, serverCertPEM, serverKeyPEM, nil
}

// postgresSANs returns the DNS Subject Alternative Names for a PostgreSQL
// service running in the given namespace.
func postgresSANs(serviceName, namespace string) []string {
	return []string{
		serviceName,
		serviceName + "." + namespace,
		serviceName + "." + namespace + ".svc",
		serviceName + "." + namespace + ".svc.cluster.local",
		"localhost",
	}
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial number: %w", err)
	}
	return serial, nil
}

func encodeCertPEM(derBytes []byte) (string, error) {
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", fmt.Errorf("encoding certificate PEM: %w", err)
	}
	return buf.String(), nil
}

func encodeECKeyPEM(key *ecdsa.PrivateKey) (string, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("marshaling EC private key: %w", err)
	}
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}); err != nil {
		return "", fmt.Errorf("encoding key PEM: %w", err)
	}
	return buf.String(), nil
}
