// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// agentConfig holds the resolved configuration from Agent or AgentTemplate
type agentConfig struct {
	agentImage         string   // OpenCode init container image (copies binary to /tools)
	executorImage      string   // Worker container image for task execution
	attachImage        string   // Lightweight image for --attach Pods
	command            []string // Command for agent container (optional, has default)
	workspaceDir       string
	contexts           []kubeopenv1alpha1.ContextItem
	skills             []kubeopenv1alpha1.SkillSource
	config             *string // OpenCode config JSON string
	credentials        []kubeopenv1alpha1.Credential
	podSpec            *kubeopenv1alpha1.AgentPodSpec
	serviceAccountName string
	maxConcurrentTasks *int32
	quota              *kubeopenv1alpha1.QuotaConfig
	caBundle           *kubeopenv1alpha1.CABundleConfig    // Custom CA bundle configuration (nil = no custom CA)
	proxy              *kubeopenv1alpha1.ProxyConfig       // HTTP/HTTPS proxy configuration (nil = no proxy)
	imagePullSecrets   []corev1.LocalObjectReference       // Image pull secrets for private registries
	port               int32                               // Server port (default 4096)
	extraPorts         []kubeopenv1alpha1.ExtraPort        // Additional ports to expose on Service/Deployment
	persistence        *kubeopenv1alpha1.PersistenceConfig // Persistence configuration
	suspend            bool                                // Whether Agent is suspended
	serverReady        bool                                // Whether Agent server is ready (from status)
}

// ResolveAgentConfig extracts configuration from the Agent spec.
func ResolveAgentConfig(agent *kubeopenv1alpha1.Agent) agentConfig {
	return agentConfig{
		agentImage:         defaultString(agent.Spec.AgentImage, DefaultAgentImage),
		executorImage:      defaultString(agent.Spec.ExecutorImage, DefaultExecutorImage),
		attachImage:        defaultString(agent.Spec.AttachImage, DefaultAttachImage),
		command:            agent.Spec.Command,
		workspaceDir:       agent.Spec.WorkspaceDir,
		contexts:           agent.Spec.Contexts,
		skills:             agent.Spec.Skills,
		config:             agent.Spec.Config,
		credentials:        agent.Spec.Credentials,
		podSpec:            agent.Spec.PodSpec,
		serviceAccountName: agent.Spec.ServiceAccountName,
		maxConcurrentTasks: agent.Spec.MaxConcurrentTasks,
		quota:              agent.Spec.Quota,
		caBundle:           agent.Spec.CABundle,
		proxy:              agent.Spec.Proxy,
		imagePullSecrets:   agent.Spec.ImagePullSecrets,
		port:               agent.Spec.Port,
		extraPorts:         agent.Spec.ExtraPorts,
		persistence:        agent.Spec.Persistence,
		suspend:            agent.Spec.Suspend,
		serverReady:        agent.Status.Ready,
	}
}

// ResolveTemplateToConfig extracts configuration from an AgentTemplate spec
// for use with templateRef-based Tasks (ephemeral Pods).
// Note: maxConcurrentTasks and quota are intentionally NOT populated because
// templateRef tasks have no persistent Agent to enforce limits against.
// port, persistence, and suspend are also not applicable for ephemeral Pods.
func ResolveTemplateToConfig(tmpl *kubeopenv1alpha1.AgentTemplate) agentConfig {
	return agentConfig{
		agentImage:         defaultString(tmpl.Spec.AgentImage, DefaultAgentImage),
		executorImage:      defaultString(tmpl.Spec.ExecutorImage, DefaultExecutorImage),
		attachImage:        defaultString(tmpl.Spec.AttachImage, DefaultAttachImage),
		command:            tmpl.Spec.Command,
		workspaceDir:       tmpl.Spec.WorkspaceDir,
		contexts:           tmpl.Spec.Contexts,
		skills:             tmpl.Spec.Skills,
		config:             tmpl.Spec.Config,
		credentials:        tmpl.Spec.Credentials,
		podSpec:            tmpl.Spec.PodSpec,
		serviceAccountName: tmpl.Spec.ServiceAccountName,
		caBundle:           tmpl.Spec.CABundle,
		proxy:              tmpl.Spec.Proxy,
		imagePullSecrets:   tmpl.Spec.ImagePullSecrets,
		extraPorts:         tmpl.Spec.ExtraPorts,
	}
}

// systemConfig holds resolved system-level configuration from KubeOpenCodeConfig.
// This configures internal KubeOpenCode components (git-init, context-init).
type systemConfig struct {
	// systemImage is the container image for internal KubeOpenCode components.
	// Defaults to DefaultKubeOpenCodeImage if not specified.
	systemImage string
	// systemImagePullPolicy is the image pull policy for system containers.
	// Defaults to IfNotPresent if not specified.
	systemImagePullPolicy corev1.PullPolicy
	// proxy is the cluster-wide proxy configuration from KubeOpenCodeConfig.
	// Agent-level proxy takes precedence over this.
	proxy *kubeopenv1alpha1.ProxyConfig
}

// applySystemDefaults merges cluster-level configuration from KubeOpenCodeConfig
// into the agent config where the Agent doesn't specify its own values.
// Agent-level settings always take precedence over cluster-level.
func (c *agentConfig) applySystemDefaults(sys systemConfig) {
	if c.proxy == nil && sys.proxy != nil {
		c.proxy = sys.proxy
	}
}

// fileMount represents a file to be mounted at a specific path
type fileMount struct {
	filePath string
	fileMode *int32 // Optional file permission mode (e.g., 0755 for executable)
}

// dirMount represents a directory to be mounted from a ConfigMap
type dirMount struct {
	dirPath       string
	configMapName string
	optional      bool
}

