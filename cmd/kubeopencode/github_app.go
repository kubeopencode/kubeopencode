// Copyright Contributors to the KubeOpenCode project

package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Environment variable names for GitHub App authentication.
//
// These are consumed by both git-init and git-sync. They are intentionally
// generic (not tied to any organization) so the same Secret can be reused for
// Git contexts and skill sources.
const (
	envGHAppID             = "GH_APP_ID"
	envGHAppInstallationID = "GH_APP_INSTALLATION_ID"
	envGHAppPrivateKey     = "GH_APP_PRIVATE_KEY"
	envGithubAPIURL        = "GITHUB_API_URL"
)

const (
	// defaultGithubAPIURL is the public GitHub REST API endpoint.
	defaultGithubAPIURL = "https://api.github.com"
	// githubAppJWTTTL is the lifetime of the signed JWT used to request an
	// installation token. GitHub rejects JWTs with an exp more than 10 minutes
	// in the future, so 9 minutes is used with a 60s backdated iat.
	githubAppJWTTTL = 9 * time.Minute
	// githubAppTokenSkew is how long before expiry a cached installation token
	// is refreshed, so long-running consumers (git-sync) never use a stale token.
	githubAppTokenSkew = 5 * time.Minute
	// githubAppHTTPTimeout bounds the token exchange request.
	githubAppHTTPTimeout = 30 * time.Second
	// githubAppMaxResponseBytes caps how much of the token response is read.
	githubAppMaxResponseBytes = 1 << 20
)

// githubAppAuth holds GitHub App credentials and caches the short-lived
// installation access token minted from them.
//
// Credential precedence in setupAuth: when these variables are present, GitHub
// App authentication is used and the static GIT_USERNAME/GIT_PASSWORD and
// GIT_SSH_KEY credentials are ignored.
type githubAppAuth struct {
	appID          string
	installationID string
	privateKeyPEM  string
	apiURL         string

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// loadGithubAppAuth reads GitHub App credentials from the environment.
//
// It returns (nil, nil) when no GitHub App credentials are configured, so
// callers can treat the absence as "use the static credential path". A
// partially-configured set of variables is an error: silently falling back to
// an unauthenticated clone would produce a confusing failure later.
//
// GH_APP_PRIVATE_KEY may contain either the PEM content itself or a path to a
// file containing it, mirroring the behavior of GIT_SSH_KEY.
func loadGithubAppAuth() (*githubAppAuth, error) {
	appID := os.Getenv(envGHAppID)
	installationID := os.Getenv(envGHAppInstallationID)
	privateKey := os.Getenv(envGHAppPrivateKey)

	if appID == "" && installationID == "" && privateKey == "" {
		return nil, nil
	}

	var missing []string
	if appID == "" {
		missing = append(missing, envGHAppID)
	}
	if installationID == "" {
		missing = append(missing, envGHAppInstallationID)
	}
	if privateKey == "" {
		missing = append(missing, envGHAppPrivateKey)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("incomplete GitHub App credentials, missing: %s", strings.Join(missing, ", "))
	}

	keyPEM, err := readPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	apiURL := getEnvOrDefault(envGithubAPIURL, defaultGithubAPIURL)
	if !strings.HasPrefix(apiURL, "https://") && !strings.HasPrefix(apiURL, "http://") {
		return nil, fmt.Errorf("%s must be an http(s) URL, got %q", envGithubAPIURL, apiURL)
	}

	return &githubAppAuth{
		appID:          appID,
		installationID: installationID,
		privateKeyPEM:  string(keyPEM),
		apiURL:         strings.TrimRight(apiURL, "/"),
	}, nil
}

// readPrivateKey resolves a private key value that may be either PEM content or
// a filesystem path to a PEM file.
func readPrivateKey(value string) ([]byte, error) {
	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		content, readErr := os.ReadFile(value) //nolint:gosec // path comes from a trusted env var
		if readErr != nil {
			return nil, fmt.Errorf("failed to read GitHub App private key from %s: %w", value, readErr)
		}
		return content, nil
	}
	if !strings.Contains(value, "-----BEGIN") {
		return nil, fmt.Errorf("%s is neither a PEM key nor a readable file path", envGHAppPrivateKey)
	}
	return []byte(value), nil
}

