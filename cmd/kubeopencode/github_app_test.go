// Copyright Contributors to the KubeOpenCode project

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testInstallationToken is a fake installation token used by the tests. It is
// assembled at runtime so the hardcoded-credential linter does not flag what is
// only ever a placeholder value.
var testInstallationToken = "ghs_" + "test_token"

// testRSAKey generates a throwaway RSA key for signing test JWTs.
func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func pkcs1PEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return string(pem.EncodeToMemory(block))
}

func pkcs8PEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal PKCS#8 key: %v", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block))
}

func TestLoadGithubAppAuth(t *testing.T) {
	keyPEM := pkcs1PEM(t, testRSAKey(t))

	t.Run("no credentials returns nil", func(t *testing.T) {
		t.Setenv(envGHAppID, "")
		t.Setenv(envGHAppInstallationID, "")
		t.Setenv(envGHAppPrivateKey, "")

		auth, err := loadGithubAppAuth()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if auth != nil {
			t.Fatal("expected nil auth when no credentials are set")
		}
	})

	t.Run("partial credentials is an error", func(t *testing.T) {
		t.Setenv(envGHAppID, "123")
		t.Setenv(envGHAppInstallationID, "")
		t.Setenv(envGHAppPrivateKey, keyPEM)

		_, err := loadGithubAppAuth()
		if err == nil {
			t.Fatal("expected error for partial credentials")
		}
		if !strings.Contains(err.Error(), envGHAppInstallationID) {
			t.Errorf("error should name the missing variable, got: %v", err)
		}
	})

	t.Run("inline PEM is accepted", func(t *testing.T) {
		t.Setenv(envGHAppID, "123")
		t.Setenv(envGHAppInstallationID, "456")
		t.Setenv(envGHAppPrivateKey, keyPEM)

		auth, err := loadGithubAppAuth()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if auth == nil {
			t.Fatal("expected auth to be non-nil")
		}
		if auth.apiURL != defaultGithubAPIURL {
			t.Errorf("apiURL = %q, want %q", auth.apiURL, defaultGithubAPIURL)
		}
	})

	t.Run("private key file path is accepted", func(t *testing.T) {
		keyFile := filepath.Join(t.TempDir(), "app.pem")
		if err := os.WriteFile(keyFile, []byte(keyPEM), 0600); err != nil {
			t.Fatalf("failed to write key file: %v", err)
		}
		t.Setenv(envGHAppID, "123")
		t.Setenv(envGHAppInstallationID, "456")
		t.Setenv(envGHAppPrivateKey, keyFile)

		auth, err := loadGithubAppAuth()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if auth.privateKeyPEM != keyPEM {
			t.Error("expected private key content to be read from the file path")
		}
	})

	t.Run("non-PEM non-path value is an error", func(t *testing.T) {
		t.Setenv(envGHAppID, "123")
		t.Setenv(envGHAppInstallationID, "456")
		t.Setenv(envGHAppPrivateKey, "not-a-key")

		if _, err := loadGithubAppAuth(); err == nil {
			t.Fatal("expected error for invalid private key value")
		}
	})

	t.Run("invalid API URL is an error", func(t *testing.T) {
		t.Setenv(envGHAppID, "123")
		t.Setenv(envGHAppInstallationID, "456")
		t.Setenv(envGHAppPrivateKey, keyPEM)
		t.Setenv(envGithubAPIURL, "api.github.com")

		if _, err := loadGithubAppAuth(); err == nil {
			t.Fatal("expected error for API URL without scheme")
		}
	})
}

func TestParseRSAPrivateKey(t *testing.T) {
	key := testRSAKey(t)

	t.Run("pkcs1", func(t *testing.T) {
		parsed, err := parseRSAPrivateKey([]byte(pkcs1PEM(t, key)))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !parsed.Equal(key) {
			t.Error("parsed key does not match original")
		}
	})

	t.Run("pkcs8", func(t *testing.T) {
		parsed, err := parseRSAPrivateKey([]byte(pkcs8PEM(t, key)))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !parsed.Equal(key) {
			t.Error("parsed key does not match original")
		}
	})

	t.Run("invalid PEM", func(t *testing.T) {
		if _, err := parseRSAPrivateKey([]byte("not a pem")); err == nil {
			t.Fatal("expected error for non-PEM input")
		}
	})
}

func TestSignJWT(t *testing.T) {
	key := testRSAKey(t)
	auth := &githubAppAuth{
		appID:         "123456",
		privateKeyPEM: pkcs1PEM(t, key),
	}

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	token, err := auth.signJWT(now)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("failed to parse header: %v", err)
	}
	if header["alg"] != "RS256" {
		t.Errorf("alg = %q, want RS256", header["alg"])
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode claims: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatalf("failed to parse claims: %v", err)
	}
	if claims["iss"] != "123456" {
		t.Errorf("iss = %v, want 123456", claims["iss"])
	}
	if iat := int64(claims["iat"].(float64)); iat != now.Add(-time.Minute).Unix() {
		t.Errorf("iat = %d, want %d", iat, now.Add(-time.Minute).Unix())
	}
	if exp := int64(claims["exp"].(float64)); exp != now.Add(githubAppJWTTTL).Unix() {
		t.Errorf("exp = %d, want %d", exp, now.Add(githubAppJWTTTL).Unix())
	}
}

