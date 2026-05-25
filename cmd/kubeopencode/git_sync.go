// Copyright Contributors to the KubeOpenCode project

package main

import (
	"context"
	"fmt"
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
)

// Default values for git-sync
const (
	defaultSyncInterval = 300 // 5 minutes in seconds
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
  GIT_SSH_KNOWN_HOSTS     Known hosts content for SSH verification`,
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
	if err := setupAuth(); err != nil {
		return fmt.Errorf("failed to setup authentication: %w", err)
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

	// Main sync loop
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Run first sync immediately
	syncOnce(targetDir, fetchRef)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("git-sync: Shutdown complete")
			return nil
		case <-ticker.C:
			syncOnce(targetDir, fetchRef)
		}
	}
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