// installationToken returns a valid installation access token, minting a new
// one when the cached token is missing or close to expiry.
func (g *githubAppAuth) installationToken() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.token != "" && time.Now().Before(g.expiresAt.Add(-githubAppTokenSkew)) {
		return g.token, nil
	}

	token, expiresAt, err := g.mintInstallationToken()
	if err != nil {
		return "", err
	}
	g.token = token
	g.expiresAt = expiresAt
	return token, nil
}

// mintInstallationToken signs a JWT with the App private key and exchanges it
// for an installation access token.
func (g *githubAppAuth) mintInstallationToken() (string, time.Time, error) {
	jwt, err := g.signJWT(time.Now())
	if err != nil {
		return "", time.Time{}, err
	}

	endpoint := fmt.Sprintf("%s/app/installations/%s/access_tokens", g.apiURL, g.installationID)
	req, err := http.NewRequest(http.MethodPost, endpoint, nil) //nolint:gosec // endpoint is derived from a trusted env var
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to build installation token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: githubAppHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("installation token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, githubAppMaxResponseBytes))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to read installation token response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", time.Time{}, fmt.Errorf("installation token request returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse installation token response: %w", err)
	}
	if parsed.Token == "" {
		return "", time.Time{}, fmt.Errorf("installation token response did not contain a token")
	}

	expiresAt := parsed.ExpiresAt
	if expiresAt.IsZero() {
		// Defensive fallback: installation tokens are valid for one hour.
		expiresAt = time.Now().Add(time.Hour)
	}

	return parsed.Token, expiresAt, nil
}

// signJWT builds a GitHub App JWT (RS256) as described by:
// https://docs.github.com/rest/apps/apps#create-a-github-app-jwt
func (g *githubAppAuth) signJWT(now time.Time) (string, error) {
	key, err := parseRSAPrivateKey([]byte(g.privateKeyPEM))
	if err != nil {
		return "", err
	}

	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT header: %w", err)
	}

	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(githubAppJWTTTL).Unix(),
		"iss": g.appID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(claims)

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// parseRSAPrivateKey parses a PEM-encoded RSA private key in either PKCS#1
// ("RSA PRIVATE KEY") or PKCS#8 ("PRIVATE KEY") form.
func parseRSAPrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("GitHub App private key does not contain a PEM block")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub App private key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("GitHub App private key is not an RSA key")
	}
	return rsaKey, nil
}

// writeGithubAppCredentials writes the installation token into the git
// credential store so that `git clone`/`fetch` over HTTPS authenticates
// transparently.
func writeGithubAppCredentials(appAuth *githubAppAuth) error {
	token, err := appAuth.installationToken()
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}

	host := repoHost(os.Getenv(envRepo))
	if host == "" {
		host = "github.com"
	}

	credFile := filepath.Join(home, ".git-credentials")
	credContent := fmt.Sprintf("https://x-access-token:%s@%s\n", token, host)
	if err := os.WriteFile(credFile, []byte(credContent), 0600); err != nil {
		return fmt.Errorf("failed to write git credentials file: %w", err)
	}
	return nil
}

// repoHost extracts the hostname from a Git repository URL in any of the forms
// git-init accepts: https://, http://, ssh://, or scp-like git@host:owner/repo.
func repoHost(repo string) string {
	if i := strings.Index(repo, "://"); i >= 0 {
		rest := repo[i+len("://"):]
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if end := strings.IndexAny(rest, "/:"); end >= 0 {
			return rest[:end]
		}
		return rest
	}

	if at := strings.Index(repo, "@"); at >= 0 {
		rest := repo[at+1:]
		if end := strings.IndexAny(rest, ":/"); end >= 0 {
			return rest[:end]
		}
		return rest
	}

	return repo
}

// httpsRepoURL converts an SSH GitHub repository URL into its HTTPS equivalent,
// so that GitHub App installation tokens (which are HTTPS credentials) can be
// used regardless of how the repository URL was written. URLs that are already
// HTTP(S) are returned unchanged.
func httpsRepoURL(repo string) string {
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "http://") {
		return repo
	}

	host := repoHost(repo)
	var path string

	if rest, ok := strings.CutPrefix(repo, "ssh://"); ok {
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		if slash := strings.Index(rest, "/"); slash >= 0 {
			path = rest[slash+1:]
		}
	} else if colon := strings.Index(repo, ":"); colon >= 0 {
		path = repo[colon+1:]
	}

	if host == "" || path == "" {
		return repo
	}
	return "https://" + host + "/" + strings.TrimPrefix(path, "/")
}
