// Copyright Contributors to the KubeTask project

// git-init is a simple Git clone utility for KubeTask Git Context.
// It clones a Git repository to a specified directory, supporting:
// - Shallow clones (configurable depth)
// - Branch/tag/commit reference
// - HTTPS authentication (username/password)
// - SSH authentication (private key)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Environment variable names
const (
	envRepo        = "GIT_REPO"
	envRef         = "GIT_REF"
	envDepth       = "GIT_DEPTH"
	envRoot        = "GIT_ROOT"
	envLink        = "GIT_LINK"
	envUsername    = "GIT_USERNAME"
	envPassword    = "GIT_PASSWORD"
	envSSHKey      = "GIT_SSH_KEY"
	envSSHHostKeys = "GIT_SSH_KNOWN_HOSTS"
)

// Default values
const (
	defaultRef   = "HEAD"
	defaultDepth = 1
	defaultRoot  = "/git"
	defaultLink  = "repo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "git-init: error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Get required environment variable
	repo := os.Getenv(envRepo)
	if repo == "" {
		return fmt.Errorf("%s environment variable is required", envRepo)
	}

	// Validate repository URL protocol to prevent SSRF attacks
	if err := validateRepoURL(repo); err != nil {
		return err
	}

	// Get optional environment variables with defaults
	ref := getEnvOrDefault(envRef, defaultRef)
	depth := getEnvIntOrDefault(envDepth, defaultDepth)
	root := getEnvOrDefault(envRoot, defaultRoot)
	link := getEnvOrDefault(envLink, defaultLink)

	// Target directory
	targetDir := filepath.Join(root, link)

	fmt.Println("git-init: Cloning repository...")
	fmt.Printf("  Repository: %s\n", repo)
	fmt.Printf("  Ref: %s\n", ref)
	fmt.Printf("  Depth: %d\n", depth)
	fmt.Printf("  Target: %s\n", targetDir)

	// Setup authentication
	if err := setupAuth(); err != nil {
		return fmt.Errorf("failed to setup authentication: %w", err)
	}

	// Ensure root directory exists
	if err := os.MkdirAll(root, 0750); err != nil {
		return fmt.Errorf("failed to create root directory: %w", err)
	}

	// Build git clone command
	args := []string{"clone", "--depth", strconv.Itoa(depth), "--single-branch"}

	// Add branch flag if not HEAD
	if ref != "HEAD" {
		args = append(args, "--branch", ref)
	}

	args = append(args, repo, targetDir)

	// Execute git clone
	cmd := exec.Command("git", args...) //nolint:gosec // args are constructed from controlled inputs
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Verify clone was successful
	gitDir := filepath.Join(targetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("clone verification failed: .git directory not found")
	}

	// Create a shared .gitconfig in the target directory for safe.directory
	// This is needed because init containers run as a different user than the main container
	// Without this, git commands fail with "detected dubious ownership" error
	// We write to a shared location so the main container can use it
	sharedGitConfig := filepath.Join(root, ".gitconfig")
	// Use directory = * because the repository may be mounted at a different path
	// in the agent container (e.g., /workspace/project instead of /git/repo).
	// This is safe because:
	// - The .gitconfig is created inside the container and only affects this Pod
	// - Container isolation is the security boundary
	// - We cannot predict the final mount path from git-init
	gitConfigContent := fmt.Sprintf("[safe]\n\tdirectory = %s\n\tdirectory = *\n", targetDir)
	if err := os.WriteFile(sharedGitConfig, []byte(gitConfigContent), 0644); err != nil {
		fmt.Printf("git-init: Warning: could not write shared .gitconfig: %v\n", err)
	} else {
		fmt.Printf("git-init: Created shared .gitconfig at %s\n", sharedGitConfig)
	}

	// Make the cloned repository writable by all users in the container.
	// This is needed because the agent container may run as a different user.
	// Without this, file modifications fail with "permission denied" error.
	//
	// Security Note: This is acceptable because:
	// - Container provides process isolation (pod is the security boundary)
	// - Agent needs write access to develop/modify cloned code
	// - Alternative approaches (shared UID/fsGroup) reduce image flexibility
	fmt.Println("git-init: Setting repository permissions...")
	chmodCmd := exec.Command("chmod", "-R", "a+w", targetDir)
	if err := chmodCmd.Run(); err != nil {
		fmt.Printf("git-init: Warning: could not set permissions: %v\n", err)
	} else {
		fmt.Printf("git-init: Set write permissions for all users on %s\n", targetDir)
	}

	// Get and print commit hash
	commitCmd := exec.Command("git", "-C", targetDir, "rev-parse", "HEAD") //nolint:gosec // targetDir is constructed from controlled inputs
	commitOutput, err := commitCmd.Output()
	if err != nil {
		fmt.Println("git-init: Clone successful! (could not get commit hash)")
	} else {
		fmt.Printf("git-init: Clone successful!\n")
		fmt.Printf("  Commit: %s\n", strings.TrimSpace(string(commitOutput)))
	}

	// Clean up credentials file after successful clone (security best practice)
	cleanupCredentials()

	return nil
}

