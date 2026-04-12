// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// ServerDeploymentSuffix is appended to Agent name for the Deployment name.
	ServerDeploymentSuffix = "-server"

	// ServerContainerName is the name of the main container in the server Deployment.
	ServerContainerName = "opencode-server"

	// DefaultServerPort is the default port for OpenCode server.
	DefaultServerPort int32 = 4096

	// ServerHealthPath is the path used for readiness probes.
	// OpenCode's /session/status endpoint returns 200 if the server is healthy.
	ServerHealthPath = "/session/status"

	// ServerSessionPVCSuffix is appended to Agent name for the session PVC name.
	ServerSessionPVCSuffix = "-server-sessions"

	// ServerSessionVolumeName is the volume name for the session PVC.
	ServerSessionVolumeName = "session-data"

	// ServerSessionMountPath is where the session PVC is mounted in the server container.
	ServerSessionMountPath = "/data/sessions"

	// ServerSessionDBPath is the full path to the OpenCode session database.
	ServerSessionDBPath = ServerSessionMountPath + "/opencode.db"

	// DefaultSessionPVCSize is the default size for the session PVC.
	DefaultSessionPVCSize = "1Gi"

	// OpenCodeDBEnvVar is the environment variable name for the OpenCode database path.
	OpenCodeDBEnvVar = "OPENCODE_DB"

	// ServerWorkspacePVCSuffix is appended to Agent name for the workspace PVC name.
	ServerWorkspacePVCSuffix = "-server-workspace"

	// DefaultWorkspacePVCSize is the default size for the workspace PVC.
	DefaultWorkspacePVCSize = "10Gi"
)

// ServerDeploymentName returns the Deployment name for a Server-mode Agent.
func ServerDeploymentName(agentName string) string {
	return agentName + ServerDeploymentSuffix
}

// ServerServiceName returns the Service name for a Server-mode Agent.
// We use the Agent name directly for simpler DNS resolution.
func ServerServiceName(agentName string) string {
	return agentName
}

// ServerURL returns the in-cluster URL for a Server-mode Agent.
func ServerURL(agentName, namespace string, port int32) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", agentName, namespace, port)
}

// ServerSessionPVCName returns the PVC name for session persistence.
func ServerSessionPVCName(agentName string) string {
	return agentName + ServerSessionPVCSuffix
}

// ServerWorkspacePVCName returns the PVC name for workspace persistence.
func ServerWorkspacePVCName(agentName string) string {
	return agentName + ServerWorkspacePVCSuffix
}

// BuildServerWorkspacePVC creates a PersistentVolumeClaim for workspace persistence.
// Returns (nil, nil) if workspace persistence is not configured.
func BuildServerWorkspacePVC(agent *kubeopenv1alpha1.Agent) (*corev1.PersistentVolumeClaim, error) {
	if agent.Spec.Persistence == nil ||
		agent.Spec.Persistence.Workspace == nil {
		return nil, nil
	}
	return buildServerPVC(agent, agent.Spec.Persistence.Workspace,
		ServerWorkspacePVCName(agent.Name), DefaultWorkspacePVCSize, "workspace")
}

// BuildServerSessionPVC creates a PersistentVolumeClaim for session data persistence.
// Returns (nil, nil) if session persistence is not configured.
func BuildServerSessionPVC(agent *kubeopenv1alpha1.Agent) (*corev1.PersistentVolumeClaim, error) {
	if agent.Spec.Persistence == nil ||
		agent.Spec.Persistence.Sessions == nil {
		return nil, nil
	}
	return buildServerPVC(agent, agent.Spec.Persistence.Sessions,
		ServerSessionPVCName(agent.Name), DefaultSessionPVCSize, "session")
}

// buildServerPVC creates a PVC with the given configuration.
func buildServerPVC(agent *kubeopenv1alpha1.Agent, vol *kubeopenv1alpha1.VolumePersistence, pvcName, defaultSize, label string) (*corev1.PersistentVolumeClaim, error) {
	size := vol.Size
	if size == "" {
		size = defaultSize
	}

	qty, err := resource.ParseQuantity(size)
	if err != nil {
		return nil, fmt.Errorf("invalid %s PVC size %q: %w", label, size, err)
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: agent.Namespace,
			Labels:    getServerLabels(agent.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}

	if vol.StorageClassName != nil {
		pvc.Spec.StorageClassName = vol.StorageClassName
	}

	return pvc, nil
}

// getServerLabels returns the common labels used by Server-mode resources.
func getServerLabels(agentName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "kubeopencode-server",
		"app.kubernetes.io/instance":   agentName,
		"app.kubernetes.io/component":  "server",
		"app.kubernetes.io/managed-by": "kubeopencode",
		AgentLabelKey:                  agentName,
	}
}

