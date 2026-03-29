// Copyright Contributors to the KubeOpenCode project

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// Environment variable names for git-init
const (
	envRepo                 = "GIT_REPO"
	envRef                  = "GIT_REF"
	envDepth                = "GIT_DEPTH"
	envRoot                 = "GIT_ROOT"
	envLink                 = "GIT_LINK"
	envUsername             = "GIT_USERNAME"
	envPassword             = "GIT_PASSWORD"
	envSSHKey               = "GIT_SSH_KEY"
	envSSHHostKeys          = "GIT_SSH_KNOWN_HOSTS"
	envGitWorkspaceDir      = "GIT_WORKSPACE_DIR"
	envGitRepoSubpath       = "GIT_REPO_SUBPATH"
	envGitRecurseSubmodules = "GIT_RECURSE_SUBMODULES"
)

// Default values for git-init
const (
	defaultRef   = "HEAD"
	defaultDepth = 1
	defaultRoot  = "/git"
	defaultLink  = "repo"
)

func init() {
	rootCmd.AddCommand(gitInitCmd)
}

var gitInitCmd = &cobra.Command{
	Use:   "git-init",
	Short: "Clone Git repositories for Git Context",
	Long: `git-init clones a Git repository to a specified directory.

It supports:
  - Shallow clones (configurable depth)
  - Branch/tag/commit reference
  - HTTPS authentication (username/password)
  - SSH authentication (private key)

Environment variables:
  GIT_REPO            Repository URL (required)
  GIT_REF             Git reference (branch/tag/commit), default: HEAD
  GIT_DEPTH           Clone depth, default: 1
  GIT_ROOT            Root directory for clone, default: /git
  GIT_LINK            Subdirectory name, default: repo
  GIT_USERNAME        HTTPS username
  GIT_PASSWORD        HTTPS password/token
  GIT_SSH_KEY             SSH private key (content or file path)
  GIT_SSH_KNOWN_HOSTS     Known hosts content for SSH verification
  GIT_RECURSE_SUBMODULES  If "true", recursively clone submodules`,
	RunE: runGitInit,
}

func runGitInit(cmd *cobra.Command, args []string) error {
	// Setup custom CA certificate before any git operations
	if err := setupCustomCA(); err != nil {
		return fmt.Errorf("failed to setup custom CA: %w", err)
	}

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
	// Use 0755 to ensure accessibility in environments where containers run with
	// random UIDs (e.g., restricted security contexts). The random UID may rely
	// on group or others permissions to access directories.
	if err := os.MkdirAll(root, 0755); err != nil { //nolint:gosec // Needs group/others access for random UID environments
		return fmt.Errorf("failed to create root directory: %w", err)
	}

	// Check if submodule cloning is enabled
	recurseSubmodules := os.Getenv(envGitRecurseSubmodules) == "true"

	// Build git clone command
	cloneArgs := []string{"clone", "--depth", strconv.Itoa(depth), "--single-branch"}

	if recurseSubmodules {
		cloneArgs = append(cloneArgs, "--recurse-submodules")
		fmt.Println("  Submodules: recursive")
	}

	// Add branch flag if not HEAD
	if ref != "HEAD" {
		cloneArgs = append(cloneArgs, "--branch", ref)
	}

	cloneArgs = append(cloneArgs, repo, targetDir)

	// Execute git clone
	cloneCmd := exec.Command("git", cloneArgs...) //nolint:gosec // args are constructed from controlled inputs
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr

	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Verify clone was successful
	gitDir := filepath.Join(targetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("clone verification failed: .git directory not found")
	}

	// Create a shared .gitconfig in the target directory for safe.directory
	sharedGitConfig := filepath.Join(root, ".gitconfig")
	gitConfigContent := fmt.Sprintf("[safe]\n\tdirectory = %s\n\tdirectory = *\n", targetDir)
	if err := os.WriteFile(sharedGitConfig, []byte(gitConfigContent), 0644); err != nil { //nolint:gosec // Needs to be readable by other UIDs in multi-container pods
		fmt.Printf("git-init: Warning: could not write shared .gitconfig: %v\n", err)
	} else {
		fmt.Printf("git-init: Created shared .gitconfig at %s\n", sharedGitConfig)
	}

	// Make the cloned repository writable by all users in the container
	fmt.Println("git-init: Setting repository permissions...")
	chmodCmd := exec.Command("chmod", "-R", "a+w", targetDir) //nolint:gosec // targetDir is constructed from controlled env vars, not user input
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

	// If GIT_WORKSPACE_DIR is set, merge cloned content into the workspace directory.
	// This is used when a Git context has mountPath equal to workspaceDir (e.g., mountPath: ".").
	// Instead of overlaying the workspace with a separate volume mount (which would shadow
	// files like task.md written by context-init), we copy the repo content so both coexist.
	if wsDir := os.Getenv(envGitWorkspaceDir); wsDir != "" {
		sourceDir := targetDir
		if subpath := os.Getenv(envGitRepoSubpath); subpath != "" {
			sourceDir = filepath.Join(targetDir, subpath)
		}
		fmt.Printf("git-init: Merging repository content into workspace %s...\n", wsDir)
		cpCmd := exec.Command("cp", "-a", sourceDir+"/.", wsDir+"/") //nolint:gosec // paths are from controlled env vars
		cpCmd.Stdout = os.Stdout
		cpCmd.Stderr = os.Stderr
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("failed to merge repository into workspace: %w", err)
		}
		fmt.Println("git-init: Repository content merged into workspace successfully")
	}

	// Clean up credentials file after successful clone
	cleanupCredentials()

	return nil
}

