// Copyright Contributors to the KubeOpenCode project

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// generateTestCA creates a self-signed CA certificate and returns the CA cert,
// CA key, and the PEM-encoded CA certificate bytes.
func generateTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	})

	return caCert, caKey, caCertPEM
}

// generateTestServerCert creates a server certificate signed by the given CA,
// valid for 127.0.0.1 and localhost.
func generateTestServerCert(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate server key: %v", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:    []string{"localhost"},
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create server certificate: %v", err)
	}

	serverCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertDER,
	})

	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		t.Fatalf("failed to marshal server key: %v", err)
	}

	serverKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: serverKeyDER,
	})

	tlsCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("failed to create TLS key pair: %v", err)
	}

	return tlsCert
}

func TestCustomCATLSVerification(t *testing.T) {
	// Generate a CA and server certificate
	caCert, caKey, caCertPEM := generateTestCA(t)
	serverTLSCert := generateTestServerCert(t, caCert, caKey)

	// Start an HTTPS test server with the server certificate
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	server.TLS = &tls.Config{ //nolint:gosec // test server, TLS version doesn't matter
		Certificates: []tls.Certificate{serverTLSCert},
	}
	server.StartTLS()
	defer server.Close()

	// Test 1: Without custom CA, HTTPS request should fail
	t.Run("without custom CA fails", func(t *testing.T) {
		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{ //nolint:gosec // test client
					// Empty root CAs means only system CAs are trusted.
					// Our test CA is not in the system trust store.
					RootCAs: x509.NewCertPool(),
				},
			},
		}

		_, err := client.Get(server.URL)
		if err == nil {
			t.Fatal("expected TLS verification to fail without custom CA, but request succeeded")
		}
	})

	// Test 2: With custom CA added to cert pool, HTTPS request should succeed.
	// This mirrors the pattern used in url_fetch.go's runURLFetch function:
	//   rootCAs, _ := x509.SystemCertPool()
	//   rootCAs.AppendCertsFromPEM(caCert)
	//   transport.TLSClientConfig = &tls.Config{RootCAs: rootCAs}
	t.Run("with custom CA succeeds", func(t *testing.T) {
		// Write the CA cert to a temp file (same as what CUSTOM_CA_CERT_PATH points to)
		caFile, err := os.CreateTemp(t.TempDir(), "test-ca-*.pem")
		if err != nil {
			t.Fatalf("failed to create temp CA file: %v", err)
		}
		if _, err := caFile.Write(caCertPEM); err != nil {
			t.Fatalf("failed to write CA cert: %v", err)
		}
		caFile.Close() //nolint:errcheck,gosec // test file

		// Read the CA cert from file and build a cert pool (mirrors url_fetch.go logic)
		caCertData, err := os.ReadFile(caFile.Name())
		if err != nil {
			t.Fatalf("failed to read CA cert file: %v", err)
		}

		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caCertData) {
			t.Fatal("failed to append CA cert to pool")
		}

		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{ //nolint:gosec // test client
					RootCAs: rootCAs,
				},
			},
		}

		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("expected request to succeed with custom CA, got error: %v", err)
		}
		defer resp.Body.Close() //nolint:errcheck // test

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
	})
}
