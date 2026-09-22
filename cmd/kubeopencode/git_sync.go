// Copyright Contributors to the KubeOpenCode project

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Environment variable names for git-sync (unique to git-sync)
const (
	envSyncInterval = "GIT_SYNC_INTERVAL"

	// envReloadURL, when set, is the base URL of the OpenCode server that should
	// be asked to re-scan its configuration after a content update lands.
	// Only set on sidecars for mounts that request a reload (skills, or Git
	// contexts with sync.reload enabled).
	envReloadURL = "OPENCODE_RELOAD_URL"
)

// Default values for git-sync
const (
	defaultSyncInterval = 300 // 5 minutes in seconds

	// sessionStatusPath reports per-session activity. A session that is
	// currently generating has type "busy"; an idle server returns an empty map.
	sessionStatusPath = "/session/status"

	// instanceDisposePath releases the server's current instance, causing it to
	// re-scan configuration and content on the next request. This is what makes
	// updated skills visible without restarting the Pod.
	instanceDisposePath = "/instance/dispose"

	// reloadRequestTimeout bounds each reload-related HTTP call so a wedged
	// server can never block the sync loop indefinitely.
	reloadRequestTimeout = 10 * time.Second
)

func init() {
	rootCmd.AddCommand(gitSyncCmd)
}

var gitSyncCmd = &cobra.Command{
	Use:   "git-sync",
	Short: "Periodically sync a Git repository (sidecar mode)",
	Long: `git-sync runs as a long-lived sidecar container that periodically
fetches and updates a previously cloned Git repository.

It is designed to work alongside git-init: the init container clones
the repository, and git-sync keeps it up-to-date by polling the remote.

Environment variables:
  GIT_REPO            Repository URL (required)
  GIT_REF             Git reference (branch/tag) to track, default: HEAD
  GIT_ROOT            Root directory for clone, default: /git
  GIT_LINK            Subdirectory name, default: repo
  GIT_SYNC_INTERVAL   Polling interval in seconds, default: 300
  GIT_USERNAME        HTTPS username
  GIT_PASSWORD        HTTPS password/token
  GIT_SSH_KEY             SSH private key (content or file path)
  GIT_SSH_KNOWN_HOSTS     Known hosts content for SSH verification

GitHub App authentication (takes precedence over the credentials above):
  GH_APP_ID               GitHub App ID
  GH_APP_INSTALLATION_ID  GitHub App installation ID
  GH_APP_PRIVATE_KEY      App private key PEM content or path to a PEM file
  GITHUB_API_URL          GitHub API base URL, default: https://api.github.com

  Installation access tokens expire after one hour, so a fresh token is minted
  before each sync cycle.

Reload (optional):
  OPENCODE_RELOAD_URL  Base URL of the OpenCode server to notify after a change
                       (e.g. http://127.0.0.1:4096). When set, the sidecar asks
                       the server to re-scan so the update takes effect without a
                       Pod restart. The reload is deferred while any session is
                       busy, and retried on the next cycle.`,
	RunE: runGitSync,
}