// BuildServerDeployment creates a Deployment for an Agent.
// The Deployment runs OpenCode in serve mode with a single replica.
// Context parameters (contextConfigMap, fileMounts, dirMounts, gitMounts) enable
// Agent-level contexts to be loaded via init containers.
func BuildServerDeployment(agent *kubeopenv1alpha1.Agent, agentCfg agentConfig, sysCfg systemConfig, contextConfigMap *corev1.ConfigMap, ctxFileMounts []fileMount, ctxDirMounts []dirMount, ctxGitMounts []gitMount, gitHashAnnotations map[string]string) *appsv1.Deployment {
	port := GetServerPort(agent)

	// Build labels for selector and pod template
	labels := getServerLabels(agent.Name)

	// Merge custom labels from PodSpec if provided
	if agentCfg.podSpec != nil && agentCfg.podSpec.Labels != nil {
		maps.Copy(labels, agentCfg.podSpec.Labels)
	}

	// Build environment variables
	// HOME and SHELL are set for SCC (Security Context Constraints) compatibility.
	// In SCC environments, containers run with random UIDs that have no /etc/passwd entry,
	// causing HOME=/ (not writable) and SHELL=/sbin/nologin.
	envVars := []corev1.EnvVar{
		{Name: "HOME", Value: DefaultHomeDir},
		{Name: "SHELL", Value: DefaultShell},
		// Prepend /tools to PATH so the OpenCode binary (copied by the init container)
		// is discoverable from interactive terminals (e.g., VS Code in browser).
		// The symlink approach (ln -sf /tools/opencode /usr/local/bin/) fails silently
		// when the container runs as non-root (UID 1000) because /usr/local/bin/ is root-owned.
		{Name: "PATH", Value: ToolsMountPath + ":/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		{Name: "WORKSPACE_DIR", Value: agentCfg.workspaceDir},
	}

	// Set OPENCODE_PERMISSION only if the Agent config does not include custom permissions.
	// When the config has a "permission" field, the user has explicitly configured
	// permission behavior (e.g., "ask" mode for interactive sessions), so we must not override it.
	if !configHasPermission(agentCfg.config) {
		envVars = append(envVars, corev1.EnvVar{
			Name:  OpenCodePermissionEnvVar,
			Value: DefaultOpenCodePermission,
		})
	}

	// Add OpenCode config if provided, or if skills are configured (skills.paths is injected into config)
	if agentCfg.config != nil || len(agentCfg.skills) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  OpenCodeConfigEnvVar,
			Value: OpenCodeConfigPath,
		})
	}

	// Build volume mounts
	volumeMounts := []corev1.VolumeMount{
		{Name: ToolsVolumeName, MountPath: ToolsMountPath},
		{Name: WorkspaceVolumeName, MountPath: agentCfg.workspaceDir},
	}

	// Build volumes
	workspaceVolumeSource := corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
	if agent.Spec.Persistence != nil && agent.Spec.Persistence.Workspace != nil {
		workspaceVolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: ServerWorkspacePVCName(agent.Name),
			},
		}
	}

	volumes := []corev1.Volume{
		{
			Name: ToolsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name:         WorkspaceVolumeName,
			VolumeSource: workspaceVolumeSource,
		},
	}

	// Add session persistence volume and env var if configured
	if agent.Spec.Persistence != nil && agent.Spec.Persistence.Sessions != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ServerSessionVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: ServerSessionPVCName(agent.Name),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ServerSessionVolumeName,
			MountPath: ServerSessionMountPath,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  OpenCodeDBEnvVar,
			Value: ServerSessionDBPath,
		})
	}

	// Add credentials (secrets as env vars or file mounts)
	credVols, credMounts, credEnvs, credEnvFroms := buildCredentials(agentCfg.credentials)
	volumes = append(volumes, credVols...)
	volumeMounts = append(volumeMounts, credMounts...)
	envVars = append(envVars, credEnvs...)

	// Track init containers (opencode-init is always first)
	var initContainers []corev1.Container
	initContainers = append(initContainers, buildOpenCodeInitContainer(agentCfg.agentImage))

	// Add context init containers and volumes
	var contextInitMounts []corev1.VolumeMount

	// Add context ConfigMap volume if it exists
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
		contextInitMounts = append(contextInitMounts, corev1.VolumeMount{
			Name:      "context-files",
			MountPath: "/configmap-files",
			ReadOnly:  true,
		})
	}

	// Add directory mounts (ConfigMapRef - entire ConfigMap as a directory)
	for i, dm := range ctxDirMounts {
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
		contextInitMounts = append(contextInitMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("/configmap-dir-%d", i),
			ReadOnly:  true,
		})
	}

	// Add context-init container if there are any files or directories to copy
	if len(ctxFileMounts) > 0 || len(ctxDirMounts) > 0 {
		contextInit := buildContextInitContainer(agentCfg.workspaceDir, ctxFileMounts, ctxDirMounts, sysCfg)
		contextInit.VolumeMounts = append(contextInit.VolumeMounts, contextInitMounts...)
		contextInit.VolumeMounts = append(contextInit.VolumeMounts, corev1.VolumeMount{
			Name:      WorkspaceVolumeName,
			MountPath: agentCfg.workspaceDir,
		})

		// Mount /tools volume so context-init can write config file
		if agentCfg.config != nil && *agentCfg.config != "" {
			contextInit.VolumeMounts = append(contextInit.VolumeMounts, corev1.VolumeMount{
				Name:      ToolsVolumeName,
				MountPath: ToolsMountPath,
			})
		}

		// Handle files outside workspace
		externalDirs := make(map[string]bool)
		for _, fm := range ctxFileMounts {
			if !isUnderPath(fm.filePath, agentCfg.workspaceDir) {
				parentDir := getParentDir(fm.filePath)
				if parentDir == ToolsMountPath {
					continue
				}
				externalDirs[parentDir] = true
			}
		}
		for dir := range externalDirs {
			volumeName := sanitizeVolumeName(dir)
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})
			contextInit.VolumeMounts = append(contextInit.VolumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: dir,
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: dir,
			})
		}

		initContainers = append(initContainers, contextInit)
	}

	// Add Git context mounts (using git-init containers)
	for i, gm := range ctxGitMounts {
		volumeName := fmt.Sprintf("git-context-%d", i)
		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		isWorkspaceRoot := filepath.Clean(gm.mountPath) == filepath.Clean(agentCfg.workspaceDir)
		gitInitContainer := buildGitInitContainer(gm, volumeName, i, sysCfg)

		if isWorkspaceRoot {
			gitInitContainer.Env = append(gitInitContainer.Env,
				corev1.EnvVar{Name: "GIT_WORKSPACE_DIR", Value: agentCfg.workspaceDir},
			)
			if gm.repoPath != "" {
				gitInitContainer.Env = append(gitInitContainer.Env,
					corev1.EnvVar{Name: "GIT_REPO_SUBPATH", Value: gm.repoPath},
				)
			}
			gitInitContainer.VolumeMounts = append(gitInitContainer.VolumeMounts,
				corev1.VolumeMount{
					Name:      WorkspaceVolumeName,
					MountPath: agentCfg.workspaceDir,
				},
			)
		}

		initContainers = append(initContainers, gitInitContainer)

		if !isWorkspaceRoot {
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

	// Add GIT_CONFIG_GLOBAL if we have Git mounts
	if len(ctxGitMounts) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_CONFIG_GLOBAL",
			Value: DefaultGitRoot + "/.gitconfig",
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "git-context-0",
			MountPath: DefaultGitRoot + "/.gitconfig",
			SubPath:   ".gitconfig",
		})
	}

	// Check if context file is being mounted and inject OPENCODE_CONFIG_CONTENT
	contextFilePath := agentCfg.workspaceDir + "/" + ContextFileRelPath
	for _, fm := range ctxFileMounts {
		if fm.filePath == contextFilePath {
			envVars = append(envVars, corev1.EnvVar{
				Name:  OpenCodeConfigContentEnvVar,
				Value: `{"instructions":["` + ContextFileRelPath + `"]}`,
			})
			break
		}
	}

	// Add custom CA bundle to all containers if configured
	if agentCfg.caBundle != nil && (agentCfg.caBundle.ConfigMapRef != nil || agentCfg.caBundle.SecretRef != nil) {
		caVolume, caMount, caEnv := buildCABundleVolumeMountEnv(agentCfg.caBundle)
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
	if agentCfg.proxy != nil {
		proxyEnvs := buildProxyEnvVars(agentCfg.proxy)
		// Add to all init containers
		for i := range initContainers {
			initContainers[i].Env = append(initContainers[i].Env, proxyEnvs...)
		}
		// Add to worker container env vars
		envVars = append(envVars, proxyEnvs...)
	}

	// Build the serve command.
	// When context-init handles config file writing, we don't need inline heredoc.
	hasContextInit := len(ctxFileMounts) > 0 || len(ctxDirMounts) > 0
	var command []string
	if agentCfg.config != nil && *agentCfg.config != "" && !hasContextInit {
		// No context-init container — write config inline in the command
		command = []string{
			"sh", "-c",
			fmt.Sprintf("%s; cat > %s << 'KOCEOF'\n%s\nKOCEOF\n/tools/opencode serve --port %d --hostname 0.0.0.0",
				OpenCodeSymlinkCmd, OpenCodeConfigPath, *agentCfg.config, port),
		}
	} else {
		// Config is written by context-init, or no config at all
		command = []string{
			"sh", "-c",
			fmt.Sprintf("%s; /tools/opencode serve --port %d --hostname 0.0.0.0", OpenCodeSymlinkCmd, port),
		}
	}

	// Build the main container
	container := corev1.Container{
		Name:            ServerContainerName,
		Image:           agentCfg.executorImage,
		ImagePullPolicy: inferImagePullPolicy(agentCfg.executorImage),
		WorkingDir:      agentCfg.workspaceDir,
		Command:         command,
		Env:             envVars,
		EnvFrom:         credEnvFroms,
		VolumeMounts:    volumeMounts,
		Ports:           buildContainerPorts(port, agentCfg.extraPorts),
		// StartupProbe gates liveness and readiness probes until the server
		// is fully initialized (e.g., session restore after resume from standby).
		// Without this, the readiness probe may pass before the server can
		// handle attach connections, causing "exit code 137" errors.
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   ServerHealthPath,
					Port:   intstr.FromInt32(port),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 2,
			PeriodSeconds:       2,
			TimeoutSeconds:      5,
			FailureThreshold:    30, // 2 + 2*30 = 62s max startup time
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(port),
				},
			},
			PeriodSeconds:    30,
			TimeoutSeconds:   5,
			FailureThreshold: 3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   ServerHealthPath,
					Port:   intstr.FromInt32(port),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			PeriodSeconds:    10,
			TimeoutSeconds:   5,
			FailureThreshold: 3,
		},
	}

	// Apply resource requirements - use custom if provided, otherwise use defaults
	if agentCfg.podSpec != nil && agentCfg.podSpec.Resources != nil {
		container.Resources = *agentCfg.podSpec.Resources
	} else {
		container.Resources = defaultResources()
	}

	// Apply security context - use custom if provided, otherwise use restricted default
	if agentCfg.podSpec != nil && agentCfg.podSpec.SecurityContext != nil {
		container.SecurityContext = agentCfg.podSpec.SecurityContext
	} else {
		container.SecurityContext = defaultSecurityContext()
	}

	// Apply lifecycle hooks (e.g., postStart for starting code-server)
	if agentCfg.podSpec != nil && agentCfg.podSpec.Lifecycle != nil {
		container.Lifecycle = agentCfg.podSpec.Lifecycle
	}

	// Apply default security context to init containers
	for i := range initContainers {
		if initContainers[i].SecurityContext == nil {
			initContainers[i].SecurityContext = defaultSecurityContext()
		}
	}

	// Build git-sync sidecar containers for HotReload policy
	var sidecars []corev1.Container

	// Pre-compute CA and proxy env vars once (shared across all sidecars)
	var sidecarCAMount corev1.VolumeMount
	var sidecarCAEnv corev1.EnvVar
	hasCA := agentCfg.caBundle != nil && (agentCfg.caBundle.ConfigMapRef != nil || agentCfg.caBundle.SecretRef != nil)
	if hasCA {
		_, sidecarCAMount, sidecarCAEnv = buildCABundleVolumeMountEnv(agentCfg.caBundle)
	}
	var proxyEnvs []corev1.EnvVar
	if agentCfg.proxy != nil {
		proxyEnvs = buildProxyEnvVars(agentCfg.proxy)
	}

	for i, gm := range ctxGitMounts {
		if gm.syncEnabled && gm.syncPolicy == kubeopenv1alpha1.GitSyncPolicyHotReload {
			sidecar := buildGitSyncSidecar(gm, fmt.Sprintf("git-context-%d", i), i, sysCfg)
			if hasCA {
				sidecar.VolumeMounts = append(sidecar.VolumeMounts, sidecarCAMount)
				sidecar.Env = append(sidecar.Env, sidecarCAEnv)
			}
			if len(proxyEnvs) > 0 {
				sidecar.Env = append(sidecar.Env, proxyEnvs...)
			}
			sidecars = append(sidecars, sidecar)
		}
	}

	// Build pod template spec
	containers := []corev1.Container{container}
	containers = append(containers, sidecars...)
	podSpec := corev1.PodSpec{
		ServiceAccountName: agentCfg.serviceAccountName,
		InitContainers:     initContainers,
		Containers:         containers,
		Volumes:            volumes,
		RestartPolicy:      corev1.RestartPolicyAlways,
	}

	// Add imagePullSecrets for private registry authentication
	if len(agentCfg.imagePullSecrets) > 0 {
		podSpec.ImagePullSecrets = agentCfg.imagePullSecrets
	}

	// Apply scheduling configuration if provided
	if agentCfg.podSpec != nil && agentCfg.podSpec.Scheduling != nil {
		scheduling := agentCfg.podSpec.Scheduling
		if scheduling.NodeSelector != nil {
			podSpec.NodeSelector = scheduling.NodeSelector
		}
		if scheduling.Tolerations != nil {
			podSpec.Tolerations = scheduling.Tolerations
		}
		if scheduling.Affinity != nil {
			podSpec.Affinity = scheduling.Affinity
		}
	}

	// Apply runtime class if specified
	if agentCfg.podSpec != nil && agentCfg.podSpec.RuntimeClassName != nil {
		podSpec.RuntimeClassName = agentCfg.podSpec.RuntimeClassName
	}

	// Apply pod-level security context if specified
	if agentCfg.podSpec != nil && agentCfg.podSpec.PodSecurityContext != nil {
		podSpec.SecurityContext = agentCfg.podSpec.PodSecurityContext
	}

	// Single replica for now (simplicity)
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServerDeploymentName(agent.Name),
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					AgentLabelKey: agent.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: gitHashAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}