// gitMount represents a Git repository to be cloned and mounted
type gitMount struct {
	contextName       string // Context name (for volume naming)
	repository        string // Git repository URL
	ref               string // Git reference (branch, tag, or commit SHA)
	repoPath          string // Path within the repository to mount
	mountPath         string // Where to mount in the container
	depth             int    // Clone depth (1 = shallow, 0 = full)
	secretName        string // Optional secret name for authentication
	recurseSubmodules bool   // Whether to recursively clone submodules

	// Skill filtering: when set, only these named subdirectories under repoPath
	// should be visible at mountPath (one SubPath mount per name).
	// Used to prevent agents from discovering unselected skills.
	names []string

	// Sync fields (only effective for Agent contexts)
	syncEnabled  bool                           // Whether auto-sync is enabled
	syncPolicy   kubeopenv1alpha1.GitSyncPolicy // HotReload or Rollout
	syncInterval time.Duration                  // Polling interval
}

// resolvedContext holds a resolved context with its content and metadata
type resolvedContext struct {
	name      string // Context name (for XML tag)
	namespace string // Context namespace (for XML tag)
	ctxType   string // Context type (for XML tag)
	content   string // Resolved content
	mountPath string // Mount path (empty = append to task.md)
	fileMode  *int32 // Optional file permission mode (e.g., 0755 for executable)
}

// sanitizeConfigMapKey converts a file path to a valid ConfigMap key.
// ConfigMap keys must be alphanumeric, '-', '_', or '.'.
func sanitizeConfigMapKey(filePath string) string {
	// Remove leading slash and replace remaining slashes with dashes
	key := strings.TrimPrefix(filePath, "/")
	key = strings.ReplaceAll(key, "/", "-")
	return key
}

// getParentDir returns the parent directory of a file path.
// For "/etc/github-app/script.sh", it returns "/etc/github-app".
func getParentDir(filePath string) string {
	lastSlash := strings.LastIndex(filePath, "/")
	if lastSlash <= 0 {
		return "/"
	}
	return filePath[:lastSlash]
}

// isUnderPath checks if filePath is under basePath.
// For example, "/workspace/task.md" is under "/workspace".
func isUnderPath(filePath, basePath string) bool {
	// Normalize paths to ensure consistent comparison
	basePath = strings.TrimSuffix(basePath, "/")
	return filePath == basePath || strings.HasPrefix(filePath, basePath+"/")
}