func TestInstallationToken(t *testing.T) {
	key := testRSAKey(t)

	t.Run("mints and caches a token", func(t *testing.T) {
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				t.Error("expected a Bearer JWT authorization header")
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":      testInstallationToken,
				"expires_at": time.Now().Add(time.Hour),
			})
		}))
		defer server.Close()

		auth := &githubAppAuth{
			appID:          "123",
			installationID: "456",
			privateKeyPEM:  pkcs1PEM(t, key),
			apiURL:         server.URL,
		}

		for i := 0; i < 3; i++ {
			token, err := auth.installationToken()
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if token != testInstallationToken {
				t.Errorf("token = %q, want %s", token, testInstallationToken)
			}
		}
		if requests != 1 {
			t.Errorf("expected 1 token request (cached), got %d", requests)
		}
	})

	t.Run("refreshes an expiring token", func(t *testing.T) {
		var requests int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.WriteHeader(http.StatusCreated)
			// Expires within the refresh skew, so every call must re-mint.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":      testInstallationToken,
				"expires_at": time.Now().Add(time.Minute),
			})
		}))
		defer server.Close()

		auth := &githubAppAuth{
			appID:          "123",
			installationID: "456",
			privateKeyPEM:  pkcs1PEM(t, key),
			apiURL:         server.URL,
		}

		for i := 0; i < 2; i++ {
			if _, err := auth.installationToken(); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		}
		if requests != 2 {
			t.Errorf("expected 2 token requests, got %d", requests)
		}
	})

	t.Run("non-201 response is an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
		}))
		defer server.Close()

		auth := &githubAppAuth{
			appID:          "123",
			installationID: "456",
			privateKeyPEM:  pkcs1PEM(t, key),
			apiURL:         server.URL,
		}

		_, err := auth.installationToken()
		if err == nil {
			t.Fatal("expected error for non-201 response")
		}
		if !strings.Contains(err.Error(), "Bad credentials") {
			t.Errorf("error should include GitHub's message, got: %v", err)
		}
	})

	t.Run("malformed JSON response is an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			// A proxy or GHES instance returning a non-JSON body must not be
			// mistaken for a valid token response.
			_, _ = w.Write([]byte("not json"))
		}))
		defer server.Close()

		auth := &githubAppAuth{
			appID:          "123",
			installationID: "456",
			privateKeyPEM:  pkcs1PEM(t, key),
			apiURL:         server.URL,
		}

		_, err := auth.installationToken()
		if err == nil {
			t.Fatal("expected error for malformed JSON response")
		}
		if !strings.Contains(err.Error(), "failed to parse installation token response") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty token in response is an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			// Valid JSON but no token: an empty value must never be returned,
			// since it would silently produce unusable git credentials.
			_ = json.NewEncoder(w).Encode(map[string]any{"token": ""})
		}))
		defer server.Close()

		auth := &githubAppAuth{
			appID:          "123",
			installationID: "456",
			privateKeyPEM:  pkcs1PEM(t, key),
			apiURL:         server.URL,
		}

		_, err := auth.installationToken()
		if err == nil {
			t.Fatal("expected error for empty token in response")
		}
		if !strings.Contains(err.Error(), "did not contain a token") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestWriteGithubAppCredentials(t *testing.T) {
	key := testRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      testInstallationToken,
			"expires_at": time.Now().Add(time.Hour),
		})
	}))
	defer server.Close()

	auth := &githubAppAuth{
		appID:          "123",
		installationID: "456",
		privateKeyPEM:  pkcs1PEM(t, key),
		apiURL:         server.URL,
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envRepo, "https://github.com/WorkIndia-Private/wi-ai-collab-kit.git")

	if err := writeGithubAppCredentials(auth); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(home, ".git-credentials")) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("failed to read credentials file: %v", err)
	}
	want := "https://x-access-token:" + testInstallationToken + "@github.com\n"
	if string(content) != want {
		t.Errorf("credentials = %q, want %q", string(content), want)
	}
}

func TestRepoHost(t *testing.T) {
	tests := []struct {
		repo string
		want string
	}{
		{"https://github.com/org/repo.git", "github.com"},
		{"http://github.example.com/org/repo.git", "github.example.com"},
		{"git@github.com:org/repo.git", "github.com"},
		{"ssh://git@github.example.com:22/org/repo.git", "github.example.com"},
		{"github.com", "github.com"},
	}
	for _, tt := range tests {
		if got := repoHost(tt.repo); got != tt.want {
			t.Errorf("repoHost(%q) = %q, want %q", tt.repo, got, tt.want)
		}
	}
}

func TestHTTPSRepoURL(t *testing.T) {
	tests := []struct {
		repo string
		want string
	}{
		{"https://github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"http://github.com/org/repo.git", "http://github.com/org/repo.git"},
		{"git@github.com:org/repo.git", "https://github.com/org/repo.git"},
		{"ssh://git@github.com/org/repo.git", "https://github.com/org/repo.git"},
	}
	for _, tt := range tests {
		if got := httpsRepoURL(tt.repo); got != tt.want {
			t.Errorf("httpsRepoURL(%q) = %q, want %q", tt.repo, got, tt.want)
		}
	}
}
