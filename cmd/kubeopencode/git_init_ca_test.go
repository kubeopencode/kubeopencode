// Copyright Contributors to the KubeOpenCode project

package main

import (
	"os"
	"strings"
	"testing"
)

const testCACert = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJALRiMLAh0KIQMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAeFw0yNDAxMDEwMDAwMDBaFw0yNTAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96HtiVQJKzMRAHxHW/t
LHLnTDHMRVKKxCEpJ0bXaxURmC3OJfSyVnCuRMqPMy8F0fXBFqBVgbMjqVMVd7dV
AgMBAAEwDQYJKoZIhvcNAQELBQADQQBiDDmeGsmF2JJcKz5NLQYHJKGJ3WbaNqSG
0YQMKQ3wPSog44rJFighFqMFrXmnSIQsjiMFikNolNMV2M2NGkPT
-----END CERTIFICATE-----`

func TestSetupCustomCA_NoEnvVar(t *testing.T) {
	// Ensure CUSTOM_CA_CERT_PATH is not set
	t.Setenv("CUSTOM_CA_CERT_PATH", "")

	// Clear GIT_SSL_CAINFO to verify it is not set by setupCustomCA
	t.Setenv("GIT_SSL_CAINFO", "")

	err := setupCustomCA()
	if err != nil {
		t.Fatalf("expected no error when CUSTOM_CA_CERT_PATH is not set, got: %v", err)
	}

	if val := os.Getenv("GIT_SSL_CAINFO"); val != "" {
		t.Errorf("expected GIT_SSL_CAINFO to be unset, got: %s", val)
	}
}

func TestSetupCustomCA_WithValidCA(t *testing.T) {
	// Create a temporary PEM file
	tmpFile, err := os.CreateTemp(t.TempDir(), "custom-ca-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(testCACert); err != nil {
		t.Fatalf("failed to write test CA cert: %v", err)
	}
	tmpFile.Close() //nolint:errcheck,gosec // test file

	t.Setenv("CUSTOM_CA_CERT_PATH", tmpFile.Name())
	t.Setenv("GIT_SSL_CAINFO", "")
	t.Cleanup(func() {
		os.Remove("/tmp/ca-bundle.crt") //nolint:errcheck,gosec // best-effort cleanup
	})

	err = setupCustomCA()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	caInfoPath := os.Getenv("GIT_SSL_CAINFO")
	if caInfoPath != "/tmp/ca-bundle.crt" {
		t.Fatalf("expected GIT_SSL_CAINFO to be /tmp/ca-bundle.crt, got: %s", caInfoPath)
	}

	// Verify the bundle file contains the custom CA content
	bundleContent, err := os.ReadFile(caInfoPath) //nolint:gosec // test file, path from env
	if err != nil {
		t.Fatalf("failed to read bundle file: %v", err)
	}

	if !strings.Contains(string(bundleContent), testCACert) {
		t.Error("expected bundle file to contain the custom CA certificate")
	}
}

func TestSetupCustomCA_FileNotFound(t *testing.T) {
	t.Setenv("CUSTOM_CA_CERT_PATH", "/nonexistent/path/ca.pem")

	err := setupCustomCA()
	if err == nil {
		t.Fatal("expected error when CA cert file does not exist, got nil")
	}

	if !strings.Contains(err.Error(), "failed to read custom CA certificate") {
		t.Errorf("expected error message about reading CA certificate, got: %v", err)
	}
}

func TestSetupCustomCA_ConcatenatesWithSystemCA(t *testing.T) {
	// Create a temporary PEM file with custom CA
	tmpFile, err := os.CreateTemp(t.TempDir(), "custom-ca-*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(testCACert); err != nil {
		t.Fatalf("failed to write test CA cert: %v", err)
	}
	tmpFile.Close() //nolint:errcheck,gosec // test file

	t.Setenv("CUSTOM_CA_CERT_PATH", tmpFile.Name())
	t.Setenv("GIT_SSL_CAINFO", "")
	t.Cleanup(func() {
		os.Remove("/tmp/ca-bundle.crt") //nolint:errcheck,gosec // best-effort cleanup
	})

	err = setupCustomCA()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	bundleContent, err := os.ReadFile("/tmp/ca-bundle.crt") //nolint:gosec // test file
	if err != nil {
		t.Fatalf("failed to read bundle file: %v", err)
	}

	bundleStr := string(bundleContent)

	// The bundle must always contain the custom CA
	if !strings.Contains(bundleStr, testCACert) {
		t.Error("expected bundle to contain the custom CA certificate")
	}

	// Check if a system CA bundle was found and concatenated.
	// On systems with a CA bundle (Linux), the bundle should be larger than just
	// the custom cert. On macOS (where system CAs are in Keychain, not a file),
	// the bundle may only contain the custom cert - both cases are valid.
	systemCAPaths := []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/cert.pem",
		"/etc/ssl/ca-bundle.pem",
	}

	systemCAFound := false
	for _, sysPath := range systemCAPaths {
		if _, err := os.Stat(sysPath); err == nil {
			systemCAFound = true
			systemCA, err := os.ReadFile(sysPath) //nolint:gosec // test file, hardcoded paths
			if err != nil {
				t.Fatalf("failed to read system CA bundle at %s: %v", sysPath, err)
			}
			// Verify the bundle starts with system CA content
			if !strings.Contains(bundleStr, string(systemCA)) {
				t.Error("expected bundle to contain system CA content when system CA bundle is available")
			}
			// Verify the combined bundle is larger than either part alone
			if len(bundleContent) <= len(systemCA) {
				t.Error("expected combined bundle to be larger than system CA bundle alone")
			}
			t.Logf("system CA bundle found at %s, verified concatenation", sysPath)
			break
		}
	}

	if !systemCAFound {
		t.Log("no system CA bundle found (e.g., macOS uses Keychain); bundle contains only custom CA")
	}
}