// sanitizeVolumeName converts a directory path to a valid Kubernetes volume name.
// Volume names must be lowercase alphanumeric, '-', '.', max 63 chars.
func sanitizeVolumeName(dirPath string) string {
	// Remove leading slash and replace slashes with dashes
	name := strings.TrimPrefix(dirPath, "/")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ToLower(name)
	// Prepend "ctx-" to make it clear this is a context volume
	name = "ctx-" + name
	// Truncate to 63 chars max
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// boolPtr returns a pointer to the given bool value
func boolPtr(b bool) *bool {
	return &b
}

// defaultString returns the first string if it's not empty, otherwise the second one.
func defaultString(val, defaultVal string) string {
	if val == "" {
		return defaultVal
	}
	return val
}

const (
	// DefaultAgentImage is the default OpenCode init container image.
	// This image copies the OpenCode binary to /tools volume.
	DefaultAgentImage = "ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest"

	// DefaultExecutorImage is the default worker container image for task execution.
	// This is the development environment where tasks actually run.
	DefaultExecutorImage = "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest"

	// DefaultAttachImage is the lightweight image for Server-mode --attach Pods.
	// This minimal image (~25MB) contains only the OpenCode binary + shell + CA certs.
	// Used when Tasks connect to a persistent OpenCode server via --attach flag.
	DefaultAttachImage = "ghcr.io/kubeopencode/kubeopencode-agent-attach:latest"

	// DefaultKubeOpenCodeImage is the default kubeopencode container image.
	// This unified image provides: controller, git-init (Git clone), etc.
	DefaultKubeOpenCodeImage = "ghcr.io/kubeopencode/kubeopencode:latest"

	// ToolsVolumeName is the volume name for sharing OpenCode binary between containers
	ToolsVolumeName = "tools"

	// WorkspaceVolumeName is the volume name for the writable workspace
	WorkspaceVolumeName = "workspace"

	// ToolsMountPath is the mount path for the tools volume
	ToolsMountPath = "/tools"

	// OpenCodeSymlinkCmd creates a symlink so that the OpenCode binary is discoverable
	// from interactive terminals (e.g. VS Code) without requiring /tools in PATH.
	// Uses "|| true" to gracefully handle read-only root filesystems.
	OpenCodeSymlinkCmd = "ln -sf /tools/opencode /usr/local/bin/opencode 2>/dev/null || true"

	// OpenCodeConfigPath is the path where OpenCode config is written
	OpenCodeConfigPath = "/tools/opencode.json"

	// OpenCodeConfigEnvVar is the environment variable name for OpenCode config path
	OpenCodeConfigEnvVar = "OPENCODE_CONFIG"

	// OpenCodeConfigContentEnvVar is the environment variable for injecting config content
	// This is used to inject instructions for loading context files without conflicting
	// with repository's AGENTS.md. OpenCode merges OPENCODE_CONFIG_CONTENT with OPENCODE_CONFIG.
	OpenCodeConfigContentEnvVar = "OPENCODE_CONFIG_CONTENT"

	// OpenCodePermissionEnvVar is the environment variable for OpenCode permission configuration.
	// This allows overriding permission settings to enable non-interactive/automated mode.
	// The value is a JSON object mapping tool names to permission actions (allow/ask/deny).
	OpenCodePermissionEnvVar = "OPENCODE_PERMISSION"

	// DefaultOpenCodePermission is the default permission configuration for automated execution.
	// In Kubernetes/CI environments, we need to allow all permissions to avoid interactive prompts
	// that would block task execution. Users can still restrict permissions via Agent.spec.config.
	//
	// The value must be valid JSON since OpenCode parses it with JSON.parse().
	// {"*":"allow"} sets all tools to "allow" mode, enabling full autonomous operation.
	// For restricted permissions, users should configure them in Agent.spec.config's permission field.
	DefaultOpenCodePermission = `{"*":"allow"}`

	// ContextFileRelPath is the relative path (from workspaceDir) for KubeOpenCode context file.
	// This path is chosen to avoid conflicts with repository's AGENTS.md or CLAUDE.md files.
	// OpenCode loads this file via the instructions config injected through OPENCODE_CONFIG_CONTENT.
	ContextFileRelPath = ".kubeopencode/context.md"

	// DefaultMemoryLimit is the default memory limit for agent containers.
	// AI coding agents can consume unbounded memory during execution,
	// which risks triggering system-level OOM and taking down the entire node.
	// This default ensures OOM kills only the container, not the node.
	DefaultMemoryLimit = "4Gi"

	// DefaultMemoryRequest is the default memory request for agent containers.
	DefaultMemoryRequest = "512Mi"

	// DefaultSecretFileMode is the default permission mode for mounted secrets.
	// 0600 gives read/write access to the owner only.
	DefaultSecretFileMode int32 = 0600

	// DefaultGitRef is the default Git reference to clone
	DefaultGitRef = "HEAD"

	// DefaultGitDepth is the default Git clone depth
	DefaultGitDepth = 1

	// DefaultGitRoot is the root directory for Git clones in init containers
	DefaultGitRoot = "/git"

	// DefaultGitLink is the default subdirectory name for Git clones
	DefaultGitLink = "repo"

	// DefaultHomeDir is the default HOME directory for SCC compatibility
	DefaultHomeDir = "/tmp"

	// DefaultShell is the default SHELL for SCC compatibility
	DefaultShell = "/bin/bash"

	// CABundleVolumeName is the volume name for custom CA certificate bundle
	CABundleVolumeName = "ca-bundle"

	// CABundleMountPath is the mount path for the custom CA bundle volume
	CABundleMountPath = "/etc/ssl/certs/custom-ca"

	// CABundleFileName is the projected filename for the CA certificate inside the volume
	CABundleFileName = "tls.crt"

	// CustomCACertEnvVar is the environment variable pointing to the CA certificate file path
	CustomCACertEnvVar = "CUSTOM_CA_CERT_PATH"

	// DefaultCABundleConfigMapKey is the default key for CA bundles stored in ConfigMaps.
	// Compatible with cert-manager trust-manager Bundle resources.
	DefaultCABundleConfigMapKey = "ca-bundle.crt"

	// DefaultCABundleSecretKey is the default key for CA bundles stored in Secrets
	DefaultCABundleSecretKey = "ca.crt"
)

// inferImagePullPolicy returns Always for images with :latest tag or no tag
// (Kubernetes defaults untagged images to :latest), and IfNotPresent for
// images with a specific version tag or digest reference (@sha256:...).
func inferImagePullPolicy(image string) corev1.PullPolicy {
	// Digest references (e.g. image@sha256:abc) are immutable — always IfNotPresent
	if strings.Contains(image, "@") {
		return corev1.PullIfNotPresent
	}
	// Extract the tag after the last colon (skip port-like colons by checking for '/')
	tag := ""
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		candidate := image[idx+1:]
		// If there's a '/' in the candidate, it's a port separator, not a tag
		if !strings.Contains(candidate, "/") {
			tag = candidate
		}
	}
	// No tag or explicitly "latest" → Always pull
	if tag == "" || tag == "latest" {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}

// buildOpenCodeInitContainer creates an init container that copies OpenCode binary to /tools.
// This enables the two-container pattern where:
// - Init container (agentImage): Contains OpenCode, copies it to /tools
// - Worker container (executorImage): Uses /tools/opencode to execute tasks
func buildOpenCodeInitContainer(agentImage string) corev1.Container {
	return corev1.Container{
		Name:            "opencode-init",
		Image:           agentImage,
		ImagePullPolicy: inferImagePullPolicy(agentImage),
		// Uses default entrypoint from agents/opencode/entrypoint.sh
		// which copies /opencode to ${TOOLS_DIR}/opencode
		Env: []corev1.EnvVar{
			{Name: "TOOLS_DIR", Value: ToolsMountPath},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: ToolsVolumeName, MountPath: ToolsMountPath},
		},
	}
}

// buildGitInitContainer creates an init container that clones a Git repository.
func buildGitInitContainer(gm gitMount, volumeName string, index int, sysCfg systemConfig) corev1.Container {
	// Set default depth to 1 (shallow clone) if not specified
	depth := gm.depth
	if depth <= 0 {
		depth = DefaultGitDepth
	}

	// Set default ref to HEAD if not specified
	ref := defaultString(gm.ref, DefaultGitRef)

	envVars := []corev1.EnvVar{
		{Name: "GIT_REPO", Value: gm.repository},
		{Name: "GIT_REF", Value: ref},
		{Name: "GIT_DEPTH", Value: strconv.Itoa(depth)},
		{Name: "GIT_ROOT", Value: DefaultGitRoot},
		{Name: "GIT_LINK", Value: DefaultGitLink},
	}

	if gm.recurseSubmodules {
		envVars = append(envVars, corev1.EnvVar{
			Name: "GIT_RECURSE_SUBMODULES", Value: "true",
		})
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: volumeName, MountPath: DefaultGitRoot},
	}

	if gm.secretName != "" {
		envVars = append(envVars, buildGitCredentialEnvVars(gm.secretName)...)
	}

	return corev1.Container{
		Name:            fmt.Sprintf("git-init-%d", index),
		Image:           sysCfg.systemImage,
		ImagePullPolicy: sysCfg.systemImagePullPolicy,
		Command:         []string{"/kubeopencode", "git-init"},
		Env:             envVars,
		VolumeMounts:    volumeMounts,
	}
}