// BuildServerService creates a Service for an Agent.
func BuildServerService(agent *kubeopenv1alpha1.Agent) *corev1.Service {
	port := GetServerPort(agent)

	labels := getServerLabels(agent.Name)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServerServiceName(agent.Name),
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				AgentLabelKey: agent.Name,
			},
			Ports: buildServicePorts(port, agent.Spec.ExtraPorts),
		},
	}
}

// buildContainerPorts constructs the container port list for an Agent Deployment.
// It always includes the main OpenCode server port and appends any extra ports.
func buildContainerPorts(serverPort int32, extraPorts []kubeopenv1alpha1.ExtraPort) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: serverPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}
	for _, ep := range extraPorts {
		protocol := ep.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          ep.Name,
			ContainerPort: ep.Port,
			Protocol:      protocol,
		})
	}
	return ports
}

// buildServicePorts constructs the service port list for an Agent Service.
// It always includes the main OpenCode server port and appends any extra ports.
func buildServicePorts(serverPort int32, extraPorts []kubeopenv1alpha1.ExtraPort) []corev1.ServicePort {
	ports := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       serverPort,
			TargetPort: intstr.FromInt32(serverPort),
			Protocol:   corev1.ProtocolTCP,
		},
	}
	for _, ep := range extraPorts {
		protocol := ep.Protocol
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}
		ports = append(ports, corev1.ServicePort{
			Name:       ep.Name,
			Port:       ep.Port,
			TargetPort: intstr.FromInt32(ep.Port),
			Protocol:   protocol,
		})
	}
	return ports
}

// GetServerPort returns the configured port or default.
func GetServerPort(agent *kubeopenv1alpha1.Agent) int32 {
	if agent.Spec.Port != 0 {
		return agent.Spec.Port
	}
	return DefaultServerPort
}
