// Copyright Contributors to the KubeOpenCode project

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=agt
// +kubebuilder:printcolumn:JSONPath=`.spec.executorImage`,name="Image",type=string,priority=1
// +kubebuilder:printcolumn:JSONPath=`.spec.workspaceDir`,name="WorkspaceDir",type=string,priority=1
// +kubebuilder:printcolumn:JSONPath=`.spec.serviceAccountName`,name="ServiceAccount",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// AgentTemplate defines a reusable base configuration for Agents.
// Teams can maintain a single AgentTemplate with shared settings (images, contexts,
// credentials, etc.) and individual users can create Agents that reference the
// template, inheriting its configuration while optionally overriding specific fields.
type AgentTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the template configuration
	Spec AgentTemplateSpec `json:"spec"`

	// Status represents the current status of the AgentTemplate
	// +optional
	Status AgentTemplateStatus `json:"status,omitempty"`
}

// AgentTemplateSpec defines the shareable agent configuration.
// These fields serve as defaults for Agents that reference this template.
// Agent-level overrides take precedence over template values.
//
// Merge strategy when an Agent references this template:
//   - Scalar/pointer fields: Agent wins if non-zero/non-nil, else template value is used
//   - List fields (contexts, credentials, imagePullSecrets): Agent replaces template if non-nil
type AgentTemplateSpec struct {
	// AgentImage specifies the OpenCode init container image.
	// This image contains the OpenCode binary that gets copied to /tools volume.
	// If not specified, defaults to "quay.io/kubeopencode/kubeopencode-agent-opencode:latest".
	// +optional
	AgentImage string `json:"agentImage,omitempty"`

	// ExecutorImage specifies the main worker container image for task execution.
	// If not specified, defaults to "quay.io/kubeopencode/kubeopencode-agent-devbox:latest".
	// +optional
	ExecutorImage string `json:"executorImage,omitempty"`

	// AttachImage specifies the lightweight image used for Server-mode --attach Pods.
	// If not specified, defaults to "quay.io/kubeopencode/kubeopencode-agent-attach:latest".
	// This field is ignored when ServerConfig is nil (Pod mode).
	// +optional
	AttachImage string `json:"attachImage,omitempty"`

	// WorkspaceDir specifies the working directory inside the agent container.
	// This is where task.md and context files are mounted.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^/.*`
	// +kubebuilder:validation:MinLength=1
	WorkspaceDir string `json:"workspaceDir"`

	// Command specifies the entrypoint command for the agent container.
	// If not specified, defaults to:
	//   ["sh", "-c", "/tools/opencode run \"$(cat ${WORKSPACE_DIR}/task.md)\""]
	// +optional
	Command []string `json:"command,omitempty"`

	// Contexts provides default contexts for all tasks using Agents derived from this template.
	// +optional
	Contexts []ContextItem `json:"contexts,omitempty"`

	// Config provides OpenCode configuration as a JSON string.
	// +optional
	Config *string `json:"config,omitempty"`

	// Credentials defines secrets that should be available to the agent.
	// +optional
	Credentials []Credential `json:"credentials,omitempty"`

	// PodSpec defines advanced Pod configuration for agent pods.
	// +optional
	PodSpec *AgentPodSpec `json:"podSpec,omitempty"`

	// ServiceAccountName specifies the Kubernetes ServiceAccount to use for agent pods.
	// +required
	ServiceAccountName string `json:"serviceAccountName"`

	// CABundle configures custom CA certificates for TLS verification.
	// +optional
	CABundle *CABundleConfig `json:"caBundle,omitempty"`

	// Proxy configures HTTP/HTTPS proxy settings for all containers in generated Pods.
	// +optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`

	// ImagePullSecrets is a list of references to secrets for pulling container images
	// from private registries.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ServerConfig enables Server mode for Agents derived from this template.
	// +optional
	ServerConfig *ServerConfig `json:"serverConfig,omitempty"`
}

// AgentTemplateStatus defines the observed state of AgentTemplate
type AgentTemplateStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the AgentTemplate's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AgentTemplateList contains a list of AgentTemplate
type AgentTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentTemplate `json:"items"`
}