// buildGitCredentialEnvVars returns env vars that reference a Secret for Git authentication.
// The Secret can contain HTTPS credentials (username + password/PAT),
// SSH credentials (ssh-privatekey + optional ssh-known-hosts), or both.
// All keys are optional so the same Secret can be used for either method.
func buildGitCredentialEnvVars(secretName string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "GIT_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "username",
					Optional:             boolPtr(true),
				},
			},
		},
		{
			Name: "GIT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "password",
					Optional:             boolPtr(true),
				},
			},
		},
		{
			Name: "GIT_SSH_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "ssh-privatekey",
					Optional:             boolPtr(true),
				},
			},
		},
		{
			Name: "GIT_SSH_KNOWN_HOSTS",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "ssh-known-hosts",
					Optional:             boolPtr(true),
				},
			},
		},
	}
}

// buildGitSyncSidecar creates a sidecar container that periodically syncs a Git repository.
// Used when sync.policy is HotReload to keep content up-to-date without Pod restart.
func buildGitSyncSidecar(gm gitMount, volumeName string, index int, sysCfg systemConfig) corev1.Container {
	ref := defaultString(gm.ref, DefaultGitRef)
	intervalSeconds := int(gm.syncInterval.Seconds())
	if intervalSeconds <= 0 {
		intervalSeconds = 300 // default 5 minutes
	}

	envVars := []corev1.EnvVar{
		{Name: "GIT_REPO", Value: gm.repository},
		{Name: "GIT_REF", Value: ref},
		{Name: "GIT_ROOT", Value: DefaultGitRoot},
		{Name: "GIT_LINK", Value: DefaultGitLink},
		{Name: "GIT_SYNC_INTERVAL", Value: strconv.Itoa(intervalSeconds)},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: volumeName, MountPath: DefaultGitRoot},
	}

	if gm.secretName != "" {
		envVars = append(envVars, buildGitCredentialEnvVars(gm.secretName)...)
	}

	return corev1.Container{
		Name:            fmt.Sprintf("git-sync-%d", index),
		Image:           sysCfg.systemImage,
		ImagePullPolicy: sysCfg.systemImagePullPolicy,
		Command:         []string{"/kubeopencode", "git-sync"},
		Env:             envVars,
		VolumeMounts:    volumeMounts,
		SecurityContext: defaultSecurityContext(),
	}
}

// contextInitFileMapping represents a mapping from ConfigMap key to target file path.
// This mirrors the FileMapping struct in cmd/kubeopencode/context_init.go.
type contextInitFileMapping struct {
	Key        string `json:"key"`
	TargetPath string `json:"targetPath"`
	FileMode   *int32 `json:"fileMode,omitempty"` // Optional file permission mode (e.g., 0755)
}

// contextInitDirMapping represents a mapping from source directory to target directory.
// This mirrors the DirMapping struct in cmd/kubeopencode/context_init.go.
type contextInitDirMapping struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
}

// buildContextInitContainer creates an init container that copies ConfigMap content to the writable workspace.
// This enables agents to create files in the workspace directory, which is not possible with direct ConfigMap mounts.
// The init container uses /kubeopencode context-init command which reads configuration from environment variables.
func buildContextInitContainer(workspaceDir string, fileMounts []fileMount, dirMounts []dirMount, sysCfg systemConfig) corev1.Container {
	envVars := []corev1.EnvVar{
		{Name: "WORKSPACE_DIR", Value: workspaceDir},
		{Name: "CONFIGMAP_PATH", Value: "/configmap-files"},
	}

	// Build file mappings JSON
	if len(fileMounts) > 0 {
		mappings := make([]contextInitFileMapping, 0, len(fileMounts))
		for _, mount := range fileMounts {
			mappings = append(mappings, contextInitFileMapping{
				Key:        sanitizeConfigMapKey(mount.filePath),
				TargetPath: mount.filePath,
				FileMode:   mount.fileMode,
			})
		}
		mappingsJSON, _ := json.Marshal(mappings)
		envVars = append(envVars, corev1.EnvVar{
			Name:  "FILE_MAPPINGS",
			Value: string(mappingsJSON),
		})
	}

	// Build directory mappings JSON
	if len(dirMounts) > 0 {
		mappings := make([]contextInitDirMapping, 0, len(dirMounts))
		for i, dm := range dirMounts {
			mappings = append(mappings, contextInitDirMapping{
				SourcePath: fmt.Sprintf("/configmap-dir-%d", i),
				TargetPath: dm.dirPath,
			})
		}
		mappingsJSON, _ := json.Marshal(mappings)
		envVars = append(envVars, corev1.EnvVar{
			Name:  "DIR_MAPPINGS",
			Value: string(mappingsJSON),
		})
	}

	return corev1.Container{
		Name:            "context-init",
		Image:           sysCfg.systemImage,
		ImagePullPolicy: sysCfg.systemImagePullPolicy,
		Command:         []string{"/kubeopencode", "context-init"},
		Env:             envVars,
		// VolumeMounts will be added by the caller
	}
}