func runGitSync(cmd *cobra.Command, args []string) error {
	// Setup custom CA certificate before any git operations
	if err := setupCustomCA(); err != nil {
		return fmt.Errorf("failed to setup custom CA: %w", err)
	}

	// Get required environment variable
	repo := os.Getenv(envRepo)
	if repo == "" {
		return fmt.Errorf("%s environment variable is required", envRepo)
	}

	if err := validateRepoURL(repo); err != nil {
		return err
	}

	// Setup authentication (persistent — no cleanup for sidecar)
	appAuth, err := setupAuth()
	if err != nil {
		return fmt.Errorf("failed to setup authentication: %w", err)
	}

	// GitHub App installation tokens are HTTPS credentials, and the repository
	// may have been written as an SSH URL.
	if appAuth != nil {
		repo = httpsRepoURL(repo)
	}

	// Get configuration
	ref := getEnvOrDefault(envRef, defaultRef)
	root := getEnvOrDefault(envRoot, defaultRoot)
	link := getEnvOrDefault(envLink, defaultLink)
	interval := getEnvIntOrDefault(envSyncInterval, defaultSyncInterval)

	targetDir := filepath.Join(root, link)

	fmt.Println("git-sync: Starting periodic sync...")
	fmt.Printf("  Repository: %s\n", repo)
	fmt.Printf("  Ref: %s\n", ref)
	fmt.Printf("  Target: %s\n", targetDir)
	fmt.Printf("  Interval: %ds\n", interval)

	// Verify the clone exists (created by git-init)
	gitDir := filepath.Join(targetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("git-sync: repository not found at %s — git-init must run first", targetDir)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		fmt.Printf("git-sync: Received signal %v, shutting down...\n", sig)
		cancel()
	}()

	// Determine the remote ref to fetch
	fetchRef := ref
	if fetchRef == "HEAD" {
		// When tracking HEAD, fetch the default branch
		fetchRef = ""
	}

	// refreshCredentials rewrites the credential file before each sync so that
	// GitHub App installation tokens (which expire after one hour) stay valid
	// for the lifetime of this long-running sidecar.
	refreshCredentials := func() {
		if appAuth == nil {
			return
		}
		if err := writeGithubAppCredentials(appAuth); err != nil {
			fmt.Printf("git-sync: Warning: could not refresh GitHub App credentials: %v\n", err)
		}
	}

	// reloadURL is set only for mounts that need the server to re-scan after an
	// update (skill sources, or Git contexts with sync.reload). Without it, this
	// sidecar behaves exactly as before: files update, no server interaction.
	reloadURL := strings.TrimRight(os.Getenv(envReloadURL), "/")
	if reloadURL != "" {
		fmt.Printf("  Reload: %s (deferred while sessions are busy)\n", reloadURL)
	}

	// The server remembers nothing about what it has scanned, and this sidecar
	// can restart at any time, so the "server is behind the working tree"
	// condition is persisted on the shared volume as the commit hash the server
	// last re-scanned. Comparing it against the local HEAD each cycle means an
	// owed reload survives a sidecar restart instead of waiting for the next
	// commit to the repository.
	stateFile := filepath.Join(root, "."+link+".reload-state")

	// On a fresh Pod the server scanned the git-init clone at startup, so the
	// current HEAD is already current. Seed the state to avoid a needless
	// reload right after boot. Recorded before the first sync so a repository
	// that is already ahead still registers as needing a reload.
	if reloadURL != "" {
		if _, ok := readReloadState(stateFile); !ok {
			if h, err := gitRevParse(targetDir, "HEAD"); err == nil {
				if err := writeReloadState(stateFile, h); err != nil {
					fmt.Printf("git-sync: Warning: could not seed reload state: %v\n", err)
				}
			}
		}
	}

	// runSync performs one sync cycle and reloads the server if it is behind.
	runSync := func() {
		syncOnce(targetDir, fetchRef)

		if reloadURL == "" {
			return
		}

		localHash, err := gitRevParse(targetDir, "HEAD")
		if err != nil {
			fmt.Printf("git-sync: Warning: could not read local HEAD for reload check: %v\n", err)
			return
		}
		// Already current: either nothing changed, or the reload succeeded on a
		// previous cycle. Checked outside the change detection so a reload
		// deferred for a busy server is retried even without a new commit.
		if scanned, _ := readReloadState(stateFile); scanned == localHash {
			return
		}

		if err := reloadOpenCodeServer(reloadURL); err != nil {
			fmt.Printf("git-sync: Reload deferred: %v\n", err)
			return
		}
		fmt.Println("git-sync: Server re-scanned updated content")
		if err := writeReloadState(stateFile, localHash); err != nil {
			fmt.Printf("git-sync: Warning: could not persist reload state: %v\n", err)
		}
	}

	// Main sync loop
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Run first sync immediately
	refreshCredentials()
	runSync()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("git-sync: Shutdown complete")
			return nil
		case <-ticker.C:
			refreshCredentials()
			runSync()
		}
	}
}

