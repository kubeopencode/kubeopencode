// Copyright Contributors to the KubeOpenCode project

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=ag
// +kubebuilder:printcolumn:JSONPath=`.spec.profile`,name="Profile",type=string,priority=1
// +kubebuilder:printcolumn:JSONPath=`.spec.executorImage`,name="Image",type=string,priority=1
// +kubebuilder:printcolumn:JSONPath=`.spec.serviceAccountName`,name="ServiceAccount",type=string
// +kubebuilder:printcolumn:JSONPath=`.spec.maxConcurrentTasks`,name="MaxTasks",type=integer,priority=1
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// Agent defines the AI agent configuration for task execution.
// Agent = AI agent + permissions + tools + infrastructure
// This is the execution black box - Task creators don't need to understand execution details.
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the agent configuration
	Spec AgentSpec `json:"spec"`

	// Status represents the current status of the Agent
	// +optional
	Status AgentStatus `json:"status,omitempty"`
}

// QuotaConfig defines rate limiting for Task starts within a sliding time window.
// This is different from maxConcurrentTasks which limits concurrent running Tasks.
// Quota limits the RATE at which new Tasks can start.
type QuotaConfig struct {
	// MaxTaskStarts is the maximum number of Task starts allowed within the window.
	// +kubebuilder:validation:Minimum=1
	// +required
	MaxTaskStarts int32 `json:"maxTaskStarts"`

	// WindowSeconds defines the sliding window duration in seconds.
	// For example, 3600 (1 hour) means "max N tasks per hour".
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=86400
	// +required
	WindowSeconds int32 `json:"windowSeconds"`
}

// TaskStartRecord represents a record of a Task start for quota tracking.
// Stored in AgentStatus to persist across controller restarts.
type TaskStartRecord struct {
	// TaskName is the name of the Task that was started.
	TaskName string `json:"taskName"`

	// TaskNamespace is the namespace of the Task.
	TaskNamespace string `json:"taskNamespace"`

	// StartTime is when the Task transitioned to Running phase.
	StartTime metav1.Time `json:"startTime"`
}

