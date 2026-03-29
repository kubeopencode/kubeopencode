// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// BuildServerDeployment creates a Deployment for a Server-mode Agent.
// The Deployment runs OpenCode in serve mode with a single replica.
// Context parameters (contextConfigMap, fileMounts, dirMounts, gitMounts) enable
// Agent-level contexts to be loaded via init containers, matching Pod mode behavior.
func BuildServerDeployment(agent *kubeopenv1alpha1.Agent, agentCfg agentConfig, sysCfg systemConfig, contextConfigMap *corev1.ConfigMap, ctxFileMounts []fileMount, ctxDirMounts []dirMount, ctxGitMounts []gitMount) *appsv1.Deployment {
	serverConfig := agent.Spec.ServerConfig
	if serverConfig == nil {
		return nil
	}

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

	// Add OpenCode config if provided
	if agentCfg.config != nil {
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
	volumes := []corev1.Volume{
		{
			Name: ToolsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: WorkspaceVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Add credentials (secrets as env vars or file mounts)
	credVols, credMounts, credEnvs, credEnvFroms := buildCredentials(agentCfg.credentials)
	volumes = append(volumes, credVols...)
	volumeMounts = append(volumeMounts, credMounts...)
	envVars = append(envVars, credEnvs...)

	// Track init containers (opencode-init is always first)
	var initContainers []corev1.Container
	initContainers = append(initContainers, buildOpenCodeInitContainer(agentCfg.agentImage))

	// Add context init containers and volumes (matching Pod mode behavior)
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

		// Handle files outside workspace (same logic as Pod mode)
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
			subPath := DefaultGitLink
			if gm.repoPath != "" {
				subPath = DefaultGitLink + "/" + strings.TrimPrefix(gm.repoPath, "/")
			}
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volumeName,
				MountPath: gm.mountPath,
				SubPath:   subPath,
			})
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

	// Build the serve command.
	// When context-init handles config file writing, we don't need inline heredoc.
	hasContextInit := len(ctxFileMounts) > 0 || len(ctxDirMounts) > 0
	var command []string
	if agentCfg.config != nil && *agentCfg.config != "" && !hasContextInit {
		// No context-init container — write config inline in the command
		command = []string{
			"sh", "-c",
			fmt.Sprintf("cat > %s << 'KOCEOF'\n%s\nKOCEOF\n/tools/opencode serve --port %d --hostname 0.0.0.0",
				OpenCodeConfigPath, *agentCfg.config, port),
		}
	} else {
		// Config is written by context-init, or no config at all
		command = []string{
			"sh", "-c",
			fmt.Sprintf("/tools/opencode serve --port %d --hostname 0.0.0.0", port),
		}
	}

	// Build the main container
	container := corev1.Container{
		Name:            ServerContainerName,
		Image:           agentCfg.executorImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		WorkingDir:      agentCfg.workspaceDir,
		Command:         command,
		Env:             envVars,
		EnvFrom:         credEnvFroms,
		VolumeMounts:    volumeMounts,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       30,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   ServerHealthPath,
					Port:   intstr.FromInt32(port),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
	}

	// Apply resource requirements if specified in podSpec
	if agentCfg.podSpec != nil && agentCfg.podSpec.Resources != nil {
		container.Resources = *agentCfg.podSpec.Resources
	}

	// Build pod template spec
	podSpec := corev1.PodSpec{
		ServiceAccountName: agentCfg.serviceAccountName,
		InitContainers:     initContainers,
		Containers:         []corev1.Container{container},
		Volumes:            volumes,
		RestartPolicy:      corev1.RestartPolicyAlways,
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
					Labels: labels,
				},
				Spec: podSpec,
			},
		},
	}
}

// BuildServerService creates a Service for a Server-mode Agent.
func BuildServerService(agent *kubeopenv1alpha1.Agent) *corev1.Service {
	serverConfig := agent.Spec.ServerConfig
	if serverConfig == nil {
		return nil
	}

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
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromInt32(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// IsServerMode returns true if the Agent is configured for Server mode.
func IsServerMode(agent *kubeopenv1alpha1.Agent) bool {
	return agent.Spec.ServerConfig != nil
}

// GetServerPort returns the configured port or default.
func GetServerPort(agent *kubeopenv1alpha1.Agent) int32 {
	if agent.Spec.ServerConfig != nil && agent.Spec.ServerConfig.Port != 0 {
		return agent.Spec.ServerConfig.Port
	}
	return DefaultServerPort
}