// buildCredentials resolves credentials into volumes, volume mounts, and environment variables.
func buildCredentials(credentials []kubeopenv1alpha1.Credential) ([]corev1.Volume, []corev1.VolumeMount, []corev1.EnvVar, []corev1.EnvFromSource) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var envVars []corev1.EnvVar
	var envFromSources []corev1.EnvFromSource

	// Add credentials (secrets as env vars or file mounts)
	for i, cred := range credentials {
		// Check if Key is specified - determines mounting behavior
		if cred.SecretRef.Key == nil || *cred.SecretRef.Key == "" {
			// No key specified: mount entire secret
			if cred.MountPath != nil && *cred.MountPath != "" {
				// Mount entire secret as a directory (each key becomes a file)
				volumeName := fmt.Sprintf("credential-%d", i)

				// Default file mode is 0600 (read/write for owner only)
				var fileMode = DefaultSecretFileMode
				if cred.FileMode != nil {
					fileMode = *cred.FileMode
				}

				volumes = append(volumes, corev1.Volume{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  cred.SecretRef.Name,
							DefaultMode: &fileMode,
						},
					},
				})
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      volumeName,
					MountPath: *cred.MountPath,
				})
			} else {
				// Mount entire secret as environment variables
				envFromSources = append(envFromSources, corev1.EnvFromSource{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cred.SecretRef.Name,
						},
					},
				})
			}
			continue
		}

		// Key is specified: use the existing single-key mounting behavior
		// Add as environment variable if Env is specified
		if cred.Env != nil && *cred.Env != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name: *cred.Env,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cred.SecretRef.Name,
						},
						Key: *cred.SecretRef.Key,
					},
				},
			})
		}

		// Add as file mount if MountPath is specified
		if cred.MountPath != nil && *cred.MountPath != "" {
			volumeName := fmt.Sprintf("credential-%d", i)

			// Default file mode is 0600 (read/write for owner only)
			var fileMode = DefaultSecretFileMode
			if cred.FileMode != nil {
				fileMode = *cred.FileMode
			}

			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: cred.SecretRef.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  *cred.SecretRef.Key,
								Path: "secret-file",
								Mode: &fileMode,
							},
						},
						DefaultMode: &fileMode,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: *cred.MountPath,
				SubPath:   "secret-file",
			})
		}
	}

	return volumes, volumeMounts, envVars, envFromSources
}

// buildCABundleVolumeMountEnv creates the Volume, VolumeMount, and EnvVar for custom CA bundle.
// It supports both ConfigMap and Secret sources, using the specified key or a default
// based on the source type. The CA certificate is projected to CABundleFileName inside the volume.
func buildCABundleVolumeMountEnv(caBundle *kubeopenv1alpha1.CABundleConfig) (corev1.Volume, corev1.VolumeMount, corev1.EnvVar) {
	var volume corev1.Volume

	if caBundle.ConfigMapRef != nil {
		key := caBundle.ConfigMapRef.Key
		if key == "" {
			key = DefaultCABundleConfigMapKey
		}
		volume = corev1.Volume{
			Name: CABundleVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: caBundle.ConfigMapRef.Name,
					},
					Items: []corev1.KeyToPath{
						{Key: key, Path: CABundleFileName},
					},
				},
			},
		}
	} else if caBundle.SecretRef != nil {
		key := caBundle.SecretRef.Key
		if key == "" {
			key = DefaultCABundleSecretKey
		}
		volume = corev1.Volume{
			Name: CABundleVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: caBundle.SecretRef.Name,
					Items: []corev1.KeyToPath{
						{Key: key, Path: CABundleFileName},
					},
				},
			},
		}
	}

	mount := corev1.VolumeMount{
		Name:      CABundleVolumeName,
		MountPath: CABundleMountPath,
		ReadOnly:  true,
	}

	env := corev1.EnvVar{
		Name:  CustomCACertEnvVar,
		Value: CABundleMountPath + "/" + CABundleFileName,
	}

	return volume, mount, env
}

// buildProxyEnvVars creates environment variables for HTTP/HTTPS proxy configuration.
// Both uppercase and lowercase variants are set for maximum compatibility.
// ".svc" and ".cluster.local" are always appended to NO_PROXY to prevent proxying
// in-cluster Kubernetes traffic.
func buildProxyEnvVars(proxy *kubeopenv1alpha1.ProxyConfig) []corev1.EnvVar {
	if proxy == nil {
		return nil
	}

	var envVars []corev1.EnvVar

	if proxy.HttpProxy != "" {
		envVars = append(envVars,
			corev1.EnvVar{Name: "HTTP_PROXY", Value: proxy.HttpProxy},
			corev1.EnvVar{Name: "http_proxy", Value: proxy.HttpProxy},
		)
	}

	if proxy.HttpsProxy != "" {
		envVars = append(envVars,
			corev1.EnvVar{Name: "HTTPS_PROXY", Value: proxy.HttpsProxy},
			corev1.EnvVar{Name: "https_proxy", Value: proxy.HttpsProxy},
		)
	}

	// Build NO_PROXY: user-specified values + mandatory in-cluster suffixes.
	// Each suffix is checked independently to avoid duplication.
	noProxy := proxy.NoProxy
	if noProxy == "" {
		noProxy = ".svc,.cluster.local"
	} else {
		if !strings.Contains(noProxy, ".svc") {
			noProxy += ",.svc"
		}
		if !strings.Contains(noProxy, ".cluster.local") {
			noProxy += ",.cluster.local"
		}
	}

	envVars = append(envVars,
		corev1.EnvVar{Name: "NO_PROXY", Value: noProxy},
		corev1.EnvVar{Name: "no_proxy", Value: noProxy},
	)

	return envVars
}

// defaultResources returns the default resource requirements for agent containers.
// This prevents unbounded memory growth from triggering node-level OOM.
// Users can override via podSpec.resources in Agent or AgentTemplate spec.
func defaultResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(DefaultMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(DefaultMemoryLimit),
		},
	}
}

