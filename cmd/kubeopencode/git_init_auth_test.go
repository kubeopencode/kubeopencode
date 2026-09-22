// Copyright Contributors to the KubeOpenCode project

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testAppToken is a fake installation token assembled at runtime so the
// hardcoded-credential linter does not flag a placeholder value.
var testAppToken = "ghs_" + "app_token"

// clearGitAuthEnv unsets all git authentication environment variables so each
// test starts from a known state.
func clearGitAuthEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		envUsername, envPassword, envSSHKey, envSSHHostKeys,
		envGHAppID, envGHAppInstallationID, envGHAppPrivateKey, envGithubAPIURL,
		envRepo,
	} {
		t.Setenv(name, "")
	}
}

func TestSetupAuth_GithubAppTakesPrecedence(t *testing.T) {
	clearGitAuthEnv(t)
	key := testRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      testAppToken,
			"expires_at": time.Now().Add(time.Hour),
		})
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envRepo, "https://github.com/org/repo.git")
	t.Setenv(envGHAppID, "123")
	t.Setenv(envGHAppInstallationID, "456")
	t.Setenv(envGHAppPrivateKey, pkcs1PEM(t, key))
	t.Setenv(envGithubAPIURL, server.URL)
	// Static credentials are also present; App auth must win.
	t.Setenv(envUsername, "static-user")
	t.Setenv(envPassword, "static-pass")

	appAuth, err := setupAuth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if appAuth == nil {
		t.Fatal("expected setupAuth to return the GitHub App auth")
	}

	content, readErr := os.ReadFile(filepath.Join(home, ".git-credentials")) //nolint:gosec // test file
	if readErr != nil {
		t.Fatalf("failed to read credentials file: %v", readErr)
	}
	if !strings.Contains(string(content), "x-access-token:"+testAppToken) {
		t.Errorf("expected GitHub App token in credentials, got: %s", string(content))
	}
	if strings.Contains(string(content), "static-user") {
		t.Error("static credentials should not be used when GitHub App credentials are present")
	}
}

func TestSetupAuth_GithubAppIncompleteIsError(t *testing.T) {
	clearGitAuthEnv(t)
	t.Setenv(envRepo, "https://github.com/org/repo.git")
	t.Setenv(envGHAppID, "123")

	if _, err := setupAuth(); err == nil {
		t.Fatal("expected error for incomplete GitHub App credentials")
	}
}

func TestSetupAuth_GithubAppMintFailurePropagates(t *testing.T) {
	clearGitAuthEnv(t)
	key := testRSAKey(t)

	// GitHub rejects the JWT; setupAuth must surface the failure rather than
	// fall back to an unauthenticated clone.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envRepo, "https://github.com/org/repo.git")
	t.Setenv(envGHAppID, "123")
	t.Setenv(envGHAppInstallationID, "456")
	t.Setenv(envGHAppPrivateKey, pkcs1PEM(t, key))
	t.Setenv(envGithubAPIURL, server.URL)

	appAuth, err := setupAuth()
	if err == nil {
		t.Fatal("expected setupAuth to fail when the installation token cannot be minted")
	}
	if appAuth != nil {
		t.Error("expected nil auth on failure")
	}
	if !strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("error should include GitHub's message, got: %v", err)
	}

	// No usable credential file should be left behind.
	if _, statErr := os.Stat(filepath.Join(home, ".git-credentials")); !os.IsNotExist(statErr) {
		t.Errorf("expected no credentials file after a failed mint, stat err = %v", statErr)
	}
}

func TestSetupAuth_StaticHTTPSStillWorks(t *testing.T) {
	clearGitAuthEnv(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envRepo, "https://github.com/org/repo.git")
	t.Setenv(envUsername, "static-user")
	t.Setenv(envPassword, "static-pass")

	appAuth, err := setupAuth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if appAuth != nil {
		t.Error("expected nil GitHub App auth for static credentials")
	}

	content, readErr := os.ReadFile(filepath.Join(home, ".git-credentials")) //nolint:gosec // test file
	if readErr != nil {
		t.Fatalf("failed to read credentials file: %v", readErr)
	}
	want := "https://static-user:static-pass@github.com\n"
	if string(content) != want {
		t.Errorf("credentials = %q, want %q", string(content), want)
	}
}

func TestCleanupCredentials_GithubApp(t *testing.T) {
	clearGitAuthEnv(t)
	key := testRSAKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      testAppToken,
			"expires_at": time.Now().Add(time.Hour),
		})
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envRepo, "https://github.com/org/repo.git")
	t.Setenv(envGHAppID, "123")
	t.Setenv(envGHAppInstallationID, "456")
	t.Setenv(envGHAppPrivateKey, pkcs1PEM(t, key))
	t.Setenv(envGithubAPIURL, server.URL)

	if _, err := setupAuth(); err != nil {
		t.Fatalf("setupAuth failed: %v", err)
	}

	credFile := filepath.Join(home, ".git-credentials")
	if _, err := os.Stat(credFile); err != nil {
		t.Fatalf("expected credentials file to exist: %v", err)
	}

	cleanupCredentials()

	if _, err := os.Stat(credFile); !os.IsNotExist(err) {
		t.Errorf("expected credentials file to be removed, stat err = %v", err)
	}
}