// reloadOpenCodeServer asks the OpenCode server to re-scan its configuration.
//
// The server caches some content at instance start (notably skills, which are
// discovered once). Updating files on disk is therefore invisible to a running
// server until its instance is disposed and rebuilt.
//
// Disposal releases the whole instance, so it must not run while a session is
// generating — that would abort the in-flight turn. This function therefore
// refuses to reload while any session is busy, returning an error that the
// caller treats as "retry next cycle" rather than a failure.
func reloadOpenCodeServer(baseURL string) error {
	client := &http.Client{Timeout: reloadRequestTimeout}

	busy, err := anySessionBusy(client, baseURL)
	if err != nil {
		return fmt.Errorf("could not determine session activity: %w", err)
	}
	if busy {
		return fmt.Errorf("a session is active; will retry next cycle")
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+instanceDisposePath, nil)
	if err != nil {
		return fmt.Errorf("building reload request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("reload request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("reload returned status %d", resp.StatusCode)
	}
	return nil
}

// anySessionBusy reports whether any session on the server is currently
// generating. The status endpoint returns a map of session ID to state, where a
// busy session has type "busy". An idle server returns an empty map.
func anySessionBusy(client *http.Client, baseURL string) (bool, error) {
	resp, err := client.Get(baseURL + sessionStatusPath) //nolint:gosec // URL from controlled env var
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("session status returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}

	var statuses map[string]struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &statuses); err != nil {
		// An unrecognized payload is treated as "busy" (fail safe) so an API
		// change cannot cause us to abort someone's work in progress.
		return true, fmt.Errorf("unexpected session status payload: %w", err)
	}

	for _, s := range statuses {
		if s.Type == "busy" {
			return true, nil
		}
	}
	return false, nil
}

// syncOnce performs a single sync cycle: fetch, compare, and update if needed.
func syncOnce(targetDir, fetchRef string) {
	// Get current local HEAD
	localHash, err := gitRevParse(targetDir, "HEAD")
	if err != nil {
		fmt.Printf("git-sync: Warning: could not get local HEAD: %v\n", err)
		return
	}

	// Fetch from remote
	fetchArgs := []string{"-C", targetDir, "fetch", "origin"}
	if fetchRef != "" {
		fetchArgs = append(fetchArgs, fetchRef)
	}
	fetchCmd := exec.Command("git", fetchArgs...) //nolint:gosec // args from controlled env vars
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		fmt.Printf("git-sync: Warning: fetch failed: %v\n", err)
		return
	}

	remoteRef := "FETCH_HEAD"

	// Get remote HEAD
	remoteHash, err := gitRevParse(targetDir, remoteRef)
	if err != nil {
		fmt.Printf("git-sync: Warning: could not resolve remote ref %s: %v\n", remoteRef, err)
		return
	}

	if localHash == remoteHash {
		fmt.Printf("git-sync: Up-to-date at %s\n", safeHash(localHash))
		return
	}

	// Update to new commit
	fmt.Printf("git-sync: Updating %s → %s\n", safeHash(localHash), safeHash(remoteHash))

	resetCmd := exec.Command("git", "-C", targetDir, "reset", "--hard", remoteRef) //nolint:gosec // controlled ref
	resetCmd.Stdout = os.Stdout
	resetCmd.Stderr = os.Stderr
	if err := resetCmd.Run(); err != nil {
		fmt.Printf("git-sync: Warning: reset failed: %v\n", err)
		return
	}

	// Ensure write permissions for all users (random UID environments)
	chmodCmd := exec.Command("chmod", "-R", "a+w", targetDir) //nolint:gosec // controlled path
	if err := chmodCmd.Run(); err != nil {
		fmt.Printf("git-sync: Warning: could not set permissions: %v\n", err)
	}

	fmt.Printf("git-sync: Successfully updated to %s\n", safeHash(remoteHash))
}

// readReloadState returns the commit hash the server last re-scanned. The second
// return value is false when no state has been recorded yet.
func readReloadState(path string) (string, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // controlled path on the shared volume
	if err != nil {
		return "", false
	}
	hash := strings.TrimSpace(string(data))
	if hash == "" {
		return "", false
	}
	return hash, true
}

// writeReloadState records the commit hash the server has most recently scanned.
//
// The file is made world-writable because containers may run as a random UID
// (SCC environments) and a restarted sidecar can therefore inherit a different
// UID than the one that created the file.
func writeReloadState(path, hash string) error {
	if err := os.WriteFile(path, []byte(hash+"\n"), 0o666); err != nil { //nolint:gosec // not sensitive
		return err
	}
	if err := os.Chmod(path, 0o666); err != nil { //nolint:gosec // not sensitive
		return err
	}
	return nil
}

// gitRevParse runs git rev-parse and returns the hash.
func gitRevParse(targetDir, ref string) (string, error) {
	cmd := exec.Command("git", "-C", targetDir, "rev-parse", ref) //nolint:gosec // controlled inputs
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// safeHash truncates a hash for display, avoiding index out of bounds.
func safeHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