// ServerConfig enables Server mode for an Agent.
// When ServerConfig is present, the Agent runs as a persistent OpenCode server
// (Deployment + Service) instead of creating ephemeral Pods per Task.
// Tasks using a Server-mode Agent create lightweight Pods that connect to the
// server using `opencode run --attach`.
//
// Use Server mode for:
//   - Long-running agents (e.g., Slack bots, interactive assistants)
//   - Shared context across multiple Tasks (pre-loaded repositories)
//   - Avoiding cold start latency for each Task
type ServerConfig struct {
	// Port is the port OpenCode server listens on.
	// Defaults to 4096 if not specified.
	// +optional
	// +kubebuilder:default=4096
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Persistence configures persistent storage for the server.
	// When set, session data (and optionally workspace files) survive pod restarts.
	// +optional
	Persistence *PersistenceConfig `json:"persistence,omitempty"`

	// Suspend scales the server deployment to 0 replicas when true.
	// The server is stopped but PVCs and Service are retained, so the agent
	// can be resumed without data loss. Tasks targeting a suspended agent
	// enter Queued phase until the agent is resumed.
	//
	// This is useful for saving compute resources during off-hours while
	// preserving session history and workspace state via persistent storage.
	//
	// Similar to Kubernetes CronJob's spec.suspend field.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// PersistenceConfig controls persistent storage for Server-mode Agents.
// Session and workspace persistence are configured independently.
type PersistenceConfig struct {
	// Sessions enables persistent storage for OpenCode session data (SQLite DB).
	// A PVC is created to store the session database, so conversation
	// history survives server pod restarts.
	// +optional
	Sessions *VolumePersistence `json:"sessions,omitempty"`

	// Workspace enables persistent storage for the workspace directory.
	// When enabled, git-cloned repos, AI-modified files, and in-progress work
	// survive server pod restarts. Without this, workspace uses EmptyDir and
	// is re-initialized on every restart (git repos re-cloned by init containers).
	// +optional
	Workspace *VolumePersistence `json:"workspace,omitempty"`
}

// VolumePersistence defines PVC configuration for a persistent volume.
type VolumePersistence struct {
	// StorageClassName for the PVC. If empty, uses cluster default StorageClass.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Size of the PVC.
	// If not specified, defaults to 1Gi for sessions and 10Gi for workspace.
	// +optional
	Size string `json:"size,omitempty"`
}

// ServerStatus represents the observed state of a Server-mode Agent.
// This is only populated when ServerConfig is present in the Agent spec.
type ServerStatus struct {
	// DeploymentName is the name of the Kubernetes Deployment running the server.
	// Format: "{agent-name}-server"
	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceName is the name of the Kubernetes Service exposing the server.
	// Format: "{agent-name}"
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// URL is the in-cluster URL to reach the OpenCode server.
	// Format: "http://{service-name}.{namespace}.svc.cluster.local:{port}"
	// Tasks use this URL with `opencode run --attach` to connect to the server.
	// +optional
	URL string `json:"url,omitempty"`

	// Ready indicates whether the server deployment is ready to accept tasks.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Suspended indicates the server is intentionally scaled to 0 replicas.
	// When true, Ready is always false. Use this to distinguish "suspended"
	// from "not ready due to an issue".
	// +optional
	Suspended bool `json:"suspended,omitempty"`
}

// ProxyConfig configures HTTP/HTTPS proxy settings for all containers in generated Pods.
// These environment variables are injected into every init container and worker container.
// The ".svc" and ".cluster.local" suffixes are always appended to NoProxy to prevent
// proxying in-cluster traffic.
type ProxyConfig struct {
	// HttpProxy is the URL of the HTTP proxy server.
	// Sets HTTP_PROXY and http_proxy environment variables.
	// Example: "http://proxy.corp.example.com:8080"
	// +optional
	HttpProxy string `json:"httpProxy,omitempty"`

	// HttpsProxy is the URL of the HTTPS proxy server.
	// Sets HTTPS_PROXY and https_proxy environment variables.
	// Example: "http://proxy.corp.example.com:8080"
	// +optional
	HttpsProxy string `json:"httpsProxy,omitempty"`

	// NoProxy is a comma-separated list of hosts that should bypass the proxy.
	// Sets NO_PROXY and no_proxy environment variables.
	// ".svc" and ".cluster.local" are always appended automatically.
	// Example: "localhost,127.0.0.1,10.0.0.0/8,.corp.example.com"
	// +optional
	NoProxy string `json:"noProxy,omitempty"`
}

// AgentTemplateReference is a reference to an AgentTemplate in the same namespace.
type AgentTemplateReference struct {
	// Name of the AgentTemplate.
	// +required
	Name string `json:"name"`
}

// AgentSpec defines agent configuration
type AgentSpec struct {
	// TemplateRef references an AgentTemplate in the same namespace.
	// When set, the Agent inherits configuration from the template.
	// Agent-level fields override template values (scalar: non-zero wins; lists: replace).
	// +optional
	TemplateRef *AgentTemplateReference `json:"templateRef,omitempty"`

	// Profile is a brief, human-readable summary of the Agent's purpose and capabilities.
	// This is for documentation and discovery only — it has no functional effect on execution.
	// Visible via `kubectl get agents -o wide` for quick identification.
	//
	// Example:
	//   profile: "Full-stack development agent with GitHub and AWS access"
	// +optional
	Profile string `json:"profile,omitempty"`

	// AgentImage specifies the OpenCode init container image.
	// This image contains the OpenCode binary that gets copied to /tools volume.
	// The init container runs this image and copies the opencode binary to /tools/opencode.
	// If not specified, defaults to "quay.io/kubeopencode/kubeopencode-agent-opencode:latest".
	// +optional
	AgentImage string `json:"agentImage,omitempty"`

	// ExecutorImage specifies the main worker container image for task execution.
	// This is the development environment where tasks actually run.
	// The container uses /tools/opencode (provided by agentImage init container) to execute AI tasks.
	// If not specified, defaults to "quay.io/kubeopencode/kubeopencode-agent-devbox:latest".
	// +optional
	ExecutorImage string `json:"executorImage,omitempty"`

	// AttachImage specifies the lightweight image used for Server-mode --attach Pods.
	// When ServerConfig is set, Tasks create Pods that run `opencode run --attach <server-url>`.
	// These Pods only need the OpenCode binary and network access, not the full development
	// environment. Using a minimal image (~25MB) instead of devbox (~1GB) significantly
	// reduces image pull time and resource usage.
	//
	// If not specified, defaults to "quay.io/kubeopencode/kubeopencode-agent-attach:latest".
	// This field is ignored when ServerConfig is nil (Pod mode).
	// +optional
	AttachImage string `json:"attachImage,omitempty"`

	// WorkspaceDir specifies the working directory inside the agent container.
	// This is where task.md and context files are mounted.
	// The agent image must support the WORKSPACE_DIR environment variable.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^/.*`
	// +kubebuilder:validation:MinLength=1
	WorkspaceDir string `json:"workspaceDir"`

	// Command specifies the entrypoint command for the agent container.
	// This is optional and overrides the default ENTRYPOINT of the container image.
	//
	// If not specified, defaults to:
	//   ["sh", "-c", "/tools/opencode run \"$(cat ${WORKSPACE_DIR}/task.md)\""]
	//
	// The command defines HOW the agent executes tasks. Most users should not
	// need to customize this. Override only if you need custom execution behavior.
	//
	// ## Example
	//
	//   command: ["sh", "-c", "/tools/opencode run --format json \"$(cat /workspace/task.md)\""]
	//
	// +optional
	Command []string `json:"command,omitempty"`

	// Contexts provides default contexts for all tasks using this Agent.
	// These have the lowest priority in context merging.
	//
	// Context priority (lowest to highest):
	//   1. Agent.contexts (Agent-level defaults)
	//   2. Task.contexts (Task-specific contexts)
	//   3. Task.description (highest, becomes ${WORKSPACE_DIR}/task.md)
	//
	// Use this for organization-wide defaults like coding standards, security policies,
	// or common tool configurations that should apply to all tasks.
	// +optional
	Contexts []ContextItem `json:"contexts,omitempty"`

	// Config provides OpenCode configuration as a JSON string.
	// This configuration is written to /tools/opencode.json and the OPENCODE_CONFIG
	// environment variable is set to point to this file.
	//
	// The config should be a valid JSON object compatible with OpenCode's config schema.
	// See: https://opencode.ai/config.json for the schema.
	//
	// Example:
	//   config: |
	//     {
	//       "$schema": "https://opencode.ai/config.json",
	//       "model": "google/gemini-2.5-pro",
	//       "small_model": "google/gemini-2.5-flash"
	//     }
	// +optional
	Config *string `json:"config,omitempty"`

	// Credentials defines secrets that should be available to the agent.
	// Similar to GitHub Actions secrets, these can be mounted as files or
	// exposed as environment variables.
	//
	// Example use cases:
	//   - GitHub token for repository access (env: GITHUB_TOKEN)
	//   - SSH keys for git operations (file: ~/.ssh/id_rsa)
	//   - API keys for external services (env: ANTHROPIC_API_KEY)
	//   - Cloud credentials (file: ~/.config/gcloud/credentials.json)
	// +optional
	Credentials []Credential `json:"credentials,omitempty"`

	// PodSpec defines advanced Pod configuration for agent pods.
	// This includes labels, scheduling, runtime class, and other Pod-level settings.
	// Use this for fine-grained control over how agent pods are created.
	// +optional
	PodSpec *AgentPodSpec `json:"podSpec,omitempty"`

	// ServiceAccountName specifies the Kubernetes ServiceAccount to use for agent pods.
	// This controls what cluster resources the agent can access via RBAC.
	//
	// The ServiceAccount must exist in the Agent's namespace (where Pods run).
	// Users are responsible for creating the ServiceAccount and appropriate RBAC bindings
	// based on what permissions their agent needs.
	//
	// +required
	ServiceAccountName string `json:"serviceAccountName"`

	// MaxConcurrentTasks limits the number of Tasks that can run concurrently
	// using this Agent. When the limit is reached, new Tasks will enter Queued
	// phase until capacity becomes available.
	//
	// This is useful when the Agent uses backend AI services with rate limits
	// (e.g., Claude, Gemini API quotas) to prevent overwhelming the service.
	//
	// - nil or 0: unlimited (default behavior, no concurrency limit)
	// - positive number: maximum number of Tasks that can be in Running phase
	//
	// Example:
	//   maxConcurrentTasks: 3  # Only 3 Tasks can run at once
	// +optional
	MaxConcurrentTasks *int32 `json:"maxConcurrentTasks,omitempty"`

	// Quota defines rate limiting for Task starts within a sliding time window.
	// When configured, Tasks will be queued if the quota is exceeded.
	// This is complementary to maxConcurrentTasks:
	//   - maxConcurrentTasks: limits how many Tasks run at once
	//   - quota: limits how quickly new Tasks can start
	//
	// Example:
	//   quota:
	//     maxTaskStarts: 10
	//     windowSeconds: 3600  # 10 tasks per hour
	// +optional
	Quota *QuotaConfig `json:"quota,omitempty"`

	// CABundle configures custom CA certificates for TLS verification.
	// The CA bundle is mounted into all init containers (git-init, url-fetch, context-init)
	// and the worker container, enabling HTTPS access to servers using private/self-signed CAs.
	//
	// Compatible with cert-manager trust-manager Bundle resources (ConfigMap with "ca-bundle.crt" key).
	//
	// Example:
	//   caBundle:
	//     configMapRef:
	//       name: custom-ca-bundle
	//       key: ca-bundle.crt
	// +optional
	CABundle *CABundleConfig `json:"caBundle,omitempty"`

	// Proxy configures HTTP/HTTPS proxy settings for all containers in generated Pods.
	// When set, HTTP_PROXY, HTTPS_PROXY, NO_PROXY (and lowercase variants) environment
	// variables are injected into all init containers and the worker container.
	//
	// This is useful in enterprise environments where all outbound traffic must go
	// through a corporate proxy server.
	//
	// Agent-level proxy overrides cluster-level proxy from KubeOpenCodeConfig.
	//
	// Example:
	//   proxy:
	//     httpProxy: "http://proxy.corp.example.com:8080"
	//     httpsProxy: "http://proxy.corp.example.com:8080"
	//     noProxy: "localhost,127.0.0.1,10.0.0.0/8"
	// +optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`

	// ImagePullSecrets is a list of references to secrets for pulling container images
	// from private registries. These are added to the Pod spec's imagePullSecrets field.
	//
	// This is useful when agentImage, executorImage, or attachImage are hosted in
	// private registries that require authentication (e.g., Harbor, AWS ECR, GCR).
	//
	// The referenced Secrets must exist in the same namespace as the Agent and be
	// of type kubernetes.io/dockerconfigjson.
	//
	// Example:
	//   imagePullSecrets:
	//     - name: harbor-registry-secret
	//     - name: gcr-secret
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ServerConfig enables Server mode for this Agent.
	// When set, the Agent runs as a persistent OpenCode server (Deployment + Service)
	// instead of creating ephemeral Pods per Task.
	//
	// Server mode is useful for:
	//   - Long-running agents (e.g., Slack bots, interactive assistants)
	//   - Shared context across Tasks (pre-loaded repositories, faster startup)
	//   - Avoiding cold start latency for each Task
	//
	// When ServerConfig is nil (default), the Agent operates in Pod mode:
	// each Task creates a new Pod that runs to completion.
	//
	// In Server mode, Tasks create lightweight Pods that use `opencode run --attach`
	// to connect to the persistent server.
	//
	// Example:
	//   serverConfig:
	//     port: 4096
	// +optional
	ServerConfig *ServerConfig `json:"serverConfig,omitempty"`
}

// AgentStatus defines the observed state of Agent
type AgentStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the Agent's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// TaskStartHistory tracks recent Task starts for quota enforcement.
	// The controller prunes entries older than the quota window automatically.
	// This is only populated when quota is configured on the Agent.
	// +optional
	// +listType=atomic
	TaskStartHistory []TaskStartRecord `json:"taskStartHistory,omitempty"`

	// ServerStatus contains the status of the OpenCode server when running in Server mode.
	// This is only populated when spec.serverConfig is set.
	// +optional
	ServerStatus *ServerStatus `json:"serverStatus,omitempty"`
}