// cleanupCredentials removes the git credentials file after a successful clone.
// This reduces the window of exposure for credentials on disk.
func cleanupCredentials() {
	username := os.Getenv(envUsername)
	password := os.Getenv(envPassword)

	// Only clean up if HTTPS credentials were used
	if username != "" && password != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		credFile := filepath.Join(home, ".git-credentials")
		if err := os.Remove(credFile); err == nil {
			fmt.Println("git-init: Cleaned up credentials file")
		}
		// Ignore error - best effort cleanup, file may not exist
	}
}

func setupAuth() error {
	username := os.Getenv(envUsername)
	password := os.Getenv(envPassword)
	sshKey := os.Getenv(envSSHKey)

	// Configure HTTPS credentials
	if username != "" && password != "" {
		fmt.Println("git-init: Configuring HTTPS authentication...")

		// Use git credential helper
		if err := gitConfig("credential.helper", "store"); err != nil {
			return err
		}

		// Get home directory
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}

		// Write credentials file
		credFile := filepath.Join(home, ".git-credentials")
		// Extract host from repo URL
		repo := os.Getenv(envRepo)
		host := extractHost(repo)
		credContent := fmt.Sprintf("https://%s:%s@%s\n", username, password, host)

		if err := os.WriteFile(credFile, []byte(credContent), 0600); err != nil {
			return fmt.Errorf("failed to write credentials file: %w", err)
		}
	}

	// Configure SSH key
	if sshKey != "" {
		fmt.Println("git-init: Configuring SSH authentication...")

		// Get home directory
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}

		sshDir := filepath.Join(home, ".ssh")
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			return fmt.Errorf("failed to create .ssh directory: %w", err)
		}

		// Check if sshKey is a file path or content
		var keyContent []byte
		if _, err := os.Stat(sshKey); err == nil {
			// It's a file path
			keyContent, err = os.ReadFile(sshKey) //nolint:gosec // sshKey path is from trusted env var
			if err != nil {
				return fmt.Errorf("failed to read SSH key file: %w", err)
			}
		} else {
			// It's the key content itself
			keyContent = []byte(sshKey)
		}

		keyFile := filepath.Join(sshDir, "id_rsa")
		if err := os.WriteFile(keyFile, keyContent, 0600); err != nil {
			return fmt.Errorf("failed to write SSH key: %w", err)
		}

		// Write SSH config to disable strict host key checking
		configContent := "Host *\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null\n"

		// If known hosts are provided, use them instead
		knownHosts := os.Getenv(envSSHHostKeys)
		if knownHosts == "" {
			fmt.Println("git-init: WARNING: SSH host key verification disabled (no GIT_SSH_KNOWN_HOSTS provided)")
			fmt.Println("git-init: This allows MITM attacks. Consider providing known_hosts for production.")
		}
		if knownHosts != "" {
			knownHostsFile := filepath.Join(sshDir, "known_hosts")
			if err := os.WriteFile(knownHostsFile, []byte(knownHosts), 0600); err != nil {
				return fmt.Errorf("failed to write known_hosts: %w", err)
			}
			configContent = "Host *\n  StrictHostKeyChecking yes\n  UserKnownHostsFile " + knownHostsFile + "\n"
		}

		configFile := filepath.Join(sshDir, "config")
		if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
			return fmt.Errorf("failed to write SSH config: %w", err)
		}

		// Set GIT_SSH_COMMAND to use our key
		sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", keyFile)
		if err := os.Setenv("GIT_SSH_COMMAND", sshCmd); err != nil {
			return fmt.Errorf("failed to set GIT_SSH_COMMAND: %w", err)
		}
	}

	return nil
}

func gitConfig(key, value string) error {
	cmd := exec.Command("git", "config", "--global", key, value)
	return cmd.Run()
}

func extractHost(repoURL string) string {
	// Remove protocol prefix
	url := repoURL
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Get host (everything before first /)
	if idx := strings.Index(url, "/"); idx != -1 {
		return url[:idx]
	}
	return url
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// validateRepoURL validates the repository URL protocol to prevent SSRF attacks.
// Only allows https://, http://, and git@ (SSH) formats.
func validateRepoURL(repo string) error {
	// Allow https:// and git@ (SSH) formats
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") {
		return nil
	}
	// Allow http:// (some internal repos use it), but warn
	if strings.HasPrefix(repo, "http://") {
		fmt.Println("git-init: WARNING: Using insecure HTTP protocol")
		return nil
	}
	// Reject file://, ftp://, and other protocols
	return fmt.Errorf("unsupported repository URL protocol: only https://, http://, and git@ (SSH) are allowed")
}