func cleanupCredentials() {
	username := os.Getenv(envUsername)
	password := os.Getenv(envPassword)

	if username != "" && password != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}
		credFile := filepath.Join(home, ".git-credentials")
		if err := os.Remove(credFile); err == nil {
			fmt.Println("git-init: Cleaned up credentials file")
		}
	}
}

func setupAuth() error {
	username := os.Getenv(envUsername)
	password := os.Getenv(envPassword)
	sshKey := os.Getenv(envSSHKey)

	// Configure HTTPS credentials
	if username != "" && password != "" {
		fmt.Println("git-init: Configuring HTTPS authentication...")

		if err := gitConfig("credential.helper", "store"); err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}

		credFile := filepath.Join(home, ".git-credentials")
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

		home, err := os.UserHomeDir()
		if err != nil {
			home = "/tmp"
		}

		sshDir := filepath.Join(home, ".ssh")
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			return fmt.Errorf("failed to create .ssh directory: %w", err)
		}

		var keyContent []byte
		if _, err := os.Stat(sshKey); err == nil {
			keyContent, err = os.ReadFile(sshKey) //nolint:gosec // sshKey path is from trusted env var
			if err != nil {
				return fmt.Errorf("failed to read SSH key file: %w", err)
			}
		} else {
			keyContent = []byte(sshKey)
		}

		keyFile := filepath.Join(sshDir, "id_rsa")
		if err := os.WriteFile(keyFile, keyContent, 0600); err != nil {
			return fmt.Errorf("failed to write SSH key: %w", err)
		}

		configContent := "Host *\n  StrictHostKeyChecking no\n  UserKnownHostsFile /dev/null\n"

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

		sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", keyFile)
		if err := os.Setenv("GIT_SSH_COMMAND", sshCmd); err != nil {
			return fmt.Errorf("failed to set GIT_SSH_COMMAND: %w", err)
		}
	}

	return nil
}

// setupCustomCA configures a custom CA certificate for git HTTPS operations.
// It reads a custom CA cert from the path specified by CUSTOM_CA_CERT_PATH,
// concatenates it with the system CA bundle, and sets GIT_SSL_CAINFO to use
// the combined bundle. This enables git clone over HTTPS with self-signed or
// internal CA certificates.
func setupCustomCA() error {
	caPath := os.Getenv("CUSTOM_CA_CERT_PATH")
	if caPath == "" {
		return nil
	}

	fmt.Printf("git-init: Setting up custom CA certificate from %s\n", caPath)

	customCA, err := os.ReadFile(caPath) //nolint:gosec // path is from trusted env var
	if err != nil {
		return fmt.Errorf("failed to read custom CA certificate from %s: %w", caPath, err)
	}

	// Try to find the system CA bundle from well-known paths
	systemCAPaths := []string{
		"/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu
		"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/CentOS
		"/etc/ssl/cert.pem",                  // Alpine
		"/etc/ssl/ca-bundle.pem",             // OpenSUSE
	}

	var bundleContent []byte
	systemCAFound := false
	for _, sysPath := range systemCAPaths {
		data, err := os.ReadFile(sysPath) //nolint:gosec // paths are hardcoded system CA locations
		if err == nil {
			fmt.Printf("git-init: Found system CA bundle at %s\n", sysPath)
			bundleContent = make([]byte, len(data), len(data)+1+len(customCA))
			copy(bundleContent, data)
			bundleContent = append(bundleContent, '\n')
			systemCAFound = true
			break
		}
	}

	if !systemCAFound {
		fmt.Println("git-init: No system CA bundle found, using custom CA certificate only")
	}

	// Append custom CA to the bundle
	bundleContent = append(bundleContent, customCA...)

	bundlePath := "/tmp/ca-bundle.crt"
	if err := os.WriteFile(bundlePath, bundleContent, 0644); err != nil { //nolint:gosec // CA bundle needs to be readable
		return fmt.Errorf("failed to write combined CA bundle to %s: %w", bundlePath, err)
	}

	if err := os.Setenv("GIT_SSL_CAINFO", bundlePath); err != nil {
		return fmt.Errorf("failed to set GIT_SSL_CAINFO: %w", err)
	}

	fmt.Printf("git-init: GIT_SSL_CAINFO set to %s\n", bundlePath)
	return nil
}

func gitConfig(key, value string) error {
	cmd := exec.Command("git", "config", "--global", key, value)
	return cmd.Run()
}

func extractHost(repoURL string) string {
	url := repoURL
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	if idx := strings.Index(url, "/"); idx != -1 {
		return url[:idx]
	}
	return url
}

func validateRepoURL(repo string) error {
	if strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") {
		return nil
	}
	if strings.HasPrefix(repo, "http://") {
		fmt.Println("git-init: WARNING: Using insecure HTTP protocol")
		return nil
	}
	return fmt.Errorf("unsupported repository URL protocol: only https://, http://, and git@ (SSH) are allowed")
}