// AgentPodSpec defines advanced Pod configuration for agent pods.
// This groups all Pod-level settings that control how the agent container runs.
// These settings apply to both Pod mode and Server mode.
type AgentPodSpec struct {
	// Labels defines additional labels to add to the agent pod.
	// These labels are applied to the Job's pod template and enable integration with:
	//   - NetworkPolicy podSelector for network isolation
	//   - Service selector for service discovery
	//   - PodMonitor/ServiceMonitor for Prometheus monitoring
	//   - Any other label-based pod selection
	//
	// Example: To make pods match a NetworkPolicy with podSelector:
	//   labels:
	//     network-policy: agent-restricted
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Scheduling defines pod scheduling configuration for agent pods.
	// This includes node selection, tolerations, and affinity rules.
	// +optional
	Scheduling *PodScheduling `json:"scheduling,omitempty"`

	// RuntimeClassName specifies the RuntimeClass to use for agent pods.
	// RuntimeClass provides a way to select container runtime configurations
	// such as gVisor (runsc) or Kata Containers for enhanced isolation.
	//
	// This is useful when running untrusted AI agent code that may generate
	// and execute arbitrary commands. Using gVisor or Kata provides an
	// additional layer of security beyond standard container isolation.
	//
	// The RuntimeClass must exist in the cluster before use.
	// Common values: "gvisor", "kata", "runc" (default if not specified)
	//
	// Example:
	//   runtimeClassName: gvisor
	//
	// See: https://kubernetes.io/docs/concepts/containers/runtime-class/
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Resources specifies the compute resources (CPU, memory) for the agent container.
	// This applies to both Pod mode (per-Task Pods) and Server mode (Deployment).
	// If not specified, uses the cluster's default resource limits.
	//
	// Example:
	//   resources:
	//     requests:
	//       memory: "512Mi"
	//       cpu: "500m"
	//     limits:
	//       memory: "2Gi"
	//       cpu: "2"
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// SecurityContext defines the security options for the agent container.
	// This is applied to the worker container (and init containers where applicable).
	//
	// If not specified, a restricted default is applied:
	//   - allowPrivilegeEscalation: false
	//   - capabilities: drop ALL
	//   - seccompProfile: RuntimeDefault
	//
	// Enterprise users can further tighten this with:
	//   - runAsNonRoot: true
	//   - readOnlyRootFilesystem: true (requires emptyDir volumes for writable paths)
	//
	// Example:
	//   securityContext:
	//     runAsNonRoot: true
	//     allowPrivilegeEscalation: false
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// PodSecurityContext defines pod-level security attributes and common container settings.
	// Applied to the Pod spec directly (affects all containers).
	//
	// Example:
	//   podSecurityContext:
	//     runAsUser: 1000
	//     runAsGroup: 1000
	//     fsGroup: 1000
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

// PodScheduling defines scheduling configuration for agent pods.
// All fields are applied directly to the Job's pod template.
type PodScheduling struct {
	// NodeSelector specifies a selector for scheduling pods to specific nodes.
	// The pod will only be scheduled to nodes that have all the specified labels.
	//
	// Example:
	//   nodeSelector:
	//     kubernetes.io/os: linux
	//     node-type: gpu
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations allows pods to be scheduled on nodes with matching taints.
	//
	// Example:
	//   tolerations:
	//     - key: "dedicated"
	//       operator: "Equal"
	//       value: "ai-workload"
	//       effect: "NoSchedule"
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity specifies affinity and anti-affinity rules for pods.
	// This enables advanced scheduling based on node attributes, pod co-location,
	// or pod anti-affinity for high availability.
	//
	// Example:
	//   affinity:
	//     nodeAffinity:
	//       requiredDuringSchedulingIgnoredDuringExecution:
	//         nodeSelectorTerms:
	//           - matchExpressions:
	//               - key: topology.kubernetes.io/zone
	//                 operator: In
	//                 values: ["us-west-2a", "us-west-2b"]
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// Credential represents a secret that should be available to the agent.
// Each credential references a Kubernetes Secret and specifies how to expose it.
//
// Mounting behavior depends on whether SecretRef.Key is specified:
//
// 1. No Key specified + No MountPath: entire Secret as environment variables
// 2. No Key specified + MountPath: entire Secret as directory (each key becomes a file)
// 3. Key specified + Env: single key as environment variable
// 4. Key specified + MountPath: single key as file
// +kubebuilder:validation:XValidation:rule="!has(self.env) || has(self.secretRef.key)",message="env can only be set when secretRef.key is specified"
type Credential struct {
	// Name is a descriptive name for this credential (for documentation purposes).
	// +required
	Name string `json:"name"`

	// SecretRef references the Kubernetes Secret containing the credential.
	// +required
	SecretRef SecretReference `json:"secretRef"`

	// MountPath specifies where to mount the secret.
	// - If SecretRef.Key is specified: mounts the single key's value as a file at this path.
	//   Example: "/home/agent/.ssh/id_rsa" for SSH keys
	// - If SecretRef.Key is not specified: mounts the entire Secret as a directory,
	//   where each key in the Secret becomes a file in the directory.
	//   Example: "/etc/ssl/certs" for a Secret containing ca.crt, client.crt, client.key
	// +optional
	MountPath *string `json:"mountPath,omitempty"`

	// Env specifies the environment variable name to expose the secret value.
	// Only applicable when SecretRef.Key is specified.
	// If specified, the secret key's value is set as this environment variable.
	// Example: "GITHUB_TOKEN" for GitHub API access
	// +optional
	Env *string `json:"env,omitempty"`

	// FileMode specifies the permission mode for mounted files.
	// Only applicable when MountPath is specified.
	// Defaults to 0600 (read/write for owner only) for security.
	// Use 0400 for read-only files like SSH keys.
	// +optional
	FileMode *int32 `json:"fileMode,omitempty"`
}

// SecretReference references a Kubernetes Secret.
// When Key is specified, only that specific key is used.
// When Key is omitted, the entire Secret is used (behavior depends on Credential.MountPath).
type SecretReference struct {
	// Name of the Secret.
	// +required
	Name string `json:"name"`

	// Key of the Secret to select.
	// If not specified, the entire Secret is used:
	// - With MountPath: mounted as a directory (each key becomes a file)
	// - Without MountPath: all keys become environment variables
	// When Key is omitted, the Env field on the Credential is ignored.
	// +optional
	Key *string `json:"key,omitempty"`
}

// CABundleConfig configures custom CA certificates for TLS verification.
// The CA bundle is mounted into all init containers and the worker container,
// enabling access to HTTPS services that use private or self-signed certificates.
//
// Exactly one of ConfigMapRef or SecretRef must be specified.
// +kubebuilder:validation:XValidation:rule="has(self.configMapRef) || has(self.secretRef)",message="either configMapRef or secretRef must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.configMapRef) && has(self.secretRef))",message="only one of configMapRef or secretRef can be specified"
type CABundleConfig struct {
	// ConfigMapRef references a ConfigMap containing PEM-encoded CA certificates.
	// Compatible with cert-manager trust-manager Bundle resources.
	// +optional
	ConfigMapRef *CABundleReference `json:"configMapRef,omitempty"`

	// SecretRef references a Secret containing PEM-encoded CA certificates.
	// +optional
	SecretRef *CABundleReference `json:"secretRef,omitempty"`
}

// CABundleReference references a ConfigMap or Secret containing a CA bundle.
type CABundleReference struct {
	// Name of the ConfigMap or Secret.
	// +required
	Name string `json:"name"`

	// Key containing the PEM-encoded CA bundle.
	// Defaults to "ca-bundle.crt" for ConfigMapRef, "ca.crt" for SecretRef.
	// +optional
	Key string `json:"key,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AgentList contains a list of Agent
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}