// defaultSecurityContext returns the default restricted security context for containers.
// This enforces baseline Pod Security Standards:
// - No privilege escalation
// - Drop all Linux capabilities
// - RuntimeDefault seccomp profile
//
// Users can override this via AgentPodSpec.SecurityContext for stricter settings
// (e.g., runAsNonRoot, readOnlyRootFilesystem).
func defaultSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// buildPod creates a Pod object for the task with context mounts.
// The Pod is created in the same namespace as the Task.
// The serverURL parameter is used for Server-mode Agents: when non-empty, the Pod will use
// `opencode run --attach <serverURL>` to connect to an existing OpenCode server instead of
// running a standalone instance.
func buildPod(task *kubeopenv1alpha1.Task, podName string, cfg agentConfig, contextConfigMap *corev1.ConfigMap, fileMounts []fileMount, dirMounts []dirMount, gitMounts []gitMount, sysCfg systemConfig, serverURL string) *corev1.Pod {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var envVars []corev1.EnvVar
	var initContainers []corev1.Container

	// Add tools volume for sharing OpenCode binary between init and worker containers.
	// The OpenCode init container copies the binary to /tools, and the worker container uses it.
	volumes = append(volumes, corev1.Volume{
		Name: ToolsVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      ToolsVolumeName,
		MountPath: ToolsMountPath,
	})

	// Add OpenCode init container FIRST - it copies the OpenCode binary to /tools
	initContainers = append(initContainers, buildOpenCodeInitContainer(cfg.agentImage))

	// Always add workspace emptyDir volume for writable workspace.
	// This is essential for SCC environments where containers run with random UIDs
	// that don't have write access to directories created in the container image.
	volumes = append(volumes, corev1.Volume{
		Name: WorkspaceVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      WorkspaceVolumeName,
		MountPath: cfg.workspaceDir,
	})

	// Base environment variables for SCC (Security Context Constraints) compatibility.
	// In environments with SCC or similar security policies, containers run with
	// random UIDs that have no /etc/passwd entry, causing:
	// - HOME=/ (not writable) - tools like gemini-cli fail to create ~/.gemini
	// - SHELL=/sbin/nologin - terminals in interactive tools fail to start
	// Setting these explicitly ensures containers work regardless of UID.
	envVars = append(envVars,
		corev1.EnvVar{Name: "HOME", Value: DefaultHomeDir},
		corev1.EnvVar{Name: "SHELL", Value: DefaultShell},
		// Prepend /tools to PATH so the OpenCode binary is discoverable from interactive terminals.
		corev1.EnvVar{Name: "PATH", Value: ToolsMountPath + ":/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		corev1.EnvVar{Name: "TASK_NAME", Value: task.Name},
		corev1.EnvVar{Name: "TASK_NAMESPACE", Value: task.Namespace},
		corev1.EnvVar{Name: "WORKSPACE_DIR", Value: cfg.workspaceDir},
	)

	// If OpenCode config is provided or skills are configured, set OPENCODE_CONFIG env var.
	// Skills require the config file because skills.paths is injected into it.
	if (cfg.config != nil && *cfg.config != "") || len(cfg.skills) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  OpenCodeConfigEnvVar,
			Value: OpenCodeConfigPath,
		})
	}

	// Set OPENCODE_PERMISSION to enable all permissions by default.
	// This is required for non-interactive/automated execution in Kubernetes.
	// Without this, OpenCode would prompt for permission approval which would
	// block task execution in a Pod environment.
	// If the Agent config contains a "permission" field, skip the default to let
	// the user's custom permission config take effect (e.g., for interactive sessions).
	if !configHasPermission(cfg.config) {
		envVars = append(envVars, corev1.EnvVar{
			Name:  OpenCodePermissionEnvVar,
			Value: DefaultOpenCodePermission,
		})
	}

	// Check if context file is being mounted and inject OPENCODE_CONFIG_CONTENT.
	// This allows OpenCode to load KubeOpenCode's context file without conflicting
	// with repository's AGENTS.md. The context file path is relative to workspaceDir.
	contextFilePath := cfg.workspaceDir + "/" + ContextFileRelPath
	for _, fm := range fileMounts {
		if fm.filePath == contextFilePath {
			// Inject instructions to load the context file
			// OpenCode will merge this with OPENCODE_CONFIG (if set)
			envVars = append(envVars, corev1.EnvVar{
				Name:  OpenCodeConfigContentEnvVar,
				Value: `{"instructions":["` + ContextFileRelPath + `"]}`,
			})
			break
		}
	}

	// Add credentials (secrets as env vars or file mounts)
	vols, mounts, envs, envFroms := buildCredentials(cfg.credentials)
	volumes = append(volumes, vols...)
	volumeMounts = append(volumeMounts, mounts...)
	envVars = append(envVars, envs...)
	envFromSources := envFroms

	// Track volume mounts for the context-init container
	var contextInitMounts []corev1.VolumeMount

	// Add context ConfigMap volume if it exists (for aggregated content)
	// The ConfigMap is mounted to the init container, which copies content to the writable workspace
	if contextConfigMap != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "context-files",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: contextConfigMap.Name,
					},
				},
			},
		})

		// Mount ConfigMap to init container at a temporary path
		contextInitMounts = append(contextInitMounts, corev1.VolumeMount{
			Name:      "context-files",
			MountPath: "/configmap-files",
			ReadOnly:  true,
		})
	}

	// Add directory mounts (ConfigMapRef - entire ConfigMap as a directory)
	// These are also mounted to the init container and copied to workspace
	for i, dm := range dirMounts {
		volumeName := fmt.Sprintf("dir-mount-%d", i)
		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: dm.configMapName,
					},
					Optional: &dm.optional,
				},
			},
		})

		// Mount ConfigMap to init container at a temporary path
		contextInitMounts = append(contextInitMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/configmap-dir-%d", i),
			ReadOnly:  true,
		})
	}

	// Add context-init container if there are any context files or directories to copy
	if len(fileMounts) > 0 || len(dirMounts) > 0 {
		contextInit := buildContextInitContainer(cfg.workspaceDir, fileMounts, dirMounts, sysCfg)
		// Add workspace mount so init container can write to it
		// Start with contextInitMounts (ConfigMap volume mounts) and add workspace mount
		contextInit.VolumeMounts = append(contextInit.VolumeMounts, contextInitMounts...)
		contextInit.VolumeMounts = append(contextInit.VolumeMounts, corev1.VolumeMount{
			Name:      WorkspaceVolumeName,
			MountPath: cfg.workspaceDir,
		})

		// If OpenCode config is provided, mount /tools volume in context-init
		// so it can write the config file. The /tools volume is already created
		// for sharing the OpenCode binary between containers.
		if cfg.config != nil && *cfg.config != "" {
			contextInit.VolumeMounts = append(contextInit.VolumeMounts, corev1.VolumeMount{
				Name:      ToolsVolumeName,
				MountPath: ToolsMountPath,
			})
		}

		// For files outside /workspace, we need to create shared emptyDir volumes
		// so that the context-init container can write files that persist to the agent container.
		// Group files by their parent directory to minimize the number of volumes.
		externalDirs := make(map[string]bool)
		for _, fm := range fileMounts {
			if !isUnderPath(fm.filePath, cfg.workspaceDir) {
				parentDir := getParentDir(fm.filePath)
				// Skip /tools as it already exists for the OpenCode binary
				if parentDir == ToolsMountPath {
					continue
				}
				externalDirs[parentDir] = true
			}
		}

		// Create emptyDir volumes for each unique external parent directory
		for dir := range externalDirs {
			volumeName := sanitizeVolumeName(dir)

			// Add emptyDir volume for this external directory
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})

			// Mount this volume in context-init container
			contextInit.VolumeMounts = append(contextInit.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: dir,
			})

			// Mount this volume in agent container
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: dir,
			})
		}

		initContainers = append(initContainers, contextInit)
	}

	// Add Git context mounts (using git-init containers)
	for i, gm := range gitMounts {
		volumeName := fmt.Sprintf("git-context-%d", i)

		// Add emptyDir volume for git content
		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		// Check if this git context should be merged into the workspace root.
		// When mountPath resolves to workspaceDir (e.g., mountPath: "."), we can't
		// overlay the workspace with a separate volume mount because it would shadow
		// files already in the workspace emptyDir (e.g., task.md from context-init).
		// Instead, git-init copies the cloned content into the workspace emptyDir.
		isWorkspaceRoot := filepath.Clean(gm.mountPath) == filepath.Clean(cfg.workspaceDir)

		// Build init container for git clone
		gitInitContainer := buildGitInitContainer(gm, volumeName, i, sysCfg)

		if isWorkspaceRoot {
			// Workspace root mode: after cloning, git-init merges repo content
			// into the workspace emptyDir so task.md and repo files coexist.
			gitInitContainer.Env = append(gitInitContainer.Env,
				corev1.EnvVar{Name: "GIT_WORKSPACE_DIR", Value: cfg.workspaceDir},
			)
			if gm.repoPath != "" {
				gitInitContainer.Env = append(gitInitContainer.Env,
					corev1.EnvVar{Name: "GIT_REPO_SUBPATH", Value: gm.repoPath},
				)
			}
			gitInitContainer.VolumeMounts = append(gitInitContainer.VolumeMounts,
				corev1.VolumeMount{
					Name:      WorkspaceVolumeName,
					MountPath: cfg.workspaceDir,
				},
			)
		}

		initContainers = append(initContainers, gitInitContainer)

		if !isWorkspaceRoot {
			// Normal case: mount git volume at the specified path in agent container.
			// If repoPath is specified, use subPath to mount only that path.
			baseSubPath := DefaultGitLink
			if gm.repoPath != "" {
				baseSubPath = DefaultGitLink + "/" + strings.TrimPrefix(gm.repoPath, "/")
			}

			if len(gm.names) > 0 {
				// When specific names are set, mount only the named subdirectories
				// instead of the entire directory. This prevents agents from
				// discovering unselected skills in the repository.
				cleanBase := filepath.Clean(baseSubPath)
				for _, name := range gm.names {
					volumeMounts = append(volumeMounts, corev1.VolumeMount{
						Name:      volumeName,
						MountPath: filepath.Join(gm.mountPath, name),
						SubPath:   filepath.Join(cleanBase, name),
					})
				}
			} else {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      volumeName,
					MountPath: gm.mountPath,
					SubPath:   baseSubPath,
				})
			}
		}
	}

	// If we have Git mounts, add GIT_CONFIG_GLOBAL to point to shared gitconfig
	// This is needed because init containers run as different users and git will
	// refuse to work without safe.directory configured
	if len(gitMounts) > 0 {
		// The first git-init container writes .gitconfig to /git/.gitconfig
		// which is shared via git-context-0 volume
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_CONFIG_GLOBAL",
			Value: DefaultGitRoot + "/.gitconfig",
		})
		// Mount the git volume root to access the .gitconfig
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "git-context-0",
			MountPath: DefaultGitRoot + "/.gitconfig",
			SubPath:   ".gitconfig",
		})
	}

	// Add custom CA bundle to all containers if configured.
	// The CA certificate volume, mount, and env var are added to every init container
	// and the worker container so that all HTTPS connections can verify custom CAs.
	if cfg.caBundle != nil && (cfg.caBundle.ConfigMapRef != nil || cfg.caBundle.SecretRef != nil) {
		caVolume, caMount, caEnv := buildCABundleVolumeMountEnv(cfg.caBundle)
		volumes = append(volumes, caVolume)

		// Add CA bundle mount and env to all init containers
		for i := range initContainers {
			initContainers[i].VolumeMounts = append(initContainers[i].VolumeMounts, caMount)
			initContainers[i].Env = append(initContainers[i].Env, caEnv)
		}

		// Add CA bundle mount and env to the worker container
		volumeMounts = append(volumeMounts, caMount)
		envVars = append(envVars, caEnv)
	}

	// Add HTTP/HTTPS proxy environment variables to all containers if configured
	if cfg.proxy != nil {
		proxyEnvs := buildProxyEnvVars(cfg.proxy)
		// Add to all init containers
		for i := range initContainers {
			initContainers[i].Env = append(initContainers[i].Env, proxyEnvs...)
		}
		// Add to worker container env vars
		envVars = append(envVars, proxyEnvs...)
	}

	// Build pod labels - start with base labels
	podLabels := map[string]string{
		"app":                  "kubeopencode",
		"kubeopencode.io/task": task.Name,
	}

	// Add custom pod labels from Agent.PodSpec
	if cfg.podSpec != nil {
		for k, v := range cfg.podSpec.Labels {
			podLabels[k] = v
		}
	}

	// Build agent container using executorImage (the worker container)
	// The OpenCode binary is available at /tools/opencode from the init container
	// Use custom command if provided, otherwise use default
	agentCommand := cfg.command
	if len(agentCommand) == 0 {
		// Generate session title: task name + random suffix for uniqueness.
		// This makes sessions identifiable in the OpenCode Web UI and enables
		// future human-in-the-loop workflows (resuming sessions by title).
		sessionTitle := sessionTitle(task)
		if serverURL != "" {
			// agentRef path: use --attach flag to connect to the Agent's OpenCode server.
			// Tasks are non-interactive — all permissions are auto-allowed via
			// OPENCODE_PERMISSION env var on the server, so no permission.asked
			// events are generated. This gives natural OpenCode TUI-style output
			// in pod logs.
			// For interactive sessions, users use `opencode attach` directly.
			agentCommand = []string{
				"sh", "-c",
				fmt.Sprintf(`%s; /tools/opencode run --attach %s --title %s "$(cat %s/task.md)"`, OpenCodeSymlinkCmd, serverURL, shellEscape(sessionTitle), cfg.workspaceDir),
			}
		} else {
			// templateRef path: run standalone OpenCode instance
			agentCommand = []string{
				"sh", "-c",
				fmt.Sprintf(`%s; /tools/opencode run --title %s "$(cat %s/task.md)"`, OpenCodeSymlinkCmd, shellEscape(sessionTitle), cfg.workspaceDir),
			}
		}
	}
	// Determine executor image: use lightweight attach image only for agentRef tasks
	// that use the default --attach command. When a custom command is provided,
	// keep the executor image since the custom command may need tools not available
	// in the minimal attach image.
	executorImage := cfg.executorImage
	if serverURL != "" && cfg.attachImage != "" && len(cfg.command) == 0 {
		// agentRef with default command: use lightweight attach image (~25MB) instead
		// of devbox (~1GB). The attach image only needs the OpenCode binary since
		// actual execution happens in the persistent server's environment.
		executorImage = cfg.attachImage
	}

	agentContainer := corev1.Container{
		Name:            "agent",
		Image:           executorImage,
		ImagePullPolicy: inferImagePullPolicy(executorImage),
		WorkingDir:      cfg.workspaceDir,
		Command:         agentCommand,
		Env:             envVars,
		EnvFrom:         envFromSources,
		VolumeMounts:    volumeMounts,
	}

	// Apply resource requirements - use custom if provided, otherwise use defaults
	if cfg.podSpec != nil && cfg.podSpec.Resources != nil {
		agentContainer.Resources = *cfg.podSpec.Resources
	} else {
		agentContainer.Resources = defaultResources()
	}

	// Apply security context - use custom if provided, otherwise use restricted default
	if cfg.podSpec != nil && cfg.podSpec.SecurityContext != nil {
		agentContainer.SecurityContext = cfg.podSpec.SecurityContext
	} else {
		agentContainer.SecurityContext = defaultSecurityContext()
	}

	// Apply lifecycle hooks (e.g., postStart for starting code-server)
	if cfg.podSpec != nil && cfg.podSpec.Lifecycle != nil {
		agentContainer.Lifecycle = cfg.podSpec.Lifecycle
	}

	// Apply default security context to init containers
	for i := range initContainers {
		if initContainers[i].SecurityContext == nil {
			initContainers[i].SecurityContext = defaultSecurityContext()
		}
	}

	// Build containers list
	containers := []corev1.Container{agentContainer}

	// Build PodSpec with scheduling configuration
	podSpec := corev1.PodSpec{
		ServiceAccountName: cfg.serviceAccountName,
		InitContainers:     initContainers,
		Containers:         containers,
		Volumes:            volumes,
		RestartPolicy:      corev1.RestartPolicyNever,
	}

	// Add imagePullSecrets for private registry authentication
	if len(cfg.imagePullSecrets) > 0 {
		podSpec.ImagePullSecrets = cfg.imagePullSecrets
	}

	// Apply PodSpec configuration if specified
	if cfg.podSpec != nil {
		// Apply scheduling configuration
		if cfg.podSpec.Scheduling != nil {
			if cfg.podSpec.Scheduling.NodeSelector != nil {
				podSpec.NodeSelector = cfg.podSpec.Scheduling.NodeSelector
			}
			if cfg.podSpec.Scheduling.Tolerations != nil {
				podSpec.Tolerations = cfg.podSpec.Scheduling.Tolerations
			}
			if cfg.podSpec.Scheduling.Affinity != nil {
				podSpec.Affinity = cfg.podSpec.Scheduling.Affinity
			}
		}

		// Apply runtime class if specified (for gVisor, Kata, etc.)
		if cfg.podSpec.RuntimeClassName != nil {
			podSpec.RuntimeClassName = cfg.podSpec.RuntimeClassName
		}

		// Apply pod-level security context if specified
		if cfg.podSpec.PodSecurityContext != nil {
			podSpec.SecurityContext = cfg.podSpec.PodSecurityContext
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: task.Namespace,
			Labels:    podLabels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(task, kubeopenv1alpha1.SchemeGroupVersion.WithKind("Task")),
			},
		},
		Spec: podSpec,
	}

	return pod
}

// configHasPermission checks if the Agent's OpenCode config JSON contains
// a "permission" field. When present, the user has explicitly configured
// permissions (e.g., for interactive sessions), so we should not override with the default
// all-allow environment variable.
func configHasPermission(config *string) bool {
	if config == nil || *config == "" {
		return false
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(*config), &parsed); err != nil {
		return false
	}
	_, ok := parsed["permission"]
	return ok
}

// sessionTitle generates a session title for the OpenCode session.
// Format: "{task-name}-{8-char-random-hex}"
func sessionTitle(task *kubeopenv1alpha1.Task) string {
	return fmt.Sprintf("%s-%s", task.Name, randomHex(4))
}

// randomHex returns a random hex string of n bytes (2n characters).
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// shellEscape wraps a string in single quotes for safe use in shell commands.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
